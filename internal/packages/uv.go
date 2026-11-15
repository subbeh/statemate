package packages

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
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

// Describe returns nothing: uv has no way to look up a tool's summary.
func (u *UVManager) Describe(pkgs []string) (Descriptions, error) {
	return Descriptions{}, nil
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
