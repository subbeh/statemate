package packages

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/subbeh/statemate/internal/util"
)

// npmBundled are globals that ship with npm itself: pruning them breaks npm.
var npmBundled = map[string]bool{"npm": true, "corepack": true}

// NPMManager installs Node CLI tools globally with `npm install -g`.
type NPMManager struct{}

func NewNPMManager() *NPMManager {
	return &NPMManager{}
}

func (n *NPMManager) Name() string {
	return "npm"
}

func (n *NPMManager) IsAvailable() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}

// ListInstalled lists the global packages a user installed, leaving out npm's
// own bundled ones so they never show up as extras.
func (n *NPMManager) ListInstalled() ([]Package, error) {
	all, err := n.globals()
	if err != nil {
		return nil, err
	}
	var packages []Package
	for _, p := range all {
		if !npmBundled[p.Name] {
			packages = append(packages, p)
		}
	}
	return packages, nil
}

func (n *NPMManager) QueryInstalled(pkgs []string) ([]Package, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}
	all, err := n.globals()
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(pkgs))
	for _, name := range pkgs {
		wanted[name] = true
	}
	var packages []Package
	for _, p := range all {
		if wanted[p.Name] {
			packages = append(packages, p)
		}
	}
	return packages, nil
}

func (n *NPMManager) globals() ([]Package, error) {
	// npm ls exits non-zero on problems like an invalid peer dep but still prints
	// the tree, so parse whatever it wrote before giving up.
	out, err := util.Command("npm", "ls", "-g", "--depth=0", "--json").Output()
	packages, perr := parseNPMList(out)
	if perr != nil {
		if err != nil {
			return nil, err
		}
		return nil, perr
	}
	return packages, nil
}

// parseNPMList reads `npm ls -g --depth=0 --json`, sorted by name.
func parseNPMList(out []byte) ([]Package, error) {
	var tree struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &tree); err != nil {
		return nil, err
	}
	packages := make([]Package, 0, len(tree.Dependencies))
	for name, dep := range tree.Dependencies {
		packages = append(packages, Package{Name: name, Version: dep.Version})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	return packages, nil
}

// Describe reads each installed package's package.json under `npm root -g`.
// Like uv it says nothing about packages that are not installed.
func (n *NPMManager) Describe(pkgs []string) (Descriptions, error) {
	if len(pkgs) == 0 {
		return Descriptions{}, nil
	}
	out, err := util.Command("npm", "root", "-g").Output()
	if err != nil {
		return Descriptions{}, err
	}
	root := strings.TrimSpace(string(out))
	result := make(map[string]string)
	for _, name := range pkgs {
		data, err := os.ReadFile(filepath.Join(root, name, "package.json"))
		if err != nil {
			continue
		}
		var manifest struct {
			Description string `json:"description"`
		}
		if json.Unmarshal(data, &manifest) == nil && manifest.Description != "" {
			result[name] = manifest.Description
		}
	}
	return Descriptions{ByName: result}, nil
}

func (n *NPMManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	return n.run("install", pkgs...)
}

func (n *NPMManager) Uninstall(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	return n.run("uninstall", pkgs...)
}

func (n *NPMManager) run(sub string, pkgs ...string) error {
	cmd := util.Command("npm", append([]string{sub, "-g"}, pkgs...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
