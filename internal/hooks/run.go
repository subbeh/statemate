package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"

	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/template"
	"github.com/subbeh/statemate/internal/util"
)

// isInteractive is a variable so tests can stand in for a terminal.
var isInteractive = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// Options controls how triggered hooks are confirmed and run.
type Options struct {
	DryRun  bool
	Verbose bool
	// Force runs hooks without asking; NoScripts skips them silently. Both mirror
	// the flags that govern scripts.
	Force     bool
	NoScripts bool
	// ProfileChain filters script steps by their #profile: attribute.
	ProfileChain []string
}

// Runner confirms and runs triggered hooks. It shares the script executor so
// prompts, [a]ll, and script steps behave exactly as they do for scripts.
type Runner struct {
	exec    *scripts.Executor
	tmplCtx *template.Context
	opts    Options
}

func NewRunner(exec *scripts.Executor, tmplCtx *template.Context, opts Options) *Runner {
	return &Runner{exec: exec, tmplCtx: tmplCtx, opts: opts}
}

// Result reports what happened to the triggered hooks.
type Result struct {
	Ran int
	// Failed holds one error per hook that failed. A failure stops that hook's
	// remaining steps but not the other hooks.
	Failed []error
	// SkippedNoTTY lists hooks skipped because there was no terminal to confirm on.
	SkippedNoTTY []string
	// Declined lists hooks the user answered [n]o to.
	Declined []string
}

// Err summarises the failures, or returns nil when there were none.
func (r *Result) Err() error {
	if r == nil || len(r.Failed) == 0 {
		return nil
	}
	return fmt.Errorf("%d hook(s) failed", len(r.Failed))
}

// Run confirms and runs each triggered hook in order. It returns an error only
// when the user quits; hook failures are collected in the result.
func (r *Runner) Run(triggered []*Triggered) (*Result, error) {
	res := &Result{}
	if r.opts.NoScripts {
		return res, nil
	}

	for _, t := range triggered {
		title := fmt.Sprintf("hook %s (%s)", t.Name, FileCount(len(t.Files)))

		rendered, err := r.render(t)
		if err != nil {
			fmt.Printf("<<< failed:  hook %s (%v)\n", t.Name, err)
			res.Failed = append(res.Failed, fmt.Errorf("hook %s: %w", t.Name, err))
			continue
		}

		if r.opts.DryRun {
			fmt.Printf("  would run: %s\n", title)
			if t.Description != "" {
				fmt.Printf("      %s\n", t.Description)
			}
			if r.opts.Verbose {
				for _, f := range t.Files {
					fmt.Printf("      file: %s\n", util.ShortenPath(f))
				}
				for _, line := range stepLines(t, rendered) {
					fmt.Printf("      %s\n", line)
				}
			}
			res.Ran++
			continue
		}

		if !r.opts.Force && !isInteractive() {
			res.SkippedNoTTY = append(res.SkippedNoTTY, title)
			continue
		}

		var details []string
		if t.Description != "" {
			details = append(details, t.Description)
		}
		details = append(details, stepLines(t, rendered)...)
		ok, err := r.exec.Confirm(title, details)
		if err != nil {
			return res, err
		}
		if !ok {
			res.Declined = append(res.Declined, t.Name)
			continue
		}

		if err := r.runSteps(t, rendered); err != nil {
			res.Failed = append(res.Failed, fmt.Errorf("hook %s: %w", t.Name, err))
			continue
		}
		res.Ran++
	}
	return res, nil
}

// RunOne runs a hook directly, as 'mate hooks run' does: no prompt and no
// triggering files.
func (r *Runner) RunOne(h *Hook) error {
	t := &Triggered{Hook: h}
	rendered, err := r.render(t)
	if err != nil {
		return err
	}
	if r.opts.DryRun {
		fmt.Printf("would run: hook %s\n", h.Name)
		for _, line := range stepLines(t, rendered) {
			fmt.Printf("  %s\n", line)
		}
		return nil
	}
	return r.runSteps(t, rendered)
}

// render expands each run step's template, with .Files set to the triggering
// files. Script steps render nothing here and keep an empty entry.
func (r *Runner) render(t *Triggered) ([]string, error) {
	out := make([]string, len(t.steps))
	for i, s := range t.steps {
		if s.run == "" {
			continue
		}
		var ctx template.Context
		if r.tmplCtx != nil {
			ctx = *r.tmplCtx
		}
		ctx.Files = t.Files
		b, err := template.Render([]byte(s.run), &ctx)
		if err != nil {
			return nil, fmt.Errorf("step %d: rendering: %w", i+1, err)
		}
		out[i] = string(b)
	}
	return out, nil
}

func stepLines(t *Triggered, rendered []string) []string {
	lines := make([]string, len(t.steps))
	for i, s := range t.steps {
		if s.script != nil {
			lines[i] = "- script " + s.script.Name
		} else {
			lines[i] = "- " + rendered[i]
		}
	}
	return lines
}

func (r *Runner) runSteps(t *Triggered, rendered []string) error {
	env := []string{
		"STATEMATE_HOOK_NAME=" + t.Name,
		"STATEMATE_HOOK_FILES=" + strings.Join(t.Files, "\n"),
	}

	fmt.Printf(">>> running: hook %s\n", t.Name)
	for i, s := range t.steps {
		var err error
		if s.script != nil {
			if s.script.Profile != "" && !matchesChain(s.script.Profile, r.opts.ProfileChain) {
				fmt.Fprintf(os.Stderr, "Warning: hook %s: skipping script %s (profile %s is not active)\n",
					t.Name, s.script.Name, s.script.Profile)
				continue
			}
			// A script keeps its own STATEMATE_SOURCE_DIR: the source it lives in.
			err = r.exec.RunAsHook(s.script, env)
		} else {
			if r.opts.Verbose {
				fmt.Printf("  $ %s\n", rendered[i])
			}
			err = r.runCommand(t, rendered[i], env)
		}
		if err != nil {
			fmt.Printf("<<< failed:  hook %s (step %d: %v)\n", t.Name, i+1, err)
			return fmt.Errorf("step %d: %w", i+1, err)
		}
	}
	fmt.Printf("<<< done:    hook %s\n", t.Name)
	return nil
}

func (r *Runner) runCommand(t *Triggered, command string, env []string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = t.Dir()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	if t.SourceDir != "" {
		cmd.Env = append(cmd.Env, "STATEMATE_SOURCE_DIR="+t.SourceDir)
	}
	return cmd.Run()
}

func matchesChain(profile string, chain []string) bool {
	for _, p := range chain {
		if p == profile {
			return true
		}
	}
	return false
}

// FileCount formats a file count as "1 file" or "N files".
func FileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}
