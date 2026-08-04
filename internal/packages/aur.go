package packages

import (
	"bufio"
	"bytes"
	"os"
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
	// -Qmtt lists foreign (AUR) packages that are unrequired and not optdeps
	cmd := exec.Command(a.helper, "-Qmtt")
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
			if strings.HasSuffix(parts[0], "-debug") {
				continue
			}
			pkg := Package{Name: parts[0]}
			if len(parts) >= 2 {
				pkg.Version = parts[1]
			}
			packages = append(packages, pkg)
		}
	}

	return packages, scanner.Err()
}

func (a *AURManager) QueryInstalled(pkgs []string) ([]Package, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}

	// Bulk query first (fast path)
	args := append([]string{"-Qm"}, pkgs...)
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
	var packages []Package
	for _, name := range pkgs {
		if pkg, ok := found[name]; ok {
			packages = append(packages, pkg)
			continue
		}
		cmd := exec.Command("pacman", "-Qm", name)
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

func (a *AURManager) Describe(pkgs []string) (map[string]string, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}
	args := append([]string{"-Qi"}, pkgs...)
	cmd := exec.Command("pacman", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

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
	return result, scanner.Err()
}

func (a *AURManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"-S", "--noconfirm"}, pkgs...)
	cmd := exec.Command(a.helper, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (a *AURManager) Uninstall(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"-R", "--noconfirm"}, pkgs...)
	cmd := exec.Command(a.helper, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
