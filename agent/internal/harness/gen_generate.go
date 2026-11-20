package harness

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"vit-daw-agent/internal/genapi"
	"vit-daw-agent/internal/genctx"
	"vit-daw-agent/internal/tools"
)

// genGenerate implements the gen.generate local tool: config-driven
// generation through the unified genapi adapter template, with artifacts
// landing on the timeline via the EXISTING GeneratedAssetService ingest
// path (assets.register_job + assets.ingest_generated_asset) — no new
// kernel surface (GEN-API-ADAPTER-1 file domain: zero kernel changes).
func (h *Harness) genGenerate(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	providerID := firstString(cmd, "provider_id", "provider")
	if providerID == "" {
		return nil, fmt.Errorf("gen.generate needs provider_id (see gen.providers for configured entries)")
	}
	provider, configPath, err := genapi.FindProvider(providerID)
	if err != nil {
		gerr := genapi.AsError(err)
		payload := gerr.ResultPayload()
		payload["config_path"] = configPath
		payload["hint"] = "fill provider/base_url/api_key/model in the genapi provider config (example in agent/internal/genapi/config.go)"
		return map[string]any{"status": "error", "error": gerr.Error(), "result": payload}, gerr
	}

	// GEN-CTX-CONSUME-1: render the prompt through the genctx template engine
	// (profile=gen_api) after the local config check and before the provider
	// is contacted — scene problems / missing project truth fail closed with
	// no API round-trip.
	renderedPrompt, scene, promptEvidence, rerr := h.genRenderPrompt(ctx, cmd)
	if rerr != nil {
		return map[string]any{"status": "error", "error": rerr.Error()}, rerr
	}
	req := genapi.GenerationRequest{
		Prompt:          renderedPrompt,
		Lyrics:          firstString(cmd, "lyrics"),
		StyleTags:       firstString(cmd, "style_tags", "tags", "style"),
		Title:           firstString(cmd, "title"),
		Instrumental:    boolValueDefault(cmd["instrumental"], false),
		DurationSeconds: floatValueFromMap(cmd, "duration_seconds", "duration"),
		TempoBPM:        floatValueFromMap(cmd, "tempo_bpm", "tempo"),
		MusicalKey:      firstString(cmd, "musical_key", "key"),
		Count:           intValueFromMap(cmd, "count"),
		ABC:             firstString(cmd, "abc", "abc_notation", "score_abc"),
	}

	// GEN-YUE-WIRE-1: the yue_local path applies the deterministic
	// conditioning map (student score conversion, prompt->style mapping,
	// instrumental score resting, dispatched summary) and ensures the local
	// engine is up before any request leaves the process.
	var (
		prepared     *genapi.YuePrepareResult
		engineHandle *genapi.EngineHandle
		scoreInput   *genapi.YueScoreInput
	)
	if provider.Dialect == "yue_local" {
		prepared, engineHandle, scoreInput, rerr = h.genYuePrepareLocal(ctx, provider, cmd, req, promptEvidence)
		if rerr != nil {
			gerr := genapi.AsError(rerr)
			return map[string]any{"status": "error", "error": gerr.Error(), "hint": "yue_local conditioning failed before submit; nothing was sent to the engine"}, gerr
		}
		req = prepared.Request
	}

	limitations := provider.Limitations(req)

	var twoStage *genapi.YueTwoStageResult
	var result *genapi.Result
	if provider.Dialect == "yue_local" && req.Instrumental {
		// YUE-INSTRUMENTAL-SCORE-TRANSFER-1 leg B: instrumental yue_local
		// requests run the two-stage score-transfer route — pass1 generation
		// -> Go Vocal->Ins score transfer -> pass2 fixed-abc re-render (two
		// engine jobs, double cost disclosed; final WAV/version from pass2).
		// Each stage carries its own provider timeout budget (Generate
		// applies cfg.TimeoutSeconds per call), so the two stages are NOT
		// wrapped in a single per-request deadline that could kill pass2
		// mid-render.
		twoStage, err = genapi.YueGenerateInstrumentalTwoStage(ctx, provider, req, genapi.YueTwoStageOptions{
			LyricsUserProvided: prepared != nil && prepared.LyricsUserProvided,
		})
		if err != nil {
			gerr := genapi.AsError(err)
			payload := gerr.ResultPayload()
			payload["provider_id"] = providerID
			return map[string]any{"status": "error", "error": gerr.Error(), "result": payload}, gerr
		}
		result = twoStage.Final()
	} else {
		genCtx, cancel := context.WithTimeout(ctx, provider.Timeout())
		defer cancel()
		result, err = genapi.Generate(genCtx, provider, req)
		if err != nil {
			gerr := genapi.AsError(err)
			payload := gerr.ResultPayload()
			payload["provider_id"] = providerID
			return map[string]any{"status": "error", "error": gerr.Error(), "result": payload}, gerr
		}
	}

	out := map[string]any{
		"status":      "ok",
		"provider_id": provider.ID,
		"dialect":     result.Dialect,
		"model":       result.Model,
		"task_id":     result.TaskID,
		"artifacts":   artifactPayloads(result.Artifacts),
	}
	if len(limitations) > 0 {
		out["limitations"] = limitations
	}
	if len(result.StatusSeen) > 0 {
		out["status_seen"] = result.StatusSeen
	}

	out["scene"] = string(scene)
	out["prompt_context"] = promptEvidence

	// GEN-YUE-WIRE-1 disclosure face: what actually got dispatched to the
	// engine (audit H6 items 2-4) plus the engine supervision state.
	if prepared != nil {
		out["dispatched"] = prepared.Dispatched
		if len(prepared.Actions) > 0 {
			out["conditioning_actions"] = prepared.Actions
		}
	}
	if twoStage != nil {
		// YUE-INSTRUMENTAL-SCORE-TRANSFER-1: job archive face — both job ids,
		// the vocal/ins conservation receipt and the pass2 request口径.
		out["two_stage"] = twoStage.ReceiptMap(provider)
	}
	if engineHandle != nil {
		out["engine"] = map[string]any{
			"external":       engineHandle.External,
			"base_url":       engineHandle.BaseURL,
			"pid":            engineHandle.PID,
			"started_command": engineHandle.Command,
			"ready_after_ms": engineHandle.ReadyAfter.Milliseconds(),
		}
	}
	if scoreInput != nil {
		// Full note list: this is the version-archive face (the original
		// student MIDI truth the arrangement was generated from).
		notes := make([]any, 0, len(scoreInput.Notes))
		for _, n := range scoreInput.Notes {
			notes = append(notes, map[string]any{"pitch": n.Pitch, "start": n.Start, "length": n.Length})
		}
		out["source_score"] = map[string]any{
			"notes":          notes,
			"time_signature": fmt.Sprintf("%d/%d", scoreInput.Numerator, scoreInput.Denominator),
			"tempo_bpm":      scoreInput.TempoBPM,
			"key":            scoreInput.Key,
		}
	}

	trackID := firstString(cmd, "track_id", "target_track_id")
	if trackID == "" {
		out["ingest"] = "skipped: pass track_id to land artifacts on the timeline (artifacts are on disk)"
		return out, nil
	}

	// Write-front sentinel (design doc 04 §3): exempt scenes produce zero
	// project writes — refuse the ingest leg explicitly instead of letting
	// the landing criterion be violated silently.
	if serr := genctx.GuardWriteCommand(scene, "bridge_ingest_generated_asset"); serr != nil {
		out["ingest"] = "refused: " + serr.Error()
		out["status"] = "error"
		out["error"] = serr.Error()
		out["hint"] = "artifacts are on disk under the genapi cache (legal free_creation product face); import manually if this was a mistake"
		return out, serr
	}

	ingested, err := h.genIngestArtifacts(ctx, provider, result, cmd, trackID)
	out["ingest"] = ingested
	if err != nil {
		out["status"] = "error"
		out["error"] = err.Error()
		out["hint"] = "artifacts remain on disk; retry with assets.ingest_generated_asset after fixing the reported failure"
		return out, err
	}
	return out, nil
}

