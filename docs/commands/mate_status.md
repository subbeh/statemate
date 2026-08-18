## mate status

Show files that would change on apply

### Synopsis

Show pending changes that would be made on apply.

Reports files to be created, modified, or in conflict, plus orphaned files,
missing packages, pending scripts, and secrets needing refresh.

Markers: '+' new, '~' modified, '!' conflict, '<' will be imported into the
source (an '#import' file whose target changed).

The positional argument filters by file or path; use --source to limit the
report to a single source.

```
mate status [path] [flags]
```

### Options

```
  -h, --help            help for status
      --short           compact output for statuslines (format: +N ~N !N <N ?N *N sN)
  -s, --source string   limit to a single source
      --sudo            use sudo to check files requiring elevated access
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

