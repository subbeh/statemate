package source

import (
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	home := "/home/u"
	tests := []struct {
		pattern, target string
		want            bool
	}{
		// No slash: a name at any depth, in any target root.
		{"*.service", "/etc/systemd/system/keyd.service", true},
		{"*.service", "/home/u/.config/systemd/user/a.service", true},
		{"*.service", "/etc/systemd/system/keyd.timer", false},
		{"profile.d", "/etc/profile.d/x.env", true},
		// Relative with a slash: anchored at home.
		{".config/tmux/*.conf", "/home/u/.config/tmux/tmux.conf", true},
		{".config/tmux/*.conf", "/home/u/.config/tmux/sub/a.conf", false},
		{".config/tmux/*.conf", "/etc/.config/tmux/tmux.conf", false},
		{"~/.config/tmux/*.conf", "/home/u/.config/tmux/tmux.conf", true},
		// ** spans segments, including none.
		{".config/tmux/**/*.conf", "/home/u/.config/tmux/tmux.conf", true},
		{".config/tmux/**/*.conf", "/home/u/.config/tmux/a/b/c.conf", true},
		{"/etc/**", "/etc/keyd/default.conf", true},
		// Absolute.
		{"/etc/keyd/*.conf", "/etc/keyd/default.conf", true},
		{"/etc/keyd/*.conf", "/home/u/etc/keyd/default.conf", false},
		// A directory matches what is under it; a trailing slash only that.
		{".config/tmux", "/home/u/.config/tmux/tmux.conf", true},
		{".config/tmux/", "/home/u/.config/tmux/tmux.conf", true},
		{"tmux.conf/", "/home/u/.config/tmux/tmux.conf", false},
		{"~", "/home/u/.zshrc", true},
		// Source attributes in patterns do not appear in the deployed target.
		{"*.service#tmpl", "/etc/systemd/system/keyd.service", true},
		{"*.service#tmpl", "/etc/systemd/system/keyd.timer", false},
		{".config#perm-r:700/tmux/*.conf#enc#tmpl", "/home/u/.config/tmux/tmux.conf", true},
		{"~/.config/tmux#owner-r:root/**/*.conf#profile:work", "/home/u/.config/tmux/a/b/c.conf", true},
		{"/etc#group-r:root/keyd/*.conf#tmpl#perm:600", "/etc/keyd/default.conf", true},
		{".config/tmux#perm:700/", "/home/u/.config/tmux/tmux.conf", true},
		{"tmux.conf#tmpl/", "/home/u/.config/tmux/tmux.conf", false},
		{".config/tmux#perm:700/*.conf#tmpl", "/home/u/.config/tmux/sub/a.conf", false},
		// Unknown hashes, escaped hashes, and glob classes remain literal.
		{"note#draft#tmpl", "/home/u/note#draft", true},
		{"note#", "/home/u/note#", true},
		{"note##draft", "/home/u/note##draft", true},
		{"note#tmpl-copy", "/home/u/note#tmpl-copy", true},
		{`note\#tmpl`, "/home/u/note#tmpl", true},
		{"note[#]tmpl", "/home/u/note#tmpl", true},
		{"note[#tmpl]#tmpl", "/home/u/note#", true},
		{`note[\]]#tmpl`, "/home/u/note]", true},
		// A literal attribute-like suffix in a real target is not stripped.
		{"note", "/home/u/note#tmpl", false},
	}
	for _, tc := range tests {
		if got := match(tc.pattern, tc.target, home); got != tc.want {
			t.Errorf("match(%q, %q) = %v, want %v", tc.pattern, tc.target, got, tc.want)
		}
	}
}

func TestMatchAllSourceAttributes(t *testing.T) {
	for _, attr := range AttrNames {
		if strings.HasSuffix(attr, ":") {
			attr += "value"
		}
		t.Run(attr, func(t *testing.T) {
			if !match(".config#"+attr+"/app.conf#"+attr, "/home/u/.config/app.conf", "/home/u") {
				t.Errorf("#%s should be ignored in directory and file patterns", attr)
			}
		})
	}
}