// genIngestArtifacts registers one AIGC job and ingests every artifact as a
// candidate take on one shared take stack (the multi-candidate substrate
// proven by GEN-MAC-BASELINE-1). Kernel failures fail closed: the error
// surfaces with the artifact paths so nothing is silently dropped.
func (h *Harness) genIngestArtifacts(ctx context.Context, provider genapi.ProviderConfig, result *genapi.Result, cmd map[string]any, trackID string) ([]map[string]any, error) {
	jobID := sanitizeJobToken(fmt.Sprintf("genapi-%s-%s", provider.ID, result.TaskID))
	if jobID == "" {
		jobID = fmt.Sprintf("genapi-%s-%d", provider.ID, time.Now().Unix())
	}
	registerCmd := tools.BuildCommand("aigc_register_job", map[string]any{
		"job_id":         jobID,
		"job_state":      "complete",
		"status_message": fmt.Sprintf("genapi %s (%s) produced %d artifact(s)", provider.ID, provider.Dialect, len(result.Artifacts)),
		"source":         "genapi:" + provider.ID,
	})
	registerReply, _, err := h.KernelSendCommand(ctx, registerCmd)
	if err != nil || !kernelReplySucceeded(registerReply) {
		return nil, fmt.Errorf("aigc_register_job failed for job %s: err=%v reply=%v", jobID, err, registerReply)
	}

	takeStack := firstString(cmd, "take_stack_id")
	if takeStack == "" {
		takeStack = "genapi-" + sanitizeJobToken(result.TaskID)
	}
	if takeStack == "" {
		takeStack = jobID
	}
	startTime := floatValueFromMap(cmd, "start_time", "start_seconds")

	var ingested []map[string]any
	for i, artifact := range result.Artifacts {
		// Only audio artifacts ride the timeline; sidecars like the YuE2
		// replay JSON (kind "replay") stay on disk as evidence.
		if artifact.Kind != "audio" {
			continue
		}
		absPath := artifact.LocalPath
		if expanded, err := filepath.Abs(absPath); err == nil {
			absPath = expanded
		}
		ingestCmd := tools.BuildCommand("bridge_ingest_generated_asset", map[string]any{
			"file_path":     absPath,
			"track_id":      trackID,
			"start_time":    startTime + float64(i)*0.0,
			"job_id":        jobID,
			"bucket":        "generated",
			"take_stack_id": takeStack,
		})
		reply, _, err := h.KernelSendCommand(ctx, ingestCmd)
		if err != nil || !kernelReplySucceeded(reply) {
			return ingested, fmt.Errorf("bridge_ingest_generated_asset failed for %s (job %s): err=%v reply=%v", absPath, jobID, err, reply)
		}
		ingested = append(ingested, map[string]any{
			"file_path":      absPath,
			"clip_id":        firstString(reply, "clip_id", "id"),
			"asset_ref":      firstString(reply, "asset_ref"),
			"active_take_id": firstString(reply, "active_take_id", "take_id"),
			"take_stack_id":  takeStack,
			"format":         artifact.Format,
			"bytes":          artifact.Bytes,
		})
	}
	return ingested, nil
}

