package cli

import (
	"os"
	"testing"
)

// A STATEMATE_KEY_FILE exported in the developer's shell would replace the age
// keys the tests encrypt with, and a STATEMATE_LOG_FILE would collect the
// tests' runs.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("STATEMATE_KEY_FILE")
	_ = os.Unsetenv("STATEMATE_LOG_FILE")
	_ = os.Unsetenv("STATEMATE_LOG_LEVEL")
	os.Exit(m.Run())
}
