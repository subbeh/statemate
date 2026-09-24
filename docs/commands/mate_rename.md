## mate rename

Rename a managed file

### Synopsis

Rename a managed file in both source and target.

This renames the source file, the target file, and updates tracking.

Hooks matching the old or the new target path run afterwards, with a
confirmation prompt (see 'mate hooks').

Examples:
  mate rename nvim/init.lua init.vim
  mate rename zsh/.zshrc .zshrc.bak

```
mate rename <source> <new-name> [flags]
```

### Options

```
  -h, --help   help for rename
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

