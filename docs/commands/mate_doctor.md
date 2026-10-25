# mate doctor

Check configuration and dependencies

## Synopsis

Check that the configuration loads and validates, that every source directory
exists, that every active hook matches some managed file, that every #enc
file in any source really is age-encrypted, that the age identity and
recipients work, and which package managers are available.

Exits 1 if any check reports an error. Run it first when something behaves
unexpectedly.

```
mate doctor [flags]
```

## Options

```
  -h, --help   help for doctor
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

