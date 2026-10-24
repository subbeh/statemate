package cli

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/util"
)

var editCmd = &cobra.Command{
	Use:   "edit <path>",
	Short: "Edit a managed file",
	Long: `Edit a managed file in your editor.

Paths are resolved like any other command-line tool: absolute paths are used
as-is, relative paths resolve against the current directory. A relative path
that is not there is also looked up in the source directory. A bare file name
is not searched for.

Files under the source directory are opened directly. If you pass a target
path (a deployed file), the corresponding source file is opened instead --
mate never edits deployed files in place.

For encrypted files (with the '#enc' attribute), the file is decrypted to a
temporary location, opened in the editor, and re-encrypted after saving. The
original file permissions are preserved.

The editor is determined by (in order):
  1. The 'editor' field in mate.yaml
  2. $VISUAL environment variable
  3. $EDITOR environment variable
  4. vi (fallback)

The editor command may include arguments, such as 'code --wait'.`,
	Example: `  mate edit ~/.config/nvim/init.lua      # opens the source in the repository
  mate edit nvim/.config/nvim/init.lua
  mate edit ~/.ssh/config                # decrypts, edits, re-encrypts`,
	Args:              cobra.ExactArgs(1),
	RunE:              runEdit,
	ValidArgsFunction: completeFilePaths,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg := env.Config

	editPath, isManagedSource, err := resolveEditPath(args[0], env)
	if err != nil {
		return err
	}

	editor := getEditor(cfg)

	if !hasEncryptedAttr(editPath) {
		before, _ := os.ReadFile(editPath)
		if err := runEditor(editor, editPath); err != nil {
			return err
		}
		if after, _ := os.ReadFile(editPath); !bytes.Equal(before, after) {
			slog.Info("source", "action", "edit", "path", editPath)
		}
		if isManagedSource {
			fmt.Printf("Edited %s -- run 'mate apply' to deploy changes\n", util.ShortenPath(editPath))
		}
		return nil
	}

	if cfg.Age == nil || (cfg.Age.Identity == "" && cfg.Age.IdentityCommand == "") {
		return fmt.Errorf("no age identity configured for decryption")
	}
	if cfg.Age == nil || len(cfg.Age.Recipients) == 0 {
		return fmt.Errorf("no age recipients configured for encryption")
	}

	enc, err := encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
	if err != nil {
		return fmt.Errorf("setting up encryption: %w", err)
	}

	// Capture the existing mode so it can be preserved on write-back.
	info, err := os.Stat(editPath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	origMode := info.Mode().Perm()

	ciphertext, err := os.ReadFile(editPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypting: %w", err)
	}

	// The temp file keeps the attribute-free name so the editor still sees the
	// real extension.
	baseName, _ := source.ParseAttrs(filepath.Base(editPath))
	tmpFile, err := os.CreateTemp("", "mate-edit-*-"+baseName)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	// Plaintext secrets must never be group/world readable.
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("securing temp file: %w", err)
	}

	if _, err := tmpFile.Write(plaintext); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	_ = tmpFile.Close()

	beforeHash, err := state.HashFile(tmpPath)
	if err != nil {
		return fmt.Errorf("hashing temp file: %w", err)
	}

	if err := runEditor(editor, tmpPath); err != nil {
		return err
	}

	afterHash, err := state.HashFile(tmpPath)
	if err != nil {
		return fmt.Errorf("hashing temp file: %w", err)
	}

	if beforeHash == afterHash {
		fmt.Println("No changes made")
		return nil
	}

	newPlaintext, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading edited file: %w", err)
	}

	newCiphertext, err := enc.Encrypt(newPlaintext)
	if err != nil {
		return fmt.Errorf("encrypting: %w", err)
	}

	if err := os.WriteFile(editPath, newCiphertext, origMode); err != nil {
		return fmt.Errorf("writing encrypted file: %w", err)
	}
	// WriteFile only applies mode when creating, so enforce it explicitly.
	if err := os.Chmod(editPath, origMode); err != nil {
		return fmt.Errorf("restoring permissions: %w", err)
	}

	slog.Info("source", "action", "edit", "path", editPath)
	fmt.Printf("Saved and encrypted: %s\n", util.ShortenPath(editPath))
	if isManagedSource {
		fmt.Println("Run 'mate apply' to deploy changes")
	}

	return nil
}

func getEditor(cfg *config.Config) string {
	if cfg.Editor != "" {
		return cfg.Editor
	}

	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	return "vi"
}

func runEditor(editor, path string) error {
	cmd := util.UserCommand(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	return nil
}

// resolveEditPath resolves a user-supplied path to the file that should be
// opened in the editor. Paths behave like any other CLI file argument: absolute
// paths are used as-is, relative paths resolve against the current directory.
//
// A path is editable if it lives under the source directory, or if it is a
// target path produced by a managed source (in which case the source file is
// returned -- mate never edits deployed files in place).
//
// The second return value reports whether the resolved file is a managed source
// file, so callers can decide whether to show an "apply pending" hint.
func resolveEditPath(input string, env *plan.Env) (string, bool, error) {
	entry, err := env.Resolve(input)
	if err == nil {
		return resolveSymlinks(entry.SourcePath), true, nil
	}
	var notManaged *plan.NotManagedError
	if !errors.As(err, &notManaged) {
		return "", false, err
	}

	// Any other file inside the repo, such as an include or var_file, is
	// edited directly.
	sourceDir := resolveSymlinks(env.Config.SourceDir())
	abs := resolveSymlinks(notManaged.Path)
	if abs == sourceDir || strings.HasPrefix(abs, sourceDir+string(filepath.Separator)) {
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return "", false, fmt.Errorf("file not found: %s", input)
			}
			return "", false, err
		}
		return abs, false, nil
	}
	return "", false, err
}

// resolveSymlinks returns the path with symlinks resolved, falling back to the
// original path when it cannot be resolved (e.g. it does not exist yet). This
// keeps prefix comparisons reliable on systems where temp and home directories
// are symlinked.
func resolveSymlinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

