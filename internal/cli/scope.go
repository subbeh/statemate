package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/scripts"
)

// scopeFlagName is the long flag shared by apply, status, and diff. It matches
// the flag mate add already uses for the same concept.
const scopeFlagName = "source"

// addScopeFlag registers --source/-s on a command.
func addScopeFlag(cmd *cobra.Command) {
	cmd.Flags().StringP(scopeFlagName, "s", "", "limit to a single source")
	_ = cmd.RegisterFlagCompletionFunc(scopeFlagName, completeSources)
}

// scopeFrom reads the scope from a command's positional args and --source flag,
// rejecting the contradictory combination of both.
func scopeFrom(cmd *cobra.Command, args []string) (plan.Scope, error) {
	var s plan.Scope
	if len(args) > 0 {
		s.Path = args[0]
	}
	s.Source, _ = cmd.Flags().GetString(scopeFlagName)

	if s.Path != "" && s.Source != "" {
		return plan.Scope{}, fmt.Errorf("cannot combine a path (%s) with --source (%s); use one or the other", s.Path, s.Source)
	}
	return s, nil
}

// scopedScripts narrows which scripts an apply may run.
//
//   - No scope: everything, as before.
//   - File scope: nothing. Deploying one file should not trigger lifecycle hooks.
//   - Source scope: only that source's scripts. Repo-root scripts are excluded
//     because they apply to the whole repository, so running them for a single
//     source would overreach.
func scopedScripts(all scripts.Scripts, scope plan.Scope) scripts.Scripts {
	switch {
	case scope.Path != "":
		return nil
	case scope.Source != "":
		var kept scripts.Scripts
		for _, s := range all {
			if s.SourceDir != "" && filepath.Base(s.SourceDir) == scope.Source {
				kept = append(kept, s)
			}
		}
		return kept
	default:
		return all
	}
}

// scopedPackages narrows which missing packages an apply may install.
//
// File scope installs nothing; source scope installs only packages contributed
// by that source's .mate.yaml. PackageStatus.Sources records the contributing
// source, so no restructuring is needed.
func scopedPackages(statuses []packages.PackageStatus, scope plan.Scope) []packages.PackageStatus {
	switch {
	case scope.Path != "":
		return nil
	case scope.Source != "":
		var kept []packages.PackageStatus
		for _, st := range statuses {
			for _, src := range st.Sources {
				if src == scope.Source {
					kept = append(kept, st)
					break
				}
			}
		}
		return kept
	default:
		return statuses
	}
}

// missingNames returns the names of statuses that are not installed.
func missingNames(statuses []packages.PackageStatus) []string {
	var names []string
	for _, st := range statuses {
		if st.Status == packages.StatusMissing {
			names = append(names, st.Name)
		}
	}
	return names
}
