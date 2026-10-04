# Templates

A file marked [`#template`](attributes.md#template) (or `#tmpl`) is rendered with
Go's [`text/template`](https://pkg.go.dev/text/template) before being deployed.

```
gitconfig#template
```

```gotemplate
[user]
    email = {{ .Vars.email }}
    name = {{ .Vars.name }}

{{ if eq .OS "darwin" }}
[credential]
    helper = osxkeychain
{{ end }}
```

Preview the result without applying:

```bash
mate eval git/gitconfig#template
mate eval --profile work git/gitconfig#template
```

Templates are also rendered in:

- a source's [`.mate.yaml`](configuration.md#source-directory-config), including
  its `generate` content
- scripts marked [`#template`](scripts.md#template)
- hook [`run:`](hooks.md#run) commands, which also get `.Files`

The repository's own `mate.yaml`, var files, and include files are **not**
rendered.

A `.mate.yaml` is rendered as one piece of text *before* it is parsed as YAML,
so the rendered result must still be valid YAML. That matters for multi-line
values inside a block scalar, which need [`indent`](#statemates-own).

## Variables

| Variable | Value |
|----------|-------|
| `.Profile` | Active profile name, empty if none |
| `.Hostname` | Machine hostname |
| `.OS` | `linux`, `darwin`, … (Go's `GOOS`) |
| `.Arch` | `amd64`, `arm64`, … (Go's `GOARCH`) |
| `.HomeDir` | Current user's home directory |
| `.Username` | `$USER`, falling back to `$USERNAME` |
| `.SourceDir` | Absolute path of the directory containing `mate.yaml` |
| `.Vars` | Map of your own variables |
| `.Env` | Map of environment variables |
| `.Files` | In a hook's `run:` only: the target paths that triggered it |

`.Vars` is assembled from, in order of increasing precedence:

1. [`variables`](configuration.md#keys), and variables from
   [`include`](configuration.md#include) files
2. [`var_files`](configuration.md#var_files), in order
3. [`variable_commands`](configuration.md#variable_commands)
4. profile `variables`, parent first, so the active profile wins

Each layer replaces top-level keys rather than merging into them: a profile that
sets `color:` replaces the whole `color` map, not just the keys it names.

```gotemplate
{{ .Vars.email }}          # a variable
{{ .Env.HOME }}            # an environment variable
{{ .Vars.nested.key }}     # nested YAML works as expected
```

### Missing values

A reference to a variable that does not exist renders as the text `<no value>`,
and is **not** an error:

```gotemplate
email = {{ .Vars.emial }}  →  email = <no value>
```

`.Env.NAME` for an unset variable renders as `<no value>` too, while
`{{ env "NAME" }}` returns an empty string. Wrap anything that must be present
in `required`, so a typo fails loudly instead of deploying a broken file:

```gotemplate
email = {{ required .Vars.email }}
```

## Functions

### Sprig

The [sprig](https://masterminds.github.io/sprig/) library is available, giving
around 200 functions for strings, lists, dictionaries, math, dates, paths, and
encoding:

```gotemplate
HostName {{ (splitList "-" .Vars.user) | first }}.example.com
{{ .Vars.name | upper | trimSuffix "X" }}
{{ join ", " .Vars.list }}
{{ ternary "yes" "no" .Vars.enabled }}
{{ regexReplaceAll "\\s+" .Vars.text " " }}
```

See sprig's documentation for the full list.

### Statemate's own

These take precedence over sprig where a name collides.

| Function | Description |
|----------|-------------|
| `bitwarden <item> <type> <field>` | Fetch a secret from the cache — see [Secrets](secrets.md) |
| `bitwardenAttachment <item> <filename>` | Shorthand for an attachment secret |
| `env <name>` | Environment variable, read from the rendering context |
| `var <name>` | Variable by name, for keys that are not valid identifiers |
| `cmd <command>` | Run a shell command and return its trimmed output |
| `required <value>` | Fail rendering if the value is missing or empty |
| `default <fallback> <value>` | Use the fallback when the value is `nil` or `""` |
| `base64Decode <string>` | Decode a base64 string |
| `indent <n> <string>` | Indent each non-empty line by `n` spaces |

```gotemplate
{{ required .Vars.api_key }}
{{ default "vim" .Vars.editor }}
{{ cmd "hostname -s" }}
{{ var "some-key-with-dashes" }}
```

`cmd` returns an empty string if the command fails, so use `required` around it
when the value matters.

`indent` is written for embedding a block inside structured output:

```gotemplate
sshKey: |
{{ indent 2 (bitwarden "work-key" "ssh" "private") }}
```

### Where statemate shadows sprig

Three names exist in both, and statemate's win:

| Function | Difference |
|----------|------------|
| `env` | Reads `.Env`, a snapshot of the environment taken when `mate` starts; returns `""` if unset |
| `default` | Substitutes only for `nil` and `""`; sprig's also replaces `0` and `false` |
| `indent` | Leaves blank lines unpadded, where sprig's pads every line |

`default` takes the fallback first, `default <fallback> <value>`, the same order
as sprig's, so `{{ .Vars.x | default "y" }}` works in pipelines. Unlike sprig's it
takes exactly two arguments.

Sprig's `nindent` is not shadowed, so unlike `indent` it pads blank lines too.

Sprig has no `required` function, so statemate's is the only one.

## Change detection

Because a rendered file differs from its source, statemate compares the **rendered
output** against the deployed target. A change to a variable — in `mate.yaml`, a
var_file, or a profile — is therefore detected as a pending change even though the
template file itself is untouched.

## Debugging

`mate eval <file>` renders to stdout, which is the fastest way to find a mistake.
It renders any file you give it, whether or not it is marked `#template`. A
template that fails to parse reports the line:

```
Error: rendering template: template: :4: function "splitLst" not defined
```

Encrypted templates are decrypted first, so `mate eval` needs your age identity.

`mate status` does not stop on a template that fails to render: it reports the
file as modified. `mate diff` and `mate apply` stop with the error. When a file
is unexpectedly listed as `~`, `mate eval` it.
