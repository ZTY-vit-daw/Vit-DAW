package harness

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/genctx"
	"vit-daw-agent/internal/tools"
)

// genRenderPrompt is the GEN-CTX-CONSUME-1 integration point: every
// gen.generate call renders its prompt through the single genctx template
// engine (profile=gen_api) before the dialect layer runs. The adapter's job
// after this is protocol conversion only — it must not alter the auto blocks
// (design doc 03 §2, adapter acceptance item).
//
// scene semantics (design doc 04): in_project (default) assembles the
// project snapshot and injects the auto blocks; exempt scenes render the
// user text alone (zero auto context) and later refuse project landing via
// the write-front sentinel.
func (h *Harness) genRenderPrompt(ctx context.Context, cmd map[string]any) (rendered string, scene genctx.Scene, evidence map[string]any, err error) {
	sceneStr := firstString(cmd, "scene")
	if sceneStr == "" {
		scene = genctx.SceneInProject
	} else if scene, err = genctx.ParseScene(sceneStr); err != nil {
		return "", scene, nil, err
	}

	userPrompt := firstString(cmd, "prompt", "description", "gpt_description_prompt")
	if strings.TrimSpace(userPrompt) == "" {
		// The auto blocks are context, not content: with no user free text
		// there is nothing to generate. This guard also keeps the empty-
		// request fail-closed probe meaningful now that in_project prompts
		// are never literally empty (they carry the injected blocks).
		return "", scene, nil, fmt.Errorf("gen.generate needs a non-empty prompt (the user free-text block)")
	}
	if scene != genctx.SceneInProject {
		// Exempt scenes carry no project context — not even a state fetch.
		out, rerr := genctx.RenderGenerationPrompt(genctx.Snapshot{
			SchemaVersion: genctx.SchemaVersion,
			Scene:         scene,
		}, userPrompt, genctx.RenderProfile{ConsumerID: "gen_api"})
		if rerr != nil {
			return "", scene, nil, rerr
		}
		return out, scene, map[string]any{
			"scene":               string(scene),
			"music_context_block": false,
			"constraints_block":   false,
			"guidance_block":      false,
			"injected":            false,
		}, nil
	}

	stateCmd := tools.BuildCommand("get_project_state", map[string]any{})
	stateReply, _, kerr := h.KernelSendCommand(ctx, stateCmd)
	if kerr != nil || !kernelReplySucceeded(stateReply) {
		return "", scene, nil, fmt.Errorf("scene in_project requires open project truth: get_project_state failed (err=%v reply_status=%v) — open a project or use scene=free_creation for exempt generation", kerr, firstString(stateReply, "status"))
	}

	call := genctx.CallParams{
		HarmonicNote: firstString(cmd, "harmonic_note"),
		EnergyNote:   firstString(cmd, "energy_note"),
	}
	snap, aerr := genctx.AssembleSnapshot(stateReply, scene, call)
	if aerr != nil {
		return "", scene, nil, fmt.Errorf("scene in_project requires open project truth: %w", aerr)
	}
	out, rerr := genctx.RenderGenerationPrompt(snap, userPrompt, genctx.RenderProfile{ConsumerID: "gen_api"})
	if rerr != nil {
		return "", scene, nil, rerr
	}

	keySet := false
	if snap.KeySignature != nil {
		keySet = true
	}
	evidence = map[string]any{
		"scene":               string(scene),
		"music_context_block": strings.Contains(out, "== MUSIC CONTEXT"),
		"constraints_block":   strings.Contains(out, "== CONSTRAINTS"),
		"guidance_block":      strings.Contains(out, "== GUIDANCE"),
		"injected":            strings.Contains(out, "== MUSIC CONTEXT"),
		"snapshot": map[string]any{
			"schema_version":   snap.SchemaVersion,
			"tempo_bpm":        snap.TempoBPM,
			"time_signature":   snap.TimeSignature,
			"key_set":          keySet,
			"project_revision": snap.ProjectRevision,
		},
	}
	return out, scene, evidence, nil
}
