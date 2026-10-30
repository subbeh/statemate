# Scripts

Scripts run around an apply — reloading a daemon after its config changes,
bootstrapping a new machine, rebuilding a font cache.

Place them in a `.matescripts/` directory, either at the repository root or inside
a source:

```
~/dotfiles/
  .matescripts/
    99-reload-env.sh#onchange#after      ← any source changing triggers this
  arch/
    .matescripts/
      01-keyd-restart.sh#onchange#after  ← only when arch/ changes
```

## Naming

```
<order>-<name>.<ext>#<frequency>#<timing>[#tmpl][#noconfirm][#profile:<name>]
```

The order prefix is optional and controls execution order. Scripts are sorted by
it across all sources together, then by name; a script without a prefix counts as
`0`. A script with no frequency attribute is **manual only**: it never runs
during an apply.

Every file in a `.matescripts/` directory is treated as a script, including a
README, so keep documentation elsewhere.

```
01-setup.sh#once#before
02-secrets.sh#onchange#before#tmpl
50-fonts.sh#weekly#after#noconfirm
99-cleanup.sh#always#after
bootstrap.sh                          ← manual only
migrate.sh#once#before#profile:arch
```

## Frequency

| Frequency | Runs |
|-----------|------|
| `once` | The first time only, on this machine |
| `onchange` | When the script's own source has pending changes, or what its [`# Onchange:`](#choosing-the-triggers) comment names changed, or its last run failed |
| `always` | On every apply |
| `daily` | At most once in 24 hours |
| `weekly` | At most once in 7 days |
| `monthly` | At most once in 30 days |
| *(omitted)* | Never automatically — manual only |

Run history is kept in the state database, per machine, keyed by the script's
full path *including its attributes*. Renaming or moving a script, or changing
one of its attributes (`#before` to `#after`, say), makes it a new script that
runs again.

### `onchange`

An `onchange` script runs when **its source** has pending changes: the files
`mate status` lists for the source the script lives in, including pending
imports and conflicts. A script in the repository root `.matescripts/` has no
owning source, so any pending change triggers it.

Editing the script itself does *not* trigger it, unless its `# Onchange:`
comment says so. Use `mate scripts run <name>` to run one on demand.

Pending changes are computed before any file is written, so an `onchange` script
sees the same set whether its timing is `before` or `after`.

A run that fails keeps the script due on later applies, even though the changes
that triggered it have since been written, until it succeeds or you answer `s` to
mark it done.

#### Choosing the triggers

