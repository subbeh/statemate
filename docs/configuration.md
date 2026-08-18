# Configuration

Statemate reads three levels of configuration:

| File | Purpose |
|------|---------|
| `mate.yaml` in the repository | The configuration itself, committed and shared |
| `~/.config/statemate/mate.yaml` | Machine-local overrides, not committed |
| `.mate.yaml` in a source directory | Settings scoped to one source |

YAML and TOML are both accepted; `mate.yaml`, `mate.yml` and `mate.toml` are
looked for in that order. The **directory containing the config file is the source
directory**, and every relative path resolves against it.

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
| `editor` | string | Editor for `mate edit`. Overrides `$VISUAL` and `$EDITOR` |
| `diff_tool` | string | External diff tool for `mate diff`, e.g. `delta`, `difft` |
| `age` | map | Encryption settings — see [age](#age) |
| `variables` | map | Values available to templates as `.Vars` |
| `variable_commands` | map | Variables whose values come from shell commands, run at load |
| `var_files` | list | YAML/TOML files to load variables from |
| `packages` | map | Packages to install — see [Packages](packages.md) |
| `include` | list | Files to merge `packages` and `variables` from |
| `ignore` | list | gitignore-style patterns excluded from scanning |
| `aur_helper` | string | AUR helper binary, e.g. `paru` or `yay`. Auto-detected if unset |
| `secrets_cache` | string | Path to the encrypted secrets cache |

### `age`

```yaml
age:
  identity: "~/.config/statemate/key.txt"
  identity_command: "op read op://Private/age/key"
  recipients:
    - age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

`identity` is a path to an age private key, used for decryption. Alternatively
`identity_command` runs a command that prints the key on stdout — useful when the
key lives in a password manager rather than on disk.

`recipients` are the public keys files are encrypted *to*. Include every machine
that needs to read them.

### `variable_commands`

Runs a shell command and stores its trimmed output as a variable:

```yaml
variable_commands:
  hostname_short: "hostname -s"
  gpg_key: "gpg --card-status | awk '/Signature key/{print $NF}'"
```

These run every time configuration is loaded, so keep them fast. A failing command
aborts the load.

### `var_files`

Loads variables from YAML or TOML files, merged in order after `variables`:

```yaml
var_files:
  - .matedata/vars.yaml
  - .matedata/secrets.yaml   # resolves .matedata/secrets.yaml#encrypted too
```

Paths are relative to the source directory. A missing file is skipped without
error. If the plain path does not exist, statemate looks for the same path with an
`#encrypted` suffix and decrypts it — so the config need not change when you
encrypt a var file.

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
an include adds to what `mate.yaml` already declares rather than replacing it.

An included file may be `#encrypted`, and as with `var_files` the suffix is found
automatically if the plain path does not exist. This is the usual way to keep a
list of secret-bearing variables in the repository.

Profiles accept their own `include`, merged into that profile only:

```yaml
profiles:
  work:
    include: [work-packages.yaml]
```

### `ignore`

gitignore-style patterns for paths statemate should not treat as managed files:

```yaml
ignore:
  - "*.md"
  - .git/
  - .DS_Store
```

Applies to all sources. For patterns that concern one source only, use `ignore` in
that source's `.mate.yaml`. (Note: `.mateignore` files are no longer supported.)

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

The first matching profile wins. Profiles are tested with more specific ones
first — a profile that extends another is tried before its parent — and
alphabetically within the same depth.

| Field | Matches |
|-------|---------|
| `hostname` | Hostname, exact or glob. A list matches any entry |
| `user` | `$USER`, exact or glob. A list matches any entry |
| `os` | `linux`, `darwin`, … (Go's `GOOS`) |
| `arch` | `amd64`, `arm64`, … (Go's `GOARCH`) |
| `command` | A shell command; matches when it exits 0 |
| `mode` | `or` (default — any field matches) or `and` (all must match) |

A profile with no `detection` block is never auto-detected; select it with
`--profile` or `profile:` in the config. This is the normal shape for a `base`
profile that exists only to be extended.

### Inheritance

`extends` merges the parent's sources, variables, and packages, with the child
winning on conflicts. Inheritance is respected everywhere profiles are used: file
filtering by [`#profile:`](attributes.md#profilename), script filtering, and
source resolution.

### Selecting a profile

In precedence order:

1. `--profile` / `-p` on the command line
2. `profile:` in the local config or `mate.yaml`
3. `$STATEMATE_PROFILE`
4. Automatic detection

`mate profile` shows which profile is active, how it was chosen, and which sources
it resolves to.

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
`variable_commands`, `packages`, `profiles`, and `diff_tool`.

## Source directory config

A `.mate.yaml` inside a source directory configures that source alone:

```yaml
# arch_root/.mate.yaml
profile: arch
target_base: /
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
| `profile` | Only use this source under the named profile |
| `target_base` | Deploy this source relative to a different root |
| `targets` | Map a subdirectory to an absolute path |
| `ignore` | gitignore-style patterns, scoped to this source |
| `owner` | Default owner for all files in the source |
| `group` | Default group for all files |
| `perm` | Default mode for all files, octal string |
| `packages` | Packages this source needs |
| `generate` | Files created from inline content — see below |

`.mate.yaml` is itself rendered as a template, so `{{ .Vars.workspace }}` works
inside it.

### `targets`

Maps a subdirectory of the source onto an absolute path, which is how system files
are managed:

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
| `target` | Path to write, relative to the target base, or absolute |
| `mode` | File mode, octal string |
| `profile` | Only generate under this profile |
| `content` | The file content, rendered as a template |

Useful for short files that would otherwise need their own source file, and for
content assembled from variables or secrets.

## Environment variables

| Variable | Effect |
|----------|--------|
| `STATEMATE_DIR` | Overrides `source_dir` and the current directory when locating `mate.yaml` |
| `STATEMATE_PROFILE` | Selects a profile, below config but above detection |
| `XDG_CONFIG_HOME` | Where the local config lives. Default `~/.config` |
| `XDG_DATA_HOME` | Where `state.db` lives. Default `~/.local/share` |
| `XDG_STATE_HOME` | Where the secrets cache lives. Default `~/.local/state` |
| `VISUAL`, `EDITOR` | Editor for `mate edit`, if `editor:` is unset |

`STATEMATE_DIR` is handy for working against a second repository without touching
your config:

```bash
STATEMATE_DIR=~/other-dotfiles mate status
```

Scripts also receive [their own environment variables](scripts.md#environment).
