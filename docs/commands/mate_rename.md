# mate rename

Rename a managed file

## Synopsis

Rename a managed file in both source and target.

This renames the source file, the target file, and updates tracking.

Attributes belong to the source file only; the target is always named
without them. If &lt;new-name> has no attributes, the source keeps the ones it
already has. If it has any, they replace the source's attributes, so give the
full set. #enc cannot be added or dropped this way, since that would
not change the content: use 'mate encrypt' or 'mate decrypt'.

Hooks matching the old or the new target path run afterwards, with a
confirmation prompt (see 'mate hooks').

```
mate rename <source> <new-name> [flags]
```

## Examples

```
  mate rename nvim/init.lua init.vim
  mate rename zsh/.zshrc .zshrc.bak
  mate rename git/.gitconfig#tmpl .gitconfig.local
      (source becomes .gitconfig.local#tmpl, target .gitconfig.local)
  mate rename ssh/.ssh/config config#perm:600
      (changes only the attributes; the target keeps its name)
```

## Options

```
  -h, --help   help for rename
```

## Options inherited from parent commands

```
  -c, --config string      config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
      --log-file string    append a log of what mate decides and runs to this file (also STATEMATE_LOG_FILE, or log_file in the config)
      --log-level string   log level: debug, info, warn or error (default info; also STATEMATE_LOG_LEVEL, or log_level in the config)
      --no-color           disable colored output (also off when stdout is not a terminal or NO_COLOR is set)
  -p, --profile string     override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

