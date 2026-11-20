package chat

// TUTOR-SIM-1: server-side three-step tutor experiment loop.
//
// This file is intentionally self-contained: the experiment schema/state
// machine lives in internal/tutorsim, while this file owns the three HTTP
// surfaces and the LLM prompt construction. A tutor conversation is handled
// before handleChat creates a goal, pending plan, semantic entry or harness
// node, so it never falls into the ordinary agent loop. Non-tutor
// conversations are untouched: the only added work on that path is a map
// lookup that misses.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/tutorsim"
)

const (
	tutorSimWorkflow = "tutor_sim"

	tutorSimStopRetry            = "tutor_sim_retry"
	tutorSimStopAdvanced         = "tutor_sim_step_advanced"
	tutorSimStopCompleted        = "tutor_sim_completed"
	tutorSimStopAlreadyCompleted = "tutor_sim_already_completed"
	tutorSimStopLLMTransient     = "tutor_sim_llm_transient"
	tutorSimStopLLMError         = "tutor_sim_llm_error"
	tutorSimStopLLMEmptyReply    = "tutor_sim_llm_empty_reply"
	tutorSimStopLLMUnavailable   = "tutor_sim_llm_unavailable"

	tutorSimFailureTransient = "transient"
	tutorSimFailurePermanent = "permanent"
	tutorSimFailureEmpty     = "empty_reply"
	tutorSimFailureConfig    = "config"
)

var errTutorSimEmptyReply = errors.New("tutor_sim: empty LLM reply")

// tutorSimCompleter is the narrow LLM seam. *llm.Client satisfies it; tests
// inject a scripted fake so the state machine and HTTP contract can be checked
// without a real model.
type tutorSimCompleter interface {
	CompleteRequest(context.Context, config.EngineConfig, llm.Request) (llm.Response, error)
}

type tutorSimRuntime struct {
	store      *tutorsim.Store
	completer  tutorSimCompleter
	loadConfig func() (config.EngineConfig, string, error)
	// kernelSendOverride is the test seam for the material load chain
	// (production wraps Server.kernel.SendCommand; the same seam shape as
	// eduJamKernelSendOverride).
	kernelSendOverride func(ctx context.Context, cmd map[string]any) (map[string]any, error)
}

func newTutorSimRuntime() *tutorSimRuntime {
	return &tutorSimRuntime{
		store:      tutorsim.NewStore(),
		loadConfig: config.Load,
	}
}

func (rt *tutorSimRuntime) config() (config.EngineConfig, string, error) {
	if rt == nil || rt.loadConfig == nil {
		return config.EngineConfig{}, "", errors.New("tutor_sim: config loader unavailable")
	}
	return rt.loadConfig()
}

func (rt *tutorSimRuntime) complete(ctx context.Context, s *Server, cfg config.EngineConfig, req llm.Request) (llm.Response, error) {
	if rt == nil {
		return llm.Response{}, errors.New("tutor_sim: runtime unavailable")
	}
	completer := rt.completer
	if completer == nil {
		if s == nil || s.llm == nil {
			return llm.Response{}, errors.New("tutor_sim: llm client unavailable")
		}
		completer = s.llm
	}
	return completer.CompleteRequest(ctx, cfg, req)
}

type tutorSimStartRequest struct {
	ConversationID string `json:"conversation_id"`
	ProjectPath    string `json:"project_path"`
}

