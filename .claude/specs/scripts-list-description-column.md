# Spec: description as a column in `mate scripts list`

## Problem

Descriptions are printed on their own indented line beneath each row:

```
  once       before   1      alpha      setup.sh                       pending
  Bootstrap the development environment and install tooling
  once       after    2      alpha      nodesc.sh                      pending
  always     after    3                 root.sh
  Short one
```

That breaks the table: rows are no longer one-per-script, the continuation line
does not align with any column, and scanning or `grep`-ing the output is awkward.

## Measured constraints

Real data, not assumptions:

- **Descriptions in the user's repo:** 16 of them, 15–58 chars, median ~30.
- **Terminal width:** 80 columns.
- **Current table width:** ~89 chars (header row 76, separator rule 90), so it
  *already* wraps at 80 before any new column is added.
- **Longest script name:** 25 chars (`50-claude-code-install.sh`), yet NAME is
  padded to a fixed 30.
- **Longest source name:** 9 chars (`tailscale`), yet SOURCE is padded to 10.

The fixed-width padding wastes space that descriptions need.

## Decisions

### One row per script, description in its own column

```
FREQUENCY  TIMING  ORDER  SOURCE  NAME       STATUS   DESCRIPTION
once       before  1      alpha   setup.sh   pending  Bootstrap the developm…
```

### Render with `tablewriter`, truncating to the terminal width

Switch from hand-rolled `fmt.Printf` padding to `tablewriter`, which `mate managed`
already uses. This gives content-sized columns for free — no more 30-char NAME
column for 25-char names.

**Verified behaviour** (tested against `tablewriter@v1.1.4` before writing this):

| Setting | Effect |
| --- | --- |
| default | **wraps** long cells onto continuation lines — reintroduces the exact problem we are removing |
| `WithRowAutoWrap(tw.WrapTruncate)` | truncates on one line with a `…` ellipsis |
| `WithMaxWidth(0)` | unlimited — no truncation |

So the required config is `WithRowAutoWrap(tw.WrapTruncate)` plus a `MaxWidth`
derived from the terminal.

### Width source: the terminal, unlimited when not a TTY

```go
width := 0 // 0 == unlimited
if term.IsTerminal(int(os.Stdout.Fd())) {
    if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
        width = w
    }
}
```

- **Interactive:** table fits the terminal; long descriptions get `…`.
- **Piped or redirected** (`| grep`, `> file`, CI): no terminal, so `MaxWidth(0)`
  and **descriptions print in full**. Truncating data the caller is about to
  process would be wrong. Verified: piped output shows the full 58-char
  description.

### Drop the leading `-` marker; STATUS already says `n/a`

Today column 1 holds `-` for profile-inactive scripts. That is redundant with the
`n/a` those rows already show under STATUS, so the marker goes and the column
count stays manageable.

### NAME keeps its existing suffixes

`[profile]` and `[T]` (template) stay appended to NAME, matching both today's
output and `mate managed`. No separate ATTRS column — the 80-col budget is
already tight.

## Out of scope

- Changing which scripts are listed, or the STATUS values themselves.
- A `--wide` / `--no-truncate` flag.
- Wrapping long descriptions (explicitly rejected — it is the current problem).
- Restyling `mate managed`.
- `--json` output (separately specced in `json-output.md`; it should emit the
  full untruncated description).

## Acceptance criteria

1. Each script occupies exactly one output row — no continuation lines.
2. `DESCRIPTION` is the final column; scripts without one leave it blank.
3. On a narrow terminal a long description is truncated with `…` and the row
   still aligns.
4. When piped, descriptions are printed in full with no truncation.
5. Column widths adapt to the content shown (no fixed 30-char NAME).
6. Profile-inactive scripts show `n/a` under STATUS; the old `-` marker is gone.
7. `[profile]` and `[T]` suffixes still appear on NAME.
8. `make test` and `make lint` pass.
