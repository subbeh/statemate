package source

import (
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
	"template", "tmpl", "encrypted", "symlink", "import",
	"profile:", "perm:", "owner:", "group:",
	"perm-r:", "owner-r:", "group-r:",
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
			case "encrypted":
				attrs.Encrypted = true
			case "template", "tmpl":
				attrs.Template = true
			case "symlink":
				attrs.Symlink = true
			case "import":
				attrs.Import = true
			}
		}
	}

	return baseName, attrs
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
