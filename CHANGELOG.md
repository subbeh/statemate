# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `mate profile` command to show active profile and detection source
- `STATEMATE_DIR` environment variable to specify dotfiles location
- Local machine config at `~/.config/statemate/mate.yaml` with same structure as repo config
- Local config overrides repo config (keys present in local config replace repo values)
- `mate init` now prompts for yaml or toml format (or use `--format`)
- `mate init` prompts to register the directory as dotfiles location
- `mate init` now handles cloned repos with existing configs (registers location instead of failing)
- Directory-level profile in `.mate.yaml` (e.g., `profile: linux`)
- Profile-level `sources` field - profiles can define additional source directories
- Sources from extended profiles are inherited (e.g., `work` extending `base` includes `base`'s sources)
- `mate managed [path]` command to list all managed files with their attributes
- `mate status` now warns about orphaned files (targets no longer in source)
- `mate rename` command to rename managed files (renames both source and target)
- `mate encrypt` command to encrypt existing files in place
- `mate decrypt` command to decrypt encrypted files in place
- `mate edit` command to edit files (decrypts/re-encrypts automatically for encrypted files)
- `mate apply` conflict prompt now has `[i]mport` option to copy target changes back to source
- `mate diff` now decrypts encrypted files before comparing
- `mate diff` now renders templates before comparing (shows actual diff, not template syntax)
- Colorized diff output for `mate diff` and `mate apply` conflict prompts
- `user` detection criteria for profile matching (matches `$USER` or `$USERNAME`)
- `command` detection criteria (renamed from `script` for clarity)
- `mate packages status --all` flag to show extra packages not in config
- `mate packages apply --prune` flag to remove packages not in config (with warning)
- `aur_helper` config option to specify AUR helper (default: yay, supports paru etc.)
- `common` package list for cross-platform packages (installed via brew or pacman)
- Package files can be split out to `.mate/packages.yaml` (auto-loaded if present)
- `package_files` config option to specify additional package files
- Source directories can define packages in `.mate/packages.yaml` (e.g., `./hyprland/.mate/packages.yaml`)
- Cross-platform builds (macOS arm64, Linux amd64) via `make build-all`

### Changed
- Profile detection now prioritizes child profiles over parents (inheritance depth sorting)
- Package management: `yay` list renamed to `aur` (works with any AUR helper)
- `brew` package list now uses `brew leaves --installed-on-request` (explicit packages only, not dependencies)
- `mate packages status` message clarifies "No packages configured" vs "No package managers available"

### Fixed
- `mate status` and `mate apply` no longer error when target file is missing
- `mate diff` now uses profile-resolved sources (was missing profile-level sources)
- Missing target files are now shown as modified (`~`) instead of conflict (`!`)
- Profile detection no longer randomly selects between parent and child profiles
- `mate profile` now shows detection reason correctly for all criteria

- Initial release
- Stow-style multi-directory source management
- Profile-based configuration with auto-detection (hostname, OS, arch, custom script)
- Profile inheritance via `extends`
- Go text/template support for dynamic configuration
- Age encryption for sensitive files
- Declarative package management (brew, pacman, yay)
- Lifecycle scripts (run_once, run_before, run_after, run_always, run_onchange)
- File attributes via `#` suffixes (profile, perm, owner, group, encrypted, template, symlink)
- Commands: init, apply, status, diff, check, add, forget, remove, rename, encrypt, decrypt, managed, profile, packages, scripts, run, doctor, version
- Shell completions (bash, zsh, fish, powershell)
- Man page and markdown documentation generation
