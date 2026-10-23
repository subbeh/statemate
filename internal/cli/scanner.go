package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/template"
	"github.com/subbeh/statemate/internal/util"
)

// newEnv resolves the Env from the command's --config and --profile flags.
func newEnv(cmd *cobra.Command) (*plan.Env, error) {
	cfgPath, _ := cmd.Flags().GetString("config")
	profileName, _ := cmd.Flags().GetString("profile")
	return plan.NewEnv(cfgPath, profileName)
}

func newScanner(cfg *config.Config, profileName string) (*source.Scanner, error) {
	env, err := plan.FromConfig(cfg, profileName)
	if err != nil {
		return nil, err
	}
	return env.Scanner, nil
}

// newTemplateContext builds a rendering context with decryption and secret
// lookups wired up where they are configured.
func newTemplateContext(cfg *config.Config, profileName string) (*template.Context, error) {
	env, err := plan.FromConfig(cfg, profileName)
	if err != nil {
		return nil, err
	}
	return env.TmplCtx, nil
}

// printConflicts lists the targets that more than one source deploys, headed
// by label.
func printConflicts(tree *source.Tree, label string) {
	fmt.Fprintf(os.Stderr, "%s conflicting targets detected\n", label)
	for _, c := range tree.Conflicts {
		fmt.Fprintf(os.Stderr, "  %s defined in:\n", util.ShortenPath(c.TargetPath))
		for _, s := range c.Sources {
			fmt.Fprintf(os.Stderr, "    - %s\n", util.ShortenPath(s))
		}
	}
}