type tutorSimStartResponse struct {
	Status         string         `json:"status"`
	Reason         string         `json:"reason,omitempty"`
	Error          string         `json:"error,omitempty"`
	FailureClass   string         `json:"failure_class,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	ExperimentID   string         `json:"experiment_id,omitempty"`
	SchemaVersion  string         `json:"schema_version,omitempty"`
	ProjectPath    string         `json:"project_path,omitempty"`
	SidecarPath    string         `json:"sidecar_path,omitempty"`
	StepIndex      int            `json:"step_index,omitempty"`
	StepTotal      int            `json:"step_total,omitempty"`
	StepID         string         `json:"step_id,omitempty"`
	Reply          string         `json:"reply,omitempty"`
	WorkflowData   map[string]any `json:"workflow_data,omitempty"`
}

type tutorSimStatusResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	ExperimentID   string `json:"experiment_id,omitempty"`
	ProjectPath    string `json:"project_path,omitempty"`
	StepIndex      int    `json:"step_index,omitempty"`
	StepTotal      int    `json:"step_total,omitempty"`
	StepID         string `json:"step_id,omitempty"`
	SessionStatus  string `json:"session_status,omitempty"`
	Completed      bool   `json:"completed"`
	Attempts       int    `json:"attempts,omitempty"`
	Failures       int    `json:"failures,omitempty"`
}

// handleTutorStart implements POST /agent/tutor/start.
func (s *Server) handleTutorStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var req tutorSimStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	projectPath := normalizeTutorSimProjectPath(req.ProjectPath)
	if projectPath == "" {
		projectPath = s.currentTutorSimProjectPath(r.Context())
	}
	if projectPath == "" {
		writeJSON(w, http.StatusOK, tutorSimStartResponse{
			Status: "unavailable",
			Reason: "no_project",
		})
		return
	}
	exp, sidecarPath, err := tutorsim.LoadProject(projectPath)
	if err != nil {
		if errors.Is(err, tutorsim.ErrSidecarMissing) {
			writeJSON(w, http.StatusOK, tutorSimStartResponse{
				Status:      "unavailable",
				Reason:      "sidecar_missing",
				ProjectPath: projectPath,
				SidecarPath: sidecarPath,
			})
			return
		}
		s.logTutorSim("[tutor_sim] start invalid_experiment project=%s sidecar=%s error=%v", projectPath, sidecarPath, err)
		writeJSON(w, http.StatusUnprocessableEntity, tutorSimStartResponse{
			Status:      "error",
			Error:       err.Error(),
			ProjectPath: projectPath,
			SidecarPath: sidecarPath,
		})
		return
	}

	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = "tutor_" + randomID()
	}
	rt := s.tutorSim
	if rt == nil {
		writeJSON(w, http.StatusInternalServerError, tutorSimStartResponse{
			Status: "error",
			Error:  "tutor_sim_runtime_unavailable",
		})
		return
	}
	cfg, cfgPath, cfgErr := rt.config()
	if cfgErr != nil || !cfg.Complete() {
		errText := "llm_config_incomplete"
		if cfgErr != nil {
			errText = cfgErr.Error()
		}
		s.logTutorSim("[tutor_sim] start llm_unavailable conversation=%s config=%s error=%s", conversationID, cfgPath, errText)
		writeJSON(w, http.StatusServiceUnavailable, tutorSimStartResponse{
			Status:         "error",
			Error:          errText,
			FailureClass:   tutorSimFailureConfig,
			ConversationID: conversationID,
			ProjectPath:    projectPath,
			SidecarPath:    sidecarPath,
		})
		return
	}

	llmResp, llmErr := rt.complete(r.Context(), s, cfg, llm.Request{
		Messages:        tutorSimOpenerMessages(exp),
		MaxOutputTokens: 512,
		Metadata: llm.RequestMetadata{
			Source:         "tutor_sim",
			ConversationID: conversationID,
		},
	})
	if llmErr != nil {
		s.writeTutorSimLLMFailure(w, http.StatusBadGateway, conversationID, projectPath, sidecarPath, exp, llmErr)
		return
	}
	reply := strings.TrimSpace(llmResp.Text)
	if reply == "" {
		s.writeTutorSimLLMFailure(w, http.StatusBadGateway, conversationID, projectPath, sidecarPath, exp, errTutorSimEmptyReply)
		return
	}
	snapshot, err := rt.store.Start(conversationID, projectPath, exp)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, tutorSimStartResponse{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}
	s.logTutorSim("[tutor_sim] start conversation=%s experiment=%s project=%s step=%d/%d", conversationID, exp.ID, projectPath, snapshot.StepIndex, snapshot.StepTotal)
	writeJSON(w, http.StatusOK, tutorSimStartResponse{
		Status:         "ok",
		ConversationID: conversationID,
		ExperimentID:   snapshot.ExperimentID,
		SchemaVersion:  exp.SchemaVersion,
		ProjectPath:    projectPath,
		SidecarPath:    sidecarPath,
		StepIndex:      snapshot.StepIndex,
		StepTotal:      snapshot.StepTotal,
		StepID:         snapshot.StepID,
		Reply:          reply,
		WorkflowData: tutorSimStepWorkflowData(exp, snapshot.StepIndex, rt.store.LoadedMaterials(conversationID), map[string]any{
			"schema_version": exp.SchemaVersion,
			"experiment_id":  exp.ID,
			"step_index":     snapshot.StepIndex,
			"step_total":     snapshot.StepTotal,
			"step_id":        snapshot.StepID,
			"passed":         false,
			"completed":      false,
			"attempts":       0,
			"failures":       0,
		}),
	})
}

// handleTutorStatus implements GET /agent/tutor/status.
func (s *Server) handleTutorStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if conversationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "conversation_id is required"})
		return
	}
	rt := s.tutorSim
	if rt == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": "tutor_sim_runtime_unavailable"})
		return
	}
	snapshot, ok := rt.store.Status(conversationID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "unknown_tutor_session"})
		return
	}
	writeJSON(w, http.StatusOK, tutorSimStatusResponse{
		Status:         "ok",
		ConversationID: snapshot.ConversationID,
		ExperimentID:   snapshot.ExperimentID,
		ProjectPath:    snapshot.ProjectPath,
		StepIndex:      snapshot.StepIndex,
		StepTotal:      snapshot.StepTotal,
		StepID:         snapshot.StepID,
		SessionStatus:  string(snapshot.Status),
		Completed:      snapshot.Completed,
		Attempts:       snapshot.Attempts,
		Failures:       snapshot.Failures,
	})
}

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-4（池序 438）：教学材料装载（MIDI clip → 工程）              */
/* ------------------------------------------------------------------ */

// tutorSimDefaultInstrumentIdentifier is the best-effort instrument attached
// to the loaded material track: the Surge XT provenance identifier recorded
// by the 384 (SRC-TEMPLATE) template-sound leg. The kernel may resolve or
// reject it; a rejection is recorded on the material, it never fails the
// track+clip load itself.
const tutorSimDefaultInstrumentIdentifier = "VST3-Surge XT-1e1bcfb-190e4fbd"

// tutorSimLoadMaterialRequest targets one material of the CURRENT step of an
// active tutor session. plugin_path/plugin_identifier are optional overrides
// for the best-effort instrument attach (Surge preset first per the card;
// preset/program selection itself is outside the proven kernel command face
// and is recorded as a known limitation).
type tutorSimLoadMaterialRequest struct {
	ConversationID   string `json:"conversation_id"`
	MaterialTitle    string `json:"material_title"`
	PluginPath       string `json:"plugin_path,omitempty"`
	PluginIdentifier string `json:"plugin_identifier,omitempty"`
}

type tutorSimLoadMaterialResponse struct {
	Status         string         `json:"status"`
	Reason         string         `json:"reason,omitempty"`
	Error          string         `json:"error,omitempty"`
	FailureClass   string         `json:"failure_class,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	AlreadyLoaded  bool           `json:"already_loaded,omitempty"`
	Load           map[string]any `json:"load,omitempty"`
	Material       map[string]any `json:"material,omitempty"`
}

