package secrets

import (
	"bytes"
	"os"
	"text/template"

	tmpl "github.com/subbeh/statemate/internal/template"
)

// DiscoverByRendering renders templates and collects all bitwarden() calls made.
// This resolves dynamic arguments (variables in range loops, etc).
func DiscoverByRendering(paths []string, ctx *tmpl.Context, decryptFn func([]byte) ([]byte, error)) []FetchItem {
	seen := make(map[string]bool)
	var all []FetchItem

	// Replace the SecretLookup with one that records calls
	origLookup := ctx.SecretLookup
	defer func() { ctx.SecretLookup = origLookup }()

	ctx.SecretLookup = func(item, typ, field string) (string, error) {
		key := CacheKey{Provider: "bitwarden", Item: item, Type: typ, Field: field}
		keyStr := key.String()
		if !seen[keyStr] {
			seen[keyStr] = true
			filename := ""
			if typ == "attachment" {
				filename = field
			}
			all = append(all, FetchItem{
				Key:      key,
				Item:     item,
				Type:     typ,
				Field:    field,
				Filename: filename,
			})
		}
		return "PLACEHOLDER", nil
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		if isEncrypted(content) && decryptFn != nil {
			decrypted, err := decryptFn(content)
			if err == nil {
				content = decrypted
			}
		}

		// Try to render — ignore errors (we just want to discover calls)
		t, err := template.New("").Funcs(discoveryFuncMap(ctx)).Parse(string(content))
		if err != nil {
			continue
		}
		var buf bytes.Buffer
		_ = t.Execute(&buf, ctx)
	}

	return all
}

// discoveryFuncMap is the normal rendering function set with the side effects
// removed. It must stay a superset of what Render accepts: a template that fails
// to parse here is skipped entirely, so its secrets are never fetched.
func discoveryFuncMap(ctx *tmpl.Context) template.FuncMap {
	fm := tmpl.FuncMap(ctx)

	// Shelling out during discovery is a side effect, and its output is not
	// needed to find which secrets a template asks for.
	fm["cmd"] = func(cmd string) string { return "" }

	// Discovery renders with placeholder secrets, so anything that validates a
	// value would abort on them. Nothing here should stop the walk.
	fm["required"] = func(val any) (any, error) { return val, nil }
	fm["base64Decode"] = func(val string) (string, error) { return val, nil }

	return fm
}
