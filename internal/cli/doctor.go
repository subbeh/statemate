package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
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

	if cfg != nil {
		issues += checkHooks(cmd)
	}

	fmt.Println()
	fmt.Println("Dependencies:")

	if cfg != nil {
		level, msg := ageStatus(cfg)
		fmt.Printf("[%s] %s\n", level, msg)
		if level == "ERROR" {
			issues++
		}
	}

	if _, err := exec.LookPath("diff"); err != nil {
		fmt.Println("[WARN] diff not found (diff output unavailable)")
	} else {
		fmt.Println("[OK] diff")
	}

	fmt.Println()
	fmt.Println("Package Managers:")

	aurHelper := ""
	if cfg != nil {
		aurHelper = cfg.AURHelper
	}
	managers := packages.GetAvailableManagersWithHelper(aurHelper)
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

// checkHooks reports hooks that fail to load, and warns about active hooks
// whose patterns match none of the managed files -- usually a typo.
func checkHooks(cmd *cobra.Command) int {
	env, err := loadHookEnv(cmd)
	if err != nil {
		fmt.Println()
		fmt.Println("Hooks:")
		fmt.Printf("[ERROR] %v\n", err)
		return 1
	}
	if len(env.set) == 0 {
		return 0
	}

	fmt.Println()
	fmt.Println("Hooks:")

	matched := make(map[string]bool)
	for _, t := range env.set.Trigger(hookChanges(env.files, env.sourcePaths), env.profileChain) {
		matched[t.Name] = true
	}
	for _, h := range env.set {
		switch {
		case !h.IsEnabled(), !h.ActiveFor(env.profileChain):
			continue
		case matched[h.Name]:
			fmt.Printf("[OK] %s\n", h.Name)
		default:
			fmt.Printf("[WARN] %s matches no managed files\n", h.Name)
		}
	}
	return 0
}

// ageStatus reports whether encryption works with the configured keys. age is
// built into mate, so the age binary being installed or not says nothing; what
// matters is whether the identity loads and both halves are present.
func ageStatus(cfg *config.Config) (level, msg string) {
	if cfg.Age == nil || (cfg.Age.Identity == "" && cfg.Age.IdentityCommand == "" && len(cfg.Age.Recipients) == 0) {
		return "OK", "age: not configured (no #encrypted files or secrets cache)"
	}
	enc, err := encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
	if err != nil {
		return "ERROR", fmt.Sprintf("age: %v", err)
	}
	switch {
	case !enc.CanDecrypt():
		return "WARN", "age: recipients but no identity (#encrypted files cannot be deployed, secrets cannot be fetched)"
	case !enc.CanEncrypt():
		return "WARN", "age: identity but no recipients (files cannot be encrypted)"
	}
	return "OK", "age: identity and recipients"
}
