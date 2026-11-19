package agentloop

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
)

// BOUNDARY-1 v2 Phase 3: prompt-render evidence (§3.1) and strict terminal
// parsing (§3.2). Every malformed terminal sample must classify unparseable /
// unadmitted — no lenient repair, no silent default.

// terminalMalformedSamples is the §3.2 malformed terminal-output sample set:
// half JSON, missing status field, wrong status, empty proposal, prose-mixed
// output, multi-object output, and long noise.
var terminalMalformedSamples = []struct {
	name string
	raw  string
}{
	{name: "half_json", raw: `{"final":true,"reply":"no candidate","free_state":{"schema_version":"free_state_decision.v1","status":"no_candidate_fo`},
	{name: "missing_status_field", raw: `{"final":true,"reply":"done","free_state":{"schema_version":"free_state_decision.v1","evidence_status":"sufficient"}}`},
	{name: "wrong_status", raw: `{"final":true,"reply":"maybe","free_state":{"schema_version":"free_state_decision.v1","status":"maybe_fix_it","evidence_status":"sufficient"}}`},
	{name: "empty_proposal", raw: `{"final":true,"reply":"propose","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible","improvement_proposal":{}}}`},
	{name: "prose_mixed", raw: `Sure, here is my final decision as requested: {"final":true,"reply":"no candidate","free_state":{"schema_version":"free_state_decision.v1","status":"no_candidate_found","evidence_status":"sufficient"}} hope this helps!`},
	{name: "multi_object", raw: `{"final":true,"reply":"a","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient"}}{"final":true,"reply":"b"}`},
	{name: "long_noise", raw: strings.Repeat("x", 4096) + ` not json at all ` + strings.Repeat("y", 512)},
}

// TestTerminalMalformedSamplesAllFailClosed: for every sample, either the
// ordinary parse rejects it, or the strict terminal raw check classifies it
// unparseable, or the locked output gate refuses it. No sample may reach an
// admitted terminal decision through any layer.
func TestTerminalMalformedSamplesAllFailClosed(t *testing.T) {
	state := terminalLockedState(nil)
	for _, sample := range terminalMalformedSamples {
		out, parseErr := parseMessageLoopOutput(sample.raw)
		if parseErr == nil {
			// The extractor may recover prose-mixed or multi-object shapes;
			// on a locked turn the strict raw check or the output gate must
			// still fail closed.
			if !messageLoopFreeStateTerminalRawUnparseable(sample.raw) &&
				messageLoopFreeStateOutputIssue(state, out) == "" {
				t.Fatalf("%s: malformed sample passed every fail-closed layer: %+v", sample.name, out)
			}
		}
	}
	// The strict raw classifier itself, pinned per sample.
	strict := map[string]bool{
		"half_json": true, "missing_status_field": false, "wrong_status": false, "empty_proposal": false,
		"prose_mixed": true, "multi_object": true, "long_noise": true,
	}
	for _, sample := range terminalMalformedSamples {
		if got := messageLoopFreeStateTerminalRawUnparseable(sample.raw); got != strict[sample.name] {
			t.Fatalf("%s: strict raw verdict = %v, want %v", sample.name, got, strict[sample.name])
		}
	}
	// A clean single-object terminal output and a single-fence wrapper stay
	// parseable — strictness must not bounce legitimate final decisions.
	clean := `{"final":true,"reply":"no candidate","free_state":{"schema_version":"free_state_decision.v1","status":"no_candidate_found","evidence_status":"sufficient"}}`
	if messageLoopFreeStateTerminalRawUnparseable(clean) {
		t.Fatal("clean single-object terminal output must stay parseable")
	}
	if messageLoopFreeStateTerminalRawUnparseable("```json\n" + clean + "\n```") {
		t.Fatal("a single markdown fence wrapper must stay parseable")
	}
}

// Full-loop strict-parse chain: a prose-mixed first response gets the one
// strengthened retry; a clean terminal decision on the retry ends the loop
// with the model-authored terminal (never the fallback).
func TestMessageLoopTerminalStrictParseRetryThenCleanTerminal(t *testing.T) {
	mixed := `Sure! {"final":true,"reply":"done","free_state":{"schema_version":"free_state_decision.v1","status":"capability_blocked","evidence_status":"insufficient"}}`
	clean := `{"final":true,"reply":"the window closed without target evidence","free_state":{"schema_version":"free_state_decision.v1","status":"capability_blocked","evidence_status":"insufficient","summary":"the observation window closed without target evidence"}}`
	client := &fakeMessageCompleter{responses: []string{mixed, clean}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText:     "improve the mix",
		AllowedTools: []string{"ccb.observation_catalog"},
		Context: map[string]any{
			"task_contract":    map[string]any{"kind": "improvement"},
			"free_state_phase": "fs4_diagnostic_round",
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
				"terminal_turn_locked": true, "terminal_turn_reason": "budget_critical",
			},
		},
	})
	if res.FreeStateDecision == nil || res.FreeStateDecision.Status != "capability_blocked" {
		t.Fatalf("clean terminal on the retry must end the loop as a model decision, got status=%q stop=%q err=%q",
			freeStateStatusOf(res), res.StopReason, res.Error)
	}
	if res.StopReason == FreeStateTerminalFallbackStopReason {
		t.Fatal("the fallback fired although the retry produced a clean terminal decision")
	}
	if len(client.calls) != 2 {
		t.Fatalf("model calls = %d, want mixed attempt + one strengthened retry", len(client.calls))
	}
}

