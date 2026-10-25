// Package plan resolves what one run of mate works from (the Env) and what the
// sources say should be on disk for it (the Plan). See GLOSSARY.md.
package plan

import (
	"errors"
	"fmt"
	"os"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/source"
	"github.com/subbeh/statemate/internal/state"
	"github.com/subbeh/statemate/internal/target"
	"github.com/subbeh/statemate/internal/template"
)

// Env is everything a run resolves before it reads any source.
type Env struct {
	Config  *config.Config
	Profile string
	// Chain is the profile's inheritance chain, empty without a profile.
	Chain []string
	// SourcePaths are the absolute directories of the profile's sources.
	SourcePaths []string
	// Enc is nil without an age block, or when the identity fails to load.
	Enc *encrypt.AgeEncryptor
	// Secrets is nil when the cache location cannot be resolved.
	Secrets *secrets.Manager
	TmplCtx *template.Context
	Scanner *source.Scanner

	// encErr is why the configured identity failed to load. It is reported only
	// when something has to decrypt, so a repository without #enc files still
	// works without its key.
	encErr error
	// cfgPath is what NewEnv loaded, for Reload.
	cfgPath string
}

// NewEnv loads and validates the config at cfgPath, and detects the profile
// when profileName is empty.
func NewEnv(cfgPath, profileName string) (*Env, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}
	e, err := FromConfig(cfg, profileName)
	if err != nil {
		return nil, err
	}
	e.cfgPath = cfgPath
	return e, nil
}

// FromConfig builds the Env for a config the caller has already loaded.
func FromConfig(cfg *config.Config, profileName string) (*Env, error) {
	e := &Env{
		Config:      cfg,
		Profile:     profileName,
		Chain:       profile.InheritanceChain(cfg, profileName),
		SourcePaths: cfg.ResolveSourcePaths(profile.ResolveSources(cfg, profileName)),
	}
	if cfg.Age != nil {
		e.Enc, e.encErr = encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
	}
	if mgr, err := secrets.NewManager(e.Enc, cfg.SecretsCache); err == nil {
		e.Secrets = mgr
	}

	if err := e.newTemplateContext(); err != nil {
		return nil, err
	}
	e.Scanner = source.NewScannerWithIgnore(cfg.TargetBase, cfg.SourceDir(), DirConfigRenderer(e.TmplCtx), cfg.Ignore)
	return e, nil
}

// Reload re-reads the config of an Env from NewEnv and rebuilds its template
// context, since #before scripts may write var_files. The identity is kept:
// loading it again could prompt a second time.
func (e *Env) Reload() error {
	cfg, err := config.Load(e.cfgPath)
	if err != nil {
		return fmt.Errorf("reloading config: %w", err)
	}
	e.Config = cfg
	return e.newTemplateContext()
}

func (e *Env) newTemplateContext() error {
	var ctxOpts []template.ContextOption
	if e.Enc != nil && e.Enc.CanDecrypt() {
		ctxOpts = append(ctxOpts, template.WithDecrypt(e.Enc.Decrypt))
	}
	tmplCtx, err := template.NewContext(e.Config, e.Profile, ctxOpts...)
	if err != nil {
		if e.encErr != nil {
			return fmt.Errorf("creating template context: %w (the age identity failed to load: %v)", err, e.encErr)
		}
		return fmt.Errorf("creating template context: %w", err)
	}
	if e.Secrets != nil {
		tmplCtx.SecretLookup = e.lookupSecret
	}
	e.TmplCtx = tmplCtx
	return nil
}

func (e *Env) lookupSecret(item, typ, field string) (string, error) {
	v, err := e.Secrets.Get(secrets.CacheKey{Provider: "bitwarden", Item: item, Type: typ, Field: field})
	if e.encErr != nil && errors.Is(err, secrets.ErrNoIdentity) {
		return "", fmt.Errorf("reading the secrets cache: the age identity failed to load: %w", e.encErr)
	}
	return v, err
}

// CheckDecrypt reports why content cannot be decrypted, if the configured
// identity failed to load.
func (e *Env) CheckDecrypt() error {
	if e.encErr != nil {
		return fmt.Errorf("the age identity failed to load: %w", e.encErr)
	}
	return nil
}

// DirConfigRenderer renders a source's .mate.yaml with tmplCtx. Everything that
// reads that file must use it, or a templated value is taken literally.
func DirConfigRenderer(tmplCtx *template.Context) config.TemplateRenderer {
	return func(data []byte) ([]byte, error) {
		return template.Render(data, tmplCtx)
	}
}

// OpenState opens the state database for cfg's repository, first resolving any
// paths an older version recorded relative to it. Commands that look up script
// runs or source paths must open the database this way, or rows written before
// the source dir became absolute would no longer be found.
func OpenState(cfg *config.Config) (*state.DB, error) {
	db, err := state.Open("")
	if err != nil {
		return nil, err
	}
	if err := db.ResolveRelativePaths(cfg.SourceDir()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("updating state database: %w", err)
	}
	return db, nil
}

// Plan is what the sources say should be on disk for an Env: the profile's
// entries and their conflicts, and on request the changes and orphans.
type Plan struct {
	Tree *source.Tree

	env *Env
	db  *state.DB
}

// Plan scans the profile's sources. Close releases the state database once
// DB, Changes or Orphans has opened it.
func (e *Env) Plan() (*Plan, error) {
	tree, err := e.Scanner.Scan(e.SourcePaths)
	if err != nil {
		return nil, fmt.Errorf("scanning sources: %w", err)
	}
	if e.Profile != "" {
		tree = tree.FilterByProfile(e.Chain)
	}
	return &Plan{Tree: tree, env: e}, nil
}

// DB opens the state database on first use.
func (p *Plan) DB() (*state.DB, error) {
	if p.db == nil {
		db, err := OpenState(p.env.Config)
		if err != nil {
			return nil, fmt.Errorf("opening state database: %w", err)
		}
		p.db = db
	}
	return p.db, nil
}

func (p *Plan) Close() error {
	if p.db == nil {
		return nil
	}
	return p.db.Close()
}

// Changes compares the entries with their targets and the state database.
// With sudo it asks for sudo first, so targets that need it are compared too.
func (p *Plan) Changes(sudo bool) (*target.ComputeResult, error) {
	if p.env.encErr != nil {
		// Otherwise the entry fails as if no identity were configured at all.
		for _, e := range p.Tree.Files() {
			if e.Attrs.Encrypted && !e.Attrs.Symlink {
				return nil, fmt.Errorf("%s is #enc, but %w", e.SourcePath, p.env.CheckDecrypt())
			}
		}
	}
	db, err := p.DB()
	if err != nil {
		return nil, err
	}
	res, err := target.ComputeChanges(p.Tree, db, target.ComputeOpts{TmplCtx: p.env.TmplCtx, Enc: p.env.Enc, Sudo: sudo})
	if err != nil {
		return nil, fmt.Errorf("computing changes: %w", err)
	}
	return res, nil
}

// Orphans are targets an earlier apply recorded that no entry deploys any
// more, and that still exist on disk.
func (p *Plan) Orphans() ([]string, error) {
	db, err := p.DB()
	if err != nil {
		return nil, err
	}
	managed, err := db.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("finding orphans: %w", err)
	}

	current := make(map[string]bool)
	for _, e := range p.Tree.Files() {
		current[e.TargetPath] = true
	}

	var orphans []string
	for _, m := range managed {
		if !current[m.TargetPath] {
			if _, err := os.Stat(m.TargetPath); err == nil {
				orphans = append(orphans, m.TargetPath)
			}
		}
	}
	return orphans, nil
}
