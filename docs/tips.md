# Tips & Tricks

Integrations that make statemate disappear into your daily workflow. Most of
these come from a [real dotfiles repository](https://github.com/Subbeh/dotfiles)
managed with statemate; follow the links for the complete files.

## Edit the source, not the target

Every deployed file is overwritten by the next `mate apply`, so an edit made to
`~/.zshrc` directly is either lost or turns into a conflict. `mate edit` takes the
*target* path and opens the *source* for you:

```bash
mate edit ~/.zshrc                 # opens zsh/.zshrc in the repository
mate edit ~/.ssh/config            # decrypts, opens, re-encrypts on save
```

If you edited the target anyway, `mate apply` stops at the conflict and offers
`[i]mport`, which copies your edit back into the source.

## Shell

### Completion

`mate completion` prints a completion script for bash, zsh, fish, or PowerShell:

```bash
# ~/.zshrc
source <(mate completion zsh)

# ~/.bashrc
source <(mate completion bash)

# fish
mate completion fish | source
```

Completion is context-aware: `mate apply`, `diff` and `status` complete
managed files, `mate clean` completes orphans, `mate scripts run` and
`mate hooks run` complete script and hook names, and `--source` / `--profile`
complete from your configuration.

### Aliases

Short aliases for the commands you run most:

```bash
alias ds='mate status'
alias da='mate apply'
alias dad='mate apply --dry-run --verbose'
alias de='mate edit'
alias ddiff='mate diff'
alias ddoc='mate doctor'
```

And a quick way into the repository from anywhere:

```bash
alias cdm='cd "$(mate config source-dir)"'
```

### Reload every open shell after a change

A [hook](hooks.md) can signal running shells when their configuration changes, so
open terminals pick up a new alias without being restarted:

```yaml
# mate.yaml
hooks:
  shell-reload:
    match:
      - ".config/zsh/**"
      - ".zshenv"
    do:
      - run: pkill -USR2 -u "$USER" zsh || true
```

```zsh
# .zshrc — re-source the configuration when SIGUSR2 arrives
TRAPUSR2() { source "${ZDOTDIR:-$HOME}/.zshrc" }
```

The `|| true` keeps the hook from failing when no zsh is running.

## Neovim

Three problems come up when editing a statemate repository in Neovim, and an
autocommand plugin solves each of them. The complete version is
[`plugin/statemate.lua`](https://github.com/Subbeh/dotfiles/blob/main/nvim/.config/nvim/plugin/statemate.lua).

### Filetype detection for source files

Neovim reads `settings.json#profile:work#encrypted` as a file with the extension
`json#profile:work#encrypted`, so highlighting and LSP stop working. Strip the
attributes and detect again:

```lua
vim.api.nvim_create_autocmd({ "BufReadPost", "BufNewFile" }, {
  pattern = "*#*",
  callback = function(args)
    local name = vim.fs.basename(args.file):gsub("#.*$", "")
    local ft = vim.filetype.match({ filename = name, buf = args.buf })
    if ft then
      vim.bo[args.buf].filetype = ft
    end
  end,
})
```

Only the basename is stripped, so attributes on a parent directory
(`etc#owner-r:root/`) are left alone.

### Offer the source when a deployed file is opened

Opening `~/.ssh/config` is almost always a mistake when the file is managed.
`mate managed <path>` resolves a target to its source, so a `BufReadPost`
autocommand can ask which one you meant:

```
statemate: managed file, edit which?
1: source: config#encrypted
2: target: ~/.ssh/config
```

The plugin runs `mate managed` on the opened path and switches the buffer to the
source if you pick it. It skips files whose `ACTIVE` column is empty, since their
source belongs to a profile that is not in use.

### Edit `#encrypted` files in place

`BufReadCmd` and `BufWriteCmd` autocommands on `*#encrypted*` can decrypt with
`age --decrypt --identity <key>` on read and re-encrypt with
`age --armor --recipient <key>…` on write, reading the recipients from
`mate.yaml`. Disable the swap file, undo file, and backups for those buffers so
the plaintext never reaches the disk.

Use `--armor`: statemate writes armored ciphertext, and matching it keeps
`git diff` readable whichever tool last wrote the file.

## Git

### Readable diffs for encrypted files

Encrypted sources are ciphertext, so `git diff` and `git log -p` show nothing
useful. A textconv filter decrypts them for display only; the files stay
encrypted on disk and in the repository:

```gitattributes
# .gitattributes in the repository
*#encrypted* diff=age
```

```ini
# ~/.gitconfig
[diff "age"]
    textconv = age --decrypt --identity ~/.config/statemate/key.txt
```

The pattern ends in `*` because `#encrypted` need not be the last attribute
(`settings.json#encrypted#import`). Tools that use git's diff machinery, such as
lazygit and tig, pick this up too.

## Diffs with an external tool

`diff_tool` in `mate.yaml` replaces the built-in `diff -u`:

```yaml
diff_tool: difft
```

statemate runs the tool as `<tool> <old> <new>` through `sh`, so the setting may
include options (`diff_tool: delta --side-by-side`). The output is captured
rather than shown on the terminal, so the tool cannot detect the terminal's
width. A wrapper that reads the width from the terminal itself:

```sh
#!/bin/sh
# ~/.local/bin/mate-diff
cols=$( (stty size < /dev/tty) 2>/dev/null | cut -d' ' -f2 )
exec delta --side-by-side --width="${cols:-160}" "$1" "$2"
```

```yaml
diff_tool: mate-diff
```

`mate diff --tool <name>` overrides the setting for one run.

## Status in your status bar

`mate status --short` prints one compact line meant for status bars, and nothing
at all when everything is up to date:

```
~2 !1 *1
```

| Token | Meaning |
|-------|---------|
| `+N` | new files |
| `~N` | modified files |
| `!N` | conflicts |
| `<N` | pending imports |
| `?N` | orphans |
| `*N` | pending scripts |
| `sN` | secrets needing a fetch |

It exits non-zero when it cannot run at all, such as when the configuration does
not load, so a status bar can tell "clean" from "broken". Every run scans your
sources, so poll it every few seconds rather than continuously.

### tmux

A status-line segment, colouring each token:

```bash
#!/usr/bin/env bash
# ~/.local/bin/tmux-statemate
status=$(mate status --short 2>/dev/null) || exit 0
[[ -z "$status" ]] && exit 0
for token in $status; do
  case "$token" in
    !*) printf '#[fg=red]%s ' "$token" ;;
    ~*) printf '#[fg=blue]%s ' "$token" ;;
    *)  printf '#[fg=default]%s ' "$token" ;;
  esac
done
```

```tmux
set -g status-interval 10
set -ag status-right '#(tmux-statemate)'
```

And a key binding that runs `mate apply` in a popup, after confirming:

```tmux
bind . confirm-before -p "Run mate apply?" \
  "display-popup -E 'mate apply --verbose; read -n 1 -s -r -p \"Press any key\"'"
```

The full versions are
[`__statemate_status`](https://github.com/Subbeh/dotfiles/blob/main/statemate/.local/bin%23perm-r:755/__statemate_status)
and
[`50-statemate.conf`](https://github.com/Subbeh/dotfiles/blob/main/statemate/.config/tmux/config.d/50-statemate.conf).

### Waybar

The same script can emit JSON for a Waybar custom module, with the full
`mate status` as the tooltip:

```json
"custom/statemate": {
  "exec": "__statemate_status waybar",
  "return-type": "json",
  "interval": 10,
  "on-click": "mate apply"
}
```

```bash
# the waybar branch of the script
if [[ -z "$status" ]]; then
  jq -cn '{text: "", class: "clean", tooltip: "statemate: up to date"}'
else
  jq -cn --arg t "$status" --arg tip "$(mate status 2>/dev/null)" \
    '{text: $t, class: "dirty", tooltip: $tip}'
fi
```

## One palette, every tool

Variables can be nested, and an [`include`](configuration.md#include) file can hold
nothing but variables. Together they make a single theme file that feeds every
application's colours:

```yaml
# .mate/theme.yaml
variables:
  color:
    bg:
      default: 1a1e24
      dark: 15181e
    fg:
      default: d7d7d7
```

```yaml
# mate.yaml
include:
  - .mate/theme.yaml
```

```gotemplate
# kitty/.config/kitty/theme.conf#template
background #{{ .Vars.color.bg.default }}
foreground #{{ .Vars.color.fg.default }}
```

Changing a colour in one place redeploys every template that uses it on the next
`mate apply`. Renaming or removing a key breaks every template that refers to it.
Run `grep -r 'color.bg.dark'` first, and `mate eval` the templates you touched.
