package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/scripts"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/target"
	"github.com/subbeh/statemate/internal/template"
)

var scriptsCmd = &cobra.Command{
	Use:   "scripts",
	Short: "Manage scripts",
	Long: `List and manage lifecycle scripts.

A script can describe itself with a comment in its first 10 lines:

  #!/usr/bin/env bash
  # Description: Bootstrap the development environment

The description is shown by 'scripts list', 'mate status', and the
confirmation prompt during apply. Matching is case-insensitive.

An '#onchange' script runs when its own source has pending changes -- the files
'mate status' lists for the source the script lives in. A script in the
repo-root .matescripts directory has no owning source, so any pending change
triggers it. Editing the script itself does not trigger it; use
'mate scripts run <name>' to run one on demand.`,
}

var scriptsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scripts",
	Long:  "List all discovered scripts and their status, including descriptions",
	RunE:  runScriptsList,
}

var scriptsRunCmd = &cobra.Command{
	Use:               "run <script>",
	Short:             "Run a script",
	Long:              "Manually run a script by name or path",
	Args:              cobra.ExactArgs(1),
	RunE:              runScript,
	ValidArgsFunction: completeScripts,
}

func init() {
	rootCmd.AddCommand(scriptsCmd)
	scriptsCmd.AddCommand(scriptsListCmd)
	scriptsCmd.AddCommand(scriptsRunCmd)

	scriptsRunCmd.Flags().Bool("dry-run", false, "show what would be done without running")
	scriptsRunCmd.Flags().BoolP("verbose", "v", false, "verbose output")
}

func runScriptsList(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	sourcePaths := cfg.ResolveSourcePaths(sources)

	discoverer := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths)
	allScripts, err := discoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering scripts: %w", err)
	}

	if len(allScripts) == 0 {
		fmt.Println("No scripts found")
		return nil
	}

	db, err := state.Open("")
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer func() { _ = db.Close() }()

	profileChain := profile.InheritanceChain(cfg, profileName)

	// #onchange scripts are scheduled by pending source changes, so compute them
	// here too -- otherwise this listing would disagree with what apply will run.
	// Failures are non-fatal: the listing degrades to "no source changes".
	var changed scripts.ChangedSources
	if scanner, err := newScanner(cfg, profileName); err == nil {
		if tree, err := scanner.Scan(sourcePaths); err == nil {
			if profileName != "" {
				tree = tree.FilterByProfile(profileChain)
			}
			if res, err := target.ComputeChanges(tree, db); err == nil {
				changed = changedSources(res.Changes)
			}
		}
	}

	var data [][]string

	for _, script := range allScripts {
		active := script.Profile == "" || matchesChain(script.Profile, profileChain)

		var status string
		if !active {
			status = "n/a"
		} else {
			lastRun, _ := db.GetScriptRun(script.Path)
			switch script.Frequency {
			case scripts.FreqOnce:
				if lastRun != nil {
					status = "done (" + lastRun.RunAt.Format("2006-01-02 15:04") + ")"
				} else {
					status = "pending"
				}
			case scripts.FreqOnchange:
				// Driven by pending changes to the script's source, so ask the
				// shared scheduler rather than duplicating the rule here.
				due, _, _ := scripts.ShouldRun(script, db, changed)
				switch {
				case due:
					status = "pending"
				case lastRun != nil:
					status = "unchanged (" + lastRun.RunAt.Format("2006-01-02 15:04") + ")"
				default:
					status = "unchanged"
				}
			default:
				if lastRun != nil {
					status = "ran " + lastRun.RunAt.Format("2006-01-02 15:04")
				}
			}
		}

		name := script.Name
		if script.Profile != "" {
			name += " [" + script.Profile + "]"
		}
		if script.Template {
			name += " [T]"
		}

		source := ""
		if script.SourceDir != "" {
			source = filepath.Base(script.SourceDir)
		}

		data = append(data, []string{
			script.Frequency.String(),
			script.Timing.String(),
			strconv.Itoa(script.Order),
			source,
			name,
			status,
			script.Description,
		})
	}

	// Fit the description to whatever the terminal leaves after the other
	// columns. Letting tablewriter enforce a global MaxWidth instead would spread
	// the shortfall across every column -- truncating names and shrinking the
	// description to a few characters -- so trim just this column here.
	//
	// With no terminal (piped, redirected, CI) nothing is truncated: descriptions
	// print in full rather than mangling data the caller is about to process.
	headers := []string{"FREQUENCY", "TIMING", "ORDER", "SOURCE", "NAME", "STATUS", "DESCRIPTION"}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			truncateDescriptions(data, descriptionBudget(data, headers, w))
		}
	}

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader(headers),
		// Truncate rather than wrap: wrapping would reintroduce the per-script
		// continuation lines this column replaced.
		tablewriter.WithRowAutoWrap(tw.WrapTruncate),
		tablewriter.WithAlignment(tw.Alignment{
			tw.AlignLeft, tw.AlignLeft, tw.AlignRight, tw.AlignLeft,
			tw.AlignLeft, tw.AlignLeft, tw.AlignLeft,
		}),
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
	return nil
}

