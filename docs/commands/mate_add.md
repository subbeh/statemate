# mate add

Add a file to source

## Synopsis

Add an existing file to the source directory.

The file is copied into a source, keeping its path relative to the target base
(stow-style), and the original stays in place. Directories cannot be added;
add the files in them one by one.

The source is the one named with --source, else 'default_source' from the
config, else one you pick from a list of the configured sources. The source must
already be listed under 'sources:' and exist as a directory.

A file outside your home directory, such as one under /etc, needs a source that
maps that location (see 'targets:' in a source's .mate.yaml); for a source
without a .mate.yaml, mate offers to create one.

--for-profile marks the file #profile:&lt;name>, so it is only deployed for that
profile. The global --profile only selects the active profile, which decides the
sources you can add to, as it does for every other command.

```
mate add <path> [flags]
```

## Examples

```
  mate add ~/.config/nvim/init.lua
  mate add --for-profile work ~/.gitconfig
  mate add --encrypt ~/.ssh/config
```

## Options

```
      --encrypt              encrypt file when adding
      --for-profile string   deploy the file only for this profile (adds #profile:<name>)
  -h, --help                 help for add
  -s, --source string        target source directory
      --template             mark file as template
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

