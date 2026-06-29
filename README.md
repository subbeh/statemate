# Statemate

Declarative system configuration management. Manage dotfiles, configs, and packages across machines.

## Features

- **Stow-style sources** - Organize configs by app, deployed relative to home
- **Profiles** - Machine-specific configs with auto-detection
- **Templates** - Go text/template for dynamic configs
- **Encryption** - Age encryption for sensitive files
- **Packages** - Declarative package management (brew, pacman, yay)
- **Scripts** - Lifecycle scripts (before/after apply, run once, on change)

## Quick Start

```bash
# Install
go install github.com/subbeh/statemate/cmd/mate@latest

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
    sources: [common]  # profile-specific sources
  work:
    extends: base
    detection:
      hostname: "work-*"
    packages:
      brew: [slack]

age:
  identity: "~/.config/statemate/key.txt"
  recipients: ["age1..."]

variables:
  email: "you@example.com"

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

## Local Config

Machine-specific overrides at `~/.config/statemate/mate.yaml`:

```yaml
source_dir: "~/dotfiles"  # where to find mate.yaml
profile: work             # override auto-detection
```

## Documentation

- Run `mate <command> --help` for command help
- Man pages: `make docs` generates to `docs/man/`
- Full command reference: `docs/markdown/`

## Shell Completions

```bash
mate completion bash > /etc/bash_completion.d/mate
mate completion zsh > "${fpath[1]}/_mate"
mate completion fish > ~/.config/fish/completions/mate.fish
```

## License

MIT
