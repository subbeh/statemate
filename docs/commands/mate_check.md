## mate check

Check if configuration is in sync

### Synopsis

Exit 0 if in sync, 1 if changes pending. Useful for CI.

```
mate check [flags]
```

### Options

```
  -h, --help    help for check
  -q, --quiet   suppress output, only use exit code
      --sudo    use sudo to check files requiring elevated access
```

### Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

