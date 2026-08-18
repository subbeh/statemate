package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// Commands that resolve their argument as an ordinary filesystem path must let the
// shell complete it. Returning a computed list sets NoFileComp, which suppresses
// shell completion entirely -- so a perfectly valid argument like
// ../../../.mate/theme.yaml could not be completed even though the command accepts
// it. That was the bug: `mate encrypt` offered almost nothing.
func TestPathCommandsDelegateCompletionToShell(t *testing.T) {
	// Every command whose argument is resolved by path, not looked up in the
	// managed-file tree.
	pathCommands := []string{"edit", "encrypt", "decrypt", "eval", "rename", "cat"}

	for _, name := range pathCommands {
		t.Run(name, func(t *testing.T) {
			cmd := findCommand(t, name)

			if cmd.ValidArgsFunction == nil {
				t.Fatalf("%s has no ValidArgsFunction", name)
			}

			completions, directive := cmd.ValidArgsFunction(cmd, nil, "")

			if directive != cobra.ShellCompDirectiveDefault {
				t.Errorf("directive = %v, want ShellCompDirectiveDefault so the shell completes paths", directive)
			}
			if len(completions) != 0 {
				t.Errorf("expected no computed completions, got %v", completions)
			}
		})
	}
}

// Commands that match against managed files keep their computed lists: those
// arguments are filters over tracked files, not arbitrary paths.
func TestManagedFileCommandsComputeCompletions(t *testing.T) {
	for _, name := range []string{"status", "diff", "apply", "forget", "clean", "managed"} {
		t.Run(name, func(t *testing.T) {
			cmd := findCommand(t, name)
			if cmd.ValidArgsFunction == nil {
				t.Fatalf("%s has no ValidArgsFunction", name)
			}

			// Called with an argument already present, these return NoFileComp
			// rather than deferring to the shell. Checking that shape avoids
			// depending on a real config being loadable in the test environment.
			_, directive := cmd.ValidArgsFunction(cmd, []string{"already"}, "")
			if directive == cobra.ShellCompDirectiveDefault {
				t.Errorf("%s should not delegate to shell file completion", name)
			}
		})
	}
}

// The second argument to rename is a new name, not an existing file, so offering
// filesystem completions there would suggest paths that must not be used.
func TestRenameSecondArgumentIsNotCompleted(t *testing.T) {
	cmd := findCommand(t, "rename")

	_, directive := cmd.ValidArgsFunction(cmd, []string{"nvim/init.lua"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp for the <new-name> argument", directive)
	}
}

func findCommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, c := range RootCmd().Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("command %q not found", name)
	return nil
}
