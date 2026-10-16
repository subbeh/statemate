package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/template"
	"github.com/subbeh/statemate/internal/util"
)

var addCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a file or directory to source",
	Long: `Add an existing file or directory to the source directory.

Files are copied into a source, keeping their paths relative to the target base,
and the originals stay in place. Directories are added recursively, including
hidden files. Files already in the chosen source are skipped; a file
managed by another source stops the command before any files are copied.
File symlinks are copied as ordinary files using their contents; directory
symlinks, configured source directories, and empty directories are not added.
Unreadable files, broken symlinks, and special files stop the command.

The source is the one named with --source, else 'default_source' from the
config, else one you pick from a list of the configured sources. The source must
already be listed under 'sources:' and exist as a directory. A named source
whose directory does not exist yet is offered for creation: mate creates the
directory and adds the file, but you still list it under 'sources:' yourself.

A file outside your home directory, such as one under /etc, needs a source that
maps that location (see 'targets:' in a source's .mate.yaml); for a source
without a .mate.yaml, mate offers to create one.

--for-profile marks each file #profile:<name>, so it is only deployed for that
profile. --encrypt and --template also apply to every added file.
The global --profile only selects the active profile, which decides the
sources you can add to, as it does for every other command.`,
	Example: `  mate add ~/.config/nvim/init.lua
  mate add ~/.config/nvim
  mate add --for-profile work ~/.gitconfig
  mate add --encrypt ~/.ssh/config`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

var (
	addForProfile string
	addEncrypt    bool
	addSource     string
	addTemplate   bool
)

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVar(&addForProfile, "for-profile", "", "deploy added files only for this profile (adds #profile:<name>)")
	addCmd.Flags().BoolVar(&addEncrypt, "encrypt", false, "encrypt each file when adding")
	addCmd.Flags().StringVarP(&addSource, "source", "s", "", "target source directory")
	addCmd.Flags().BoolVar(&addTemplate, "template", false, "mark each file as template")

	_ = addCmd.RegisterFlagCompletionFunc("source", completeSources)
	_ = addCmd.RegisterFlagCompletionFunc("for-profile", completeProfiles)
}

func runAdd(cmd *cobra.Command, args []string) error {
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
	absSources := cfg.ResolveSourcePaths(sources)
	if len(absSources) == 0 {
		return fmt.Errorf("no sources configured")
	}

	targetPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("file not found: %s", targetPath)
	}

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}
	tree, err := scanner.Scan(absSources)
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	if !info.IsDir() {
		for _, entry := range tree.Files() {
			if entry.TargetPath == targetPath {
				return fmt.Errorf("file already managed: %s (in %s)", targetPath, filepath.Base(filepath.Dir(entry.SourcePath)))
			}
		}
	}

	var sourceDir string
	sourceName := addSource
	if sourceName == "" {
		sourceName = cfg.DefaultSource
	}

	if sourceName != "" {
		if filepath.IsAbs(sourceName) {
			sourceDir = sourceName
		} else {
			sourceDir = filepath.Join(cfg.SourceDir(), sourceName)
		}
		found := false
		for _, s := range absSources {
			if s == sourceDir {
				found = true
				break
			}
		}
		if !found {
			notConfigured := fmt.Errorf("source %q not in configured sources", sourceName)
			if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
				return notConfigured
			}
			if err := promptCreateSource(sourceDir, notConfigured); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s source %q is not in the config; add it to 'sources:' so mate deploys it\n", color.YellowString("Warning:"), sourceName)
		}
	} else {
		// Must offer the same list that absSources is indexed from: cfg.Sources
		// omits profile-provided sources, so showing it both hid valid choices and
		// mapped the selection to the wrong source.
		idx, err := promptSourceSelection(sources)
		if err != nil {
			return err
		}
		sourceDir = absSources[idx]
	}

	tmplCtx, err := template.NewContext(cfg, profileName)
	if err != nil {
		return fmt.Errorf("creating template context: %w", err)
	}
	renderer := func(data []byte) ([]byte, error) {
		return template.Render(data, tmplCtx)
	}

	if !info.IsDir() {
		return addFileToSource(targetPath, info, sourceDir, cfg, tree, renderer)
	}

	files, err := collectAddFiles(targetPath, absSources)
	if err != nil {
		return err
	}
	files, err = unmanagedAddFiles(files, sourceDir, tree)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := addFileToSource(file.path, file.info, sourceDir, cfg, tree, renderer); err != nil {
			return fmt.Errorf("adding %s: %w", file.path, err)
		}
	}
	return nil
}

type addFile struct {
	path string
	info os.FileInfo
}

