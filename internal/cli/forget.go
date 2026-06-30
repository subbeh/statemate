package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/state"
)

var forgetCmd = &cobra.Command{
	Use:               "forget <path>",
	Short:             "Remove file from tracking",
	Long: `Remove a file from statemate's tracking database.

The file at target remains untouched. Only the tracking entry is removed.
This is useful when you want statemate to stop managing a file without
deleting it.

Example:
  mate forget ~/.config/nvim/init.lua`,
	Args:              cobra.ExactArgs(1),
	RunE:              runForget,
	ValidArgsFunction: completeTrackedFiles,
}

func init() {
	rootCmd.AddCommand(forgetCmd)
}

func runForget(cmd *cobra.Command, args []string) error {
	targetPath := args[0]
	targetPath = expandPath(targetPath)

	db, err := state.Open("")
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer db.Close()

	existing, err := db.GetFile(targetPath)
	if err != nil {
		return fmt.Errorf("checking file: %w", err)
	}

	if existing == nil {
		return fmt.Errorf("file not tracked: %s", targetPath)
	}

	if err := db.DeleteFile(targetPath); err != nil {
		return fmt.Errorf("removing from database: %w", err)
	}

	fmt.Printf("Forgot %s (target file unchanged)\n", targetPath)
	return nil
}
