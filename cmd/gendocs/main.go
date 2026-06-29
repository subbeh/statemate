package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra/doc"
	"github.com/subbeh/statemate/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gendocs <output-dir>")
		fmt.Fprintln(os.Stderr, "       gendocs --man <output-dir>")
		fmt.Fprintln(os.Stderr, "       gendocs --markdown <output-dir>")
		os.Exit(1)
	}

	var genMan, genMarkdown bool
	var outDir string

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--man":
			genMan = true
		case "--markdown":
			genMarkdown = true
		default:
			outDir = arg
		}
	}

	if outDir == "" {
		fmt.Fprintln(os.Stderr, "Error: output directory required")
		os.Exit(1)
	}

	if !genMan && !genMarkdown {
		genMan = true
		genMarkdown = true
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	rootCmd := cli.RootCmd()

	if genMarkdown {
		mdDir := filepath.Join(outDir, "markdown")
		if err := os.MkdirAll(mdDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating markdown directory: %v\n", err)
			os.Exit(1)
		}

		if err := doc.GenMarkdownTree(rootCmd, mdDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating markdown: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Generated markdown docs in %s\n", mdDir)
	}

	if genMan {
		manDir := filepath.Join(outDir, "man")
		if err := os.MkdirAll(manDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating man directory: %v\n", err)
			os.Exit(1)
		}

		header := &doc.GenManHeader{
			Title:   "MATE",
			Section: "1",
			Date:    &time.Time{},
			Source:  "Statemate",
			Manual:  "User Commands",
		}
		now := time.Now()
		header.Date = &now

		if err := doc.GenManTree(rootCmd, header, manDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating man pages: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Generated man pages in %s\n", manDir)
	}
}
