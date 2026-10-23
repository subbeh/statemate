# mate

Statemate - system configuration management

## Synopsis

Statemate manages your dotfiles, system configuration, and packages declaratively.

Features:

- Multi-directory source management, one directory per application
- Profile-based configuration with auto-detection
- Template rendering with Go text/template
- Age encryption for sensitive files
- Declarative package management (brew, pacman, and the AUR via yay or paru)
- System file management with permission control

Use "mate [command] --help" for more information about a command.

## Options

```
  -c, --config string      config file (default: mate.yaml in $STATEMATE_DIR if set, else in the local config's source_dir, else in the current directory)
  -h, --help               help for mate
      --log-file string    append a log of what mate changes to this file, and at debug what it decides and runs (also STATEMATE_LOG_FILE, or log_file in the config)
      --log-level string   log level: debug, info, warn or error (default info; also STATEMATE_LOG_LEVEL, or log_level in the config)
      --no-color           disable colored output (also off when stdout is not a terminal or NO_COLOR is set)
  -p, --profile string     override auto-detected profile
  -v, --verbose count      print log records to stderr: -v for info (what changed), -vv for debug
```

## SEE ALSO

* [mate add](mate_add.md)	 - Add a file or directory to source
* [mate apply](mate_apply.md)	 - Apply configuration to target
* [mate cat](mate_cat.md)	 - Print a file, decrypting it if needed
* [mate check](mate_check.md)	 - Check if configuration is in sync
* [mate clean](mate_clean.md)	 - Remove orphaned files
* [mate config](mate_config.md)	 - Inspect resolved configuration
* [mate decrypt](mate_decrypt.md)	 - Decrypt a managed file
* [mate delete](mate_delete.md)	 - Delete file from source and target
* [mate diff](mate_diff.md)	 - Show pending changes
* [mate doctor](mate_doctor.md)	 - Check configuration and dependencies
* [mate edit](mate_edit.md)	 - Edit a managed file
* [mate encrypt](mate_encrypt.md)	 - Encrypt a managed file
* [mate eval](mate_eval.md)	 - Render a template file
* [mate forget](mate_forget.md)	 - Remove files from tracking
* [mate hooks](mate_hooks.md)	 - Manage hooks
* [mate init](mate_init.md)	 - Initialize a new statemate repository
* [mate managed](mate_managed.md)	 - List all managed files
* [mate packages](mate_packages.md)	 - Manage packages
* [mate profile](mate_profile.md)	 - Show active profile
* [mate rename](mate_rename.md)	 - Rename a managed file
* [mate scripts](mate_scripts.md)	 - Manage scripts
* [mate secrets](mate_secrets.md)	 - Manage secrets
* [mate status](mate_status.md)	 - Show what apply would change
* [mate unmanaged](mate_unmanaged.md)	 - List files that are not managed
* [mate version](mate_version.md)	 - Print version information

