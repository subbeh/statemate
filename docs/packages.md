# Packages

Packages are declared in configuration and installed by the native package
manager.

```yaml
packages:
  brew: [ripgrep, fd, jq]
  pacman: [keyd, base-devel]
  aur: [statemate-bin]
  common: [git, curl]
```

## Supported managers

| Key | Manager | Detected by |
|-----|---------|-------------|
| `brew` | Homebrew | `brew` on `$PATH` |
| `pacman` | Arch's pacman | `pacman` on `$PATH` |
| `aur` | An AUR helper (`yay` or `paru`) | the helper on `$PATH` |
| `common` | Whichever of brew/pacman is present | — |

Homebrew casks go under `brew:` too; there is no separate `cask:` key.

A manager that is not available is ignored entirely, so one config can serve both
a macOS and an Arch machine.

### `common`

`common` resolves to the **primary** manager: brew if present, otherwise pacman.
It never resolves to the AUR. Use it for packages named identically everywhere:

```yaml
packages:
  common: [git, ripgrep, fd]
```

If the same package is also listed under a specific manager, the two entries merge
rather than duplicating.

### Homebrew taps

A formula from a third-party tap can be declared either by its fully-qualified
name or bare, and both are recognised as installed:

```yaml
packages:
  brew:
    - jamf/internal-tap/hermes    # fully qualified
    - hermes                      # equivalent for an installed formula
```

Prefer the qualified form: `brew install` needs it to find a formula that is not
already tapped, whereas the bare name only works once the tap is added.

Statemate accepts either because Homebrew itself is inconsistent — `brew list
--formula` reports a tap formula under its bare name while `brew leaves` reports it
fully qualified. One consequence is that two taps providing the same formula name
cannot be told apart when comparing bare names.

statemate does not run `brew tap` itself, but `brew install owner/tap/formula`
taps automatically, which is another reason to prefer the qualified form.

### AUR helper

When `aur_helper` is unset, statemate uses `yay` if it is on `$PATH`, otherwise
`paru`. Set it to choose explicitly:

```yaml
aur_helper: paru
```

Without either helper, `aur:` packages are ignored, like any unavailable manager.

AUR packages are queried separately from native ones, so an AUR package is not
reported as an unexpected extra under pacman.

## Where packages can be declared

All of these are merged, and a package may appear in several:

```yaml
# mate.yaml — every machine
packages:
  brew: [ripgrep]

profiles:
  work:
    packages:                 # only under the work profile
      brew: [awscli]

include:
  - packages.yaml             # packages: and variables: only
```

Profile packages are inherited along `extends`, like sources and variables: a
profile that extends `base` gets `base`'s packages too. A profile can also have
its own `include:`.

```yaml
# nvim/.mate.yaml — only when this source is active
packages:
  common: [neovim]
```

`mate packages status` shows where each package was declared, which is the
quickest way to find out why something is on the list: `config` for `mate.yaml`
and its includes, `profile:<name>` for a profile, or the source's name.

A `packages:` key in your [local config](configuration.md#local-config) replaces
the repository's top-level packages rather than adding to them.

## Versions

statemate does not pin versions. A package name is passed to the manager exactly
as written, so Homebrew's versioned formulae work like any other package:

```yaml
packages:
  brew: [node@20, python@3.12]
```

## Commands

```bash
mate packages status           # every declared package: ✓ installed, + missing
mate packages status --all     # also list installed packages not in config
mate packages status -v        # include package descriptions
mate packages apply            # install what is missing
mate packages apply --prune    # also remove packages not in config
mate packages apply -y         # without the confirmation prompt
```

`mate packages apply` asks once per manager before installing.

`mate apply` also prompts, after writing files, once per manager:

```
Missing pacman packages: keyd, tlp
Install? [y/N]
```

`--force` answers yes. `--dry-run` lists the packages without prompting. Without
a terminal to prompt on, installs are skipped with a warning.

`mate apply <path>` skips packages entirely, and `mate apply --source <name>`
only considers that source's packages. `mate status` reports missing packages
under "Missing packages".

The commands statemate runs:

| Manager | Install | Remove (`--prune`) |
|---------|---------|--------------------|
| Homebrew | `brew install …` | `brew uninstall …` |
| pacman | `sudo pacman -S --noconfirm …` | `sudo pacman -R --noconfirm …` |
| AUR | `<helper> -S --noconfirm …` | `<helper> -R --noconfirm …` |

All of a manager's missing packages go into one command, so one bad name can
fail the whole batch.

### `--all` and extras

Without `--all`, `mate packages status` reports only what is *missing*. Listing
extras — installed packages that no source declares — means asking the manager for
every explicitly-installed package, which takes about a second with `brew`. That
cost is only paid when you ask for it:

```
Use --all to also show packages not in config.
```

For the same reason `mate status` and `mate apply` never compute extras.

### `-v` and `<unknown>`

`-v` adds a DESCRIPTION column. Two kinds of empty look different on purpose:

```
 ✓  font-hack-nerd-font    brew   macos
 +  github-cli             brew   git     <unknown>
```

An **empty** description means the package manager knows the package but publishes
no description for it (common for font casks). **`<unknown>`** means the manager
matched no such package at all, which usually points at a mistake in your config —
a typo (`github-cli` where Homebrew calls it `gh`), a name that only exists on
another platform (`man`, `sudo` in a `common:` list on macOS), or a package that
has since been renamed or removed. Such a package can never be installed. It also
fails the install of every other package in the same batch until the name is
corrected.

`<unknown>` is only reported for Homebrew; pacman and the AUR leave the
description empty.

A Homebrew alias is described under the formula it is installed as, so `kubectl`
shows the description of `kubernetes-cli`. An alias of a formula that is *not*
installed is still reported as `<unknown>`, because the local name lists brew
publishes contain no aliases — declaring the canonical name avoids the ambiguity.

### `--prune`

`--prune` uninstalls anything not declared in your configuration. Since "not
declared" includes packages you installed deliberately but never wrote down, review
`mate packages status --all` first.

## What counts as installed

A declared package counts as installed if it is present at all, including as a
dependency of something else.

Finding **extras**, for `--all` and `--prune`, is stricter. Only packages you
installed explicitly count (`brew leaves --installed-on-request` plus every cask,
`pacman -Qen`, and `<helper> -Qmtt` for the AUR), so dependencies are never
offered for removal.

Virtual packages and provides are resolved, so declaring `man` is satisfied by
`man-db`.

Homebrew aliases count as installed: declaring `kubectl` is satisfied by
`kubernetes-cli`, and `az` by `azure-cli`. Only the canonical name appears in
`brew list`, so the alias is resolved through the `opt/` link Homebrew creates for
it.
