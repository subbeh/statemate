package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/util"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new statemate repository",
	Long:  "Create a minimal mate.yaml or mate.toml configuration file with comments",
	RunE:  runInit,
}

var initFormat string

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initFormat, "format", "", "config format: yaml or toml")
}

const defaultConfigYAML = `# Statemate configuration
# See: https://github.com/subbeh/statemate

# Source directories to manage (stow-style)
# Each directory's contents are deployed relative to target_base
# Example: sources: [nvim, zsh, git] will look for nvim/, zsh/, git/ in this directory
sources: []

# Default source for 'mate add' (optional)
# default_source: ""

# Default target for deployed files (default: $HOME)
target_base: "~"

# Profiles for machine-specific configuration
# profiles:
#   work:
#     detection:
#       hostname: "work-*"
#     packages:
#       brew: [slack]
#
#   personal:
#     detection:
#       hostname: ["macbook-*", "desktop-*"]

# Age encryption for sensitive files
# age:
#   identity: "~/.config/statemate/key.txt"
#   recipients:
#     - "age1..."

# Template variables
# variables:
#   email: "you@example.com"
#   editor: "nvim"

# Global packages (installed regardless of profile)
# packages:
#   brew:
#     - ripgrep
#     - fd
`

const defaultConfigTOML = `# Statemate configuration
# See: https://github.com/subbeh/statemate

# Source directories to manage (stow-style)
# Each directory's contents are deployed relative to target_base
# Example: sources = ["nvim", "zsh", "git"] will look for nvim/, zsh/, git/ in this directory
sources = []

# Default source for 'mate add' (optional)
# default_source = ""

# Default target for deployed files (default: $HOME)
target_base = "~"

# Profiles for machine-specific configuration
# [profiles.work]
# [profiles.work.detection]
# hostname = "work-*"
# [profiles.work.packages]
# brew = ["slack"]

# [profiles.personal]
# [profiles.personal.detection]
# hostname = ["macbook-*", "desktop-*"]

# Age encryption for sensitive files
# [age]
# identity = "~/.config/statemate/key.txt"
# recipients = ["age1..."]

# Template variables
# [variables]
# email = "you@example.com"
# editor = "nvim"

# Global packages (installed regardless of profile)
# [packages]
# brew = ["ripgrep", "fd"]
`

const defaultReadme = `# Dotfiles

Managed with [statemate](https://github.com/subbeh/statemate).

## Setup on a new machine

1. Install statemate:
   ` + "```" + `sh
   # macOS
   brew install subbeh/tap/statemate

   # Arch Linux
   yay -S statemate
   ` + "```" + `

2. Clone this repository:
   ` + "```" + `sh
   git clone <your-repo-url> ~/.dotfiles
   cd ~/.dotfiles
   ` + "```" + `

3. Register and apply:
   ` + "```" + `sh
   mate init    # Register this directory
   mate apply   # Deploy configuration
   ` + "```" + `
`

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	format := initFormat

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	configExists := false
	var existingConfigPath string
	if _, err := os.Stat("mate.yaml"); err == nil {
		configExists = true
		existingConfigPath = "mate.yaml"
	} else if _, err := os.Stat("mate.toml"); err == nil {
		configExists = true
		existingConfigPath = "mate.toml"
	}

	if configExists {
		return handleExistingRepo(cwd, existingConfigPath)
	}

	if format == "" {
		fmt.Print("Config format [yaml/toml] (default: yaml): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		format = strings.TrimSpace(strings.ToLower(input))
		if format == "" {
			format = "yaml"
		}
	}

	var configPath, content string
	switch format {
	case "yaml", "yml":
		configPath = "mate.yaml"
		content = defaultConfigYAML
	case "toml":
		configPath = "mate.toml"
		content = defaultConfigTOML
	default:
		return fmt.Errorf("unknown format %q, use yaml or toml", format)
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Created %s\n", filepath.Join(cwd, configPath))

	if err := os.WriteFile("README.md", []byte(defaultReadme), 0644); err != nil {
		return fmt.Errorf("writing README: %w", err)
	}
	fmt.Printf("Created %s\n", filepath.Join(cwd, "README.md"))

	if err := initGitRepo(); err != nil {
		return err
	}

	if err := registerSourceDir(reader, cwd); err != nil {
		return err
	}

	fmt.Println("\nNext steps:")
	fmt.Printf("  1. Add source directories to %s (e.g., sources: [nvim, zsh])\n", configPath)
	fmt.Println("  2. Add files: mate add ~/.config/nvim/init.lua")
	fmt.Println("  3. Check status: mate status")
	fmt.Println("  4. Apply changes: mate apply")

	return nil
}

func handleExistingRepo(cwd, configPath string) error {
	fmt.Printf("Found existing config: %s\n", configPath)

	if localConfigExists() && config.SourceDir() == cwd {
		fmt.Println("This directory is already registered as your dotfiles location.")
		fmt.Println("\nRun 'mate apply' to apply your configuration.")
		return nil
	}

	if err := config.SaveLocalSourceDir(cwd); err != nil {
		return fmt.Errorf("saving local config: %w", err)
	}
	fmt.Printf("Registered in %s\n", util.ShortenPath(config.LocalConfigPath()))

	fmt.Println("\nRun 'mate apply' to apply your configuration.")
	return nil
}

func registerSourceDir(reader *bufio.Reader, cwd string) error {
	existingSourceDir := config.SourceDir()
	if existingSourceDir == cwd {
		return nil
	}

	if localConfigExists() {
		fmt.Printf("\nLocal config already points to: %s\n", util.ShortenPath(existingSourceDir))
		fmt.Printf("Update to this directory instead? [y/N]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
			if err := config.SaveLocalSourceDir(cwd); err != nil {
				return fmt.Errorf("saving local config: %w", err)
			}
			fmt.Printf("Updated %s\n", util.ShortenPath(config.LocalConfigPath()))
		}
	} else {
		fmt.Printf("\nRegister this directory as your dotfiles location? [Y/n]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "" || input == "y" || input == "yes" {
			if err := config.SaveLocalSourceDir(cwd); err != nil {
				return fmt.Errorf("saving local config: %w", err)
			}
			fmt.Printf("Saved to %s\n", util.ShortenPath(config.LocalConfigPath()))
		}
	}
	return nil
}

func localConfigExists() bool {
	_, err := os.Stat(config.LocalConfigPath())
	return err == nil
}

func initGitRepo() error {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("initializing git repository: %w", err)
	}
	fmt.Println("Initialized git repository")
	return nil
}
