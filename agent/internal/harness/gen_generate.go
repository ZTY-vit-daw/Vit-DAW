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
	}

	limitations := provider.Limitations(req)

	genCtx, cancel := context.WithTimeout(ctx, provider.Timeout())
	defer cancel()
	result, err := genapi.Generate(genCtx, provider, req)
	if err != nil {
		gerr := genapi.AsError(err)
		payload := gerr.ResultPayload()
		payload["provider_id"] = providerID
		return map[string]any{"status": "error", "error": gerr.Error(), "result": payload}, gerr
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
