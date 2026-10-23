package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/subbeh/statemate/internal/util"
)

// sudoLstat runs statCommand through sudo. Running it without sudo checks the
// format against this platform's real stat, which differs between GNU and BSD.
func TestStatCommandMatchesLstat(t *testing.T) {
	dir := t.TempDir()
	file, sub, link := filepath.Join(dir, "f"), filepath.Join(dir, "d"), filepath.Join(dir, "l")
	if err := os.WriteFile(file, []byte("hi\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sub, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{file, sub, link} {
		args := statCommand(p)
		out, err := util.Command(args[0], args[1:]...).Output()
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		got, err := parseStat(p, out)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got.Mode().Perm() != want.Mode().Perm() || got.Mode().Type() != want.Mode().Type() || got.Size() != want.Size() {
			t.Errorf("%s: got mode %v size %d, want mode %v size %d", filepath.Base(p), got.Mode(), got.Size(), want.Mode(), want.Size())
		}
	}
}
