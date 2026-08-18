# Getting Started

## Install

```bash
# Homebrew (macOS/Linux)
brew install subbeh/tap/statemate

# Arch (AUR)
paru -S statemate-bin

# Go
go install github.com/subbeh/statemate/cmd/mate@latest
```

Verify with `mate version`.

## Create a repository

Statemate keeps your configuration in an ordinary git repository. Create one and
initialise it:

```bash
mkdir ~/dotfiles && cd ~/dotfiles
git init
mate init
```

`mate init` writes a commented `mate.yaml`. The directory containing that file is
the **source directory** — every relative path in the config resolves against it.

Tell statemate where the repository lives, so commands work from anywhere:

```bash
mate config source-dir            # prints the resolved directory
```

If you run `mate` from outside the repository, set `source_dir` in
`~/.config/statemate/mate.yaml`:

```yaml
source_dir: "~/dotfiles"
```

## Add your first file

```bash
mate add ~/.zshrc
```

This copies `~/.zshrc` into a source directory and leaves the original in place.
`mate add` asks which source to use, creating one if needed. The result looks
like:

```
~/dotfiles/
  mate.yaml
  zsh/
    .zshrc        →  ~/.zshrc
```

A source directory is deployed **relative to your home directory**, stow-style:
`zsh/.zshrc` becomes `~/.zshrc`, and `nvim/.config/nvim/init.lua` becomes
`~/.config/nvim/init.lua`.

List the sources in `mate.yaml` so statemate knows to scan them:

```yaml
sources: [zsh, nvim]
```

## The apply cycle

Three commands, in increasing order of commitment:

```bash
mate status      # which files would change
mate diff        # exactly how they would change
mate apply       # write them
```

`mate apply` records a hash of every file it writes, which is how it later tells
"you edited the source" apart from "something else edited the target". See
[Concepts](concepts.md) for what that buys you.

Add `--dry-run` to `apply` to walk the whole run — including scripts and packages
— without writing anything.

## Next steps

- Deploy different files on different machines → [Profiles](configuration.md#profiles)
- Substitute values per machine → [Templates](templates.md)
- Store a private key in the repository → [File Attributes](attributes.md#encrypted) and [Secrets](secrets.md)
- Install packages declaratively → [Packages](packages.md)
- Run a command after applying → [Scripts](scripts.md)
