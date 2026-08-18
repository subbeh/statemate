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
| `aur` | An AUR helper | the helper on `$PATH` |
| `common` | Whichever of brew/pacman is present | — |

A manager that is not available is ignored entirely, so one config can serve both
a macOS and an Arch machine.

### `common`

`common` resolves to the **primary** manager — brew if present, otherwise pacman.
Use it for packages named identically everywhere:

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

Statemate does not add taps for you. Run `brew tap <owner>/<name>` yourself, or add
it to a [script](scripts.md).

### AUR helper

Set explicitly, or leave it to be detected:

```yaml
aur_helper: paru
```

AUR packages are queried separately from native ones, so an AUR package is not
reported as an unexpected extra under pacman.

## Where packages can be declared

All four are merged, and a package may appear in several:

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

```yaml
# nvim/.mate.yaml — only when this source is active
packages:
  common: [neovim]
```

`mate packages status` shows which source contributed each package, which is the
quickest way to find out why something is on the list.

## Versions

A package may pin a version with `@`:

```yaml
packages:
  brew: [node@20]
```

The part before `@` is the package name; the rest is passed to the manager as
written.

## Commands

```bash
mate packages status           # what is missing
mate packages status --all     # also list installed packages not in config
mate packages status -v        # include package descriptions
mate packages apply            # install what is missing
mate packages apply --prune    # also remove packages not in config
```

`mate apply` prompts to install missing packages as part of a normal run, and
`mate status` reports them under "Missing packages".

### `--all` and extras

Without `--all`, `mate packages status` reports only what is *missing*. Listing
extras — installed packages that no source declares — means asking the manager for
every explicitly-installed package, which takes about a second with `brew`. That
cost is only paid when you ask for it:

```
Use --all to also show packages not in config.
```

For the same reason `mate status` and `mate apply` never compute extras.

### `--prune`

`--prune` uninstalls anything not declared in your configuration. Since "not
declared" includes packages you installed deliberately but never wrote down, review
`mate packages status --all` first.

## What counts as installed

Only **explicitly installed** packages are considered, not those pulled in as
dependencies (`brew leaves --installed-on-request`, `pacman -Qen`). A package that
is present purely as another package's dependency is still reported as missing,
because removing that other package would take it with it.

Virtual packages and provides are resolved, so declaring `man` is satisfied by
`man-db`.