// handleTutorLoadMaterial implements POST /agent/tutor/load_material.
//
// The load uses only the proven kernel command face (add_track /
// rename_track / insert_midi_clip / add_midi_notes — scripts/
// yue_wire_smoke.py W1 — plus a best-effort rack_add_node for the
// instrument). Track+clip placement is the load: a rejected instrument
// attach is recorded, not fatal. The session's reserve/finalize table makes
// repeated loads idempotent.
func (s *Server) handleTutorLoadMaterial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var req tutorSimLoadMaterialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	conversationID := strings.TrimSpace(req.ConversationID)
	title := strings.TrimSpace(req.MaterialTitle)
	if conversationID == "" || title == "" {
		writeJSON(w, http.StatusBadRequest, tutorSimLoadMaterialResponse{
			Status: "error",
			Error:  "conversation_id and material_title are required",
		})
		return
	}
	rt := s.tutorSim
	if rt == nil {
		writeJSON(w, http.StatusInternalServerError, tutorSimLoadMaterialResponse{
			Status: "error",
			Error:  "tutor_sim_runtime_unavailable",
		})
		return
	}

	materials, loaded, sessionOK := rt.store.CurrentMaterials(conversationID)
	if !sessionOK {
		writeJSON(w, http.StatusNotFound, tutorSimLoadMaterialResponse{
			Status:         "error",
			Error:          "unknown_tutor_session",
			ConversationID: conversationID,
		})
		return
	}
	var material *tutorsim.Material
	for i := range materials {
		if materials[i].Title == title {
			material = &materials[i]
			break
		}
	}
	if material == nil {
		writeJSON(w, http.StatusNotFound, tutorSimLoadMaterialResponse{
			Status:         "error",
			Error:          "unknown_material",
			Reason:         "material is not declared on the current step",
			ConversationID: conversationID,
		})
		return
	}
	if state, loadedBefore := loaded[title]; loadedBefore && state.Loaded {
		writeJSON(w, http.StatusOK, tutorSimLoadMaterialResponse{
			Status:         "ok",
			ConversationID: conversationID,
			AlreadyLoaded:  true,
			Load:           tutorSimLoadStatePayload(state),
			Material:       tutorSimMaterialPayload(*material, state),
		})
		return
	}

	reserved, canLoad := rt.store.ReserveMaterial(conversationID, title)
	if !canLoad {
		// Either a load is in flight or the concurrent twin just finished.
		if reserved.Loaded {
			writeJSON(w, http.StatusOK, tutorSimLoadMaterialResponse{
				Status:         "ok",
				ConversationID: conversationID,
				AlreadyLoaded:  true,
				Load:           tutorSimLoadStatePayload(reserved),
				Material:       tutorSimMaterialPayload(*material, reserved),
			})
			return
		}
		writeJSON(w, http.StatusConflict, tutorSimLoadMaterialResponse{
			Status:         "error",
			Error:          "load_in_progress",
			ConversationID: conversationID,
			Material:       tutorSimMaterialPayload(*material, reserved),
		})
		return
	}

	fail := func(status int, failureClass, reason, errText string) {
		rt.store.FinalizeMaterial(conversationID, title, tutorsim.MaterialLoadState{Error: errText})
		state := rt.store.LoadedMaterials(conversationID)[title]
		s.logTutorSim("[tutor_sim] load_material refused conversation=%s title=%q reason=%s error=%s", conversationID, title, reason, errText)
		writeJSON(w, status, tutorSimLoadMaterialResponse{
			Status:         "error",
			Reason:         reason,
			Error:          errText,
			FailureClass:   failureClass,
			ConversationID: conversationID,
			Material:       tutorSimMaterialPayload(*material, state),
		})
	}

	if material.Kind == tutorsim.MaterialKindAudio {
		fail(http.StatusUnprocessableEntity, "audio_not_loadable", "audio materials are a schema slot without loadable content this round", "audio material cannot be loaded")
		return
	}
	if material.SummaryError != "" || material.DurationBeats <= 0 {
		fail(http.StatusUnprocessableEntity, "material_unavailable", "material file could not be summarized", firstNonEmpty(material.SummaryError, "material summary unavailable"))
		return
	}

	send := rt.kernelSendOverride
	if send == nil {
		if s.kernel == nil {
			fail(http.StatusServiceUnavailable, "kernel_unavailable", "kernel client is nil", "kernel unavailable")
			return
		}
		send = func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
			reply, _, err := s.kernel.SendCommand(ctx, cmd)
			return reply, err
		}
	}

	midiData, err := os.ReadFile(material.AbsPath)
	if err != nil {
		fail(http.StatusUnprocessableEntity, "material_unavailable", "material file unreadable at load time", err.Error())
		return
	}
	doc, err := tutorsim.ParseMidi(midiData)
	if err != nil || len(doc.Notes) == 0 {
		text := err.Error()
		if text == "" {
			text = "material parsed to zero notes"
		}
		fail(http.StatusUnprocessableEntity, "material_unavailable", "material file unparseable at load time", text)
		return
	}

	result, err := tutorSimLoadMidiMaterial(r.Context(), send, material.Title, material.DurationBeats, doc.Notes, req.PluginPath, req.PluginIdentifier)
	if err != nil {
		state := tutorsim.MaterialLoadState{Error: err.Error()}
		if result != nil {
			state.TrackID = result.TrackID
			state.ClipID = result.ClipID
		}
		rt.store.FinalizeMaterial(conversationID, title, state)
		s.logTutorSim("[tutor_sim] load_material kernel_failed conversation=%s title=%q error=%v", conversationID, title, err)
		writeJSON(w, http.StatusBadGateway, tutorSimLoadMaterialResponse{
			Status:         "error",
			Error:          err.Error(),
			FailureClass:   "kernel_error",
			ConversationID: conversationID,
			Load:           tutorSimLoadStatePayload(state),
			Material:       tutorSimMaterialPayload(*material, state),
		})
		return
	}
	final := tutorsim.MaterialLoadState{
		Loaded:             true,
		TrackID:            result.TrackID,
		ClipID:             result.ClipID,
		PluginID:           result.PluginID,
		InstrumentAttached: result.InstrumentAttached,
		InstrumentError:    result.InstrumentError,
	}
	rt.store.FinalizeMaterial(conversationID, title, final)
	s.logTutorSim("[tutor_sim] load_material conversation=%s title=%q track=%s clip=%s instrument_attached=%t", conversationID, title, result.TrackID, result.ClipID, result.InstrumentAttached)
	writeJSON(w, http.StatusOK, tutorSimLoadMaterialResponse{
		Status:         "ok",
		ConversationID: conversationID,
		Load:           tutorSimLoadStatePayload(final),
		Material:       tutorSimMaterialPayload(*material, final),
	})
}

