package packages

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/subbeh/statemate/internal/config"
)

type SyncResult struct {
	Manager  string
	Statuses []PackageStatus

	// extrasComputed records whether extras were looked for at all, so Extra()
	// can distinguish "none installed" from "never asked".
	extrasComputed bool
}

type syncOptions struct {
	verbose bool
	extras  bool
}

type SyncOption func(*syncOptions)

func WithVerbose(v bool) SyncOption {
	return func(o *syncOptions) { o.verbose = v }
}

// WithExtras enables detection of installed packages that no source declares.
//
// It is off by default because it costs a full list of explicitly-installed
// packages from every manager, and `brew leaves` alone takes about a second --
// which used to dominate the runtime of `mate status` and `mate apply`, neither
// of which reports extras.
func WithExtras(v bool) SyncOption {
	return func(o *syncOptions) { o.extras = v }
}

func (r *SyncResult) Missing() []string {
	var result []string
	for _, s := range r.Statuses {
		if s.Status == StatusMissing {
			result = append(result, s.Name)
		}
	}
	return result
}

// Extra lists installed packages that no source declares. It is always empty
// unless ComputeSync was called with WithExtras.
func (r *SyncResult) Extra() []string {
	var result []string
	for _, s := range r.Statuses {
		if s.Status == StatusExtra {
			result = append(result, s.Name)
		}
	}
	return result
}

// ExtrasComputed reports whether extras were looked for, so a caller can tell an
// empty Extra() apart from one that was never populated.
func (r *SyncResult) ExtrasComputed() bool {
	return r.extrasComputed
}

// Indirection so tests can supply fake managers instead of shelling out to a
// real brew or pacman.
var (
	getManager       = GetManager
	availableManager = GetAvailableManagersWithHelper
)

