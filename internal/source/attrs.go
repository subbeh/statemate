package source

import (
	"fmt"
	"strconv"
	"strings"
)

type Attrs struct {
	Profile   string
	Perm      uint32
	Owner     string
	Group     string
	Encrypted bool
	Template  bool
	Symlink   bool
	// Import marks a file whose target is normally authoritative, because the
	// application owns it and rewrites it (~/.claude/settings.json, say). A
	// changed target is imported back into the source instead of prompting.
	Import bool
	// Recursive attributes (inherited by children)
	PermR  uint32
	OwnerR string
	GroupR string
}

// AttrNames lists every attribute ParseAttrs understands. Value-taking
// attributes keep their trailing colon, so the entries read as they appear in a
// filename.
//
// This exists so the documentation test can enumerate attributes; keep it in step
// with ParseAttrs below. TestAttrNamesAreParsed checks that every name here
// actually parses, and the docs test checks that every name is documented.
var AttrNames = []string{
	"tmpl", "enc", "link", "import",
	"profile:", "perm:", "owner:", "group:",
	"perm-r:", "owner-r:", "group-r:",
}

// renamedAttrs maps the long attribute names mate used to accept to their
// replacements, so the error for a file not yet renamed says what to do.
var renamedAttrs = map[string]string{
	"encrypted": "enc",
	"template":  "tmpl",
	"symlink":   "link",
}

// CheckAttrs reports the first attribute in name that is not recognised, or
// whose mode does not parse. Unknown attributes used to be dropped silently, so a
// typo such as config#templte deployed an unrendered config.
func CheckAttrs(name string) error {
	for _, part := range strings.Split(name, "#")[1:] {
		if part == "" {
			continue
		}
		if short, ok := renamedAttrs[part]; ok {
			return fmt.Errorf("#%s was renamed to #%s", part, short)
		}
		known := false
		for _, a := range AttrNames {
			if part == a || (strings.HasSuffix(a, ":") && strings.HasPrefix(part, a)) {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errorf("unknown attribute #%s", part)
		}
		if key, val, _ := strings.Cut(part, ":"); key == "perm" || key == "perm-r" {
			if _, err := strconv.ParseUint(val, 8, 32); err != nil {
				return fmt.Errorf("#%s: %q is not an octal mode", part, val)
			}
		}
	}
	return nil
}

func ParseAttrs(name string) (baseName string, attrs Attrs) {
	parts := strings.Split(name, "#")
	baseName = parts[0]

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if part == "" {
			continue
		}

		if idx := strings.Index(part, ":"); idx != -1 {
			key := part[:idx]
			val := part[idx+1:]
			switch key {
			case "profile":
				attrs.Profile = val
			case "perm":
				if p, err := strconv.ParseUint(val, 8, 32); err == nil {
					attrs.Perm = uint32(p)
				}
			case "perm-r":
				if p, err := strconv.ParseUint(val, 8, 32); err == nil {
					attrs.PermR = uint32(p)
				}
			case "owner":
				attrs.Owner = val
			case "owner-r":
				attrs.OwnerR = val
			case "group":
				attrs.Group = val
			case "group-r":
				attrs.GroupR = val
			}
		} else {
			switch part {
			case "enc":
				attrs.Encrypted = true
			case "tmpl":
				attrs.Template = true
			case "link":
				attrs.Symlink = true
			case "import":
				attrs.Import = true
			}
		}
	}

	return baseName, attrs
}

// RemoveAttr returns name without the given flag attribute (e.g. "enc"),
// wherever it appears among the attributes. Attributes are not ordered, so
// x#enc#tmpl is as valid as x#tmpl#enc, and trimming a suffix would miss the
// former.
func RemoveAttr(name, attr string) string {
	parts := strings.Split(name, "#")
	kept := parts[:1]
	for _, part := range parts[1:] {
		if part != attr {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "#")
}

func (a *Attrs) Merge(parent Attrs) {
	if a.Profile == "" {
		a.Profile = parent.Profile
	}
	// Inherit recursive attributes from parent
	if a.PermR == 0 {
		a.PermR = parent.PermR
	}
	if a.OwnerR == "" {
		a.OwnerR = parent.OwnerR
	}
	if a.GroupR == "" {
		a.GroupR = parent.GroupR
	}
	// Apply recursive attributes as defaults if not explicitly set
	if a.Perm == 0 && a.PermR != 0 {
		a.Perm = a.PermR
	}
	if a.Owner == "" && a.OwnerR != "" {
		a.Owner = a.OwnerR
	}
	if a.Group == "" && a.GroupR != "" {
		a.Group = a.GroupR
	}
}
