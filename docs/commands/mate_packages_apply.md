# mate packages apply

Install missing packages

## Synopsis

Install packages that are declared but not installed.

Missing packages are listed per package manager, and each manager's install is
confirmed separately with [y/N]. Use -y/--yes (or --noconfirm) to confirm all of them.

Use --prune to also remove installed packages that are not in config. Removals
are confirmed the same way, and -y/--yes confirms them too.

```
mate packages apply [flags]
```

## Options

```
  -h, --help        help for apply
      --noconfirm   same as --yes
      --prune       remove packages not in config (dangerous)
  -y, --yes         auto-confirm all changes
```

## Options inherited from parent commands

```
  -c, --config string      config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
      --log-file string    append a log of what mate decides and runs to this file (also STATEMATE_LOG_FILE, or log_file in the config)
      --log-level string   log level: debug, info, warn or error (default info; also STATEMATE_LOG_LEVEL, or log_level in the config)
      --no-color           disable colored output (also off when stdout is not a terminal or NO_COLOR is set)
  -p, --profile string     override auto-detected profile
```

## SEE ALSO

* [mate packages](mate_packages.md)	 - Manage packages

