# Packages

Packages are declared in configuration and installed by the native package
manager. `packages:` is a list with one package per item, so it can be kept
sorted by name:

```yaml
packages:
  - gh: [brew, pacman: github-cli]   # brew gh, pacman github-cli
  - gnu-sed: brew                    # brew only
  - paru: aur                        # the AUR only
  - ripgrep                          # the system manager, on every machine
  - tpack: [brew, aur: tpack-bin]    # brew tpack, and tpack-bin from the AUR
```

- **A bare name** goes to the system manager: brew if it is present, otherwise
  pacman. It never goes to the AUR.
- **A name followed by managers** is installed with exactly those managers: one
  (`gnu-sed: brew`) or a list (`[brew, aur]`). An entry of the list can rename
  the package for that manager (`pacman: github-cli`).

The same item written in block style:

```yaml
packages:
  - tpack:
      - brew
      - aur: tpack-bin
```

In TOML the list is an array: `packages = ["ripgrep", {gnu-sed = "brew"},
{gh = ["brew", {pacman = "github-cli"}]}]`.

A package that is installed with both pacman and the AUR is two packages, so
listing `pacman` and `aur` on one item is an error. So is an unknown manager, a
manager listed twice, or a map with several names in one item. The manager-keyed
lists of earlier versions (`brew: [...]`, `common: [...]`) are rejected with a
hint showing the list form.

## Supported managers

| Manager | | Detected by |
|---------|-|-------------|
| `brew` | Homebrew | `brew` on `$PATH` |
| `pacman` | Arch's pacman | `pacman` on `$PATH` |
| `aur` | An AUR helper (`yay` or `paru`) | the helper on `$PATH` |

Homebrew casks are `brew` packages too; there is no separate `cask` manager.

A manager that is not available is ignored entirely, so one config can serve both
a macOS and an Arch machine. A package that resolves to the same manager and name
in several places is listed once, with every place that declared it.

### Homebrew taps

A formula from a third-party tap can be declared either by its fully-qualified
name or bare, and both are recognised as installed:

```yaml
packages:
  - hashicorp/tap/terraform: brew   # fully qualified
  - terraform: brew                 # equivalent for an installed formula
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

Without either helper, `aur` packages are ignored, like any unavailable manager.

AUR packages are queried separately from native ones, so an AUR package is not
reported as an unexpected extra under pacman.

## Where packages can be declared

All of these are merged, and a package may appear in several:

```yaml
# mate.yaml — every machine
packages:
  - ripgrep

profiles:
  work:
    packages:                 # only under the work profile
      - awscli

include:
  - packages.yaml             # packages: and variables: only
```

Profile packages are inherited along `extends`, like sources and variables: a
profile that extends `base` gets `base`'s packages too. A profile can also have
its own `include:`.

```yaml
# nvim/.mate.yaml — only when this source is active
packages:
  - neovim
```

`mate packages list` shows where each package was declared, which is the
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
mate packages list             # every declared package: ✓ installed, + missing
mate packages list --all       # also list installed packages not in config
mate packages list -d          # include package descriptions
mate packages apply            # install what is missing
mate packages apply --prune    # also remove packages not in config
mate packages apply -y         # without the confirmation prompt (or --noconfirm)
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

Without `--all`, `mate packages list` reports only what is *missing*. Listing
extras — installed packages that no source declares — means asking the manager for
every explicitly-installed package, which takes about a second with `brew`. That
cost is only paid when you ask for it:

```
Use --all to also show packages not in config.
```

For the same reason `mate status` and `mate apply` never compute extras.

### `-d` and `<unknown>`

`-d` adds a DESCRIPTION column. Two kinds of empty look different on purpose:

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
`mate packages list --all` first.

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
