package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/template"
)

type Executor struct {
	db      *state.DB
	tmplCtx *template.Context
	dryRun  bool
	verbose bool
}

func NewExecutor(db *state.DB, tmplCtx *template.Context, dryRun, verbose bool) *Executor {
	return &Executor{
		db:      db,
		tmplCtx: tmplCtx,
		dryRun:  dryRun,
		verbose: verbose,
	}
}

type ExecuteResult struct {
	Executed int
	Skipped  int
	Failed   int
	Errors   []error
}

func (e *Executor) Execute(scripts Scripts) (*ExecuteResult, error) {
	result := &ExecuteResult{}

	for _, script := range scripts {
		shouldRun, reason, err := e.shouldRun(script)
		if err != nil {
			return nil, fmt.Errorf("checking script %s: %w", script.Name, err)
		}

		if !shouldRun {
			if e.verbose {
				fmt.Printf("  skip: %s (%s)\n", script.Name, reason)
			}
			result.Skipped++
			continue
		}

		if e.dryRun {
			fmt.Printf("  would run: %s\n", script.Path)
			result.Executed++
			continue
		}

		if e.verbose {
			fmt.Printf("  run: %s\n", script.Path)
		}

		if err := e.run(script); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", script.Name, err))
			result.Failed++
			return result, fmt.Errorf("script %s failed: %w", script.Name, err)
		}

		if err := e.recordRun(script); err != nil {
			return nil, fmt.Errorf("recording script run: %w", err)
		}
		result.Executed++
	}

	return result, nil
}

func (e *Executor) ExecuteOne(script *Script) error {
	if e.dryRun {
		fmt.Printf("would run: %s\n", script.Path)
		return nil
	}

	if e.verbose {
		fmt.Printf("run: %s\n", script.Path)
	}

	if err := e.run(script); err != nil {
		return err
	}

	return e.recordRun(script)
}

func (e *Executor) shouldRun(script *Script) (bool, string, error) {
	switch script.Frequency {
	case FreqManual:
		return false, "manual only", nil

	case FreqOnce:
		hasRun, err := e.db.HasScriptRun(script.Path)
		if err != nil {
			return false, "", err
		}
		if hasRun {
			return false, "already run", nil
		}
		return true, "", nil

	case FreqOnchange:
		hasRunWithHash, err := e.db.HasScriptRunWithHash(script.Path, script.ContentHash)
		if err != nil {
			return false, "", err
		}
		if hasRunWithHash {
			return false, "unchanged", nil
		}
		return true, "", nil

	case FreqAlways:
		return true, "", nil

	default:
		return false, "unknown frequency", nil
	}
}

func (e *Executor) run(script *Script) error {
	scriptPath := script.Path

	if script.Template {
		if e.tmplCtx == nil {
			return fmt.Errorf("template script %s requires template context", script.Name)
		}

		rendered, err := template.RenderFile(script.Path, e.tmplCtx)
		if err != nil {
			return fmt.Errorf("rendering template: %w", err)
		}

		tmpFile, err := os.CreateTemp("", "mate-script-*")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(rendered); err != nil {
			tmpFile.Close()
			return fmt.Errorf("writing temp file: %w", err)
		}
		tmpFile.Close()

		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			return fmt.Errorf("setting temp file permissions: %w", err)
		}

		scriptPath = tmpFile.Name()
	} else {
		if !script.IsExecutable() {
			return fmt.Errorf("script is not executable: %s", script.Path)
		}
	}

	cmd := exec.Command(scriptPath)
	cmd.Dir = filepath.Dir(script.Path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.Env = append(os.Environ(),
		"STATEMATE_SCRIPT="+script.Path,
		"STATEMATE_SCRIPT_NAME="+script.Name,
		"STATEMATE_SCRIPT_FREQUENCY="+script.Frequency.String(),
		"STATEMATE_SCRIPT_TIMING="+script.Timing.String(),
	)

	if script.SourceDir != "" {
		cmd.Env = append(cmd.Env, "STATEMATE_SOURCE_DIR="+script.SourceDir)
	}

	return cmd.Run()
}

func (e *Executor) recordRun(script *Script) error {
	if e.dryRun {
		return nil
	}
	if script.Frequency == FreqOnce || script.Frequency == FreqOnchange {
		return e.db.RecordScriptRun(script.Path, script.ContentHash)
	}
	return nil
}
