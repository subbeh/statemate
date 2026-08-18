## mate delete

Delete file from source and target

### Synopsis

Delete a file from both source and target.

This deletes the source file and optionally the target file,
and removes the tracking entry from the database.

Example:
  mate delete ~/.config/nvim/init.lua

```
mate delete <path> [flags]
```

### Options

```
  -f, --force         don't prompt for confirmation
  -h, --help          help for delete
      --keep-target   keep the target file, only delete source
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

