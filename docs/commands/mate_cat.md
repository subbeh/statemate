# mate cat

Print a file, decrypting it if needed

## Synopsis

Print a file, decrypting it first if it is age-encrypted.

Works like cat, but an age-encrypted file is decrypted with the configured
identity. The path is used as given: absolute, starting with ~, or relative to
the current directory.

```
mate cat <file> [flags]
```

## Examples

```
  mate cat ssh/.ssh/config#enc
  mate cat ~/.config/app/config.yaml
```

## Options

```
  -h, --help   help for cat
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

