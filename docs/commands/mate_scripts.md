# mate scripts

Manage scripts

## Synopsis

List and manage lifecycle scripts.

A script can describe itself with a comment in its first 10 lines:

```
#!/usr/bin/env bash
# Description: Bootstrap the development environment
```

The description is shown by 'mate scripts list', 'mate status', and the
confirmation prompt during apply. Matching is case-insensitive.

An '#onchange' script runs when its own source has pending changes -- the files
'mate status' lists for the source the script lives in. A script in the
repo-root .matescripts directory has no owning source, so any pending change
triggers it. Editing the script itself does not trigger it; use
'mate scripts run &lt;name>' to run one on demand.

## Options

```
  -h, --help   help for scripts
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
* [mate scripts list](mate_scripts_list.md)	 - List all scripts
* [mate scripts run](mate_scripts_run.md)	 - Run a script, or every script with a tag

