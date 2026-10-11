package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
)

// The interactive source picker shows one list and indexes into another. If the
// two ever diverge, a selection silently maps to the wrong source -- so they must
// be derived from the same resolved list, not from cfg.Sources.
func TestSourcePickerListMatchesIndexedList(t *testing.T) {
	repo := t.TempDir()

	for _, d := range []string{"app", "extra"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// "extra" is contributed by a profile, not by the top-level sources list.
	cfgContent := "target_base: " + t.TempDir() + "\n" +
		"sources:\n  - app\n" +
		"profiles:\n" +
		"  here:\n" +
		"    detection:\n" +
		"      os: " + osName() + "\n" +
		"    sources:\n      - extra\n"

	cfgPath := filepath.Join(repo, "mate.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	profileName := profile.Detect(cfg)
	if profileName == "" {
		t.Skip("profile did not match on this platform")
	}

	sources := profile.ResolveSources(cfg, profileName)
	absSources := cfg.ResolveSourcePaths(sources)

	// This is the invariant: runAdd shows `sources` and indexes `absSources`.
	if len(sources) != len(absSources) {
		t.Fatalf("picker list (%d) and indexed list (%d) differ in length", len(sources), len(absSources))
	}

	// The profile-provided source must be offered at all -- showing cfg.Sources
	// would omit it.
	if len(sources) <= len(cfg.Sources) {
		t.Errorf("expected profile sources to extend cfg.Sources (%v), got %v", cfg.Sources, sources)
	}

	var found bool
	for _, s := range sources {
		if s == "extra" {
			found = true
		}
	}
	if !found {
		t.Errorf("profile-provided source missing from picker list: %v", sources)
	}

	// Every displayed entry must resolve to the matching absolute path, so
	// selecting index i yields the source the user actually saw.
	for i, s := range sources {
		want := filepath.Join(cfg.SourceDir(), s)
		if absSources[i] != want {
			t.Errorf("index %d: showed %q but would use %q, want %q", i, s, absSources[i], want)
		}
	}
}

// Adding a file from outside the target base to a source with no .mate.yaml
// must not give the whole source a new target_base while it already holds home
// files: that would redeploy ~/.zshrc as /etc/.zshrc.
func TestAddOutsideTargetBaseRefusesSourceWithHomeFiles(t *testing.T) {
	home := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "shell")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".zshrc"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tree := &source.Tree{}
	tree.AddEntry(&source.Entry{
		SourcePath: filepath.Join(sourceDir, ".zshrc"),
		TargetPath: filepath.Join(home, ".zshrc"),
		RelPath:    ".zshrc",
	})

	// Confirm the .mate.yaml prompt, should it be (wrongly) shown.
	withStdin(t, "y\n")

	_, err := resolveTargetBaseForAdd(sourceDir, "/etc/hosts", home, tree, nil)
	if err == nil || !strings.Contains(err.Error(), "existing files") {
		t.Errorf("expected an error about the source's existing files, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(sourceDir, ".mate.yaml")); statErr == nil {
		t.Error(".mate.yaml with a new target_base was written for a source holding home files")
	}
}

// executeAdd runs `mate add` through the root command, as the binary does,
// resetting the flags it may set afterwards.
func executeAdd(t *testing.T, args ...string) error {
	t.Helper()
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		for _, f := range []string{"config", "profile"} {
			_ = rootCmd.PersistentFlags().Set(f, "")
			rootCmd.PersistentFlags().Lookup(f).Changed = false
		}
		addForProfile, addSource, addEncrypt, addTemplate = "", "", false, false
	})
	rootCmd.SetArgs(append([]string{"add"}, args...))
	return rootCmd.Execute()
}

// addRepo creates a repository whose "extra" source is contributed only by the
// work profile, which nothing auto-detects, and a home file to add.
func addRepo(t *testing.T) (cfgPath, file string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("STATEMATE_PROFILE", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := t.TempDir()
	for _, d := range []string{"base", "extra"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	cfgPath = filepath.Join(repo, "mate.yaml")
	cfg := "target_base: " + home + "\nsources: [base]\nprofiles:\n  work:\n    sources: [extra]\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	file = filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(file, []byte("[user]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return cfgPath, file
}

// The global -p/--profile must work on add like on every other command, and
// pick the sources add can choose from. A local --profile used to shadow it, so
// -p was an unknown flag and profile-only sources could not be added to.
func TestAddHonoursGlobalProfileFlag(t *testing.T) {
	cfgPath, file := addRepo(t)

	if err := executeAdd(t, "-c", cfgPath, "-p", "work", "-s", "extra", file); err != nil {
		t.Fatalf("mate add -p work -s extra: %v", err)
	}

	want := filepath.Join(filepath.Dir(cfgPath), "extra", ".gitconfig")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s to be added without a profile suffix: %v", want, err)
	}
}

func TestAddForProfileAddsSuffix(t *testing.T) {
	cfgPath, file := addRepo(t)

	if err := executeAdd(t, "-c", cfgPath, "--for-profile", "work", "-s", "base", file); err != nil {
		t.Fatalf("mate add --for-profile work: %v", err)
	}

	want := filepath.Join(filepath.Dir(cfgPath), "base", ".gitconfig#profile:work")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s: %v", want, err)
	}
}

