## mate hooks

Manage hooks

### Synopsis

List and run hooks.

A hook runs commands or scripts after files matching its patterns are written
by 'mate apply' or removed by 'mate clean', 'mate delete', or 'mate rename':

  hooks:
    systemd-reload:
      match: ["*.service", "*.timer"]
      do:
        - run: sudo systemctl daemon-reload

Each triggered hook runs once per command, however many files matched, after
packages and before #after scripts. Hooks are confirmed like scripts; --force
auto-confirms them and --no-scripts skips them.

Hooks declared in a source's .mate.yaml are named <source>/<name> and match
only that source's files.

### Options

```
  -h, --help   help for hooks
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management
* [mate hooks list](mate_hooks_list.md)	 - List all hooks
* [mate hooks run](mate_hooks_run.md)	 - Run a hook

