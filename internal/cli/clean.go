package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/hooks"
	"github.com/subbeh/statemate/internal/target"
	"github.com/subbeh/statemate/internal/util"
)

var cleanCmd = &cobra.Command{
	Use:   "clean [path...]",
	Short: "Remove orphaned files",
	Long: `Remove orphaned files: targets mate deployed earlier that no longer come
from any active source, usually because the file was deleted from the
repository or its source was dropped from 'sources:'.

With no arguments, the orphans are listed and nothing is removed. Name orphans
to remove them, or pass --all for every one. Each removal is confirmed unless
--force (or --noconfirm) is given. Files you cannot write to are removed with sudo.

Hooks matching the removed files run afterwards (see 'mate hooks'); --force
also confirms them.

To stop tracking a file without deleting it, use 'mate forget'.`,
	Example: `  mate clean                              # list orphans
  mate clean ~/.config/old/file.conf      # remove one orphan
  mate clean --all                        # remove every orphan, asking for each
  mate clean --all --force                # remove every orphan without asking`,
	RunE:              runClean,
	ValidArgsFunction: completeOrphanedFiles,
}

func init() {
	cleanCmd.Flags().Bool("force", false, "skip confirmation prompts")
	cleanCmd.Flags().Bool("noconfirm", false, "same as --force")
	cleanCmd.Flags().Bool("all", false, "remove all orphans")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	sourcePaths := env.SourcePaths

	p, err := env.Plan()
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()

	db, err := p.DB()
	if err != nil {
		return err
	}

	orphans, err := p.Orphans()
	if err != nil {
		return err
	}

	if len(orphans) == 0 {
		fmt.Println(color.GreenString("No orphaned files"))
		return nil
	}

	force, _ := cmd.Flags().GetBool("force")
	noConfirm, _ := cmd.Flags().GetBool("noconfirm")
	force = force || noConfirm
	all, _ := cmd.Flags().GetBool("all")

	if len(args) == 0 && !all {
		fmt.Println(bold("Orphaned files:"))
		for _, o := range orphans {
			fmt.Printf("  %s\n", util.ShortenPath(o))
		}
		fmt.Println("\nUse 'mate clean <path>' to remove specific files, or 'mate clean --all' to remove all.")
		return nil
	}

	var toRemove []string
	if all {
		toRemove = orphans
	} else {
		orphanSet := make(map[string]bool)
		for _, o := range orphans {
			orphanSet[o] = true
		}
		for _, arg := range args {
			path := expandPath(arg)
			if !orphanSet[path] {
				return fmt.Errorf("not an orphan: %s", path)
			}
			toRemove = append(toRemove, path)
		}
	}

	var removed []hooks.Change
	for _, path := range toRemove {
		if !force {
			ok, err := util.Confirm(fmt.Sprintf("Remove %s? [y/N] ", util.ShortenPath(path)), false)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("  Skipped")
				continue
			}
		}

		if err := os.Remove(path); err != nil {
			if os.IsPermission(err) {
				if err := target.SudoRemove(path); err != nil {
					return fmt.Errorf("removing %s: %w", path, err)
				}
			} else {
				return fmt.Errorf("removing %s: %w", path, err)
			}
		}

		// Look up the source before the record goes, so the orphan still
		// triggers hooks declared in the source it came from.
		change := hooks.Change{Path: path}
		if fe, _ := db.GetFile(path); fe != nil {
			change.SourceDir = hooks.OwningSource(fe.SourcePath, sourcePaths)
		}

		if err := db.DeleteFile(path); err != nil {
			return fmt.Errorf("removing %s from database: %w", path, err)
		}

		slog.Info("removed", "target", path)
		fmt.Printf("Removed %s\n", util.ShortenPath(path))
		removed = append(removed, change)
	}

	return runRemovalHooks(env, db, removed, force)
}
