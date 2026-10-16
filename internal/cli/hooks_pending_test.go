package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/state"
)

// A hook that did not run -- here skipped for want of a terminal, as it would be
// after Ctrl-C or [q]uit -- stays pending once its files are written: mate status
// lists it, and the next apply offers it even though no file changed.
func TestApply_HookNotRunStaysPending(t *testing.T) {
	r := newHookRepo(t, "sources: [app]\n")

	origConfirm := noConfirm
	noConfirm = true
	defer func() { noConfirm = origConfirm }()
	cmd := &cobra.Command{Use: "apply", RunE: runApply}
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("profile", "", "")
	addScopeFlag(cmd)
	if err := runApply(cmd, nil); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if r.ran("app-own") {
		t.Fatal("the hook should have been skipped without a terminal")
	}
	if pending := pendingHookNames(t); !strings.Contains(pending, "app/own") {
		t.Fatalf("app/own should be pending, got %q", pending)
	}

	if out := runStatusOutput(t); !strings.Contains(out, "Pending hooks:") || !strings.Contains(out, "app/own") {
		t.Errorf("mate status should list the pending hook:\n%s", out)
	}

	if err := r.apply(t, nil, ""); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if !r.ran("app-own") {
		t.Error("the pending hook should run on the next apply")
	}
	if pending := pendingHookNames(t); pending != "" {
		t.Errorf("a hook that ran should no longer be pending, got %q", pending)
	}
}

func pendingHookNames(t *testing.T) string {
	t.Helper()
	db, err := state.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	pending, err := db.PendingHooks()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range pending {
		names = append(names, name)
	}
	return strings.Join(names, ",")
}

func runStatusOutput(t *testing.T) string {
	t.Helper()
	cmd := &cobra.Command{Use: "status", RunE: runStatus}
	cmd.Flags().String("config", "", "")
	cmd.Flags().String("profile", "", "")
	cmd.Flags().Bool("short", false, "")
	cmd.Flags().Bool("sudo", false, "")
	addScopeFlag(cmd)

	orig := os.Stdout
	rd, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := runStatus(cmd, nil)
	_ = w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(rd)
	_ = rd.Close()
	if runErr != nil {
		t.Fatalf("status: %v\n%s", runErr, out)
	}
	return string(out)
}
