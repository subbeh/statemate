## mate diff

Show pending changes

### Synopsis

Show full unified diff of pending changes.

The positional argument filters by file or path; use --source to limit the diff
to a single source.

Use --tool to specify an external diff tool (e.g., delta, difft, vimdiff).
This can also be set in config with 'diff_tool'.

```
mate diff [path] [flags]
```

### Options

```
  -h, --help            help for diff
  -s, --source string   limit to a single source
      --sudo            use sudo to check files requiring elevated access
  -t, --tool string     external diff tool to use
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

