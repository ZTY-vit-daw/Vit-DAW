package vpsforge

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/vps"
)

func TestDesktopStartsWithoutWorkspaceAndCanInitializeOne(t *testing.T) {
	manager := NewDesktopManager()
	handler := manager.Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Vit VPS Forge") {
		t.Fatalf("desktop page = %d %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "打开实时见证 GUI") || strings.Contains(recorder.Body.String(), "<audio") {
		t.Fatalf("desktop page must launch concurrent live witnessing rather than post-render audio playback: %s", recorder.Body.String())
	}
	root := t.TempDir() + "/workspace"
	body := `{"workspace":` + quoteJSON(root) + `,"manufacturer":"FabFilter","name":"FabFilter Pro-Q","format":"VST3","version":"unknown","capabilities":["equalizer.v2"]}`
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/init", strings.NewReader(body)))
	if recorder.Code != http.StatusOK || manager.Root() != root {
		t.Fatalf("desktop init = %d root=%q body=%s", recorder.Code, manager.Root(), recorder.Body.String())
	}
}

func TestDesktopListsStagingAndStartsOnlyTheObservedWitnessPlugin(t *testing.T) {
	stagingRoot := t.TempDir()
	workspace := filepath.Join(stagingRoot, "observed-plugin")
	identity := vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Observed Plugin", Format: "VST3", Version: "1.0", InstallPath: `C:\plugins\observed.vst3`}
	if _, err := Init(InitRequest{Root: workspace, Identity: identity, Capabilities: []string{"equalizer.v2"}, Now: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := readJSON(filepath.Join(workspace, manifestFile), &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Status = "preflight_complete"
	if err := writeJSON(filepath.Join(workspace, manifestFile), manifest); err != nil {
		t.Fatal(err)
	}

	launchedPath := ""
	manager := NewDesktopManager(DesktopOptions{
		StagingRoot: stagingRoot,
		LaunchWitness: func(pluginPath string) (int, error) {
			launchedPath = pluginPath
			return 4242, nil
		},
	})
	handler := manager.Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/workspaces", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Observed Plugin") {
		t.Fatalf("workspace list = %d %s", recorder.Code, recorder.Body.String())
	}

	body := `{"workspace":` + quoteJSON(workspace) + `}`
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/witness/open", strings.NewReader(body)))
	if recorder.Code != http.StatusOK || launchedPath != identity.InstallPath || !strings.Contains(recorder.Body.String(), "4242") {
		t.Fatalf("witness launch = %d path=%q body=%s", recorder.Code, launchedPath, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/witness/open", strings.NewReader(`{"workspace":`+quoteJSON(t.TempDir())+`}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("outside staging workspace should be rejected, got %d", recorder.Code)
	}
}

func TestDesktopDoesNotExposePostRenderAudioPlayback(t *testing.T) {
	manager := NewDesktopManager(DesktopOptions{StagingRoot: t.TempDir()})
	handler := manager.Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/listening/pack", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("post-render listening endpoint must not be exposed, got %d", recorder.Code)
	}
}

func quoteJSON(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
