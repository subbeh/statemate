package scripts

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Trigger int

const (
	TriggerOnce     Trigger = iota // run_once: run once ever
	TriggerOnchange                // run_onchange: run when content changes
	TriggerBefore                  // run_before: run before apply
	TriggerAfter                   // run_after: run after apply
	TriggerAlways                  // run_always: run on every apply
	TriggerManual                  // no prefix: manual only
)

func (t Trigger) String() string {
	switch t {
	case TriggerOnce:
		return "once"
	case TriggerOnchange:
		return "onchange"
	case TriggerBefore:
		return "before"
	case TriggerAfter:
		return "after"
	case TriggerAlways:
		return "always"
	case TriggerManual:
		return "manual"
	default:
		return "unknown"
	}
}

type Script struct {
	Path        string
	Name        string
	Trigger     Trigger
	Order       int
	SourceDir   string
	ContentHash string
}

func (s *Script) IsExecutable() bool {
	info, err := os.Stat(s.Path)
	if err != nil {
		return false
	}
	return info.Mode()&0111 != 0
}

var scriptPattern = regexp.MustCompile(`^run_(once|onchange|before|after|always)_(\d+)-(.+)$`)

func ParseScriptName(name string) (Trigger, int, string) {
	matches := scriptPattern.FindStringSubmatch(name)
	if matches == nil {
		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		return TriggerManual, 0, baseName
	}

	var trigger Trigger
	switch matches[1] {
	case "once":
		trigger = TriggerOnce
	case "onchange":
		trigger = TriggerOnchange
	case "before":
		trigger = TriggerBefore
	case "after":
		trigger = TriggerAfter
	case "always":
		trigger = TriggerAlways
	}

	order, _ := strconv.Atoi(matches[2])
	scriptName := strings.TrimSuffix(matches[3], filepath.Ext(matches[3]))

	return trigger, order, scriptName
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

func (s Scripts) ByTrigger(t Trigger) Scripts {
	var result Scripts
	for _, script := range s {
		if script.Trigger == t {
			result = append(result, script)
		}
	}
	return result
}
