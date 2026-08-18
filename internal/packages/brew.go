package packages

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
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

// brewIndex answers "is this package installed?" for a name that may or may not
// carry a tap prefix.
//
// Homebrew is inconsistent about which form it prints: `brew list --formula`
// reports a tap formula by its bare name (hermes), while `brew leaves` reports the
// fully-qualified one (jamf/internal-tap/hermes). Users may declare either. Index
// both spellings so any combination matches, rather than reporting an installed
// package as missing and reinstalling it on every apply.
type brewIndex struct {
	full  map[string]bool
	short map[string]bool
}

func newBrewIndex(names []string) *brewIndex {
	idx := &brewIndex{
		full:  make(map[string]bool, len(names)),
		short: make(map[string]bool, len(names)),
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		idx.full[name] = true
		idx.short[unqualifiedName(name)] = true
	}
	return idx
}

// has reports whether the declared name is installed, comparing on the bare name
// when the exact spelling does not match.
//
// Falling back to the bare name means two taps providing the same formula name are
// indistinguishable here. That is deliberate: the alternative is telling someone a
// package they have installed is missing, and then failing to install it forever.
func (i *brewIndex) has(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if i.full[name] {
		return true
	}
	return i.short[unqualifiedName(name)]
}

// listNames runs a `brew list`-style command and returns the names it printed.
func listNames(args ...string) []string {
	cmd := exec.Command("brew", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	var names []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		if name := strings.TrimSpace(scanner.Text()); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func (b *BrewManager) ListInstalled() ([]Package, error) {
	// brew leaves lists only explicitly installed formulae, not dependencies. It
	// reports tap formulae fully qualified.
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
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Also get explicitly installed casks
	for _, name := range listNames("list", "--cask", "-1") {
		packages = append(packages, Package{Name: name})
	}

	return packages, nil
}

func (b *BrewManager) QueryInstalled(pkgs []string) ([]Package, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}

	// --full-name gives tap formulae their qualified name; brewIndex also matches
	// the bare form, so a package declared either way is found.
	names := listNames("list", "--formula", "--full-name")
	names = append(names, listNames("list", "--cask", "--full-name")...)
	installed := newBrewIndex(names)

	var result []Package
	for _, name := range pkgs {
		if installed.has(name) {
			result = append(result, Package{Name: name})
		}
	}
	return result, nil
}

func (b *BrewManager) Describe(pkgs []string) (map[string]string, error) {
	if len(pkgs) == 0 {
		return nil, nil
	}
	args := append([]string{"info", "--json=v2"}, pkgs...)
	cmd := exec.Command("brew", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var info struct {
		Formulae []struct {
			Name string `json:"name"`
			Desc string `json:"desc"`
		} `json:"formulae"`
		Casks []struct {
			Token string `json:"token"`
			Desc  string `json:"desc"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, f := range info.Formulae {
		result[f.Name] = f.Desc
	}
	for _, c := range info.Casks {
		result[c.Token] = c.Desc
	}
	return result, nil
}

func (b *BrewManager) Install(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"install"}, pkgs...)
	cmd := exec.Command("brew", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *BrewManager) Uninstall(pkgs []string) error {
	if len(pkgs) == 0 {
		return nil
	}
	args := append([]string{"uninstall"}, pkgs...)
	cmd := exec.Command("brew", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
