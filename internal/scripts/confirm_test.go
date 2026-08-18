package scripts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/state"
)

func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadDescription(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "canonical form",
			content: "#!/usr/bin/env bash\n# Description: Bootstrap the dev environment\n",
			want:    "Bootstrap the dev environment",
		},
		{
			name:    "lowercase and no spaces",
			content: "#!/bin/sh\n#description:tight\n",
			want:    "tight",
		},
		{
			name:    "extra whitespace",
			content: "#!/bin/sh\n#   Description:    padded out   \n",
			want:    "padded out",
		},
		{
			name:    "no shebang",
			content: "# Description: still found\n",
			want:    "still found",
		},
		{
			name:    "no description present",
			content: "#!/bin/sh\n# just a normal comment\necho hi\n",
			want:    "",
		},
		{
			name:    "shebang alone is not a description",
			content: "#!/usr/bin/env bash\n",
			want:    "",
		},
		{
			name:    "beyond scan limit is ignored",
			content: "#!/bin/sh\n\n\n\n\n\n\n\n\n\n\n# Description: too far down\n",
			want:    "",
		},
		{
			name:    "empty description value",
			content: "#!/bin/sh\n# Description:\n",
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeScript(t, dir, "s-"+tc.name+".sh", tc.content)
			if got := ReadDescription(path); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadDescription_MissingFile(t *testing.T) {
	if got := ReadDescription(filepath.Join(t.TempDir(), "nope.sh")); got != "" {
		t.Errorf("expected empty description for missing file, got %q", got)
	}
}

func TestDiscoverPopulatesDescription(t *testing.T) {
	repo := t.TempDir()
	scriptsDir := filepath.Join(repo, ScriptsDir)
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, scriptsDir, "01-setup#once#before.sh",
		"#!/usr/bin/env bash\n# Description: Set things up\n")

	found, err := NewDiscoverer(repo, nil).Discover()
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 script, got %d", len(found))
	}
	if found[0].Description != "Set things up" {
		t.Errorf("description not populated during discovery: got %q", found[0].Description)
	}
}

// newTestExecutor builds an executor over a temp state DB.
func newTestExecutor(t *testing.T, dryRun bool) *Executor {
	t.Helper()
	db, err := state.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewExecutor(db, nil, dryRun, false)
}

func TestCanMarkSkipped(t *testing.T) {
	tests := []struct {
		freq Frequency
		want bool
	}{
		{FreqOnce, true},
		{FreqOnchange, true},
		{FreqDaily, true},
		{FreqWeekly, true},
		{FreqMonthly, true},
		// `always` runs are never recorded, so there is nothing to mark.
		{FreqAlways, false},
		{FreqManual, false},
	}

	for _, tc := range tests {
		got := canMarkSkipped(&Script{Frequency: tc.freq})
		if got != tc.want {
			t.Errorf("%v: got %v, want %v", tc.freq, got, tc.want)
		}
	}
}

// canMarkSkipped must agree with recordRun: the option is only offered for
// frequencies whose runs are actually persisted, otherwise "mark as done" would
// silently do nothing.
func TestCanMarkSkippedMatchesRecordRun(t *testing.T) {
	dir := t.TempDir()
	path := writeScript(t, dir, "01-x.sh#once#before", "#!/bin/bash\ntrue\n")

	for _, freq := range []Frequency{FreqOnce, FreqOnchange, FreqDaily, FreqWeekly, FreqMonthly, FreqAlways, FreqManual} {
		executor := newTestExecutor(t, false)
		script := &Script{Path: path, Name: "x", Frequency: freq, ContentHash: "h"}

		if err := executor.recordRun(script); err != nil {
			t.Fatalf("%v: recordRun failed: %v", freq, err)
		}

		hasRun, err := executor.db.HasScriptRun(path)
		if err != nil {
			t.Fatal(err)
		}
		if hasRun != canMarkSkipped(script) {
			t.Errorf("%v: recordRun persisted=%v but canMarkSkipped=%v", freq, hasRun, canMarkSkipped(script))
		}
	}
}

