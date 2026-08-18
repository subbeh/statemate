# Statemate

Declarative system configuration management. Manage dotfiles, configs, and packages
across machines.

## Features

- **Stow-style sources** — organize configs by app, deployed relative to home
- **Profiles** — machine-specific configs with auto-detection
- **Templates** — Go templates with the sprig function library
- **Encryption** — age encryption for sensitive files
- **Secrets** — Bitwarden references resolved from an encrypted cache
- **Packages** — declarative package management (brew, pacman, AUR)
- **Scripts** — lifecycle scripts (before/after apply, run once, on change)

## Installation

```bash
# Homebrew (macOS/Linux)
brew install subbeh/tap/statemate

# Arch (AUR)
paru -S statemate-bin

# Go
go install github.com/subbeh/statemate/cmd/mate@latest

# From source
git clone https://github.com/subbeh/statemate
cd statemate && make build-all
```

## Quick Start

```bash
mkdir ~/dotfiles && cd ~/dotfiles
mate init                      # create mate.yaml

mate add ~/.config/nvim/init.lua
mate status                    # what would change
mate diff                      # how it would change
mate apply                     # make it so
```

A source directory is deployed relative to your home directory:

```
~/dotfiles/
  mate.yaml
  nvim/
    .config/nvim/init.lua      →  ~/.config/nvim/init.lua
  zsh/
    .zshrc                     →  ~/.zshrc
```

Behavior is controlled by `#` suffixes on filenames, stripped from the target:

```
.ssh/config#encrypted#perm:600         encrypted in the repo, mode 0600 deployed
gitconfig#template                     rendered as a Go template
.gitconfig#profile:work                only on machines matching the work profile
.claude/settings.json#import           app owns it; changes flow back to the repo
```

## Documentation

Full documentation is in [`docs/`](docs/README.md):

| Guide | Contents |
|-------|----------|
| [Getting Started](docs/getting-started.md) | Install, create a repository, add your first file |
| [Concepts](docs/concepts.md) | Sources, targets, state, and how a change is detected |
| [Configuration](docs/configuration.md) | `mate.yaml`, `.mate.yaml`, profiles, local overrides |
| [File Attributes](docs/attributes.md) | Every `#` suffix and what it does |
| [Templates](docs/templates.md) | Variables and functions available when rendering |
| [Secrets](docs/secrets.md) | Bitwarden references and the encrypted cache |
| [Scripts](docs/scripts.md) | Lifecycle scripts, frequency, timing |
| [Packages](docs/packages.md) | Declarative packages across brew, pacman, and the AUR |
| [Command Reference](docs/commands/mate.md) | Every command and flag |

`mate <command> --help` gives the same reference in the terminal.

## Configuration at a glance

```yaml
# mate.yaml
sources: [nvim, zsh, git]

profiles:
  work:
    extends: base
    detection:
      hostname: "work-*"
    variables:
      email: "me@company.com"
    packages:
      brew: [slack]

age:
  identity: "~/.config/statemate/key.txt"
  recipients: ["age1..."]

packages:
  brew: [ripgrep, fd]
```

See [Configuration](docs/configuration.md) for every key.

## Shell Completions

```bash
mate completion bash > /etc/bash_completion.d/mate
mate completion zsh > "${fpath[1]}/_mate"
mate completion fish > ~/.config/fish/completions/mate.fish
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). In short: `make test` and `make lint` must
pass, user-visible changes need a CHANGELOG entry, and `make docs` regenerates the
command reference when help text changes.

## License

MIT
