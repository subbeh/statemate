# mate add

Add a file or directory to source

## Synopsis

Add an existing file or directory to the source directory.

Files are copied into a source, keeping their paths relative to the target base
(stow-style), and the originals stay in place. Directories are added recursively,
including hidden files. Files already in the chosen source are skipped; a file
managed by another source stops the command before any files are copied.
File symlinks are copied as ordinary files using their contents; directory
symlinks, configured source directories, and empty directories are not added.
Unreadable files, broken symlinks, and special files stop the command.

The source is the one named with --source, else 'default_source' from the
config, else one you pick from a list of the configured sources. The source must
already be listed under 'sources:' and exist as a directory.

A file outside your home directory, such as one under /etc, needs a source that
maps that location (see 'targets:' in a source's .mate.yaml); for a source
without a .mate.yaml, mate offers to create one.

--for-profile marks each file #profile:&lt;name>, so it is only deployed for that
profile. --encrypt and --template also apply to every added file.
The global --profile only selects the active profile, which decides the
sources you can add to, as it does for every other command.

```
mate add <path> [flags]
```

## Examples

```
  mate add ~/.config/nvim/init.lua
  mate add ~/.config/nvim
  mate add --for-profile work ~/.gitconfig
  mate add --encrypt ~/.ssh/config
```

## Options

```
      --encrypt              encrypt each file when adding
      --for-profile string   deploy added files only for this profile (adds #profile:<name>)
  -h, --help                 help for add
  -s, --source string        target source directory
      --template             mark each file as template
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

