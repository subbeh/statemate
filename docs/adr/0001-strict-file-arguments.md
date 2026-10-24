# File arguments are paths, not search terms

Commands that act on one managed file (`edit`, `rename`, `encrypt`, `decrypt`,
`delete`) treat their argument like any command-line path: absolute, under `~`,
or relative to the current directory. A relative path that is not there is also
tried against the source directory. The path names an entry by its source file,
with or without the attributes, or by its target. A bare name such as
`init.lua` is never searched for.

We chose this over suffix matching because suffix matching picked an arbitrary
file when several matched, and `rename` and `delete` change files. It also lets
tab completion be plain filesystem completion: what the shell offers is exactly
what the command accepts. `mate edit` made this choice first (ef6e629), and the
other commands follow it.

**Considered options:** suffix matching that fails when it is ambiguous. We
rejected it because a name that is unique today becomes ambiguous once another
source adds a file of the same name, so the same command would stop working
later.