// genProviders lists the configured adapter entries without any secret
// material (api keys are reported as presence only).
func (h *Harness) genProviders() (map[string]any, error) {
	providers, configPath, err := genapi.LoadProviders()
	if err != nil {
		gerr := genapi.AsError(err)
		payload := gerr.ResultPayload()
		payload["config_path"] = configPath
		return map[string]any{"status": "error", "error": gerr.Error(), "result": payload}, gerr
	}
	out := map[string]any{
		"status":      "ok",
		"config_path": configPath,
		"count":       len(providers),
		"providers":   genapi.SecretFreeJSON(providers),
	}
	if len(providers) == 0 {
		out["hint"] = "no providers configured: copy ExampleConfig from agent/internal/genapi/config.go into the config path, fill provider/base_url/api_key/model, and restart the agent (or set VIT_GENAPI_CONFIG to a run-scoped file)"
	}
	return out, nil
}

func artifactPayloads(artifacts []genapi.Artifact) []map[string]any {
	out := make([]map[string]any, 0, len(artifacts))
	for _, a := range artifacts {
		out = append(out, map[string]any{
			"kind":             a.Kind,
			"format":           a.Format,
			"local_path":       a.LocalPath,
			"bytes":            a.Bytes,
			"source_url":       a.SourceURL,
			"title":            a.Title,
			"duration_seconds": a.DurationSeconds,
		})
	}
	return out
}

var jobTokenUnsafe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func sanitizeJobToken(s string) string {
	return strings.Trim(jobTokenUnsafe.ReplaceAllString(strings.TrimSpace(s), "-"), "-")
}

func floatValueFromMap(row map[string]any, keys ...string) float64 {
	v, ok := numberValueFromMap(row, keys...)
	if !ok {
		return 0
	}
	return v
}

func intValueFromMap(row map[string]any, keys ...string) int {
	v, ok := numberValueFromMap(row, keys...)
	if !ok {
		return 0
	}
	return int(v)
}

