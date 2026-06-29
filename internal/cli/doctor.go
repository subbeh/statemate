package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/packages"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check configuration and dependencies",
	Long:  "Verify that statemate is properly configured and dependencies are available",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Statemate Doctor")
	fmt.Println("================")
	fmt.Println()

	issues := 0

	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Printf("[ERROR] Configuration: %v\n", err)
		issues++
	} else {
		fmt.Println("[OK] Configuration loaded")

		if err := cfg.Validate(); err != nil {
			fmt.Printf("[ERROR] Configuration validation: %v\n", err)
			issues++
		} else {
			fmt.Println("[OK] Configuration valid")
		}

		for _, source := range cfg.AbsoluteSources() {
			if info, err := os.Stat(source); err != nil {
				fmt.Printf("[ERROR] Source directory missing: %s\n", source)
				issues++
			} else if !info.IsDir() {
				fmt.Printf("[ERROR] Source is not a directory: %s\n", source)
				issues++
			} else {
				fmt.Printf("[OK] Source directory: %s\n", source)
			}
		}
	}

	fmt.Println()
	fmt.Println("Dependencies:")

	if _, err := exec.LookPath("age"); err != nil {
		fmt.Println("[WARN] age not found (encryption unavailable)")
	} else {
		fmt.Println("[OK] age")
	}

	if _, err := exec.LookPath("diff"); err != nil {
		fmt.Println("[WARN] diff not found (diff output unavailable)")
	} else {
		fmt.Println("[OK] diff")
	}

	fmt.Println()
	fmt.Println("Package Managers:")

	managers := packages.GetAvailableManagers()
	if len(managers) == 0 {
		fmt.Println("[WARN] No package managers found")
	} else {
		for _, m := range managers {
			fmt.Printf("[OK] %s\n", m.Name())
		}
	}

	fmt.Println()
	if issues > 0 {
		fmt.Printf("Found %d issue(s)\n", issues)
		os.Exit(1)
	} else {
		fmt.Println("No issues found")
	}

	return nil
}
