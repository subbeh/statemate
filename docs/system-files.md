# Managing System Files

statemate can manage files outside your home directory, such as `/etc/keyd`,
systemd units, `/etc/ssh/ssh_config.d`, using the same repository and the same
`mate apply`. It uses `sudo` for paths you cannot write to.

## Map a directory onto `/etc`

Give a source a [`.mate.yaml`](configuration.md#source-directory-config) that maps
its `etc/` subdirectory onto `/etc`, and mark that directory as root-owned:

```yaml
# arch/.mate.yaml
targets:
  etc: /etc
```

```
arch/
  .mate.yaml
  etc#owner-r:root/
    keyd/default.conf             →  /etc/keyd/default.conf, owned by root
    pacman.d/hooks/paccache.hook  →  /etc/pacman.d/hooks/paccache.hook
  .config/hypr/hyprland.conf      →  ~/.config/hypr/hyprland.conf
```

Only the first path component is looked up in `targets`, so `etc` maps the whole
subtree. Everything that does not match a mapping still deploys relative to your
home directory, so one source can hold a tool's system files and its user
configuration side by side.

`#owner-r:root` on the directory makes every file beneath it root-owned without
annotating each one (see [recursive attributes](attributes.md#recursive-attributes)).
Add `#perm:600` to anything that holds a secret:

```
restic/
  etc#owner-r:root/
    restic/password#template#perm:600
```

Several mappings can share a source:

```yaml
targets:
  etc: /etc
  root: /root
```

> If a `.mate.yaml` sets `target_base`, its `targets` are ignored. Use one or the
> other: `target_base: /` deploys the *whole* source relative to `/`, so an
> `etc/` directory lands in `/etc` and nothing in the source goes to `~`.

## Adding an existing system file

`mate add` handles the mapping for you once it exists:

```bash
mate add /etc/keyd/default.conf --source arch
```

The file is copied into `arch/etc#owner-r:root/keyd/default.conf`, reusing the
attributed directory that is already there. `mate add` reads the file as you,
so a root-only file such as `/etc/sudoers` fails with `permission denied`. Copy
that one into the source by hand with `sudo cat`.

For a source with no `.mate.yaml` yet, `mate add` offers to create one with
`target_base` set to the file's top-level directory (`/etc`). That is right for a
source dedicated to system files. For a source that also holds home files, write
the `targets:` mapping yourself first.

## How sudo is used

statemate does not run as root. For each target it checks whether you can write
to the path, or to its nearest existing parent, and uses `sudo` only where you
cannot. The first write that needs sudo asks for your password, and sudo's
credential cache covers the rest of the run, so with a normal sudo setup you are
asked once per apply.

- **Existing directories are left alone** unless they carry a `#perm`, `#owner` or
  `#group` attribute that differs from what is on disk. Mapping `/etc` never
  chmods `/etc` itself.
- **Missing directories** are created with sudo where needed, with the attributes
  their source directory declares.
- **Ownership is applied when a file is written.** A change of owner or group on
  its own is not detected, unlike a mode change under `#perm`. Adding `#owner:`
  to a file that is already deployed takes effect the next time its content
  changes; `chown` it by hand to fix it now.

### Reading root-only targets

`mate status`, `mate diff` and `mate check` run without sudo by default, so a
target you cannot read is skipped with a warning:

```
Warning: 2 file(s) skipped (permission denied, use --sudo to check)
```

Pass `--sudo` to read them:

```bash
mate status --sudo
mate diff --sudo /etc/sudoers.d/10-wheel
```

## Reload services after a change

System files usually need a service reloaded. A [hook](hooks.md) runs only when a
matching file was actually written:

```yaml
# mate.yaml
hooks:
  systemd-reload:
    profile: linux
    match:
      - "/etc/systemd/**/*.service"
      - "/etc/systemd/**/*.timer"
    do:
      - run: sudo systemctl daemon-reload

  keyd:
    match: /etc/keyd/*.conf
    do:
      - run: sudo systemctl restart keyd
```

Hook commands are not elevated automatically, so write `sudo` yourself. After an
apply that wrote system files, the sudo session it opened usually means no
second password prompt.

## One-time system setup

Enabling a service, adding yourself to a group, or enrolling a fingerprint happens
once per machine rather than on every change. Use a [`#once` script](scripts.md):

```bash
#!/usr/bin/env bash
# arch/.matescripts/00-keyd.sh#once#after
# Description: Enable keyd
sudo systemctl enable --now keyd
```

It is confirmed before it runs, recorded in the state database, and never offered
again on this machine.
