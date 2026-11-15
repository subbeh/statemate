package packages

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/subbeh/statemate/internal/util"
)

// UVManager installs Python CLI tools with `uv tool`.
type UVManager struct{}

func NewUVManager() *UVManager {
	return &UVManager{}
}

func (u *UVManager) Name() string {
	return "uv"
}

func (u *UVManager) IsAvailable() bool {
	_, err := exec.LookPath("uv")
	return err == nil
}

// ListInstalled parses `uv tool list`, which prints each tool as "ruff v0.6.9"
// followed by its executables as "- ruff" lines.
func (u *UVManager) ListInstalled() ([]Package, error) {
	out, err := util.Command("uv", "tool", "list").Output()
	if err != nil {
		return nil, err
	}

	var packages []Package
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		parts := strings.Fields(line)
		pkg := Package{Name: parts[0]}
		if len(parts) >= 2 {
			pkg.Version = strings.TrimPrefix(parts[1], "v")
		}
		packages = append(packages, pkg)
	}
	return packages, scanner.Err()
}

func (u *UVManager) QueryInstalled(pkgs []string) ([]Package, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}
	installed, err := u.ListInstalled()
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(pkgs))
	for _, name := range pkgs {
		wanted[name] = true
	}
	var packages []Package
	for _, p := range installed {
		if wanted[p.Name] {
			packages = append(packages, p)
		}
	}
	return packages, nil
}

// Describe reads each installed tool's summary from the package metadata in its
// environment, since uv has no command for it. Like pacman it says nothing about
// tools that are not installed.
func (u *UVManager) Describe(pkgs []string) (Descriptions, error) {
	if len(pkgs) == 0 {
		return Descriptions{}, nil
	}
	out, err := util.Command("uv", "tool", "dir").Output()
	if err != nil {
		return Descriptions{}, err
	}
	return Descriptions{ByName: toolSummaries(strings.TrimSpace(string(out)), pkgs)}, nil
}

// toolSummaries finds <dir>/<name>/lib/python*/site-packages/<name>-*.dist-info/METADATA,
// where the dist-info name has - and . turned into _.
func toolSummaries(dir string, pkgs []string) map[string]string {
	result := make(map[string]string)
	for _, name := range pkgs {
		dist := strings.NewReplacer("-", "_", ".", "_").Replace(name)
		matches, _ := filepath.Glob(filepath.Join(dir, name, "lib", "python*", "site-packages", dist+"-*.dist-info", "METADATA"))
		for _, m := range matches {
			if summary := metadataSummary(m); summary != "" {
				result[name] = summary
				break
			}
		}
	}
	return result
}

func metadataSummary(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	// The headers end at the first blank line, before the long description.
	for scanner.Scan() && scanner.Text() != "" {
		if s, ok := strings.CutPrefix(scanner.Text(), "Summary:"); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// Install runs `uv tool install` once per package, since it takes one at a time.
func (u *UVManager) Install(pkgs []string) error {
	for _, pkg := range pkgs {
		if err := u.run("install", pkg); err != nil {
			return err
		}
	}
	return nil
}

func (u *UVManager) Uninstall(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	return u.run("uninstall", pkgs...)
}

func (u *UVManager) run(sub string, pkgs ...string) error {
	cmd := util.Command("uv", append([]string{"tool", sub}, pkgs...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
