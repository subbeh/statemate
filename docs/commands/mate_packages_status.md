# mate packages status

Show package sync status

## Synopsis

Show package sync status across configured package managers.

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
Use --verbose to show package descriptions. A description of &lt;unknown> means the
package manager does not recognise the name at all, usually a typo or a package
that only exists on another platform; an empty description means the package
exists but publishes none.

```
mate packages status [flags]
```

## Options

```
      --all       also show extra packages not in config
  -h, --help      help for status
  -v, --verbose   show package descriptions
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate packages](mate_packages.md)	 - Manage packages

