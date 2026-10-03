## mate hooks run

Run a hook

### Synopsis

Run a hook manually, ignoring its patterns.

There is no confirmation prompt, and no files triggered the run, so
STATEMATE_HOOK_FILES and .Files are empty.

```
mate hooks run <name> [flags]
```

### Options

```
      --dry-run   show what would be done without running
  -h, --help      help for run
  -v, --verbose   verbose output
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate hooks](mate_hooks.md)	 - Manage hooks