func tutorSimLoadStatePayload(state tutorsim.MaterialLoadState) map[string]any {
	return tutorSimMaterialPayload(tutorsim.Material{}, state)
}

type tutorSimLoadResult struct {
	TrackID            string
	ClipID             string
	PluginID           string
	InstrumentAttached bool
	InstrumentError    string
}

// tutorSimKernelReplyStatus mirrors the W1 acceptance per command: add_track
// may answer ok|success; the clip/notes faces answer strict ok.
func tutorSimKernelReplyStatus(reply map[string]any) string {
	if reply == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(fmt.Sprint(reply["status"])))
}

func tutorSimKernelReplyError(reply map[string]any) string {
	return firstNonEmpty(
		strings.TrimSpace(fmt.Sprint(reply["error"])),
		strings.TrimSpace(fmt.Sprint(reply["message"])),
		"unknown kernel error",
	)
}

// tutorSimLoadMidiMaterial places the material as track+clip with its notes
// using the proven kernel command chain, then attempts the best-effort
// instrument attach. A partially completed chain returns an error carrying
// the evidence ids created so far (no invented compensation commands).
func tutorSimLoadMidiMaterial(
	ctx context.Context,
	send func(ctx context.Context, cmd map[string]any) (map[string]any, error),
	title string,
	durationBeats float64,
	notes []tutorsim.MidiNote,
	pluginPath, pluginIdentifier string,
) (*tutorSimLoadResult, error) {
	result := &tutorSimLoadResult{}

	reply, err := send(ctx, map[string]any{"cmd": "add_track", "command_timeout_ms": 60000})
	if err != nil {
		return result, fmt.Errorf("add_track: %w", err)
	}
	if status := tutorSimKernelReplyStatus(reply); status != "ok" && status != "success" {
		return result, fmt.Errorf("add_track failed: %s", tutorSimKernelReplyError(reply))
	}
	result.TrackID = strings.TrimSpace(fmt.Sprint(reply["track_id"]))
	if result.TrackID == "" {
		return result, fmt.Errorf("add_track returned no track_id")
	}

	reply, err = send(ctx, map[string]any{"cmd": "rename_track", "track_id": result.TrackID, "name": title, "command_timeout_ms": 60000})
	if err != nil {
		return result, fmt.Errorf("rename_track: %w", err)
	}
	if status := tutorSimKernelReplyStatus(reply); status != "ok" && status != "success" {
		return result, fmt.Errorf("rename_track failed: %s", tutorSimKernelReplyError(reply))
	}

	reply, err = send(ctx, map[string]any{
		"cmd": "insert_midi_clip", "track_id": result.TrackID,
		"time_unit": "beats", "start": 0.0, "length": durationBeats,
		"command_timeout_ms": 60000,
	})
	if err != nil {
		return result, fmt.Errorf("insert_midi_clip: %w", err)
	}
	if status := tutorSimKernelReplyStatus(reply); status != "ok" {
		return result, fmt.Errorf("insert_midi_clip failed: %s", tutorSimKernelReplyError(reply))
	}
	result.ClipID = strings.TrimSpace(fmt.Sprint(reply["clip_id"]))
	if result.ClipID == "" {
		result.ClipID = strings.TrimSpace(fmt.Sprint(reply["new_clip_id"]))
	}
	if result.ClipID == "" {
		return result, fmt.Errorf("insert_midi_clip returned no clip_id")
	}

	rows := make([]map[string]any, 0, len(notes))
	for _, note := range notes {
		rows = append(rows, map[string]any{
			"pitch":    note.Pitch,
			"start":    note.Start,
			"length":   note.Length,
			"velocity": note.Velocity,
		})
	}
	reply, err = send(ctx, map[string]any{
		"cmd": "add_midi_notes", "track_id": result.TrackID, "clip_id": result.ClipID,
		"time_unit": "beats", "notes": rows, "command_timeout_ms": 60000,
	})
	if err != nil {
		return result, fmt.Errorf("add_midi_notes: %w", err)
	}
	if status := tutorSimKernelReplyStatus(reply); status != "ok" {
		return result, fmt.Errorf("add_midi_notes failed: %s", tutorSimKernelReplyError(reply))
	}

	// Best-effort instrument attach (Surge 预设优先，384 腿 D 模板音色锚点).
	// Rejection is recorded, never fatal: the material (track+clip+notes) is
	// already in place.
	identifier := strings.TrimSpace(pluginIdentifier)
	if identifier == "" {
		identifier = tutorSimDefaultInstrumentIdentifier
	}
	instrumentCmd := map[string]any{
		"cmd":                "rack_add_node",
		"track_id":           result.TrackID,
		"plugin_identifier":  identifier,
		"plugin_kind":        "Instrument",
		"auto_connect":       true,
		"x":                  0.32,
		"y":                  0.32,
		"command_timeout_ms": 60000,
	}
	if path := strings.TrimSpace(pluginPath); path != "" {
		instrumentCmd["plugin_path"] = path
	}
	reply, err = send(ctx, instrumentCmd)
	if err != nil {
		result.InstrumentError = err.Error()
		return result, nil
	}
	if status := tutorSimKernelReplyStatus(reply); status != "ok" && status != "success" {
		result.InstrumentError = tutorSimKernelReplyError(reply)
		return result, nil
	}
	result.InstrumentAttached = true
	result.PluginID = strings.TrimSpace(fmt.Sprint(reply["plugin_id"]))
	if result.PluginID == "" {
		result.PluginID = strings.TrimSpace(fmt.Sprint(reply["plugin_item_id"]))
	}
	return result, nil
}

