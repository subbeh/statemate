# mate hooks

Manage hooks

## Synopsis

List and run hooks.

A hook runs commands or scripts after files matching its patterns are written
by 'mate apply' or removed by 'mate clean', 'mate delete', or 'mate rename':

```
hooks:
  systemd-reload:
    match: ["*.service", "*.timer"]
    do:
      - run: sudo systemctl daemon-reload
```

Each triggered hook runs once per command, however many files matched, after
packages and before #after scripts. Hooks are confirmed like scripts; --force
auto-confirms them and --no-scripts skips them.

Hooks declared in a source's .mate.yaml are named &lt;source>/&lt;name> and match
only that source's files. Patterns match target paths; source attributes such
as #tmpl and #perm:600 are ignored when included in a pattern.

## Options

```
  -h, --help   help for hooks
```

## Options inherited from parent commands

```
  -c, --config string      config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
      --log-file string    append a log of what mate changes to this file, and at debug what it decides and runs (also STATEMATE_LOG_FILE, or log_file in the config)
      --log-level string   log level: debug, info, warn or error (default info; also STATEMATE_LOG_LEVEL, or log_level in the config)
      --no-color           disable colored output (also off when stdout is not a terminal or NO_COLOR is set)
  -p, --profile string     override auto-detected profile
  -v, --verbose count      print log records to stderr: -v for info (what changed), -vv for debug
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management
* [mate hooks list](mate_hooks_list.md)	 - List all hooks
* [mate hooks run](mate_hooks_run.md)	 - Run a hook

