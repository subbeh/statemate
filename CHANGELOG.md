# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- `mate packages status` no longer shows AUR packages as extras under pacman (uses `-Qen` for native-only)
- `mate edit` now works on `include`/`var_files` (e.g. `.matedata/secrets.yaml#encrypted`), which previously failed with "file not found"
- `mate edit` tab completion now offers real filesystem paths instead of a computed list that could suggest unopenable files
- `mate edit` preserves the original file permissions when re-encrypting, and writes the plaintext temp file as `0600`

### Changed
- `mate edit` resolves paths strictly like other CLI tools (absolute or relative to the current directory). Fuzzy suffix matching is no longer supported — `mate edit nvim/init.lua` must be run from the directory containing `nvim/`
- `mate edit` accepts any file under the source directory, and resolves target paths to their source file
- `mate edit` prints a reminder to run `mate apply` after editing a managed source file

## [0.2.1] - 2026-08-02

### Fixed
- `mate status`, `mate diff`, and `mate check` now render templates before comparing hashes — variable changes in include files are correctly detected as pending changes
- `mate apply` now re-renders templates when variables change, even if the source file is unchanged
- `mate apply` restores secret lookup after config reload (before-scripts no longer break templates with bitwarden calls)
- `mate status`, `mate diff`, and `mate check` no longer fail on permission-denied files — uses non-interactive sudo to check when possible, skips with a warning otherwise
- `mate status` and `mate apply` now detect and fix directory permission mismatches
- `mate apply` respects perm attributes on directories (previously hardcoded 0755)
- `mate add` now works with files under `targets` mappings (e.g. adding `/etc/...` to a source with `targets: { etc: /etc }`)
- `mate add` resolves existing attrs-suffixed directories (e.g. `etc#owner-r:root`) instead of creating plain duplicates
- `mate edit` resolves files relative to cwd and target paths

### Added
- `--sudo` flag for `mate status`, `mate diff`, and `mate check` to prompt for elevated access when checking restricted files
- AUR publishing via goreleaser (`statemate-bin` package)
- `#tmpl` as short alias for `#template` file suffix

### Changed
- Consolidated duplicate change-detection logic into single `computeChange` function (status/diff/check/apply all use the same path)

## [0.2.0] - 2026-07-22

### Fixed
- `mate status` and `mate apply` now detect permission mismatches — files with correct content but wrong permissions show as modified and get fixed on apply
- `perm-r`, `owner-r`, and `group-r` recursive attributes on directories now correctly propagate to child files
- Bitwarden provider now shows clear error messages when vault is locked or session is invalid instead of "unexpected end of JSON input"
- `mate encrypt` and `mate decrypt` now work with files outside the source tree (e.g. var_files in `.matedata/`)
- Encrypted var_files (`#encrypted` suffix) are now transparently decrypted during template rendering
- Tab completion now shows only files in the current directory tree with relative paths instead of all files with absolute paths
- Scripts now run correctly when not marked executable (falls back to shebang interpreter)
- Package install check separated from extras detection for accurate status
- Scripts respect profile inheritance for filtering

### Added
- `ignore` key in `mate.yaml` for gitignore-style patterns to exclude files from scanning (replaces `.mateignore` files)
- `ignore` key in per-source `.mate.yaml` for patterns scoped to that source only
- `#tmpl` as a short alias for `#template` file suffix
- Documented the `STATEMATE_DIR` environment variable, which overrides `source_dir` to point at a different dotfiles directory
- Per-source `target_base` in `.mate.yaml` to deploy files to a different root directory
- `.mate.yaml` files now support template rendering (use variables like `{{ .Vars.workspace }}`)
- `mate add` prompts to create/update `.mate.yaml` when adding files outside home directory
- `mate packages apply` now shows package manager output during install/uninstall
- `generate` directive in `.mate.yaml` to dynamically create files from templates
- `indent` template function for proper YAML multiline content formatting
- `daily`, `weekly`, and `monthly` script frequencies for interval-based execution
- Script profile attribute for per-profile execution
- `mate scripts list` shows source, profile, and timestamps
- `mate apply` fetches missing secrets before applying
- `mate apply` prompts to install missing packages during apply
- `mate scripts run` subcommand with execution logging

### Changed
- `mate profile` now shows the sources that apply to the active profile
- Profile inheritance now respected globally for file filtering

### Removed
- `.mateignore` files are no longer supported (use `ignore` in `mate.yaml` instead)

## [0.1.0] - 2026-07-07

Initial release.

### Features
- Stow-style multi-directory source management
- Profile-based configuration with auto-detection (hostname, OS, arch, user, command)
- Profile inheritance via `extends`
- Go text/template support for dynamic configuration
- Age encryption for sensitive files
- Declarative package management (brew, pacman, AUR)
- Lifecycle scripts with frequency control (once, always, onchange)
- File attributes via `#` suffixes (profile, perm, owner, group, encrypted, template, symlink)
- Secrets management with Bitwarden integration and age-encrypted cache
- `.mateignore` file support for excluding files
- Source-level configuration via `.mate.yaml`
- Shell completions (bash, zsh, fish, powershell)

### Commands
- `init` - Initialize a new statemate repository
- `apply` - Apply configuration to target
- `status` - Show pending changes
- `diff` - Show full unified diff
- `check` - Validate configuration
- `add` - Add file to source
- `forget` - Remove file from tracking
- `delete` - Delete file from source and target
- `rename` - Rename managed file
- `encrypt` - Encrypt a file in place
- `decrypt` - Decrypt a file in place
- `edit` - Edit file (with auto decrypt/encrypt)
- `eval` - Render template to stdout
- `cat` - Display file contents (with auto decrypt)
- `clean` - Remove orphaned files
- `managed` - List managed files
- `profile` - Show active profile
- `packages` - Manage system packages
- `scripts` - List and run scripts
- `secrets` - Manage secrets cache
- `doctor` - Check system health
