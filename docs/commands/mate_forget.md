# mate forget

Remove files from tracking

## Synopsis

Remove files from statemate's tracking database.

The files at target remain untouched. Only the tracking entries are removed.
That sticks only once the file is gone from the source: if a source still
provides it, the next apply tracks it again.

Paths are matched against the target, and may be absolute, start with ~, or be
relative to the current directory.

Supports wildcards (glob patterns) to forget multiple files at once. Quote a
pattern so the shell does not expand it first.

```
mate forget <path>... [flags]
```

## Examples

```
  mate forget ~/.config/nvim/init.lua
  mate forget '~/.config/nvim/*.lua'
  mate forget .config/nvim/init.lua
  mate forget ~/.config/app/file1.conf ~/.config/app/file2.conf
```

## Options

```
  -h, --help   help for forget
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

