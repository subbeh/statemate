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
  mate cat ssh/.ssh/config#encrypted
  mate cat ~/.config/app/config.yaml
```

## Options

```
  -h, --help   help for cat
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

