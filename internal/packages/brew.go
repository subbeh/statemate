package packages

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

// brewPrefix returns Homebrew's install prefix, or "" when brew cannot say.
func brewPrefix() string {
	out, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// canonicalOptName maps a declared name to the formula Homebrew has installed it
// as, or "" when nothing is installed under that name.
//
// Many formulae are known by an alias -- kubectl for kubernetes-cli, az for
// azure-cli -- and the alias is usually the command name people know, so that is
// what they declare. No `brew list` output ever mentions it, so such a package
// looked missing on every run: status offered to install it and brew answered
// "already installed and up-to-date". Homebrew does symlink every alias of an
// installed formula into opt/ next to its canonical name (opt/kubectl and
// opt/kubernetes-cli both point at Cellar/kubernetes-cli/<version>), so reading
// the link resolves the alias for free. The alternative, `brew info --json=v2
// --installed`, carries the same information but takes seconds and would slow down
// every mate status.
func canonicalOptName(prefix, name string) string {
	name = unqualifiedName(strings.TrimSpace(name))
	// An empty prefix means brew could not say where it lives; resolving a relative
	// path from the working directory instead would be nonsense.
	if prefix == "" || name == "" {
		return ""
	}
	link, err := os.Readlink(filepath.Join(prefix, "opt", name))
	if err != nil {
		return ""
	}
	// The link points at Cellar/<formula>/<version>, so the formula is the
	// second-to-last element.
	parts := strings.Split(filepath.ToSlash(filepath.Clean(link)), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2]
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
	var unmatched []string
	for _, name := range pkgs {
		if installed.has(name) {
			result = append(result, Package{Name: name})
		} else {
			unmatched = append(unmatched, name)
		}
	}

	// A name brew never listed may still be an alias of something installed. Only
	// look then: locating the prefix costs a brew call, and cross-checking the
	// resolved name against the list keeps `brew list` the authority on what is
	// installed.
	if len(unmatched) > 0 {
		if prefix := brewPrefix(); prefix != "" {
			for _, name := range unmatched {
				if canonical := canonicalOptName(prefix, name); installed.has(canonical) {
					result = append(result, Package{Name: name})
				}
			}
		}
	}
	return result, nil
}

// Describe returns one-line descriptions for the names the caller asked for, and
// reports which of them match no formula or cask.
//
// `brew info` fails as a whole when any single argument is not a known formula or
// cask, so a package that does not exist -- a Linux-only name like man or sudo
// declared in a shared list, or a typo like github-cli for gh -- would otherwise
// blank the description of every other package. Filtering the arguments against the
// known-name lists first means an unresolvable name costs only its own description,
// and identifies the bad names so they can be reported as such.
//
// The name lists come from brew's local cache and cover tap-qualified spellings, so
// this costs no network access and little time. They carry no aliases, so an alias
// of an installed formula is looked up under the name it is installed as -- see
// canonicalOptName. An alias of a formula that is not installed still reports as
// unknown: nothing local knows the name at that point.
func (b *BrewManager) Describe(pkgs []string) (Descriptions, error) {
	if len(pkgs) == 0 {
		return Descriptions{}, nil
	}

	known := newBrewIndex(append(listNames("formulae"), listNames("casks")...))
	prefix := brewPrefix()
	query := make([]string, 0, len(pkgs))
	unknown := make(map[string]bool)
	// aliases maps the name brew answers under to the name the caller asked about,
	// so a description looked up by one is returned for the other.
	aliases := make(map[string]string)
	for _, name := range pkgs {
		if known.has(name) {
			query = append(query, name)
			continue
		}
		// The name lists hold no aliases, so an installed alias such as kubectl looks
		// unresolvable. Reporting <unknown> for a package already marked installed
		// contradicts itself, so ask what it is installed as first.
		if canonical := canonicalOptName(prefix, name); canonical != "" && known.has(canonical) {
			query = append(query, canonical)
			aliases[canonical] = name
			continue
		}
		unknown[name] = true
	}

	// brew info with no arguments dumps the entire catalogue, which is slow and
	// useless here, so skip the call when nothing resolved.
	if len(query) == 0 {
		return Descriptions{Unknown: unknown}, nil
	}

	args := append([]string{"info", "--json=v2"}, query...)
	cmd := exec.Command("brew", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Descriptions{}, err
	}

	byName, err := parseBrewDescriptions(out.Bytes())
	if err != nil {
		return Descriptions{}, err
	}
	for canonical, declared := range aliases {
		byName[declared] = byName[canonical]
	}
	return Descriptions{ByName: byName, Unknown: unknown}, nil
}

// parseBrewDescriptions maps `brew info --json=v2` output to descriptions keyed by
// every name the package answers to.
//
// brew reports a tap formula as name=hermes with
// full_name=jamf/internal-tap/hermes, and a package may be declared either way, so
// keying on only one spelling leaves tap packages with a blank description.
//
// A package brew knows but has no description for (desc is null, common for font
// casks) is still recorded, with an empty value. Callers distinguish "no
// description" from "no such package" by key presence, so dropping the key would
// make a real package look unresolvable.
func parseBrewDescriptions(data []byte) (map[string]string, error) {
	var info struct {
		Formulae []struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Desc     string `json:"desc"`
		} `json:"formulae"`
		Casks []struct {
			Token     string `json:"token"`
			FullToken string `json:"full_token"`
			Desc      string `json:"desc"`
		} `json:"casks"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	put := func(name, fullName, desc string) {
		if name != "" {
			result[name] = desc
		}
		if fullName != "" {
			result[fullName] = desc
		}
	}
	for _, f := range info.Formulae {
		put(f.Name, f.FullName, f.Desc)
	}
	for _, c := range info.Casks {
		put(c.Token, c.FullToken, c.Desc)
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
