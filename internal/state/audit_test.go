package state

import (
	"path/filepath"
	"testing"
	"time"
)

// Runs come back newest first, and only the newest keep runs are kept.
func TestAuditRuns(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i, cmd := range []string{"one", "two", "three"} {
		run := AuditRun{Time: start.Add(time.Duration(i) * time.Hour), Args: []string{cmd}, Changes: []AuditChange{{Level: "INFO", Msg: "wrote", Attrs: [][2]string{{"target", "/x"}}}}}
		if err := db.RecordAuditRun(run, 2); err != nil {
			t.Fatal(err)
		}
	}

	runs, err := db.AuditRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].Args[0] != "three" || runs[1].Args[0] != "two" {
		t.Fatalf("runs = %+v, want three then two", runs)
	}
	if !runs[0].Time.Equal(start.Add(2*time.Hour)) || runs[0].Changes[0].Attrs[0] != [2]string{"target", "/x"} {
		t.Errorf("run did not round-trip: %+v", runs[0])
	}
}
