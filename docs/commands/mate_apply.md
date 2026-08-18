## mate apply

Apply configuration to target

### Synopsis

Apply files from source directories to their targets.

With no argument, applies everything. Otherwise the run is narrowed:

  mate apply <path>        apply matching files only -- no scripts, no
                           packages, no secret fetch
  mate apply -s <source>   apply that source's files, run its scripts, and
                           prompt for its packages

The positional argument is always a file or path filter; --source is the only
way to select a source. Repo-root scripts are not run under --source, since
they apply to the whole repository.

Scripts due to run are confirmed individually before executing:

  [y]es    run the script
  [n]o     skip this time; ask again on the next apply
  [s]kip   mark as done without running, so it is not offered again
           (not available for 'always' scripts, whose runs are never recorded)
  [a]ll    run this and auto-confirm the rest
  [q]uit   abort the apply

A script marked as done still appears in 'mate scripts list' and can be run
manually with 'mate scripts run'.

Use --force to auto-confirm all scripts, or --no-scripts to skip them entirely
(useful for automated runs). Without a terminal to prompt on, scripts are
skipped with a warning.

A file marked '#import' is not prompted about when only its target changed: the
target is treated as authoritative and copied back into the source. Use it for
files an application rewrites, such as ~/.claude/settings.json. If the source
changed too, the conflict prompt still appears.

```
mate apply [path] [flags]
```

### Options

```
      --dry-run         show what would be done without making changes
      --force           overwrite modified targets and auto-confirm scripts
  -h, --help            help for apply
      --no-scripts      skip all scripts
  -s, --source string   limit to a single source
  -V, --verbose count   increase verbosity (can be repeated)
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

