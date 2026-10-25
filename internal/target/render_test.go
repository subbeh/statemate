package target

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/fatih/color"

	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/template"
)

// One render decides what an entry deploys. Apply, status and diff each had a
// copy, and the copies disagreed when there was no identity or no context.
func TestRender(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt.NewAgeEncryptor(identity.String(), "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatal(err)
	}
	recipientsOnly, err := encrypt.NewAgeEncryptor("", "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatal(err)
	}
	encrypted := func(s string) string {
		b, err := enc.Encrypt([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	ctx := &template.Context{Profile: "work"}
	tmpl := source.Attrs{Template: true}
	encAttr := source.Attrs{Encrypted: true}

	tests := []struct {
		name      string
		attrs     source.Attrs
		content   string
		generated bool
		tmplCtx   *template.Context
		enc       *encrypt.AgeEncryptor
		want      string
		wantErr   string
	}{
		{name: "plain", content: "{{ .Profile }}", tmplCtx: ctx, enc: enc, want: "{{ .Profile }}"},
		{name: "generated", generated: true, content: "made", want: "made"},
		{name: "tmpl", attrs: tmpl, content: "{{ .Profile }}", tmplCtx: ctx, want: "work"},
		{name: "tmpl without context", attrs: tmpl, content: "{{ .Profile }}", wantErr: "no template context"},
		{name: "tmpl that fails", attrs: tmpl, content: "{{ .Profile", tmplCtx: ctx, wantErr: "rendering"},
		{name: "enc", attrs: encAttr, content: encrypted("secret"), enc: enc, want: "secret"},
		{name: "enc without age config", attrs: encAttr, content: encrypted("secret"), wantErr: "no age identity"},
		{name: "enc with recipients only", attrs: encAttr, content: encrypted("secret"), enc: recipientsOnly, wantErr: "no age identity"},
		{name: "enc tmpl", attrs: source.Attrs{Encrypted: true, Template: true}, content: encrypted("{{ .Profile }}"), tmplCtx: ctx, enc: enc, want: "work"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			// A generated entry's source is the directory whose .mate.yaml made it.
			entry := &source.Entry{SourcePath: dir, Attrs: tt.attrs, Generated: tt.generated, GeneratedContent: tt.content}
			if !tt.generated {
				entry.SourcePath = filepath.Join(dir, fmt.Sprint(i))
				if err := os.WriteFile(entry.SourcePath, []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := render(entry, tt.tmplCtx, tt.enc)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), entry.SourcePath) {
					t.Fatalf("expected an error naming %s and %q, got %v", entry.SourcePath, tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A template that cannot render fails status, naming the file. Its error was
// swallowed, so status compared the unrendered template with the target and
// reported a change that apply then failed on.
func TestComputeChanges_RenderErrorFails(t *testing.T) {
	f := newImportFixture(t, "config#tmpl", "{{ .Profile")

	_, err := ComputeChanges(f.tree, f.db, ComputeOpts{TmplCtx: &template.Context{}})
	if err == nil || !strings.Contains(err.Error(), f.sourcePath) {
		t.Fatalf("expected an error naming %s, got %v", f.sourcePath, err)
	}
}

func TestDiff(t *testing.T) {
	saved := color.NoColor
	t.Cleanup(func() { color.NoColor = saved })
	color.NoColor = true

	dir := t.TempDir()
	src := filepath.Join(dir, "config.toml#tmpl")
	dst := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(src, []byte("name = {{ .Profile }}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	entry := &source.Entry{SourcePath: src, TargetPath: dst, Attrs: source.Attrs{Template: true}}
	diff := func(entry *source.Entry, status ChangeStatus, tool string) string {
		t.Helper()
		got, err := Diff(&Change{Entry: entry, Status: status}, &template.Context{Profile: "work"}, nil, tool)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	contains := func(got string, want ...string) {
		t.Helper()
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("diff does not contain %q:\n%s", w, got)
			}
		}
	}

	// Without a target everything is added, and what is added is rendered.
	contains(diff(entry, StatusNew, ""), "--- /dev/null\n+++ "+src+"\n", "+name = work")

	if err := os.WriteFile(dst, []byte("name = home\n"), 0644); err != nil {
		t.Fatal(err)
	}
	contains(diff(entry, StatusModified, ""), "--- "+dst+"\n+++ "+src+"\n", "-name = home", "+name = work")

	// #import writes the target into the source, so the sides swap.
	imported := &source.Entry{SourcePath: src, TargetPath: dst, Attrs: source.Attrs{Import: true}}
	contains(diff(imported, StatusImport, ""), "--- "+src+"\n+++ "+dst+"\n", "-name = {{ .Profile }}", "+name = home")

	// A tool gets two temp files named after the target, and they are removed.
	paths := strings.Fields(diff(entry, StatusModified, "echo"))
	if len(paths) != 2 {
		t.Fatalf("expected two paths from the tool, got %q", paths)
	}
	for _, p := range paths {
		if !strings.HasSuffix(p, "-config.toml") {
			t.Errorf("temp file %s is not named after the target", p)
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("temp file %s was left behind", p)
		}
	}

	if err := os.WriteFile(dst, []byte("\x00\x01"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := diff(entry, StatusModified, ""); got != "Binary files differ" {
		t.Errorf("binary target: got %q", got)
	}
}

// A target only root can read, such as /etc/crypttab, is diffed through sudo.
// diff -u was handed the path and printed "Permission denied".
func TestDiff_UnreadableTargetUsesSudo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: mode 000 is still readable")
	}
	saved := color.NoColor
	t.Cleanup(func() { color.NoColor = saved })
	color.NoColor = true

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "sudo"), []byte("#!/bin/sh\nprintf 'deployed\\n'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := t.TempDir()
	src := filepath.Join(dir, "crypttab")
	dst := filepath.Join(dir, "target")
	if err := os.WriteFile(src, []byte("edited\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("unreadable\n"), 0000); err != nil {
		t.Fatal(err)
	}

	got, err := Diff(&Change{Entry: &source.Entry{SourcePath: src, TargetPath: dst}, Status: StatusConflict}, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-deployed", "+edited"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff does not contain %q:\n%s", want, got)
		}
	}
}

// mate diff shows a #link change as link text. Diffing the destinations
// failed for a link to a directory or to a path that does not exist.
func TestSymlinkDiff(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "link#link")
	dst := filepath.Join(dir, "link")
	if err := os.Symlink(dir, src); err != nil {
		t.Fatal(err)
	}
	entry := &source.Entry{SourcePath: src, TargetPath: dst, Attrs: source.Attrs{Symlink: true}}

	if got, want := symlinkDiff(entry), "+ -> "+dir; got != want {
		t.Errorf("missing target: got %q, want %q", got, want)
	}

	if err := os.Symlink("/nonexistent", dst); err != nil {
		t.Fatal(err)
	}
	if got, want := symlinkDiff(entry), "- -> /nonexistent\n+ -> "+dir; got != want {
		t.Errorf("relinked target: got %q, want %q", got, want)
	}

	if err := os.Remove(dst); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, want := symlinkDiff(entry), "- (not a symlink)\n+ -> "+dir; got != want {
		t.Errorf("regular file target: got %q, want %q", got, want)
	}
}
