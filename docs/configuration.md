# Configuration

Statemate reads three levels of configuration:

| File | Purpose |
|------|---------|
| `mate.yaml` in the repository | The configuration itself, committed and shared |
| `~/.config/statemate/mate.yaml` | Machine-local overrides, not committed |
| `.mate.yaml` in a source directory | Settings scoped to one source |

The repository config may be YAML or TOML: `mate.yaml`, `mate.yml` and
`mate.toml` are looked for in that order, and the same goes for `.mate.yaml` in a
source. The local config is always `mate.yaml`. The **directory containing the
config file is the source directory**, and relative paths in `sources`,
`var_files` and `include` resolve against it. Other paths, such as
`age.identity` and `secrets_cache`, should be absolute or start with `~/`.

Unknown keys are ignored without a warning, so a misspelt key simply has no
effect. `mate doctor` and `mate check` catch most of the mistakes that matter.

### Finding the config

statemate does not search parent directories. It uses the first of:

1. `--config` / `-c` on the command line
2. `$STATEMATE_DIR`
3. `source_dir` in the [local config](#local-config)
4. the current directory

`mate init` writes `source_dir` for you, so after setup `mate` works from any
directory. `mate config source-dir` prints the directory in use.

## `mate.yaml`

```yaml
sources: [zsh, nvim, git]
default_source: misc
target_base: "~"

profiles:
  work:
    extends: base
    detection:
      hostname: "work-*"

age:
  identity: "~/.config/statemate/key.txt"
  recipients: ["age1..."]

variables:
  email: "you@example.com"

variable_commands:
  gpg_key: "gpg --list-secret-keys --with-colons | awk -F: '/^fpr/{print $10; exit}'"

var_files:
  - .matedata/secrets.yaml

packages:
  brew: [ripgrep, fd]

include:
  - packages.yaml

ignore:
  - "*.md"
  - .DS_Store

editor: nvim
diff_tool: delta
aur_helper: paru
secrets_cache: "~/.local/state/statemate/secrets.age"
```

### Keys

| Key | Type | Description |
|-----|------|-------------|
| `sources` | list | Source directories to scan, relative to the source directory or absolute |
| `default_source` | string | Source `mate add` uses when none is given |
| `target_base` | string | Root to deploy into. Defaults to `~` |
| `profiles` | map | Profile definitions — see [Profiles](#profiles) |
| `profile` | string | Force a profile, skipping detection |
| `source_dir` | string | Where to find `mate.yaml`. Only meaningful in the local config |
| `editor` | string | Editor for `mate edit`, with any arguments (`code --wait`). Overrides `$VISUAL` and `$EDITOR`; falls back to `vi` |
| `diff_tool` | string | External diff tool for `mate diff`, with any arguments. Run as `<tool> <old> <new>` |
| `age` | map | Encryption settings — see [age](#age) |
| `variables` | map | Values available to templates as `.Vars` |
| `variable_commands` | map | Variables whose values come from shell commands, run at load |
| `var_files` | list | YAML/TOML files to load variables from |
| `packages` | map | Packages to install — see [Packages](packages.md) |
| `hooks` | map | Commands to run when matching files change — see [Hooks](hooks.md) |
| `include` | list | Files to merge `packages` and `variables` from. Also valid inside a profile |
| `ignore` | list | gitignore-style patterns excluded from scanning |
| `aur_helper` | string | AUR helper binary, `yay` or `paru`. If unset, `yay` is used if installed, else `paru` |
| `secrets_cache` | string | Path to the encrypted secrets cache |

### `age`

```yaml
age:
  identity: "~/.config/statemate/key.txt"
  identity_command: "op read op://Private/age/key"
  recipients:
    - age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

`identity` is a path to an age private key file, as written by `age-keygen`, used
for decryption. Alternatively `identity_command` runs a command that prints the
key on stdout, which is useful when the key lives in a password manager rather
than on disk. If both are set, `identity_command` wins. Its output must be the
bare `AGE-SECRET-KEY-…` line, without the comments `age-keygen` writes.

`recipients` are the public keys (`age1…`) files are encrypted *to*. Include every
key that needs to read them. SSH keys are not supported as recipients.

[Encryption](encryption.md) walks through creating a key and storing it.

### `variable_commands`

Runs a shell command and stores its trimmed output as a variable:

```yaml
variable_commands:
  hostname_short: "hostname -s"
  gpg_key: "gpg --card-status | awk '/Signature key/{print $NF}'"
```

Each command runs through `sh -c` whenever templates are rendered, which is most
commands, so keep them fast. A failing command aborts the command you ran.

### `var_files`

Loads variables from YAML or TOML files, merged in order after `variables`:

```yaml
var_files:
  - .matedata/vars.yaml
  - .matedata/secrets.yaml   # resolves .matedata/secrets.yaml#encrypted too
```

Paths are relative to the source directory, or absolute, or start with `~/`. A
missing file is skipped without error, and so is a file that is not `.yaml`,
`.yml` or `.toml`. If the plain path does not exist, statemate looks for the same
path with an `#encrypted` suffix and decrypts it, so the config need not change
when you encrypt a var file.

### `include`

Splits **packages and variables** across files:

```yaml
include:
  - packages.yaml
  - .matedata/secrets.yaml#encrypted
```

```yaml
# packages.yaml
packages:
  brew: [ripgrep, fd]
variables:
  email: "me@example.com"
```

Only `packages` and `variables` are read from an included file; other keys are
ignored. Package lists are appended (duplicates dropped) and variables merged, so
an include adds to what `mate.yaml` already declares. Where both define the same
variable, the include wins. A missing include file is skipped.

An included file may be `#encrypted`, and as with `var_files` the suffix is found
automatically if the plain path does not exist. This is the usual way to keep a
list of secret-bearing variables in the repository.

Profiles accept their own `include`, merged into that profile only:

```yaml
profiles:
  work:
    include: [work-packages.yaml]
```

Every profile's includes are read when the config loads, including inactive
profiles. An `#encrypted` include therefore needs a working age identity on every
machine, not only on those that use that profile.

### `ignore`

gitignore-style patterns for paths statemate should not treat as managed files:

```yaml
ignore:
  - "*.md"
  - "!**/.claude/**/*.md"     # but keep these
  - .DS_Store
```

Applies to all sources. For patterns that concern one source only, use `ignore` in
that source's `.mate.yaml`, where patterns match the path inside the source.

How patterns differ from git's:

- `!` re-includes a file, with the last matching pattern winning. It cannot
  re-include anything inside an ignored *directory*, because statemate does not
  descend into one.
- `?` is a literal character, not a wildcard.
- A pattern with a `/` in the middle is not anchored: `etc/foo` also matches
  `x/etc/foo`.
- Top-level patterns match `<source>/<path>`, so a leading `/` anchors at the
  source name: `/zsh/notes.txt`.

`.git`, `.matescripts`, and `.mate.yaml` (`.yml`, `.toml`) are never deployed,
so they need no pattern. `.mateignore` files are no longer supported.

## Profiles

A profile bundles the sources, variables, and packages for a kind of machine.

```yaml
profiles:
  base:
    sources: [common, shell]
    variables:
      email: "me@example.com"

  work:
    extends: base
    detection:
      hostname: ["work-laptop", "work-*"]
    variables:
      email: "me@company.com"
    packages:
      brew: [slack, awscli]

  arch:
    extends: base
    detection:
      mode: and
      os: linux
      command: "test -f /etc/arch-release"
    packages:
      pacman: [keyd]
```

### Detection

The first matching profile wins. Profiles are sorted by how deep they sit in an
`extends` chain, deepest first, and alphabetically within the same depth. So
every profile that extends something is tried before every profile that does
not, which makes OS-level profiles natural fallbacks.

| Field | Matches |
|-------|---------|
| `hostname` | The hostname. A pattern containing `*` is a glob; anything else must match exactly. A list matches any entry |
| `user` | `$USER` (or `$USERNAME`), with the same rules as `hostname` |
| `os` | `linux`, `darwin`, … (Go's `GOOS`). A single value |
| `arch` | `amd64`, `arm64`, … (Go's `GOARCH`). A single value |
| `command` | A shell command, run with `sh -c`; matches when it exits 0 |
| `mode` | `or` (default, where any field matches) or `and` (all must match) |

A profile with no `detection` block, or an empty one, is never auto-detected;
select it with `--profile` or `profile:` in the config. This is the normal shape
for a `base` profile that exists only to be extended.

When no profile matches, only the top-level `sources` and `variables` apply, and
[`#profile:`](attributes.md#profilename) files are **not** filtered at all. Make
sure every machine matches some profile.

### Inheritance

`extends` names a single parent, and chains may be several levels deep. Along the
chain:

- **sources** are added: top-level `sources`, then each ancestor's, then the
  profile's own, without duplicates
- **variables** are merged, the child winning where both define one
- **packages** are added, like sources
- **detection** is not inherited

Inheritance is respected everywhere profiles are used: file filtering by
[`#profile:`](attributes.md#profilename), script and hook filtering, and source
resolution.

### Selecting a profile

In precedence order:

1. `--profile` / `-p` on the command line
2. `profile:` in the local config or `mate.yaml`
3. `$STATEMATE_PROFILE`
4. Automatic detection

`mate profile` shows which profile is active, how it was chosen, and which sources
it resolves to.

Forcing a profile that is not defined is not an error. It selects no profile's
sources or variables, and only files tagged with that exact name pass the
`#profile:` filter.

## Local config

`~/.config/statemate/mate.yaml` (or `$XDG_CONFIG_HOME/statemate/mate.yaml`) holds
machine-specific settings that should not be committed:

```yaml
source_dir: "~/dotfiles"
profile: work
editor: nvim
```

`source_dir` is what lets `mate` run from any directory. Without it, statemate
looks for `mate.yaml` in the current directory.

Only a subset of keys is honoured here: `sources`, `default_source`,
`target_base`, `profile`, `editor`, `age`, `variables`, `var_files`,
`variable_commands`, `packages`, `profiles`, `diff_tool`, and `hooks`. Others,
including `include`, `ignore`, `aur_helper` and `secrets_cache`, are ignored.

Each key **replaces** the repository's value; nothing is merged. A local
`variables:` with one entry drops all of the repository's top-level variables,
and a local `profiles:` replaces every profile. The exception is `hooks`, which
are merged by name: a local hook replaces a repository hook of the same name, and
the rest are kept. See [Hooks](hooks.md).

A local config that fails to parse is ignored without a warning. Run
`mate doctor` if local settings seem to have no effect.

## Source directory config

A `.mate.yaml` inside a source directory configures that source alone:

```yaml
# arch/.mate.yaml
profile: arch
owner: root
group: root
perm: "644"

targets:
  etc: /etc

ignore:
  - "*.md"

packages:
  pacman: [keyd]

generate:
  - target: .config/app/generated.conf
    mode: "600"
    content: |
      key = {{ .Vars.api_key }}
```

| Key | Description |
|-----|-------------|
| `profile` | Default [`#profile:`](attributes.md#profilename) for every file in the source that has none of its own |
| `target_base` | Deploy this source relative to a different root. Disables `targets` |
| `targets` | Map a top-level subdirectory to a path |
| `ignore` | Patterns as for [`ignore`](#ignore), matched against the path inside this source |
| `owner` | Default owner for files and directories in the source |
| `group` | Default group for files and directories |
| `perm` | Default mode for files, octal string. Directories are not affected |
| `packages` | Packages this source needs |
| `generate` | Files created from inline content — see below |
| `hooks` | Hooks matching only this source's files — see [Hooks](hooks.md) |

Attributes in file and directory names take precedence over `owner`, `group`
and `perm`.

`profile` here only tags files. The source's `packages`, `generate` entries and
hooks are not filtered by it; to keep a whole source off a machine, list it under
that machine's profile `sources` instead.

`.mate.yaml` is itself rendered as a [template](templates.md) before it is parsed,
so `{{ .Vars.workspace }}` works inside it, and so do loops:

```yaml
# ssh/.mate.yaml: one private key file per name in the sshKeys variable
generate:
  {{- range $key := (var "sshKeys") }}
  - target: ".ssh/keys.d/id_{{ $key }}"
    mode: "0600"
    content: |
{{ indent 6 (bitwarden $key "ssh" "private") }}
  {{- end }}
```

A `.mate.yaml` that fails to parse or render is an error that names the file,
rather than being skipped.

### `targets`

Maps a subdirectory of the source onto another path, which is how system files
are managed. Only the first component of a file's path is looked up, so `etc`
maps the whole `etc/` subtree; a key such as `etc/keyd` never matches. Values may
start with `~` and may use template variables. See
[Managing System Files](system-files.md).

```yaml
targets:
  etc: /etc
```

```
arch_root/
  .mate.yaml
  etc/
    issue                    →  /etc/issue
    keyd/default.conf        →  /etc/keyd/default.conf
```

Files outside the mapped directories still deploy to the source's normal target
base. `targets` is ignored if the same `.mate.yaml` sets `target_base`.

Directories outside your writable tree are created with sudo as needed.

### `generate`

Creates a file from content in the config rather than from a file in the
repository:

```yaml
generate:
  - target: .config/app/config.toml
    mode: "600"
    profile: work
    content: |
      token = "{{ bitwarden "app" "field" "token" }}"
```

| Field | Description |
|-------|-------------|
| `target` | Path to write, relative to the source's target base, absolute, or starting with `~` |
| `mode` | File mode, octal string. Default `0644` |
| `profile` | Only generate under this profile (or one extending it) |
| `content` | The file content |

`content` is rendered as part of the whole `.mate.yaml`, before YAML parsing, so
multi-line values must be indented to stay inside the block scalar. That is what
`indent 6` does in the example above. `targets` and `ignore` do not apply to
generated files.

Useful for short files that would otherwise need their own source file, and for
content assembled from variables or secrets.

## Environment variables

| Variable | Effect |
|----------|--------|
| `STATEMATE_DIR` | Where to find `mate.yaml`. Overrides `source_dir` and the current directory, but not `--config` |
| `STATEMATE_PROFILE` | Selects a profile, below config but above detection |
| `XDG_CONFIG_HOME` | Where the local config lives. Default `~/.config` |
| `XDG_DATA_HOME` | Where `state.db` lives. Default `~/.local/share` |
| `XDG_STATE_HOME` | Where the secrets cache lives. Default `~/.local/state` |
| `VISUAL`, `EDITOR` | Editor for `mate edit`, if `editor:` is unset |
| `USER`, `USERNAME` | The user for profile detection and `.Username` |

`STATEMATE_DIR` is handy for working against a second repository without touching
your config:

```bash
STATEMATE_DIR=~/other-dotfiles mate status
```

Scripts also receive [their own environment variables](scripts.md#environment),
and hooks [theirs](hooks.md#environment).
