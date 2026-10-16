# mate secrets

Manage secrets

## Synopsis

Fetch and inspect secrets referenced in templates

## Options

```
  -h, --help   help for secrets
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
* [mate secrets fetch](mate_secrets_fetch.md)	 - Fetch secrets from providers
* [mate secrets list](mate_secrets_list.md)	 - List secrets referenced in templates and cache status
* [mate secrets status](mate_secrets_status.md)	 - Show secrets that need fetching

