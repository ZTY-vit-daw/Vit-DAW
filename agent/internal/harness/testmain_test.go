package harness

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("VIT_AGENT_JOURNAL_PATH", "off")
	// FIX-TEST-DRAFT-ISOLATION-1: tests must never materialize drafts under the
	// real per-user ProjectHistory root (AGENTS §10). Isolate the process default
	// draft root into a throwaway directory for every test of this package; an
	// externally provided VIT_HISTORY_DRAFT_ROOT (CI) wins.
	draftRoot := ""
	if os.Getenv("VIT_HISTORY_DRAFT_ROOT") == "" {
		if dir, err := os.MkdirTemp("", "vit-harness-test-drafts-"); err == nil {
			draftRoot = dir
			_ = os.Setenv("VIT_HISTORY_DRAFT_ROOT", draftRoot)
		}
	}
	code := m.Run()
	if draftRoot != "" {
		_ = os.RemoveAll(draftRoot)
	}
	os.Exit(code)
}
