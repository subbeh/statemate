package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/plan"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/util"
)

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Manage packages",
	Long:  "Declarative package management across package managers",
}

var packagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List packages and whether they are installed",
	Long: `List declared packages across configured package managers, and whether each
is installed. Each manager's packages are sorted by name.

Packages can be defined in:
  - mate.yaml packages (global packages)
  - mate.yaml profiles.<name>.packages, for the active profile and every
    profile it extends
  - <source>/.mate.yaml packages, for each active source
  - Files listed under 'include', either top-level (global) or in a
    profile's 'include' (part of that profile)

The SOURCE column shows where each package is declared: config, profile:<name>,
or the source directory.

Use --all to show extra packages not in config. Detecting extras means listing
every installed package, which is noticeably slower, so it is only done when
--all is given.
Use --descriptions to show package descriptions. A description of <unknown> means the
package manager does not recognise the name at all, usually a typo or a package
that only exists on another platform; an empty description means the package
exists but publishes none.`,
	RunE: runPackagesList,
}

var packagesApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Install missing packages",
	Long: `Install packages that are declared but not installed.

Missing packages are listed per package manager, and each manager's install is
confirmed separately with [y/N]. Use -y/--yes (or --noconfirm) to confirm all of them.

Use --prune to also remove installed packages that are not in config. Removals
are confirmed the same way, and -y/--yes confirms them too.`,
	RunE: runPackagesApply,
}

var (
	packagesAutoConfirm  bool
	packagesShowAll      bool
	packagesDescriptions bool
	packagesPrune        bool
)

func init() {
	rootCmd.AddCommand(packagesCmd)
	packagesCmd.AddCommand(packagesListCmd)
	packagesCmd.AddCommand(packagesApplyCmd)

	packagesListCmd.Flags().BoolVar(&packagesShowAll, "all", false, "also show extra packages not in config")
	packagesListCmd.Flags().BoolVarP(&packagesDescriptions, "descriptions", "d", false, "show package descriptions")
	packagesApplyCmd.Flags().BoolVarP(&packagesAutoConfirm, "yes", "y", false, "auto-confirm all changes")
	packagesApplyCmd.Flags().BoolVar(&packagesAutoConfirm, "noconfirm", false, "same as --yes")
	packagesApplyCmd.Flags().BoolVar(&packagesPrune, "prune", false, "remove packages not in config (dangerous)")
}

func runPackagesList(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	env, err := plan.FromConfig(cfg, profileName)
	if err != nil {
		return err
	}
	tmplCtx := env.TmplCtx

	sources := profile.ResolveSources(cfg, profileName)
	results, err := packages.ComputeSync(cfg, profileName, cfg.ResolveSourcePaths(sources),
		packages.WithVerbose(packagesDescriptions), packages.WithExtras(packagesShowAll),
		packages.WithDirConfigRenderer(plan.DirConfigRenderer(tmplCtx)))
	if err != nil {
		return fmt.Errorf("computing sync: %w", err)
	}

	if len(results) == 0 {
		managers := packages.GetAvailableManagersWithHelper(cfg.AURHelper)
		if len(managers) == 0 {
			fmt.Println("No package managers available")
		} else {
			fmt.Println("No packages configured")
		}
		return nil
	}

	var data [][]string
	for _, result := range results {
		for _, s := range result.Statuses {
			if s.Status == packages.StatusExtra && !packagesShowAll {
				continue
			}

			var indicator string
			switch s.Status {
			case packages.StatusInstalled:
				indicator = color.GreenString("✓")
			case packages.StatusMissing:
				indicator = color.RedString("+")
			case packages.StatusExtra:
				indicator = color.YellowString("-")
			}

			source := strings.Join(s.Sources, ", ")
			row := []string{indicator, s.Name, result.Manager, source}
			if packagesDescriptions {
				// A package the manager does not recognise is called out, since a
				// blank description would look like the package merely has none --
				// hiding a name that can never be installed, such as a typo or a
				// Linux-only package declared for every platform.
				desc := s.Description
				if s.DescriptionUnknown {
					desc = color.YellowString("<unknown>")
				}
				row = append(row, desc)
			}
			data = append(data, row)
		}
	}

	header := []string{"", "PACKAGE", "MANAGER", "SOURCE"}
	alignment := tw.Alignment{tw.AlignCenter, tw.AlignLeft, tw.AlignLeft, tw.AlignLeft}
	if packagesDescriptions {
		header = append(header, "DESCRIPTION")
		alignment = append(alignment, tw.AlignLeft)
	}

	var buf bytes.Buffer
	table := tablewriter.NewTable(&buf,
		tablewriter.WithHeader(header),
		tablewriter.WithAlignment(alignment),
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.BorderNone,
			Settings: tw.Settings{
				Separators: tw.SeparatorsNone,
				Lines:      tw.LinesNone,
			},
		}),
	)
	_ = table.Bulk(data)
	_ = table.Render()

	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		fmt.Println(strings.TrimRight(scanner.Text(), " "))
	}

	// Reporting how many extras exist would mean listing every installed package,
	// which is the slow half of a sync (about a second for brew). The hint is
	// static so the common case stays fast.
	if !packagesShowAll {
		fmt.Println("\nUse --all to also show packages not in config.")
	}

	return nil
}

