# File Attributes

Attributes are `#`-suffixes on a filename that control how a file is deployed.
They are stripped from the target name, so `config#encrypted#perm:600` deploys as
`config`.

```
.ssh/config#encrypted#perm:600      →  ~/.ssh/config, decrypted, mode 0600
```

Order does not matter, and any number can be combined. An unrecognised attribute
is ignored silently but still stripped, so a typo such as `config#templte`
deploys an unrendered `config`. Everything after the first `#` in a name is
attributes, so a file whose real name contains `#` cannot be managed.

| Attribute | Effect |
|-----------|--------|
| [`#template`](#template) | Render as a Go template |
| [`#tmpl`](#template) | Alias for `#template` |
| [`#encrypted`](#encrypted) | Decrypt with age before deploying |
| [`#symlink`](#symlink) | Recreate a symlink (source must be one) |
| [`#import`](#import) | Target is authoritative; changes flow back to the source |
| [`#profile:name`](#profilename) | Only deploy under this profile |
| [`#perm:600`](#perm600) | Set file mode |
| [`#owner:user`](#owneruser) | Set owner |
| [`#group:group`](#groupgroup) | Set group |
| [`#perm-r:600`](#recursive-attributes) | Mode for a directory and its children |
| [`#owner-r:user`](#recursive-attributes) | Owner for a directory and its children |
| [`#group-r:group`](#recursive-attributes) | Group for a directory and its children |

Attributes are written on directories too. There, only some take effect:

| On a directory | Effect |
|----------------|--------|
| `#profile:name` | The directory and everything beneath it deploy only under that profile |
| `#perm:`, `#owner:`, `#group:` | The directory itself |
| `#perm-r:`, `#owner-r:`, `#group-r:` | The directory and everything beneath it |
| anything else | Nothing, but it is still stripped from the name |

The source directory's own name counts as well: a source named
`etc#owner-r:root` makes every file in it root-owned.

## `#template`

Renders the file with Go's `text/template` before writing it. See
[Templates](templates.md) for the available variables and functions.

```
gitconfig#template
```

```gotemplate
[user]
    email = {{ .Vars.email }}
```

`#tmpl` is an equivalent short form.

Because the deployed content differs from the source, statemate compares the
*rendered* output against the target. A template whose variables change is
detected as modified even though the source file itself did not change.

## `#encrypted`

Marks the file as age-encrypted in the repository. It is decrypted on apply, so
the target holds plaintext while the repository holds ciphertext.

```
.ssh/id_ed25519#encrypted#perm:600
```

Deploying needs an age identity (`identity` or `identity_command` in
[`age:`](configuration.md#age)) to decrypt with. Without one, `mate apply` stops
with an error rather than deploy ciphertext. Encrypting needs only the
`recipients`. See [Encryption](encryption.md) for setting up a key.

Use `mate encrypt` and `mate decrypt` to convert a file in place. They add and
remove the suffix for you.

`mate edit` decrypts to a temporary file, opens your editor, and re-encrypts on
save, preserving the original mode. `mate cat` decrypts to stdout.

Combines with `#template`: the file is decrypted first, then rendered.

## `#symlink`

Reproduces a symlink at the target. **The source file must itself be a symlink**;
statemate reads where it points and recreates a link to that same destination.

```bash
cd ~/dotfiles/mysource
ln -s /opt/homebrew/bin/nvim 'bin/vim#symlink'
```

```
~/dotfiles/mysource/bin/vim#symlink  →  /opt/homebrew/bin/nvim
                                  ⇓ apply
~/bin/vim                           →  /opt/homebrew/bin/nvim
```

Use it for links to paths outside your repository, such as a binary in `/opt` or a
large directory you do not want to copy. Note that this does *not* link the target back
at the source file, so editing the deployed file does not edit the repository.

The link text is copied verbatim, so a relative link resolves relative to the
*target's* location, not the source's. Change detection compares the link text:
retargeting the source link is a change, and a link that points nowhere on this
machine is deployed as-is.

A `#symlink` attribute on a regular file is an error, reported by `mate status`
and `mate apply` alike:

```
Error: computing changes: readlink .../f.txt#symlink: invalid argument
```

Because the link destination is copied verbatim, `#template`, `#encrypted`,
`#perm:`, `#owner:` and `#group:` have no effect alongside `#symlink`: there is
no content to render, decrypt, chmod, or chown. Links are created without sudo,
so a `#symlink` cannot target a directory you cannot write to.

If a target is a symlink but the source is not marked `#symlink`, statemate treats
it as a conflict the first time it sees it, rather than silently replacing it.
This is what happens on the first apply after migrating from GNU Stow; answer
`[o]verwrite` to replace the links with real files.

## `#import`

Marks a file whose **target is normally authoritative**, because the application
owns it and rewrites it. `~/.claude/settings.json` is the motivating case: it
changes whenever you toggle a setting, so without `#import` every `mate apply`
stops to ask about a drifted target, and the answer is always "import".

```
claude/.claude/settings.json#encrypted#import
```

| Source | Target | Result |
|--------|--------|--------|
| unchanged | unchanged | nothing to do |
| changed | unchanged | source deployed to target, as usual |
| unchanged | changed | **target imported into the source, no prompt** |
| changed | changed | conflict prompt |

Divergence on both sides still prompts, deliberately: letting the target win there
would discard an edit you made on purpose.

A missing target is created from the source, so `#import` works when setting up a
new machine — you get the file, and from then on the application's edits flow
back. On the *first* encounter with a pre-existing untracked target, statemate
still prompts once, because no recorded state exists to say which side is newer.

`mate status` marks a pending import with `<` (`<N` in `--short`), and `mate diff`
shows the diff in the import direction — target as the new side. `mate apply`
prints `← path (imported)`.

Composes with `#encrypted`: imported content is encrypted before being written to
the source, so the repository never holds plaintext.

**Cannot be combined with `#template`.** Importing would write the rendered output
over the template, destroying the source. Statemate rejects the combination with
an error rather than doing it.

## `#profile:name`

Deploys the file only when the named profile is active. Profile inheritance is
respected, so a file marked `#profile:base` also applies under a profile that
`extends: base`.

```
.gitconfig#profile:work
```

Under any other profile the file is skipped entirely: it is neither deployed nor
reported as a change.

> When **no** profile is active, there is nothing to filter by and every
> `#profile:` file is deployed. Two variants of the same target then conflict.
> Define a profile for every machine; see [Different Machines](machines.md).

Several files may therefore claim the same target, one per profile:

```
.claude/settings.json#profile:personal
.claude/settings.json#profile:work
```

Only variants that deploy together count as a
[conflict](concepts.md#sources-and-targets) — which happens when inheritance brings
two of them into the same profile chain, such as a `#profile:base` variant next to
a `#profile:work` one under a profile that `extends: base`.

## `#perm:600`

Sets the file mode, in octal.

```
.ssh/config#perm:600
```

Without this attribute the source file's own mode is used when the file is
written, and the target's mode is not checked afterwards. With it, a target whose
mode differs is reported as modified and corrected on apply.

A value that is not valid octal, such as `#perm:rw`, is ignored.

## `#owner:user`

Sets the file owner. statemate uses sudo for targets in locations you cannot
write to, such as `/etc`. In a location you *can* write to, the chown runs as you
and fails unless you are root, so `#owner:` belongs on system files.

Ownership is applied when the file is written. Unlike `#perm:`, a target whose
owner differs is not reported as modified, so adding `#owner:` to a file that is
already deployed takes effect the next time its content changes.

```
etc/nginx/nginx.conf#owner:root
```

Usually more convenient as a source-wide default in
[`.mate.yaml`](configuration.md#source-directory-config), or as `#owner-r:` on a
parent directory.

## `#group:group`

Sets the file group, with the same elevation caveat as `#owner:`.

```
etc/wireguard/wg0.conf#group:systemd-network
```

## Recursive attributes

`#perm-r:`, `#owner-r:` and `#group-r:` on a directory apply to that directory
*and* everything inside it. Children inherit the value as a default, so an
explicit attribute on a child still wins, and the nearest ancestor's value wins
over one further up.

`#perm-r:` gives files and subdirectories the **same** mode. That suits `755`
(a `bin` directory) and `700`, but `#perm-r:600` would leave subdirectories
untraversable. For a private directory of private files, combine `#perm:700` on
the directory with `#perm:600` on the files.

The order of precedence for a file's mode, owner, and group:

1. Its own attribute
2. The nearest ancestor's `-r` attribute
3. The `perm`, `owner` and `group` defaults in the source's
   [`.mate.yaml`](configuration.md#source-directory-config)

```
etc#owner-r:root#group-r:root/
  ssh/
    ssh_config.d/
      50-storagebox.conf        ← owned by root:root, inherited
      99-local.conf#owner:me    ← explicitly overridden
```

This is how a whole system directory is managed without annotating every file. A
real example, deploying to `/etc` as root:

```
restic/
  .mate.yaml                    # targets: { etc: /etc }
  etc#owner-r:root/
    ssh/ssh_config.d/50-storagebox.conf#template
```

Note that directories which already exist and carry no perm/owner/group attribute
are left completely untouched, so mapping a root like `/etc` never chmods it.

## Empty directories

Every directory in a source is created on the target, whether or not it contains
files. That is how a directory which only needs to *exist* is declared — a
socket directory, a cache directory some program refuses to create itself:

```
ssh/
  .cache/ssh/controlmasters#perm:700/
    .gitkeep                    ← so git stores the directory at all
  .ssh/config#template
```

Git cannot store an empty directory, so the keep-file is unavoidable. Hide it
from the target with an [`ignore`](configuration.md#source-directory-config)
pattern in the source's `.mate.yaml`; ignoring a file does not remove its parent
directory from the tree:

```yaml
ignore:
  - .gitkeep
```

`mate status` lists a declared empty directory as `+` until it exists. Directories
that contain files are not listed separately — they arrive with those files.

Deleting the directory from the source stops it from being created, but does not
remove it from the target: unlike files, directories are not tracked in the state
database, so `mate status` will not report it as orphaned.