func TestAddDirectoryRecursively(t *testing.T) {
	cfgPath, file := addRepo(t)
	home := filepath.Dir(file)
	dir := filepath.Join(home, ".config", "nvim")
	files := map[string]string{
		"init.lua":             "-- config\n",
		"lua/plugins/main.lua": "return {}\n",
		".hidden/settings":     "hidden\n",
	}
	for path, content := range files {
		writeAddTestFile(t, filepath.Join(dir, path), content)
	}
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "init.lua"), 0700); err != nil {
		t.Fatal(err)
	}

	if err := executeAdd(t, "-c", cfgPath, "-s", "base", dir); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(filepath.Dir(cfgPath), "base", ".config", "nvim")
	for path, want := range files {
		for _, root := range []string{sourceDir, dir} {
			got, err := os.ReadFile(filepath.Join(root, path))
			if err != nil || string(got) != want {
				t.Errorf("%s: content = %q, err = %v; want %q", filepath.Join(root, path), got, err, want)
			}
		}
	}
	info, err := os.Stat(filepath.Join(sourceDir, "init.lua"))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("executable mode was not preserved: %v, %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "empty")); !os.IsNotExist(err) {
		t.Fatalf("empty directory should not be added: %v", err)
	}
}

func TestAddDirectoryAppliesFlagsToEachFile(t *testing.T) {
	cfgPath, file := addRepo(t)
	dir := filepath.Join(filepath.Dir(file), "configs")
	for _, name := range []string{"one", "nested/two"} {
		writeAddTestFile(t, filepath.Join(dir, name), "{{ .Profile }}\n")
	}
	if err := executeAdd(t, "-c", cfgPath, "-p", "work", "-s", "extra", "--for-profile", "work", "--template", dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "nested/two"} {
		path := filepath.Join(filepath.Dir(cfgPath), "extra", "configs", name+"#profile:work#template")
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "{{ .Profile }}\n" {
			t.Errorf("template %s: content = %q, err = %v", path, got, err)
		}
	}
}