func runPackagesApply(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	env, err := plan.FromConfig(cfg, profileName)
	if err != nil {
		return err
	}
	tmplCtx := env.TmplCtx

	sources := profile.ResolveSources(cfg, profileName)
	results, err := packages.ComputeSync(cfg, profileName, cfg.ResolveSourcePaths(sources),
		packages.WithExtras(packagesPrune), packages.WithDirConfigRenderer(plan.DirConfigRenderer(tmplCtx)))
	if err != nil {
		return fmt.Errorf("computing sync: %w", err)
	}

	if len(results) == 0 {
		managers := packages.GetAvailableManagersWithHelper(cfg.AURHelper)
		if len(managers) == 0 {
			fmt.Println("No package managers available")
		} else {
			fmt.Println("No packages configured")
		}
		return nil
	}

	anyChanges := false
	for _, result := range results {
		if len(result.Missing()) > 0 || (packagesPrune && len(result.Extra()) > 0) {
			anyChanges = true
			break
		}
	}

	if !anyChanges {
		fmt.Println(color.GreenString("All packages are in sync"))
		return nil
	}

	for _, result := range results {
		missing := result.Missing()
		extra := result.Extra()
		hasMissing := len(missing) > 0
		hasExtra := packagesPrune && len(extra) > 0

		if !hasMissing && !hasExtra {
			continue
		}

		manager, err := packages.GetManager(result.Manager, cfg.AURHelper)
		if err != nil {
			return err
		}

		fmt.Println("\n" + bold("=== "+result.Manager+" ==="))

		if hasMissing {
			fmt.Printf("Will install: %s\n", strings.Join(missing, ", "))
		}

		if hasExtra {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s Will remove: %s\n", yellow("WARNING:"), strings.Join(extra, ", "))
		}

		if !packagesAutoConfirm {
			prompt := "Continue? [y/N] "
			if hasExtra {
				prompt = "This will REMOVE packages. Continue? [y/N] "
			}
			ok, err := util.Confirm(prompt, false)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Skipped")
				continue
			}
		}

		if hasMissing {
			fmt.Printf("Installing %d packages...\n", len(missing))
			if err := manager.Install(missing); err != nil {
				return fmt.Errorf("installing packages: %w", err)
			}
			slog.Info("installed", "manager", manager.Name(), "packages", missing)
		}

		if hasExtra {
			fmt.Printf("Removing %d packages...\n", len(extra))
			if err := manager.Uninstall(extra); err != nil {
				return fmt.Errorf("removing packages: %w", err)
			}
			slog.Info("uninstalled", "manager", manager.Name(), "packages", extra)
		}

		fmt.Println(color.GreenString("Done"))
	}

	return nil
}
