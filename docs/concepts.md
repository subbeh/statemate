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

The path inside the source is preserved verbatim; only the source's own name is
stripped. Nothing is deployed by directory name, so two sources can contribute to
the same subtree. If two sources claim the *same* target, that is a conflict and
statemate refuses to apply until you resolve it.

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

Two hashes rather than one is what lets statemate tell *which side* of a file
changed. For a plain file they are identical. For a `#template` or `#encrypted`
file they differ, because the target holds rendered or decrypted content — and
comparing the target against the source directly would report every such file as
permanently modified.

## How a change is classified

On each run statemate hashes the source, computes what the target *should*
contain, and compares both against the recorded state:

| Source vs recorded | Target vs recorded | Result | Marker |
|---|---|---|---|
| same | same | unchanged | |
| changed | same | modified — deploy | `~` |
| same | changed | conflict, or [import](attributes.md#import) | `!` / `<` |
| changed | changed | conflict | `!` |
| — | missing | new — deploy | `+` |

A **conflict** means the target changed without statemate's knowledge, so
overwriting it would destroy work. `mate apply` stops and asks:

```
[o]verwrite, [i]mport, [s]kip, [d]iff, [a]bort
```

`[i]mport` copies the target's content back into the source. For files where that
is always the right answer, mark them [`#import`](attributes.md#import) and
statemate stops asking.

Permission differences count as changes too: a file with correct content but the
wrong mode shows as modified and is fixed on apply.

### Untracked targets

If a file has no recorded state — a fresh machine, or a file you just added — but
the target already exists with different content, statemate reports a conflict.
With nothing recorded there is no way to know which side is newer, so it asks
rather than guessing. This applies to `#import` files too, on their first
encounter only.

## Orphans

A file tracked in the database but no longer present in any source is an
**orphan**. `mate status` warns about them; `mate clean` removes them. This
happens when you delete a file from the repository — statemate will not remove the
deployed copy until you say so.

Use `mate forget` to drop tracking while leaving the deployed file alone.

## Profiles

A profile selects which sources and variables apply to the current machine.
Detection is automatic, based on hostname, user, OS, architecture, or the exit
status of a command. See [Configuration](configuration.md#profiles).

Profiles also filter individual files via the
[`#profile:`](attributes.md#profilename) attribute, and scripts via the same
attribute in their filename.

## The order of an apply

`mate apply` runs these phases:

1. Scan sources, filter by profile and by any `--source`/path scope
2. Fetch missing [secrets](secrets.md)
3. Run `#before` [scripts](scripts.md), then reload config (a script may have
   generated a var_file)
4. Write files: create, modify, import, or prompt on conflict
5. Prompt for missing [packages](packages.md)
6. Run `#after` scripts

Pending changes are computed *once*, before step 3, so an `#onchange` script sees
the same set whether it runs before or after — by the time files are written there
are no pending changes left to observe.
