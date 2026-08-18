package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect resolved configuration",
	Long:  "Show resolved configuration values, for use in scripts and editor integrations",
}

var configSourceDirCmd = &cobra.Command{
	Use:   "source-dir",
	Short: "Print the resolved source directory",
	Long: `Print the absolute path of the directory containing mate.yaml.

The path is printed bare, with no label, so it can be used directly:

  cd "$(mate config source-dir)"

Resolution order matches the rest of mate: the --config flag, then the
STATEMATE_DIR environment variable, then source_dir in the local config
(~/.config/statemate/mate.yaml), then the current directory.`,
	Args: cobra.NoArgs,
	RunE: runConfigSourceDir,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSourceDirCmd)
}

func runConfigSourceDir(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dir := cfg.SourceDir()
	if dir == "" {
		return fmt.Errorf("could not resolve source directory")
	}

	fmt.Println(dir)
	return nil
}
