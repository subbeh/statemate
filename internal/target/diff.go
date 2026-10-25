package target

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/template"
	"github.com/subbeh/statemate/internal/util"
)

func isPermissionDenied(err error) bool {
	return errors.Is(err, os.ErrPermission)
}

type Change struct {
	Entry   *source.Entry
	Status  ChangeStatus
	OldHash string
	NewHash string
	// PermOnly marks a modification that only fixes the mode: the content on
	// disk already matches.
	PermOnly bool
}

func desiredMode(entry *source.Entry) os.FileMode {
	if entry.Attrs.Perm != 0 {
		return os.FileMode(entry.Attrs.Perm)
	}
	return entry.Mode.Perm()
}

func permMismatch(entry *source.Entry, info os.FileInfo) bool {
	if entry.Attrs.Perm == 0 {
		return false
	}
	return info.Mode().Perm() != desiredMode(entry)
}

type ComputeOpts struct {
	TmplCtx *template.Context
	Enc     *encrypt.AgeEncryptor
	Sudo    bool // prompt for sudo upfront to cache credentials
}

type ComputeResult struct {
	Changes  []*Change
	Skipped  []string // paths skipped due to permission denied
}

func ComputeChanges(tree *source.Tree, db *state.DB, opts ...ComputeOpts) (*ComputeResult, error) {
	var o ComputeOpts
	if len(opts) > 0 {
		o = opts[0]
	}

	if o.Sudo {
		SudoPrompt()
	}

	result := &ComputeResult{}

	emptyDirs := tree.EmptyDirTargets()

	for _, dir := range tree.Dirs() {
		// A directory with files under it is covered by those files, and one with
		// no perm attribute has nothing to compare -- but an empty directory is
		// only ever created by apply, so its absence is a pending change.
		if dir.Attrs.Perm == 0 && !emptyDirs[dir.TargetPath] {
			continue
		}
		info, err := os.Lstat(dir.TargetPath)
		if err != nil {
			if isPermissionDenied(err) {
				info, err = sudoLstat(dir.TargetPath)
			}
			if err != nil {
				switch {
				case isPermissionDenied(err):
					result.Skipped = append(result.Skipped, dir.TargetPath)
				case os.IsNotExist(err) && emptyDirs[dir.TargetPath]:
					result.Changes = append(result.Changes, &Change{Entry: dir, Status: StatusNew})
				}
				continue
			}
		}
		if dir.Attrs.Perm != 0 && info.Mode().Perm() != os.FileMode(dir.Attrs.Perm) {
			result.Changes = append(result.Changes, &Change{Entry: dir, Status: StatusModified})
		}
	}

	for _, entry := range tree.Files() {
		change, err := computeChange(entry, db, &o)
		if err != nil {
			return nil, err
		}
		switch change.Status {
		case StatusUnchanged, StatusStateOnly:
			// skip
		case StatusSkipped:
			result.Skipped = append(result.Skipped, entry.TargetPath)
		default:
			result.Changes = append(result.Changes, change)
		}
	}

	return result, nil
}

// computeChange is classify, logged: apply, status, diff and check all go
// through it.
func computeChange(entry *source.Entry, db *state.DB, opts *ComputeOpts) (*Change, error) {
	change, err := classify(entry, db, opts)
	if err == nil {
		slog.Debug("file", "target", entry.TargetPath, "source", entry.SourcePath, "status", change.Status)
	}
	return change, err
}

