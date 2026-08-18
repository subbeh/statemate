## mate clean

Remove orphaned files

### Synopsis

Remove orphaned files that are no longer in the source.

Orphans are files that were previously managed but are no longer defined
in any source directory. By default, this command prompts for confirmation
before each deletion.

Flags:
  --force   Skip confirmation prompts
  --all     Remove all orphans (otherwise specify paths)

Example:
  mate clean                              # list orphans
  mate clean ~/.config/old/file.conf      # remove specific orphan
  mate clean --all                        # remove all orphans (with prompts)
  mate clean --all --force                # remove all orphans (no prompts)

```
mate clean [path...] [flags]
```

### Options

```
      --all     remove all orphans
      --force   skip confirmation prompts
  -h, --help    help for clean
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

