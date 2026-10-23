package plan

import (
	"os"
	"testing"
)

// A STATEMATE_KEY_FILE exported in the developer's shell would replace the age
// identity the tests configure.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("STATEMATE_KEY_FILE")
	os.Exit(m.Run())
}
