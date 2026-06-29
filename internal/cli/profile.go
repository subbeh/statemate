package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Show active profile",
	Long:  "Show which profile will be used and how it was determined",
	RunE:  runProfile,
}

func init() {
	rootCmd.AddCommand(profileCmd)
}

func runProfile(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profileFlag, _ := cmd.Flags().GetString("profile")

	if profileFlag != "" {
		fmt.Printf("Profile: %s\n", profileFlag)
		fmt.Println("Source:  --profile flag")
		return nil
	}

	if envProfile := os.Getenv("STATEMATE_PROFILE"); envProfile != "" {
		fmt.Printf("Profile: %s\n", envProfile)
		fmt.Println("Source:  STATEMATE_PROFILE environment variable")
		return nil
	}

	if cfg.Profile != "" {
		fmt.Printf("Profile: %s\n", cfg.Profile)
		fmt.Println("Source:  config file (profile field)")
		return nil
	}

	detected := profile.Detect(cfg)
	if detected != "" {
		fmt.Printf("Profile: %s\n", detected)
		fmt.Println("Source:  auto-detected")
		printDetectionReason(cfg, detected)
		return nil
	}

	fmt.Println("Profile: (none)")
	fmt.Println("Source:  no profile matched")

	if len(cfg.Profiles) > 0 {
		fmt.Println("\nConfigured profiles:")
		for name := range cfg.Profiles {
			fmt.Printf("  - %s\n", name)
		}
	}

	return nil
}

func printDetectionReason(cfg *config.Config, profileName string) {
	p, ok := cfg.Profiles[profileName]
	if !ok || p.Detection == nil {
		return
	}

	fmt.Println("\nMatched because:")
	d := p.Detection

	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	if d.Hostname != nil {
		fmt.Printf("  hostname: %s matches %v\n", hostname, d.Hostname)
	}
	if d.User != nil {
		fmt.Printf("  user: %s matches %v\n", username, d.User)
	}
	if d.OS != "" {
		fmt.Printf("  os: %s\n", d.OS)
	}
	if d.Arch != "" {
		fmt.Printf("  arch: %s\n", d.Arch)
	}
	if d.Command != "" {
		fmt.Printf("  command: %s (exit 0)\n", d.Command)
	}
}
