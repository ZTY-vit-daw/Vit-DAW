package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/shadow"
)

// M3-AUTH-RESTORE-CARRY-1 (10-29 incident, card 223 restart-leg ruling): a
// clean restart (no pending work) seeds the next session from the canonical
// workspace, whose runtime state lags the previous session's last persist, so
// the user's authority selection used to reset to manual. These probes cover
// the full activation path: durable switch -> registry reset (fresh process)
// -> re-activation seeds the new session with the newest persisted mode, and
// the scheduler's disk replay must not revert it.

// bootstrapAuthCarryProject activates the workspace once and switches the
// authority mode through the real control plane (handleAuthorityMode POST,
// including its persist), so the value lands in the previous session's
// durable runtime state exactly the way production writes it.
func bootstrapAuthCarryProject(t *testing.T, projectPath, projectUUID string, modes ...string) (*Server, *shadow.Project) {
	t.Helper()
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	server := New(nil, project, nil)
	for _, mode := range modes {
		resp := authorityPostForTest(server, mode)
		if resp.Code != http.StatusOK {
			t.Fatalf("authority POST %s: status=%d body=%s", mode, resp.Code, resp.Body.String())
		}
		var payload struct {
			Persisted      bool `json:"persisted"`
			WorkspaceReady bool `json:"workspace_ready"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("authority POST %s decode: %v body=%s", mode, err, resp.Body.String())
		}
		if !payload.Persisted || !payload.WorkspaceReady {
			t.Fatalf("authority POST %s not durable: %+v body=%s", mode, payload, resp.Body.String())
		}
	}
	return server, project
}

func readAuthorityFromDurableState(t *testing.T, projectPath, projectUUID string) string {
	t.Helper()
	data, err := history.ReadAgentRuntimeState(projectPath, projectUUID)
	if err != nil {
		t.Fatalf("ReadAgentRuntimeState %s: %v", projectPath, err)
	}
	var row struct {
		AuthorityMode string `json:"authority_mode"`
	}
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatalf("decode runtime state %s: %v", projectPath, err)
	}
	return row.AuthorityMode
}

// Probe 1: full access survives a clean restart — restore adopts the newest
// persisted mode, writes it into the freshly seeded workspace, and the
// scheduler disk replay keeps it.
func TestCleanRestartRestoresDurableAuthorityModeFull(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "carry_full.vit")
	const projectUUID = "vitproj_auth_carry_full"
	_, project := bootstrapAuthCarryProject(t, projectPath, projectUUID, authorityModeFull)

	history.ResetProjectIdentityRegistryForTest()
	restarted := New(nil, project, nil)
	restarted.activateCurrentProjectWorkspace(context.Background())
	if got := restarted.authorityModeSnapshot(); got != authorityModeFull {
		t.Fatalf("authority after clean restart=%q, want %q", got, authorityModeFull)
	}
	// The adopted mode must be durable in the seeded workspace, or the
	// continuation scheduler's replay would revert it within one tick.
	if durable := readAuthorityFromDurableState(t, projectPath, projectUUID); durable != authorityModeFull {
		t.Fatalf("seeded workspace durable authority=%q, want %q", durable, authorityModeFull)
	}
	if err := restarted.reloadActiveRuntimeState(); err != nil {
		t.Fatal(err)
	}
	if got := restarted.authorityModeSnapshot(); got != authorityModeFull {
		t.Fatalf("authority after scheduler replay=%q, want %q (write-back missing)", got, authorityModeFull)
	}
}

// Probe 2 (both directions): the newest persisted mode wins — a full -> manual
// switch before the restart restores manual, not the stale full.
func TestCleanRestartRestoresDurableAuthorityModeManual(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "carry_manual.vit")
	const projectUUID = "vitproj_auth_carry_manual"
	_, project := bootstrapAuthCarryProject(t, projectPath, projectUUID, authorityModeFull, authorityModeManual)

	history.ResetProjectIdentityRegistryForTest()
	restarted := New(nil, project, nil)
	restarted.activateCurrentProjectWorkspace(context.Background())
	if got := restarted.authorityModeSnapshot(); got != authorityModeManual {
		t.Fatalf("authority after clean restart=%q, want %q (newest persisted manual must win)", got, authorityModeManual)
	}
	if durable := readAuthorityFromDurableState(t, projectPath, projectUUID); durable != authorityModeManual {
		t.Fatalf("seeded workspace durable authority=%q, want %q", durable, authorityModeManual)
	}
}

// Probe 3: no persisted authority state at all -> the default manual stands
// (fail-closed; nothing is invented to seed from).
func TestCleanRestartWithoutDurableAuthorityFailsClosedToManual(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "carry_blank.vit")
	const projectUUID = "vitproj_auth_carry_blank"
	if err := os.WriteFile(projectPath, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	// Activate but never persist: no runtime state file anywhere.
	bootstrap := New(nil, project, nil)
	bootstrap.activateCurrentProjectWorkspace(context.Background())

	history.ResetProjectIdentityRegistryForTest()
	restarted := New(nil, project, nil)
	restarted.activateCurrentProjectWorkspace(context.Background())
	if got := restarted.authorityModeSnapshot(); got != authorityModeManual {
		t.Fatalf("authority after clean restart without durable state=%q, want %q", got, authorityModeManual)
	}
}

// Probe 4: per-project isolation across the restart boundary — project A's
// persisted full access never seeds project B's clean restart, and B's
// activation does not rewrite A's durable state (AUTHORITY-LOST-1 zero
// regression on the new seeding path).
func TestCleanRestartAuthoritySeedDoesNotLeakAcrossProjects(t *testing.T) {
	projectA := filepath.Join(t.TempDir(), "carryA.vit")
	projectB := filepath.Join(t.TempDir(), "carryB.vit")
	const uuidA = "vitproj_auth_carry_iso_a"
	const uuidB = "vitproj_auth_carry_iso_b"
	_, project := bootstrapAuthCarryProject(t, projectA, uuidA, authorityModeFull)

	// Project B's clean restart: fresh registry, B never persisted a mode.
	history.ResetProjectIdentityRegistryForTest()
	project.Initialize(map[string]any{"status": "ok", "project_path": projectB, "project_uuid": uuidB})
	serverB := New(nil, project, nil)
	serverB.activateCurrentProjectWorkspace(context.Background())
	if got := serverB.authorityModeSnapshot(); got != authorityModeManual {
		t.Fatalf("project B authority after clean restart=%q, want %q (A's full must not seed B)", got, authorityModeManual)
	}
	// A's newest durable value lives in its session workspace (the registry no
	// longer binds it after the reset), so the chain-wide read is the right
	// probe here: B's activation must not have rewritten any A surface.
	if durable, ok := history.LatestPersistedAuthorityMode(projectA, uuidA); !ok || durable != authorityModeFull {
		t.Fatalf("project A durable authority after B's activation=(%q,%t), want (%q,true)", durable, ok, authorityModeFull)
	}

	// Back to A: its own value still restores.
	history.ResetProjectIdentityRegistryForTest()
	project.Initialize(map[string]any{"status": "ok", "project_path": projectA, "project_uuid": uuidA})
	serverA := New(nil, project, nil)
	serverA.activateCurrentProjectWorkspace(context.Background())
	if got := serverA.authorityModeSnapshot(); got != authorityModeFull {
		t.Fatalf("project A authority after clean restart=%q, want %q (B's activation must not pollute A)", got, authorityModeFull)
	}
}
