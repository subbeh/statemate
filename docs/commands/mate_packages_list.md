# mate packages list

List packages and whether they are installed

## Synopsis

List declared packages across configured package managers, and whether each
is installed. Each manager's packages are sorted by name.

Packages can be defined in:

- mate.yaml packages (global packages)
- mate.yaml profiles.&lt;name>.packages, for the active profile and every
  profile it extends
- &lt;source>/.mate.yaml packages, for each active source
- Files listed under 'include', either top-level (global) or in a
  profile's 'include' (part of that profile)

The SOURCE column shows where each package is declared: config, profile:&lt;name>,
or the source directory.

Use --all to show extra packages not in config. Detecting extras means listing
every installed package, which is noticeably slower, so it is only done when
--all is given.
Use --descriptions to show package descriptions. A description of &lt;unknown> means the
package manager does not recognise the name at all, usually a typo or a package
that only exists on another platform; an empty description means the package
exists but publishes none.

```
mate packages list [flags]
```

## Options

```
      --all            also show extra packages not in config
  -d, --descriptions   show package descriptions
  -h, --help           help for list
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

* [mate packages](mate_packages.md)	 - Manage packages

