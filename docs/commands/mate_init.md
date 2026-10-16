# mate init

Initialize a new statemate repository

## Synopsis

Set up the current directory as a statemate repository.

In a directory without a config, mate init writes a commented mate.yaml (or
mate.toml), writes a README.md with setup instructions unless one exists, and
runs git init unless the directory is already inside a git repository. It then
offers to register the directory.

In a directory that already has a mate.yaml, mate.yml or mate.toml, such as a
fresh clone on a new machine, it only registers the directory.

Registering writes source_dir to the local config
(~/.config/statemate/config.yaml), so mate works from any directory. Other
settings in that file are kept.

```
mate init [flags]
```

## Options

```
      --format string   config format: yaml or toml
  -h, --help            help for init
```

## Options inherited from parent commands

```
  -c, --config string      config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
      --log-file string    append a log of what mate decides and runs to this file (also STATEMATE_LOG_FILE, or log_file in the config)
      --log-level string   log level: debug, info, warn or error (default info; also STATEMATE_LOG_LEVEL, or log_level in the config)
      --no-color           disable colored output (also off when stdout is not a terminal or NO_COLOR is set)
  -p, --profile string     override auto-detected profile
```

## SEE ALSO

* [mate](mate.md)	 - Statemate - system configuration management

