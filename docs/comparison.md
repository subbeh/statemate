# Comparison

How statemate compares with other dotfile managers. The other tools change over
time and plugins fill many of their gaps, so check their own documentation before
relying on a cell. Corrections are welcome.

| | statemate | chezmoi | yadm | GNU Stow | dotbot |
|---|---|---|---|---|---|
| Written in | Go, single binary | Go, single binary | Bash, needs git | Perl | Python, usually a git submodule |
| Platforms | Linux, macOS | Linux, macOS, Windows, BSD | Unix-like, WSL | Unix-like | Linux, macOS, Windows |
| Deploys by | copying (symlinks with [`#link`](attributes.md#link)) | copying (symlink mode optional) | the home directory is the git work tree | symlinking | symlinking |
| Per-file options | `#` suffixes: `config#enc#perm:600` | prefixes: `private_dot_config` | `##` suffixes for alternates | none | YAML config |
| Per machine | named [profiles](machines.md), auto-detected, with inheritance | template data and conditionals | alternate files by OS, host, user, or class | choose packages by hand | separate config files |
| Templates | Go templates with sprig | Go templates with sprig | built-in, esh, or Jinja | no | no |
| Encryption | age | age, gpg | gpg or openssl archive, git-crypt, transcrypt | no | no |
| Password managers | Bitwarden, cached in an encrypted file | many: 1Password, Bitwarden, pass, KeePassXC, Vault, … | no | no | no |
| Target edited outside the tool | detected, asks before overwriting | detected, asks before overwriting | n/a, the target is the repository | n/a, targets are links | n/a, targets are links |
| App rewrites its own config | flows back to the repository automatically with [`#import`](attributes.md#import) | `chezmoi re-add`, by hand, not for templates | shows in `git status` | edits the repository directly | edits the repository directly |
| Scripts | once, on change, always, daily, weekly, monthly; before or after | once, on change, always; before or after | one bootstrap script, command hooks | no | `shell` steps |
| Commands when matching files change | [hooks](hooks.md) by glob | `run_onchange_` script with a hash of the file | no | no | no |
| Packages | [declared](packages.md): brew, pacman, AUR | via scripts | via bootstrap | no | via plugins |
| Files outside home | [`/etc` and friends](system-files.md), sudo, owner and group | another destination directory, no sudo or ownership | home only | any target directory, sudo yourself | any path |
| Files removed from the repository | reported as orphans, removed by `mate clean` | left in place, unless `.chezmoiremove` or `exact_` | removed with the file | `stow -D` removes links | `clean` removes dead links |

## When another tool fits better

- **Windows.** statemate builds for Linux and macOS only. chezmoi and dotbot
  support Windows.
- **A password manager other than Bitwarden.** chezmoi supports most of them.
- **Edit in place, nothing to apply.** With symlinks (Stow, dotbot) or a bare
  repository (yadm), editing `~/.zshrc` edits the repository. With statemate you
  edit the source, with `mate edit`, and apply.
- **Nothing to install.** yadm is a single shell script, and Stow is packaged by
  most distributions.

## Migrating

statemate's layout is the same as Stow's: one directory per tool, its contents
deployed relative to home. A Stow repository becomes a statemate repository by
listing those directories under `sources:` in `mate.yaml`. On the first apply the
existing symlinks are reported as conflicts; answer `[o]verwrite` to replace them
with real files. See [`#link`](attributes.md#link).

From any other tool, bring files in one at a time with `mate add`, which copies
the deployed file into a source.