func ComputeSync(cfg *config.Config, profileName string, sources []string, opts ...SyncOption) ([]SyncResult, error) {
	var o syncOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Collect packages with source tracking
	// key: "manager:pkgspec", value: list of sources
	type pkgEntry struct {
		manager string
		spec    string
		sources []string
	}
	entries := make(map[string]*pkgEntry) // key: "manager\x00name"

	addPkgs := func(pkgs *config.PackageList, source string) {
		if pkgs == nil {
			return
		}
		add := func(manager string, specs []string) {
			for _, spec := range specs {
				name, _ := ParsePackageSpec(spec)
				key := manager + "\x00" + name
				if e, ok := entries[key]; ok {
					e.sources = appendUnique(e.sources, source)
				} else {
					entries[key] = &pkgEntry{manager: manager, spec: spec, sources: []string{source}}
				}
			}
		}
		add("common", pkgs.Common)
		add("brew", pkgs.Brew)
		add("pacman", pkgs.Pacman)
		add("aur", pkgs.AUR)
	}

	// Global packages
	if cfg.Packages != nil {
		addPkgs(cfg.Packages, "config")
	}

	// Profile-specific packages
	if profileName != "" {
		if profile, ok := cfg.Profiles[profileName]; ok {
			if profile.Packages != nil {
				addPkgs(profile.Packages, "profile:"+profileName)
			}
		}
	}

	// Source directory packages
	for _, source := range sources {
		dirCfg, _ := config.LoadDirConfig(source)
		if dirCfg != nil && dirCfg.Packages != nil {
			addPkgs(dirCfg.Packages, filepath.Base(source))
		}
	}

	// Resolve common packages to the primary available manager
	primaryManager := getPrimaryManager(cfg.AURHelper)
	for key, e := range entries {
		if e.manager != "common" {
			continue
		}
		if primaryManager == "" {
			continue
		}
		name, _ := ParsePackageSpec(e.spec)
		targetKey := primaryManager + "\x00" + name
		if existing, ok := entries[targetKey]; ok {
			for _, s := range e.sources {
				existing.sources = appendUnique(existing.sources, s)
			}
		} else {
			entries[targetKey] = &pkgEntry{manager: primaryManager, spec: e.spec, sources: e.sources}
		}
		delete(entries, key)
	}

	// Group by manager
	managerPkgs := make(map[string][]*pkgEntry)
	for _, e := range entries {
		managerPkgs[e.manager] = append(managerPkgs[e.manager], e)
	}

	// Ensure all available managers are included
	for _, m := range availableManager(cfg.AURHelper) {
		if _, ok := managerPkgs[m.Name()]; !ok {
			managerPkgs[m.Name()] = nil
		}
	}

	var results []SyncResult

	for managerName, pkgs := range managerPkgs {
		manager, err := getManager(managerName, cfg.AURHelper)
		if err != nil {
			continue
		}
		if !manager.IsAvailable() {
			continue
		}

		wantedMap := make(map[string]*pkgEntry)
		for _, e := range pkgs {
			name, _ := ParsePackageSpec(e.spec)
			wantedMap[name] = e
		}

		// Query wanted packages (includes deps, accurate check)
		wantedNames := make([]string, 0, len(wantedMap))
		for name := range wantedMap {
			wantedNames = append(wantedNames, name)
		}
		queried, err := manager.QueryInstalled(wantedNames)
		if err != nil {
			return nil, err
		}
		queriedMap := make(map[string]Package)
		for _, p := range queried {
			queriedMap[p.Name] = p
		}

		result := SyncResult{Manager: managerName, extrasComputed: o.extras}

		for name, e := range wantedMap {
			_, version := ParsePackageSpec(e.spec)
			if inst, ok := queriedMap[name]; ok {
				status := PackageStatus{
					Name:      name,
					Version:   version,
					Status:    StatusInstalled,
					Installed: inst.Version,
					Sources:   e.sources,
				}
				if version != "" && inst.Version != "" && inst.Version != version {
					status.Status = StatusVersionMismatch
				}
				result.Statuses = append(result.Statuses, status)
			} else {
				result.Statuses = append(result.Statuses, PackageStatus{
					Name:    name,
					Version: version,
					Status:  StatusMissing,
					Sources: e.sources,
				})
			}
		}

		// Listing every explicitly-installed package is the expensive part of a
		// sync (about a second for brew), so only do it when the caller wants
		// extras reported.
		if o.extras {
			installed, err := manager.ListInstalled()
			if err != nil {
				return nil, err
			}

			// Compare on the unqualified name as well. brew reports a tap formula
			// as jamf/internal-tap/hermes here but hermes elsewhere, so matching
			// only the exact string would list a declared package as an extra.
			wantedShort := make(map[string]bool, len(wantedMap))
			for name := range wantedMap {
				wantedShort[unqualifiedName(name)] = true
			}

			for _, inst := range installed {
				if _, ok := wantedMap[inst.Name]; ok {
					continue
				}
				if wantedShort[unqualifiedName(inst.Name)] {
					continue
				}
				result.Statuses = append(result.Statuses, PackageStatus{
					Name:      inst.Name,
					Status:    StatusExtra,
					Installed: inst.Version,
				})
			}
		}

		if o.verbose {
			var names []string
			for i := range result.Statuses {
				names = append(names, result.Statuses[i].Name)
			}
			if descs, err := manager.Describe(names); err == nil {
				for i := range result.Statuses {
					desc, resolved := descs.Lookup(result.Statuses[i].Name)
					result.Statuses[i].Description = desc
					result.Statuses[i].DescriptionUnknown = !resolved
				}
			}
		}

		sort.Slice(result.Statuses, func(i, j int) bool {
			iExtra := result.Statuses[i].Status == StatusExtra
			jExtra := result.Statuses[j].Status == StatusExtra
			if iExtra != jExtra {
				return !iExtra
			}
			si := strings.Join(result.Statuses[i].Sources, ",")
			sj := strings.Join(result.Statuses[j].Sources, ",")
			if si != sj {
				return si < sj
			}
			return result.Statuses[i].Name < result.Statuses[j].Name
		})

		results = append(results, result)
	}

	return results, nil
}

// getPrimaryManager returns the first available package manager for common packages
func getPrimaryManager(aurHelper string) string {
	managers := []Manager{
		NewBrewManager(),
		NewPacmanManager(),
	}

	for _, m := range managers {
		if m.IsAvailable() {
			return m.Name()
		}
	}
	return ""
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
