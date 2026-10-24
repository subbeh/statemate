package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/hooks"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/util"
)

var renameCmd = &cobra.Command{
	Use:   "rename <source> <new-name>",
	Short: "Rename a managed file",
	Long: `Rename a managed file in both source and target.

This renames the source file, the target file, and updates tracking.

<source> names the file by its source, with or without the attributes, or by
its target. It is a path like any other: absolute, under ~, or relative to the
current directory, and a relative path that is not there is also looked up in
the source directory. A bare file name is not searched for.

Attributes belong to the source file only; the target is always named
without them. If <new-name> has no attributes, the source keeps the ones it
already has. If it has any, they replace the source's attributes, so give the
full set. #enc cannot be added or dropped this way, since that would
not change the content: use 'mate encrypt' or 'mate decrypt'.

Hooks matching the old or the new target path run afterwards, with a
confirmation prompt (see 'mate hooks').`,
	Example: `  mate rename nvim/.config/nvim/init.lua init.vim
  mate rename ~/.zshrc .zshrc.bak
  mate rename git/.gitconfig#tmpl .gitconfig.local
      (source becomes .gitconfig.local#tmpl, target .gitconfig.local)
  mate rename ssh/.ssh/config config#perm:600
      (changes only the attributes; the target keeps its name)`,
	Args:              cobra.ExactArgs(2),
	RunE:              runRename,
	ValidArgsFunction: completeFilePaths,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg := env.Config

	newName := args[1]
	entry, err := env.Resolve(args[0])
	if err != nil {
		return err
	}

	newSourceName, newTargetName, err := renamedNames(filepath.Base(entry.SourcePath), newName)
	if err != nil {
		return err
	}
	newSourcePath := filepath.Join(filepath.Dir(entry.SourcePath), newSourceName)
	newTargetPath := filepath.Join(filepath.Dir(entry.TargetPath), newTargetName)
	// Changing only the attributes leaves the target where it is.
	targetRenamed := newTargetPath != entry.TargetPath

	if _, err := os.Lstat(newSourcePath); err == nil {
		return fmt.Errorf("target already exists: %s", newSourcePath)
	}
	// Checked before anything moves, so a clash does not leave the source
	// renamed and the target not.
	if targetRenamed {
		if _, err := os.Lstat(newTargetPath); err == nil {
			return fmt.Errorf("target already exists: %s", util.ShortenPath(newTargetPath))
		}
	}

	if err := os.Rename(entry.SourcePath, newSourcePath); err != nil {
		return fmt.Errorf("renaming file: %w", err)
	}
	slog.Info("source", "action", "rename", "path", entry.SourcePath, "to", newSourcePath)

	db, err := plan.OpenState(cfg)
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer func() { _ = db.Close() }()

	targetMoved := false
	if _, err := os.Lstat(entry.TargetPath); err == nil && targetRenamed {
		if err := os.Rename(entry.TargetPath, newTargetPath); err != nil {
			return fmt.Errorf("renaming target: %w", err)
		}
		slog.Info("moved", "target", entry.TargetPath, "to", newTargetPath)
		targetMoved = true
	}

	existing, _ := db.GetFile(entry.TargetPath)
	if existing != nil {
		if err := db.DeleteFile(entry.TargetPath); err != nil {
			return fmt.Errorf("updating database: %w", err)
		}

		// A #link is tracked by its link text, as status compares it.
		hashFn := state.HashFile
		if entry.Attrs.Symlink {
			hashFn = state.HashLink
		}

		targetHash, err := hashFn(newTargetPath)
		if err != nil {
			return fmt.Errorf("hashing new target: %w", err)
		}

		sourceHash, err := hashFn(newSourcePath)
		if err != nil {
			return fmt.Errorf("hashing new source: %w", err)
		}

		if err := db.SaveFile(&state.FileEntry{
			SourcePath:  newSourcePath,
			TargetPath:  newTargetPath,
			SourceHash:  sourceHash,
			AppliedHash: targetHash,
			Mode:        existing.Mode,
		}); err != nil {
			return fmt.Errorf("saving new tracking entry: %w", err)
		}
	}

	fmt.Println(bold("Renamed:"))
	fmt.Printf("  source: %s -> %s\n", util.ShortenPath(entry.SourcePath), util.ShortenPath(newSourcePath))
	if targetRenamed {
		fmt.Printf("  target: %s -> %s\n", util.ShortenPath(entry.TargetPath), util.ShortenPath(newTargetPath))
	} else {
		fmt.Printf("  target: %s (unchanged)\n", util.ShortenPath(entry.TargetPath))
	}

	if !targetMoved {
		return nil
	}

	// The old target is gone and the new one is written; both can trigger
	// hooks. Only sources of the active profile contribute source hooks.
	srcDir := hooks.OwningSource(entry.SourcePath, env.SourcePaths)
	changes := []hooks.Change{
		{Path: entry.TargetPath, SourceDir: srcDir},
		{Path: newTargetPath, SourceDir: srcDir},
	}
	return runRemovalHooks(cfg, env.Profile, env.SourcePaths, env.Scanner, db, changes, false)
}

// renamedNames works out the new source and target file names for renaming the
// source file sourceName to newName. Attributes belong to the source only, so
// the target is always the attribute-free name. A newName without attributes
// keeps the source's existing ones, so a plain rename does not silently turn
// a template or encrypted file into an ordinary one; a newName with attributes
// replaces them.
//
// #enc describes the content rather than how it is deployed, so it can
// only be changed by mate encrypt and mate decrypt, which convert the content
// to match.
func renamedNames(sourceName, newName string) (newSourceName, newTargetName string, err error) {
	newBase, newAttrs := source.ParseAttrs(newName)
	_, oldAttrs := source.ParseAttrs(sourceName)
	if newBase == "" {
		return "", "", fmt.Errorf("new name %q has no file name before its attributes", newName)
	}

	newSourceName = newName
	if !strings.Contains(newName, "#") {
		if i := strings.Index(sourceName, "#"); i != -1 {
			newSourceName = newName + sourceName[i:]
		}
		newAttrs = oldAttrs
	}

	if newAttrs.Encrypted != oldAttrs.Encrypted {
		if oldAttrs.Encrypted {
			return "", "", fmt.Errorf("%s drops #enc from %s: use 'mate decrypt' to decrypt it", newName, sourceName)
		}
		return "", "", fmt.Errorf("%s adds #enc to %s: use 'mate encrypt' to encrypt it", newName, sourceName)
	}

	return newSourceName, newBase, nil
}