func classify(entry *source.Entry, db *state.DB, opts *ComputeOpts) (*Change, error) {
	change := &Change{Entry: entry}

	// Importing a template would overwrite the template with its own rendered
	// output, destroying the source. The conflict prompt already refuses this;
	// as an attribute it is a mistake worth naming.
	if entry.Attrs.Import && entry.Attrs.Template {
		return nil, fmt.Errorf("%s: #import cannot be combined with #tmpl -- importing would overwrite the template with its rendered output", entry.SourcePath)
	}

	if entry.Attrs.Symlink {
		return computeSymlinkChange(entry, db)
	}

	var sourceHash string
	var err error
	if entry.Generated {
		sourceHash = state.HashBytes([]byte(entry.GeneratedContent))
	} else {
		sourceHash, err = state.HashFile(entry.SourcePath)
		if err != nil {
			return nil, err
		}
	}
	change.NewHash = sourceHash

	// The target is compared with what apply would write. A render that fails
	// fails the comparison too: falling back to the source's own hash reported a
	// status that apply then contradicted.
	renderedHash := sourceHash
	if !entry.Generated && (entry.Attrs.Encrypted || entry.Attrs.Template) {
		content, err := render(entry, opts.TmplCtx, opts.Enc)
		if err != nil {
			return nil, err
		}
		renderedHash = state.HashBytes(content)
	}

	existing, err := db.GetFile(entry.TargetPath)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		info, err := os.Lstat(entry.TargetPath)
		if os.IsNotExist(err) {
			change.Status = StatusNew
		} else if isPermissionDenied(err) {
			info, err = sudoLstat(entry.TargetPath)
			if err != nil {
				change.Status = StatusSkipped
				return change, nil
			}
			targetHash, err := sudoHashFile(entry.TargetPath)
			if err != nil {
				change.Status = StatusSkipped
				return change, nil
			}
			if targetHash != renderedHash {
				change.Status = StatusConflict
				change.OldHash = targetHash
			} else if permMismatch(entry, info) {
				change.Status = StatusModified
				change.PermOnly = true
			} else {
				change.Status = StatusStateOnly
			}
			return change, nil
		} else if err != nil {
			return nil, err
		} else {
			// If target is a symlink but source is not a symlink type, treat as conflict
			targetIsSymlink := info.Mode()&os.ModeSymlink != 0
			if targetIsSymlink && !entry.Attrs.Symlink {
				change.Status = StatusConflict
				return change, nil
			}

			targetHash, err := state.HashFile(entry.TargetPath)
			if err != nil {
				if isPermissionDenied(err) {
					targetHash, err = sudoHashFile(entry.TargetPath)
				}
				if err != nil {
					if isPermissionDenied(err) {
						change.Status = StatusSkipped
						return change, nil
					}
					return nil, err
				}
			}
			if targetHash != renderedHash {
				change.Status = StatusConflict
				change.OldHash = targetHash
			} else if permMismatch(entry, info) {
				change.Status = StatusModified
				change.PermOnly = true
			} else {
				change.Status = StatusStateOnly
			}
		}
		return change, nil
	}

	change.OldHash = existing.AppliedHash

	info, err := os.Lstat(entry.TargetPath)
	targetExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		if isPermissionDenied(err) {
			info, err = sudoLstat(entry.TargetPath)
			if err != nil {
				change.Status = StatusSkipped
				return change, nil
			}
			targetExists = true
		} else {
			return nil, err
		}
	}

	if !targetExists {
		change.Status = StatusModified
		return change, nil
	}

	// If target is a symlink but source is not a symlink type, treat as modified
	targetIsSymlink := info.Mode()&os.ModeSymlink != 0
	if targetIsSymlink && !entry.Attrs.Symlink {
		change.Status = StatusModified
		return change, nil
	}

	targetHash, err := state.HashFile(entry.TargetPath)
	if err != nil {
		if isPermissionDenied(err) {
			targetHash, err = sudoHashFile(entry.TargetPath)
		}
		if err != nil {
			if isPermissionDenied(err) {
				change.Status = StatusSkipped
				return change, nil
			}
			return nil, err
		}
	}

	if existing.SourceHash == sourceHash {
		if targetHash == existing.AppliedHash {
			if renderedHash != targetHash {
				change.Status = StatusModified
			} else if permMismatch(entry, info) {
				change.Status = StatusModified
				change.PermOnly = true
			} else {
				change.Status = StatusUnchanged
			}
		} else if entry.Attrs.Import {
			// The application owns this file and changed it, while the source
			// stood still. Take the target as truth rather than prompting.
			change.Status = StatusImport
		} else {
			change.Status = StatusConflict
		}
		return change, nil
	}

	if targetHash != existing.AppliedHash {
		// Both sides moved. Even for #import this is a real divergence, so ask
		// rather than silently discarding the source edit.
		change.Status = StatusConflict
	} else {
		change.Status = StatusModified
	}

	return change, nil
}

// computeSymlinkChange compares a #link entry by link text. What the link
// points at is irrelevant -- apply copies the link verbatim -- and following it
// fails outright for a directory or a destination that does not exist.
func computeSymlinkChange(entry *source.Entry, db *state.DB) (*Change, error) {
	change := &Change{Entry: entry}

	sourceHash, err := state.HashLink(entry.SourcePath)
	if err != nil {
		return nil, err
	}
	change.NewHash = sourceHash

	existing, err := db.GetFile(entry.TargetPath)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		change.OldHash = existing.AppliedHash
	}

	info, err := os.Lstat(entry.TargetPath)
	if os.IsNotExist(err) {
		if existing == nil {
			change.Status = StatusNew
		} else {
			change.Status = StatusModified
		}
		return change, nil
	}
	if isPermissionDenied(err) {
		change.Status = StatusSkipped
		return change, nil
	}
	if err != nil {
		return nil, err
	}

	// A regular file or directory in the link's place never matches.
	var targetHash string
	if info.Mode()&os.ModeSymlink != 0 {
		if targetHash, err = state.HashLink(entry.TargetPath); err != nil {
			return nil, err
		}
	}

	if targetHash == sourceHash {
		// The link already says the right thing, whatever the state DB holds --
		// including content hashes recorded before links were hashed this way.
		if existing != nil && existing.SourceHash == sourceHash && existing.AppliedHash == sourceHash {
			change.Status = StatusUnchanged
		} else {
			change.Status = StatusStateOnly
		}
		return change, nil
	}

	// Same rules as files: the source moved and the target is as last applied,
	// so deploy; anything else in the target's place is a conflict.
	if existing != nil && targetHash == existing.AppliedHash {
		change.Status = StatusModified
	} else {
		change.Status = StatusConflict
	}
	return change, nil
}

