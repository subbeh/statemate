# Statemate

Statemate deploys files from source directories in a repository to their targets
on a machine, as a profile selects them.

## Language

**Env**:
Everything one run of mate resolves before it looks at any source: the config,
the profile, the age identity and the template context.
_Avoid_: setup, context, environment (environment variables are a separate thing)

**Plan**:
What the sources say should be on disk for one Env, and how that differs from
the targets and the state database: entries, conflicts, changes and orphans.
_Avoid_: desired state, tree, diff
