# Encryption and Secrets

A dotfiles repository is usually public, or at least pushed somewhere you do not
fully control. statemate keeps secrets out of it in two ways, and both rest on
one [age](https://age-encryption.org) key:

| | [`#encrypted` files](attributes.md#encrypted) | [Bitwarden references](secrets.md) |
|---|---|---|
| The repository holds | age ciphertext | only a reference, such as `{{ bitwarden "github" "field" "token" }}` |
| The value lives | in the repository | in your Bitwarden vault |
| Applying needs | your age key | your age key, and the vault for the first fetch |
| Best for | whole files: an SSH config, a kubeconfig, an app's settings | single values inside otherwise ordinary files |

This page sets up both.

## 1. Create an age key

```bash
mkdir -p ~/.config/statemate
age-keygen -o ~/.config/statemate/key.txt
chmod 600 ~/.config/statemate/key.txt
```

`age-keygen` prints the public key, which looks like `age1ql3z7hjy54…`. The file
holds the private key, `AGE-SECRET-KEY-1…`.

The `age-keygen` command comes with age (`brew install age`, `pacman -S age`).
statemate itself has age built in and does not need the `age` binary.

## 2. Configure it

```yaml
# mate.yaml
age:
  identity: "~/.config/statemate/key.txt"
  recipients:
    - age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
```

- `recipients` is what files are encrypted *to*. Public keys are safe to commit.
- `identity` is the private key used to decrypt. Only the path is committed, so
  use the same path on every machine.

## 3. Back up the private key

**Without the private key, every `#encrypted` file is unrecoverable.** Store a copy
somewhere that does not depend on this repository. A password manager works well:
statemate's author keeps it as a custom field of a Bitwarden item, which is also
what makes [bootstrapping a new machine](new-machine.md) a single step.

Instead of a file, you can have statemate read the key from the password manager
on demand with `identity_command`:

```yaml
age:
  identity_command: "rbw get statemate --field age-key"
  recipients: [age1ql3z7hjy54…]
```

The command must print only the `AGE-SECRET-KEY-1…` line. `cat`-ing the file
`age-keygen` wrote does not work, because that file also contains comment lines.

## 4. Encrypt files

Add a file encrypted from the start:

```bash
mate add --encrypt ~/.ssh/config
```

Or encrypt one that is already managed:

```bash
mate encrypt ssh/.ssh/config          # becomes ssh/.ssh/config#encrypted
```

The repository now holds armored ciphertext, while `~/.ssh/config` stays plaintext.
To work with encrypted sources:

```bash
mate edit ~/.ssh/config     # decrypt to a temp file, edit, re-encrypt on save
mate cat ssh/.ssh/config#encrypted
mate diff                   # diffs the decrypted content
```

To make `git diff` readable too, see
[Tips: readable diffs](tips.md#readable-diffs-for-encrypted-files).

### More than one key

Each machine can have its own key; list every public key under `recipients`.
Files are encrypted to the recipients at the time they are encrypted, so after
adding a key, re-encrypt existing files for the new machine to read them:

```bash
mate decrypt path/to/file#encrypted && mate encrypt path/to/file
```

One shared key, kept in a password manager, avoids that bookkeeping.

## 5. Secrets from Bitwarden

For values that already live in your vault, reference them from a
[template](templates.md) instead of committing them at all:

```gotemplate
# git/.config/gh/hosts.yml#template
github.com:
  oauth_token: {{ bitwarden "github.com" "field" "gh-cli-token" }}
```

You need the [Bitwarden CLI](https://bitwarden.com/help/cli/) (`bw`), logged in
once:

```bash
bw login
mate secrets fetch
```

`mate secrets fetch` finds every reference, asks for your master password if the
vault is locked, and stores the values in a local cache encrypted with your age
identity. From then on `mate apply`, `status` and `diff` read the cache and never
touch the vault, and `mate apply` fetches only references it has not seen before.
See [Secrets](secrets.md) for the reference syntax and the cache.

### Generating files from secrets

A source's `.mate.yaml` can create files straight from the vault, without a
source file per secret. This one writes one SSH private key per name in a
variable:

```yaml
# mate.yaml
profiles:
  work:
    variables:
      sshKeys: [laptop-work-ssh-key, github-ssh-key]
```

```yaml
# ssh/.mate.yaml
generate:
  {{- range $key := (var "sshKeys") }}
  - target: ".ssh/keys.d/id_{{ $key }}"
    mode: "0600"
    content: |
{{ indent 6 (bitwarden $key "ssh" "private") }}
  {{- end }}
```

Each name is a Bitwarden SSH-key item. `indent 6` keeps the multi-line key inside
the YAML block scalar. See [`generate`](configuration.md#generate).

## What needs what

| Operation | Needs |
|-----------|-------|
| `mate encrypt`, `mate add --encrypt` | `recipients` |
| Deploying, `mate cat`, `mate eval` on an `#encrypted` file | `identity` or `identity_command` |
| `mate edit` on an `#encrypted` file | both |
| `mate secrets fetch`, and the secrets cache in general | `identity` or `identity_command`, plus `bw` |
| Importing into an `#encrypted` source ([`#import`](attributes.md#import)) | `recipients` |

Without the identity, statemate stops with an error instead of deploying
ciphertext or writing the secrets cache unencrypted.
