# Statemate

Declarative system configuration management tool. Binary is `mate`.

## Build & Test

```bash
make build-all  # Build for all platforms (use this, not `make build`)
make test       # Run tests with race detection
make docs       # Regenerate docs/commands/ from the cobra command tree
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

Documentation is committed under `docs/` and split by who writes it:

- **`docs/commands/` is GENERATED** by `make docs` from the cobra `Short`/`Long`
  fields. Never edit these files — change the help text in `internal/cli/*.go` and
  regenerate. CI and the pre-commit hook both fail on drift.
- **`docs/*.md` are HAND-WRITTEN** guides for what cobra cannot see: file
  attributes, config keys, templates, secrets, scripts, packages. Cobra only knows
  about commands and flags, so these formats have no other home.
- **README.md** is a quick start that links into `docs/` - don't duplicate the
  reference there
- **CHANGELOG.md** tracks user-facing changes

`internal/cli/docs_test.go` fails when a new attribute, config key, template
function, script frequency, or env var is not mentioned in the guides. If it
fails, the fix is to document the feature, not to relax the test. Man pages were
removed — they were never actually shipped to users, and cobra's roff output only
duplicated `--help`.

## When Making Changes

1. Update command help text in `internal/cli/*.go` if changing CLI behavior
2. **Update CHANGELOG.md** — see the mandatory rule below
3. Document new features in the relevant `docs/*.md` guide
4. Run `make docs` if command help changed, and stage `docs/commands/`
5. Run `make test` and `make lint` before finishing

## CHANGELOG is Mandatory

**Every commit that changes user-visible behavior MUST include a CHANGELOG.md entry in the same commit.**

This is not optional and is repeatedly forgotten. Treat it as part of the change, not a follow-up step.

**This is enforced by a pre-commit hook** (`.githooks/pre-commit`). Commits that stage
non-test files under `internal/` or `cmd/` without staging `CHANGELOG.md` are rejected.
The same hook regenerates `docs/commands/` when `internal/cli/` changes, and rejects
the commit if the result is not staged.

One-time setup after cloning (`core.hooksPath` is local git config, not version-controlled):

```bash
git config core.hooksPath .githooks
```

If the change genuinely isn't user-facing, bypass with `git commit --no-verify`.

**Checklist before every `git commit`:**

```bash
git diff --cached --stat   # if this touches internal/ or cmd/, CHANGELOG.md must be staged too
```

**What requires an entry:** bug fixes, new features, changed behavior, new/changed flags, removed functionality — anything a user would notice.

**What does not:** internal refactors with no behavior change, test-only changes, doc-only changes (CLAUDE.md, CONTRIBUTING.md), CI config.

Write entries from the **user's** perspective (what they observe), not the implementation's:

- Good: `` `mate clean` now uses sudo to remove files requiring elevated access ``
- Bad: `Added SudoRemove helper to target package`

## Development Workflow

Active development happens on the `develop` branch. Commit directly there — no feature branches or PRs needed during development.

1. **Work on develop**: `git checkout develop`
2. **Implement changes**: Update code
3. **Update CHANGELOG.md** under `[Unreleased]` (mandatory — see above)
4. **Run checks**: `make test` and `make lint` must pass
5. **Commit**: Use conventional commit format, including CHANGELOG.md in the same commit
6. **Push**: `git push` directly to develop

When ready for a release, merge `develop` into `main` (via PR or direct merge).

**No direct pushes to main** — main is the stable release branch.

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