A comment in the script's first 10 lines narrows what triggers it, matched like
the [description](#descriptions):

```bash
#!/usr/bin/env bash
# Description: Restart keyd after its configuration changes
# Onchange: self /etc/keyd/*.conf
```

The comment lists triggers separated by whitespace, and any one of them makes
the script due:

| Trigger | Due when |
|---------|----------|
| `self` | The script differs from what it was at its last recorded run, or it has never run. Answering `s` at the prompt records it as run |
| A pattern | A pending change in the script's source has a target path the pattern matches. A repository-root script matches changes in every source |

Patterns follow the [hook pattern rules](hooks.md#patterns): they match where a
file is deployed, so `*.conf`, `.config/keyd/*.conf` and `/etc/keyd/*.conf` all
work. A pattern cannot contain whitespace.

With the comment, only the listed triggers count: `# Onchange: self` runs the
script when you edit it, not when anything else in the source changes. A failed
run is still retried. Like an attribute, the comment is checked when scripts are
discovered: `mate apply` refuses to start when it is empty, has a pattern that
does not parse, or is on a script that is not `#onchange`. For a `#tmpl` script,
the comment and `self` both use the script as it is in the repository, not the
rendered copy.

For a follow-up that depends on *which* files changed, or that is a single
command, use a [hook](hooks.md).

## Timing

| Timing | When |
|--------|------|
| `before` | Before any file is applied (the default) |
| `after` | After all files are applied |

Configuration is reloaded after `before` scripts, so a script that generates a
var_file has its output picked up by the files deployed in the same run.

Missing secrets are fetched *before* the `before` scripts run, so a script cannot
prepare something the fetch depends on.

## Descriptions

A script can describe itself with a comment in its first 10 lines:

```bash
#!/usr/bin/env bash
# Description: Restart keyd after its configuration changes
```

Matching is case-insensitive and tolerant of whitespace. The description appears in
`mate scripts list`, `mate status`, and the confirmation prompt.

## Tags

A `# Tags:` comment, read the same way, gives a script one or more tags,
separated by whitespace:

```bash
#!/usr/bin/env bash
# Description: Upgrade Homebrew packages
# Tags: update brew
```

`mate scripts run --tags update` then runs every script tagged `update` in one
go, and `mate scripts list --tags update` lists them. Both take several tags,
comma-separated or with the flag repeated, and match scripts with any of them:
`--tags update,brew`. `mate scripts list` shows the tags in a `TAGS` column once
any script has them.

## Confirmation

`mate apply` asks before running each script:

```
Run keyd-restart.sh (after, onchange)?
  Restart keyd after its configuration changes
[Y]es / [n]o / [s]kip (mark as done) / [a]ll / [q]uit:
```

| Answer | Effect |
|--------|--------|
| `y` or Enter | Run it |
| `n` | Skip this time; offered again next apply |
| `s` | Mark as done without running, so it is not offered again |
| `a` | Run this and auto-confirm the rest |
| `q` | Abort the apply |

Each answer is a single keypress — there is no need to press Enter. Enter on its
own answers yes, and Ctrl-C aborts like `q`.

`[s]kip` is not offered for `always` scripts, whose runs are never recorded — there
is no meaningful "never ask again" for a script that opted into running every time.

Answering with EOF (no terminal input available) aborts rather than assuming
consent.

A script marked `#noconfirm` runs without asking, even when there is no
terminal. Use it for scripts that are safe to run unattended, such as rebuilding
a font cache. `--no-scripts` still skips it.

A script marked as done still appears in `mate scripts list` and can be run
manually.

For unattended runs:

| Flag | Effect |
|------|--------|
| `--force` | Auto-confirm every script, hook, and package install, and overwrite conflicts |
| `--no-scripts` | Skip all scripts silently |
| `--noconfirm` | Apply without listing the file changes and asking first (implied by `--force`) |

Without a terminal to prompt on, scripts are **skipped with a warning**. Automated
runs must pass `--force` or `--no-scripts` explicitly.

## Execution

Scripts run with the working directory set to the script's own directory, the
`.matescripts/` directory itself, not the source. An executable script is run
directly. A non-executable one is run through the interpreter named in its
shebang, or `sh` if it has none, so forgetting `chmod +x` is not fatal. That
fallback takes the shebang's program name only: write `#!/usr/bin/env bash` and
use `set -e` in the script, not `#!/bin/bash -e`.

A failing script aborts the apply.

### Environment

| Variable | Value |
|----------|-------|
| `STATEMATE_SCRIPT` | Absolute path of the script |
| `STATEMATE_SCRIPT_NAME` | Script name, without order prefix or attributes, such as `keyd-restart.sh` |
| `STATEMATE_SCRIPT_FREQUENCY` | `once`, `onchange`, `always`, `daily`, … |
| `STATEMATE_SCRIPT_TIMING` | `before` or `after` |
| `STATEMATE_SOURCE_DIR` | The owning source directory, if the script has one |

The parent environment is inherited as well.

### `#tmpl`

A script marked `#tmpl` is rendered before execution, giving it access to
[variables and secrets](templates.md):

```bash
#!/usr/bin/env bash
# Description: Write the API token
echo "{{ bitwarden "api" "field" "token" }}" > ~/.config/app/token
```

The rendered copy is what runs; the script in the repository is left alone. It
is written to a temporary executable file and run directly, so a `#tmpl`
script **must** start with a shebang.

## Commands

```bash
mate scripts list              # every script, its status, and description
mate scripts run <name>        # run one manually, ignoring frequency
mate scripts run <name> --dry-run
mate scripts list --tags <tag> # only the scripts with the tag
mate scripts run --tags <tag>  # run every script with the tag
mate apply --no-scripts        # apply files without any scripts
```

`mate scripts run` takes the name shown in `mate scripts list` or a path. It does
not prompt, ignores the script's `#profile:`, and records the run, so running a
`#once` script by hand marks it done.

`mate scripts run --tags <tag>` runs every script with that [tag](#tags), or
with any of several:
`before` scripts, then `after` scripts, each in order, whatever their
frequency. Unlike a run by name, it skips scripts whose `#profile:` does not
match the active profile. It does not prompt, records each run, and stops at the
first script that fails.

Scoped applies run fewer scripts: `mate apply <path>` runs none, and
`mate apply --source <name>` runs only that source's scripts, not those in the
repository-root `.matescripts/`.

`mate scripts list` shows one row per script:

```
 ORDER  NAME             SOURCE  FREQUENCY  TIMING  STATUS   DESCRIPTION
     0  bootstrap.sh             manual     before           One-time machine bootstrap
     1  keyd-restart.sh  arch    onchange   after   pending  Restart keyd after its configuration changes
    99  reload-env.sh            always     after            Reload the shell environment
```

An empty `SOURCE` means the script lives in the repository-root `.matescripts/`.
A profile-scoped script's name is followed by its profile, such as
`migrate.sh [arch]`, a templated one by `[T]`, and a `#noconfirm` one by
`[noconfirm]`.

`STATUS` depends on frequency:

| Frequency | Status shown |
|-----------|--------------|
| `once` | `pending`, or `done (<timestamp>)` |
| `onchange` | `pending`, or `unchanged`, with a timestamp once it has run |
| `daily`, `weekly`, `monthly` | blank, or `ran <timestamp>` |
| `always`, manual | always blank, since their runs are not recorded |
| any, profile not active | `n/a` |

Long descriptions are truncated with `…` to fit the terminal; when the output is
piped or redirected they are printed in full.
