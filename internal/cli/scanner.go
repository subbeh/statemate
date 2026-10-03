package cli

import (
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/template"
)

func newScanner(cfg *config.Config, profileName string) (*source.Scanner, error) {
	tmplCtx, err := newTemplateContext(cfg, profileName)
	if err != nil {
		return nil, err
	}

	return source.NewScannerWithIgnore(cfg.TargetBase, cfg.SourceDir(), dirConfigRenderer(tmplCtx), cfg.Ignore), nil
}

// dirConfigRenderer renders a source's .mate.yaml with tmplCtx. Everything that
// reads that file must use it, or a templated value is taken literally.
func dirConfigRenderer(tmplCtx *template.Context) config.TemplateRenderer {
	return func(data []byte) ([]byte, error) {
		return template.Render(data, tmplCtx)
	}
}

// newTemplateContext builds a rendering context with decryption and secret
// lookups wired up where they are configured.
func newTemplateContext(cfg *config.Config, profileName string) (*template.Context, error) {
	var enc *encrypt.AgeEncryptor
	if cfg.Age != nil {
		enc, _ = encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
	}

	var ctxOpts []template.ContextOption
	if enc != nil && enc.CanDecrypt() {
		ctxOpts = append(ctxOpts, template.WithDecrypt(enc.Decrypt))
	}
	tmplCtx, err := template.NewContext(cfg, profileName, ctxOpts...)
	if err != nil {
		return nil, err
	}

	mgr, err := secrets.NewManager(enc, cfg.SecretsCache)
	if err == nil {
		tmplCtx.SecretLookup = func(item, typ, field string) (string, error) {
			key := secrets.CacheKey{Provider: "bitwarden", Item: item, Type: typ, Field: field}
			return mgr.Get(key)
		}
	}

	return tmplCtx, nil
}
