# mate secrets status

Show secrets that need fetching

## Synopsis

List the secret references the cache does not hold yet. 'mate apply' fetches these before deploying.

```
mate secrets status [flags]
```

## Options

```
  -h, --help   help for status
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

* [mate secrets](mate_secrets.md)	 - Manage secrets

