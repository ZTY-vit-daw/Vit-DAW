package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/logx"
)

// countAuthorityRestoreLines reads the test log file and counts restore lines.
func countAuthorityRestoreLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log %s: %v", path, err)
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "[authority] restore") {
			count++
		}
	}
	return count
}

// AUTH-RESTORE-LOGSPAM-1: the scheduler reload path replays the durable
// snapshot every tick, so identical restores must not repeat the INFO line;
// a real mode change must surface again.
func TestAuthorityRestoreLogSilentOnIdenticalAdoptedRestores(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent_last.log")
	server := &Server{logger: logx.New(false, logPath, 256)}
	state := projectAgentRuntimeState{SavedAt: time.Now().UTC(), AuthorityMode: authorityModeFull}

	server.restoreProjectAgentRuntimeStateLocked(state)
	if got := countAuthorityRestoreLines(t, logPath); got != 1 {
		t.Fatalf("first restore: want 1 restore line, got %d", got)
	}
	if server.authorityMode != authorityModeFull {
		t.Fatalf("adopted restore must set mode, got %q", server.authorityMode)
	}

	for i := 0; i < 25; i++ {
		server.restoreProjectAgentRuntimeStateLocked(state)
	}
	if got := countAuthorityRestoreLines(t, logPath); got != 1 {
		t.Fatalf("identical repeats must stay silent: want 1 restore line, got %d", got)
	}

	changed := state
	changed.AuthorityMode = authorityModeManual
	server.restoreProjectAgentRuntimeStateLocked(changed)
	if got := countAuthorityRestoreLines(t, logPath); got != 2 {
		t.Fatalf("mode change must log again: want 2 restore lines, got %d", got)
	}
	if server.authorityMode != authorityModeManual {
		t.Fatalf("adopted restore must follow disk mode change, got %q", server.authorityMode)
	}
}

// AUTH-RESTORE-LOGSPAM-1: under an explicit latch the spam evidence shape
// (kept explicit mode=full disk=full, repeated) must collapse to one line,
// while a disk divergence stays visible; latch semantics are untouched.
func TestAuthorityRestoreLogExplicitLatchDedupeKeepsDivergenceVisible(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent_last.log")
	server := &Server{logger: logx.New(false, logPath, 256), authorityMode: authorityModeFull, authorityModeExplicit: true}
	state := projectAgentRuntimeState{SavedAt: time.Now().UTC(), AuthorityMode: authorityModeFull}

	server.restoreProjectAgentRuntimeStateLocked(state)
	for i := 0; i < 25; i++ {
		server.restoreProjectAgentRuntimeStateLocked(state)
	}
	if got := countAuthorityRestoreLines(t, logPath); got != 1 {
		t.Fatalf("explicit-latch identical repeats must stay silent: want 1 restore line, got %d", got)
	}
	if !server.authorityModeExplicit || server.authorityMode != authorityModeFull {
		t.Fatalf("kept-explicit restore must not touch latch or mode: mode=%q explicit=%t", server.authorityMode, server.authorityModeExplicit)
	}

	diverged := state
	diverged.AuthorityMode = authorityModeManual
	server.restoreProjectAgentRuntimeStateLocked(diverged)
	if got := countAuthorityRestoreLines(t, logPath); got != 2 {
		t.Fatalf("disk divergence under explicit latch must log: want 2 restore lines, got %d", got)
	}
	if server.authorityMode != authorityModeFull || !server.authorityModeExplicit {
		t.Fatalf("kept-explicit restore must keep in-memory mode: mode=%q explicit=%t", server.authorityMode, server.authorityModeExplicit)
	}
}
