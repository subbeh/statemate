# File Attributes

Attributes are `#`-suffixes on a filename that control how a file is deployed.
They are stripped from the target name, so `config#encrypted#perm:600` deploys as
`config`.

```
.ssh/config#encrypted#perm:600      →  ~/.ssh/config, decrypted, mode 0600
```

Order does not matter, and any number can be combined. An unrecognised attribute
is ignored silently.

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

Attributes work on directories as well as files. On a directory, `#perm:`,
`#owner:` and `#group:` apply to the directory itself; the `-r` variants also
apply to everything beneath it.

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

Requires an `age:` identity in [`mate.yaml`](configuration.md#age). Use
`mate encrypt` and `mate decrypt` to convert a file in place — they add and
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

Use it for links to paths outside your repository — a binary in `/opt`, a large
directory you do not want to copy. Note that this does *not* link the target back
at the source file, so editing the deployed file does not edit the repository.

Applying a `#symlink` attribute to a regular file fails:

```
Error: creating symlink: readlink .../f.txt#symlink: invalid argument
```

Because the link destination is copied verbatim, `#template`, `#encrypted` and
`#perm:` have no effect alongside `#symlink` — there is no content to render,
decrypt, or chmod.

If a target is a symlink but the source is not marked `#symlink`, statemate treats
it as a conflict rather than silently replacing it.

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

Under any other profile the file is skipped entirely — not deployed, not reported
as a change.

## `#perm:600`

Sets the file mode, in octal.

```
.ssh/config#perm:600
```

Without this attribute the source file's own mode is used. With it, a target whose
mode differs is reported as modified and corrected on apply.

## `#owner:user`

Sets the file owner. Applying this needs elevated access, so statemate uses sudo
for files whose owner it cannot otherwise set.

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
explicit attribute on a child still wins.

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
