## mate scripts

Manage scripts

### Synopsis

List and manage lifecycle scripts.

A script can describe itself with a comment in its first 10 lines:

  #!/usr/bin/env bash
  # Description: Bootstrap the development environment

The description is shown by 'scripts list', 'mate status', and the
confirmation prompt during apply. Matching is case-insensitive.

An '#onchange' script runs when its own source has pending changes -- the files
'mate status' lists for the source the script lives in. A script in the
repo-root .matescripts directory has no owning source, so any pending change
triggers it. Editing the script itself does not trigger it; use
'mate scripts run <name>' to run one on demand.

### Options

```
  -h, --help   help for scripts
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management
* [mate scripts list](mate_scripts_list.md)	 - List all scripts
* [mate scripts run](mate_scripts_run.md)	 - Run a script

