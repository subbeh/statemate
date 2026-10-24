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
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/util"
)

var decryptCmd = &cobra.Command{
	Use:   "decrypt <source>",
	Short: "Decrypt a managed file",
	Long: `Decrypt a file in place.

This reads the encrypted file, decrypts it using the configured age identity,
writes it back, and removes the #enc attribute from the filename
wherever it appears among the attributes (config#enc#tmpl becomes
config#tmpl).

The file can be a managed file, named by its source or its target, or any
other file (e.g. a var_file in .matedata/). Paths are resolved relative to the
current directory, falling back to the source directory. The #enc suffix is
optional. A bare file name is not searched for.

The age identity must be configured in mate.yaml:

  age:
    identity: "~/.config/statemate/key.txt"`,
	Example: `  mate decrypt nvim/secrets.yaml#enc
  mate decrypt .matedata/secrets.yaml`,
	Args:              cobra.ExactArgs(1),
	RunE:              runDecrypt,
	ValidArgsFunction: completeFilePaths,
}

func init() {
	rootCmd.AddCommand(decryptCmd)
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg := env.Config

	if cfg.Age == nil || (cfg.Age.Identity == "" && cfg.Age.IdentityCommand == "") {
		return fmt.Errorf("no age identity configured in mate.yaml")
	}

	enc, err := encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, nil)
	if err != nil {
		return fmt.Errorf("setting up decryption: %w", err)
	}

	srcPattern := args[0]
	entry, err := env.Resolve(srcPattern)
	switch {
	case err == nil:
		if !entry.Attrs.Encrypted {
			return fmt.Errorf("file is not encrypted: %s", srcPattern)
		}
		return decryptFileAt(entry.SourcePath, entry.Mode.Perm(), enc)
	case !errors.As(err, new(*plan.NotManagedError)):
		return err
	}

	// Not a managed file, such as a var_file in .matedata/ (try with #enc suffix too)
	filePath := resolveEncryptedFilePath(srcPattern, cfg.SourceDir())
	if filePath == "" {
		return fmt.Errorf("file not found: %s", srcPattern)
	}
	if !hasEncryptedAttr(filePath) {
		return fmt.Errorf("file is not encrypted: %s", srcPattern)
	}
	info, statErr := os.Stat(filePath)
	if statErr != nil {
		return fmt.Errorf("file not found: %s", srcPattern)
	}

	return decryptFileAt(filePath, info.Mode().Perm(), enc)
}

func decryptFileAt(path string, perm os.FileMode, enc *encrypt.AgeEncryptor) error {
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypting: %w", err)
	}

	newPath := filepath.Join(filepath.Dir(path), source.RemoveAttr(filepath.Base(path), "enc"))
	// Without an #enc attribute to drop, the plaintext would overwrite the
	// ciphertext and the removal below would then delete it.
	if newPath == path {
		return fmt.Errorf("%s has no #enc attribute to remove", util.ShortenPath(path))
	}

	if err := os.WriteFile(newPath, plaintext, perm); err != nil {
		return fmt.Errorf("writing decrypted file: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing encrypted file: %w", err)
	}

	slog.Info("source", "action", "decrypt", "path", path, "to", newPath)
	fmt.Printf("Decrypted: %s -> %s\n", util.ShortenPath(path), util.ShortenPath(newPath))
	return nil
}

// hasEncryptedAttr reports whether the file's own name carries the #enc
// attribute, in any position among its attributes. A suffix test misses
// x#enc#tmpl; a substring test also matches x#enc-backup, or a
// file inside a directory whose name contains "#enc".
func hasEncryptedAttr(path string) bool {
	_, attrs := source.ParseAttrs(filepath.Base(path))
	return attrs.Encrypted
}