// Diff shows what applying a change would do: the target is the old side and
// what apply would write the new one. An #import change runs the other way, since
// the target is what will be written into the source. Without a tool it runs
// diff -u and colors the result; a tool's output is returned as is.
func Diff(change *Change, tmplCtx *template.Context, enc *encrypt.AgeEncryptor, tool string) (string, error) {
	entry := change.Entry
	if entry.Attrs.Symlink {
		// What changes is the link text; diffing what it points at would fail
		// for a directory or a destination that does not exist.
		return colorizeDiff(symlinkDiff(entry)), nil
	}

	rendered, err := render(entry, tmplCtx, enc)
	if err != nil {
		return "", err
	}
	targetLabel := entry.TargetPath
	current, err := readTarget(entry.TargetPath)
	if errors.Is(err, fs.ErrNotExist) {
		targetLabel = "/dev/null"
	} else if err != nil {
		return "", err
	}
	if isBinary(rendered) || isBinary(current) {
		return "Binary files differ", nil
	}

	oldLabel, oldContent, newLabel, newContent := targetLabel, current, entry.SourcePath, rendered
	if change.Status == StatusImport {
		oldLabel, oldContent, newLabel, newContent = newLabel, newContent, oldLabel, oldContent
	}

	// Named after the target, so a tool can tell the file type from the extension.
	name := filepath.Base(entry.TargetPath)
	oldPath, err := writeTemp(oldContent, name)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(oldPath) }()
	newPath, err := writeTemp(newContent, name)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(newPath) }()

	cmd := util.Command("diff", "-u", "--label", oldLabel, "--label", newLabel, oldPath, newPath)
	if tool != "" {
		cmd = util.UserCommand(tool, oldPath, newPath)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// diff exits 1 when the files differ, and tools disagree on what theirs means.
	_ = cmd.Run()

	if tool != "" {
		return out.String(), nil
	}
	return colorizeDiff(out.String()), nil
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

func writeTemp(content []byte, name string) (string, error) {
	f, err := os.CreateTemp("", "mate-*-"+name)
	if err != nil {
		return "", err
	}
	_, err = f.Write(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// isBinary looks for a NUL in the first 512 bytes, as diff itself does.
func isBinary(content []byte) bool {
	return bytes.IndexByte(content[:min(len(content), 512)], 0) >= 0
}

func colorizeDiff(diff string) string {
	if diff == "" {
		return ""
	}

	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	var result strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			result.WriteString(cyan(line))
		case strings.HasPrefix(line, "@@"):
			result.WriteString(cyan(line))
		case strings.HasPrefix(line, "+"):
			result.WriteString(green(line))
		case strings.HasPrefix(line, "-"):
			result.WriteString(red(line))
		default:
			result.WriteString(line)
		}
		result.WriteString("\n")
	}

	return strings.TrimSuffix(result.String(), "\n")
}

func errNoIdentity(entry *source.Entry) error {
	return fmt.Errorf("%s is #enc but no age identity is configured", entry.SourcePath)
}

// render returns the bytes an entry deploys to its target: generated content as
// is, and a source file decrypted when #enc and rendered when #tmpl. Apply writes
// them, classify hashes them and Diff compares them, so the three agree.
func render(entry *source.Entry, tmplCtx *template.Context, enc *encrypt.AgeEncryptor) ([]byte, error) {
	if entry.Generated {
		return []byte(entry.GeneratedContent), nil
	}
	content, err := os.ReadFile(entry.SourcePath)
	if err != nil {
		return nil, err
	}
	if entry.Attrs.Encrypted {
		// Never fall through to the ciphertext, which apply would deploy and
		// status would then report as up to date.
		if enc == nil || !enc.CanDecrypt() {
			return nil, errNoIdentity(entry)
		}
		if content, err = enc.Decrypt(content); err != nil {
			return nil, fmt.Errorf("decrypting %s: %w", entry.SourcePath, err)
		}
	}
	if entry.Attrs.Template {
		if tmplCtx == nil {
			return nil, fmt.Errorf("%s is #tmpl but there is no template context to render it with", entry.SourcePath)
		}
		if content, err = template.Render(content, tmplCtx); err != nil {
			return nil, fmt.Errorf("rendering %s: %w", entry.SourcePath, err)
		}
	}
	return content, nil
}
