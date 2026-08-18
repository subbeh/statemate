# `#import` attribute

## Problem

Some managed files are owned by the application, not by the user. `~/.claude/settings.json`
is the motivating case: Claude Code rewrites it as you change settings, so the
target drifts from the source constantly.

Today statemate treats a changed target as a conflict. Every `mate apply` stops and
asks:

```
Conflict: /Users/x/.claude/settings.json
  Target has been modified since last apply
  [o]verwrite, [i]mport, [s]kip, [d]iff, [a]bort:
```

The answer is always `[i]mport`. Worse, the file is `#encrypted`, so the drift is
invisible in `mate status` output — you cannot tell whether the app changed
something meaningful without decrypting and diffing by hand.

## Solution

A new filename attribute, `#import`, marking a file whose **target is normally
authoritative**. Statemate imports the target back into the source instead of
prompting.

```
claude/.claude/settings.json#profile:work#encrypted#import
```

It composes with existing attributes. `#encrypted` still applies: the imported
content is encrypted before being written to the source.

## Behavior

Four cases, decided by comparing the source hash and target hash against what the
state DB recorded at the last apply:

| Source | Target | Status | Action |
|---|---|---|---|
| unchanged | unchanged | `StatusUnchanged` | nothing |
| changed | unchanged | `StatusModified` | deploy source → target (normal) |
| unchanged | changed | **`StatusImport`** | import target → source, no prompt |
| changed | changed | `StatusConflict` | prompt (existing behavior) |

Two further cases are unchanged from today:

- **Target missing** (fresh machine) → `StatusNew`, deploy source → target. This
  is what makes `#import` usable for bootstrapping: you get your settings file,
  and from then on the app's edits flow back.
- **Not yet tracked** (no DB row) → existing logic. With no recorded applied hash
  there is no way to tell which side changed, so a differing target stays a
  conflict.

Divergence on both sides falls back to the prompt deliberately. Silently letting
the target win would discard an intentional source edit — the one case where the
user's own change is at stake.

## Display

`mate status` marks an import with `<`, pointing back toward the source:

```
  TARGET                              SOURCE
~ ~/.gitconfig                        git
< ~/.claude/settings.json             claude
```

`mate status --short` gets a `<N` counter: `~1 <1`.

`mate apply` reports the direction:

```
← ~/.claude/settings.json (imported)
```

`mate apply --dry-run` reports `would import ~/.claude/settings.json → source`.

`mate diff` shows the diff in the import direction for these files — target as the
new version — so the diff reads the way the change will actually be applied.

## Implementation

- `internal/source/attrs.go` — add `Import bool`, parse the bare `import` token.
  Not recursive; `#import` on a directory is not meaningful and is ignored.
- `internal/target/apply.go` — add `StatusImport` to the `ChangeStatus` enum and
  its `String()`. In `Apply`, handle `StatusImport` by calling the existing
  `importFile`, which already encrypts when `Attrs.Encrypted` is set.
- `internal/target/diff.go` — in `computeChange`, at the point where source is
  unchanged but the target hash differs from the applied hash (currently an
  unconditional `StatusConflict`), return `StatusImport` when `entry.Attrs.Import`.
  The both-changed branch is left alone.
- `internal/cli/status.go` — `<` marker and `<N` short counter.
- `internal/cli/apply.go` / `printChange` — import direction in output.

`importFile` already exists and is already used by the conflict prompt's `[i]mport`
answer, so the write path is proven; this only changes what schedules it.

## Out of scope

- `#import` on directories.
- A `.mate.yaml` list form. The attribute is per-file and visible in `ls`; if
  marking many files at once becomes tedious, that can be added later.
- Merging (e.g. JSON-aware three-way merge). The file is copied whole.
