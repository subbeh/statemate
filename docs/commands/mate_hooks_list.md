## mate hooks list

List all hooks

### Synopsis

List every hook with where it is declared, its patterns, and its status.

SCOPE is 'repo' for mate.yaml, 'local' for the machine-local config, or the
source name. STATUS shows 'disabled' for a hook switched off with
'enabled: false', and 'n/a' when its profile is not active.

```
mate hooks list [flags]
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate hooks](mate_hooks.md)	 - Manage hooks

