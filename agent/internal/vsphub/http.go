package vsphub

import (
	"encoding/json"
	"io"
	"net/http"
)

func (h *Hub) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Health())
}

func (h *Hub) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Status())
}

func (h *Hub) handleHTTPVSP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxHTTPBodyBytes))
	if err != nil {
		writeHubResponse(w, ErrorResponse(nil, http.StatusBadRequest, "session", "session.close", "validation_error", "read VSP body: "+err.Error()))
		return
	}
	env, err := ParseEnvelopeBytes(body)
	if err != nil {
		writeHubResponse(w, ErrorResponse(nil, http.StatusBadRequest, "session", "session.close", "validation_error", err.Error()))
		return
	}
	resp := h.Dispatch(r.Context(), env, TransportHTTP)
	writeHubResponse(w, resp)
}

func writeHubResponse(w http.ResponseWriter, resp HubResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Vit-VSP-Transport", TransportHTTP)
	w.WriteHeader(resp.HTTPStatus)
	_, _ = w.Write(resp.Body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
