# Getting Started

This page takes you from nothing to a managed `~/.zshrc` in about five minutes,
then sets up a second machine from the same repository.

## Install

```bash
# Homebrew (macOS, Linux)
brew install subbeh/tap/statemate

# Arch Linux (AUR)
yay -S statemate-bin            # or: paru -S statemate-bin

# Prebuilt binary: mate_{linux,darwin}_{amd64,arm64}.tar.gz
mkdir -p ~/.local/bin
curl -fsSL https://github.com/subbeh/statemate/releases/download/latest/mate_linux_amd64.tar.gz \
  | tar -xz -C ~/.local/bin mate

# From source (Go 1.25+)
go install github.com/subbeh/statemate/cmd/mate@latest
```

The binary is called `mate`. Check it with `mate version`.

## Create a repository

statemate keeps your configuration in an ordinary git repository, usually
`~/dotfiles`:

```bash
mkdir ~/dotfiles && cd ~/dotfiles
mate init
```

`mate init` asks for a config format (press Enter for YAML), then:

- writes a commented `mate.yaml`
- writes a `README.md` with setup instructions, unless you already have one
- runs `git init`, unless the directory is already in a git repository
- offers to **register** the directory: answer `y`

```
Register this directory as your dotfiles location? [Y/n]: y
Saved to ~/.config/statemate/mate.yaml
```

Registering writes `source_dir: "~/dotfiles"` to the
[local config](configuration.md#local-config), which is what lets every `mate`
command work from any directory. `mate config source-dir` prints the directory in
use.

## Create a source

A **source** is a directory in the repository whose contents are deployed
relative to your home directory, stow-style. Create one per application and
list it in `mate.yaml`:

```bash
mkdir zsh
```

```yaml
# mate.yaml
sources: [zsh]
```

Inside a source, paths mirror your home directory: `zsh/.zshrc` deploys to
`~/.zshrc`, and `nvim/.config/nvim/init.lua` deploys to `~/.config/nvim/init.lua`.
See [Organising Your Repository](repository-layout.md) for a layout that
scales.

## Add your first file

```bash
mate add ~/.zshrc
```

`mate add` copies the file into a source and leaves the original in place. It
asks which source to use, offering the ones listed in `mate.yaml`. Pick one with
the arrow keys, or press `/` to search:

```
? Select source:
  ▸ zsh
    nvim
    git
```

Skip the question with `--source zsh`, or set
[`default_source`](configuration.md#keys) in `mate.yaml`. The source must
already exist as a directory and be listed under `sources:`. `mate add` will
not create one.

```
~/dotfiles/
  mate.yaml
  zsh/
    .zshrc        →  ~/.zshrc
```

Nothing is reported as pending yet, because `~/.zshrc` and its source are
identical. statemate starts tracking it on the next `mate apply`:

```bash
mate apply          # 1 unchanged
```

## The apply cycle

From now on, edit the copy in the repository, not the deployed file. An edit
made to `~/.zshrc` directly turns into a conflict on the next apply.
`mate edit ~/.zshrc` opens the right one for you.

Then, in increasing order of commitment:

```bash
mate status      # which files would change
mate diff        # exactly how they would change
mate apply       # write them
```

```
$ mate status
  TARGET    SOURCE
~ ~/.zshrc  zsh

$ mate apply
applied 1
```

`mate apply --dry-run` walks the whole run, including scripts and packages,
without writing anything, and `-V` lists every file written.

If a deployed file was changed behind statemate's back, apply stops and asks what
to do:

```
Conflict: /home/you/.zshrc
  Target has been modified since last apply
  [o]verwrite, [i]mport, [s]kip, [d]iff, [a]bort:
```

`[i]mport` copies the change back into the repository. See
[Concepts](concepts.md) for how statemate tells which side changed.

Commit and push the repository as you would any other:

```bash
cd ~/dotfiles && git add -A && git commit -m "Add zsh" && git push
```

## Set up a second machine

Install `mate`, then clone and register the repository:

```bash
git clone git@github.com:you/dotfiles.git ~/dotfiles
cd ~/dotfiles && mate init      # finds mate.yaml, registers it
mate apply
```

On a machine that already has its own `~/.zshrc`, the first apply reports a
conflict for it, since statemate has no record of which version is newer. Answer
`[d]` to compare, then `[o]` to take the repository's version or `[i]` to keep
the machine's.

[Setting Up a New Machine](new-machine.md) covers the rest: encryption keys,
secrets, packages, and a bootstrap script.

## Next steps

- **Different files on different machines:** [Different Machines](machines.md)
- **Values that vary per machine:** [Templates](templates.md)
- **Private keys and tokens:** [Encryption and Secrets](encryption.md)
- **Install packages declaratively:** [Packages](packages.md)
- **Run something after applying:** [Scripts](scripts.md) and [Hooks](hooks.md)
- **Files under `/etc`:** [Managing System Files](system-files.md)
- **Editor, shell and status bar integration:** [Tips & Tricks](tips.md)
- **Something not working:** `mate doctor`, then [Troubleshooting](troubleshooting.md)
