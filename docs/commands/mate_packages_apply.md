# mate packages apply

Install missing packages

## Synopsis

Install packages that are declared but not installed.

Missing packages are listed per package manager, and each manager's install is
confirmed separately with [y/N]. Use -y/--yes to confirm all of them.

Use --prune to also remove installed packages that are not in config. Removals
are confirmed the same way, and -y/--yes confirms them too.

```
mate packages apply [flags]
```

## Options

```
  -h, --help    help for apply
      --prune   remove packages not in config (dangerous)
  -y, --yes     auto-confirm all changes
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate packages](mate_packages.md)	 - Manage packages