// genYuePrepareLocal runs the yue_local conditioning map (GEN-YUE-WIRE-1):
// ensure the engine surface (starting it per the provider engine config when
// configured and down), read the student score from the kernel when clip_id
// is given, and apply the deterministic request preparation. It returns the
// prepared request, the engine supervision handle and the raw score input
// (for job version archiving).
func (h *Harness) genYuePrepareLocal(ctx context.Context, provider genapi.ProviderConfig, cmd map[string]any, req genapi.GenerationRequest, promptEvidence map[string]any) (*genapi.YuePrepareResult, *genapi.EngineHandle, *genapi.YueScoreInput, error) {
	// Engine supervision first: classified failure before any submit.
	engineHandle, eerr := genapi.EnsureYueEngine(ctx, provider)
	if eerr != nil {
		return nil, nil, nil, eerr
	}

	// Student score face: kernel truth only (existing read-only commands,
	// zero kernel changes).
	scoreInput, serr := h.genReadStudentScore(ctx, cmd)
	if serr != nil {
		return nil, engineHandle, nil, serr
	}

	userPrompt := ""
	if promptEvidence != nil {
		if s, ok := promptEvidence["user_prompt"].(string); ok {
			userPrompt = s
		}
	}
	prepared, perr := genapi.YuePrepareRequest(req, genapi.YuePrepareOptions{
		UserPrompt: userPrompt,
		Score:      scoreInput,
	})
	if perr != nil {
		return nil, engineHandle, scoreInput, perr
	}
	return &prepared, engineHandle, scoreInput, nil
}

// genReadStudentScore reads the student melody clip through the existing
// read-only kernel commands (get_midi_clip_notes + get_project_state's
// music_context) and assembles the deterministic conversion input. Nil clip
// (or no clip_id) returns nil without error — no score face requested.
func (h *Harness) genReadStudentScore(ctx context.Context, cmd map[string]any) (*genapi.YueScoreInput, error) {
	clipID := firstString(cmd, "clip_id", "score_clip_id", "melody_clip_id")
	if strings.TrimSpace(clipID) == "" {
		return nil, nil
	}
	if h.kernel == nil {
		return nil, fmt.Errorf("score face requires a connected kernel (clip_id=%s)", clipID)
	}
	notesCmd := tools.BuildCommand("get_midi_clip_notes", map[string]any{"clip_id": clipID})
	notesReply, _, nerr := h.KernelSendCommand(ctx, notesCmd)
	if nerr != nil || !kernelReplySucceeded(notesReply) {
		return nil, fmt.Errorf("get_midi_clip_notes failed for %s (err=%v reply_status=%s)", clipID, nerr, firstString(notesReply, "status"))
	}
	rows, ok := notesReply["notes"].([]any)
	if !ok {
		return nil, fmt.Errorf("get_midi_clip_notes for %s returned no notes[] array (is the clip a MIDI clip?)", clipID)
	}
	in := &genapi.YueScoreInput{Title: firstString(cmd, "title")}
	for i, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("notes[%d] is not an object", i)
		}
		pitch, pOK := numberValueFromMap(row, "pitch")
		start, sOK := numberValueFromMap(row, "start")
		length, lOK := numberValueFromMap(row, "length")
		if !pOK || !sOK || !lOK {
			return nil, fmt.Errorf("notes[%d] missing pitch/start/length", i)
		}
		in.Notes = append(in.Notes, genapi.YueNote{Pitch: int(pitch), Start: start, Length: length})
	}
	if len(in.Notes) == 0 {
		return nil, fmt.Errorf("clip %s has no notes to convert", clipID)
	}
	// music_context for meter/tempo/key (missing context degrades to 4/4 C —
	// disclosed through the converted header, never a hidden guess about
	// audible values).
	stateCmd := tools.BuildCommand("get_project_state", map[string]any{})
	if stateReply, _, serr := h.KernelSendCommand(ctx, stateCmd); serr == nil && kernelReplySucceeded(stateReply) {
		if mc, ok := stateReply["music_context"].(map[string]any); ok {
			if n, ok := numberValueFromMap(mc, "time_signature_numerator"); ok && n > 0 {
				in.Numerator = int(n)
			}
			if d, ok := numberValueFromMap(mc, "time_signature_denominator"); ok && d > 0 {
				in.Denominator = int(d)
			}
			if t, ok := numberValueFromMap(mc, "tempo_bpm"); ok {
				in.TempoBPM = t
			}
			if key, ok := mc["key_signature"].(map[string]any); ok {
				root := strings.TrimSpace(stringOfAnyMap(key, "root_name"))
				scale := strings.TrimSpace(stringOfAnyMap(key, "scale"))
				if root != "" {
					k := root
					if strings.EqualFold(scale, "minor") && !strings.HasSuffix(k, "m") {
						k += "m"
					}
					in.Key = k
				}
			}
		}
	}
	if in.Numerator == 0 {
		in.Numerator = 4
	}
	if in.Denominator == 0 {
		in.Denominator = 4
	}
	return in, nil
}

func stringOfAnyMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
