package cli

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/template"
)

var evalCmd = &cobra.Command{
	Use:   "eval <file>",
	Short: "Render a template file",
	Long: `Render a file as a template and print the result.

Useful for debugging a template, or previewing what apply would deploy. Any
file is rendered, whether or not it is marked #tmpl. An encrypted file is
decrypted first, which needs the age identity.

The path is used as given: absolute, starting with ~, or relative to the
current directory. Use --profile to render it as another profile would.`,
	Example: `  mate eval git/.config/git/config#tmpl
  mate eval --profile work git/.config/git/config#tmpl`,
	Args:              cobra.ExactArgs(1),
	RunE:              runEval,
	ValidArgsFunction: completeFilePaths,
}

func init() {
	rootCmd.AddCommand(evalCmd)
}

func runEval(cmd *cobra.Command, args []string) error {
	filePath := expandPath(args[0])

	env, err := newEnv(cmd)
	if err != nil {
		return err
	}
	enc, tmplCtx := env.Enc, env.TmplCtx

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if isEncrypted(content) {
		if err := env.CheckDecrypt(); err != nil {
			return err
		}
		if enc != nil && enc.CanDecrypt() {
			content, err = enc.Decrypt(content)
			if err != nil {
				return fmt.Errorf("decrypting file: %w", err)
			}
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
