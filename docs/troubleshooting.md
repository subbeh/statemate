# Troubleshooting

## Start with `mate doctor`

```bash
mate doctor
```

It loads and validates the configuration, checks that every source directory
exists, that your age identity and recipients work, which package managers are
available, and that every active hook matches at least one file. It exits 1 if
anything is an error.

Other read-only commands that narrow a problem down:

| Command | Answers |
|---------|---------|
| `mate profile` | Which profile is active, why, and which sources it uses |
| `mate config source-dir` | Which repository `mate` is using |
| `mate managed <path>` | Which source a deployed file comes from, and whether it is active |
| `mate eval <file>` | What a template renders to |
| `mate apply --dry-run -V` | Everything an apply would do, without doing it |
| `mate scripts list`, `mate hooks list` | Scripts and hooks, and why they would or would not run |

## Common errors

### `no config file found in . (tried: mate.yaml, mate.yml, mate.toml)`

`mate` was run outside the repository, and no repository is registered.
statemate does not search parent directories. Run `mate init` once inside the
repository to register it, or set `STATEMATE_DIR`. See
[Finding the config](configuration.md#finding-the-config).

### `no sources configured`

`mate add` needs at least one source listed under `sources:` in `mate.yaml`,
with a matching directory:

```bash
mkdir zsh
```

```yaml
sources: [zsh]
```

### `source directory does not exist: …/zsh`

A directory named in `sources:` (or in a profile's `sources:`) is missing.
Create it, or remove it from the list. An empty source needs a file in it for git
to keep it, so after a fresh clone a source that only ever held a `.gitkeep` may
be missing.

### `source "nope" not in configured sources`

`mate add --source` names a source that is not in the active list. Under a
profile, the active list is the top-level `sources` plus the profile's own. Use
`-p <profile>` to add to a source only another profile uses.

### `no files match "zsh"; did you mean --source zsh?`

A positional argument to `apply`, `status` or `diff` is a *path* filter. To
narrow to a source, use `--source zsh` (or `-s zsh`).

### `conflicting targets detected`

```
Error: conflicting targets detected
  ~/.x defined in:
    - ~/dotfiles/zsh/.x
    - ~/dotfiles/other/.x
```

Two files deploy to the same place. Remove one, or tag them with different
[`#profile:`](attributes.md#profilename) attributes. If they already have
different profiles, check `mate profile`: with **no** active profile, nothing is
filtered, so every variant is deployed and they collide.

### `profile "work" extends unknown profile "base"`

`extends` names a profile that is not defined, often because it was renamed.
Profiles in the local config *replace* the repository's entirely, so a local
`profiles:` block can cause this too.

### `#import cannot be combined with #template`

Importing would write the rendered output over the template. Drop one of the two
attributes; see [`#import`](attributes.md#import).

### `… is #encrypted but no age identity is configured`

The file can only be deployed after it is decrypted. Configure `age.identity` or
`age.identity_command`, and check with `mate doctor`. See
[Encryption](encryption.md).

### `… /.mate.yaml: parsing YAML: …`

A source's `.mate.yaml` is rendered as a template and then parsed. If it uses
template syntax, the problem may be in the rendered result, for example a
multi-line value that lost its indentation. `mate eval zsh/.mate.yaml` shows
the rendered text.

### `hook "h": match is required` (and other hook errors)

Hooks are validated before anything is written. The message names the hook and
the problem: a missing `match`, an empty `do`, a step with both or neither of
`run` and `script`, a `script:` that matches no script, or a `run:` template
that does not parse.

### `secrets not configured (run 'mate secrets fetch')` / `secret not cached`

A template uses `bitwarden`, and the cache does not have the value yet. Run
`mate secrets fetch`. `mate apply` fetches missing secrets itself, but `mate
eval`, `status` and `diff` only read the cache.

### `not logged in to Bitwarden. Run: bw login`

The Bitwarden CLI has no account configured. Run `bw login` once; statemate
handles unlocking after that.

### `conflict on … needs an answer, but there is no terminal to ask on`

An unattended apply hit a conflict. Run `mate apply` interactively once, or pass
`--force` to overwrite. Scripts, hooks and package installs are skipped with a
warning in the same situation.

## Surprises

### A file is listed as modified (`~`) but I changed nothing

- **A template's inputs changed.** A variable, var file, profile, or secret it
  uses changed, so its rendered output differs. `mate diff` shows how.
- **A template fails to render.** `status` reports it as modified rather than
  stopping; `mate eval <file>` shows the error.
- **The target was deleted.** A tracked file whose target is gone is redeployed.
- **The mode differs** and the file has a `#perm:` attribute.

### The same file conflicts on every apply

An application rewrites the file itself, such as an editor's plugin lock file or
`~/.claude/settings.json`. Mark it [`#import`](attributes.md#import) so the
target's changes flow back into the repository instead.

### `Warning: N file(s) skipped (permission denied, use --sudo to check)`

`status`, `diff` and `check` read targets as you, so root-only files cannot be
compared. Add `--sudo`.

### A script runs again although it is `#once`

Script runs are recorded by the script's full path, including attributes.
Renaming, moving, or changing an attribute of a script makes it a new one.

### `mate status` against another repository lists everything as orphaned

There is one state database per user, shared by every repository. Files the
other repository deployed have no source in this one, so they look orphaned.
**Do not run `mate clean --all` there.**

### A setting in `~/.config/statemate/mate.yaml` has no effect

The local config honours only some keys, and a file that fails to parse is
ignored silently. See [Local config](configuration.md#local-config) for the list,
and check the YAML.

## Reporting a bug

Include the output of `mate version` and `mate doctor`, and the smallest
`mate.yaml` and source layout that reproduces it, at
<https://github.com/subbeh/statemate/issues>.
