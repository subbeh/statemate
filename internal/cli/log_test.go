package cli

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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
		_ = rootCmd.PersistentFlags().Set("verbose", "0")
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

// The flag wins over the config, so a run can turn the level down. At info the
// log is an audit log, and a run that changed nothing leaves no line in it.
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
	if log := string(data); log != "" {
		t.Errorf("want an empty log for a run that changed nothing:\n%s", log)
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

// At info the "run" line is written just before the run's first record.
func TestRunHeaderPrecedesFirstRecord(t *testing.T) {
	var buf bytes.Buffer
	h := &runHeader{
		Handler: slog.NewTextHandler(&buf, nil),
		header:  slog.NewRecord(time.Now(), slog.LevelInfo, "run", 0),
		once:    new(sync.Once),
	}
	if buf.Len() != 0 {
		t.Fatalf("header written before any record:\n%s", buf.String())
	}
	log := slog.New(h)
	log.Info("wrote", "target", "a")
	log.With("k", "v").Info("wrote", "target", "b")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], "msg=run") || strings.Contains(lines[2], "msg=run") {
		t.Errorf("want one run line, first:\n%s", buf.String())
	}
}

// -v prints info records to stderr and -vv adds debug ones, without
// timestamps; without -v, stderr gets none.
func TestVerboseLogsToStderr(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mate.yaml"), "profiles:\n  box:\n    detection:\n      command: \"true\"\n", 0644)
	t.Setenv("STATEMATE_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	stderr := func(args ...string) string {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stderr
		os.Stderr = w
		runErr := runRoot(t, args...)
		os.Stderr = orig
		_ = w.Close()
		out, _ := io.ReadAll(r)
		if runErr != nil {
			t.Fatalf("mate %v: %v", args, runErr)
		}
		slog.SetDefault(slog.New(slog.DiscardHandler))
		_ = rootCmd.PersistentFlags().Set("verbose", "0")
		return string(out)
	}

	if out := stderr("profile"); out != "" {
		t.Errorf("without -v, stderr = %q", out)
	}
	if out := stderr("-v", "profile"); strings.Contains(out, "level=DEBUG") {
		t.Errorf("-v printed debug records:\n%s", out)
	}
	out := stderr("-vv", "profile")
	if !strings.Contains(out, "level=DEBUG msg=profile name=box") || strings.Contains(out, "time=") {
		t.Errorf("-vv stderr:\n%s", out)
	}
}
