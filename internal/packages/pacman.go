package packages

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strings"
)

type PacmanManager struct{}

func NewPacmanManager() *PacmanManager {
	return &PacmanManager{}
}

func (p *PacmanManager) Name() string {
	return "pacman"
}

func (p *PacmanManager) IsAvailable() bool {
	_, err := exec.LookPath("pacman")
	return err == nil
}

func (p *PacmanManager) ListInstalled() ([]Package, error) {
	cmd := exec.Command("pacman", "-Qen")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var packages []Package
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			pkg := Package{Name: parts[0]}
			if len(parts) >= 2 {
				pkg.Version = parts[1]
			}
			packages = append(packages, pkg)
		}
	}

	return packages, scanner.Err()
}

func (p *PacmanManager) QueryInstalled(pkgs []string) ([]Package, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}

	// Bulk query first (fast path)
	args := append([]string{"-Q"}, pkgs...)
	cmd := exec.Command("pacman", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	found := make(map[string]Package)
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			pkg := Package{Name: parts[0]}
			if len(parts) >= 2 {
				pkg.Version = parts[1]
			}
			found[pkg.Name] = pkg
		}
	}

	// For packages not found by exact name, re-check individually
	// (handles virtual/provides packages like "man" -> "man-db")
	var packages []Package
	for _, name := range pkgs {
		if pkg, ok := found[name]; ok {
			packages = append(packages, pkg)
			continue
		}
		cmd := exec.Command("pacman", "-Q", name)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			continue
		}
		line := strings.TrimSpace(out.String())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		pkg := Package{Name: name}
		if len(parts) >= 2 {
			pkg.Version = parts[1]
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

// Describe returns descriptions for the installed packages among pkgs.
//
// It reports nothing as Unknown: `pacman -Qi` queries the local database, so a name
// it has no entry for is simply not installed, which says nothing about whether the
// package exists. Only a manager that can consult the full catalogue (see
// BrewManager.Describe) can make that claim.
func (p *PacmanManager) Describe(pkgs []string) (Descriptions, error) {
	if len(pkgs) == 0 {
		return Descriptions{}, nil
	}
	args := append([]string{"-Qi"}, pkgs...)
	cmd := exec.Command("pacman", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	// pacman -Qi exits non-zero when any argument is not installed, but still
	// prints the entries it did find. Status lists packages that are missing by
	// definition, so treating that exit code as fatal would blank every
	// description instead of just the ones pacman has nothing to say about.
	_ = cmd.Run()

	result := make(map[string]string)
	var currentName string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Name") {
			currentName = strings.TrimSpace(strings.TrimPrefix(line, "Name            :"))
		} else if strings.HasPrefix(line, "Description") {
			desc := strings.TrimSpace(strings.TrimPrefix(line, "Description     :"))
			if currentName != "" {
				result[currentName] = desc
			}
		}
	}
	return Descriptions{ByName: result}, scanner.Err()
}

func (p *PacmanManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"-S", "--noconfirm"}, pkgs...)
	cmd := exec.Command("sudo", append([]string{"pacman"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (p *PacmanManager) Uninstall(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"-R", "--noconfirm"}, pkgs...)
	cmd := exec.Command("sudo", append([]string{"pacman"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
