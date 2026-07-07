# Contributing to Statemate

This document describes the development workflow for statemate. It's designed for solo development with structure, using PRs and branches to maintain code quality and provide review checkpoints.

## Branch Strategy

### Main Branch
- `main` is the stable, releasable branch
- **Protected**: no direct pushes allowed, all changes go through PRs
- Releases are created by tagging commits on main

### Feature Branches
Use type-prefixed branch names matching the conventional commits format:

```
feat/eval-command       # New features
fix/symlink-detection   # Bug fixes
chore/update-deps       # Maintenance tasks
docs/improve-readme     # Documentation
refactor/cli-structure  # Code refactoring
test/add-config-tests   # Test additions
```

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout main
git pull
git checkout -b feat/your-feature-name
```

### 2. Implement Changes

- Make your changes in small, logical commits
- Follow the existing code style
- Update CHANGELOG.md under `[Unreleased]` with user-facing changes
- Run tests locally: `make test`
- Run linter locally: `make lint`

### 3. Code Review (AI-Assisted)

Before creating the PR, run an AI-assisted code review:

```bash
# In Claude Code
/code-review
```

Fix any issues identified by the review before proceeding.

### 4. Create Pull Request

```bash
git push -u origin feat/your-feature-name
```

Then create a PR on GitHub with:

**Title**: Follow conventional commits format
- `feat(scope): add new feature`
- `fix(scope): resolve issue with X`
- `chore(scope): update dependencies`

**Body**: Use the Summary + Test Plan format:

```markdown
## Summary
- Brief description of what changed
- Why this change was needed
- Any notable implementation details

## Test plan
- [ ] How to verify this works
- [ ] Edge cases to check
- [ ] Commands to run
```

### 5. CI Checks

PRs require passing checks before merge:
- `make test` - all tests must pass
- `make lint` - no linting errors

### 6. Merge

- Use **squash merge** to combine all commits into one clean commit
- The squash commit message uses the PR title as subject and PR body as body
- Branch is automatically deleted after merge

## Work in Progress

For incomplete work that needs to be pushed (backup, switching machines):

1. Open as a **Draft PR** on GitHub
2. Convert to "Ready for review" when complete
3. Draft PRs don't need to pass CI checks

## Release Process

1. Ensure all desired changes are merged to main
2. Update CHANGELOG.md: move items from `[Unreleased]` to a versioned section
3. Create and push a version tag:

```bash
git checkout main
git pull
git tag v1.2.3
git push origin v1.2.3
```

The GitHub Actions release workflow handles the rest.

## Working with Claude Code

When using Claude Code for development:

1. **Claude creates the branch**: Ask Claude to create a feature branch and implement the changes
2. **Claude runs review**: Claude runs `/code-review` before creating the PR
3. **Claude opens PR**: Claude creates the PR with proper title and description
4. **You approve merge**: Review the PR yourself and merge when satisfied

Example prompt:
> "Create a feature branch and implement X. Run code review, then open a PR."

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]
```

**Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `ci`

**Examples**:
- `feat(cli): add eval command for template debugging`
- `fix(apply): resolve symlink detection for sudo targets`
- `chore(deps): update cobra to v1.8.0`

## Quick Reference

| Step | Command/Action |
|------|----------------|
| Create branch | `git checkout -b feat/name` |
| Run tests | `make test` |
| Run linter | `make lint` |
| Code review | `/code-review` in Claude Code |
| Push branch | `git push -u origin feat/name` |
| Create PR | GitHub UI or `gh pr create` |
| Merge | Squash merge via GitHub UI |
| Release | Tag on main: `git tag v1.x.x && git push origin v1.x.x` |
