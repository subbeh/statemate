package cli

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/util"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt <source>",
	Short: "Encrypt a managed file",
	Long: `Encrypt a file in place.

This reads the file, encrypts it using the configured age recipients,
writes it back, and adds the #enc suffix to the filename.

The file can be a managed file, named by its source or its target, or any
other file (e.g. a var_file in .matedata/). Paths are resolved relative to the
current directory, falling back to the source directory. A bare file name is
not searched for.

The age recipients must be configured in mate.yaml:

  age:
    recipients:
      - age1...`,
	Example: `  mate encrypt nvim/secrets.yaml
  mate encrypt .matedata/secrets.yaml`,
	Args:              cobra.ExactArgs(1),
	RunE:              runEncrypt,
	ValidArgsFunction: completeFilePaths,
}

func init() {
	rootCmd.AddCommand(encryptCmd)
}

func runEncrypt(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg := env.Config

	if cfg.Age == nil || len(cfg.Age.Recipients) == 0 {
		return fmt.Errorf("no age recipients configured in mate.yaml")
	}

	enc, err := encrypt.NewAgeEncryptor("", "", cfg.Age.Recipients)
	if err != nil {
		return fmt.Errorf("setting up encryption: %w", err)
	}

	srcPattern := args[0]
	entry, err := env.Resolve(srcPattern)
	var notManaged *plan.NotManagedError
	switch {
	case err == nil:
		if entry.Attrs.Encrypted {
			return fmt.Errorf("file is already encrypted: %s", srcPattern)
		}
		return encryptFileAt(entry.SourcePath, entry.Mode.Perm(), enc)
	case !errors.As(err, &notManaged):
		return err
	}

	// Not a managed file, such as a var_file in .matedata/.
	filePath := notManaged.Path
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not found: %s", srcPattern)
	}
	if hasEncryptedAttr(filePath) {
		return fmt.Errorf("file is already encrypted: %s", srcPattern)
	}

	return encryptFileAt(filePath, info.Mode().Perm(), enc)
}

func resolveEncryptedFilePath(pattern string, sourceDir string) string {
	// The exact path, then its #enc variant.
	find := func(p string) string {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		if hasEncryptedAttr(p) {
			return ""
		}
		return util.EncryptedVariant(p)
	}

	if filepath.IsAbs(pattern) {
		return find(pattern)
	}
	// Relative to cwd first, then to the source directory.
	if abs, err := filepath.Abs(pattern); err == nil {
		if p := find(abs); p != "" {
			return p
		}
	}
	return find(filepath.Join(sourceDir, pattern))
}

func encryptFileAt(path string, perm os.FileMode, enc *encrypt.AgeEncryptor) error {
	plaintext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypting: %w", err)
	}

	newPath := path + "#enc"

	if err := os.WriteFile(newPath, ciphertext, perm); err != nil {
		return fmt.Errorf("writing encrypted file: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing original file: %w", err)
	}

	slog.Info("source", "action", "encrypt", "path", path, "to", newPath)
	fmt.Printf("Encrypted: %s -> %s\n", util.ShortenPath(path), util.ShortenPath(newPath))
	return nil
}
