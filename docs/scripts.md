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
<order>-<name>.<ext>#<frequency>#<timing>[#template][#profile:<name>]
```

The order prefix is optional and controls execution order. A script with no
frequency attribute is **manual only** — it never runs during an apply.

```
01-setup.sh#once#before
02-secrets.sh#onchange#before#template
50-fonts.sh#weekly#after
99-cleanup.sh#always#after
bootstrap.sh                          ← manual only
migrate.sh#once#before#profile:arch
```

## Frequency

| Frequency | Runs |
|-----------|------|
| `once` | The first time only, ever |
| `onchange` | When the script's own source has pending changes |
| `always` | On every apply |
| `daily` | At most once in 24 hours |
| `weekly` | At most once in 7 days |
| `monthly` | At most once in 30 days |
| *(omitted)* | Never automatically — manual only |

Run history is kept in the state database, per machine.

### `onchange`

An `onchange` script runs when **its source** has pending changes — the files
`mate status` lists for the source the script lives in. A script in the repository
root `.matescripts/` has no owning source, so any pending change triggers it.

Editing the script itself does *not* trigger it. Use `mate scripts run <name>` to
run one on demand.

Pending changes are computed before any file is written, so an `onchange` script
sees the same set whether its timing is `before` or `after`.

## Timing

| Timing | When |
|--------|------|
| `before` | Before any file is applied (the default) |
| `after` | After all files are applied |

Configuration is reloaded after `before` scripts, so a script that generates a
var_file or fetches secrets has its output picked up by the templates rendered in
the same run.

## Descriptions

A script can describe itself with a comment in its first 10 lines:

```bash
#!/usr/bin/env bash
# Description: Restart keyd after its configuration changes
```

Matching is case-insensitive and tolerant of whitespace. The description appears in
`mate scripts list`, `mate status`, and the confirmation prompt.

## Confirmation

`mate apply` asks before running each script:

```
Run keyd-restart.sh (after, onchange)?
  Restart keyd after its configuration changes
[y]es / [n]o / [s]kip (mark as done) / [a]ll / [q]uit:
```

| Answer | Effect |
|--------|--------|
| `y` | Run it |
| `n` | Skip this time; offered again next apply |
| `s` | Mark as done without running, so it is not offered again |
| `a` | Run this and auto-confirm the rest |
| `q` | Abort the apply |

`[s]kip` is not offered for `always` scripts, whose runs are never recorded — there
is no meaningful "never ask again" for a script that opted into running every time.

Answering with EOF (no terminal input available) aborts rather than assuming
consent.

A script marked as done still appears in `mate scripts list` and can be run
manually.

For unattended runs:

| Flag | Effect |
|------|--------|
| `--force` | Auto-confirm every script |
| `--no-scripts` | Skip all scripts silently |

Without a terminal to prompt on, scripts are **skipped with a warning**. Automated
runs must pass `--force` or `--no-scripts` explicitly.

## Execution

Scripts run with the working directory set to the script's own directory. An
executable script is run directly; a non-executable one is run via the interpreter
named in its shebang, so forgetting `chmod +x` is not fatal.

A failing script aborts the apply.

### Environment

| Variable | Value |
|----------|-------|
| `STATEMATE_SCRIPT` | Absolute path of the script |
| `STATEMATE_SCRIPT_NAME` | Script name, without order prefix or attributes |
| `STATEMATE_SCRIPT_FREQUENCY` | `once`, `onchange`, `always`, `daily`, … |
| `STATEMATE_SCRIPT_TIMING` | `before` or `after` |
| `STATEMATE_SOURCE_DIR` | The owning source directory, if the script has one |

The parent environment is inherited as well.

### `#template`

A script marked `#template` is rendered before execution, giving it access to
[variables and secrets](templates.md):

```bash
#!/usr/bin/env bash
# Description: Write the API token
echo "{{ bitwarden "api" "field" "token" }}" > ~/.config/app/token
```

The rendered copy is what runs; the script in the repository is left alone.

## Commands

```bash
mate scripts list              # every script, its status, and description
mate scripts run <name>        # run one manually, ignoring frequency
mate apply --no-scripts        # apply files without any scripts
```

`mate scripts list` shows one row per script:

```
 ORDER  NAME             SOURCE  FREQUENCY  TIMING  STATUS   DESCRIPTION
     0  bootstrap.sh             manual     before           One-time machine bootstrap
     1  keyd-restart.sh  arch    onchange   after   pending  Restart keyd after its configuration changes
    99  reload-env.sh            always     after            Reload the shell environment
```

An empty `SOURCE` means the script lives in the repository-root `.matescripts/`.
The name carries `[profile]` and `[T]` markers for a profile-scoped or templated
script.

`STATUS` depends on frequency:

| Frequency | Status shown |
|-----------|--------------|
| `once` | `pending`, or `done (<timestamp>)` |
| `onchange` | `pending`, or `unchanged (<timestamp>)` |
| `always`, `daily`, `weekly`, `monthly`, manual | blank, or `ran <timestamp>` |
| any, profile not active | `n/a` |

Long descriptions are truncated with `…` to fit the terminal; when the output is
piped or redirected they are printed in full.

## Alternative: `.mate.yaml`

A source's [`.mate.yaml`](configuration.md#source-directory-config) can name
scripts explicitly instead of relying on the `.matescripts/` convention:

```yaml
scripts:
  before_apply: [bin/prepare.sh]
  after_apply: [bin/reload.sh]
```

Paths are relative to the source directory. Attributes in the filename still
apply.
