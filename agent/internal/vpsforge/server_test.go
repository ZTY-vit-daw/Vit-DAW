package vpsforge

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vit-daw-agent/internal/vps"
)

func TestLocalHandlerExposesWorkspaceWithoutInstallationRoute(t *testing.T) {
	root := t.TempDir() + "/authoring"
	if _, err := Init(InitRequest{Root: root, Identity: vps.PluginIdentity{Name: "Test EQ", Format: "VST3"}, Capabilities: []string{vps.EqualizerCapabilityID}, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	handler := Handler(root)
	request := httptest.NewRequest(http.MethodGet, "/v1/workspace", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("workspace status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	for _, path := range []string{"/v1/install", "/v1/credential", "/v1/catalog"} {
		recorder = httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("unsafe route %s exists: %d", path, recorder.Code)
		}
	}
}
