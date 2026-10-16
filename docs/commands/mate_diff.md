# mate diff

Show pending changes

## Synopsis

Show full unified diff of pending changes.

The positional argument filters by file or path; use --source to limit the diff
to a single source.

Use --tool to specify an external diff tool (e.g., delta, difft, vimdiff), or
set 'diff_tool' in config. The tool is run as '&lt;tool> &lt;old> &lt;new>' and its
output is captured, so it may include arguments but cannot be interactive.

```
mate diff [path] [flags]
```

## Options

```
  -h, --help            help for diff
  -s, --source string   limit to a single source
      --sudo            use sudo to check files requiring elevated access
  -t, --tool string     external diff tool to use
```

## Options inherited from parent commands

```
  -c, --config string      config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
      --log-file string    append a log of what mate decides and runs to this file (also STATEMATE_LOG_FILE, or log_file in the config)
      --log-level string   log level: debug, info, warn or error (default info; also STATEMATE_LOG_LEVEL, or log_level in the config)
      --no-color           disable colored output (also off when stdout is not a terminal or NO_COLOR is set)
  -p, --profile string     override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

