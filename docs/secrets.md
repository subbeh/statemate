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

Two commands talk to Bitwarden: `mate secrets fetch`, which re-reads every
reference, and `mate apply`, which fetches only references missing from the cache
before it reads or deploys anything. `mate status`, `diff` and `eval` only read
the cache, so on a new machine run `mate apply` or `mate secrets fetch` before
them. If that fetch fails, the apply stops. `mate apply
<path>` skips it, and `--dry-run` only reports how many it would fetch.

Because values are cached, a changed secret in the vault is not picked up until
the next `mate secrets fetch`.

The cache lives at `~/.local/state/statemate/secrets.age` (or
`$XDG_STATE_HOME/statemate/secrets.age`), encrypted to your **local age identity
only** rather than to the recipients in `mate.yaml`, since it never leaves the
machine. With no identity configured, statemate refuses to fetch rather than
write secrets to disk unencrypted. Override the location with:

```yaml
secrets_cache: "~/.local/state/statemate/secrets.age"
```

## Requirements

- The [Bitwarden CLI](https://bitwarden.com/help/cli/) (`bw`), logged in with
  `bw login`. Only `bw` is supported, not `rbw`.
- An age identity, `identity` or `identity_command` in
  [`age:`](configuration.md#age), used to encrypt the cache. See
  [Encryption](encryption.md) for creating one.

When the vault is locked, statemate unlocks it for you: first with `bw unlock`
alone (for biometric or PIN unlock), then by asking for your master password,
which is handed to `bw` through an environment variable rather than the command
line. The session lasts for that one `mate` run. To unlock once for several
runs, export `BW_SESSION` yourself:

```bash
export BW_SESSION=$(bw unlock --raw)
```

Every fetch runs `bw sync` first, so recent vault changes are seen.

## Syntax

```gotemplate
{{ bitwarden "<item-name>" "<type>" "<field>" }}
```

`<item-name>` is the item's name in your vault, matched exactly. If two items share
a name, the first one wins.

| Type | `<field>` | Returns |
|------|-----------|---------|
| `field` | the custom field's name | A custom field's value |
| `login` | `username`, `password`, or `uri` | Part of a login item; `uri` is the first URI |
| `ssh` | `private` or `public` | An SSH key item's key |
| `attachment` | the attachment's filename | The attachment, **base64-encoded** |
| `totp` | *(ignored, pass `""`)* | The TOTP code generated at fetch time, which is cached like everything else, so it is stale by the time a file is deployed |

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
mate secrets fetch "github*"   # only items whose name starts with "github"
mate secrets fetch github.com  # only the item named exactly "github.com"
mate secrets list              # every reference, with cache status
mate secrets status            # only those needing a fetch
```

The pattern is an exact item name, or a prefix ending in `*`; other glob
characters are not supported.

A fetch is all or nothing. If one reference names an item or field that does not
exist, nothing is saved, and `mate secrets fetch` offers to carry on with the
existing cache.

`mate status` counts secrets needing a fetch as `sN` in `--short`.

## Discovery

`mate secrets fetch` finds references by **rendering every template** with the
`bitwarden` function replaced by a recorder: `#template` files in active sources,
`#template` scripts in any `.matescripts/`, and every source's `.mate.yaml`. This means a reference inside a
conditional is only discovered when that branch is taken under the current
profile — which is usually what you want, since the other branch's secret is not
needed on this machine.

It also means a template that fails to parse is skipped, and its secrets never
fetched. If a secret is unexpectedly missing, run `mate eval` on the file to see
the parse error.

During discovery `cmd` returns an empty string rather than shelling out,
`required` never fails, and `base64Decode` passes its input through, so nothing
aborts the walk. A reference inside
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