// handleTutorSimChatTurn intercepts POST /agent/chat for a conversation that
// belongs to an in-memory tutor session. It returns true when the tutor loop
// handled the request (and wrote the response); false leaves the ordinary chat
// path completely intact.
func (s *Server) handleTutorSimChatTurn(w http.ResponseWriter, r *http.Request, conversationID, message string) bool {
	if s == nil {
		return false
	}
	rt := s.tutorSim
	if rt == nil {
		return false
	}
	verdict, exp, ok := rt.store.Submit(conversationID, message)
	if !ok {
		return false
	}
	resp := ChatResponse{
		ConversationID: conversationID,
		Workflow:       tutorSimWorkflow,
	}
	data := tutorSimVerdictData(exp, verdict)
	if verdict.AlreadyCompleted {
		resp.Reply = "本实验已完成，不再有新的步骤。如需重做，请重新开始实验。"
		resp.WorkflowData = data
		resp.StopReason = tutorSimStopAlreadyCompleted
		resp.GoalStatus = string(agentruntime.StatusCompleted)
		writeJSON(w, http.StatusOK, resp)
		return true
	}

	cfg, cfgPath, cfgErr := rt.config()
	if cfgErr != nil || !cfg.Complete() {
		errText := "llm_config_incomplete"
		if cfgErr != nil {
			errText = cfgErr.Error()
		}
		resp = tutorSimFailureResponse(resp, data, tutorSimFailureConfig, tutorSimStopLLMUnavailable, errText, verdict)
		resp.Error = errText
		s.logTutorSim("[tutor_sim] turn llm_unavailable conversation=%s config=%s error=%s", conversationID, cfgPath, errText)
		writeJSON(w, http.StatusOK, resp)
		return true
	}

	llmResp, llmErr := rt.complete(r.Context(), s, cfg, llm.Request{
		Messages:        tutorSimTurnMessages(exp, verdict, message),
		MaxOutputTokens: 512,
		Metadata: llm.RequestMetadata{
			Source:         "tutor_sim",
			ConversationID: conversationID,
		},
	})
	if llmErr != nil {
		class := tutorSimFailureClass(llmErr)
		resp = tutorSimFailureResponse(resp, data, class, tutorSimStopReasonForFailure(class), llmErr.Error(), verdict)
		resp.Error = llmErr.Error()
		s.logTutorSim("[tutor_sim] turn llm_failure conversation=%s class=%s step=%d/%d completed=%t error=%v", conversationID, class, verdict.StepIndex, verdict.StepTotal, verdict.Completed, llmErr)
		writeJSON(w, http.StatusOK, resp)
		return true
	}
	reply := strings.TrimSpace(llmResp.Text)
	if reply == "" {
		class := tutorSimFailureEmpty
		resp = tutorSimFailureResponse(resp, data, class, tutorSimStopReasonForFailure(class), errTutorSimEmptyReply.Error(), verdict)
		resp.Error = errTutorSimEmptyReply.Error()
		s.logTutorSim("[tutor_sim] turn empty_reply conversation=%s step=%d/%d", conversationID, verdict.StepIndex, verdict.StepTotal)
		writeJSON(w, http.StatusOK, resp)
		return true
	}
	resp.Reply = reply
	resp.WorkflowData = data
	resp.StopReason = tutorSimStopReasonForVerdict(verdict)
	if verdict.Completed {
		resp.GoalStatus = string(agentruntime.StatusCompleted)
	}
	s.logTutorSim("[tutor_sim] turn conversation=%s step=%d/%d passed=%t completed=%t stop=%s", conversationID, verdict.StepIndex, verdict.StepTotal, verdict.Passed, verdict.Completed, resp.StopReason)
	writeJSON(w, http.StatusOK, resp)
	return true
}

