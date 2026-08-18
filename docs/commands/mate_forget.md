## mate forget

Remove files from tracking

### Synopsis

Remove files from statemate's tracking database.

The files at target remain untouched. Only the tracking entries are removed.
This is useful when you want statemate to stop managing files without
deleting them.

Supports wildcards (glob patterns) to forget multiple files at once.

Example:
  mate forget ~/.config/nvim/init.lua
  mate forget ~/.config/nvim/*.lua
  mate forget ~/.config/app/file1.conf ~/.config/app/file2.conf

```
mate forget <path>... [flags]
```

### Options

```
  -h, --help   help for forget
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

