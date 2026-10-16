# mate scripts run

Run a script

## Synopsis

Run a script now, by the name 'mate scripts list' shows or by path, whatever
its frequency, timing or #profile:.

There is no confirmation prompt. The run is recorded, so running a #once
script this way marks it done.

```
mate scripts run <script> [flags]
```

## Options

```
      --dry-run   show what would be done without running
  -h, --help      help for run
  -v, --verbose   verbose output
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

* [mate scripts](mate_scripts.md)	 - Manage scripts

