package cli

import (
	"fmt"

	"github.com/subbeh/statemate/internal/config"
	"github.com/subbeh/statemate/internal/state"
)

// openState opens the state database for cfg's repository, first resolving any
// paths an older version recorded relative to it. Commands that look up script
// runs or source paths must open the database this way, or rows written before
// the source dir became absolute would no longer be found.
func openState(cfg *config.Config) (*state.DB, error) {
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
