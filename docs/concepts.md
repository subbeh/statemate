# Concepts

## Sources and targets

A **source** is a directory in your repository, listed under `sources:` in
`mate.yaml`. Its contents are deployed relative to a **target base**, which
defaults to your home directory:

```
~/dotfiles/                 ← source directory (contains mate.yaml)
  zsh/                      ← a source
    .zshrc                  →  ~/.zshrc
    .config/
      zsh/aliases.zsh       →  ~/.config/zsh/aliases.zsh
```

The path inside the source is preserved, minus the source's own name and any
[attributes](attributes.md) on its components: `zsh/.config#perm:700/zsh/aliases.zsh`
deploys to `~/.config/zsh/aliases.zsh`. Nothing is deployed by directory name, so
two sources can contribute to the same subtree. If two sources claim the *same*
target, that is a conflict and statemate refuses to apply until you resolve it.
Files excluded by [`#profile:`](attributes.md#profilename) do not count, so a
target may have one variant per profile.

A source can deploy somewhere other than home with `target_base` or `targets` in
its [`.mate.yaml`](configuration.md#source-directory-config) — this is how system
files under `/etc` are managed.

## State

`mate apply` records, for every file it writes:

- the **source hash** — the file as it exists in the repository
- the **applied hash** — the bytes actually written to the target

The database lives at `~/.local/share/statemate/state.db` (or
`$XDG_DATA_HOME/statemate/state.db`). It is local to each machine and is not
meant to be committed.

There is one database per user, keyed by target path, whichever repository you
run `mate` against. Pointing `mate` at a second repository therefore reports
every file the first one manages as an orphan. Never run `mate clean --all`
against a repository other than your usual one.

Two hashes rather than one is what lets statemate tell *which side* of a file
changed. For a plain file they are identical. For a `#template` or `#encrypted`
file they differ, because the target holds rendered or decrypted content — and
comparing the target against the source directly would report every such file as
permanently modified.

## How a change is classified

On each run statemate hashes the source, computes what the target *should*
contain, and compares both against the recorded state:

For a file with a record:

| Source vs recorded | Target vs recorded | Result | Marker |
|---|---|---|---|
| same | same | unchanged | |
| changed | same | modified, deploy | `~` |
| same | changed | conflict, or [import](attributes.md#import) | `!` / `<` |
| changed | changed | conflict | `!` |
| any | missing | modified, deploy again | `~` |

"Source changed" includes a template whose rendered output changed because a
variable or secret did, even when the template file itself did not.

For a file with **no** record, such as on a new machine or after `mate add`:

| Target | Result | Marker |
|---|---|---|
| missing | new, deploy | `+` |
| identical to what would be deployed | adopted silently: recorded on the next apply, nothing written | |
| different | conflict | `!` |

A **conflict** means the target changed without statemate's knowledge, so
overwriting it would destroy work. `mate apply` stops and asks, with a single
keypress:

```
Conflict: /home/you/.zshrc
  Target has been modified since last apply
  [o]verwrite, [i]mport, [s]kip, [d]iff, [a]bort:
```

`[i]mport` copies the target's content back into the source. It is not offered for
`#template` files, whose source is not the deployed content. `[d]iff` shows the
difference and asks again. For files where importing is always the right answer,
mark them [`#import`](attributes.md#import) and statemate stops asking.

`mate apply --force` overwrites every conflict without asking, and keeps no
backup. `--dry-run` still asks, so you can look at the diffs.

A mode difference counts as a change for files with a
[`#perm:`](attributes.md#perm600) attribute (or a `perm` default): the file shows
as modified and is fixed on apply. Without one, the mode is set when the file is
written and not checked afterwards. Owner and group are never checked.

### Untracked targets

If a file has no recorded state, as on a fresh machine or for a file you just
added, but the target already exists with different content, statemate reports a
conflict. With nothing recorded there is no way to know which side is newer, so it
asks rather than guessing. This applies to `#import` files too, on their first
encounter only.

A target that already matches, which is the usual case right after `mate add`, is
not reported at all. The next `mate apply` records it and reports it as
unchanged.

## Orphans

A file tracked in the database but no longer present in any active source, while
its target still exists, is an **orphan**. This happens when you delete a file
from the repository, or drop a source from `sources:`. statemate will not remove
the deployed copy until you say so.

```bash
mate status                 # lists orphans under a warning
mate clean                  # lists them too
mate clean ~/.old.conf      # removes one, after asking
mate clean --all            # removes all, asking for each
mate clean --all --force    # removes all, without asking
```

`mate forget <path>` drops the tracking but leaves the deployed file alone. That
only sticks once the file is gone from the source. If it is still there, the next
apply picks it up again.

## Profiles

A profile selects which sources and variables apply to the current machine.
Detection is automatic, based on hostname, user, OS, architecture, or the exit
status of a command. See [Configuration](configuration.md#profiles).

Profiles also filter individual files via the
[`#profile:`](attributes.md#profilename) attribute, and scripts via the same
attribute in their filename.

## The order of an apply

`mate apply` runs these phases:

1. Load and validate the config, including the [hooks](hooks.md) in `mate.yaml`
2. Fetch missing [secrets](secrets.md) (skipped for a path scope)
3. Scan sources, filter by profile, and refuse if two files claim one target
4. Narrow to any `--source` or path scope, and validate the hooks declared in
   sources
5. Run `#before` [scripts](scripts.md), then reload config (a script may have
   generated a var_file)
6. Write directories and files: create, modify, import, or prompt on conflict
7. Prompt for missing [packages](packages.md) (skipped for a path scope)
8. Run the hooks that the written files triggered
9. Run `#after` scripts

A config mistake caught in steps 1–4 stops the run before any script runs or any
file is written.

Pending changes are computed *once*, before step 5, so an `#onchange` script sees
the same set whether it runs before or after — by the time files are written there
are no pending changes left to observe.