func (s *Server) writeTutorSimLLMFailure(w http.ResponseWriter, status int, conversationID, projectPath, sidecarPath string, exp *tutorsim.Experiment, err error) {
	class := tutorSimFailureClass(err)
	s.logTutorSim("[tutor_sim] start llm_failure conversation=%s class=%s experiment=%s error=%v", conversationID, class, exp.ID, err)
	writeJSON(w, status, tutorSimStartResponse{
		Status:         "error",
		Error:          err.Error(),
		FailureClass:   class,
		ConversationID: conversationID,
		ExperimentID:   exp.ID,
		SchemaVersion:  exp.SchemaVersion,
		ProjectPath:    projectPath,
		SidecarPath:    sidecarPath,
	})
}

func tutorSimFailureClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, errTutorSimEmptyReply):
		return tutorSimFailureEmpty
	// Reuse the package-wide transient-service classifier so tutor failures use
	// the same marker set as the free-state loop.
	case freeStateTransientServiceError(err.Error()):
		return tutorSimFailureTransient
	default:
		return tutorSimFailurePermanent
	}
}

func tutorSimStopReasonForFailure(class string) string {
	switch class {
	case tutorSimFailureTransient:
		return tutorSimStopLLMTransient
	case tutorSimFailureEmpty:
		return tutorSimStopLLMEmptyReply
	default:
		return tutorSimStopLLMError
	}
}

