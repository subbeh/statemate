# mate check

Check if configuration is in sync

## Synopsis

Exit 0 if every managed file is in sync, and 1 if any would change.

Only files count: pending changes and conflicts. Orphans, missing packages,
pending scripts and secrets do not. Hooks are validated as well, so a broken
hook fails the check, and so does any other error, such as a config that does
not load. Use -q to print nothing and rely on the exit code.

```
mate check [flags]
```

## Examples

```
  mate check -q || echo "dotfiles out of sync"
```

## Options

```
  -h, --help    help for check
  -q, --quiet   suppress output, only use exit code
      --sudo    use sudo to check files requiring elevated access
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

