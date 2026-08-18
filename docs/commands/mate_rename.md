## mate rename

Rename a managed file

### Synopsis

Rename a managed file in both source and target.

This renames the source file, the target file, and updates tracking.

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

