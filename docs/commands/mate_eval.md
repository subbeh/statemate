## mate eval

Render a template file

### Synopsis

Render a template file and output the result to stdout.

Useful for debugging templates or previewing output before applying.
If the file is encrypted, it will be decrypted first (requires age identity).

Example:
  mate eval ~/.statemate/files/config.tmpl
  mate eval --profile work ~/.statemate/files/config.tmpl

```
mate eval <file> [flags]
```

### Options

```
  -h, --help   help for eval
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

