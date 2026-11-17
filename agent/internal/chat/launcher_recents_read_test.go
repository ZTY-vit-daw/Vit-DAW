package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLauncherRecentProjectsReadOnly(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "existing.vit")
	if err := os.WriteFile(existing, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.vit")
	for _, entered := range []bool{false, true} {
		t.Run(map[bool]string{false: "startup", true: "entered-project"}[entered], func(t *testing.T) {
			kernel := &launcherFakeKernel{recents: []string{existing, missing, existing}}
			s := newLauncherTestServer(kernel, nil)
			s.userEnteredProject = entered
			mode := s.authorityModeSnapshot()
			path, uuid := s.harness.CurrentProjectIdentity(context.Background())
			rr := httptest.NewRecorder()
			s.handleLauncherRecentProjects(rr, httptest.NewRequest(http.MethodGet, "/agent/launcher/recent-projects", nil))
			if rr.Code != http.StatusOK {
				t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
			}
			var body struct {
				Status string `json:"status"`
				Count  int    `json:"recent_count"`
				Cards  []struct {
					Path      string            `json:"file_path"`
					Exists    bool              `json:"exists"`
					Directive LauncherDirective `json:"launcher_directive"`
				} `json:"project_result_cards"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Status != "ok" || body.Count != 2 || len(body.Cards) != 2 {
				t.Fatalf("unexpected response: %s", rr.Body.String())
			}
			for i, card := range body.Cards {
				if card.Exists != (i == 0) || card.Directive.Action != launcherActionOpen || card.Directive.FilePath != card.Path || !card.Directive.RequiresGovernance {
					t.Fatalf("card contract failed: %+v", card)
				}
			}
			if kernel.calls != 1 {
				t.Fatalf("kernel calls=%d", kernel.calls)
			}
			if s.userEnteredProjectSnapshot() != entered || s.authorityModeSnapshot() != mode {
				t.Fatal("read changed launch/authority state")
			}
			if afterPath, afterUUID := s.harness.CurrentProjectIdentity(context.Background()); afterPath != path || afterUUID != uuid {
				t.Fatal("read activated a workspace")
			}
			// Server has no agent/LLM wired: a chat/LLM dependency would fail this
			// read. Goal and conversation maps must remain untouched as well.
			if len(s.conversationGoals) != 0 || len(s.conversationMemory) != 0 || len(s.goalContinuations) != 0 {
				t.Fatal("read entered chat/goal execution")
			}
			if contents, err := os.ReadFile(existing); err != nil || string(contents) != "fixture" {
				t.Fatal("read modified fixture")
			}
		})
	}
}

func TestLauncherRecentProjectsEmptyErrorAndMethod(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		kernel       *launcherFakeKernel
		code, calls  int
	}{
		{"empty", http.MethodGet, &launcherFakeKernel{}, 200, 1},
		{"unreachable", http.MethodGet, &launcherFakeKernel{err: errors.New("offline")}, 503, 1},
		{"method", http.MethodPost, &launcherFakeKernel{}, 405, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newLauncherTestServer(tc.kernel, nil)
			rr := httptest.NewRecorder()
			s.handleLauncherRecentProjects(rr, httptest.NewRequest(tc.method, "/agent/launcher/recent-projects", nil))
			if rr.Code != tc.code || tc.kernel.calls != tc.calls {
				t.Fatalf("code=%d calls=%d body=%s", rr.Code, tc.kernel.calls, rr.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tc.name == "empty" {
				if cards, ok := body["project_result_cards"].([]any); !ok || len(cards) != 0 {
					t.Fatal("empty must be an array")
				}
			}
			if tc.name == "unreachable" {
				if body["kernel_unavailable"] != true || body["status"] != "error" || body["project_result_cards"] != nil {
					t.Fatal("kernel error disguised as empty")
				}
			}
		})
	}
}
