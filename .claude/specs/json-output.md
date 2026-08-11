# Spec: `--json` machine-readable output

## Problem

Every command prints human-formatted text — aligned tables, symbol prefixes
(`+`/`~`/`!`), `~`-shortened paths, warnings on stderr. Anything consuming mate
programmatically has to screen-scrape that, which is fragile and breaks whenever
formatting changes.

The nvim integration built earlier is a concrete example: it parses
`mate managed <path>` by whitespace-splitting the first two columns, and has to
special-case the fact that a blank `ACTIVE` column shifts later fields.

## Scope

A global persistent `--json` flag, implemented by the **read-only reporting
commands**:

| Command | Payload |
| --- | --- |
| `status` | changes, orphans, missing packages, pending scripts, secrets |
| `managed` | managed file list with target/source/active/attrs |
| `packages status` | per-manager package status |
| `scripts list` | scripts with frequency, timing, status, description |
| `profile` | active profile, how it was detected, resolved sources |

JSON only — no `--yaml`. The internal structs are format-agnostic, so YAML can be
added later without reshaping anything.

## Flag behavior

`--json` is a **persistent flag on the root command**, so it parses uniformly and
appears in every command's help.

Commands that do not implement it must **error clearly**:

```
Error: --json is not supported by 'apply'
```

Silently ignoring the flag is not acceptable — a script that expects JSON would
otherwise parse human text and appear to work.

`--json` combined with `mate status --short` is an error: they are two competing
output formats, and silently picking one would surprise the caller.

## Output rules

- **Indented with 2 spaces.** Readable when inspected by hand; parsers are
  indifferent.
- **Absolute paths**, not `~`-shortened. Machine output should not require the
  consumer to expand anything.
- **Lowercase status strings** (`"new"`, `"modified"`, `"conflict"`), which is
  already what `ChangeStatus.String()` returns. Not the display symbols.
- **Empty collections are `[]`, never omitted.** Every documented key is always
  present, so `jq '.changes | length'` works unconditionally and consumers need
  no nil handling.
- **Warnings move into the payload.** Orphaned files, permission-skipped files,
  and no-TTY skipped scripts are data, not chatter — a consumer reading stdout
  alone gets the whole picture. stderr is reserved for actual errors.

## Schemas

Objects with named sections mirroring the text output, so a consumer can read
just the part it needs.

`mate status --json`:

```json
{
  "changes": [
    {
      "target": "/Users/you/.ssh/config",
      "source": "/Users/you/dotfiles/ssh/.ssh/config#encrypted",
      "source_name": "ssh",
      "status": "modified"
    }
  ],
  "orphans": ["/Users/you/.config/old/thing.conf"],
  "packages": [
    { "manager": "brew", "missing": ["jq", "ripgrep"] }
  ],
  "scripts": [
    {
      "name": "setup",
      "frequency": "once",
      "timing": "before",
      "description": "Bootstrap the environment"
    }
  ],
  "secrets": { "pending": 2 },
  "skipped": ["/etc/polkit-1/rules.d/50-pcscd.rules"]
}
```

`mate managed --json`:

```json
[
  {
    "target": "/Users/you/.ssh/config",
    "source": "/Users/you/dotfiles/ssh/.ssh/config#encrypted",
    "source_name": "ssh",
    "active": true,
    "attrs": { "encrypted": true, "template": false, "perm": "0600" }
  }
]
```

A top-level array is right here because `managed` is a homogeneous list, unlike
`status`'s five distinct sections. Note this makes the nvim integration's parsing
trivial and removes the blank-column ambiguity described above.

`packages status`, `scripts list`, and `profile` follow the same shape: arrays of
objects for lists, named fields for scalars.

## Exit codes

**Unchanged.** Output format must not alter semantics — `mate check` still exits 1
on drift, `status` still exits 0. A caller can rely on the exit code identically
in either mode.

## Errors

On failure (bad config, unreadable source), **stdout stays empty** and the
human-readable error goes to stderr with a non-zero exit. This makes a partial or
invalid parse impossible, and matches the convention established by
`mate config source-dir`.

Deliberately *not* emitting `{"error": "..."}`: if both success and failure
produced parseable JSON, callers would have to inspect a field instead of the exit
code, and a missed check would read an error as data.

## Implementation notes

- Add `--json` via `rootCmd.PersistentFlags()` alongside `--config`/`--profile`.
- A shared helper for the unsupported-command error keeps the message uniform;
  supporting commands opt in explicitly rather than every other command opting
  out.
- Define payload structs with `json:` tags in the CLI package rather than
  marshalling domain types directly, so the wire format is decoupled from
  internal representations and can stay stable as those evolve.
- Use `json.MarshalIndent` with two spaces.
- Initialise slices as `[]T{}` rather than leaving them nil, so they marshal to
  `[]` instead of `null`.

## Out of scope

- `--yaml` (structs are format-agnostic; can follow later).
- `--json` for mutating commands (`apply`, `add`, `clean`, ...).
- `diff`, `doctor`, `check`, `secrets list`.
- A `--compact` variant.
- Any stability guarantee on the schema.

## Acceptance criteria

1. `mate status --json` emits a valid JSON object with all documented keys
   present, empty ones as `[]`.
2. `mate managed --json` emits a JSON array; `mate managed <path> --json` emits a
   single-element array.
3. `packages status`, `scripts list`, and `profile` support `--json`.
4. Paths are absolute; statuses are lowercase strings.
5. `mate apply --json` errors with a clear "not supported" message and non-zero
   exit.
6. `mate status --short --json` errors.
7. Exit codes are identical with and without `--json`.
8. On error, stdout is empty and the message is on stderr.
9. Output parses under `jq` for every supported command.
10. Text output is byte-identical to before when `--json` is absent.
11. `make test` and `make lint` pass.
