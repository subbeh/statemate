# Statemate

Declarative system configuration management. Manage dotfiles, configs, and packages across machines.

## Features

- **Stow-style sources** - Organize configs by app, deployed relative to home
- **Profiles** - Machine-specific configs with auto-detection
- **Templates** - Go text/template for dynamic configs
- **Encryption** - Age encryption for sensitive files
- **Packages** - Declarative package management (brew, pacman, AUR)
- **Scripts** - Lifecycle scripts (before/after apply, run once, on change)

## Installation

```bash
# Homebrew (macOS/Linux)
brew install subbeh/tap/statemate

# Go install
go install github.com/subbeh/statemate/cmd/mate@latest

# From source
git clone https://github.com/subbeh/statemate
cd statemate && make build-all
```

## Quick Start

```bash
# Initialize (creates mate.yaml or mate.toml)
mate init

# Add source directories to config, then add files
mate add ~/.config/nvim/init.lua

# Preview changes
mate status
mate diff

# Apply
mate apply
```

## Configuration

```yaml
# mate.yaml
sources: [nvim, zsh, git]
target_base: "~"

profiles:
  base:
    sources: [common]
  work:
    extends: base
    detection:
      hostname: "work-*"
    variables:
      email: "work@example.com"
    packages:
      brew: [slack]

age:
  identity: "~/.config/statemate/key.txt"
  recipients: ["age1..."]

variables:
  email: "you@example.com"

var_files:
  - .matedata/secrets.yaml  # loaded relative to source dir

packages:
  brew: [ripgrep, fd]
```

## File Attributes

Use `#` suffixes to control behavior (stripped from target filename):

| Suffix | Description |
|--------|-------------|
| `#profile:name` | Only apply for specified profile |
| `#perm:600` | Set file permissions |
| `#owner:user` | Set file owner |
| `#group:group` | Set file group |
| `#encrypted` | File is age-encrypted |
| `#template` | Process as Go template |
| `#symlink` | Create symlink instead of copy |

Example: `.ssh/config#encrypted#perm:600`

## Scripts

Place scripts in `.matescripts/` at repo root or in source directories.

Format: `<order>-<name>.<ext>#<freq>#<timing>[#template]`

| Attribute | Values |
|-----------|--------|
| freq | `once`, `onchange`, `always` (omit for manual) |
| timing | `before`, `after` (default: before) |
| template | Add `#template` to render before execution |

Examples:
- `01-setup.sh#once#before` - run once before apply
- `02-secrets.sh#onchange#before#template` - run when changed, render first
- `99-cleanup.sh#always#after` - run after every apply

## Templates

Templates use Go's `text/template` syntax with these variables and functions:

**Variables:**
- `.Profile`, `.Hostname`, `.OS`, `.Arch`, `.HomeDir`, `.Username`
- `.SourceDir` - path to dotfiles directory
- `.Vars` - custom variables from config/var_files
- `.Env` - environment variables

**Functions:**
- `{{ required .Vars.secret }}` - error if value is missing/empty
- `{{ default "fallback" .Vars.maybe }}` - use fallback if nil/empty
- `{{ env "HOME" }}` - get environment variable
- `{{ cmd "hostname -s" }}` - run command

## Local Config

Machine-specific overrides at `~/.config/statemate/mate.yaml`:

```yaml
source_dir: "~/dotfiles"  # where to find mate.yaml
profile: work             # override auto-detection
```

## Commands

```
mate init       Initialize a new dotfiles repo
mate apply      Apply configuration to targets
mate status     Show pending changes (--short for statusline)
mate diff       Show file differences
mate add        Add a file to management
mate edit       Edit a managed file
mate forget     Stop tracking a file
mate delete     Remove source and target file
mate encrypt    Encrypt a file in place
mate decrypt    Decrypt a file in place
mate rename     Rename a managed file
mate managed    List all managed files
mate profile    Show active profile
mate packages   Manage packages
mate scripts    List scripts
mate run        Run a script manually
mate doctor     Check configuration
```

## Shell Completions

```bash
mate completion bash > /etc/bash_completion.d/mate
mate completion zsh > "${fpath[1]}/_mate"
mate completion fish > ~/.config/fish/completions/mate.fish
```

## License

MIT
