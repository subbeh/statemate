package scripts

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/template"
)

func readShebang(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "sh"
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#!") {
			interp := strings.TrimSpace(line[2:])
			if strings.HasPrefix(interp, "/usr/bin/env ") {
				return strings.TrimSpace(interp[len("/usr/bin/env "):])
			}
			return interp
		}
	}
	return "sh"
}

// descriptionScanLines bounds how far into a script we look for the description
// comment, so listing scripts never requires reading whole files.
const descriptionScanLines = 10

var descriptionPattern = regexp.MustCompile(`(?i)^#\s*description:\s*(.*)$`)

// ReadDescription returns the script's "# Description: ..." comment, if any.
// Matching is case-insensitive with flexible whitespace and is limited to the
// first descriptionScanLines lines. The file is read raw -- templated scripts
// are not rendered, since a description is metadata about the script rather than
// part of its output.
func ReadDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for i := 0; i < descriptionScanLines && scanner.Scan(); i++ {
		matches := descriptionPattern.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
		if matches != nil {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

type Executor struct {
	db      *state.DB
	tmplCtx *template.Context
	dryRun  bool
	verbose bool

	// force auto-confirms every script; noScripts skips them all silently.
	force     bool
	noScripts bool
	// confirmAll is set once the user answers "all" at a prompt.
	confirmAll bool
	stdin      *bufio.Reader
	// changed names the sources with pending changes, used by #onchange scripts.
	changed ChangedSources
}

// WithChangedSources tells the executor which sources have pending changes, so
// #onchange scripts can be scheduled. Without it, no #onchange script runs.
func (e *Executor) WithChangedSources(changed ChangedSources) *Executor {
	e.changed = changed
	return e
}

func NewExecutor(db *state.DB, tmplCtx *template.Context, dryRun, verbose bool) *Executor {
	return &Executor{
		db:      db,
		tmplCtx: tmplCtx,
		dryRun:  dryRun,
		verbose: verbose,
		stdin:   bufio.NewReader(os.Stdin),
	}
}

// WithConfirmation configures how scripts are confirmed before running.
// force auto-confirms all scripts; noScripts skips them all.
func (e *Executor) WithConfirmation(force, noScripts bool) *Executor {
	e.force = force
	e.noScripts = noScripts
	return e
}

type ExecuteResult struct {
	Executed int
	Skipped  int
	Failed   int
	Errors   []error
	// SkippedNoTTY lists scripts that were due but skipped because there was no
	// terminal to prompt on. Callers surface this as a warning.
	SkippedNoTTY []string
}

// scriptAction is the outcome of asking whether a script should run.
type scriptAction int

const (
	actionRun scriptAction = iota
	actionSkip     // skip for now; ask again next apply
	actionSkipMark // skip and record as run, so it is never asked again
	actionAbort
)

// canMarkSkipped reports whether a script's run can be recorded without
// contradicting its frequency. An `always` script is explicitly opted into
// running on every apply, so there is no meaningful "never ask again" for it --
// recordRun does not persist that frequency either.
func canMarkSkipped(script *Script) bool {
	switch script.Frequency {
	case FreqOnce, FreqOnchange, FreqDaily, FreqWeekly, FreqMonthly:
		return true
	}
	return false
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// describe renders a script's identity for prompts and listings.
func describe(script *Script) string {
	return fmt.Sprintf("%s (%s, %s)", script.Name, script.Timing, script.Frequency)
}

// confirm asks whether a single script should run, honouring force/confirmAll.
func (e *Executor) confirm(script *Script) (scriptAction, error) {
	if e.force || e.confirmAll {
		return actionRun, nil
	}

	fmt.Printf("\nRun %s?\n", describe(script))
	if script.Description != "" {
		fmt.Printf("  %s\n", script.Description)
	}

	// [s]kip records the script as run without executing it, so it is not offered
	// again while that record stands. Meaningless for `always` scripts, whose runs
	// are never recorded, so the option is hidden for them.
	markable := canMarkSkipped(script)
	prompt := "[y]es / [n]o / [a]ll / [q]uit: "
	if markable {
		prompt = "[y]es / [n]o / [s]kip (mark as done) / [a]ll / [q]uit: "
	}

	for {
		fmt.Print(prompt)
		input, err := e.stdin.ReadString('\n')
		if err != nil {
			// EOF with no answer -- treat as abort rather than assuming consent.
			return actionAbort, nil
		}

		switch strings.TrimSpace(strings.ToLower(input)) {
		case "y", "yes":
			return actionRun, nil
		case "n", "no":
			return actionSkip, nil
		case "s", "skip":
			if markable {
				return actionSkipMark, nil
			}
		case "a", "all":
			e.confirmAll = true
			return actionRun, nil
		case "q", "quit":
			return actionAbort, nil
		}
	}
}

func (e *Executor) Execute(scripts Scripts) (*ExecuteResult, error) {
	result := &ExecuteResult{}

	// --no-scripts skips everything silently and records nothing, so the scripts
	// remain pending for a later interactive apply.
	if e.noScripts {
		result.Skipped = len(scripts)
		return result, nil
	}

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

		// Dry run reports what would happen without prompting -- there is nothing
		// to consent to when nothing executes.
		if e.dryRun {
			fmt.Printf("  would run: %s\n", describe(script))
			if script.Description != "" {
				fmt.Printf("      %s\n", script.Description)
			}
			result.Executed++
			continue
		}

		// Without a terminal we cannot ask, so skip rather than run unreviewed
		// code unattended. Callers warn about this.
		if !e.force && !isInteractive() {
			result.SkippedNoTTY = append(result.SkippedNoTTY, describe(script))
			result.Skipped++
			continue
		}

		action, err := e.confirm(script)
		if err != nil {
			return nil, err
		}
		switch action {
		case actionSkip:
			// Deliberately not recorded, so the script is offered again next apply.
			result.Skipped++
			continue
		case actionSkipMark:
			// Record without running, so it is treated as done and never offered
			// again. 'mate scripts list' still shows it, and 'mate scripts run'
			// can run it manually.
			if err := e.recordRun(script); err != nil {
				return nil, fmt.Errorf("recording skipped script: %w", err)
			}
			fmt.Printf("  marked as done without running: %s\n", script.Name)
			result.Skipped++
			continue
		case actionAbort:
			return result, fmt.Errorf("aborted by user")
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
	return ShouldRun(script, e.db, e.changed)
}

// ShouldRun reports whether a script is due.
//
// changed names the sources with pending changes and is only consulted for
// #onchange scripts; pass a zero ChangedSources when there are none.
func ShouldRun(script *Script, db *state.DB, changed ChangedSources) (bool, string, error) {
	switch script.Frequency {
	case FreqManual:
		return false, "manual only", nil

	case FreqOnce:
		hasRun, err := db.HasScriptRun(script.Path)
		if err != nil {
			return false, "", err
		}
		if hasRun {
			return false, "already run", nil
		}
		return true, "", nil

	case FreqOnchange:
		// Triggered by changes to the script's own source, not to the script
		// itself. A repo-root script has no owning source, so any change counts.
		if script.SourceDir == "" {
			if changed.Any() {
				return true, "", nil
			}
			return false, "no source changes", nil
		}
		if changed.Has(script.SourceDir) {
			return true, "", nil
		}
		return false, "no source changes", nil

	case FreqAlways:
		return true, "", nil

	case FreqDaily:
		return shouldRunInterval(script, db, 24*time.Hour)

	case FreqWeekly:
		return shouldRunInterval(script, db, 7*24*time.Hour)

	case FreqMonthly:
		return shouldRunInterval(script, db, 30*24*time.Hour)

	default:
		return false, "unknown frequency", nil
	}
}

func shouldRunInterval(script *Script, db *state.DB, interval time.Duration) (bool, string, error) {
	run, err := db.GetScriptRun(script.Path)
	if err != nil {
		return false, "", err
	}
	if run == nil {
		return true, "", nil
	}
	if time.Since(run.RunAt) >= interval {
		return true, "", nil
	}
	return false, fmt.Sprintf("last run %s ago", time.Since(run.RunAt).Round(time.Hour)), nil
}

// PendingScripts returns the scripts that are due.
//
// changed names the sources with pending changes, so #onchange scripts are
// reported consistently with what apply will actually run.
func PendingScripts(scripts Scripts, db *state.DB, changed ChangedSources) (Scripts, error) {
	var pending Scripts
	for _, script := range scripts {
		shouldRun, _, err := ShouldRun(script, db, changed)
		if err != nil {
			return nil, err
		}
		if shouldRun {
			pending = append(pending, script)
		}
	}
	return pending, nil
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
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.Write(rendered); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("writing temp file: %w", err)
		}
		_ = tmpFile.Close()

		if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
			return fmt.Errorf("setting temp file permissions: %w", err)
		}

		scriptPath = tmpFile.Name()
	}

	var cmd *exec.Cmd
	if script.IsExecutable() || scriptPath != script.Path {
		cmd = exec.Command(scriptPath)
	} else {
		interpreter := readShebang(scriptPath)
		cmd = exec.Command(interpreter, scriptPath)
	}
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

	fmt.Printf(">>> running: %s (%s/%s)\n", script.Name, script.Frequency, script.Timing)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("<<< failed:  %s (%v)\n", script.Name, err)
	} else {
		fmt.Printf("<<< done:    %s\n", script.Name)
	}
	return err
}

func (e *Executor) recordRun(script *Script) error {
	if e.dryRun {
		return nil
	}
	switch script.Frequency {
	case FreqOnce, FreqOnchange, FreqDaily, FreqWeekly, FreqMonthly:
		return e.db.RecordScriptRun(script.Path, script.ContentHash)
	}
	return nil
}
