package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/target"
	"github.com/subbeh/statemate/internal/template"
	"github.com/subbeh/statemate/internal/util"
)

var diffCmd = &cobra.Command{
	Use:               "diff [path]",
	Short:             "Show pending changes",
	Long: `Show full unified diff of pending changes.

The positional argument filters by file or path; use --source to limit the diff
to a single source.

Use --tool to specify an external diff tool (e.g., delta, difft, vimdiff), or
set 'diff_tool' in config. The tool is run as '<tool> <old> <new>' and its
output is captured, so it may include arguments but cannot be interactive.`,
	Args:              cobra.MaximumNArgs(1),
	RunE:              runDiff,
	ValidArgsFunction: completePendingFiles,
}

func init() {
	diffCmd.Flags().StringP("tool", "t", "", "external diff tool to use")
	diffCmd.Flags().Bool("sudo", false, "use sudo to check files requiring elevated access")
	addScopeFlag(diffCmd)
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	cfg, profileName, enc, tmplCtx := env.Config, env.Profile, env.Enc, env.TmplCtx

	p, err := env.Plan()
	if err != nil {
		return err
	}
	defer func() { _ = p.Close() }()
	tree := p.Tree

	if tree.HasConflicts() {
		printConflicts(tree, color.RedString("Error:"))
		return fmt.Errorf("resolve conflicts before diffing")
	}

	useSudo, _ := cmd.Flags().GetBool("sudo")
	result, err := p.Changes(useSudo)
	if err != nil {
		return err
	}
	changes := result.Changes

	scope, err := scopeFrom(cmd, args)
	if err != nil {
		return err
	}
	if err := scope.validate(cfg, profileName, tree.Files()); err != nil {
		return err
	}

	if !scope.IsZero() {
		var filtered []*target.Change
		for _, c := range changes {
			if scope.Matches(c.Entry, cfg.SourceDir()) {
				filtered = append(filtered, c)
			}
		}
		changes = filtered
	}

	if len(changes) == 0 {
		if len(result.Skipped) > 0 {
			fmt.Fprintf(os.Stderr, "%s %d file(s) skipped (permission denied, use --sudo to check)\n", color.YellowString("Warning:"), len(result.Skipped))
		}
		fmt.Println(color.GreenString("No changes"))
		return nil
	}

	diffTool, _ := cmd.Flags().GetString("tool")
	if diffTool == "" {
		diffTool = cfg.DiffTool
	}

	for _, change := range changes {
		if change.Entry.IsDir {
			// Directories have no content to compare; status reports them.
			fmt.Printf("%s\n(directory)\n", bold("=== "+util.ShortenPath(change.Entry.TargetPath)+" ==="))
			continue
		}

		if change.Entry.Attrs.Symlink {
			// What changes is the link text; diffing what it points at would fail
			// for a directory or a destination that does not exist.
			fmt.Println(bold("=== " + util.ShortenPath(change.Entry.TargetPath) + " ==="))
			fmt.Println(target.ColorizeDiff(symlinkDiff(change.Entry)))
			continue
		}

		if !change.Entry.Generated && target.IsBinaryFile(change.Entry.SourcePath) {
			fmt.Printf("Binary files differ: %s\n", util.ShortenPath(change.Entry.TargetPath))
			continue
		}

		var diff string
		if change.Status == target.StatusImport {
			// The target is about to be written into the source, so show the diff
			// that way round -- source as old, target as new.
			diff, err = generateImportDiff(change.Entry, enc, diffTool)
		} else if change.Entry.Generated {
			diff, err = generateGeneratedDiff(change.Entry, diffTool)
		} else if change.Entry.Attrs.Encrypted && change.Entry.Attrs.Template && enc != nil && enc.CanDecrypt() {
			diff, err = generateEncryptedTemplateDiff(change.Entry, enc, tmplCtx, diffTool)
		} else if change.Entry.Attrs.Encrypted && enc != nil && enc.CanDecrypt() {
			diff, err = generateDecryptedDiff(change.Entry, enc, diffTool)
		} else if change.Entry.Attrs.Template {
			diff, err = generateTemplatedDiff(change.Entry, tmplCtx, diffTool)
		} else {
			diff, err = target.GenerateDiffWithTool(change.Entry.SourcePath, change.Entry.TargetPath, diffTool)
		}
		fmt.Println(bold("=== " + util.ShortenPath(change.Entry.TargetPath) + " ==="))
		if err != nil {
			if errors.As(err, new(*secrets.NotCachedError)) {
				fmt.Fprintf(os.Stderr, "  (secrets not cached, run 'mate secrets fetch')\n")
			} else {
				return fmt.Errorf("generating diff for %s: %w", util.ShortenPath(change.Entry.TargetPath), err)
			}
		} else if diff != "" {
			if diffTool == "" {
				fmt.Println(target.ColorizeDiff(diff))
			} else {
				fmt.Println(diff)
			}
		} else {
			fmt.Println("(no content difference, state mismatch only)")
		}
	}

	return nil
}

