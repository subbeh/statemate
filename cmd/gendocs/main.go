// Command gendocs writes the command reference under docs/commands/, one
// markdown file per command, generated from the cobra command tree.
//
// The output is committed, and CI regenerates it to check the tree still matches
// the help text (see .github/workflows/ci.yaml). That only works if generation is
// deterministic, which is why the auto-gen timestamp is disabled -- otherwise the
// footer date would change daily and every run would look like a drift.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/subbeh/statemate/internal/cli"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: gendocs <output-dir>")
		os.Exit(1)
	}
	outDir := os.Args[1]

	if err := run(outDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(outDir string) error {
	rootCmd := cli.RootCmd()
	disableAutoGenTag(rootCmd)

	// Stale files would otherwise linger after a command is renamed or removed,
	// and the drift check cannot see a file that generation never touches.
	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("clearing %s: %w", outDir, err)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", outDir, err)
	}

	if err := doc.GenMarkdownTreeCustom(rootCmd, outDir, filePrepender, linkHandler); err != nil {
		return fmt.Errorf("generating markdown: %w", err)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	fmt.Printf("Generated %d command pages in %s\n", len(entries), outDir)
	return nil
}

// disableAutoGenTag suppresses cobra's "Auto generated ... on <date>" footer for
// the whole tree, keeping output stable across days.
func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, sub := range cmd.Commands() {
		disableAutoGenTag(sub)
	}
}

func filePrepender(string) string { return "" }

// linkHandler points cross-references at sibling files in the same directory.
func linkHandler(name string) string {
	return strings.ToLower(name)
}
