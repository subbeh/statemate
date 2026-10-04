# Different Machines

One repository usually has to serve several machines: a work laptop and a
personal one, macOS and Linux, a desktop and a server. statemate has three tools
for this, from coarse to fine:

| Tool | Varies | Example |
|------|--------|---------|
| Profile `sources` | Which tools are installed at all | `hyprland` only on the Arch machine |
| [`#profile:`](attributes.md#profilename) | Which version of a file is deployed | a work and a personal `settings.json` |
| [Templates](templates.md) | Lines within one file | the git email, a macOS-only block |

Use the coarsest one that does the job. A whole source per platform is easier to
reason about than a template full of `if` blocks.

## Define the profiles

A realistic set: one profile per OS, and one per machine that extends its OS.

```yaml
# mate.yaml
sources: [core, git, nvim, tmux, zsh]      # every machine

profiles:
  linux:
    detection:
      os: linux

  macos:
    detection:
      os: darwin
    sources: [aerospace]
    variables:
      configHome: "~/Library/Application Support"

  personal:
    extends: linux
    detection:
      command: "test -f /etc/arch-release"
    sources: [arch, hyprland]
    variables:
      email: "me@example.com"

  work:
    extends: macos
    detection:
      user: jdoe
    sources: [work]
    variables:
      email: "jdoe@company.com"
```

On the work laptop this resolves to profile `work` and deploys `core`, `git`,
`nvim`, `tmux`, `zsh`, `aerospace`, and `work`.

### How detection picks one

Exactly one profile is active. Profiles are tried **deepest in the `extends`
chain first**, then alphabetically, and the first whose `detection` matches wins.
So `personal` and `work` are tested before `linux` and `macos`, which act as
fallbacks for a machine that matches neither.

Check the result on each machine:

```bash
mate profile
```

To skip detection on one machine, put `profile: work` in that machine's
[local config](configuration.md#local-config), or set `STATEMATE_PROFILE`.

### What a child profile inherits

`extends` brings in the parent's `sources`, `variables`, and `packages`, with the
child's variables winning where both define one. `detection` is not inherited,
since each profile needs its own way to recognise its machine.

## Deploy per-profile files

When two machines need genuinely different versions of a file, keep both and tag
each with the profile it belongs to:

```
claude/
  .claude/settings.json#profile:personal
  .claude/settings.json#profile:work
```

Only one variant is active under any profile, so they do not conflict.
Inheritance is respected: a `#profile:linux` file deploys under `personal`
too.

The attribute also works on a directory, where it covers everything inside:

```
tailscale/
  etc#profile:linux/
    systemd/system/tailscaled.service.d/override.conf
```

For per-OS variants of files that are otherwise named alike, a common pattern is
a suffix in the name plus the attribute:

```
zsh/.config/zsh/11-aliases-linux.zsh#profile:linux
zsh/.config/zsh/11-aliases-macos.zsh#profile:macos
```

> **Make sure a profile always matches.** When *no* profile is active,
> `#profile:` filtering is switched off: every variant is deployed, and variants
> of the same target conflict. A profile per OS, as above, guarantees that one
> always matches. Run `mate profile` on a new machine before the first apply.

## Vary lines with templates

For files that are mostly the same, a [template](templates.md) is less to
maintain than two copies:

```gotemplate
# git/.config/git/config#template
[user]
    email = {{ .Vars.email }}

{{ if eq .OS "darwin" -}}
[credential]
    helper = osxkeychain
{{ end -}}
```

Branch on the profile when the difference is about the machine rather than the
OS:

```gotemplate
{{ if eq .Profile "work" }}...{{ end }}
```

Note that `.Profile` is only the active profile's own name. To match a profile
*and its descendants*, put the value in a variable on the parent profile and test
the variable instead.

## Deploy to different paths per machine

Some applications keep their configuration in different places per OS: `~/.config`
on Linux, `~/Library/Application Support` on macOS. A source's `.mate.yaml` is a
template too, so its [`targets`](configuration.md#targets) can come from a
variable:

```yaml
# mate.yaml
variables:
  configHome: "~/.config"
profiles:
  macos:
    variables:
      configHome: "~/Library/Application Support"
```

```yaml
# bitwarden/.mate.yaml
targets:
  config: "{{ .Vars.configHome }}"
```

```
bitwarden/
  config/Bitwarden CLI/data.json   →  ~/.config/Bitwarden CLI/data.json            (linux)
                                   →  ~/Library/Application Support/Bitwarden CLI/data.json  (macos)
```

## Preview another machine

Every read-only command accepts `--profile`, so you can check what another machine
would get without being on it:

```bash
mate profile -p work                  # which sources it resolves to
mate status -p work                   # what would change there
mate eval -p work git/.config/git/config#template
```

The results reflect *this* machine's files and state, so `status` is only a rough
guide. `eval` is exact.
