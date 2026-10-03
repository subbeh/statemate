# mate clean

Remove orphaned files

## Synopsis

Remove orphaned files: targets mate deployed earlier that no longer come
from any active source, usually because the file was deleted from the
repository or its source was dropped from 'sources:'.

With no arguments, the orphans are listed and nothing is removed. Name orphans
to remove them, or pass --all for every one. Each removal is confirmed unless
--force is given. Files you cannot write to are removed with sudo.

Hooks matching the removed files run afterwards (see 'mate hooks'); --force
also confirms them.

To stop tracking a file without deleting it, use 'mate forget'.

```
mate clean [path...] [flags]
```

## Examples

```
  mate clean                              # list orphans
  mate clean ~/.config/old/file.conf      # remove one orphan
  mate clean --all                        # remove every orphan, asking for each
  mate clean --all --force                # remove every orphan without asking
```

## Options

```
      --all     remove all orphans
      --force   skip confirmation prompts
  -h, --help    help for clean
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

