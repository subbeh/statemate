package util

import (
	"os"
	"testing"
)

// withStdin replaces os.Stdin with a pipe holding input for the duration of
// the test.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		_ = r.Close()
	})
}

// Piped input is read a line at a time, so each prompt consumes exactly one
// answer and leaves the rest for the next one.
func TestReadKey_PipedInputConsumesOneLinePerCall(t *testing.T) {
	withStdin(t, "yes\n\nn")

	for _, want := range []byte{'y', '\n', 'n'} {
		got, err := ReadKey()
		if err != nil {
			t.Fatalf("ReadKey: %v", err)
		}
		if got != want {
			t.Errorf("ReadKey: got %q, want %q", got, want)
		}
	}

	if _, err := ReadKey(); err == nil {
		t.Error("expected an error at EOF")
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		input      string
		defaultYes bool
		want       bool
	}{
		{"y\n", false, true},
		{"Y\n", false, true},
		{"n\n", true, false},
		{"\n", true, true},
		{"\n", false, false},
		{"x\n", true, false},
	}
	for _, tt := range tests {
		withStdin(t, tt.input)
		got, err := Confirm("", tt.defaultYes)
		if err != nil {
			t.Fatalf("Confirm(%q): %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("Confirm(%q, defaultYes=%v): got %v, want %v", tt.input, tt.defaultYes, got, tt.want)
		}
	}
}
