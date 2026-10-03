# Hooks

A hook runs commands or scripts after files matching a pattern are deployed or
removed — reloading systemd after a unit file changes, re-sourcing tmux after its
config changes.

```yaml
hooks:
  systemd-reload:
    match: ["*.service", "*.timer"]
    profile: arch
    description: Reload systemd after unit changes
    do:
      - run: sudo systemctl daemon-reload

  tmux:
    match: .config/tmux/**/*.conf
    do:
      - run: tmux source-file ~/.config/tmux/tmux.conf

  session-env:
    match: .config/environment.d/*.conf
    do:
      - run: dbus-update-activation-environment --systemd --all

  keyd:
    match: /etc/keyd/*.conf
    do:
      - run: sudo systemctl daemon-reload
      - script: keyd-restart.sh
      - run: notify-send "keyd reloaded ({{ len .Files }} files)"
```

| Field | Type | Description |
|-------|------|-------------|
| `match` | string or list | Target path patterns. Required |
| `do` | list | Steps run in order, each either `run:` or `script:`. At least one |
| `profile` | string | Only active under this profile, or one inheriting from it |
| `description` | string | Shown in the prompt, `mate hooks list`, and `mate status` |
| `enabled` | bool | Default `true`. `false` switches the hook off |

In TOML the same structure is `[hooks.keyd]` with `[[hooks.keyd.do]]` steps.

## Where hooks are declared

| File | Matches |
|------|---------|
| `mate.yaml` | Files from every source |
| Local config (`~/.config/statemate/mate.yaml`) | Files from every source |
| A source's `.mate.yaml` | Only that source's files, and only while the source is active |

Hooks from `mate.yaml` and the local config share one namespace. A local hook
with the same name **replaces** the repository one entirely; fields are not
merged. To switch a repository hook off on one machine:

```yaml
# ~/.config/statemate/mate.yaml
hooks:
  systemd-reload: { enabled: false }
```

A disabled hook needs neither `match` nor `do`.

Hooks in a source's `.mate.yaml` are named after the source, `arch/keyd`, so they
never collide with other hooks. The local config cannot override them.

## Patterns

