# mate managed

List all managed files

## Synopsis

List all files in source directories that are managed by mate.

Files from every source are listed, including those only some profiles use;
ACTIVE marks the ones the current profile deploys.

With no argument, lists every managed file. With an argument, filters the list:

- A path to an existing file (absolute, or relative to the current directory)
  matches only that file, whether you give its target or its source path.
- Anything else is treated as a name fragment, so 'mate managed nvim' lists
  every file in the nvim source.

```
mate managed [path] [flags]
```

## Examples

```
  mate managed                    # all managed files
  mate managed ~/.ssh/config      # just that file
  mate managed config             # that file if it exists here, else all matches
  mate managed nvim               # everything in the nvim source
```

## Options

```
  -h, --help   help for managed
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

* [mate](mate.md)	 - Statemate - system configuration management

