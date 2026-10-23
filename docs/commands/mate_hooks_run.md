# mate hooks run

Run a hook

## Synopsis

Run a hook manually, ignoring its patterns.

There is no confirmation prompt, and no files triggered the run, so
STATEMATE_HOOK_FILES and .Files are empty. A successful run settles a hook that
mate apply left pending.

```
mate hooks run <name> [flags]
```

## Options

```
      --dry-run   show what would be done without running
  -h, --help      help for run
```

## Options inherited from parent commands

```
  -c, --config string      config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
      --log-file string    append a log of what mate changes to this file, and at debug what it decides and runs (also STATEMATE_LOG_FILE, or log_file in the config)
      --log-level string   log level: debug, info, warn or error (default info; also STATEMATE_LOG_LEVEL, or log_level in the config)
      --no-color           disable colored output (also off when stdout is not a terminal or NO_COLOR is set)
  -p, --profile string     override auto-detected profile
  -v, --verbose count      print log records to stderr: -v for info (what changed), -vv for debug
```

## SEE ALSO

* [mate hooks](mate_hooks.md)	 - Manage hooks

