package cli

import "testing"

// The new name was used verbatim for source and target, so attributes were
// dropped from the source unless retyped -- and retyped ones leaked into the
// target filename.
func TestRenamedNames(t *testing.T) {
	tests := []struct {
		source, newName      string
		wantSource, wantTarget string
	}{
		// No attributes given: keep the source's own.
		{"init.lua#tmpl", "init.vim", "init.vim#tmpl", "init.vim"},
		{"secret#enc#perm:600", "token", "token#enc#perm:600", "token"},
		{"init.lua", "init.vim", "init.vim", "init.vim"},
		// Attributes given: they replace the source's, and never reach the target.
		{"init.lua#tmpl", "init.vim#perm:600", "init.vim#perm:600", "init.vim"},
		{"secret#enc#tmpl", "token#tmpl#enc", "token#tmpl#enc", "token"},
		// Only the attributes change.
		{"init.lua", "init.lua#tmpl", "init.lua#tmpl", "init.lua"},
	}
	for _, tc := range tests {
		gotSource, gotTarget, err := renamedNames(tc.source, tc.newName)
		if err != nil {
			t.Errorf("renamedNames(%q, %q): %v", tc.source, tc.newName, err)
			continue
		}
		if gotSource != tc.wantSource || gotTarget != tc.wantTarget {
			t.Errorf("renamedNames(%q, %q) = %q, %q; want %q, %q",
				tc.source, tc.newName, gotSource, gotTarget, tc.wantSource, tc.wantTarget)
		}
	}
}

// Adding or dropping #enc by renaming would not encrypt or decrypt the
// content, leaving plaintext marked encrypted or ciphertext deployed as-is.
func TestRenamedNamesRejectsEncryptedChange(t *testing.T) {
	for _, tc := range []struct{ source, newName string }{
		{"secret#enc", "token#tmpl"},
		{"config", "config#enc"},
	} {
		if _, _, err := renamedNames(tc.source, tc.newName); err == nil {
			t.Errorf("renamedNames(%q, %q): expected an error", tc.source, tc.newName)
		}
	}
}
