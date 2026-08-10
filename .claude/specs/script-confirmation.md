# Spec: script confirmation prompts and descriptions

## Problem

Scripts run silently during `mate apply`. They execute arbitrary code, but there
is no per-script visibility or opportunity to decline. There is also no way for a
script to describe what it does — `mate scripts list` shows only name, frequency,
timing, source, and profile, so a script's purpose has to be inferred from its
filename.

## Part 1: Confirmation prompts

### Scope

Every **automatic** script (frequency != `manual`) that is due to run prompts for
confirmation during `mate apply`. This includes `always` scripts, which will
therefore prompt on every apply.

Manual scripts are unaffected — they only run via `mate scripts run`, which is
already an explicit action.

### Prompt options

```
Run 01-setup.sh? (before, once)
  Bootstrap the development environment
[y]es / [n]o / [a]ll / [q]uit:
```

| Key | Behavior |
| --- | --- |
| `y` | Run this script |
| `n` | Skip this script (not recorded — will prompt again next apply) |
| `a` | Run this and auto-confirm all remaining scripts this apply |
| `q` | Abort the apply |

`q` aborts the whole apply. Because `before` scripts may be prerequisites for the
file changes that follow, continuing after an explicit quit would be wrong.

### Declining is temporary

Answering `n` must **not** record a run in the state DB. A `once` script that is
skipped by accident has to be offered again on the next apply — permanently
dismissing it would silently drop work with no way to recover it.

### Flags

| Flag | Behavior |
| --- | --- |
| `--force` | Auto-confirm all scripts (consistent with its existing role of suppressing conflict prompts) |
| `--no-scripts` | Skip all scripts silently — nothing runs, nothing is recorded |
| `--dry-run` | No prompting; lists what would run, including descriptions |

`--no-scripts` exists for automation that should never run scripts. It skips
silently (no per-script listing).

### Non-interactive behavior

When stdin is not a TTY and neither `--force` nor `--no-scripts` is given:

- Do **not** run any scripts.
- Print a loud warning naming the scripts that were skipped, and stating that
  `--force` runs them or `--no-scripts` silences the warning.

**This is a behavior change.** Existing cron/CI jobs running `mate apply` will
stop running scripts until they add `--force` (to run) or `--no-scripts` (to
skip). This is the accepted tradeoff: never execute unreviewed code unattended.

## Part 2: Script descriptions

### Syntax

A script declares a description with a comment line in the **first 10 lines**:

```bash
#!/usr/bin/env bash
# Description: Bootstrap the development environment
```

Matching is **case-insensitive with flexible whitespace** — all of these work:

- `# Description: text`
- `#Description:text`
- `# description:   text`

The scan is bounded to the first 10 lines so listing scripts never requires
reading whole files. Shebang lines are skipped naturally (they don't match).

If no description line is found, the description is empty and displays are
unchanged from today.

### Read from the raw file

Descriptions are read from the file **on disk without template rendering**, even
for `#template` scripts. Rendering would require a template context in
`scripts list` and `status` (where one may not exist) and could fail mid-listing.
A description is metadata about the script, not part of its rendered output.

### Where descriptions appear

1. **The confirmation prompt** — indented under the script name (the main point).
2. **`mate scripts list`** — so you can see what each script does at a glance.
3. **`mate status`** — in the existing `Pending scripts:` section.

## Implementation notes

- `Script` gains a `Description string` field, populated during discovery
  (`internal/scripts/discover.go`) alongside the existing `ContentHash`.
- Description parsing belongs next to `readShebang` in `internal/scripts/`, which
  already does bounded line-wise reads of script files.
- `Executor` needs the prompt state (`confirmAll` after `a`, plus `force` /
  `noScripts` / TTY detection). Prompting lives in the executor rather than the
  CLI so both `before` and `after` script batches share it.
- Reuse the existing prompt style from `promptMissingPackages` /
  `promptConflict` for consistency.

## Out of scope

- Prompting for manual scripts run via `mate scripts run`.
- A `[d]iff`/`[s]how` prompt option to print script contents.
- Per-script opt-in/opt-out attributes for prompting.
- Templated (variable-interpolated) descriptions.
- Adding a script counter to `mate status --short`.

## Acceptance criteria

1. `mate apply` prompts before each due automatic script, showing frequency,
   timing, and description when present.
2. `y` runs, `n` skips without recording, `a` auto-confirms the rest, `q` aborts.
3. A script declined with `n` prompts again on the next `mate apply`.
4. `--force` runs all scripts with no prompting.
5. `--no-scripts` skips all scripts silently and records nothing.
6. `--dry-run` lists scripts with descriptions and does not prompt.
7. With no TTY and no relevant flag, scripts are skipped and a warning names them
   and the two flags.
8. `# Description:` is picked up case-insensitively with flexible spacing, only
   within the first 10 lines, from the raw file.
9. Descriptions appear in the prompt, `mate scripts list`, and `mate status`.
10. Scripts with no description line behave exactly as before.
11. `make test` and `make lint` pass.
