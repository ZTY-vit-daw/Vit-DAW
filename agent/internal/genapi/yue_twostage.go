package genapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Two-stage instrumental generation via score transfer
// (YUE-INSTRUMENTAL-SCORE-TRANSFER-1 leg B).
//
// The instrumental request path for yue_local runs TWO jobs on the same
// engine (the user+Codex-verified fifth route of 2026-11-20, success round
// standalone_instrumental_repeat_20261120_214808):
//
//	pass1  the prepared request as-is (official instrumental conditioning;
//	       free or student-score generation). The engine may still sing —
//	       its generated ABC (echoed in the result's replay JSON) is the
//	       TRANSFER SOURCE, so a vocal pass1 is expected, not a failure.
//	transfer  YueInstrumentalizeTransfer moves every sounding Vocal note
//	       into the Ins voice (vocal-priority overlap, chords/meter/tempo
//	       preserved). The transferred score carries ZERO sounding Vocal
//	       notes — the pass2 vocal suppression is structural, no engine
//	       switch and no single-stage suppression claim.
//	pass2  the same request with abc=transferred score re-rendered as a
//	       SECOND job (214808 instrumental.request.json 口径: abc=transfer
//	       score, cot re-derived from the score's Vocal chords, style keeps
//	       the Instrumental prefix + negative conditions verbatim, the
//	       fixed seed/lm_seed ride the same provider extra_params). The
//	       final WAV/version comes from pass2; pass1 artifacts stay on disk
//	       as evidence.
//
// Cost is honestly double (two engine jobs per instrumental request).
// Failure classes follow 372 §8: pass2 short output is the known
// probabilistic legW8 boundary — recorded as-is, never auto-retried here.

// YueTwoStageOptions carries the caller-side facts the pass2 lyrics
// discipline needs.
type YueTwoStageOptions struct {
	// LyricsUserProvided: the caller's lyrics were explicitly provided (not
	// prepare-derived section tags). User lyrics ride pass2 verbatim;
	// otherwise pass2 lyrics are re-derived from the transferred score
	// (official discipline: exact section tags or empty).
	LyricsUserProvided bool
}

// YueTwoStageResult carries both jobs plus the conversion receipt. The
// final version/artifacts live in Pass2 (Final()).
type YueTwoStageResult struct {
	Pass1          *Result              `json:"-"`
	Pass2          *Result              `json:"-"`
	GeneratedABC   string               `json:"-"`
	TransferredABC string               `json:"-"`
	Transfer       *YueTransferReceipt  `json:"transfer,omitempty"`
	Pass2Request   GenerationRequest    `json:"-"`
	Pass2Dispatched map[string]any      `json:"-"`
	Actions        []string             `json:"actions,omitempty"`
}

// Final returns the pass2 result whose artifacts are the delivered version.
func (r *YueTwoStageResult) Final() *Result {
	if r == nil {
		return nil
	}
	return r.Pass2
}

// ReceiptMap renders the job-archive face: both job ids, the transfer
// conservation receipt and the pass2 request口径 (style/cot/lyrics shape),
// with the fixed seeds recorded as configured.
func (r *YueTwoStageResult) ReceiptMap(cfg ProviderConfig) map[string]any {
	out := map[string]any{
		"mode":             "score_transfer_two_stage",
		"final_version_source": "pass2",
		"pass1_task_id":    r.Pass1.TaskID,
		"pass2_task_id":    r.Pass2.TaskID,
		"generated_abc_chars": len([]rune(r.GeneratedABC)),
		"transferred_abc_chars": len([]rune(r.TransferredABC)),
		"style":            r.Pass2Request.StyleTags,
		"cot":              r.Pass2Request.COT,
		"actions":          r.Actions,
		"pass2_dispatched": r.Pass2Dispatched,
	}
	if r.Transfer != nil {
		out["transfer"] = r.Transfer.ReceiptMap()
	}
	var pass1Artifacts []map[string]any
	for _, a := range r.Pass1.Artifacts {
		pass1Artifacts = append(pass1Artifacts, map[string]any{"kind": a.Kind, "local_path": a.LocalPath, "bytes": a.Bytes})
	}
	out["pass1_artifacts_on_disk"] = pass1Artifacts
	seeds := map[string]any{}
	for _, k := range []string{"seed", "lm_seed"} {
		if v, ok := cfg.ExtraParams[k]; ok {
			seeds[k] = v
		}
	}
	if len(seeds) > 0 {
		out["fixed_seeds"] = seeds
	}
	return out
}

