package scripts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseScriptName(t *testing.T) {
	tests := []struct {
		name        string
		want        Trigger
		wantOrder   int
		wantName    string
	}{
		{"run_once_10-setup.sh", TriggerOnce, 10, "setup"},
		{"run_onchange_20-update.sh", TriggerOnchange, 20, "update"},
		{"run_before_05-prepare.sh", TriggerBefore, 5, "prepare"},
		{"run_after_30-cleanup.sh", TriggerAfter, 30, "cleanup"},
		{"run_always_01-log.sh", TriggerAlways, 1, "log"},
		{"manual-script.sh", TriggerManual, 0, "manual-script"},
		{"script", TriggerManual, 0, "script"},
		{"run_once_100-long-name-here.zsh", TriggerOnce, 100, "long-name-here"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trigger, order, name := ParseScriptName(tc.name)
			if trigger != tc.want {
				t.Errorf("trigger = %v, want %v", trigger, tc.want)
			}
			if order != tc.wantOrder {
				t.Errorf("order = %v, want %v", order, tc.wantOrder)
			}
			if name != tc.wantName {
				t.Errorf("name = %v, want %v", name, tc.wantName)
			}
		})
	}
}

func TestTriggerString(t *testing.T) {
	tests := []struct {
		trigger Trigger
		want    string
	}{
		{TriggerOnce, "once"},
		{TriggerOnchange, "onchange"},
		{TriggerBefore, "before"},
		{TriggerAfter, "after"},
		{TriggerAlways, "always"},
		{TriggerManual, "manual"},
	}

	for _, tc := range tests {
		if got := tc.trigger.String(); got != tc.want {
			t.Errorf("%v.String() = %v, want %v", tc.trigger, got, tc.want)
		}
	}
}

func TestScriptsSort(t *testing.T) {
	scripts := Scripts{
		{Name: "z", Order: 10},
		{Name: "a", Order: 20},
		{Name: "b", Order: 10},
	}

	scripts.Sort()

	if scripts[0].Name != "b" || scripts[0].Order != 10 {
		t.Errorf("first should be b/10, got %s/%d", scripts[0].Name, scripts[0].Order)
	}
	if scripts[1].Name != "z" || scripts[1].Order != 10 {
		t.Errorf("second should be z/10, got %s/%d", scripts[1].Name, scripts[1].Order)
	}
	if scripts[2].Name != "a" || scripts[2].Order != 20 {
		t.Errorf("third should be a/20, got %s/%d", scripts[2].Name, scripts[2].Order)
	}
}

func TestScriptsByTrigger(t *testing.T) {
	scripts := Scripts{
		{Name: "a", Trigger: TriggerBefore},
		{Name: "b", Trigger: TriggerAfter},
		{Name: "c", Trigger: TriggerBefore},
		{Name: "d", Trigger: TriggerOnce},
	}

	before := scripts.ByTrigger(TriggerBefore)
	if len(before) != 2 {
		t.Errorf("expected 2 before scripts, got %d", len(before))
	}

	after := scripts.ByTrigger(TriggerAfter)
	if len(after) != 1 {
		t.Errorf("expected 1 after script, got %d", len(after))
	}

	always := scripts.ByTrigger(TriggerAlways)
	if len(always) != 0 {
		t.Errorf("expected 0 always scripts, got %d", len(always))
	}
}

func TestDiscoverer(t *testing.T) {
	tmpDir := t.TempDir()

	scriptsDir := filepath.Join(tmpDir, ".matescripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(scriptsDir, "run_once_10-test.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatal(err)
	}

	sourceDir := filepath.Join(tmpDir, "mysource")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	sourceScriptsDir := filepath.Join(sourceDir, ".matescripts")
	if err := os.MkdirAll(sourceScriptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	sourceScriptPath := filepath.Join(sourceScriptsDir, "run_after_20-cleanup.sh")
	if err := os.WriteFile(sourceScriptPath, []byte("#!/bin/bash\necho cleanup"), 0755); err != nil {
		t.Fatal(err)
	}

	discoverer := NewDiscoverer(tmpDir, []string{sourceDir})
	scripts, err := discoverer.Discover()
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if len(scripts) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(scripts))
	}

	if scripts[0].Order != 10 {
		t.Errorf("expected first script order 10, got %d", scripts[0].Order)
	}
	if scripts[1].Order != 20 {
		t.Errorf("expected second script order 20, got %d", scripts[1].Order)
	}
}

func TestScriptIsExecutable(t *testing.T) {
	tmpDir := t.TempDir()

	execPath := filepath.Join(tmpDir, "executable.sh")
	if err := os.WriteFile(execPath, []byte("#!/bin/bash"), 0755); err != nil {
		t.Fatal(err)
	}

	nonExecPath := filepath.Join(tmpDir, "nonexec.sh")
	if err := os.WriteFile(nonExecPath, []byte("#!/bin/bash"), 0644); err != nil {
		t.Fatal(err)
	}

	execScript := &Script{Path: execPath}
	if !execScript.IsExecutable() {
		t.Error("expected executable script to be executable")
	}

	nonExecScript := &Script{Path: nonExecPath}
	if nonExecScript.IsExecutable() {
		t.Error("expected non-executable script to not be executable")
	}
}
