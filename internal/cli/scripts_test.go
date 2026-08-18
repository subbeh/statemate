package cli

import (
	"strings"
	"testing"
)

var listHeaders = []string{"ORDER", "NAME", "SOURCE", "FREQUENCY", "TIMING", "STATUS", "DESCRIPTION"}

// row builds a scripts-list row in column order. Arguments stay in the caller's
// reading order (frequency first) so the tests below read naturally; the returned
// slice is in display order.
func row(freq, timing, order, source, name, status, desc string) []string {
	return []string{order, name, source, freq, timing, status, desc}
}

func TestTruncateDescriptions(t *testing.T) {
	tests := []struct {
		name   string
		desc   string
		budget int
		want   string
	}{
		{"fits exactly", "abcde", 5, "abcde"},
		{"shorter than budget", "abc", 10, "abc"},
		{"too long is ellipsized", "abcdefghij", 5, "abcd…"},
		{"empty stays empty", "", 5, ""},
		// A budget of 1 would leave only the ellipsis, which carries no
		// information -- clear the column instead.
		{"budget 1 clears", "abcdef", 1, ""},
		{"budget 0 clears", "abcdef", 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := [][]string{row("once", "before", "1", "src", "s.sh", "pending", tc.desc)}
			truncateDescriptions(rows, tc.budget)
			if got := rows[0][descriptionColumn]; got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTruncateDescriptions_ResultNeverExceedsBudget(t *testing.T) {
	long := strings.Repeat("x", 200)
	for budget := 2; budget <= 40; budget++ {
		rows := [][]string{row("once", "before", "1", "src", "s.sh", "pending", long)}
		truncateDescriptions(rows, budget)
		if n := len([]rune(rows[0][descriptionColumn])); n > budget {
			t.Fatalf("budget %d produced %d runes", budget, n)
		}
	}
}

// The budget must leave room for every other column at its widest value,
// including the header text.
func TestDescriptionBudget_FitsTerminal(t *testing.T) {
	rows := [][]string{
		row("once", "before", "1", "alpha", "setup.sh", "pending", strings.Repeat("d", 100)),
		row("always", "after", "12", "beta", "a-longer-name.sh [T]", "n/a", "short"),
	}

	for _, width := range []int{80, 100, 120, 200} {
		budget := descriptionBudget(rows, listHeaders, width)

		// Recreate the widest row as tablewriter would render it: every column
		// padded to its widest cell (header included), two padding chars per
		// column, plus a leading and trailing space.
		used := 0
		for i, h := range listHeaders {
			w := len([]rune(h))
			for _, r := range rows {
				if n := len([]rune(r[i])); n > w && i != descriptionColumn {
					w = n
				}
			}
			if i == descriptionColumn {
				if budget > w {
					w = budget
				}
			}
			used += w
		}
		total := used + 2 + 2*len(listHeaders)

		if total > width {
			t.Errorf("width %d: budget %d yields a %d-wide table", width, budget, total)
		}
	}
}

func TestDescriptionBudget_ZeroWhenNoRoom(t *testing.T) {
	rows := [][]string{
		row("once", "before", "1", "a-very-long-source-name", "a-very-long-script-name.sh", "pending", "desc"),
	}
	// A terminal narrower than the other columns alone leaves nothing.
	if got := descriptionBudget(rows, listHeaders, 40); got != 0 {
		t.Errorf("expected no budget on a narrow terminal, got %d", got)
	}
}

func TestDescriptionBudget_GrowsWithTerminal(t *testing.T) {
	rows := [][]string{row("once", "before", "1", "alpha", "setup.sh", "pending", "desc")}

	narrow := descriptionBudget(rows, listHeaders, 90)
	wide := descriptionBudget(rows, listHeaders, 140)
	if wide <= narrow {
		t.Errorf("a wider terminal should allow a longer description: %d vs %d", narrow, wide)
	}
}
