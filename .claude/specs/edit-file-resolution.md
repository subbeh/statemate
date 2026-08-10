# Spec: `mate edit` file resolution and completion

## Problem

Two related bugs in `mate edit`:

1. **Include/var_files can't be edited.** `findSourceEntry` only searches
   `tree.Files()` (the scanned source tree). Files listed under `include:` or
   `var_files:` — e.g. `.matedata/secrets.yaml#encrypted` — are not in the tree,
   so `mate edit .matedata/secrets.yaml#encrypted` fails with
   `Error: file not found`.

2. **Completion offers what edit can't open.** `completeSourceFiles` adds
   include/var_files via `resolveExtraFiles`, so the shell suggests
   `.matedata/secrets.yaml#encrypted` — the exact path that then errors. It also
   returns almost nothing in most directories because `cwdRelativeCompletion`
   only yields a path when cwd is inside a source dir or the target is under cwd.

## Decisions

### Resolution semantics — strict, CLI-like

`mate edit <path>` treats `<path>` like any normal CLI file argument:

- Absolute paths resolve as-is.
- Relative paths resolve against cwd.
- **No suffix/fuzzy search.** Bare `init.lua` resolves only if it exists in cwd.
  This is an accepted behavior change: `mate edit nvim/init.lua` from outside the
  repo root previously worked via suffix match and will now fail.

Path forms that must work:

| Input | Result |
| --- | --- |
| `.matedata/secrets.yaml#encrypted` (relative, from repo root) | opens that file, decrypted |
| `/abs/path/to/dotfiles/nvim/init.lua` | opens that source file |
| `nvim/init.lua` (relative, from repo root) | opens that source file |
| `~/.config/nvim/init.lua` (target path) | opens the **source** that produces it |
| `~/.bashrc` (target, unmanaged) | error — not managed |
| `../outside/file.txt` | error — not managed |

### Always edit the source

We never edit a deployed target in place. When the given path is a target path,
resolve it to the managed source entry and open that. If no managed source
produces that target, error.

### What counts as "managed"

A file is editable if **either**:

- it lives anywhere under `source_dir` (the repo root), **or**
- it is a target path that maps to a managed source entry.

"Anywhere under `source_dir`" deliberately includes files excluded by `ignore`
(e.g. `*.md`) and repo-root files like `mate.yaml`. Anything else errors with a
clear "not managed by mate" message.

### Encryption detection — filename suffix only

A file is treated as encrypted iff its name contains the `#encrypted` suffix.
This matches the convention used everywhere else in mate. No content sniffing —
we do not want a second, inconsistent detection path.

For encrypted files the flow is unchanged in shape: decrypt → temp file → editor
→ re-encrypt → write back.

### Permissions

The re-encrypted file must be written with **the mode the file already had**
(stat before editing, restore after). The current code uses `entry.Mode.Perm()`
from the scan tree, which does not exist for include/var_files.

### Temp file safety

- Assert the plaintext temp file is `0600` (Go's `os.CreateTemp` already does
  this; make it explicit so a future change can't silently loosen it).
- Temp file is removed on success as today.

### Completion — plain filesystem completion

Remove the custom completion for `edit` entirely. Return
`cobra.ShellCompDirectiveDefault` so the shell completes real files and
directories exactly like `ls` or `vim` would.

Rationale: with strict cwd-relative resolution, the filesystem *is* the correct
completion source. A hand-built list can only drift from what resolution
accepts — which is precisely bug #2. Validation ("is this managed?") happens when
`edit` runs, not during completion.

This means `completeSourceFiles` is no longer used by `edit`. Check whether any
other command still needs it before deleting it.

### Post-edit hint

After successfully saving a managed source file, print a one-line reminder that
the file now differs from its target and `mate apply` will deploy it. Do not
prompt, and do not run apply.

## Out of scope

- Editing deployed target files in place (no `--target` flag).
- Content-based encryption detection.
- Resolving unmanaged files outside `source_dir`.
- Handling editor swap/backup files that may contain plaintext.

## Acceptance criteria

1. `mate edit .matedata/secrets.yaml#encrypted` from the repo root opens the
   decrypted contents and re-encrypts on save, preserving the original file mode.
2. `mate edit nvim/init.lua` from the repo root opens the source file.
3. `mate edit ~/.config/nvim/init.lua` opens the corresponding **source** file.
4. `mate edit ~/.bashrc` (unmanaged target) errors with a clear message.
5. `mate edit ../somewhere/else.txt` (outside source_dir) errors.
6. Tab completion after `mate edit ` lists real files/dirs relative to cwd, and
   every completion offered either opens or errors with the "not managed"
   message — never "file not found".
7. Editing a managed source prints a pending-changes hint.
8. `make test` and `make lint` pass.