// symlinkDiff describes a pending #link change as the link the target has
// now and the one apply will create.
func symlinkDiff(entry *source.Entry) string {
	var lines []string
	if old, err := os.Readlink(entry.TargetPath); err == nil {
		lines = append(lines, "- -> "+old)
	} else if _, err := os.Lstat(entry.TargetPath); err == nil {
		lines = append(lines, "- (not a symlink)")
	}
	if dest, err := os.Readlink(entry.SourcePath); err == nil {
		lines = append(lines, "+ -> "+dest)
	}
	return strings.Join(lines, "\n")
}

func generateEncryptedTemplateDiff(entry *source.Entry, enc *encrypt.AgeEncryptor, tmplCtx *template.Context, diffTool string) (string, error) {
	ciphertext, err := os.ReadFile(entry.SourcePath)
	if err != nil {
		return "", err
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypting source: %w", err)
	}

	rendered, err := template.Render(plaintext, tmplCtx)
	if err != nil {
		return "", fmt.Errorf("rendering template: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "mate-diff-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.Write(rendered); err != nil {
		return "", err
	}
	_ = tmpFile.Close()

	return target.GenerateDiffWithTool(tmpFile.Name(), entry.TargetPath, diffTool)
}

// generateImportDiff shows what importing would change in the source: the source
// is the old side and the target the new one, the reverse of a normal diff.
//
// An encrypted source is decrypted first, so the diff compares plaintext rather
// than reporting that two ciphertexts differ.
func generateImportDiff(entry *source.Entry, enc *encrypt.AgeEncryptor, diffTool string) (string, error) {
	oldPath := entry.SourcePath

	if entry.Attrs.Encrypted {
		if enc == nil || !enc.CanDecrypt() {
			return "", fmt.Errorf("cannot show diff: source is encrypted and no decryption key is available")
		}
		ciphertext, err := os.ReadFile(entry.SourcePath)
		if err != nil {
			return "", err
		}
		plaintext, err := enc.Decrypt(ciphertext)
		if err != nil {
			return "", fmt.Errorf("decrypting source: %w", err)
		}

		tmpFile, err := os.CreateTemp("", "mate-diff-*")
		if err != nil {
			return "", err
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()
		if _, err := tmpFile.Write(plaintext); err != nil {
			_ = tmpFile.Close()
			return "", err
		}
		_ = tmpFile.Close()
		oldPath = tmpFile.Name()
	}

	return target.GenerateDiffBetween(oldPath, entry.TargetPath, diffTool)
}

func generateDecryptedDiff(entry *source.Entry, enc *encrypt.AgeEncryptor, diffTool string) (string, error) {
	ciphertext, err := os.ReadFile(entry.SourcePath)
	if err != nil {
		return "", err
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypting source: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "mate-diff-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.Write(plaintext); err != nil {
		return "", err
	}
	_ = tmpFile.Close()

	return target.GenerateDiffWithTool(tmpFile.Name(), entry.TargetPath, diffTool)
}

func generateTemplatedDiff(entry *source.Entry, tmplCtx *template.Context, diffTool string) (string, error) {
	rendered, err := template.RenderFile(entry.SourcePath, tmplCtx)
	if err != nil {
		return "", fmt.Errorf("rendering template: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "mate-diff-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.Write(rendered); err != nil {
		return "", err
	}
	_ = tmpFile.Close()

	return target.GenerateDiffWithTool(tmpFile.Name(), entry.TargetPath, diffTool)
}

func generateGeneratedDiff(entry *source.Entry, diffTool string) (string, error) {
	tmpFile, err := os.CreateTemp("", "mate-diff-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.WriteString(entry.GeneratedContent); err != nil {
		return "", err
	}
	_ = tmpFile.Close()

	return target.GenerateDiffWithTool(tmpFile.Name(), entry.TargetPath, diffTool)
}
