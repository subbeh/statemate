package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/util"
)

var editCmd = &cobra.Command{
	Use:   "edit <path>",
	Short: "Edit a managed file",
	Long: `Edit a managed file in your editor.

Paths are resolved like any other command-line tool: absolute paths are used
as-is, relative paths resolve against the current directory.

Files under the source directory are opened directly. If you pass a target
path (a deployed file), the corresponding source file is opened instead --
mate never edits deployed files in place.

For encrypted files (with the '#encrypted' suffix), the file is decrypted to a
temporary location, opened in the editor, and re-encrypted after saving. The
original file permissions are preserved.

The editor is determined by (in order):
  1. The 'editor' field in mate.yaml
  2. $VISUAL environment variable
  3. $EDITOR environment variable
  4. vi (fallback)

Examples:
  mate edit nvim/init.lua
  mate edit .matedata/secrets.yaml#encrypted
  mate edit ~/.config/nvim/init.lua`,
	Args: cobra.ExactArgs(1),
	RunE: runEdit,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveDefault
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	allSources := profile.AllSources(cfg)
	allSourcePaths := cfg.ResolveSourcePaths(allSources)

	scanner := source.NewScannerWithIgnore(cfg.TargetBase, cfg.SourceDir(), nil, cfg.Ignore)
	tree, err := scanner.Scan(allSourcePaths)
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	editPath, isManagedSource, err := resolveEditPath(args[0], cfg, tree.Files())
	if err != nil {
		return err
	}

	editor := getEditor(cfg)

	if !strings.Contains(filepath.Base(editPath), "#encrypted") {
		if err := runEditor(editor, editPath); err != nil {
			return err
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

	baseName := strings.TrimSuffix(filepath.Base(editPath), "#encrypted")
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
	cmd := exec.Command(editor, path)
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
func resolveEditPath(input string, cfg *config.Config, entries []*source.Entry) (string, bool, error) {
	abs, err := expandToAbs(input)
	if err != nil {
		return "", false, err
	}

	sourceDir := resolveSymlinks(cfg.SourceDir())
	abs = resolveSymlinks(abs)

	// Path is inside the repo -- edit it directly.
	if abs == sourceDir || strings.HasPrefix(abs, sourceDir+string(filepath.Separator)) {
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return "", false, fmt.Errorf("file not found: %s", input)
			}
			return "", false, err
		}
		return abs, isManagedSourcePath(abs, entries), nil
	}

	// Path is outside the repo -- only valid if it is a managed target.
	for _, e := range entries {
		if resolveSymlinks(e.TargetPath) == abs {
			return resolveSymlinks(e.SourcePath), true, nil
		}
	}

	return "", false, fmt.Errorf("%s is not managed by mate", input)
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

// expandToAbs expands a leading ~ and resolves the path against the current
// directory, returning a cleaned absolute path.
func expandToAbs(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path %s: %w", path, err)
	}
	return abs, nil
}

// isManagedSourcePath reports whether the path is the source of a managed entry.
// The input is expected to already have symlinks resolved.
func isManagedSourcePath(abs string, entries []*source.Entry) bool {
	for _, e := range entries {
		if resolveSymlinks(e.SourcePath) == abs {
			return true
		}
	}
	return false
}
