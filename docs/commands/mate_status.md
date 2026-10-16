# mate status

Show what apply would change

## Synopsis

Show pending changes that would be made on apply.

Reports files to be created, modified, or in conflict, plus orphaned files,
missing packages, pending scripts, the hooks the changes would trigger or that
an earlier apply left pending, and secrets that need fetching.

An empty directory in a source -- one with no files under it -- is reported until
it exists on the target. Directories that hold files are not listed separately;
they arrive with those files.

Markers: '+' new, '~' modified, '!' conflict, '&lt;' will be imported into the
source (an '#import' file whose target changed).

The positional argument filters by file or path; use --source to limit the
file report to a single source. Packages, scripts and secrets are always
reported in full.

--short prints one compact line for status bars, and nothing when there is
nothing to do. It covers files, orphans, scripts and secrets, not packages or
hooks.

```
mate status [path] [flags]
```

## Options

```
  -h, --help            help for status
      --short           compact output for statuslines (format: +N ~N !N <N ?N *N sN)
  -s, --source string   limit to a single source
      --sudo            use sudo to check files requiring elevated access
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

