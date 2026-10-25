package scripts

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/subbeh/statemate/internal/state"
)

const ScriptsDir = ".matescripts"

type Discoverer struct {
	rootDir string
	sources []string
}

func NewDiscoverer(rootDir string, sources []string) *Discoverer {
	return &Discoverer{
		rootDir: rootDir,
		sources: sources,
	}
}

func (d *Discoverer) Discover() (Scripts, error) {
	var scripts Scripts

	rootScripts, err := d.discoverDir(d.rootDir, "")
	if err != nil {
		return nil, err
	}
	scripts = append(scripts, rootScripts...)

	for _, source := range d.sources {
		sourceScripts, err := d.discoverDir(source, source)
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, sourceScripts...)
	}

	scripts.Sort()
	return scripts, nil
}

func (d *Discoverer) discoverDir(dir, sourceDir string) (Scripts, error) {
	scriptsPath := filepath.Join(dir, ScriptsDir)

	info, err := os.Stat(scriptsPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var scripts Scripts
	err = filepath.WalkDir(scriptsPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		if err := CheckScriptName(entry.Name()); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		name, freq, timing, template, profile, order := ParseScriptName(entry.Name())

		contentHash, err := state.HashFile(path)
		if err != nil {
			return err
		}
		onchange, err := ReadOnchange(path, freq)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		scripts = append(scripts, &Script{
			Path:        path,
			Name:        name,
			Frequency:   freq,
			Timing:      timing,
			Template:    template,
			NoConfirm:   HasNoConfirm(entry.Name()),
			Profile:     profile,
			Order:       order,
			SourceDir:   sourceDir,
			ContentHash: contentHash,
			Description: ReadDescription(path),
			Onchange:    onchange,
		})
		return nil
	})

	return scripts, err
}
