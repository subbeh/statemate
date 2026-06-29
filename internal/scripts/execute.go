package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/subbeh/statemate/internal/state"
)

type Executor struct {
	db      *state.DB
	dryRun  bool
	verbose bool
}

func NewExecutor(db *state.DB, dryRun, verbose bool) *Executor {
	return &Executor{
		db:      db,
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
	switch script.Trigger {
	case TriggerManual:
		return false, "manual only", nil

	case TriggerOnce:
		hasRun, err := e.db.HasScriptRun(script.Path)
		if err != nil {
			return false, "", err
		}
		if hasRun {
			return false, "already run", nil
		}
		return true, "", nil

	case TriggerOnchange:
		hasRunWithHash, err := e.db.HasScriptRunWithHash(script.Path, script.ContentHash)
		if err != nil {
			return false, "", err
		}
		if hasRunWithHash {
			return false, "unchanged", nil
		}
		return true, "", nil

	case TriggerBefore, TriggerAfter, TriggerAlways:
		return true, "", nil

	default:
		return false, "unknown trigger", nil
	}
}

func (e *Executor) run(script *Script) error {
	if !script.IsExecutable() {
		return fmt.Errorf("script is not executable: %s", script.Path)
	}

	cmd := exec.Command(script.Path)
	cmd.Dir = filepath.Dir(script.Path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.Env = append(os.Environ(),
		"STATEMATE_SCRIPT="+script.Path,
		"STATEMATE_SCRIPT_NAME="+script.Name,
		"STATEMATE_SCRIPT_TRIGGER="+script.Trigger.String(),
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
	if script.Trigger == TriggerOnce || script.Trigger == TriggerOnchange {
		return e.db.RecordScriptRun(script.Path, script.ContentHash)
	}
	return nil
}