// YueGenerateInstrumentalTwoStage runs the pass1 -> transfer -> pass2
// pipeline on one provider config. Each stage gets the provider's own
// timeout budget (Generate applies cfg.TimeoutSeconds per call); the caller
// must NOT wrap both stages in a single per-request deadline that could
// kill pass2 mid-render.
func YueGenerateInstrumentalTwoStage(ctx context.Context, cfg ProviderConfig, req GenerationRequest, opts YueTwoStageOptions) (*YueTwoStageResult, error) {
	out := &YueTwoStageResult{}

	// Stage 1: the prepared request as-is. A vocal pass1 is the expected
	// transfer-source face, not a failure; the engine has no instrumental
	// switch (381 forensics) and none is claimed.
	pass1, err := Generate(ctx, cfg, req)
	if err != nil {
		return nil, fmt.Errorf("two-stage pass1 (free/prepared generation): %w", err)
	}
	out.Pass1 = pass1

	genABC, aerr := replayABCFromResult(pass1)
	if aerr != nil {
		return nil, aerr
	}
	out.GeneratedABC = genABC
	out.Actions = append(out.Actions, fmt.Sprintf("two-stage pass1: job %s generated %d-char abc (replay echo; vocals possible — this melody is the transfer source)", pass1.TaskID, len([]rune(genABC))))

	transferred, receipt, terr := YueInstrumentalizeTransfer(genABC)
	if terr != nil {
		return nil, &Error{Class: ClassParseError, ProviderMessage: fmt.Sprintf("two-stage pass2 aborted: score transfer failed for pass1 job %s: %v (pass1 artifacts remain on disk; failure class: invalid generated abc)", pass1.TaskID, terr)}
	}
	out.TransferredABC = transferred
	out.Transfer = receipt
	out.Actions = append(out.Actions, fmt.Sprintf("two-stage transfer: vocal notes %d -> 0, ins %d -> %d on the transferred score (chords/meter/tempo preserved; suppression in pass2 is structural)",
		receipt.VocalNotesBefore, receipt.OriginalInsNotes, receipt.OutputInsNotes))

	// Pass2 request: the 214808 instrumental.request.json 口径 — abc=transferred
	// score, style kept verbatim (already recipe-normalized by the prepare
	// step), cot re-derived from the transferred score's Vocal chords, and
	// the official lyrics discipline re-applied to the transferred score.
	req2 := req
	req2.ABC = transferred
	if YueVocalHasChords(transferred) {
		req2.COT = "full"
	} else {
		req2.COT = "melody"
	}
	if opts.LyricsUserProvided {
		out.Actions = append(out.Actions, "two-stage pass2 lyrics: user-provided lyrics kept verbatim (official recipe allows only empty or exact section tags; sung words may summon vocals — real-stack listening gate)")
	} else if tags, has := YueSectionTagLyrics(transferred); has {
		req2.Lyrics = tags
		out.Actions = append(out.Actions, "two-stage pass2 lyrics: section tags re-derived from the transferred score (official instrumental recipe; no sung words)")
	} else {
		req2.Lyrics = ""
		out.Actions = append(out.Actions, "two-stage pass2 lyrics: empty (transferred score carries no section comments; official instrumental recipe)")
	}
	out.Pass2Request = req2
	out.Pass2Dispatched = YueDispatchedSummary(req2, cfg)

	pass2, err := Generate(ctx, cfg, req2)
	if err != nil {
		return nil, fmt.Errorf("two-stage pass2 (fixed-abc re-render of the transferred score): %w", err)
	}
	out.Pass2 = pass2
	out.Actions = append(out.Actions, fmt.Sprintf("two-stage pass2: job %s re-rendered the transferred score; final WAV/version from pass2 (double generation cost: 2 engine jobs)", pass2.TaskID))
	return out, nil
}

// replayABCFromResult extracts the generated score from a pass1 result: the
// yue_local dialect stores the engine's replay JSON (request echo + the
// generated abc for free-form jobs) as a kind="replay" sidecar artifact.
// Success round 214808 anchor: for a no-abc pass1 the replay abc IS the
// generated score (1904 bytes = the gold original.abc).
func replayABCFromResult(res *Result) (string, error) {
	for _, a := range res.Artifacts {
		if a.Kind != "replay" || strings.TrimSpace(a.LocalPath) == "" {
			continue
		}
		raw, rerr := os.ReadFile(a.LocalPath)
		if rerr != nil {
			return "", &Error{Class: ClassParseError, ProviderMessage: fmt.Sprintf("two-stage pass2 cannot start: read pass1 replay sidecar %s: %v", a.LocalPath, rerr)}
		}
		var doc map[string]any
		if jerr := json.Unmarshal(raw, &doc); jerr != nil {
			return "", &Error{Class: ClassParseError, ProviderMessage: fmt.Sprintf("two-stage pass2 cannot start: pass1 replay sidecar %s is not JSON: %v", a.LocalPath, jerr)}
		}
		if s, ok := doc["abc"].(string); ok && strings.TrimSpace(s) != "" {
			return s, nil
		}
	}
	return "", &Error{Class: ClassParseError, ProviderMessage: fmt.Sprintf("two-stage pass2 cannot start: the pass1 result (job %s) carried no replay abc — the engine echoed no generated score; pass1 artifacts remain on disk", res.TaskID)}
}