func freeStateStatusOf(res Result) string {
	if res.FreeStateDecision == nil {
		return ""
	}
	return res.FreeStateDecision.Status
}

// §3.1 prompt-render evidence: every model turn's assembled system prompt is
// persisted in full with the round identity and the stable section tag.
func TestMessageLoopPromptRenderEvidencePersisted(t *testing.T) {
	dir := t.TempDir()
	renderPath := filepath.Join(dir, "prompt_render.jsonl")
	t.Setenv("VIT_AGENT_PROMPT_RENDER_PATH", renderPath)
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"the window closed without target evidence","free_state":{"schema_version":"free_state_decision.v1","status":"capability_blocked","evidence_status":"insufficient","summary":"the observation window closed without target evidence"}}`,
	}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 1, MaxToolCalls: 1, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText:     "improve the mix",
		AllowedTools: []string{"ccb.observation_catalog"},
		Context: map[string]any{
			"task_contract":    map[string]any{"kind": "improvement"},
			"free_state_phase": "fs4_diagnostic_round",
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
				"terminal_turn_locked": true, "terminal_turn_reason": "budget_critical",
			},
		},
	})
	if res.FreeStateDecision == nil {
		t.Fatalf("fixture loop did not finish: status=%q err=%q", res.Status, res.Error)
	}
	file, err := os.Open(renderPath)
	if err != nil {
		t.Fatalf("prompt render evidence missing: %v", err)
	}
	defer file.Close()
	rows := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for scanner.Scan() {
		var row messageLoopPromptRenderDiagnostic
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("malformed render row: %v", err)
		}
		rows++
		if row.Stage != "prompt_assembly" || row.Turn != 1 {
			t.Fatalf("render row missing stage/turn identity: %+v", row)
		}
		if row.Tag != "message_loop_neutral_family_selection" {
			t.Fatalf("render row tag = %q, want the neutral-family section tag", row.Tag)
		}
		// The persisted prompt keeps the full model-visible contract after the
		// L1-4-IMPL-A split: the byte-stable skeleton stays in system_prompt,
		// the frozen terminal sentence renders in turn_directives (user turn).
		if !strings.Contains(row.SystemPrompt, "free_state_decision.v1") {
			t.Fatal("persisted system prompt lost its decision contract body")
		}
		if !strings.Contains(row.TurnDirectives, freeStateTerminalTurnSentence) {
			t.Fatal("persisted render evidence lost the per-turn terminal directive")
		}
		// A1c S4 diagnostic gap: the model context renders in the user turn,
		// so the row must also carry the exact compact model snapshot (the
		// lesson/observation sections never appear in system_prompt).
		if row.ModelSnapshot == "" {
			t.Fatal("persisted render evidence lost the model snapshot")
		}
		if !strings.Contains(row.ModelSnapshot, "\"schema_version\":\"vit_context_snapshot.v1\"") {
			t.Fatalf("model snapshot is not the contextruntime projection: %s", row.ModelSnapshot)
		}
	}
	if rows != 1 {
		t.Fatalf("render rows = %d, want one per model turn", rows)
	}
}

// readMessageLoopPromptRenderRows reads back the prompt-render diagnostic rows
// written under the test's VIT_AGENT_PROMPT_RENDER_PATH override.
func readMessageLoopPromptRenderRows(t *testing.T, path string) []messageLoopPromptRenderDiagnostic {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("prompt render evidence missing: %v", err)
	}
	defer file.Close()
	rows := []messageLoopPromptRenderDiagnostic{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var row messageLoopPromptRenderDiagnostic
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("malformed render row: %v", err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read render rows: %v", err)
	}
	return rows
}

// A1c S4 diagnostic gap: lesson_context renders in the model snapshot (user
// turn), not in system_prompt. The prompt-render row must carry the snapshot so
// the real-stack probe can assert card/node/selected clip reached the model.
func TestMessageLoopPromptRenderDiagnosticCarriesLessonContext(t *testing.T) {
	dir := t.TempDir()
	renderPath := filepath.Join(dir, "prompt_render.jsonl")
	t.Setenv("VIT_AGENT_PROMPT_RENDER_PATH", renderPath)
	lesson := map[string]any{
		"schema_version": contextruntime.LessonContextSchemaVersion,
		"card":           map[string]any{"id": "first-run-twinkle", "title": "一个旋律的变奏"},
		"node":           map[string]any{"id": "n1_read_and_fix", "title": "读谱纠错", "goal": "把识别错的音改对"},
		"assets":         []map[string]any{{"id": "theme", "kind": "midi_reference"}},
		"selected_clip":  map[string]any{"clip_id": "c_take_second", "track_id": "t1"},
		"allowed":        []string{"mutate_midi_notes"},
		"truncated":      false,
	}
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"这一关要把识别错的音改对。","free_state":{"schema_version":"free_state_decision.v1","status":"capability_blocked","evidence_status":"insufficient","summary":"lesson question answered from bounded context"}}`,
	}}
	loop := &MessageLoop{
		Runtime: loopTestRuntime(),
		Client:  client,
		Config:  config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget:  Budget{MaxTurns: 1, MaxToolCalls: 1, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		GoalID:       "goal-render-lesson",
		RunID:        "run-render-lesson",
		UserText:     "这一关为什么要把这个音改对？",
		Summary:      "这一关为什么要把这个音改对？",
		AllowedTools: []string{"ccb.observation_catalog"},
		Context: map[string]any{
			"task_contract":                 map[string]any{"kind": "improvement"},
			"free_state_phase":              "fs4_diagnostic_round",
			contextruntime.LessonContextKey: lesson,
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "这一关为什么要把这个音改对？",
				"terminal_turn_locked": true, "terminal_turn_reason": "budget_critical",
			},
		},
	})
	if res.FreeStateDecision == nil {
		t.Fatalf("fixture loop did not finish: status=%q err=%q", res.Status, res.Error)
	}
	rows := readMessageLoopPromptRenderRows(t, renderPath)
	if len(rows) != 1 {
		t.Fatalf("render rows = %d, want one per model turn", len(rows))
	}
	row := rows[0]
	if row.Tag != "message_loop_neutral_family_selection" {
		t.Fatalf("render row tag = %q", row.Tag)
	}
	if row.ModelSnapshot == "" {
		t.Fatal("render row lost the model snapshot")
	}
	for _, want := range []string{
		`"lesson_context"`,
		`"vit.edu_jam.lesson_context.v1"`,
		`"first-run-twinkle"`,
		`"n1_read_and_fix"`,
		`"clip_id":"c_take_second"`,
		`"track_id":"t1"`,
	} {
		if !strings.Contains(row.ModelSnapshot, want) {
			t.Fatalf("render row model_snapshot missing %s: %s", want, row.ModelSnapshot)
		}
	}
	if !strings.Contains(row.SystemPrompt, "free_state_decision.v1") {
		t.Fatal("persisted system prompt lost its decision contract body")
	}
}