// Finish discovery before writing anything into a source. Besides avoiding
// recursive self-imports, this reports traversal errors before copying files.
func collectAddFiles(root string, sources []string) ([]addFile, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolving directory %s: %w", root, err)
	}
	var realSources []string
	for _, sourceDir := range sources {
		realSource, err := filepath.EvalSymlinks(sourceDir)
		if err != nil {
			return nil, fmt.Errorf("resolving source %s: %w", sourceDir, err)
		}
		realSources = append(realSources, realSource)
	}

	var files []addFile
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			realPath := filepath.Join(realRoot, rel)
			for _, sourceDir := range realSources {
				if pathWithin(sourceDir, realPath) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// Stat follows file symlinks, matching single-file add. WalkDir itself
		// never descends into directory symlinks, including loops.
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cannot add non-regular file: %s", path)
		}
		files = append(files, addFile{path: path, info: info})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", root, err)
	}
	return files, nil
}

// Check the entire batch before copying, so a conflict in a later file cannot
// leave a directory half imported. A target may appear in more than one source.
func unmanagedAddFiles(files []addFile, sourceDir string, tree *source.Tree) ([]addFile, error) {
	managed := make(map[string][]*source.Entry)
	for _, entry := range tree.Files() {
		managed[entry.TargetPath] = append(managed[entry.TargetPath], entry)
	}
	var pending []addFile
	for _, file := range files {
		owners := managed[file.path]
		for _, owner := range owners {
			ownerDir := owner.SourcePath
			if !owner.Generated {
				ownerDir = strings.TrimSuffix(owner.SourcePath, string(filepath.Separator)+owner.RelPath)
			}
			if filepath.Clean(ownerDir) != filepath.Clean(sourceDir) {
				return nil, fmt.Errorf("file already managed by another source: %s (in %s)", file.path, ownerDir)
			}
		}
		if len(owners) == 0 {
			pending = append(pending, file)
		}
	}
	return pending, nil
}

func pathWithin(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func addFileToSource(targetPath string, info os.FileInfo, sourceDir string, cfg *config.Config, tree *source.Tree, renderer config.TemplateRenderer) error {
	targetBase, err := resolveTargetBaseForAdd(sourceDir, targetPath, cfg.TargetBase, tree, renderer)
	if err != nil {
		return err
	}

	relPath, err := computeRelativePath(targetPath, targetBase)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	destName := filepath.Base(relPath)
	if addForProfile != "" {
		destName = destName + "#profile:" + addForProfile
	}
	if addEncrypt {
		destName = destName + "#enc"
	}
	if addTemplate {
		destName = destName + "#tmpl"
	}

	destPath := filepath.Join(resolveAttrsPath(sourceDir, filepath.Dir(relPath)), destName)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if addEncrypt {
		if cfg.Age == nil || len(cfg.Age.Recipients) == 0 {
			return fmt.Errorf("encryption requested but no age recipients configured")
		}

		enc, err := encrypt.NewAgeEncryptor("", "", cfg.Age.Recipients)
		if err != nil {
			return fmt.Errorf("creating encryptor: %w", err)
		}

		content, err = enc.Encrypt(content)
		if err != nil {
			return fmt.Errorf("encrypting: %w", err)
		}
	}

	if err := os.WriteFile(destPath, content, info.Mode()); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	fmt.Printf("Added %s -> %s\n", targetPath, destPath)
	return nil
}

func computeRelativePath(targetPath, targetBase string) (string, error) {
	targetBase = expandPath(targetBase)
	if targetBase == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		targetBase = home
	}

	absBase, err := filepath.Abs(targetBase)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(targetPath, absBase) {
		return "", fmt.Errorf("file %s is not under target base %s", targetPath, absBase)
	}

	rel, err := filepath.Rel(absBase, targetPath)
	if err != nil {
		return "", err
	}

	return rel, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func promptSourceSelection(sources []string) (int, error) {
	prompt := promptui.Select{
		Label: "Select source",
		Items: sources,
		Size:  10,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(sources[index]), strings.ToLower(input))
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	return idx, nil
}

func resolveTargetBaseForAdd(sourceDir, targetPath, globalTargetBase string, tree *source.Tree, renderer config.TemplateRenderer) (string, error) {
	dirCfg, err := config.LoadDirConfigRaw(sourceDir, renderer)
	if err != nil {
		// Treating a broken .mate.yaml as absent would offer to create one,
		// overwriting it.
		return "", fmt.Errorf("loading .mate.yaml in %s: %w", sourceDir, err)
	}

	// Check if file is under global target base
	globalBase := expandPath(globalTargetBase)
	if globalBase == "" {
		home, _ := os.UserHomeDir()
		globalBase = home
	}
	fileUnderGlobal := strings.HasPrefix(targetPath, globalBase+string(filepath.Separator)) || targetPath == globalBase

	// Case 1: No .mate.yaml exists
	if dirCfg == nil {
		if fileUnderGlobal {
			return globalBase, nil
		}
		// File is outside global target base - need to create .mate.yaml with
		// target_base. That base applies to the whole source, so refuse when the
		// source already deploys files under the global base: they would move,
		// ~/.zshrc becoming /etc/.zshrc.
		if sourceHasFiles(sourceDir, tree) {
			return "", fmt.Errorf("source %q has existing files under %s; cannot add file from %s", filepath.Base(sourceDir), globalBase, targetPath)
		}
		return promptCreateDirConfig(sourceDir, targetPath)
	}

	// Case 2: .mate.yaml exists but no target_base set
	if dirCfg.TargetBase == "" {
		if fileUnderGlobal {
			return globalBase, nil
		}
		// Check if file falls under a targets mapping
		for _, mappedBase := range dirCfg.Targets {
			mappedBase = expandPath(mappedBase)
			if strings.HasPrefix(targetPath, mappedBase+string(filepath.Separator)) || targetPath == mappedBase {
				return filepath.Dir(mappedBase), nil
			}
		}
		// Check if source already has files
		if sourceHasFiles(sourceDir, tree) {
			return "", fmt.Errorf("source %q has existing files under %s; cannot add file from %s", filepath.Base(sourceDir), globalBase, targetPath)
		}
		// Prompt to add target_base to existing .mate.yaml
		return promptAddTargetBase(sourceDir, targetPath)
	}

	// Case 3: .mate.yaml exists with target_base set
	configuredBase := expandPath(dirCfg.TargetBase)
	fileUnderConfigured := strings.HasPrefix(targetPath, configuredBase+string(filepath.Separator)) || targetPath == configuredBase

	if !fileUnderConfigured {
		return "", fmt.Errorf("file %s is not under source's target_base %s", targetPath, configuredBase)
	}

	return configuredBase, nil
}

func resolveAttrsPath(baseDir, relPath string) string {
	parts := strings.Split(relPath, string(filepath.Separator))
	current := baseDir
	for _, part := range parts {
		if part == "." {
			continue
		}
		exact := filepath.Join(current, part)
		if _, err := os.Stat(exact); err == nil {
			current = exact
			continue
		}
		// Look for a directory with attrs suffix matching this base name
		entries, err := os.ReadDir(current)
		if err != nil {
			current = exact
			continue
		}
		found := false
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			baseName := strings.SplitN(name, "#", 2)[0]
			if baseName == part {
				current = filepath.Join(current, name)
				found = true
				break
			}
		}
		if !found {
			current = exact
		}
	}
	return current
}

