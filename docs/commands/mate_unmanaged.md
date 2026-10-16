# mate unmanaged

List files that are not managed

## Synopsis

List files under a directory that no source deploys.

With no argument, lists the current directory; otherwise the given directory.
A file counts as managed when any source deploys it, for any profile, matching
what 'mate managed' lists.

Only directories that contain managed files are descended into. A directory
holding nothing managed is listed once, with a trailing slash, rather than
file by file. Paths are relative to the current directory, so they can be passed
to 'mate add'.

```
mate unmanaged [path] [flags]
```

## Examples

```
  mate unmanaged                  # unmanaged files here
  mate unmanaged ~/.config        # unmanaged files in ~/.config
```

## Options

```
  -h, --help   help for unmanaged
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

