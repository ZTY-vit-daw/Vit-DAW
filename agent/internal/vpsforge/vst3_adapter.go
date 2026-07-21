package vpsforge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
	"vit-daw-agent/internal/vst3host"
)

const VST3HostAdapterProtocol = "vit.vps_host_adapter.v1"

// VST3HostAdapter owns a single crash-isolated native worker. It deliberately
// has no knowledge of a VPS Library, Credential, Catalog, SPAL route, or Vit
// project: all of those authority-bearing concerns remain outside this host.
type VST3HostAdapter struct {
	mu         sync.Mutex
	workerPath string
	worker     *vst3host.Worker
	loaded     bool
	lastError  string
}

func NewVST3HostAdapter(workerPath string) *VST3HostAdapter {
	return &VST3HostAdapter{workerPath: strings.TrimSpace(workerPath)}
}

func (a *VST3HostAdapter) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.worker != nil {
		_ = a.worker.Close()
	}
	a.worker = nil
	a.loaded = false
	return nil
}

func (a *VST3HostAdapter) load(ctx context.Context, request map[string]any) (vst3host.Response, error) {
	if a == nil {
		return vst3host.Response{}, fmt.Errorf("VST3 host adapter is nil")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.worker != nil {
		_ = a.worker.Close()
		a.worker = nil
		a.loaded = false
	}
	worker, err := vst3host.Start(a.workerPath)
	if err != nil {
		a.lastError = err.Error()
		return vst3host.Response{}, err
	}
	response, err := worker.Call(ctx, "load", request)
	if err != nil {
		_ = worker.Close()
		a.lastError = err.Error()
		return vst3host.Response{}, err
	}
	a.worker = worker
	a.loaded = true
	a.lastError = ""
	return response, nil
}

func (a *VST3HostAdapter) call(ctx context.Context, operation string, request map[string]any) (vst3host.Response, error) {
	if a == nil {
		return vst3host.Response{}, fmt.Errorf("VST3 host adapter is nil")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.worker == nil || !a.loaded {
		return vst3host.Response{}, fmt.Errorf("no VST3 plugin is loaded")
	}
	response, err := a.worker.Call(ctx, operation, request)
	if err != nil {
		a.lastError = err.Error()
	}
	if operation == "unload" && err == nil {
		_ = a.worker.Close()
		a.worker = nil
		a.loaded = false
	}
	return response, err
}

func (a *VST3HostAdapter) state() map[string]any {
	if a == nil {
		return map[string]any{"loaded": false}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]any{
		"protocol":    VST3HostAdapterProtocol,
		"loaded":      a.loaded,
		"worker_path": a.workerPath,
		"last_error":  a.lastError,
	}
}

// workerPID is deliberately package-private: preflight experiments may record
// that two adapters used separate native processes, but callers cannot use it
// to manage or signal those workers.
func (a *VST3HostAdapter) workerPID() int {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.worker == nil || !a.loaded {
		return 0
	}
	return a.worker.PID()
}

// Handler exposes a stable, loopback-only caller-facing API. Binding to a
// loopback address is enforced by the CLI; this handler itself has no route
// that can write a VPS Library or issue any Credential.
func (a *VST3HostAdapter) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
			return
		}
		writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "service": "vpsforge-vst3-host-adapter", "state": a.state()})
	})
	mux.HandleFunc("/v1/plugin/load", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		var request map[string]any
		if err := decodeAdapterHTTP(r, &request); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		response, err := a.load(r.Context(), request)
		a.writeWorkerResult(w, "load", response, err, true)
	})
	mux.HandleFunc("/v1/plugin/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
			return
		}
		response, err := a.call(r.Context(), "snapshot", nil)
		if err != nil {
			a.writeWorkerResult(w, "snapshot", response, err, true)
			return
		}
		snapshot, err := NormalizeVST3WorkerSnapshot(response.Result, response.Logs)
		if err != nil {
			writeHTTP(w, http.StatusBadGateway, map[string]any{"status": "error", "error": "normalize native snapshot: " + err.Error()})
			return
		}
		writeHTTP(w, http.StatusOK, snapshot)
	})
	mux.HandleFunc("/v1/plugin/unload", a.operationHandler("unload", true))
	mux.HandleFunc("/v1/plugin/parameters/write", a.operationHandler("write", false))
	mux.HandleFunc("/v1/plugin/state/save", a.operationHandler("save_state", false))
	mux.HandleFunc("/v1/plugin/state/restore", a.operationHandler("restore_state", false))
	mux.HandleFunc("/v1/plugin/state/roundtrip", a.operationHandler("roundtrip_state", false))
	mux.HandleFunc("/v1/plugin/rollback", a.operationHandler("rollback", false))
	mux.HandleFunc("/v1/plugin/render", a.operationHandler("render", false))
	return mux
}

