package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/packages"
	"github.com/subbeh/statemate/internal/profile"
)

var packagesCmd = &cobra.Command{
	Use:   "packages",
	Short: "Manage packages",
	Long:  "Declarative package management across package managers",
}

var packagesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show package sync status",
	Long:  "Show which packages are missing. Use --all to also show extra packages not in config.",
	RunE:  runPackagesStatus,
}

var packagesApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Sync packages",
	Long:  "Install missing packages. Use --prune to also remove packages not in config.",
	RunE:  runPackagesApply,
}

var (
	packagesAutoConfirm bool
	packagesShowAll     bool
	packagesPrune       bool
)

func init() {
	rootCmd.AddCommand(packagesCmd)
	packagesCmd.AddCommand(packagesStatusCmd)
	packagesCmd.AddCommand(packagesApplyCmd)

	packagesStatusCmd.Flags().BoolVar(&packagesShowAll, "all", false, "also show extra packages not in config")
	packagesApplyCmd.Flags().BoolVarP(&packagesAutoConfirm, "yes", "y", false, "auto-confirm all changes")
	packagesApplyCmd.Flags().BoolVar(&packagesPrune, "prune", false, "remove packages not in config (dangerous)")
}

func runPackagesStatus(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	results, err := packages.ComputeSync(cfg, profileName, cfg.AbsoluteSources())
	if err != nil {
		return fmt.Errorf("computing sync: %w", err)
	}

	if len(results) == 0 {
		managers := packages.GetAvailableManagers()
		if len(managers) == 0 {
			fmt.Println("No package managers available")
		} else {
			fmt.Println("No packages configured")
		}
		return nil
	}

	for _, result := range results {
		fmt.Printf("\n=== %s ===\n", result.Manager)

		if len(result.Missing) > 0 {
			fmt.Printf("Missing (%d):\n", len(result.Missing))
			for _, p := range result.Missing {
				fmt.Printf("  + %s\n", p)
			}
		}

		if packagesShowAll && len(result.Extra) > 0 {
			fmt.Printf("Extra (%d):\n", len(result.Extra))
			for _, p := range result.Extra {
				fmt.Printf("  - %s\n", p)
			}
		}

		installed := 0
		for _, s := range result.Statuses {
			if s.Status == packages.StatusInstalled {
				installed++
			}
		}
		if installed > 0 {
			fmt.Printf("Installed: %d packages\n", installed)
		}

		if len(result.Missing) == 0 {
			if packagesShowAll && len(result.Extra) > 0 {
				fmt.Printf("(%d extra packages not in config)\n", len(result.Extra))
			} else if !packagesShowAll && len(result.Extra) > 0 {
				fmt.Printf("All configured packages installed (%d extra, use --all to show)\n", len(result.Extra))
			} else {
				fmt.Println("All packages in sync")
			}
		}
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

	results, err := packages.ComputeSync(cfg, profileName, cfg.AbsoluteSources())
	if err != nil {
		return fmt.Errorf("computing sync: %w", err)
	}

	if len(results) == 0 {
		managers := packages.GetAvailableManagers()
		if len(managers) == 0 {
			fmt.Println("No package managers available")
		} else {
			fmt.Println("No packages configured")
		}
		return nil
	}

	anyChanges := false
	for _, result := range results {
		if len(result.Missing) > 0 || (packagesPrune && len(result.Extra) > 0) {
			anyChanges = true
			break
		}
	}

	if !anyChanges {
		fmt.Println("All packages are in sync")
		return nil
	}

	for _, result := range results {
		hasMissing := len(result.Missing) > 0
		hasExtra := packagesPrune && len(result.Extra) > 0

		if !hasMissing && !hasExtra {
			continue
		}

		manager, err := packages.GetManager(result.Manager, cfg.AURHelper)
		if err != nil {
			return err
		}

		fmt.Printf("\n=== %s ===\n", result.Manager)

		if hasMissing {
			fmt.Printf("Will install: %s\n", strings.Join(result.Missing, ", "))
		}

		if hasExtra {
			yellow := color.New(color.FgYellow).SprintFunc()
			fmt.Printf("%s Will remove: %s\n", yellow("WARNING:"), strings.Join(result.Extra, ", "))
		}

		if !packagesAutoConfirm {
			prompt := "Continue? [y/N] "
			if hasExtra {
				prompt = "This will REMOVE packages. Continue? [y/N] "
			}
			fmt.Print(prompt)
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Skipped")
				continue
			}
		}

		if hasMissing {
			fmt.Printf("Installing %d packages...\n", len(result.Missing))
			if err := manager.Install(result.Missing); err != nil {
				return fmt.Errorf("installing packages: %w", err)
			}
		}

		if hasExtra {
			fmt.Printf("Removing %d packages...\n", len(result.Extra))
			if err := manager.Uninstall(result.Extra); err != nil {
				return fmt.Errorf("removing packages: %w", err)
			}
		}

		fmt.Println("Done")
	}

	return nil
}
