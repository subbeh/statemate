## mate config source-dir

Print the resolved source directory

### Synopsis

Print the absolute path of the directory containing mate.yaml.

The path is printed bare, with no label, so it can be used directly:

  cd "$(mate config source-dir)"

Resolution order matches the rest of mate: the --config flag, then the
STATEMATE_DIR environment variable, then source_dir in the local config
(~/.config/statemate/mate.yaml), then the current directory.

```
mate config source-dir [flags]
```

### Options

```
  -h, --help   help for source-dir
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate config](mate_config.md)	 - Inspect resolved configuration

