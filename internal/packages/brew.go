package packages

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
)

type BrewManager struct{}

func NewBrewManager() *BrewManager {
	return &BrewManager{}
}

func (b *BrewManager) Name() string {
	return "brew"
}

func (b *BrewManager) IsAvailable() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

func (b *BrewManager) ListInstalled() ([]Package, error) {
	// Use brew leaves to get only explicitly installed packages (not dependencies)
	cmd := exec.Command("brew", "leaves", "--installed-on-request")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var packages []Package
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			packages = append(packages, Package{Name: name})
		}
	}

	// Also get explicitly installed casks
	cmd = exec.Command("brew", "list", "--cask", "-1")
	out.Reset()
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		scanner = bufio.NewScanner(&out)
		for scanner.Scan() {
			name := strings.TrimSpace(scanner.Text())
			if name != "" {
				packages = append(packages, Package{Name: name})
			}
		}
	}

	return packages, scanner.Err()
}

func (b *BrewManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"install"}, pkgs...)
	cmd := exec.Command("brew", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (b *BrewManager) Uninstall(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"uninstall"}, pkgs...)
	cmd := exec.Command("brew", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
