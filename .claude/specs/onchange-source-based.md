# Spec: `#onchange` triggers on source changes, not script changes

## Problem

`#onchange` currently means "run when **the script file** changes". `ShouldRun`
compares the script's own content hash against the last recorded run:

```go
case FreqOnchange:
    hasRunWithHash, err := db.HasScriptRunWithHash(script.Path, script.ContentHash)
    ...
    if hasRunWithHash { return false, "unchanged", nil }
```

That is almost never what the script author wants. A real example from the
user's repo:

```
arch/.matescripts/00-env_reload.sh#onchange#after
```

This exists to reload the environment **when the arch source's files change**.
Under the current behaviour it only reruns when you edit `00-env_reload.sh`
itself — which essentially never happens — so the script effectively never
fires. The feature is inverted from its intent.

## Decisions

### `#onchange` means: the script's source has pending changes

An `#onchange` script runs when **its own source** has pending changes, i.e.
files that `mate status` would list for that source.

- Scripts live in a per-source `.matescripts/` directory (`arch/.matescripts/…`),
  so a script has a natural owning source: `Script.SourceDir`.
- A script in the **repo-root** `.matescripts/` has no owning source
  (`SourceDir == ""`). Those are repo-wide: they run when **any** source has
  pending changes.

### The script's own content is NOT a trigger

Editing an `#onchange` script does not cause it to run. There is one rule:
source files changed. To test a script while iterating on it, either touch a
source file or run it directly with `mate scripts run <name>`.

This is a **behaviour change** — today editing the script reruns it.

### Frequency and timing are orthogonal

`#onchange` is the *frequency* (whether to run); `#before`/`#after` is the
*timing* (when during apply). A script has both, e.g. `#onchange#after`. The
pending-change set decides *whether* an `#onchange` script runs; its timing
attribute independently decides *when*.

### Which statuses count as "changed"

Anything `mate status` lists: `new`, `modified`, and `conflict`.

Accepted imprecision: if a change is pending but you decline it at the conflict
prompt, the script still runs. Basing the decision purely on pending changes
keeps one simple rule, and lifecycle scripts should be idempotent regardless.

### No new state, decided per-run

Nothing extra is recorded. Each apply asks "does this source have pending
changes *right now?*". After a successful apply there are none, so the script
naturally will not rerun until something changes again.

`RecordScriptRun` is still called for `#onchange` so that:

- `mate scripts list` can show a last-run timestamp, and
- the `[s]kip` (mark as done) prompt action keeps working.

The record simply no longer decides whether the script runs.
`HasScriptRunWithHash` is no longer consulted for `FreqOnchange`.

### Plumbing: callers pass the changed-source set in

`ShouldRun(script, db)` has no notion of pending changes. Rather than have it
rescan the tree per script, callers pass in the set of source names with pending
changes. They already compute the change set once.

Signature becomes roughly:

```go
ShouldRun(script *Script, db *state.DB, changed ChangedSources) (bool, string, error)
```

where `ChangedSources` is a set of source directory names, plus a flag for
"any source changed" (for root scripts). A nil/empty set means "no changes",
so an `#onchange` script does not run.

Call sites:

| Caller | Passes |
| --- | --- |
| `apply` | change set from the `ComputeChanges` it already runs |
| `status` | change set from the `ComputeChanges` it already runs |
| `scripts list` | computes the change set too, so its status column agrees |

`mate scripts list` gaining a tree scan makes it slower. That is accepted for
consistency — status and apply must not disagree with what `scripts list`
reports. (Note: `mate status`/`apply` performance is already tracked separately
in TODO.md; this adds the same cost to `scripts list`.)

### `mate status` shows it automatically

Because `status` shares `ShouldRun`, an `#onchange` script appears under
`Pending scripts:` exactly when its source has pending changes — which is
exactly when `apply` will offer it. Consistency falls out of the shared path
rather than being maintained separately.

## Out of scope

- Explicit per-script watch patterns (e.g. `#onchange:.config/foo/**`).
- Triggering on which files were *actually written* during apply (as opposed to
  pending) — that would make `#before` and `#after` behave differently.
- Changing any other frequency's semantics.
- Optimising the added scan in `scripts list`.

## Acceptance criteria

1. An `#onchange` script in `<src>/.matescripts/` runs when `<src>` has pending
   changes, and does not run when it has none.
2. A change in a *different* source does not trigger it.
3. A script in the repo-root `.matescripts/` runs when any source has pending
   changes.
4. Editing an `#onchange` script does **not** cause it to run.
5. `mate scripts run <name>` still runs it on demand.
6. After a successful apply with no further edits, the script does not rerun.
7. `mate status` lists the script under `Pending scripts:` exactly when its
   source has pending changes.
8. `mate scripts list` shows a last-run timestamp after it runs.
9. `[s]kip` (mark as done) still works for `#onchange` scripts.
10. Other frequencies (`once`, `always`, `daily`, `weekly`, `monthly`,
    `manual`) are unaffected.
11. `make test` and `make lint` pass.
