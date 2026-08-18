## mate encrypt

Encrypt a managed file

### Synopsis

Encrypt a file in place.

This reads the file, encrypts it using the configured age recipients,
writes it back, and adds the #encrypted suffix to the filename.

The file can be a managed source file or any file path (e.g. a var_file
in .matedata/). Paths are resolved relative to the current directory,
falling back to the source directory.

The age recipients must be configured in mate.yaml:

  age:
    recipients:
      - age1...

Examples:
  mate encrypt nvim/secrets.yaml
  mate encrypt .matedata/secrets.yaml

```
mate encrypt <source> [flags]
```

### Options

```
  -h, --help   help for encrypt
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

