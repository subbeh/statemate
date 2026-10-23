# mate config source-dir

Print the resolved source directory

## Synopsis

Print the absolute path of the directory containing mate.yaml.

The path is printed bare, with no label, so it can be used directly:

```
cd "$(mate config source-dir)"
```

Resolution order matches the rest of mate: the --config flag, then the
STATEMATE_DIR environment variable, then source_dir in the local config
(~/.config/statemate/config.yaml), then the current directory.

```
mate config source-dir [flags]
```

## Options

```
  -h, --help   help for source-dir
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

* [mate config](mate_config.md)	 - Inspect resolved configuration

