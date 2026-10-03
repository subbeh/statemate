# mate add

Add a file to source

## Synopsis

Add an existing file to the source directory.

The file is copied from its current location to the appropriate source directory,
following stow-style conventions. The original file remains in place.

--for-profile marks the file #profile:&lt;name>, so it is only deployed for that
profile. The global --profile only selects the active profile, which decides the
sources you can add to, as it does for every other command.

Examples:

```
mate add ~/.config/nvim/init.lua
mate add --for-profile work ~/.gitconfig
mate add --encrypt ~/.ssh/config
```

```
mate add <path> [flags]
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

