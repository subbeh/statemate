## mate decrypt

Decrypt a managed file

### Synopsis

Decrypt a file in place.

This reads the encrypted file, decrypts it using the configured age identity,
writes it back, and removes the #encrypted suffix from the filename.

The file can be a managed source file or any file path (e.g. a var_file
in .matedata/). Paths are resolved relative to the current directory,
falling back to the source directory. The #encrypted suffix is optional.

The age identity must be configured in mate.yaml:

  age:
    identity: "~/.config/statemate/key.txt"

Examples:
  mate decrypt nvim/secrets.yaml#encrypted
  mate decrypt .matedata/secrets.yaml

```
mate decrypt <source> [flags]
```

### Options

```
  -h, --help   help for decrypt
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

