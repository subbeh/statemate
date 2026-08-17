# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- Secret discovery now renders templates with the same functions as `mate apply`. A template using a function discovery did not know about failed to parse there and was skipped silently, so its secrets were never fetched and the apply failed on a cache miss
- `mate apply` can now record state for files it wrote via sudo. A root-owned or `0600` target (such as a rendered secret) is unreadable as the invoking user, so the apply failed on hashing the file it had just written successfully
- `mate apply` now uses sudo to create directories outside your writable tree, so a source mapping `targets: { etc: /etc }` no longer fails with `creating directory /etc/restic: permission denied`. Directory `owner`/`group` attributes are also applied now, which they previously were not. An existing directory with no perm/owner/group attribute is left completely untouched, so mapped roots like `/etc` are never chmodded
- `mate apply -s <source>` no longer fetches secrets or discovers scripts belonging to other sources. Secret discovery walks the source directories directly, so a scoped run could fail trying to fetch a secret referenced only by a source it was not applying
- `mate packages status` no longer shows AUR packages as extras under pacman (uses `-Qen` for native-only)
- `mate packages status` now detects virtual/provides packages as installed (e.g. `man` provided by `man-db`)
- `mate clean` now uses sudo to remove files requiring elevated access instead of failing with "permission denied"
- `mate status`, `mate diff`, and `mate check` now correctly detect permission errors on wrapped errors, and always attempt non-interactive sudo for restricted files
- Tab completion for `--source` now lists sources from all profiles, not just the detected one
- `mate edit` now works on `include`/`var_files` (e.g. `.matedata/secrets.yaml#encrypted`), which previously failed with "file not found"
- `mate edit` tab completion now offers real filesystem paths instead of a computed list that could suggest unopenable files
- `mate edit` preserves the original file permissions when re-encrypting, and writes the plaintext temp file as `0600`
- `mate add` source picker now lists profile-provided sources, not just the top-level `sources:` list. Previously the picker showed a different list than it indexed, so a selection could map to the wrong source

### Added
- Templates now have the [sprig](https://masterminds.github.io/sprig/) function library (~200 functions: `splitList`, `trimSuffix`, `upper`, `join`, `ternary`, `regexReplaceAll`, and so on). Previously only 9 functions existed, so a template using anything else failed with `function "splitList" not defined`. Statemate's own functions take precedence where a name collides — `env` still reads the rendering context rather than the live process environment, `default` still substitutes only for `nil` and `""` (sprig's also replaces `0` and `false`), and `indent` still leaves blank lines unpadded
- `mate apply` can now be scoped: `mate apply <path>` applies only matching files (no scripts, packages, or secret fetch), and `mate apply -s <source>` applies that source's files, runs its scripts, and prompts for its packages. Repo-root scripts are not run under `--source`, since they apply to the whole repository
- `--source`/`-s` flag for `mate status` and `mate diff` to limit output to a single source, matching the flag `mate add` already uses
- `mate config source-dir` prints the resolved source directory as a bare path, for use in scripts and editor integrations (`cd "$(mate config source-dir)"`)
- `mate status` now reports missing packages (declared in config but not installed), grouped by package manager
- `mate apply` now asks for confirmation before running each script, with `[y]es / [n]o / [s]kip / [a]ll / [q]uit`. `[n]o` skips this time only, so the script is offered again on the next apply; `[s]kip` marks it as done without running so it is not offered again. `[s]kip` is not offered for `always` scripts, whose runs are never recorded. A script marked as done still appears in `mate scripts list` and can be run manually with `mate scripts run`
- Scripts can describe themselves with a `# Description: <text>` comment in their first 10 lines. Descriptions are shown in the confirmation prompt, `mate scripts list`, and `mate status`
- `--no-scripts` flag for `mate apply` to skip all scripts (intended for automated runs)

### Changed
- **`mate status` and `mate apply` are much faster** — measured on real repos, 1.10s → 0.10s with brew (11x) and 1.15s → 0.48s with pacman/AUR (2.4x). Both commands computed the list of installed-but-undeclared packages and then discarded it: neither reports extras. That cost a `brew leaves` call (~1.0s, 95% of total runtime) on macOS, or `pacman -Qen` plus `paru -Qmtt` (~0.7s) on Arch. Extras are now only computed by the commands that report them
- `mate packages status` no longer counts extra packages unless `--all` is given (1.05s → 0.38s on Arch, 1.02s → 0.06s with brew). Without `--all` it prints a static `Use --all to also show packages not in config` hint instead of `(N extra brew packages not in config…)`, since producing the count is the expensive part
- **The positional argument to `mate status` and `mate diff` is now a file/path filter only.** Previously a bare word also prefix-matched a whole source, so `mate status nvim` filtered by source; use `mate status -s nvim` instead. A positional that names a source now errors with that suggestion rather than silently matching files
- `mate scripts list` shows descriptions in a `DESCRIPTION` column instead of an unaligned line under each row, so there is one row per script. Long descriptions are truncated with `…` to fit the terminal; when the output is piped or redirected they are printed in full. Column widths now size to their content, and the `-` marker for profile-inactive scripts is gone since those rows already show `n/a`
- **`#onchange` scripts now trigger on changes to their source, not to the script itself.** A script in `<source>/.matescripts/` runs when `<source>` has pending changes (the files `mate status` lists); one in the repo-root `.matescripts/` runs when any source has pending changes. Editing an `#onchange` script no longer reruns it — use `mate scripts run <name>` to run it on demand. Previously `#onchange` compared the script's own content hash, so a script like `arch/.matescripts/00-env_reload.sh#onchange#after` only fired when you edited the reload script, which meant it effectively never ran
- `mate managed <path>` now matches exactly one file when given a path to an existing file (target or source), instead of every file with the same basename. Bare names that do not resolve to a file still match loosely, so `mate managed nvim` continues to list a whole source
- `mate apply --force` now also auto-confirms scripts, in addition to overwriting modified targets
- `mate apply` skips scripts with a warning when there is no terminal to confirm on. **Automated runs that relied on scripts executing must now pass `--force` (to run them) or `--no-scripts` (to skip silently)**
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
