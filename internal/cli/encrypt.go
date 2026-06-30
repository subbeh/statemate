package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/util"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt <source>",
	Short: "Encrypt a managed file",
	Long: `Encrypt a managed file in place.

This reads the file, encrypts it using the configured age recipients,
writes it back, and adds the #encrypted suffix to the filename.

The age recipients must be configured in mate.yaml:

  age:
    recipients:
      - age1...

Examples:
  mate encrypt nvim/secrets.yaml
  mate encrypt zsh/.zshrc`,
	Args:              cobra.ExactArgs(1),
	RunE:              runEncrypt,
	ValidArgsFunction: completeManagedFiles,
}

func init() {
	rootCmd.AddCommand(encryptCmd)
}

func runEncrypt(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	if cfg.Age == nil || len(cfg.Age.Recipients) == 0 {
		return fmt.Errorf("no age recipients configured in mate.yaml")
	}

	enc, err := encrypt.NewAgeEncryptor("", "", cfg.Age.Recipients)
	if err != nil {
		return fmt.Errorf("setting up encryption: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	allSources := profile.AllSources(cfg)
	allSourcePaths := cfg.ResolveSourcePaths(allSources)

	scanner := source.NewScanner(cfg.TargetBase, cfg.SourceDir())
	tree, err := scanner.Scan(allSourcePaths)
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	srcPattern := args[0]

	var entry *source.Entry
	for _, e := range tree.Files() {
		srcDir := strings.TrimSuffix(e.SourcePath, "/"+e.RelPath)
		relPath := filepath.Join(filepath.Base(srcDir), e.RelPath)
		if relPath == srcPattern || e.SourcePath == srcPattern || e.TargetPath == srcPattern ||
			strings.HasSuffix(relPath, "/"+srcPattern) ||
			strings.HasSuffix(e.SourcePath, "/"+srcPattern) ||
			strings.HasSuffix(e.TargetPath, "/"+srcPattern) {
			entry = e
			break
		}
	}

	if entry == nil {
		return fmt.Errorf("file not found: %s", srcPattern)
	}

	if entry.Attrs.Encrypted {
		return fmt.Errorf("file is already encrypted: %s", srcPattern)
	}

	plaintext, err := os.ReadFile(entry.SourcePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypting: %w", err)
	}

	newPath := entry.SourcePath + "#encrypted"

	if err := os.WriteFile(newPath, ciphertext, entry.Mode.Perm()); err != nil {
		return fmt.Errorf("writing encrypted file: %w", err)
	}

	if err := os.Remove(entry.SourcePath); err != nil {
		return fmt.Errorf("removing original file: %w", err)
	}

	fmt.Printf("Encrypted: %s -> %s\n", util.ShortenPath(entry.SourcePath), util.ShortenPath(newPath))

	return nil
}
