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
	"path/filepath"
	"regexp"
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
	for _, e := range entries {
		path := filepath.Join(outDir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(polish(string(content))), 0644); err != nil {
			return err
		}
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

var (
	headingRe  = regexp.MustCompile(`^#{1,6} `)
	listItemRe = regexp.MustCompile(`^(- |\d+\. )`)
)

// polish turns cobra's markdown into something that renders as intended.
//
// The Synopsis is the Long help text verbatim, which is laid out for a terminal:
// two-space-indented blocks hold example commands, flag tables and YAML, and
// placeholders such as <path> appear in prose. Rendered as markdown, the blocks
// collapse into a single paragraph and the placeholders are swallowed as HTML
// tags. Fences cannot go in the Long text itself, because --help would print them,
// so indented runs are fenced here, except for runs that start with a list marker,
// which already render as lists. Headings are promoted one level so each page
// opens with an H1, which the docs site uses as the page title.
func polish(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	inFence, inSynopsis := false, false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "```"):
			inFence = !inFence
			inSynopsis = false
			out = append(out, line)
		case inFence:
			out = append(out, line)
		case headingRe.MatchString(line):
			inSynopsis = line == "### Synopsis"
			out = append(out, line[1:])
		case inSynopsis && isIndented(line):
			end := i
			for end < len(lines) && (isIndented(lines[end]) ||
				lines[end] == "" && end+1 < len(lines) && isIndented(lines[end+1])) {
				end++
			}
			block := dedent(lines[i:end])
			out = ensureBlank(out)
			if listItemRe.MatchString(block[0]) {
				for _, l := range block {
					out = append(out, escapeTags(l))
				}
			} else {
				out = append(out, "```")
				out = append(out, block...)
				out = append(out, "```")
			}
			if end < len(lines) && lines[end] != "" {
				out = append(out, "")
			}
			i = end - 1
		case inSynopsis:
			out = append(out, escapeTags(line))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func isIndented(line string) bool {
	return strings.HasPrefix(line, "  ")
}

// dedent removes the indent common to all non-blank lines.
func dedent(lines []string) []string {
	indent := -1
	for _, l := range lines {
		if l == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if l != "" {
			out[i] = l[indent:]
		}
	}
	return out
}

// ensureBlank separates a block from a preceding paragraph line, without which
// Python-Markdown treats a following list as part of the paragraph.
func ensureBlank(out []string) []string {
	if len(out) > 0 && out[len(out)-1] != "" {
		return append(out, "")
	}
	return out
}

func escapeTags(line string) string {
	return strings.ReplaceAll(line, "<", "&lt;")
}
