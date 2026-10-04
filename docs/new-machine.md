# Setting Up a New Machine

With statemate, a new machine needs four things: the `mate` binary, your
repository, your age key if you use encryption, and one `mate apply`. This
page covers doing that by hand, and then automating it with a bootstrap script.

## By hand

```bash
# 1. Install mate
brew install subbeh/tap/statemate          # or yay/paru -S statemate-bin

# 2. Restore the age key (skip if you don't use encryption)
mkdir -p ~/.config/statemate
bw login
bw get item statemate | jq -r '.fields[] | select(.name=="age-key").value' \
  > ~/.config/statemate/key.txt
chmod 600 ~/.config/statemate/key.txt

# 3. Clone and register the repository
git clone https://github.com/you/dotfiles.git ~/dotfiles
cd ~/dotfiles && mate init

# 4. Fetch secrets, check, then apply
mate profile
mate secrets fetch          # skip if you don't use Bitwarden references
mate status
mate apply
```

`mate status` only reads the secrets cache, so fetching first matters when a
source's `.mate.yaml` uses `bitwarden`, which would otherwise fail to render.
`mate apply` fetches missing secrets itself.

Step 2 assumes the key is stored as a custom field `age-key` on a Bitwarden
item called `statemate`, as suggested in [Encryption](encryption.md). Adapt it
to wherever you keep yours.

### Check the profile first

`mate profile` should name the profile you expect for this machine. If none
matches, `#profile:` files are not filtered and every variant is deployed (see
[Different Machines](machines.md)). Fix the detection rules, or pin the profile
for this machine in the local config:

```yaml
# ~/.config/statemate/mate.yaml (written by mate init)
source_dir: "~/dotfiles"
profile: work
```

### What the first apply does

The first `mate apply` on a new machine is the biggest it will ever be:

1. **Secrets** referenced by templates are fetched from Bitwarden. You are asked
   for your master password if the vault is locked.
2. **`#before` scripts** marked `#once` run, each confirmed first.
3. **Files** are written. Any target that already exists with different content,
   such as the default `~/.zshrc` the OS ships, is a conflict, because there is
   no record of which side is newer. Answer `[d]` to compare and `[o]` to
   overwrite.
4. **Packages** missing on this machine are offered for install, one
   confirmation per package manager.
5. **Hooks** triggered by the written files run.
6. **`#after` scripts** marked `#once` run, such as changing the login shell or
   installing editor plugins.

`mate apply --dry-run` shows all of it beforehand, and `mate scripts list` shows
the scripts that are still pending.

Every `#once` script is recorded in this machine's state database, so it is not
offered again. Answering `[s]kip` marks one as done without running it.

## A bootstrap script

To make a new machine a single command, put a script at the root of the
repository and fetch it with `curl`:

```bash
curl -fsSL https://raw.githubusercontent.com/you/dotfiles/main/install.sh | bash
```

A useful bootstrap script is interactive and safe to re-run. It asks before each
step and skips what is already done. The steps:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Prerequisites: a package manager and the Bitwarden CLI
command -v brew >/dev/null || /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
command -v bw   >/dev/null || brew install bitwarden-cli

# statemate itself
command -v mate >/dev/null || brew install subbeh/tap/statemate

# The age key, and an SSH key to clone with, from the vault
export BW_SESSION=$(bw unlock --raw)
bw sync
mkdir -p ~/.config/statemate
bw get item statemate | jq -r '.fields[] | select(.name=="age-key").value' \
  > ~/.config/statemate/key.txt
chmod 600 ~/.config/statemate/key.txt

# Clone, register, apply
[ -d ~/dotfiles ] || git clone git@github.com:you/dotfiles.git ~/dotfiles
cd ~/dotfiles
mate init
mate apply
```

`BW_SESSION` is exported, so the `mate apply` at the end reuses the unlocked
vault for its secrets instead of asking again.

A complete example, with prompts for each step, branch selection, and building
statemate from source, is
[`install.sh`](https://github.com/Subbeh/dotfiles/blob/main/install.sh) in
statemate's author's dotfiles.

### `--force` in scripts

`mate apply --force` answers every prompt with yes: it **overwrites every
conflicting file without a backup**, runs every pending script and hook, and
installs every missing package. That suits a machine with nothing on it worth
keeping. On a machine you have used, run a plain `mate apply` and answer the
prompts.

Without a terminal, for example in CI or under `ssh host 'mate apply'`, scripts,
hooks and package installs are skipped with a warning, and conflicts stop the
apply. Pass `--force` or `--no-scripts` to decide explicitly.
