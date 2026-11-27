# Statemate

Declarative system configuration management. Manage dotfiles, configs, packages, and
maintenance tasks across machines, from one git repository and one binary, `mate`.

## Why statemate

- **Nothing changes unseen.** `mate status` and `mate diff` show what an apply would do,
  `mate apply` asks before writing, and `mate audit` lists what recent runs changed.
- **Your edits are safe.** A target you edited outside the tool is detected, and
  `mate apply` asks before overwriting it. Apps that rewrite their own config flow back
  into the repository with `#import`.
- **One repository, many machines.** Profiles detect the machine and inherit from each
  other; the same repository sets up a Mac, an Arch box and a work laptop.
- **More than dotfiles.** Packages, scripts, hooks and `/etc` files with owner and sudo
  live next to your configs, so a new machine is one `mate apply` away.
- **Removing is as easy as adding.** Drop a file from the repository and
  `mate apply --clean` removes it from the machine.

See the [comparison](docs/comparison.md) with chezmoi, yadm, GNU Stow and dotbot.

## Features

- **Sources** — organize configs by app, deployed relative to home
- **Profiles** — machine-specific configs with auto-detection
- **Templates** — Go templates with the sprig function library
- **Encryption** — age encryption for sensitive files
- **Secrets** — Bitwarden references resolved from an encrypted cache
- **Packages** — declarative package management (brew, pacman, AUR, uv tools, npm globals)
- **Scripts and hooks** — run setup once, on a schedule, or when matching files change
- **System files** — manage `/etc` with ownership and sudo handled for you

## Installation

```bash
# Homebrew (macOS/Linux)
brew install subbeh/tap/statemate

# Arch (AUR)
yay -S statemate-bin            # or: paru -S statemate-bin

# Go 1.25+
go install github.com/subbeh/statemate/cmd/mate@latest
```

Prebuilt binaries for Linux and macOS are on the
[releases page](https://github.com/subbeh/statemate/releases/latest).

## Quick Start

```bash
mkdir ~/dotfiles && cd ~/dotfiles
mate init                      # create mate.yaml and register the repository

mkdir nvim                     # a source, then list it: sources: [nvim]
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
.ssh/config#enc#perm:600         encrypted in the repo, mode 0600 deployed
.gitconfig#tmpl                  rendered as a Go template
.gitconfig#profile:work          only on machines matching the work profile
.claude/settings.json#import     app owns it; changes flow back to the repo
```

## Documentation

Full documentation is at **[subbeh.github.io/statemate](https://subbeh.github.io/statemate/)**,
built from [`docs/`](docs/README.md):

| Guide | Contents |
|-------|----------|
| [Getting Started](docs/getting-started.md) | Install, create a repository, add your first file |
| [Concepts](docs/concepts.md) | Sources, targets, state, and how a change is detected |
| [Comparison](docs/comparison.md) | statemate next to chezmoi, yadm, GNU Stow, and dotbot |
| [How-to guides](docs/README.md#how-to-guides) | Repository layout, multiple machines, encryption, system files, new machines, tips |
| [Configuration](docs/configuration.md) | `mate.yaml`, `.mate.yaml`, profiles, local overrides |
| [File Attributes](docs/attributes.md) | Every `#` suffix and what it does |
| [Templates](docs/templates.md) | Variables and functions available when rendering |
| [Secrets](docs/secrets.md) | Bitwarden references and the encrypted cache |
| [Scripts](docs/scripts.md) | Lifecycle scripts, frequency, timing |
| [Hooks](docs/hooks.md) | Commands run when files matching a pattern change |
| [Packages](docs/packages.md) | Declarative packages across brew, pacman, the AUR, and uv |
| [Command Reference](docs/commands/mate.md) | Every command and flag |

`mate <command> --help` gives the same reference in the terminal.

For a complete setup to borrow from, with profiles for macOS and Arch Linux,
encrypted files, Bitwarden secrets, scripts and hooks, see the author's
[dotfiles](https://github.com/subbeh/dotfiles).

## Configuration at a glance

```yaml
# mate.yaml
sources: [nvim, zsh, git, arch]

profiles:
  macos:
    detection:
      os: darwin
  work:
    extends: macos
    detection:
      hostname: "work-*"
    variables:
      email: "me@company.com"
    packages:
      - slack

age:
  identity: "~/.config/statemate/key.txt"
  recipients: ["age1..."]

packages:
  - fd
  - gh: [brew, pacman: github-cli]
  - ruff: uv

hooks:
  tmux:
    match: .config/tmux/**/*.conf
    do:
      - run: tmux source-file ~/.config/tmux/tmux.conf
```

Scripts sit in a `.matescripts/` directory and carry their schedule in the name, and
a source can deploy outside home, with ownership kept by `#owner-r:root`:

```
.matescripts/
  01-bootstrap.sh#once#before          runs once, before the files are written
  50-update.sh#weekly#after            runs weekly; tag it with "# Tags: update"
arch/
  .mate.yaml                           targets: {etc: /etc}
  etc#owner-r:root/keyd/default.conf   /etc/keyd/default.conf, owned by root
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
