package packages

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
)

type AURManager struct {
	helper string
}

func NewAURManager(helper string) *AURManager {
	if helper == "" {
		helper = "yay"
	}
	return &AURManager{helper: helper}
}

func (a *AURManager) Name() string {
	return "aur"
}

func (a *AURManager) IsAvailable() bool {
	_, err := exec.LookPath(a.helper)
	return err == nil
}

func (a *AURManager) ListInstalled() ([]Package, error) {
	// -Qm lists foreign (AUR) packages
	cmd := exec.Command(a.helper, "-Qm")
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

func (a *AURManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"-S", "--noconfirm"}, pkgs...)
	cmd := exec.Command(a.helper, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (a *AURManager) Uninstall(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"-R", "--noconfirm"}, pkgs...)
	cmd := exec.Command(a.helper, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
