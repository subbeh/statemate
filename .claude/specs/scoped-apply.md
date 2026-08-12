# Spec: scope `apply`, `status`, and `diff` to a file or source

## Problem

`mate apply` is all-or-nothing: it fetches secrets, runs before-scripts, applies
every file, prompts for every missing package, then runs after-scripts. While
iterating on one config file there is no way to deploy just that file.

`status` and `diff` do take a positional `[path]`, but it is matched loosely by
`matchesPath`, which prefix-matches so a bare word can match a whole source *or*
individual files. That ambiguity is why `apply` cannot simply reuse it —
"apply everything under nvim" and "apply this one file" need different behaviour.

## Decisions

### Two distinct scopes, distinguished by syntax not guessing

| Invocation | Scope | What runs |
| --- | --- | --- |
| `mate apply` | everything | all five phases, as today |
| `mate apply <path>` | **file** | matching files only |
| `mate apply -s <source>` | **source** | that source's files, scripts, and packages |

The positional argument is **always** a file/path filter. `--source`/`-s` is the
**only** way to select a source. No inference from whether a word happens to name
a source — the two scopes do different things, so guessing wrong would silently
do the wrong work.

`-s`/`--source` matches the flag `mate add` already uses for the same concept.

Single source per invocation; run the command twice for two sources.

### File scope: files only

`mate apply <path>` deploys just the matching files. **No** secret fetch, **no**
scripts, **no** package installs. This is the "I changed one config, push it"
path, and it should be fast and surprise-free.

### Source scope: that source's full lifecycle

`mate apply -s nvim` runs, for that source only:

- its files,
- its scripts (`nvim/.matescripts/` — **not** repo-root scripts, which are
  repo-wide and would overreach),
- its packages (the `packages:` in `nvim/.mate.yaml` — not global or other
  sources' packages).

Package filtering is feasible without restructuring: `PackageStatus.Sources`
already records which source contributed each package.

### The same rule applies to `status` and `diff`

For consistency across the three commands, `status` and `diff` also get
`-s`/`--source`, and their positional argument becomes a **file/path filter
only**.

**This is a behaviour change.** `mate status nvim` currently matches a whole
source via prefix matching; it will need `mate status -s nvim`. Accepted
deliberately: one rule across three commands beats three commands that each
interpret a bare word differently.

### Orphans: only report in-scope ones

A scoped run reports orphaned files that fall inside its scope and stays quiet
about the rest. Nagging about unrelated orphans during a focused apply is noise;
hiding in-scope ones would lose information the user asked for.

### Error when a positional matches nothing

If the positional matches no files **but does name a configured source**, fail
with a signpost rather than a bare error:

```
Error: no files match "nvim"; did you mean --source nvim?
```

This turns the deliberate positional/`--source` split into guidance at exactly
the moment someone trips over it. Exiting 0 having done nothing is not
acceptable — it reads as success.

## Implementation notes

- Scope resolution belongs in one shared helper used by all three commands, so
  they cannot drift the way the two change-detection paths did previously.
- `apply` currently takes `cobra.NoArgs` implicitly (no `Args` set); it needs
  `cobra.MaximumNArgs(1)`.
- Passing both a positional and `--source` should be rejected as contradictory
  rather than silently preferring one.
- `--source` should offer shell completion from the configured sources, reusing
  `completeSources` (already registered for `mate add`).
- The scoped file set feeds the existing `changedSources` computation, so
  `#onchange` scripts still see a correct change set within the scope.

## Out of scope

- Multiple `--source` values.
- Glob or pattern matching beyond what `matchesPath` already does for paths.
- Scoping `mate clean`, `mate check`, or `mate packages`.
- Running repo-root scripts under source scope.
- Changing what the five phases do when unscoped.

## Acceptance criteria

1. `mate apply <path>` deploys only matching files; no scripts, packages, or
   secret fetch run.
2. `mate apply -s <source>` deploys that source's files, runs only that source's
   scripts, and prompts only for that source's packages.
3. `mate apply` with no arguments behaves exactly as before.
4. Repo-root scripts do **not** run under `-s`.
5. A positional that matches no files but names a source errors with the
   `--source` suggestion and a non-zero exit.
6. Passing both a positional and `--source` is rejected.
7. `mate status -s <source>` and `mate diff -s <source>` filter by source.
8. `mate status <path>` and `mate diff <path>` filter by file/path only.
9. Scoped runs report only in-scope orphans.
10. `--source` tab-completes configured source names.
11. `make test` and `make lint` pass.
