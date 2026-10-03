package cli

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/encrypt"
	"github.com/subbeh/statemate/internal/profile"
	"github.com/subbeh/statemate/internal/secrets"
	"github.com/subbeh/statemate/internal/template"
)

var evalCmd = &cobra.Command{
	Use:   "eval <file>",
	Short: "Render a template file",
	Long: `Render a file as a template and print the result.

Useful for debugging a template, or previewing what apply would deploy. Any
file is rendered, whether or not it is marked #template. An encrypted file is
decrypted first, which needs the age identity.

The path is used as given: absolute, starting with ~, or relative to the
current directory. Use --profile to render it as another profile would.`,
	Example: `  mate eval git/.config/git/config#template
  mate eval --profile work git/.config/git/config#template`,
	Args:              cobra.ExactArgs(1),
	RunE:              runEval,
	ValidArgsFunction: completeFilePaths,
}

func init() {
	rootCmd.AddCommand(evalCmd)
}

func runEval(cmd *cobra.Command, args []string) error {
	filePath := expandPath(args[0])

	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	profileName, _ := cmd.Flags().GetString("profile")
	if profileName == "" {
		profileName = profile.Detect(cfg)
	}

	var enc *encrypt.AgeEncryptor
	if cfg.Age != nil {
		enc, err = encrypt.NewAgeEncryptor(cfg.Age.Identity, cfg.Age.IdentityCommand, cfg.Age.Recipients)
		if err != nil {
			return fmt.Errorf("setting up encryption: %w", err)
		}
	}

	var ctxOpts []template.ContextOption
	if enc != nil && enc.CanDecrypt() {
		ctxOpts = append(ctxOpts, template.WithDecrypt(enc.Decrypt))
	}
	tmplCtx, err := template.NewContext(cfg, profileName, ctxOpts...)
	if err != nil {
		return fmt.Errorf("creating template context: %w", err)
	}

	if enc != nil && enc.CanDecrypt() {
		mgr, err := secrets.NewManager(enc, cfg.SecretsCache)
		if err == nil {
			tmplCtx.SecretLookup = func(item, typ, field string) (string, error) {
				key := secrets.CacheKey{Provider: "bitwarden", Item: item, Type: typ, Field: field}
				return mgr.Get(key)
			}
		}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if enc != nil && enc.CanDecrypt() && isEncrypted(content) {
		content, err = enc.Decrypt(content)
		if err != nil {
			return fmt.Errorf("decrypting file: %w", err)
		}
	}

	rendered, err := template.Render(content, tmplCtx)
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	_, _ = os.Stdout.Write(rendered)
	return nil
}

// isEncrypted reports whether content is an age file in armored form, which is what
// mate encrypt writes.
//
// The comparison has to be a prefix test: comparing a fixed-length slice of the
// content against a shorter string never matched, so mate cat printed ciphertext and
// mate eval tried to render it as a template.
func isEncrypted(content []byte) bool {
	return bytes.HasPrefix(content, []byte("-----BEGIN AGE ENCRYPTED FILE-----"))
}
