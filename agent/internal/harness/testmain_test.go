package harness

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("VIT_AGENT_JOURNAL_PATH", "off")
	os.Exit(m.Run())
}
