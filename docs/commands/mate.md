## mate

Statemate - system configuration management

### Synopsis

Statemate manages your dotfiles, system configuration, and packages declaratively.

Features:
  - Stow-style multi-directory source management
  - Profile-based configuration with auto-detection
  - Template rendering with Go text/template
  - Age encryption for sensitive files
  - Declarative package management (brew, pacman, yay)
  - System file management with permission control

Use "mate [command] --help" for more information about a command.

### Options

```
  -c, --config string    config file (default: mate.yaml in current directory)
  -h, --help             help for mate
  -p, --profile string   override auto-detected profile
```

### SEE ALSO

* [mate add](mate_add.md)	 - Add a file to source
* [mate apply](mate_apply.md)	 - Apply configuration to target
* [mate cat](mate_cat.md)	 - Display file contents
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
* [mate init](mate_init.md)	 - Initialize a new statemate repository
* [mate managed](mate_managed.md)	 - List all managed files
* [mate packages](mate_packages.md)	 - Manage packages
* [mate profile](mate_profile.md)	 - Show active profile
* [mate rename](mate_rename.md)	 - Rename a managed file
* [mate scripts](mate_scripts.md)	 - Manage scripts
* [mate secrets](mate_secrets.md)	 - Manage secrets
* [mate status](mate_status.md)	 - Show files that would change on apply
* [mate version](mate_version.md)	 - Print version information