Patterns match the **target** path — where the file is deployed — with the same
gitignore-style rules as [`ignore`](configuration.md#ignore):

| Pattern | Matches |
|---------|---------|
| `*.service` | Any file named `*.service`, at any depth, under any target root |
| `.config/tmux/**/*.conf` | Relative to `~` |
| `~/.config/environment.d/*.conf` | Relative to `~`, spelled out |
| `/etc/keyd/*.conf` | Absolute |

`**` matches any number of directories, and a pattern naming a directory matches
everything under it. Attributes such as `#template` are not part of the target
path, so they never need to appear in a pattern. `target_base` and `targets:`
mappings do not matter either: a unit deployed to `/etc/systemd/system` matches
`*.service` and `/etc/systemd/system/*.service` alike.

## When hooks run

A hook triggers when at least one matching file was:

| Event | Command |
|-------|---------|
| Written, new or with changed content | `mate apply` |
| Removed as an orphan | `mate clean` |
| Removed along with its source | `mate delete` (not with `--keep-target`) |
| Renamed | `mate rename` — the old path and the new path are both matched |

Only what actually reached the disk counts. These do not trigger hooks:

- Permission-only fixes
- `#import` files, which are copied back into the source rather than deployed
- Writes that failed or were skipped, such as a conflict answered with skip

Scoped runs (`mate apply <path>`, `mate apply --source <name>`) trigger hooks for
the files they wrote.

In `mate apply`, hooks run after files and packages and before `#after`
scripts:

```
#before scripts → files → packages → hooks → #after scripts
```

Each triggered hook runs **once**, however many of its files changed. Triggered
hooks run in name order, so a numeric prefix (`10-systemd`, `20-keyd`) forces an
order. Steps run in the order listed.

## Steps

### `run:`

A command run with `sh -c`. It is rendered as a [template](templates.md) first,
with `.Files` set to the triggering target paths, sorted.

Commands run in the directory of the config file that declared the hook: the
repository root for `mate.yaml`, the source directory for `.mate.yaml`, and
`~/.config/statemate` for the local config.

mate does not elevate hook commands; write `sudo` yourself. After `mate apply`
wrote system files, the sudo session it opened usually means no second password
prompt.

### `script:`

A script by the name `mate scripts list` shows in its NAME column, such as
`keyd-restart.sh`. The name is resolved in this order:

1. For a hook in a source's `.mate.yaml`, a script in that source.
2. A script in the repository-root `.matescripts/`.
3. A script in any other source. If more than one matches, it is a config error.

The script runs as it would with `mate scripts run`: its attributes, environment,
and working directory are unchanged, and the run is recorded. If the script has
a `#profile:` that is not active, the step is skipped with a warning.

A script a hook ran is not run again as an `#after` script in the same apply.

### Environment

Every step receives, on top of the usual environment:

| Variable | Value |
|----------|-------|
| `STATEMATE_HOOK_NAME` | The hook name, `arch/keyd` for a source hook |
| `STATEMATE_HOOK_FILES` | The triggering target paths, newline-separated, sorted |
| `STATEMATE_SOURCE_DIR` | The declaring source, for a hook in a source's `.mate.yaml` |

`STATEMATE_HOOK_FILES` is empty when the hook is run with `mate hooks run`.

## Confirmation

Hooks are confirmed like [scripts](scripts.md#confirmation), once per hook:

```
Run hook keyd (2 files)?
  Reload keyd after config changes
  - sudo systemctl daemon-reload
  - script keyd-restart.sh
  - notify-send "keyd reloaded (2 files)"
[y]es / [n]o / [a]ll / [q]uit:
```

- `[a]ll` confirms the remaining hooks and the `#after` scripts that follow.
- `[q]uit` stops the remaining hooks and `#after` scripts. The files are already
  written.
- `--force` confirms without asking. `--no-scripts` skips hooks too.
- Without a terminal, hooks are skipped with a warning.

Declining is not remembered. Once the files are written they are unchanged, so
the hook will not trigger on the next apply; run it with `mate hooks run <name>`.
`mate delete` and `mate clean` confirm hooks unless `--force` is given, and
`mate rename` always asks.

## Failure

A failing step stops its hook; the remaining steps are skipped. Other hooks and
the `#after` scripts still run, and the command exits non-zero. A failed hook is
not retried on the next apply.

## Checking

A hook is validated when it is loaded, so `mate apply` refuses to start and
`mate check` fails when:

- `match` is missing or a pattern does not parse
- `do` is empty, or a step has both or neither of `run` and `script`
- a `script:` step matches no script, or more than one
- a `run:` template does not parse

`mate doctor` warns about an active hook whose patterns match no managed file,
which usually means a typo.

## Commands

| Command | Purpose |
|---------|---------|
| `mate hooks list` | Every hook with its scope, patterns, profile, and status |
| `mate hooks run <name>` | Run a hook now, without a prompt |
| `mate apply --dry-run` | Lists the hooks that would run; `-V` adds the files and steps |
| `mate status` | Lists the hooks that pending changes would trigger |

In `mate hooks list`, SCOPE is `repo`, `local`, or the source name. STATUS is
`disabled` for a hook with `enabled: false` and `n/a` when its profile is not
active. `mate hooks run` refuses a disabled hook or one whose profile is not
active.

## Hooks or `#onchange` scripts

An [`onchange`](scripts.md#onchange) script runs when anything in its source
changes and needs a file of its own. Use a hook when the follow-up depends on
*which* files changed, or when it is a single command. Use an `onchange` script
for longer logic tied to a whole source, or call that script from a hook with
`script:`.