func sourceHasFiles(sourceDir string, tree *source.Tree) bool {
	for _, entry := range tree.Files() {
		if strings.HasPrefix(entry.SourcePath, sourceDir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func promptCreateDirConfig(sourceDir, targetPath string) (string, error) {
	targetBase := inferTargetBase(targetPath)

	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("Create .mate.yaml with target_base: %s", targetBase),
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return "", fmt.Errorf("file %s is not under home directory; specify target_base in source's .mate.yaml", targetPath)
		}
		return "", err
	}

	if err := writeDirConfig(sourceDir, targetBase); err != nil {
		return "", fmt.Errorf("creating .mate.yaml: %w", err)
	}

	fmt.Printf("Created %s/.mate.yaml\n", sourceDir)
	return targetBase, nil
}

// Only the directory is created; the config is left for the user to edit.
func promptCreateSource(sourceDir string, declined error) error {
	ok, err := util.Confirm(fmt.Sprintf("Create source directory %s? [y/N] ", util.ShortenPath(sourceDir)), false)
	if err != nil {
		return err
	}
	if !ok {
		return declined
	}

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("creating source directory: %w", err)
	}

	fmt.Printf("Created %s\n", sourceDir)
	return nil
}

func promptAddTargetBase(sourceDir, targetPath string) (string, error) {
	targetBase := inferTargetBase(targetPath)

	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("Add target_base: %s to .mate.yaml", targetBase),
		IsConfirm: true,
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return "", fmt.Errorf("file %s is not under home directory; add target_base to source's .mate.yaml", targetPath)
		}
		return "", err
	}

	if err := addTargetBaseToDirConfig(sourceDir, targetBase); err != nil {
		return "", fmt.Errorf("updating .mate.yaml: %w", err)
	}

	fmt.Printf("Updated %s/.mate.yaml\n", sourceDir)
	return targetBase, nil
}

func inferTargetBase(targetPath string) string {
	// Walk up the path to find a reasonable base
	// Use the parent of the first path component after root
	dir := filepath.Dir(targetPath)
	for {
		parent := filepath.Dir(dir)
		if parent == "/" || parent == "." {
			return dir
		}
		dir = parent
	}
}

func writeDirConfig(sourceDir, targetBase string) error {
	configPath := filepath.Join(sourceDir, ".mate.yaml")
	content := fmt.Sprintf("target_base: %s\n", targetBase)
	return os.WriteFile(configPath, []byte(content), 0644)
}

func addTargetBaseToDirConfig(sourceDir, targetBase string) error {
	configPath := filepath.Join(sourceDir, ".mate.yaml")
	existing, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("target_base: %s\n%s", targetBase, string(existing))
	return os.WriteFile(configPath, []byte(content), 0644)
}
