# mate eval

Render a template file

## Synopsis

Render a file as a template and print the result.

Useful for debugging a template, or previewing what apply would deploy. Any
file is rendered, whether or not it is marked #tmpl. An encrypted file is
decrypted first, which needs the age identity.

The path is used as given: absolute, starting with ~, or relative to the
current directory. Use --profile to render it as another profile would.

```
mate eval <file> [flags]
```

## Examples

```
  mate eval git/.config/git/config#tmpl
  mate eval --profile work git/.config/git/config#tmpl
```

## Options

```
  -h, --help   help for eval
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