func TestAddDirectoryManagedFiles(t *testing.T) {
	for _, otherSource := range []bool{false, true} {
		t.Run(map[bool]string{false: "same source", true: "different source"}[otherSource], func(t *testing.T) {
			cfgPath, file := addRepo(t)
			dir := filepath.Join(filepath.Dir(file), "configs")
			writeAddTestFile(t, filepath.Join(dir, "a-new"), "new")
			writeAddTestFile(t, filepath.Join(dir, "z-managed"), "target edits")
			owner := "base"
			if otherSource {
				owner = "extra"
			}
			managed := filepath.Join(filepath.Dir(cfgPath), owner, "configs", "z-managed")
			writeAddTestFile(t, managed, "source content")
			err := executeAdd(t, "-c", cfgPath, "-p", "work", "-s", "base", dir)
			newPath := filepath.Join(filepath.Dir(cfgPath), "base", "configs", "a-new")
			if otherSource {
				if err == nil || !strings.Contains(err.Error(), "another source") {
					t.Fatalf("want another-source conflict, got %v", err)
				}
				if _, err := os.Stat(newPath); !os.IsNotExist(err) {
					t.Fatalf("new file copied before conflict was checked: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(newPath); err != nil {
					t.Fatal(err)
				}
			}
			got, err := os.ReadFile(managed)
			if err != nil || string(got) != "source content" {
				t.Fatalf("managed file was changed: %q, %v", got, err)
			}
		})
	}
}

func TestAddDirectorySymlinks(t *testing.T) {
	cfgPath, file := addRepo(t)
	dir := filepath.Join(filepath.Dir(file), "configs")
	writeAddTestFile(t, filepath.Join(dir, "regular"), "contents")
	for name, target := range map[string]string{"linked": "regular", "loop": ".", "directory": t.TempDir()} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := executeAdd(t, "-c", cfgPath, "-s", "base", dir); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(filepath.Dir(cfgPath), "base", "configs")
	for _, name := range []string{"regular", "linked"} {
		path := filepath.Join(sourceDir, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("expected ordinary file %s: %v, %v", path, info, err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "contents" {
			t.Fatalf("unexpected contents %q: %v", got, err)
		}
	}
	for _, name := range []string{"loop", "directory"} {
		if _, err := os.Lstat(filepath.Join(sourceDir, name)); !os.IsNotExist(err) {
			t.Fatalf("directory symlink %s should be skipped: %v", name, err)
		}
	}
}

func TestAddDirectoryBrokenSymlinkDoesNotCopyFiles(t *testing.T) {
	cfgPath, file := addRepo(t)
	dir := filepath.Join(filepath.Dir(file), "configs")
	writeAddTestFile(t, filepath.Join(dir, "a-valid"), "contents")
	broken := filepath.Join(dir, "z-broken")
	if err := os.Symlink("missing", broken); err != nil {
		t.Fatal(err)
	}
	err := executeAdd(t, "-c", cfgPath, "-s", "base", dir)
	if err == nil || !strings.Contains(err.Error(), broken) {
		t.Fatalf("expected broken symlink error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfgPath), "base", "configs")); !os.IsNotExist(err) {
		t.Fatalf("files copied before traversal completed: %v", err)
	}
}

func TestAddDirectoryExcludesSources(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "directory", true: "symlinked source"}[symlink], func(t *testing.T) {
			cfgPath, file := addRepo(t)
			home := filepath.Dir(file)
			sourceDir := filepath.Join(home, "source")
			writeAddTestFile(t, filepath.Join(sourceDir, "managed"), "managed")
			configuredSource := sourceDir
			if symlink {
				configuredSource = filepath.Join(t.TempDir(), "source-link")
				if err := os.Symlink(sourceDir, configuredSource); err != nil {
					t.Fatal(err)
				}
			}
			cfg := "target_base: " + home + "\nsources: [" + configuredSource + "]\ndefault_source: " + configuredSource + "\n"
			writeAddTestFile(t, cfgPath, cfg)
			if err := executeAdd(t, "-c", cfgPath, home); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(sourceDir, ".gitconfig")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(sourceDir, "source")); !os.IsNotExist(err) {
				t.Fatalf("source copied into itself: %v", err)
			}
		})
	}
}

func TestAddEmptyDirectory(t *testing.T) {
	cfgPath, file := addRepo(t)
	dir := filepath.Join(filepath.Dir(file), "empty")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := executeAdd(t, "-c", cfgPath, "-s", "base", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfgPath), "base", "empty")); !os.IsNotExist(err) {
		t.Fatalf("empty directory should not be copied: %v", err)
	}
}

func TestAddDirectoryEncrypted(t *testing.T) {
	cfgPath, file := addRepo(t)
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg = append(cfg, []byte("age:\n  recipients: ["+identity.Recipient().String()+"]\n")...)
	writeAddTestFile(t, cfgPath, string(cfg))
	dir := filepath.Join(filepath.Dir(file), "secrets")
	for _, name := range []string{"one", "nested/two"} {
		writeAddTestFile(t, filepath.Join(dir, name), "secret "+name)
	}
	if err := executeAdd(t, "-c", cfgPath, "-s", "base", "--encrypt", dir); err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt.NewAgeEncryptor(identity.String(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one", "nested/two"} {
		path := filepath.Join(filepath.Dir(cfgPath), "base", "secrets", name+"#encrypted")
		got, err := enc.DecryptFile(path)
		if err != nil || string(got) != "secret "+name {
			t.Fatalf("encrypted file %s: %q, %v", path, got, err)
		}
	}
}

func TestAddDirectoryWithTargetMappingAndAttributes(t *testing.T) {
	cfgPath, _ := addRepo(t)
	outside := filepath.Join(t.TempDir(), "etc")
	writeAddTestFile(t, filepath.Join(outside, "app", "config"), "config")
	sourceDir := filepath.Join(filepath.Dir(cfgPath), "base")
	writeAddTestFile(t, filepath.Join(sourceDir, ".mate.yaml"), "targets:\n  etc: "+outside+"\n")
	if err := os.Mkdir(filepath.Join(sourceDir, "etc#owner-r:root"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := executeAdd(t, "-c", cfgPath, "-s", "base", outside); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sourceDir, "etc#owner-r:root", "app", "config")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := newScanner(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := scanner.Scan([]string{sourceDir})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files()) != 1 || tree.Files()[0].TargetPath != filepath.Join(outside, "app", "config") {
		t.Fatalf("added file does not map back to original target: %+v", tree.Files())
	}
}

func writeAddTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func osName() string {
	return runtime.GOOS
}
