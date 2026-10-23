package cli

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/subbeh/statemate/internal/config"
)

// Nothing is logged unless a log file is set: the default slog handler would
// print to stderr.
func init() {
	slog.SetDefault(slog.New(slog.DiscardHandler))
}

// openLog sends log records to stderr at -v (info) or -vv (debug), and to the
// file named by --log-file, STATEMATE_LOG_FILE or log_file, in that order, at
// the level named the same way (info when none is). The file is appended to, so
// it keeps earlier runs; each run's records follow a "run" line.
func openLog(cmd *cobra.Command) error {
	var handlers []slog.Handler
	if v, _ := cmd.Flags().GetCount("verbose"); v > 0 {
		level := slog.LevelInfo
		if v > 1 {
			level = slog.LevelDebug
		}
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level, ReplaceAttr: dropTime}))
	}

	file, err := logFileHandler(cmd)
	if err != nil {
		return err
	}
	if file != nil {
		handlers = append(handlers, file)
	}
	if len(handlers) > 0 {
		slog.SetDefault(slog.New(slog.NewMultiHandler(handlers...)))
	}
	return nil
}

func logFileHandler(cmd *cobra.Command) (slog.Handler, error) {
	flagFile, _ := cmd.Flags().GetString("log-file")
	flagLevel, _ := cmd.Flags().GetString("log-level")
	cfgPath, _ := cmd.Flags().GetString("config")
	cfgFile, cfgLevel := config.LogSettings(cfgPath)

	file := cmp.Or(flagFile, os.Getenv("STATEMATE_LOG_FILE"), cfgFile)
	if file == "" {
		return nil, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(cmp.Or(flagLevel, os.Getenv("STATEMATE_LOG_LEVEL"), cfgLevel, "info"))); err != nil {
		return nil, fmt.Errorf("log level: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}
	// 0600: the log names every file and command, and a hook's command line
	// can hold whatever its template rendered into it.
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}
	h := &runHeader{
		Handler: slog.NewTextHandler(f, &slog.HandlerOptions{Level: level}),
		header:  slog.NewRecord(time.Now(), slog.LevelInfo, "run", 0),
		once:    new(sync.Once),
	}
	h.header.AddAttrs(slog.String("version", version), slog.Any("args", os.Args[1:]))
	// At debug every run is worth a line; at info only one that changed something.
	if level <= slog.LevelDebug {
		h.writeHeader(context.Background())
	}
	return h, nil
}

// runHeader writes the run's "run" record just before the first record it
// passes on, so at info the audit log holds only the runs that changed
// something.
type runHeader struct {
	slog.Handler
	header slog.Record
	once   *sync.Once
}

func (h *runHeader) writeHeader(ctx context.Context) {
	h.once.Do(func() { _ = h.Handler.Handle(ctx, h.header) })
}

func (h *runHeader) Handle(ctx context.Context, r slog.Record) error {
	h.writeHeader(ctx)
	return h.Handler.Handle(ctx, r)
}

func (h *runHeader) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &runHeader{h.Handler.WithAttrs(attrs), h.header, h.once}
}

func (h *runHeader) WithGroup(name string) slog.Handler {
	return &runHeader{h.Handler.WithGroup(name), h.header, h.once}
}

// dropTime leaves the timestamp out of stderr records: they print as they
// happen, so it only adds noise.
func dropTime(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}
