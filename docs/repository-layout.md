# Organising Your Repository

statemate does not impose a layout beyond "sources are directories listed in
`mate.yaml`". This page describes a layout that scales from a handful of files to
a few hundred, taken from a [real repository](https://github.com/Subbeh/dotfiles)
that manages two operating systems and four machine profiles.

## One source per tool

Give every application its own source, named after it, and keep everything that
application needs inside: its configuration files, the packages that install it,
and the scripts that set it up.

```
~/dotfiles/
  mate.yaml
  git/
    .mate.yaml                      ← packages: git, lazygit, gh
    .config/git/config#template
  nvim/
    .mate.yaml                      ← packages: neovim, tree-sitter-cli
    .config/nvim/init.lua
    .matescripts/
      70-nvim-plugins.sh#monthly#after
  tmux/
    .mate.yaml
    .config/tmux/tmux.conf
    .local/bin#perm-r:755/
      tmux-sessionizer
```

```yaml
# mate.yaml
sources:
  - git
  - nvim
  - tmux
```

Why this works:

- **Removing a tool is one step.** Drop it from `sources:` and its files,
  packages, and scripts all go with it. `mate status` then lists its files as
  orphans for `mate clean`.
- **Profiles pick whole tools.** A profile adds the sources that only make sense
  on some machines (`hyprland` on Linux, `aerospace` on macOS), with no per-file
  bookkeeping. See [Different Machines](machines.md).
- **`mate packages status` explains itself.** Each package is listed with the
  source that declared it.
- **Scripts run when their tool changes.** An [`#onchange`](scripts.md#onchange)
  script fires when anything in *its* source changes, so a per-tool source gives
  it exactly the right trigger.

A `core` source is a good home for what does not belong to one tool: shell
profile files, `~/.local/bin` basics, common CLI packages.

## Per-source packages

A source's `.mate.yaml` declares the packages that tool needs, using `common` for
names shared by Homebrew and pacman and the manager-specific keys for the rest:

```yaml
# git/.mate.yaml
packages:
  common:
    - git
    - lazygit
  brew:
    - gh
  pacman:
    - github-cli
```

The packages are only considered while the source is active, so a macOS-only
source's casks never show up as missing on Linux. See [Packages](packages.md).

## Executable scripts in `~/.local/bin`

Files keep the mode they have in the repository, but git only records the
executable bit, and a fresh clone's umask decides the rest. Mark the directory
instead, so every script in it is deployed executable whatever the checkout
looks like:

```
tmux/.local/bin#perm-r:755/
  tmux-sessionizer
  __tmux_status_right
```

`#perm-r:` applies to the directory and everything beneath it. Several sources can
each contribute a `.local/bin#perm-r:755/` directory; they all deploy into the
same `~/.local/bin`.

## Repository-level files

Only directories listed in `sources:` are deployed. Everything else at the top of
the repository stays put, which makes the root a natural place for:

| Path | Purpose |
|------|---------|
| `mate.yaml` | The configuration |
| `.matescripts/` | Scripts that belong to no single source |
| `.mate/` (any name) | Files pulled in by [`include`](configuration.md#include), such as a shared colour palette |
| `README.md`, `install.sh` | Documentation and a [bootstrap script](new-machine.md) |
| `.gitattributes` | [Readable diffs for encrypted files](tips.md#readable-diffs-for-encrypted-files) |

Inside sources, some names are never deployed: `.git`, `.mate.yaml` (and `.yml`,
`.toml`) and `.matescripts`. Anything else you want to keep in a source but not
deploy, such as a README or a linter config, goes in `ignore`:

```yaml
# mate.yaml: applies to every source
ignore:
  - "*.md"
  - ".gitkeep"
```

```yaml
# nvim/.mate.yaml: applies to this source only
ignore:
  - .stylua.toml
  - selene.toml
```

## Ordering with numeric prefixes

Two things run in name order, so a numeric prefix is the way to control order:

- **Scripts** in `.matescripts/`, `00-` to `99-`, across all sources together.
- **Hooks**, when several are triggered by the same apply.

It is worth reserving bands, such as `00` for system setup, `50` for per-app
initialisation, and `70` for plugin installs that need the app configured first,
so a new script has an obvious number.

The same trick works for your own shell configuration: if `~/.config/zsh/` sources
`*.zsh` in order, `10-functions.zsh` and `50-fzf.zsh` can live in different
statemate sources and still load in the right sequence.

## Changing a file's attributes

Attributes are part of the filename, so changing one is a rename. State is
keyed by the target path, which carries no attributes, so a plain `git mv` that
only changes attributes is fine: `mate status` shows no change.

Two changes need a command, because they touch more than the name:

```bash
mate encrypt ssh/.ssh/config             # adds #encrypted and encrypts the content
mate decrypt ssh/.ssh/config#encrypted   # removes it and decrypts the content
mate rename  zsh/.zshrc .zshrc.local     # renames the deployed file too
```
