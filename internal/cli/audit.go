package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/util"
)

// auditKeep is how many runs the state database keeps for `mate audit`.
const auditKeep = 100

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "List the changes of recent runs",
	Long: `List what recent mate runs changed, newest first.

Every run that changes something records its changes in the state database:
each target file written, moved or removed, each change to the source tree,
each forget, script, hook and package install, each secrets fetch that changed
a secret, and the error the run ended with. These are the info-level records
of the log file, kept whether or not a log file is set. A --dry-run, and a run
that only reads, records nothing. The newest 100 runs are kept.`,
	Example: `  mate audit
  mate audit -n 3`,
	Args: cobra.NoArgs,
	RunE: runAudit,
}

func init() {
	auditCmd.Flags().IntP("runs", "n", 10, "number of runs to list")
	rootCmd.AddCommand(auditCmd)
}

func runAudit(cmd *cobra.Command, args []string) error {
	n, _ := cmd.Flags().GetInt("runs")
	db, err := state.Open("")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	runs, err := db.AuditRuns(n)
	if err != nil {
		return fmt.Errorf("reading audit log: %w", err)
	}
	if len(runs) == 0 {
		fmt.Println("No changes recorded yet.")
		return nil
	}
	for i, run := range runs {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(bold(run.Time.Local().Format(time.DateTime) + "  " + strings.Join(append([]string{"mate"}, run.Args...), " ")))
		for _, c := range run.Changes {
			line := c.Msg
			for _, a := range c.Attrs {
				line += " " + a[0] + "=" + util.ShortenPath(a[1])
			}
			fmt.Println("  " + line)
		}
	}
	return nil
}

// auditLog collects this run's changes for the state database; Execute saves
// them once the command is done.
var auditLog *auditRecorder

// auditRecorder is a slog handler that keeps the info-and-higher records of a
// run in memory.
type auditRecorder struct {
	mu      *sync.Mutex
	changes *[]state.AuditChange
	attrs   [][2]string
	group   string
}

func newAuditRecorder() *auditRecorder {
	return &auditRecorder{mu: new(sync.Mutex), changes: new([]state.AuditChange)}
}

func (h *auditRecorder) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo
}

func (h *auditRecorder) Handle(_ context.Context, r slog.Record) error {
	c := state.AuditChange{Level: r.Level.String(), Msg: r.Message, Attrs: append([][2]string(nil), h.attrs...)}
	r.Attrs(func(a slog.Attr) bool {
		c.Attrs = append(c.Attrs, [2]string{h.group + a.Key, a.Value.Resolve().String()})
		return true
	})
	h.mu.Lock()
	*h.changes = append(*h.changes, c)
	h.mu.Unlock()
	return nil
}

func (h *auditRecorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.attrs = append([][2]string(nil), h.attrs...)
	for _, a := range attrs {
		c.attrs = append(c.attrs, [2]string{h.group + a.Key, a.Value.Resolve().String()})
	}
	return &c
}

func (h *auditRecorder) WithGroup(name string) slog.Handler {
	c := *h
	c.group += name + "."
	return &c
}

// save records the run, unless it changed nothing: a run that only failed
// holds just its error.
func (h *auditRecorder) save() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	changes := *h.changes
	h.mu.Unlock()
	changed := false
	for _, c := range changes {
		changed = changed || c.Level != slog.LevelError.String()
	}
	if !changed {
		return nil
	}

	db, err := state.Open("")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	return db.RecordAuditRun(state.AuditRun{Time: time.Now(), Args: os.Args[1:], Changes: changes}, auditKeep)
}