// Ordinary (non-free-state) turns must persist a non-empty model snapshot too:
// the diagnostic field is the model-context evidence for every loop turn.
func TestMessageLoopPromptRenderDiagnosticCarriesModelSnapshotOnOrdinaryTurns(t *testing.T) {
	dir := t.TempDir()
	renderPath := filepath.Join(dir, "prompt_render.jsonl")
	t.Setenv("VIT_AGENT_PROMPT_RENDER_PATH", renderPath)
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"continued"}`}}
	loop := &MessageLoop{
		Runtime: loopTestRuntime(),
		Client:  client,
		Config:  config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget:  Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		GoalID:   "goal-render-ordinary",
		RunID:    "run-render-ordinary",
		UserText: "show me the project state",
		Summary:  "show me the project state",
		Context:  map[string]any{"initialized": true, "selected_track_id": "t1"},
	})
	if res.Reply != "continued" {
		t.Fatalf("ordinary loop result = status=%q reply=%q stop=%q", res.Status, res.Reply, res.StopReason)
	}
	rows := readMessageLoopPromptRenderRows(t, renderPath)
	if len(rows) != 1 {
		t.Fatalf("render rows = %d, want one per model turn", len(rows))
	}
	row := rows[0]
	if row.Tag != "message_loop_system" {
		t.Fatalf("ordinary render row tag = %q", row.Tag)
	}
	if row.ModelSnapshot == "" {
		t.Fatal("ordinary render row lost the model snapshot")
	}
	for _, want := range []string{`"schema_version":"vit_context_snapshot.v1"`, `"user_text":"show me the project state"`} {
		if !strings.Contains(row.ModelSnapshot, want) {
			t.Fatalf("ordinary render row model_snapshot missing %s: %s", want, row.ModelSnapshot)
		}
	}
}
