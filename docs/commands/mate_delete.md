# mate delete

Delete file from source and target

## Synopsis

Delete a managed file from the source and the target, and stop tracking it.

Give the target path, meaning the deployed file. The target is deleted as well
unless --keep-target is given, and the deletion is confirmed unless --force is.

Hooks matching the removed target run afterwards (see 'mate hooks'); --force
also confirms them. Nothing runs with --keep-target.

```
mate delete <path> [flags]
```

## Examples

```
  mate delete ~/.config/nvim/init.lua
  mate delete --keep-target ~/.zshrc
```

## Options

```
  -f, --force         don't prompt for confirmation
  -h, --help          help for delete
      --keep-target   keep the target file, only delete source
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

