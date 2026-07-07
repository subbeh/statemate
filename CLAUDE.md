# Statemate

Declarative system configuration management tool. Binary is `mate`.

## Build & Test

```bash
make build-all  # Build for all platforms (use this, not `make build`)
make test       # Run tests with race detection
make docs       # Generate man pages and markdown docs
make lint       # Run golangci-lint
```

## Project Structure

- `cmd/mate/` - Main binary entry point
- `cmd/gendocs/` - Documentation generator
- `internal/cli/` - Cobra commands
- `internal/config/` - Config parsing (YAML/TOML)
- `internal/source/` - Source tree discovery
- `internal/target/` - Apply logic, diff, sudo
- `internal/state/` - SQLite state database
- `internal/profile/` - Profile detection
- `internal/template/` - Go template rendering
- `internal/encrypt/` - Age encryption
- `internal/packages/` - Package managers (brew, pacman, aur)
- `internal/scripts/` - Lifecycle script execution

## Documentation

- **Command help text** is the source of truth - update `Short` and `Long` fields in cobra commands
- **Man pages and markdown** are generated from code via `make docs`
- **README.md** is a quick start only - don't duplicate command reference there
- **CHANGELOG.md** tracks user-facing changes

## When Making Changes

1. Update command help text in `internal/cli/*.go` if changing CLI behavior
2. Add user-facing changes to CHANGELOG.md under `[Unreleased]`
3. Run `make test` before finishing
4. Run `make docs` if command help changed (docs are gitignored, generated at release)

## Development Workflow

**All changes go through PRs** - no direct pushes to main.

1. **Create feature branch**: `git checkout -b feat/name` (or `fix/`, `chore/`, `docs/`, `refactor/`, `test/`)
2. **Implement changes**: Update code and CHANGELOG.md under `[Unreleased]`
3. **Run checks**: `make test` and `make lint` must pass
4. **Code review**: Run `/code-review` and fix any issues before creating PR
5. **Create PR**: Use conventional commit format for title, include Summary + Test Plan in body
6. **Squash merge**: All commits become one clean commit on main

For incomplete work, use **Draft PRs** on GitHub.

See CONTRIBUTING.md for full details.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `ci`

Examples:
- `feat(packages): add verbose flag for descriptions`
- `fix(add): resolve profile sources for inherited profiles`
- `chore(release): update goreleaser config`

## CHANGELOG Format

Add entries under `[Unreleased]` using these categories:
- **Added** - New features
- **Changed** - Changes to existing functionality  
- **Deprecated** - Features that will be removed
- **Removed** - Removed features
- **Fixed** - Bug fixes
- **Security** - Security fixes
