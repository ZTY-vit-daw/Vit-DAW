package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// M3-AUTH-RESTORE-CARRY-1 probe obligations: the clean-restart seeding source
// is the durable agent runtime state chain (the files WriteAgentRuntimeState
// maintains), and the newest persisted authority_mode for the project identity
// wins — the canonical workspace may lag the previous session's last persist.

func writeAuthSeedRuntimeState(t *testing.T, path, mode string, savedAt time.Time) {
	t.Helper()
	payload := map[string]any{"schema_version": "vit_project_agent_runtime.v2"}
	if mode != "" {
		payload["authority_mode"] = mode
	}
	if !savedAt.IsZero() {
		payload["saved_at"] = savedAt
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeAuthSeedSession lays down one working session on disk the way
// EnsureWorkingSession leaves it, so LatestPersistedAuthorityMode sees a real
// .sessions/<uuid>/<sid> layout. Returns the session workspace state path.
func writeAuthSeedSession(t *testing.T, root, projectUUID, sessionID, projectPath string) string {
	t.Helper()
	sessionRoot := filepath.Join(root, DirName, workingSessionsDirName, projectUUID, sessionID)
	workspaceDir := filepath.Join(sessionRoot, "workspace")
	session := WorkingSession{
		SchemaVersion: "vit_project_working_session.v1", SessionID: sessionID,
		ProjectPath: projectPath, ProjectUUID: projectUUID,
		WorkspaceDir: workspaceDir, Status: "active",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionRoot, "session.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(workspaceDir, "state", agentRuntimeStateFile)
}

func TestLatestPersistedAuthorityModePrefersPreviousSessionValueWhenCanonicalLags(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "carry.vit")
	const projectUUID = "vitproj_seed_lag"
	older := time.Now().UTC().Add(-time.Hour)

	// The previous session persisted full; the canonical workspace (the
	// clean-restart seeding source) carries no authority_mode at all — the
	// 10-29 incident shape.
	sessionState := writeAuthSeedSession(t, root, projectUUID, "session_old", projectPath)
	writeAuthSeedRuntimeState(t, sessionState, "full_project_access", older)

	mode, ok := LatestPersistedAuthorityMode(projectPath, projectUUID)
	if !ok || mode != "full_project_access" {
		t.Fatalf("LatestPersistedAuthorityMode=(%q,%t), want (full_project_access,true)", mode, ok)
	}
}

func TestLatestPersistedAuthorityModeAdoptsNewestPersistedValue(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "carry.vit")
	const projectUUID = "vitproj_seed_newest"
	older := time.Now().UTC().Add(-2 * time.Hour)
	newer := time.Now().UTC().Add(-time.Hour)

	// The user switched full -> manual: the newest durable record is the
	// session's manual, the stale canonical still says full. Manual wins.
	sessionState := writeAuthSeedSession(t, root, projectUUID, "session_switch", projectPath)
	writeAuthSeedRuntimeState(t, sessionState, "manual_confirmation", newer)
	writeAuthSeedRuntimeState(t, filepath.Join(root, DirName, projectUUID, "state", agentRuntimeStateFile), "full_project_access", older)

	if mode, ok := LatestPersistedAuthorityMode(projectPath, projectUUID); !ok || mode != "manual_confirmation" {
		t.Fatalf("newest session manual must win: got (%q,%t)", mode, ok)
	}

	// Reversed staleness: the canonical commit is newer than the old session.
	writeAuthSeedRuntimeState(t, filepath.Join(root, DirName, projectUUID, "state", agentRuntimeStateFile), "manual_confirmation", time.Now().UTC())
	if mode, ok := LatestPersistedAuthorityMode(projectPath, projectUUID); !ok || mode != "manual_confirmation" {
		t.Fatalf("newest canonical manual must win: got (%q,%t)", mode, ok)
	}
}

func TestLatestPersistedAuthorityModeFailsClosedWithoutPersistedMode(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "carry.vit")
	const projectUUID = "vitproj_seed_absent"

	// Files exist but none carries an authority_mode: no value, not a default.
	writeAuthSeedRuntimeState(t, filepath.Join(root, DirName, projectUUID, "state", agentRuntimeStateFile), "", time.Now().UTC())
	sessionState := writeAuthSeedSession(t, root, projectUUID, "session_blank", projectPath)
	writeAuthSeedRuntimeState(t, sessionState, "", time.Now().UTC())

	if mode, ok := LatestPersistedAuthorityMode(projectPath, projectUUID); ok || mode != "" {
		t.Fatalf("no persisted mode must return (\"\",false), got (%q,%t)", mode, ok)
	}

	// A foreign project's session under the same root never contributes.
	foreignPath := filepath.Join(root, "other.vit")
	foreignState := writeAuthSeedSession(t, root, projectUUID, "session_foreign", foreignPath)
	writeAuthSeedRuntimeState(t, foreignState, "full_project_access", time.Now().UTC())
	if mode, ok := LatestPersistedAuthorityMode(projectPath, projectUUID); ok || mode != "" {
		t.Fatalf("foreign session must not leak its mode: got (%q,%t)", mode, ok)
	}
}

func TestLatestPersistedAuthorityModePassesValueThroughUnnormalized(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "carry.vit")
	const projectUUID = "vitproj_seed_raw"
	sessionState := writeAuthSeedSession(t, root, projectUUID, "session_raw", projectPath)
	writeAuthSeedRuntimeState(t, sessionState, "banana", time.Now().UTC())

	// The read is faithful to disk; fail-closed normalization is the restore
	// side's job (normalizeAuthorityModeOrDefault -> manual).
	if mode, ok := LatestPersistedAuthorityMode(projectPath, projectUUID); !ok || mode != "banana" {
		t.Fatalf("raw disk value must pass through: got (%q,%t)", mode, ok)
	}
}
