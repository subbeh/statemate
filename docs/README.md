# Statemate Documentation

Statemate manages dotfiles, system configuration, and packages declaratively. You
describe what your machine should look like in a git repository; `mate apply`
makes it so.

## Guides

| Guide | Contents |
|-------|----------|
| [Getting Started](getting-started.md) | Install, create a repository, add your first file |
| [Concepts](concepts.md) | Sources, targets, state, and how a change is detected |
| [Configuration](configuration.md) | `mate.yaml`, `.mate.yaml`, profiles, local overrides |
| [File Attributes](attributes.md) | `#template`, `#encrypted`, `#import`, `#perm:`, and the rest |
| [Templates](templates.md) | Variables and functions available when rendering |
| [Secrets](secrets.md) | Bitwarden references and the encrypted cache |
| [Scripts](scripts.md) | Lifecycle scripts, frequency, timing |
| [Packages](packages.md) | Declarative packages across brew, pacman, and the AUR |
| [Command Reference](commands/mate.md) | Every command and flag |

## Quick reference

```bash
mate init                  # create mate.yaml
mate add ~/.zshrc          # bring a file under management
mate status                # what would change
mate diff                  # how it would change
mate apply                 # make it so
```

Status markers, as shown by `mate status`:

| Marker | `--short` | Meaning |
|--------|-----------|---------|
| `+` | `+N` | new — target does not exist yet |
| `~` | `~N` | modified — source changed since last apply |
| `!` | `!N` | conflict — target changed unexpectedly |
| `<` | `<N` | will be imported into the source (see [`#import`](attributes.md#import)) |
| | `?N` | orphaned — tracked, but no longer in any source |
| | `*N` | pending scripts |
| | `sN` | secrets needing a fetch |

## Contributing to these docs

`docs/commands/` is generated from the cobra command definitions by `make docs`.
Do not edit those files; change the `Short`/`Long` text in `internal/cli/` and
regenerate. CI fails if they are out of sync.

Everything else here is hand-written. A test
(`internal/cli/docs_test.go`) asserts that every file attribute, config key,
template function, and script frequency the code knows about appears in these
guides, so a new feature cannot ship undocumented.
