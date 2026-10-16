package cli

import (
	"cmp"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
)

// Nothing is logged unless a log file is set: the default slog handler would
// print to stderr.
func init() {
	slog.SetDefault(slog.New(slog.DiscardHandler))
}

// openLog starts logging to the file named by --log-file, STATEMATE_LOG_FILE
// or log_file, in that order, at the level named the same way (info when none
// is). The file is appended to, so it keeps earlier runs; each run starts with
// a "run" line.
func openLog(cmd *cobra.Command) error {
	flagFile, _ := cmd.Flags().GetString("log-file")
	flagLevel, _ := cmd.Flags().GetString("log-level")
	cfgPath, _ := cmd.Flags().GetString("config")
	cfgFile, cfgLevel := config.LogSettings(cfgPath)

	file := cmp.Or(flagFile, os.Getenv("STATEMATE_LOG_FILE"), cfgFile)
	if file == "" {
		return nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(cmp.Or(flagLevel, os.Getenv("STATEMATE_LOG_LEVEL"), cfgLevel, "info"))); err != nil {
		return fmt.Errorf("log level: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	// 0600: the log names every file and command, and a hook's command line
	// can hold whatever its template rendered into it.
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: level})))
	slog.Info("run", "version", version, "args", os.Args[1:])
	return nil
}
