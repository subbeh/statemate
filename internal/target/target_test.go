package target

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBinaryFile(t *testing.T) {
	tmpDir := t.TempDir()

	textFile := filepath.Join(tmpDir, "text.txt")
	if err := os.WriteFile(textFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	binaryFile := filepath.Join(tmpDir, "binary.bin")
	if err := os.WriteFile(binaryFile, []byte{0x00, 0x01, 0x02, 0x03}, 0644); err != nil {
		t.Fatal(err)
	}

	if IsBinaryFile(textFile) {
		t.Error("text file detected as binary")
	}

	if !IsBinaryFile(binaryFile) {
		t.Error("binary file not detected as binary")
	}
}

func TestGenerateDiff(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcFile, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	diff, err := GenerateDiff(srcFile, filepath.Join(tmpDir, "nonexistent.txt"))
	if err != nil {
		t.Fatalf("GenerateDiff failed: %v", err)
	}

	if diff == "" {
		t.Error("expected diff output for new file")
	}
}

func TestNeedsSudo(t *testing.T) {
	home, _ := os.UserHomeDir()

	if needsSudo(filepath.Join(home, "test.txt")) {
		t.Error("home directory should not need sudo")
	}
}

func TestSudoBatch(t *testing.T) {
	batch := NewSudoBatch()

	if !batch.IsEmpty() {
		t.Error("new batch should be empty")
	}

	batch.AddMkdir("/tmp/test", 0755)
	if batch.IsEmpty() {
		t.Error("batch should not be empty after adding operation")
	}

	batch.AddWriteFile("/tmp/test/file.txt", []byte("content"), 0644)
	batch.AddChown("/tmp/test/file.txt", "root", "wheel")
	batch.AddChmod("/tmp/test/file.txt", 0600)

	script := batch.String()
	if script == "" {
		t.Error("batch script should not be empty")
	}
}

func TestChangeStatusString(t *testing.T) {
	tests := []struct {
		status ChangeStatus
		want   string
	}{
		{StatusUnchanged, "unchanged"},
		{StatusNew, "new"},
		{StatusModified, "modified"},
		{StatusConflict, "conflict"},
	}

	for _, tc := range tests {
		if got := tc.status.String(); got != tc.want {
			t.Errorf("%d.String() = %q, want %q", tc.status, got, tc.want)
		}
	}
}