func (a *VST3HostAdapter) operationHandler(operation string, allowEmpty bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		request := map[string]any{}
		if !allowEmpty || r.ContentLength > 0 {
			if err := decodeAdapterHTTP(r, &request); err != nil {
				writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
				return
			}
		}
		response, err := a.call(r.Context(), operation, request)
		a.writeWorkerResult(w, operation, response, err, false)
	}
}

func (a *VST3HostAdapter) writeWorkerResult(w http.ResponseWriter, operation string, response vst3host.Response, err error, snapshot bool) {
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "no VST3 plugin is loaded") {
			status = http.StatusConflict
		}
		payload := map[string]any{"status": "error", "operation": operation, "error": err.Error(), "state": a.state()}
		if workerErr, ok := err.(*vst3host.WorkerError); ok && strings.TrimSpace(workerErr.Diagnostic) != "" {
			payload["worker_diagnostic"] = workerErr.Diagnostic
		}
		writeHTTP(w, status, payload)
		return
	}
	var result any
	if len(response.Result) > 0 {
		if err := json.Unmarshal(response.Result, &result); err != nil {
			writeHTTP(w, http.StatusBadGateway, map[string]any{"status": "error", "operation": operation, "error": "native worker returned invalid result JSON"})
			return
		}
	}
	if snapshot {
		writeHTTP(w, http.StatusOK, result)
		return
	}
	writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "protocol": VST3HostAdapterProtocol, "operation": operation, "result": result, "logs": response.Logs})
}

func decodeAdapterHTTP(r *http.Request, output any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 24<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(output)
}

type workerSnapshot struct {
	Identity struct {
		Manufacturer    string `json:"manufacturer"`
		Name            string `json:"name"`
		Version         string `json:"version"`
		Format          string `json:"format"`
		InstallPath     string `json:"install_path"`
		FileFingerprint string `json:"file_fingerprint"`
		ClassIDs        []struct {
			ClassID       string `json:"class_id"`
			Name          string `json:"name"`
			Vendor        string `json:"vendor"`
			Version       string `json:"version"`
			Category      string `json:"category"`
			Subcategories string `json:"subcategories"`
		} `json:"class_ids"`
	} `json:"identity"`
	Parameters []struct {
		ID             string   `json:"id"`
		IDProvenance   string   `json:"id_provenance"`
		StableID       bool     `json:"stable_id"`
		HostLabel      string   `json:"host_label"`
		Unit           string   `json:"unit"`
		Normalized     float64  `json:"normalized_value"`
		Display        string   `json:"display_value"`
		Default        float64  `json:"default_normalized_value"`
		Automation     string   `json:"automation"`
		Discrete       bool     `json:"is_discrete"`
		Boolean        bool     `json:"is_boolean"`
		Meta           bool     `json:"is_meta"`
		NumSteps       int      `json:"num_steps"`
		Category       int      `json:"category"`
		DisplayChoices []string `json:"display_choices"`
	} `json:"parameters"`
	InputBuses     []map[string]any `json:"input_buses"`
	OutputBuses    []map[string]any `json:"output_buses"`
	LatencySamples int              `json:"latency_samples"`
	TailSeconds    float64          `json:"tail_seconds"`
	SampleRate     float64          `json:"sample_rate"`
	BlockSize      int              `json:"block_size"`
	CapturedAt     time.Time        `json:"captured_at"`
}

