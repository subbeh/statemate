# mate secrets fetch

Fetch secrets from providers

## Synopsis

Find every secret reference in templates, fetch the values from Bitwarden, and
store them in the encrypted cache.

Every reference is fetched again, not only missing ones. With a pattern, only
the item with exactly that name is fetched, or every item whose name starts
with a prefix ending in '*'.

Needs the bw CLI, logged in, and an age identity to encrypt the cache with. A
locked vault is unlocked for you.

```
mate secrets fetch [pattern] [flags]
```

## Examples

```
  mate secrets fetch
  mate secrets fetch 'github*'
```

## Options

```
  -h, --help   help for fetch
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

