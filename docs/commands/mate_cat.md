# mate cat

Display file contents

## Synopsis

Display file contents, decrypting if necessary.

Works like cat but automatically decrypts age-encrypted files.

Example:

```
mate cat ~/.statemate/files/secrets.age
mate cat ~/.config/app/config.yaml
```

```
mate cat <file> [flags]
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

