package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
)

var catCmd = &cobra.Command{
	Use:   "cat <file>",
	Short: "Print a file, decrypting it if needed",
	Long: `Print a file, decrypting it first if it is age-encrypted.

Works like cat, but an age-encrypted file is decrypted with the configured
identity. The path is used as given: absolute, starting with ~, or relative to
the current directory.`,
	Example: `  mate cat ssh/.ssh/config#encrypted
  mate cat ~/.config/app/config.yaml`,
	Args:              cobra.ExactArgs(1),
	RunE:              runCat,
	ValidArgsFunction: completeFilePaths,
}

func init() {
	rootCmd.AddCommand(catCmd)
}

func runCat(cmd *cobra.Command, args []string) error {
	filePath := expandPath(args[0])

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if !isEncrypted(content) {
		_, _ = os.Stdout.Write(content)
		return nil
	}

	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.Age == nil {
		return fmt.Errorf("file is encrypted but no age config found")
	}

	enc, err := encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
	if err != nil {
		return fmt.Errorf("setting up encryption: %w", err)
	}

	if !enc.CanDecrypt() {
		return fmt.Errorf("file is encrypted but no identity available for decryption")
	}

	decrypted, err := enc.Decrypt(content)
	if err != nil {
		return fmt.Errorf("decrypting file: %w", err)
	}

	_, _ = os.Stdout.Write(decrypted)
	return nil
}
