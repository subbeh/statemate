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
		{"init.lua#template", "init.vim", "init.vim#template", "init.vim"},
		{"secret#encrypted#perm:600", "token", "token#encrypted#perm:600", "token"},
		{"init.lua", "init.vim", "init.vim", "init.vim"},
		// Attributes given: they replace the source's, and never reach the target.
		{"init.lua#template", "init.vim#perm:600", "init.vim#perm:600", "init.vim"},
		{"secret#encrypted#template", "token#template#encrypted", "token#template#encrypted", "token"},
		// Only the attributes change.
		{"init.lua", "init.lua#template", "init.lua#template", "init.lua"},
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

// Adding or dropping #encrypted by renaming would not encrypt or decrypt the
// content, leaving plaintext marked encrypted or ciphertext deployed as-is.
func TestRenamedNamesRejectsEncryptedChange(t *testing.T) {
	for _, tc := range []struct{ source, newName string }{
		{"secret#encrypted", "token#template"},
		{"config", "config#encrypted"},
	} {
		if _, _, err := renamedNames(tc.source, tc.newName); err == nil {
			t.Errorf("renamedNames(%q, %q): expected an error", tc.source, tc.newName)
		}
	}
}
