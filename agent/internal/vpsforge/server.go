package vpsforge

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"vit-daw-agent/internal/fxm"
)

// Handler exposes the staging workspace to a local Codex/tool client. Every
// mutating endpoint is confined to the selected authoring directory; there is
// deliberately no VPS Library installation or Credential endpoint.
func Handler(root string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
			return
		}
		writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "service": "vpsforge", "workspace": root})
	})
	mux.HandleFunc("/v1/workspace", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
			return
		}
		status, err := Inspect(root)
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeHTTP(w, http.StatusOK, status)
	})
	mux.HandleFunc("/v1/surface", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		var snapshot SurfaceSnapshot
		if err := decodeHTTP(r, &snapshot); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		status, err := IngestSurface(root, snapshot, "vpsforge.http", time.Now().UTC())
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeHTTP(w, http.StatusOK, status)
	})
	mux.HandleFunc("/v1/fxm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		var input fxm.Input
		if err := decodeHTTP(r, &input); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		status, err := RecordFXM(root, input, time.Now().UTC())
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeHTTP(w, http.StatusOK, status)
	})
	mux.HandleFunc("/v1/probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		var request struct {
			HostURL string `json:"host_url"`
		}
		if err := decodeHTTP(r, &request); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		status, err := ProbeHost(r.Context(), root, request.HostURL, nil)
		if err != nil {
			writeHTTP(w, http.StatusBadGateway, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeHTTP(w, http.StatusOK, status)
	})
	return mux
}

func decodeHTTP(r *http.Request, output any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(output)
}

func writeHTTP(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
