## mate add

Add a file to source

### Synopsis

Add an existing file to the source directory.

The file is copied from its current location to the appropriate source directory,
following stow-style conventions. The original file remains in place.

Examples:
  mate add ~/.config/nvim/init.lua
  mate add --profile work ~/.gitconfig
  mate add --encrypt ~/.ssh/config

```
mate add <path> [flags]
```

### Options

```
      --encrypt          encrypt file when adding
  -h, --help             help for add
      --profile string   add file with profile suffix
  -s, --source string    target source directory
      --template         mark file as template
```

### Options inherited from parent commands

```
  -c, --config string   config file (default: mate.yaml in current directory)
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

