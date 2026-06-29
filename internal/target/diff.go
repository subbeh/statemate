package target

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
)

type Change struct {
	Entry   *source.Entry
	Status  ChangeStatus
	OldHash string
	NewHash string
}

func ComputeChanges(tree *source.Tree, db *state.DB) ([]*Change, error) {
	var changes []*Change

	for _, entry := range tree.Files() {
		change, err := computeChange(entry, db)
		if err != nil {
			return nil, err
		}
		if change.Status != StatusUnchanged {
			changes = append(changes, change)
		}
	}

	return changes, nil
}

func computeChange(entry *source.Entry, db *state.DB) (*Change, error) {
	change := &Change{Entry: entry}

	sourceHash, err := state.HashFile(entry.SourcePath)
	if err != nil {
		return nil, err
	}
	change.NewHash = sourceHash

	existing, err := db.GetFile(entry.TargetPath)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		if _, err := os.Stat(entry.TargetPath); os.IsNotExist(err) {
			change.Status = StatusNew
		} else {
			targetHash, err := state.HashFile(entry.TargetPath)
			if err != nil {
				return nil, err
			}
			if targetHash != sourceHash {
				change.Status = StatusConflict
				change.OldHash = targetHash
			} else {
				change.Status = StatusUnchanged
			}
		}
		return change, nil
	}

	change.OldHash = existing.AppliedHash

	targetExists := true
	if _, err := os.Stat(entry.TargetPath); os.IsNotExist(err) {
		targetExists = false
	}

	if !targetExists {
		change.Status = StatusModified
		return change, nil
	}

	targetHash, err := state.HashFile(entry.TargetPath)
	if err != nil {
		return nil, err
	}

	if existing.SourceHash == sourceHash {
		if targetHash == existing.AppliedHash {
			change.Status = StatusUnchanged
		} else {
			change.Status = StatusConflict
		}
		return change, nil
	}

	if targetHash != existing.AppliedHash {
		change.Status = StatusConflict
	} else {
		change.Status = StatusModified
	}

	return change, nil
}

func showDiff(sourcePath, targetPath string) error {
	cmd := exec.Command("diff", "-u", targetPath, sourcePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	cmd.Run()

	fmt.Println(ColorizeDiff(out.String()))
	return nil
}

func ColorizeDiff(diff string) string {
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

func GenerateDiff(sourcePath, targetPath string) (string, error) {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		content, err := os.ReadFile(sourcePath)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("+++ %s (new file)\n%s", targetPath, content), nil
	}

	cmd := exec.Command("diff", "-u", targetPath, sourcePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	cmd.Run()

	return out.String(), nil
}

func IsBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}
