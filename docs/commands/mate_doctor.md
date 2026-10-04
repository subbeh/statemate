# mate doctor

Check configuration and dependencies

## Synopsis

Check that the configuration loads and validates, that every source directory
exists, that every active hook matches some managed file, that the age identity
and recipients work, and which package managers are available.

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
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

