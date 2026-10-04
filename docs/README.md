# Statemate

Statemate manages dotfiles, system configuration, and packages declaratively. You
describe what your machines should look like in a git repository; `mate apply`
makes it so.

```
~/dotfiles/
  mate.yaml                          sources, profiles, packages, hooks
  zsh/.zshrc                      →  ~/.zshrc
  git/.config/git/config#template →  ~/.config/git/config, rendered per machine
  ssh/.ssh/config#encrypted       →  ~/.ssh/config, decrypted
  arch/etc#owner-r:root/keyd/…    →  /etc/keyd/…, owned by root
  nvim/.mate.yaml                    packages: { common: [neovim] }
  nvim/.matescripts/
    70-plugins.sh#monthly#after      run after applying, once a month
```

- **Stow-style sources.** One directory per tool, deployed relative to your home
  directory or anywhere else.
- **Knows which side changed.** Two hashes per file tell your edit in the
  repository apart from an application rewriting its own config, so nothing is
  overwritten silently.
- **One repository, many machines.** Auto-detected profiles pick sources, files,
  variables, and packages per machine.
- **Templates and secrets.** Go templates with sprig, age encryption, and
  Bitwarden references cached locally.
- **Packages.** Homebrew, pacman, and the AUR, declared next to the config they
  belong to.
- **Scripts and hooks.** Run setup once, on a schedule, or when particular files
  change.
- **System files.** `/etc` and friends, with ownership and sudo handled for you.

## Start here

| Guide | Contents |
|-------|----------|
| [Getting Started](getting-started.md) | Install, create a repository, add your first file, set up a second machine |
| [Concepts](concepts.md) | Sources, targets, state, and how a change is detected |

## How-to guides

| Guide | Contents |
|-------|----------|
| [Organising Your Repository](repository-layout.md) | A layout that scales: one source per tool, packages and scripts alongside |
| [Different Machines](machines.md) | Profiles, per-machine files, and templates |
| [Encryption and Secrets](encryption.md) | Creating an age key, encrypting files, Bitwarden secrets |
| [Managing System Files](system-files.md) | `/etc`, ownership, sudo, and reloading services |
| [Setting Up a New Machine](new-machine.md) | The first apply, and a bootstrap script |
| [Tips & Tricks](tips.md) | Neovim, tmux, git, shell, and status bar integration |
| [Troubleshooting](troubleshooting.md) | `mate doctor`, common errors, and surprises |

## Reference

| Reference | Contents |
|-----------|----------|
| [Configuration](configuration.md) | `mate.yaml`, `.mate.yaml`, profiles, local overrides |
| [File Attributes](attributes.md) | `#template`, `#encrypted`, `#import`, `#perm:`, and the rest |
| [Templates](templates.md) | Variables and functions available when rendering |
| [Secrets](secrets.md) | Bitwarden references and the encrypted cache |
| [Scripts](scripts.md) | Lifecycle scripts, frequency, timing |
| [Hooks](hooks.md) | Commands run when files matching a pattern change |
| [Packages](packages.md) | Declarative packages across brew, pacman, and the AUR |
| [Command Reference](commands/mate.md) | Every command and flag |

## Quick reference

```bash
mate init                  # create or register a repository
mate add ~/.zshrc          # bring a file under management
mate edit ~/.zshrc         # edit its source
mate status                # what would change
mate diff                  # how it would change
mate apply                 # make it so
mate doctor                # check the setup
```

Status markers, as shown by `mate status`:

| Marker | `--short` | Meaning |
|--------|-----------|---------|
| `+` | `+N` | new: the target does not exist yet |
| `~` | `~N` | modified: the source changed since the last apply |
| `!` | `!N` | conflict: the target changed unexpectedly |
| `<` | `<N` | will be imported into the source (see [`#import`](attributes.md#import)) |
| | `?N` | orphaned: tracked, but no longer in any source |
| | `*N` | pending scripts |
| | `sN` | secrets needing a fetch |
