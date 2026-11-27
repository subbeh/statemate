# mate audit

List the changes of recent runs

## Synopsis

List what recent mate runs changed, newest first.

Every run that changes something records its changes in the state database:
each target file written, moved or removed, each change to the source tree,
each forget, script, hook and package install, each secrets update that changed
a secret, and the error the run ended with. These are the info-level records
of the log file, kept whether or not a log file is set. A --dry-run, and a run
that only reads, records nothing. The newest 100 runs are kept.

```
mate audit [flags]
```

## Examples

```
  mate audit
  mate audit -n 3
```

## Options

```
  -h, --help       help for audit
  -n, --runs int   number of runs to list (default 10)
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

* [mate](mate.md)	 - Statemate - system configuration management