// Marking a script as done must record it without executing it.
func TestSkipMarkRecordsWithoutRunning(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	path := writeScript(t, dir, "01-x.sh#once#before", "#!/bin/bash\ntouch "+marker+"\n")

	executor := newTestExecutor(t, false)
	script := &Script{Path: path, Name: "x", Frequency: FreqOnce, Timing: TimingBefore, ContentHash: "h"}

	if err := executor.recordRun(script); err != nil {
		t.Fatalf("recordRun failed: %v", err)
	}

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("marking as done must not execute the script")
	}

	// With a run recorded, the script is no longer due.
	shouldRun, reason, err := ShouldRun(script, executor.db, ChangedSources{})
	if err != nil {
		t.Fatal(err)
	}
	if shouldRun {
		t.Error("expected a marked script to no longer be due")
	}
	if reason != "already run" {
		t.Errorf("unexpected reason: %q", reason)
	}
}

func TestExecute_NoScriptsSkipsEverything(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	path := writeScript(t, dir, "01-x#always#before.sh", "#!/bin/bash\ntouch "+marker+"\n")

	script := &Script{Path: path, Name: "x", Frequency: FreqAlways, Timing: TimingBefore}

	executor := newTestExecutor(t, false).WithConfirmation(false, true)
	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.Executed != 0 {
		t.Errorf("expected nothing executed with --no-scripts, got %d", result.Executed)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("script must not run with --no-scripts")
	}
	if len(result.SkippedNoTTY) != 0 {
		t.Error("--no-scripts is explicit, so it should not warn about a missing TTY")
	}
}

// Tests run without a TTY, so this exercises the non-interactive path.
func TestExecute_NoTTYSkipsAndReports(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	path := writeScript(t, dir, "01-x#always#before.sh", "#!/bin/bash\ntouch "+marker+"\n")

	script := &Script{
		Path:        path,
		Name:        "x",
		Frequency:   FreqAlways,
		Timing:      TimingBefore,
		Description: "does a thing",
	}

	executor := newTestExecutor(t, false).WithConfirmation(false, false)
	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.Executed != 0 {
		t.Errorf("expected nothing executed without a TTY, got %d", result.Executed)
	}
	if len(result.SkippedNoTTY) != 1 {
		t.Fatalf("expected 1 script reported as skipped for no TTY, got %d", len(result.SkippedNoTTY))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("script must not run when it could not be confirmed")
	}
}

func TestExecute_ForceRunsWithoutPrompting(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	path := writeScript(t, dir, "01-x#always#before.sh", "#!/bin/bash\ntouch "+marker+"\n")

	script := &Script{Path: path, Name: "x", Frequency: FreqAlways, Timing: TimingBefore}

	executor := newTestExecutor(t, false).WithConfirmation(true, false)
	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if result.Executed != 1 {
		t.Errorf("expected 1 executed with --force, got %d", result.Executed)
	}
	if len(result.SkippedNoTTY) != 0 {
		t.Error("--force should not report TTY skips")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("script should have run with --force")
	}
}

// Dry run must neither prompt nor be blocked by the missing-TTY guard.
func TestExecute_DryRunDoesNotPrompt(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	path := writeScript(t, dir, "01-x#always#before.sh", "#!/bin/bash\ntouch "+marker+"\n")

	script := &Script{Path: path, Name: "x", Frequency: FreqAlways, Timing: TimingBefore}

	executor := newTestExecutor(t, true).WithConfirmation(false, false)
	result, err := executor.Execute(Scripts{script})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if result.Executed != 1 {
		t.Errorf("expected 1 would-run in dry run, got %d", result.Executed)
	}
	if len(result.SkippedNoTTY) != 0 {
		t.Error("dry run should not report TTY skips")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("dry run must not execute the script")
	}
}
