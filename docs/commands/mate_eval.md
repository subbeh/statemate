# mate eval

Render a template file

## Synopsis

Render a file as a template and print the result.

Useful for debugging a template, or previewing what apply would deploy. Any
file is rendered, whether or not it is marked #template. An encrypted file is
decrypted first, which needs the age identity.

The path is used as given: absolute, starting with ~, or relative to the
current directory. Use --profile to render it as another profile would.

```
mate eval <file> [flags]
```

## Examples

```
  mate eval git/.config/git/config#template
  mate eval --profile work git/.config/git/config#template
```

## Options

```
  -h, --help   help for eval
```

## Options inherited from parent commands

```
  -c, --config string    config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -p, --profile string   override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