func tutorSimStopReasonForVerdict(verdict tutorsim.Verdict) string {
	switch {
	case verdict.Completed:
		return tutorSimStopCompleted
	case verdict.Passed:
		return tutorSimStopAdvanced
	default:
		return tutorSimStopRetry
	}
}

func tutorSimVerdictData(exp *tutorsim.Experiment, verdict tutorsim.Verdict) map[string]any {
	return tutorSimStepWorkflowData(exp, verdict.StepIndex, verdict.MaterialsLoaded, map[string]any{
		"schema_version": exp.SchemaVersion,
		"experiment_id":  exp.ID,
		"step_index":     verdict.StepIndex,
		"step_total":     verdict.StepTotal,
		"step_id":        verdict.StepID,
		"passed":         verdict.Passed,
		"completed":      verdict.Completed,
		"attempts":       verdict.Attempts,
		"failures":       verdict.Failures,
	})
}

// tutorSimStepWorkflowData augments the structured verdict fields with the
// current step's teaching payload (TUTOR-SIM-4 v1): brief prose and the
// material list annotated with the session's load state. v0 experiments
// carry an empty brief and an empty (non-null) materials list so the UI can
// key on presence without schema sniffing.
func tutorSimStepWorkflowData(exp *tutorsim.Experiment, stepIndex int, loaded map[string]tutorsim.MaterialLoadState, data map[string]any) map[string]any {
	if exp == nil || stepIndex < 1 || stepIndex > len(exp.Steps) {
		return data
	}
	step := exp.Steps[stepIndex-1]
	data["brief"] = step.Brief
	materials := make([]map[string]any, 0, len(step.Materials))
	for _, material := range step.Materials {
		materials = append(materials, tutorSimMaterialPayload(material, loaded[material.Title]))
	}
	data["materials"] = materials
	return data
}

// tutorSimMaterialPayload is the workflow_data/material-response projection of
// one material. The declared YAML path is echoed (never the absolute host
// path); the summary is computed at experiment load time.
func tutorSimMaterialPayload(material tutorsim.Material, state tutorsim.MaterialLoadState) map[string]any {
	payload := map[string]any{
		"kind":    material.Kind,
		"title":   material.Title,
		"path":    material.Path,
		"loaded":  state.Loaded,
		"loading": state.Loading,
	}
	if material.NoteCount > 0 {
		payload["note_count"] = material.NoteCount
		payload["duration_beats"] = material.DurationBeats
		payload["tempo_bpm"] = material.TempoBPM
	}
	if material.SummaryError != "" {
		payload["summary_error"] = material.SummaryError
	}
	if state.TrackID != "" {
		payload["track_id"] = state.TrackID
	}
	if state.ClipID != "" {
		payload["clip_id"] = state.ClipID
	}
	if state.PluginID != "" {
		payload["plugin_id"] = state.PluginID
	}
	payload["instrument_attached"] = state.InstrumentAttached
	if state.InstrumentError != "" {
		payload["instrument_error"] = state.InstrumentError
	}
	if state.Error != "" {
		payload["load_error"] = state.Error
	}
	return payload
}

