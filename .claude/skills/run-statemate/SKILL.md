---
name: run-statemate
description: Build, run, and drive statemate (the `mate` CLI) — a declarative dotfiles/system config manager. Use when asked to run, start, build, test, smoke-test, or verify mate; to exercise its interactive prompts; or to check a change to apply/status/managed/scripts/packages behaviour end-to-end.
---

# Running statemate

`mate` is a Go CLI that deploys files from a dotfiles repo to your system. It has
no GUI or server — but it is **not** safely driven by hand, because it mutates the
real filesystem and several commands prompt on a TTY.

Two traps make ad-hoc testing give wrong answers:

- **State leaks.** The SQLite state DB lives at
  `$XDG_DATA_HOME/statemate/state.db`. Without overriding `XDG_DATA_HOME`, mate
  opens the developer's *real* DB and reports their actual dotfiles as orphans
  (~190 lines of noise on this machine).
- **A pipe is not a TTY.** `mate apply` calls `term.IsTerminal`; with piped
  stdin it takes the non-interactive branch and **skips every script**. So
  `printf 'y\n' | mate apply` cannot test the confirmation prompts — it silently
  exercises the wrong code path.

`.claude/skills/run-statemate/driver.py` handles both: it builds a disposable
dotfiles repo, isolates all state, and can run mate under a real pseudo-terminal.

All paths below are relative to the repo root.

## Prerequisites

Go (version from `go.mod`) and Python 3 for the driver. No system packages
needed. Optional: `golangci-lint` for `make lint`, `age` for encrypted-file work.

```bash
go version      # go1.26.5 here; go.mod requires 1.25.11
python3 --version
```

## Build

```bash
make build          # -> ./mate
make build-all      # -> dist/mate-{darwin-arm64,linux-amd64}
```

The driver builds automatically, so you rarely need this directly.

## Run (agent path)

**One command verifies the whole surface.** Start here:

```bash
python3 .claude/skills/run-statemate/driver.py smoke
```

Exits non-zero on failure. On success it prints 13 `ok:` lines and the scratch
dir path. Verified output:

```
[driver] ok: status lists a pending file
[driver] ok: status reports missing packages
[driver] ok: managed lists the templated file
[driver] ok: managed <path> matches one entry
[driver] ok: scripts list shows descriptions
[driver] ok: config source-dir
[driver] ok: apply deployed a file
[driver] ok: template was rendered
[driver] ok: perm-r attribute applied recursively
[driver] ok: --no-scripts skipped scripts
[driver] ok: apply prompts for script confirmation
[driver] ok: PTY prompt ran the script
[driver] ALL CHECKS PASSED
```

### Poking at it interactively

Create a scratch repo once and reuse it across calls:

```bash
export STATEMATE_SCRATCH=$(mktemp -d)/sm
python3 .claude/skills/run-statemate/driver.py scaffold
```

The scaffold covers a plain file, a `#template` file, a `#perm-r:755` directory,
a per-source `.mate.yaml` with packages, and two scripts (`#once#before` with a
`# Description:`, `#always#after`).

Run any mate command against it — stdin closed, non-interactive:

```bash
python3 .claude/skills/run-statemate/driver.py run status
python3 .claude/skills/run-statemate/driver.py run managed
python3 .claude/skills/run-statemate/driver.py run apply --no-scripts
```

`apply` still prints the package prompt (`Install? [y/N]`) — with stdin closed it
reads EOF and declines, then proceeds, so you'll see `applied 3` after it.

**To exercise prompts, use `pty` and feed one keystroke per line:**

```bash
# Decline both script prompts
python3 .claude/skills/run-statemate/driver.py pty --keys $'n\nn\n' apply

# Run the first, mark the second as done
python3 .claude/skills/run-statemate/driver.py pty --keys $'y\ns\n' apply
```

Verified prompt output — note `once` offers `[s]kip` and `always` does not:

```
Run setup.sh (before, once)?
  Create a marker proving the script ran
[y]es / [n]o / [s]kip (mark as done) / [a]ll / [q]uit: n
Run after.sh (after, always)?
[y]es / [n]o / [a]ll / [q]uit:
```

## Direct invocation (for PRs touching internals)

Most changes here land in `internal/` (recent commits: `internal/scripts`,
`internal/cli`, `internal/target`). Go tests are the fast inner loop — no driver
needed:

```bash
go test ./internal/scripts/ -run TestReadDescription -v
go test ./internal/cli/ -run TestManagedFilter -v
go test ./internal/target/ -run TestComputeChanges -v
```

## Test

```bash
make test    # go test -race -cover ./...  (10 packages, ~15s)
make lint    # golangci-lint run
make docs    # regenerate docs/ from cobra help text
```

Run `make docs` after changing any command's `Short`/`Long` — command help is the
source of truth and `docs/` is generated from it.

## Run (human path)

```bash
./mate status      # against your real dotfiles, from anywhere
./mate apply
```

Useless for verifying changes: it reads your live config and mutates `$HOME`.
Use the driver.

## Gotchas

- **`XDG_DATA_HOME`, not `XDG_STATE_HOME`.** The state DB is under
  `$XDG_DATA_HOME/statemate/`. Overriding `XDG_STATE_HOME` looks right and does
  nothing — you get the real DB and a flood of orphan warnings.
- **A pipe is not a TTY, so scripts silently don't run.** Piping stdin makes
  `mate apply` print `Warning: N script(s) skipped (no terminal to confirm on)`
  and skip them. Use `driver.py pty` for anything prompt-related.
- **`--force` is broader than it sounds.** It auto-confirms *package installs*
  as well as file conflicts and scripts. A fixture declaring an uninstallable
  package plus `--force` aborts the whole apply with
  `Error: installing packages: exit status 1`. The driver passes `--no-scripts`
  without `--force` and lets the package prompt read EOF and decline.
- **A source's packages ignore its `profile:` key.** `internal/packages/sync.go`
  collects `dirCfg.Packages` unconditionally; `profile:` in `.mate.yaml` gates
  *files* only. You cannot hide a package behind a non-matching profile.
- **`mate` resolves the repo from config, not cwd.** `~/.config/statemate/mate.yaml`
  sets `source_dir`, which beats the current directory. `cd`-ing into a scratch
  repo is **not** enough — you must set `STATEMATE_DIR` (the driver does).
- **Resolve symlinks in scratch paths.** On macOS `mktemp -d` returns
  `/var/...` which is a symlink to `/private/var/...`. `mate edit` and
  `mate managed <path>` compare resolved paths, so an unresolved path yields
  `... is not managed by mate`. The driver calls `.resolve()`.
- **`#` in filenames.** Safe as a mid-word shell argument in bash and zsh
  (`file.conf#template` needs no quoting). But it *is* nvim's alternate-file
  token, so `:edit path#encrypted` fails with `E194` — use `:edit path\#encrypted`
  or `vim.fn.fnameescape`.
- **`make build-all` skips linux/arm64.** Only darwin-arm64 and linux-amd64.

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `Warning: orphaned files` listing your real dotfiles | `XDG_DATA_HOME` not isolated. Use the driver, or set it explicitly. |
| `Warning: N script(s) skipped (no terminal to confirm on)` | Stdin is a pipe. Use `driver.py pty`. |
| `Error: installing packages: exit status 1` | `--force` tried to install a missing package. Drop `--force`. |
| `<path> is not managed by mate` for a file that clearly is | Unresolved symlink in the path, or `STATEMATE_DIR` unset so mate is reading a different repo. |
| `no config file found in <dir>` | `STATEMATE_DIR` points somewhere without `mate.yaml`. |
| Driver says `no scratch dir; run 'scaffold' first` | Run `scaffold`, or export `STATEMATE_SCRATCH`. |
| Pre-commit hook rejects your commit | Staged files under `internal/`/`cmd/` need a `CHANGELOG.md` entry. Add one, or `--no-verify` for genuinely internal changes. |
