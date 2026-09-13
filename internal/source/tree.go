package source

import (
	"os"
	"path/filepath"
)

type Entry struct {
	SourcePath string
	TargetPath string
	RelPath    string
	Name       string
	Attrs      Attrs
	IsDir      bool
	Mode       os.FileMode

	// Generated entries have no source file - content is provided directly
	Generated        bool
	GeneratedContent string
}

type Tree struct {
	Entries  []*Entry
	Conflicts []Conflict
}

type Conflict struct {
	TargetPath string
	Sources    []string
}

func (t *Tree) AddEntry(e *Entry) {
	t.Entries = append(t.Entries, e)
}

func (t *Tree) CheckConflicts() {
	targets := make(map[string][]string)
	for _, e := range t.Entries {
		if !e.IsDir {
			targets[e.TargetPath] = append(targets[e.TargetPath], e.SourcePath)
		}
	}

	for target, sources := range targets {
		if len(sources) > 1 {
			t.Conflicts = append(t.Conflicts, Conflict{
				TargetPath: target,
				Sources:    sources,
			})
		}
	}
}

func (t *Tree) HasConflicts() bool {
	return len(t.Conflicts) > 0
}

func (t *Tree) FilterByProfile(profileChain []string) *Tree {
	filtered := &Tree{}
	for _, e := range t.Entries {
		if e.Attrs.Profile == "" || profileMatches(e.Attrs.Profile, profileChain) {
			filtered.Entries = append(filtered.Entries, e)
		}
	}
	return filtered
}

func profileMatches(target string, chain []string) bool {
	for _, p := range chain {
		if p == target {
			return true
		}
	}
	return false
}

func (t *Tree) Files() []*Entry {
	var files []*Entry
	for _, e := range t.Entries {
		if !e.IsDir {
			files = append(files, e)
		}
	}
	return files
}

func (t *Tree) Dirs() []*Entry {
	var dirs []*Entry
	for _, e := range t.Entries {
		if e.IsDir {
			dirs = append(dirs, e)
		}
	}
	return dirs
}

// EmptyDirTargets returns the target paths of directories with nothing beneath
// them in the tree. A directory holding files is created as a side effect of
// applying those files, so it needs no reporting of its own; a directory holding
// nothing exists in the source purely to be created, which is the only way to
// declare an empty directory.
func (t *Tree) EmptyDirTargets() map[string]bool {
	empty := make(map[string]bool)
	for _, d := range t.Dirs() {
		empty[d.TargetPath] = true
	}
	// Every ancestor of an entry holds something, so it is not empty.
	for _, e := range t.Entries {
		for p := filepath.Dir(e.TargetPath); ; {
			delete(empty, p)
			parent := filepath.Dir(p)
			if parent == p {
				break
			}
			p = parent
		}
	}
	return empty
}
