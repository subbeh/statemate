# mate encrypt

Encrypt a managed file

## Synopsis

Encrypt a file in place.

This reads the file, encrypts it using the configured age recipients,
writes it back, and adds the #enc suffix to the filename.

The file can be a managed file, named by its source or its target, or any
other file (e.g. a var_file in .matedata/). Paths are resolved relative to the
current directory, falling back to the source directory. A bare file name is
not searched for.

The age recipients must be configured in mate.yaml:

```
age:
  recipients:
    - age1...
```

```
mate encrypt <source> [flags]
```

## Examples

```
  mate encrypt nvim/secrets.yaml
  mate encrypt .matedata/secrets.yaml
```

## Options

```
  -h, --help   help for encrypt
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

