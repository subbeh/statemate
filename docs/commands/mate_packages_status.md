## mate packages status

Show package sync status

### Synopsis

Show package sync status across configured package managers.

Packages can be defined in:
  - mate.yaml (global packages)
  - mate.yaml profiles.<name>.packages (profile-specific)
  - <source>/.mate.yaml packages (source-level)
  - Files referenced via 'include' field

Use --all to show extra packages not in config. Detecting extras means listing
every installed package, which is noticeably slower, so it is only done when
--all is given.
Use --verbose to show package descriptions. A description of <unknown> means the
package manager does not recognise the name at all, usually a typo or a package
that only exists on another platform; an empty description means the package
exists but publishes none.

```
mate packages status [flags]
```

### Options

```
      --all       also show extra packages not in config
  -h, --help      help for status
  -v, --verbose   show package descriptions
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate packages](mate_packages.md)	 - Manage packages

