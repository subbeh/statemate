package scripts

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Frequency int

const (
	FreqOnce     Frequency = iota // run once ever
	FreqOnchange                  // run when content changes
	FreqAlways                    // run on every apply
	FreqDaily                     // run at most once per day
	FreqWeekly                    // run at most once per week
	FreqMonthly                   // run at most once per month
	FreqManual                    // manual only
)

func (f Frequency) String() string {
	switch f {
	case FreqOnce:
		return "once"
	case FreqOnchange:
		return "onchange"
	case FreqAlways:
		return "always"
	case FreqDaily:
		return "daily"
	case FreqWeekly:
		return "weekly"
	case FreqMonthly:
		return "monthly"
	case FreqManual:
		return "manual"
	default:
		return "unknown"
	}
}

type Timing int

const (
	TimingBefore Timing = iota // run before apply
	TimingAfter                // run after apply
)

func (t Timing) String() string {
	switch t {
	case TimingBefore:
		return "before"
	case TimingAfter:
		return "after"
	default:
		return "unknown"
	}
}

type Script struct {
	Path        string
	Name        string
	Frequency   Frequency
	Timing      Timing
	Template    bool
	Profile     string
	Order       int
	SourceDir   string
	ContentHash string
	Description string
}

func (s *Script) IsExecutable() bool {
	info, err := os.Stat(s.Path)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}

// Script naming format: <order>-<name>.<ext>#<freq>#<timing>[#template][#profile:<name>]
// Examples:
//   01-setup.sh#once#before
//   02-cleanup.sh#always#after
//   03-render.sh#onchange#before#template
//   04-init.sh#once#before#profile:arch
//   manual-task.sh (no attributes = manual)
var orderPattern = regexp.MustCompile(`^(\d+)-(.+)$`)

func ParseScriptName(filename string) (name string, freq Frequency, timing Timing, template bool, profile string, order int) {
	parts := strings.Split(filename, "#")
	nameWithOrder := parts[0]

	// Parse order prefix if present
	if matches := orderPattern.FindStringSubmatch(nameWithOrder); matches != nil {
		order, _ = strconv.Atoi(matches[1])
		name = matches[2]
	} else {
		name = nameWithOrder
	}

	// Default to manual
	freq = FreqManual
	timing = TimingBefore

	// Parse attributes
	for _, attr := range parts[1:] {
		lower := strings.ToLower(attr)
		switch {
		case lower == "once":
			freq = FreqOnce
		case lower == "onchange":
			freq = FreqOnchange
		case lower == "always":
			freq = FreqAlways
		case lower == "daily":
			freq = FreqDaily
		case lower == "weekly":
			freq = FreqWeekly
		case lower == "monthly":
			freq = FreqMonthly
		case lower == "before":
			timing = TimingBefore
		case lower == "after":
			timing = TimingAfter
		case lower == "template":
			template = true
		case strings.HasPrefix(lower, "profile:"):
			profile = strings.TrimPrefix(attr, "profile:")
		}
	}

	return name, freq, timing, template, profile, order
}

// ChangedSources names the source directories that have pending changes, and is
// what decides whether an #onchange script runs.
//
// Callers build this from the change set they already compute (mate status and
// mate apply both call ComputeChanges), so ShouldRun does not have to rescan the
// tree once per script.
type ChangedSources struct {
	// names holds the base name of each source directory with pending changes.
	names map[string]bool
}

// NewChangedSources builds a set from source directory paths or names.
func NewChangedSources(sources ...string) ChangedSources {
	c := ChangedSources{names: make(map[string]bool, len(sources))}
	for _, s := range sources {
		if s != "" {
			c.names[filepath.Base(s)] = true
		}
	}
	return c
}

// Has reports whether the given source directory has pending changes.
func (c ChangedSources) Has(sourceDir string) bool {
	if c.names == nil || sourceDir == "" {
		return false
	}
	return c.names[filepath.Base(sourceDir)]
}

// Any reports whether any source has pending changes. Repo-root scripts, which
// have no owning source, use this.
func (c ChangedSources) Any() bool {
	return len(c.names) > 0
}

type Scripts []*Script

func (s Scripts) Len() int      { return len(s) }
func (s Scripts) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s Scripts) Less(i, j int) bool {
	if s[i].Order != s[j].Order {
		return s[i].Order < s[j].Order
	}
	return s[i].Name < s[j].Name
}

func (s Scripts) Sort() {
	sort.Sort(s)
}

func (s Scripts) ByTiming(t Timing) Scripts {
	var result Scripts
	for _, script := range s {
		if script.Timing == t {
			result = append(result, script)
		}
	}
	return result
}

func (s Scripts) ByFrequency(f Frequency) Scripts {
	var result Scripts
	for _, script := range s {
		if script.Frequency == f {
			result = append(result, script)
		}
	}
	return result
}

func (s Scripts) Automatic() Scripts {
	var result Scripts
	for _, script := range s {
		if script.Frequency != FreqManual {
			result = append(result, script)
		}
	}
	return result
}

func (s Scripts) ByProfile(profileChain []string) Scripts {
	var result Scripts
	for _, script := range s {
		if script.Profile == "" || matchesProfile(script.Profile, profileChain) {
			result = append(result, script)
		}
	}
	return result
}

func matchesProfile(scriptProfile string, chain []string) bool {
	for _, p := range chain {
		if p == scriptProfile {
			return true
		}
	}
	return false
}
