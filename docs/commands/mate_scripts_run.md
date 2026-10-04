# mate scripts run

Run a script

## Synopsis

Run a script now, by the name 'mate scripts list' shows or by path, whatever
its frequency, timing or #profile:.

There is no confirmation prompt. The run is recorded, so running a #once
script this way marks it done.

```
mate scripts run <script> [flags]
```

## Options

```
      --dry-run   show what would be done without running
  -h, --help      help for run
  -v, --verbose   verbose output
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate scripts](mate_scripts.md)	 - Manage scripts

