# Secrets

Secrets are referenced inline in [templates](templates.md) and fetched from
Bitwarden into a local age-encrypted cache. There is no secrets section in the
config — a reference in a template is the declaration.

```gotemplate
{{ bitwarden "github.com" "field" "gh-cli-token" }}
```

## Why a cache

`mate apply` reads secrets from the cache, never from Bitwarden directly. That
means `apply`, `status` and `diff` work without unlocking your vault, and without
a network round trip per secret.

`mate secrets fetch` is the only command that talks to Bitwarden. `mate apply`
fetches missing secrets first as a convenience, but existing ones are never
re-read.

The cache lives at `~/.local/state/statemate/secrets.age` (or
`$XDG_STATE_HOME/statemate/secrets.age`), encrypted to your **local age identity
only** — not to the recipients in `mate.yaml`, since it never leaves the machine.
Override the location with:

```yaml
secrets_cache: "~/.local/state/statemate/secrets.age"
```

## Requirements

- The [Bitwarden CLI](https://bitwarden.com/help/cli/) (`bw`), logged in and
  unlocked. Statemate reports clearly when the vault is locked rather than failing
  obscurely.
- An age identity in [`age:`](configuration.md#age), used to encrypt the cache.

## Syntax

```gotemplate
{{ bitwarden "<item-name>" "<type>" "<field>" }}
```

`<item-name>` is the item's name in your vault.

| Type | `<field>` | Returns |
|------|-----------|---------|
| `field` | the custom field's name | A custom field's value |
| `login` | `username`, `password`, or `uri` | Part of a login item |
| `ssh` | `private` or `public` | An SSH key item's key |
| `attachment` | the attachment's filename | The attachment, **base64-encoded** |
| `totp` | *(ignored, pass `""`)* | A freshly generated TOTP code |

```gotemplate
{{ bitwarden "github.com" "field" "gh-cli-token" }}
{{ bitwarden "github.com" "login" "password" }}
{{ bitwarden "work-ssh-key" "ssh" "private" }}
{{ bitwarden "gpg-keys" "attachment" "user@example.com.priv.asc" }}
{{ bitwarden "github.com" "totp" "" }}
```

### Attachments

Attachments come back base64-encoded, because they may be binary. Decode them for
text content:

```gotemplate
{{ bitwarden "gpg-keys" "attachment" "key.asc" | base64Decode }}
```

`bitwardenAttachment` is a two-argument shorthand:

```gotemplate
{{ bitwardenAttachment "gpg-keys" "key.asc" | base64Decode }}
```

### Multi-line values

An SSH key or certificate needs indenting to sit inside structured output:

```gotemplate
sshKey: |
{{ indent 2 (bitwarden "work-key" "ssh" "private") }}
```

## Commands

```bash
mate secrets fetch             # fetch every discovered secret
mate secrets fetch "github*"   # only items matching a pattern
mate secrets list              # every reference, with cache status
mate secrets status            # only those needing a fetch
```

`mate status` counts secrets needing a fetch as `sN` in `--short`.

## Discovery

`mate secrets fetch` finds references by **rendering every template** with the
`bitwarden` function replaced by a recorder. This means a reference inside a
conditional is only discovered when that branch is taken under the current
profile — which is usually what you want, since the other branch's secret is not
needed on this machine.

It also means a template that fails to parse is skipped, and its secrets never
fetched. If a secret is unexpectedly missing, run `mate eval` on the file to see
the parse error.

During discovery `cmd` returns an empty string rather than shelling out, and
`required` never fails, so nothing aborts the walk. A reference inside
`{{ if cmd "..." }}` may therefore go unnoticed — put such a condition on a
variable instead.

## Keeping secrets out of the repository

Two mechanisms, for different needs:

- **`bitwarden` references** — the value lives in your vault, and only a reference
  is committed. Best for credentials that already belong in a password manager.
- **[`#encrypted` files](attributes.md#encrypted)** — the value is committed as
  age ciphertext. Best for files that are wholly secret, like a private key, and
  for machines that must apply without vault access.

They compose: an `#encrypted#template` file is decrypted, then rendered, so it can
hold `bitwarden` references too.
