package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/util"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <path>",
	Short: "Delete file from source and target",
	Long: `Delete a managed file from the source and the target, and stop tracking it.

Give the target path, meaning the deployed file, or the source file. It is a
path like any other: absolute, under ~, or relative to the current directory,
and a relative path that is not there is also looked up in the source
directory. The file may come from any source, not only the current profile's.
The target is deleted as well unless --keep-target is given, and the deletion
is confirmed unless --force (or --noconfirm) is.

Hooks matching the removed target run afterwards (see 'mate hooks'); --force
also confirms them. Nothing runs with --keep-target.`,
	Example: `  mate delete ~/.config/nvim/init.lua
  mate delete --keep-target ~/.zshrc`,
	Args:              cobra.ExactArgs(1),
	RunE:              runDelete,
	ValidArgsFunction: completeManagedFiles,
}

var (
	deleteForce      bool
	deleteKeepTarget bool
)

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "don't prompt for confirmation")
	deleteCmd.Flags().BoolVar(&deleteForce, "noconfirm", false, "same as --force")
	deleteCmd.Flags().BoolVar(&deleteKeepTarget, "keep-target", false, "keep the target file, only delete source")
}

func runDelete(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg, profileName, sourcePaths, scanner := env.Config, env.Profile, env.SourcePaths, env.Scanner

	entry, err := env.Resolve(args[0])
	if err != nil {
		return err
	}

	if !deleteForce {
		fmt.Println(bold("Will remove:"))
		fmt.Printf("  Source: %s\n", util.ShortenPath(entry.SourcePath))
		if !deleteKeepTarget {
			fmt.Printf("  Target: %s\n", util.ShortenPath(entry.TargetPath))
		}
		ok, err := util.Confirm("Continue? [y/N] ", false)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Aborted")
			return nil
		}
	}

	if err := os.Remove(entry.SourcePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing source: %w", err)
	}
	slog.Info("source", "action", "delete", "path", entry.SourcePath)
	fmt.Printf("Removed source: %s\n", util.ShortenPath(entry.SourcePath))

	if !deleteKeepTarget {
		if err := os.Remove(entry.TargetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing target: %w", err)
		}
		slog.Info("removed", "target", entry.TargetPath)
		fmt.Printf("Removed target: %s\n", util.ShortenPath(entry.TargetPath))
	}

	db, err := plan.OpenState(cfg)
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.DeleteFile(entry.TargetPath); err != nil {
		return fmt.Errorf("removing from database: %w", err)
	}

	if deleteKeepTarget {
		return nil
	}
	return runRemovalHooks(cfg, profileName, sourcePaths, scanner, db, hookChanges([]*source.Entry{entry}, sourcePaths), deleteForce)
}