// NormalizeVST3WorkerSnapshot turns raw native observations into the existing
// Forge host-probe shape while retaining the raw native snapshot verbatim as
// observed evidence. It performs no semantic inference or credential action.
func NormalizeVST3WorkerSnapshot(raw json.RawMessage, logs []string) (HostSnapshot, error) {
	var source workerSnapshot
	if err := json.Unmarshal(raw, &source); err != nil {
		return HostSnapshot{}, err
	}
	if strings.TrimSpace(source.Identity.Name) == "" || len(source.Parameters) == 0 {
		return HostSnapshot{}, fmt.Errorf("native snapshot has no plugin name or parameters")
	}
	identity := PluginIdentity{
		Manufacturer: source.Identity.Manufacturer,
		Name:         source.Identity.Name,
		Format:       strings.ToUpper(firstNonEmpty(source.Identity.Format, "VST3")),
		Version:      source.Identity.Version,
		InstallPath:  source.Identity.InstallPath,
	}
	descriptors := make([]parameterSurfaceDescriptor, 0, len(source.Parameters))
	parameters := make([]SurfaceParameter, 0, len(source.Parameters))
	allStableIDs := true
	for _, parameter := range source.Parameters {
		if strings.TrimSpace(parameter.ID) == "" || !finiteHostValue(parameter.Normalized) || !finiteHostValue(parameter.Default) {
			return HostSnapshot{}, fmt.Errorf("native snapshot contains an invalid parameter")
		}
		minimum, maximum := 0.0, 1.0
		kind := "continuous"
		if parameter.Boolean {
			kind = "boolean"
		} else if parameter.Discrete {
			// VPS v3 uses the same canonical "enum" type for an observed
			// bounded discrete host parameter everywhere it crosses an
			// authority boundary.  The Vit parameter bridge already emits
			// "enum"; keeping a separate "discrete" spelling here made an
			// identical VST3 surface hash differently in Forge and Vit.
			kind = "enum"
		}
		allStableIDs = allStableIDs && parameter.StableID
		descriptors = append(descriptors, parameterSurfaceDescriptor{
			ID: parameter.ID, Type: kind, Min: &minimum, Max: &maximum,
			EnumValues: parameter.DisplayChoices, DisplayDomain: "normalized_to_host_display", Unit: parameter.Unit, Scale: "normalized",
		})
		current, defaultValue := parameter.Normalized, parameter.Default
		parameters = append(parameters, SurfaceParameter{
			ID:                     parameter.ID,
			Name:                   parameter.HostLabel,
			NormalizedValue:        &current,
			DefaultNormalizedValue: &defaultValue,
			DisplayText:            parameter.Display,
			Unit:                   parameter.Unit,
			HostControllable:       parameter.Automation == "automatable",
			Automation:             parameter.Automation,
			IDProvenance:           parameter.IDProvenance,
			StableID:               parameter.StableID,
			DisplayDomain:          DisplayDomain{Text: parameter.Display, Unit: parameter.Unit, Min: &minimum, Max: &maximum, Scale: "normalized"},
			Observed: map[string]any{
				"is_discrete": parameter.Discrete, "is_boolean": parameter.Boolean, "is_meta": parameter.Meta,
				"num_steps": parameter.NumSteps, "category": parameter.Category, "display_choices": parameter.DisplayChoices,
			},
		})
	}
	if allStableIDs && strings.TrimSpace(source.Identity.FileFingerprint) != "" {
		if fingerprint, err := buildPluginFingerprint(source.Identity.FileFingerprint, descriptors); err == nil {
			identity.Fingerprint = fingerprint
		}
	}
	if identity.Fingerprint.Installation == "" {
		identity.Fingerprint.Installation = source.Identity.FileFingerprint
	}
	var rawObserved any
	_ = json.Unmarshal(raw, &rawObserved)
	surface := SurfaceSnapshot{
		SchemaVersion:  SurfaceSchema,
		PluginIdentity: identity,
		Parameters:     parameters,
		DisplaySurface: map[string]any{"identity": source.Identity, "class_ids": source.Identity.ClassIDs},
		HostCapabilities: map[string]any{
			"adapter_protocol":       VST3HostAdapterProtocol,
			"native_worker_protocol": vst3host.WorkerProtocol,
			"input_buses":            source.InputBuses, "output_buses": source.OutputBuses,
			"latency_samples": source.LatencySamples, "tail_seconds": source.TailSeconds,
			"sample_rate": source.SampleRate, "block_size": source.BlockSize,
			"all_parameter_ids_stable": allStableIDs,
			"raw_observed_snapshot":    rawObserved,
		},
		Logs:       boundedHostLogs(logs),
		CapturedAt: source.CapturedAt.UTC(),
	}
	if surface.CapturedAt.IsZero() {
		surface.CapturedAt = time.Now().UTC()
	}
	return HostSnapshot{Identity: identity, Surface: surface, Logs: boundedHostLogs(logs), AdapterSnapshot: append(json.RawMessage(nil), raw...)}, nil
}

func finiteHostValue(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func boundedHostLogs(values []string) []string {
	const limit = 256
	if len(values) > limit {
		values = values[len(values)-limit:]
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

type parameterSurfaceDescriptor struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Min           *float64 `json:"min,omitempty"`
	Max           *float64 `json:"max,omitempty"`
	EnumValues    []string `json:"enum_values,omitempty"`
	DisplayDomain string   `json:"display_domain,omitempty"`
	Unit          string   `json:"unit,omitempty"`
	Scale         string   `json:"scale,omitempty"`
}

func buildPluginFingerprint(installation string, descriptors []parameterSurfaceDescriptor) (PluginFingerprint, error) {
	if strings.TrimSpace(installation) == "" {
		return PluginFingerprint{}, fmt.Errorf("installation fingerprint is required")
	}
	raw, err := json.Marshal(descriptors)
	if err != nil {
		return PluginFingerprint{}, err
	}
	sum := sha256.Sum256(raw)
	return PluginFingerprint{Installation: installation, ParameterSurface: "sha256:" + hex.EncodeToString(sum[:])}, nil
}
