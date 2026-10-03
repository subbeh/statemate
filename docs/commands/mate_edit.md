# mate edit

Edit a managed file

## Synopsis

Edit a managed file in your editor.

Paths are resolved like any other command-line tool: absolute paths are used
as-is, relative paths resolve against the current directory.

Files under the source directory are opened directly. If you pass a target
path (a deployed file), the corresponding source file is opened instead --
mate never edits deployed files in place.

For encrypted files (with the '#encrypted' attribute), the file is decrypted to a
temporary location, opened in the editor, and re-encrypted after saving. The
original file permissions are preserved.

The editor is determined by (in order):

1. The 'editor' field in mate.yaml
2. $VISUAL environment variable
3. $EDITOR environment variable
4. vi (fallback)

The editor command may include arguments, such as 'code --wait'.

```
mate edit <path> [flags]
```

## Examples

```
  mate edit ~/.config/nvim/init.lua      # opens the source in the repository
  mate edit nvim/.config/nvim/init.lua
  mate edit ~/.ssh/config                # decrypts, edits, re-encrypts
```

## Options

```
  -h, --help   help for edit
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

