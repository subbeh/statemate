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
			case "owner":
				attrs.Owner = val
			case "group":
				attrs.Group = val
			}
		} else {
			switch part {
			case "encrypted":
				attrs.Encrypted = true
			case "template":
				attrs.Template = true
			case "symlink":
				attrs.Symlink = true
			}
		}
	}

	return baseName, attrs
}

func (a *Attrs) Merge(parent Attrs) {
	if a.Profile == "" {
		a.Profile = parent.Profile
	}
	if a.Perm == 0 {
		a.Perm = parent.Perm
	}
	if a.Owner == "" {
		a.Owner = parent.Owner
	}
	if a.Group == "" {
		a.Group = parent.Group
	}
}