// descriptionColumn is the index of the description in a scripts-list row.
const descriptionColumn = 6

// descriptionBudget returns how many characters the description column may use
// so the widest row still fits termWidth. Returns 0 when there is no room.
//
// tablewriter pads every column to its widest cell -- header included -- so the
// budget is the terminal minus those column widths and the padding around each
// cell.
func descriptionBudget(rows [][]string, headers []string, termWidth int) int {
	used := 0
	for i, h := range headers {
		if i == descriptionColumn {
			continue
		}
		w := len([]rune(h))
		for _, r := range rows {
			if i < len(r) {
				if n := len([]rune(r[i])); n > w {
					w = n
				}
			}
		}
		used += w
	}

	// Two padding characters per column, plus a leading and a trailing space.
	overhead := 2 + 2*len(headers)

	budget := termWidth - used - overhead
	if budget < 0 {
		return 0
	}
	return budget
}

// truncateDescriptions trims each description to budget runes, marking cut text
// with an ellipsis. A budget of 0 clears them entirely -- better than a column of
// bare ellipses.
func truncateDescriptions(rows [][]string, budget int) {
	for _, r := range rows {
		if len(r) <= descriptionColumn {
			continue
		}
		desc := []rune(r[descriptionColumn])
		if len(desc) == 0 {
			continue
		}
		switch {
		case budget <= 1:
			r[descriptionColumn] = ""
		case len(desc) > budget:
			r[descriptionColumn] = string(desc[:budget-1]) + "…"
		}
	}
}

func matchesChain(target string, chain []string) bool {
	for _, p := range chain {
		if p == target {
			return true
		}
	}
	return false
}

func runScript(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	sources := profile.ResolveSources(cfg, profileName)
	sourcePaths := cfg.ResolveSourcePaths(sources)

	discoverer := scripts.NewDiscoverer(cfg.SourceDir(), sourcePaths)
	allScripts, err := discoverer.Discover()
	if err != nil {
		return fmt.Errorf("discovering scripts: %w", err)
	}

	scriptArg := args[0]
	var script *scripts.Script

	for _, s := range allScripts {
		if s.Name == scriptArg || s.Path == scriptArg || filepath.Base(s.Path) == scriptArg {
			script = s
			break
		}
	}

	if script == nil {
		if info, err := os.Stat(scriptArg); err == nil && !info.IsDir() {
			contentHash, err := state.HashFile(scriptArg)
			if err != nil {
				return fmt.Errorf("hashing script: %w", err)
			}
			name, freq, timing, tmpl, prof, order := scripts.ParseScriptName(filepath.Base(scriptArg))
			script = &scripts.Script{
				Path:        scriptArg,
				Name:        name,
				Frequency:   freq,
				Timing:      timing,
				Template:    tmpl,
				Profile:     prof,
				Order:       order,
				ContentHash: contentHash,
			}
		}
	}

	if script == nil {
		return fmt.Errorf("script not found: %s", scriptArg)
	}

	db, err := state.Open("")
	if err != nil {
		return fmt.Errorf("opening state database: %w", err)
	}
	defer func() { _ = db.Close() }()

	var tmplCtx *template.Context
	if script.Template {
		tmplCtx, err = template.NewContext(cfg, profileName)
		if err != nil {
			return fmt.Errorf("creating template context: %w", err)
		}
	}

	executor := scripts.NewExecutor(db, tmplCtx, dryRun, verbose)
	if err := executor.ExecuteOne(script); err != nil {
		return fmt.Errorf("running script: %w", err)
	}

	return nil
}
