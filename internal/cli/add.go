package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/template"
)

var addCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a file to source",
	Long: `Add an existing file to the source directory.

The file is copied into a source, keeping its path relative to the target base
(stow-style), and the original stays in place. Directories cannot be added;
add the files in them one by one.

The source is the one named with --source, else 'default_source' from the
config, else one you pick from a list of the configured sources. The source must
already be listed under 'sources:' and exist as a directory.

A file outside your home directory, such as one under /etc, needs a source that
maps that location (see 'targets:' in a source's .mate.yaml); for a source
without a .mate.yaml, mate offers to create one.

--for-profile marks the file #profile:<name>, so it is only deployed for that
profile. The global --profile only selects the active profile, which decides the
sources you can add to, as it does for every other command.`,
	Example: `  mate add ~/.config/nvim/init.lua
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
	addCmd.Flags().StringVar(&addForProfile, "for-profile", "", "deploy the file only for this profile (adds #profile:<name>)")
	addCmd.Flags().BoolVar(&addEncrypt, "encrypt", false, "encrypt file when adding")
	addCmd.Flags().StringVarP(&addSource, "source", "s", "", "target source directory")
	addCmd.Flags().BoolVar(&addTemplate, "template", false, "mark file as template")

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
	if info.IsDir() {
		return fmt.Errorf("cannot add directories directly, add files individually")
	}

	scanner, err := newScanner(cfg, profileName)
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}
	tree, err := scanner.Scan(absSources)
	if err != nil {
		return fmt.Errorf("scanning sources: %w", err)
	}

	for _, entry := range tree.Files() {
		if entry.TargetPath == targetPath {
			return fmt.Errorf("file already managed: %s (in %s)", targetPath, filepath.Base(filepath.Dir(entry.SourcePath)))
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
			return fmt.Errorf("source %q not in configured sources", sourceName)
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
		destName = destName + "#encrypted"
	}
	if addTemplate {
		destName = destName + "#template"
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