func tutorSimFailureResponse(base ChatResponse, data map[string]any, class, stopReason, errText string, verdict tutorsim.Verdict) ChatResponse {
	data["failure_class"] = class
	data["failure"] = errText
	base.Reply = tutorSimFailureReply(verdict, class)
	base.WorkflowData = data
	base.StopReason = stopReason
	if verdict.Completed {
		base.GoalStatus = string(agentruntime.StatusCompleted)
	}
	return base
}

// tutorSimFailureReply is an explicit failure disclosure, never a fabricated
// model reply. The server-side verdict recorded in workflow_data stays
// authoritative.
func tutorSimFailureReply(verdict tutorsim.Verdict, class string) string {
	state := fmt.Sprintf("第 %d/%d 步", verdict.StepIndex, verdict.StepTotal)
	switch {
	case verdict.Completed:
		return fmt.Sprintf("导师回复生成失败（%s）；服务器已记录实验完成。", class)
	case verdict.Passed:
		return fmt.Sprintf("导师回复生成失败（%s）；服务器已记录本步通过，当前%s。", class, state)
	default:
		return fmt.Sprintf("导师回复生成失败（%s）；本次输入未通过，仍停留在%s。", class, state)
	}
}

func tutorSimSystemPrompt(exp *tutorsim.Experiment) string {
	return strings.TrimSpace(fmt.Sprintf(`你是 Vit-DAW 导师实验里的导师。
人设：
%s

规则：
- 只围绕当前步骤引导，不替用户回答，不透露任何步骤的答案。
- 服务器独立校验用户输入；只有事件明确给出“通过/未通过”时，你才可以复述该判定。
- 回复简短（1-3 句），使用中文，不输出命令、JSON 或代码块。`, exp.Persona))
}

func tutorSimOpenerMessages(exp *tutorsim.Experiment) []llm.Message {
	total := len(exp.Steps)
	userPrompt := fmt.Sprintf(`实验 %s 开始，共 %d 步。
第 1 步任务：%s
请用你的人设开场，说明当前是第 1/%d 步，并邀请用户完成这一步。不要替用户回答，不要透露答案，不要提及服务器校验。`,
		exp.ID, total, exp.Steps[0].Ask, total)
	return []llm.Message{
		{Role: "system", Content: tutorSimSystemPrompt(exp)},
		{Role: "user", Content: userPrompt},
	}
}

func tutorSimTurnMessages(exp *tutorsim.Experiment, verdict tutorsim.Verdict, userMessage string) []llm.Message {
	var userPrompt string
	userText := strings.TrimSpace(userMessage)
	switch {
	case verdict.Completed:
		userPrompt = fmt.Sprintf(`用户输入：%s
服务器判定：通过（已完成第 %d/%d 步），实验完成。
通过反馈素材：%s
请简短收尾并祝贺用户；不要再给出任务或答案。`,
			userText, verdict.AnsweredStepIndex, verdict.StepTotal, verdict.AnsweredStep.Pass)
	case verdict.Passed:
		next := exp.Steps[verdict.StepIndex-1]
		userPrompt = fmt.Sprintf(`用户输入：%s
服务器判定：通过（已完成第 %d/%d 步）。
通过反馈素材：%s
下一步任务（第 %d/%d 步）：%s
请简短确认通过，并给出下一步任务；不要透露任何答案，不要提前展开更后面的步骤。`,
			userText, verdict.AnsweredStepIndex, verdict.StepTotal, verdict.AnsweredStep.Pass,
			verdict.StepIndex, verdict.StepTotal, next.Ask)
	default:
		userPrompt = fmt.Sprintf(`用户输入：%s
服务器判定：未通过，停留在第 %d/%d 步。
当前步骤任务：%s
重试引导素材：%s
请按重试引导回应用户；不要透露正确答案，不要进入下一步。`,
			userText, verdict.StepIndex, verdict.StepTotal, verdict.AnsweredStep.Ask, verdict.AnsweredStep.Retry)
	}
	return []llm.Message{
		{Role: "system", Content: tutorSimSystemPrompt(exp)},
		{Role: "user", Content: userPrompt},
	}
}

func normalizeTutorSimProjectPath(projectPath string) string {
	path := strings.TrimSpace(projectPath)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func (s *Server) currentTutorSimProjectPath(ctx context.Context) string {
	if s == nil {
		return ""
	}
	if s.harness != nil {
		if path, _ := s.harness.CurrentProjectIdentity(ctx); strings.TrimSpace(path) != "" {
			return normalizeTutorSimProjectPath(path)
		}
	}
	return normalizeTutorSimProjectPath(s.activeWorkspacePathSnapshotForUI())
}

func (s *Server) logTutorSim(format string, args ...any) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Info(format, args...)
}
