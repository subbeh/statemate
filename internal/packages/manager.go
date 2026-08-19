package packages

import (
	"fmt"
	"strings"
)

type Manager interface {
	Name() string
	IsAvailable() bool
	ListInstalled() ([]Package, error)
	QueryInstalled(pkgs []string) ([]Package, error)
	Describe(pkgs []string) (Descriptions, error)
	Install(pkgs []string) error
	Uninstall(pkgs []string) error
}

// Descriptions holds the one-line descriptions a manager could look up.
//
// Unknown is reported separately rather than inferred from a missing ByName entry,
// because absence means different things per manager: brew can say a name matches
// no formula or cask at all, while `pacman -Qi` only describes installed packages
// and says nothing about whether a missing one exists. Guessing from absence would
// label every not-yet-installed pacman package as nonexistent.
type Descriptions struct {
	ByName map[string]string

	// Unknown lists requested names the manager positively could not resolve.
	Unknown map[string]bool
}

// Lookup returns the description for a name and whether the manager resolved it at
// all. A resolved package with nothing to say about itself yields ("", true).
func (d Descriptions) Lookup(name string) (desc string, resolved bool) {
	if d.Unknown[name] {
		return "", false
	}
	desc, ok := d.ByName[name]
	if !ok {
		// Not resolved and not positively unknown: the manager had no opinion, so
		// treat it as a package with no description rather than a bad name.
		return "", true
	}
	return desc, true
}

// unqualifiedName strips a source prefix from a package name, turning
// jamf/internal-tap/hermes into hermes. Names without a prefix are returned
// unchanged, so this is a no-op for managers that have no equivalent of a tap.
//
// Homebrew is inconsistent about which form it prints -- `brew list --formula`
// gives the bare name while `brew leaves` gives the qualified one -- so both
// spellings have to compare equal.
func unqualifiedName(name string) string {
	if i := strings.LastIndex(name, "/"); i != -1 {
		return name[i+1:]
	}
	return name
}

type Package struct {
	Name        string
	Version     string
	Description string
}

func (p Package) String() string {
	if p.Version != "" {
		return p.Name + "@" + p.Version
	}
	return p.Name
}

type PackageStatus struct {
	Name        string
	Version     string
	Status      Status
	Sources     []string
	Installed   string
	Description string

	// DescriptionUnknown records that the package manager does not recognise this
	// name at all, as opposed to recognising it and having no description. Only set
	// when descriptions were requested.
	DescriptionUnknown bool
}

type Status int

const (
	StatusInstalled Status = iota
	StatusMissing
	StatusExtra
	StatusVersionMismatch
)

func (s Status) String() string {
	switch s {
	case StatusInstalled:
		return "installed"
	case StatusMissing:
		return "missing"
	case StatusExtra:
		return "extra"
	case StatusVersionMismatch:
		return "version mismatch"
	default:
		return "unknown"
	}
}

func ParsePackageSpec(spec string) (name, version string) {
	if idx := strings.Index(spec, "@"); idx != -1 {
		return spec[:idx], spec[idx+1:]
	}
	return spec, ""
}

func GetManager(name string, aurHelper string) (Manager, error) {
	switch name {
	case "brew":
		return NewBrewManager(), nil
	case "pacman":
		return NewPacmanManager(), nil
	case "aur":
		return NewAURManager(aurHelper), nil
	default:
		return nil, fmt.Errorf("unknown package manager: %s", name)
	}
}

func GetAvailableManagers() []Manager {
	return GetAvailableManagersWithHelper("")
}

func GetAvailableManagersWithHelper(aurHelper string) []Manager {
	managers := []Manager{
		NewBrewManager(),
		NewPacmanManager(),
		NewAURManager(aurHelper),
	}

	var available []Manager
	for _, m := range managers {
		if m.IsAvailable() {
			available = append(available, m)
		}
	}
	return available
}
