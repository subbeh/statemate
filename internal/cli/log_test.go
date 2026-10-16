package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runRoot(t *testing.T, args ...string) error {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		// Flag values outlive the run, and so does the logger.
		_ = rootCmd.PersistentFlags().Set("log-file", "")
		_ = rootCmd.PersistentFlags().Set("log-level", "")
		slog.SetDefault(slog.New(slog.DiscardHandler))
	})
	return rootCmd.Execute()
}

// log_file and log_level in mate.yaml turn logging on. Each run is appended,
// with its decisions at debug and the commands it ran.
func TestLogFileFromConfig(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "logs", "mate.log")
	writeFile(t, filepath.Join(dir, "mate.yaml"), "log_file: "+logPath+"\nlog_level: debug\nprofiles:\n  box:\n    detection:\n      command: \"true\"\n", 0644)
	t.Setenv("STATEMATE_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	for range 2 {
		if err := runRoot(t, "profile"); err != nil {
			t.Fatalf("mate profile: %v", err)
		}
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	if n := strings.Count(log, "msg=run"); n != 2 {
		t.Errorf("log has %d runs, want 2 (appended):\n%s", n, log)
	}
	for _, want := range []string{`msg="config loaded"`, "msg=profile name=box", `msg=exec args="[sh -c true]"`, "exit=0"} {
		if !strings.Contains(log, want) {
			t.Errorf("log does not contain %s:\n%s", want, log)
		}
	}
	if info, _ := os.Stat(logPath); info.Mode().Perm() != 0600 {
		t.Errorf("log mode = %o, want 600", info.Mode().Perm())
	}
}

// The flag wins over the config, so a run can turn the level down.
func TestLogLevelFlagOverridesConfig(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mate.log")
	writeFile(t, filepath.Join(dir, "mate.yaml"), "log_file: "+logPath+"\nlog_level: debug\n", 0644)
	t.Setenv("STATEMATE_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := runRoot(t, "--log-level", "info", "profile"); err != nil {
		t.Fatalf("mate profile: %v", err)
	}
	data, _ := os.ReadFile(logPath)
	if log := string(data); !strings.Contains(log, "msg=run") || strings.Contains(log, "level=DEBUG") {
		t.Errorf("want only info lines:\n%s", log)
	}
}

func TestBadLogLevel(t *testing.T) {
	t.Setenv("STATEMATE_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("STATEMATE_LOG_LEVEL", "loud")

	err := runRoot(t, "--log-file", filepath.Join(t.TempDir(), "mate.log"), "version")
	if err == nil || !strings.Contains(err.Error(), "log level") {
		t.Errorf("err = %v, want a log level error", err)
	}
}
