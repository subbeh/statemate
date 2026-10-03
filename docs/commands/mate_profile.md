# mate profile

Show active profile

## Synopsis

Show the active profile, how it was chosen, and the sources it resolves to.

The profile is chosen by the first of: --profile, 'profile:' in the local
config or mate.yaml, $STATEMATE_PROFILE, and automatic detection.

```
mate profile [flags]
```

## Options

```
  -h, --help   help for profile
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

