# mate scripts run

Run a script, or every script with a tag

## Synopsis

Run a script now, by the name 'mate scripts list' shows or by path, whatever
its frequency, timing or #profile:.

With --tags, run every script whose "# Tags:" comment names any of the tags:
before scripts then after scripts, in order, whatever their frequency. Scripts
whose #profile: does not match the active profile are skipped.

There is no confirmation prompt. The run is recorded, so running a #once
script this way marks it done.

```
mate scripts run <script> | --tags <tag>[,<tag>...] [flags]
```

## Examples

```
  mate scripts run setup.sh
  mate scripts run --tags update
  mate scripts run --tags update,brew
```

## Options

```
      --dry-run        show what would be done without running
  -h, --help           help for run
      --tags strings   run every script whose # Tags: comment names any of these tags
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

* [mate scripts](mate_scripts.md)	 - Manage scripts

