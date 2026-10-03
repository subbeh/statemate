# mate secrets status

Show secrets that need fetching

## Synopsis

List the secret references the cache does not hold yet. 'mate apply' fetches these before deploying.

```
mate secrets status [flags]
```

## Options

```
  -h, --help   help for status
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate secrets](mate_secrets.md)	 - Manage secrets

