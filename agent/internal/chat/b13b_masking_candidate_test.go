package chat

import (
	"sort"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/contextruntime"
)

// B13-B candidate-source coverage pins (2026-09-13).
//
// Live form (FALLBACK-1 receipt §5.2, 912.vit): mix.frequency_relationship was
// ready with conflict_candidates = [] while mix.masking_relationship was ready
// with coverage.candidate_count = 138 and four directional candidate rows.
// audioClosureCandidateRows had no masking arm and its facts reader only knew
// the two conflict key families, so the frontier stayed empty, G5 refused, and
// the closure hard-failed.
//
// The four pins below are the acceptance criteria verbatim:
//  ① a masking ready candidate row reaches the frontier with its band /
//     masker / target / margin field families mapped;
//  ② the 912.vit live shape (frequency zero + masking present) yields a
//     non-empty frontier that passes G5;
//  ③ views without a candidate family add zero noise;
//  ④ the digest shape and the durable full-package shape agree.

const b13bMaskingViewID = "mix.masking_relationship"

// b13bMaskingCandidateRow is one MOM masking directional row verbatim
// (vit_relative_energetic_masking_risk.v1): the pair is named explicitly and
// the band is band_id, not the conflict views' "band".
func b13bMaskingCandidateRow(band, maskerID, maskerName, targetID, targetName string, median, p90, maxMargin float64) map[string]any {
	return map[string]any{
		"band_id": band, "masker_track_id": maskerID, "masker_track_name": maskerName,
		"target_track_id": targetID, "target_track_name": targetName,
		"median_margin_db": median, "p90_margin_db": p90, "max_margin_db": maxMargin,
		"risk_coverage_ratio": 1, "risk_frame_count": 216, "active_frame_count": 216,
		"min_hz": 2000, "max_hz": 6000,
	}
}

// b13bMaskingCandidates is the live four-row body plus MOM's own truncation
// sentinel, exactly as the 912.vit ledger carried it.
func b13bMaskingCandidates() []any {
	return []any{
		b13bMaskingCandidateRow("presence", "1012", "drums", "1007", "bass", 30.612, 41.49, 45.921),
		b13bMaskingCandidateRow("low_mid", "1007", "bass", "1012", "drums", 12.5, 18.25, 22.0),
		b13bMaskingCandidateRow("high_mid", "1017", "gtr", "1007", "bass", 9.75, 14.5, 19.5),
		b13bMaskingCandidateRow("sub", "1007", "bass", "1022", "kick", 7.25, 11.0, 15.75),
		map[string]any{"omitted_items": 1},
	}
}

// b13bMaskingDigestFacts is the decision-digest facts map the live ledger
// stored for the masking view (§5.2(3)): flat, key family "candidates".
func b13bMaskingDigestFacts() map[string]any {
	return map[string]any{
		"status": "ready", "freshness": "fresh", "candidate_only": true,
		"measurement_id": "mask_412cdda0fe0107ce969d7d47",
		"model_version":  "vit_relative_energetic_masking_risk.v1",
		"coverage": map[string]any{"candidate_count": 138, "projected_candidate_count": 96,
			"candidates_truncated": true, "eligible_track_count": 6, "track_count": 6},
		"candidates": b13bMaskingCandidates(),
	}
}

// b13bLiveLedger reproduces the 912.vit observation ledger at the granularity
// of §5.2(2)+(3): the frequency view is ready with an empty conflict candidate
// family, the masking view is ready with four candidates.
func b13bLiveLedger() map[string]any {
	return map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"available_views": map[string]any{
			"track:1007::" + "mix.frequency_relationship": map[string]any{
				"view_id": "mix.frequency_relationship", "status": "ready", "observation_id": "obs-912-freq",
				"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "bass"},
				"freshness":  map[string]any{"status": "current_observation", "project_revision": "912"},
				"conclusion": map[string]any{
					"digest_kind": "decision_digest", "projection_status": "ready",
					"facts": map[string]any{
						"conflict_candidates": []any{}, "coverage": map[string]any{},
						"interpretation_limits": []any{},
						"relationship_inputs": map[string]any{"status": "ready", "tap_points": []any{"source_file_pre_fx"},
							"track_count": 6, "usable_track_count": 6},
					},
				},
			},
			"track:1007::" + b13bMaskingViewID: map[string]any{
				"view_id": b13bMaskingViewID, "status": "ready", "observation_id": "obs-912-mask",
				"target_ref":    map[string]any{"kind": "track", "id": "1007", "label": "bass"},
				"freshness":     map[string]any{"status": "current_observation", "project_revision": "912"},
				"evidence_refs": []any{"observation:obs-912-mask"},
				"conclusion": map[string]any{
					"digest_kind": "decision_digest", "projection_status": "ready",
					"facts": b13bMaskingDigestFacts(),
				},
			},
		},
	}
}

func b13bCandidateIDs(candidates []audioclosure.Candidate) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.ID)
	}
	sort.Strings(out)
	return out
}

// ① A masking ready candidate row reaches the frontier, and the row the
// extractor consumed carries the band / directional-pair / margin families.
func TestB13BMaskingCandidateRowsMapBandPairAndMarginFamilies(t *testing.T) {
	summary := map[string]any{
		"status": "ready", "observation_id": "obs-912-mask",
		"requested_views": []any{b13bMaskingViewID},
		"evidence_refs":   []any{"observation:obs-912-mask"},
		"views": map[string]any{b13bMaskingViewID: map[string]any{
			"status": "ready", "projection_status": "ready", "facts": b13bMaskingDigestFacts(),
		}},
	}
	rows := audioClosureCandidateRows(summary, b13bMaskingViewID)
	// Four directional rows; MOM's own {"omitted_items":1} sentinel is not a
	// candidate and must not become one.
	if len(rows) != 4 {
		t.Fatalf("masking fact rows produced %d candidate rows: %+v", len(rows), rows)
	}
	row := rows[0]
	// Band family: MOM names it band_id; the extractor reads region/band.
	if row["band_id"] != "presence" || row["region"] != "presence" || row["band"] != "presence" {
		t.Fatalf("masking band family was not mapped: %+v", row)
	}
	// Directional pair family: both sides stay readable under their own keys.
	if row["target_track_id"] != "1007" || row["target_track_name"] != "bass" {
		t.Fatalf("masking target family was not preserved: %+v", row)
	}
	if row["masker_track_id"] != "1012" || row["masker_track_name"] != "drums" {
		t.Fatalf("masking masker family was not preserved: %+v", row)
	}
	// Margin family stays verbatim: the frontier row family has no margin slot
	// and none is invented.
	if row["median_margin_db"] != 30.612 || row["p90_margin_db"] != 41.49 || row["max_margin_db"] != 45.921 {
		t.Fatalf("masking margin family was not preserved: %+v", row)
	}
	// Extractor family: the pair is exposed where audioClosureCandidateTracks reads.
	tracks := freeStateMapRows(row["tracks"])
	if len(tracks) != 2 || tracks[0]["track_id"] != "1007" || tracks[1]["track_id"] != "1012" {
		t.Fatalf("masking pair was not exposed to the extractor: %+v", tracks)
	}

	candidates := audioClosureCandidates(nil, []*agentloop.RecentObservation{{
		Tool: "ccb.observation_request", Status: "ready", Summary: summary,
	}})
	if len(candidates) != 4 {
		t.Fatalf("masking rows produced %d frontier candidates: %+v", len(candidates), candidates)
	}
	for _, candidate := range candidates {
		if candidate.ViewID != b13bMaskingViewID || candidate.Region == "" || len(candidate.TrackIDs) != 2 {
			t.Fatalf("masking candidate lost its view/region/pair binding: %+v", candidate)
		}
	}
}

// ② The 912.vit live shape: frequency candidates are zero, masking candidates
// are present, and the frontier is therefore non-empty — which is exactly what
// G5 reads.
func TestB13BMaskingCandidatesEstablishFrontierWhenFrequencyIsEmpty(t *testing.T) {
	observations := freeStateLedgerObservations(b13bLiveLedger())
	if len(observations) != 2 {
		t.Fatalf("live ledger rehydration produced %d observations: %+v", len(observations), observations)
	}
	frontier, actionability := audioClosureFrontier(audioclosure.HypothesisFrontier{}, agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
	}, observations)
	if len(frontier.Candidates) != 4 {
		t.Fatalf("912.vit live shape produced %d frontier candidates, want 4: %+v", len(frontier.Candidates), frontier)
	}
	if actionability != audioclosure.ActionabilityUnknown {
		t.Fatalf("observation-only round changed actionability: %v", actionability)
	}
	for _, candidate := range frontier.Candidates {
		if candidate.ViewID != b13bMaskingViewID {
			t.Fatalf("a zero-candidate frequency view contributed a candidate: %+v", candidate)
		}
		if candidate.SourceObservationID != "obs-912-mask" {
			t.Fatalf("masking candidate lost its source observation: %+v", candidate)
		}
	}
	// G5_frontier_established reads the closure's hypothesis_frontier.candidates.
	closure := map[string]any{
		"project_uuid": "project-912", "project_revision": "912",
		"hypothesis_frontier": frontier,
	}
	audit := agentloop.AuditFreeStateNeedsExperimentGate(map[string]any{
		"task_contract":         map[string]any{"project_uuid": "project-912", "project_revision": "912"},
		"minimal_audio_closure": closure,
		"free_state_capacity_assessment": map[string]any{
			"capacity_level": "within_free_state", "selected_capability": "free_state",
		},
	}, &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema})
	for _, failed := range audit.FailedGateIDs {
		if failed == "G5_frontier_established" {
			t.Fatalf("G5 still refuses a frontier built from the 912.vit masking shape: %+v", audit)
		}
	}
}

// ③ Zero noise: the masking candidate key family is read for exactly the
// masking view, and a view without a candidate family adds no candidate.
func TestB13BMaskingCandidateFamilyStaysViewScoped(t *testing.T) {
	// A "candidates" array on any other view is not this view's candidate
	// family: the same rows under project.structure must yield nothing.
	foreign := map[string]any{
		"status": "ready", "observation_id": "obs-foreign",
		"requested_views": []any{"project.structure"},
		"views": map[string]any{"project.structure": map[string]any{
			"status": "ready", "facts": map[string]any{"candidates": b13bMaskingCandidates()},
		}},
	}
	if rows := audioClosureCandidateRows(foreign, "project.structure"); len(rows) != 0 {
		t.Fatalf("a foreign candidates array was read as a candidate family: %+v", rows)
	}
	if candidates := audioClosureCandidates(nil, []*agentloop.RecentObservation{{
		Tool: "ccb.observation_request", Status: "ready", Summary: foreign,
	}}); len(candidates) != 0 {
		t.Fatalf("a foreign candidates array reached the frontier: %+v", candidates)
	}

	// A masking view whose facts carry no candidate family produces none.
	empty := map[string]any{
		"status": "ready", "observation_id": "obs-empty",
		"requested_views": []any{b13bMaskingViewID},
		"views": map[string]any{b13bMaskingViewID: map[string]any{
			"status": "ready", "facts": map[string]any{"status": "ready", "candidates": []any{}},
		}},
	}
	if rows := audioClosureCandidateRows(empty, b13bMaskingViewID); len(rows) != 0 {
		t.Fatalf("an empty masking candidate family produced rows: %+v", rows)
	}

	// Truncation sentinels only: no track identity, so still no candidate.
	sentinel := map[string]any{
		"status": "ready", "observation_id": "obs-sentinel",
		"requested_views": []any{b13bMaskingViewID},
		"views": map[string]any{b13bMaskingViewID: map[string]any{
			"status": "ready", "facts": map[string]any{"candidates": []any{map[string]any{"omitted_items": 138}}},
		}},
	}
	candidates := audioClosureCandidates(nil, []*agentloop.RecentObservation{{
		Tool: "ccb.observation_request", Status: "ready", Summary: sentinel,
	}})
	if len(candidates) != 0 {
		t.Fatalf("a truncation sentinel became a candidate: %+v", candidates)
	}
}

// ④ Shape parity: the digest shape (the live bundle ProjectCCBViewConclusion
// digests, and the compact envelope the ledger stores) and the durable
// full-package shape (mom_projection.masking_relationship) yield the same
// candidates.
func TestB13BMaskingDigestAndFullPackageShapesAgree(t *testing.T) {
	rawFacts := map[string]any{
		"status": "ready", "freshness": "fresh", "candidate_only": true,
		"measurement_id": "mask_412cdda0fe0107ce969d7d47",
		"model_version":  "vit_relative_energetic_masking_risk.v1",
		"coverage": map[string]any{"candidate_count": 138, "projected_candidate_count": 96,
			"candidates_truncated": true, "eligible_track_count": 6, "track_count": 6},
		"candidates": b13bMaskingCandidates(),
	}

	// (a) Live CCB bundle: the raw MOM facts reachable by ProjectCCBViewConclusion.
	digestSource := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-912-mask",
		"requested_views": []any{b13bMaskingViewID},
		"evidence_refs":   []any{"observation:obs-912-mask"},
		"views": map[string]any{b13bMaskingViewID: map[string]any{
			"status": "ready",
			"facts": map[string]any{"observation.mom_projection": map[string]any{
				"masking_relationship": rawFacts,
			}},
		}},
	}
	// The digest itself must carry the masking candidate key family — this is
	// the first path's input, and it is produced by contextruntime unchanged.
	conclusion := contextruntime.ProjectCCBViewConclusion(digestSource, b13bMaskingViewID, contextruntime.Options{
		MaxTextRunes: 900, MaxListItems: 8, MaxPreviewBytes: 6 * 1024, SkipPluginSemanticLoad: true,
	})
	digestRows := freeStateMapRows(firstMapFromAny(conclusion["facts"])["candidates"])
	directional := 0
	for _, row := range digestRows {
		if firstStringFromMap(row, "target_track_id") != "" {
			directional++
		}
	}
	if directional != 4 {
		t.Fatalf("the masking digest carries %d directional rows, want 4: %+v", directional, conclusion)
	}

	// (b) The durable ledger envelope: that digest re-hung under views[v].facts.
	compact := map[string]any{
		"status": "ready", "observation_id": "obs-912-mask",
		"requested_views": []any{b13bMaskingViewID},
		"evidence_refs":   []any{"observation:obs-912-mask"},
		"views": map[string]any{b13bMaskingViewID: map[string]any{
			"status": "ready", "facts": b13bMaskingDigestFacts(),
		}},
	}
	// (c) The durable full acoustic package: no requested_views, no view
	// envelope — only the MOM projection the harness persists.
	fullPackage := map[string]any{
		"status": "ready", "observation_id": "obs-912-mask",
		"evidence_refs":  []any{"observation:obs-912-mask"},
		"mom_projection": map[string]any{"masking_relationship": rawFacts},
	}

	viewIDs := audioClosureCandidateViewIDs(fullPackage)
	if len(viewIDs) != 1 || viewIDs[0] != b13bMaskingViewID {
		t.Fatalf("the durable masking package did not select its own view family: %v", viewIDs)
	}

	extract := func(summary map[string]any) []audioclosure.Candidate {
		return audioClosureCandidates(nil, []*agentloop.RecentObservation{{
			Tool: "ccb.observation_request", Status: "ready", Summary: summary,
		}})
	}
	fromDigest, fromCompact, fromPackage := extract(digestSource), extract(compact), extract(fullPackage)
	if len(fromDigest) != 4 || len(fromCompact) != 4 || len(fromPackage) != 4 {
		t.Fatalf("shape parity broke: digest=%d compact=%d package=%d", len(fromDigest), len(fromCompact), len(fromPackage))
	}
	digestIDs, compactIDs, packageIDs := b13bCandidateIDs(fromDigest), b13bCandidateIDs(fromCompact), b13bCandidateIDs(fromPackage)
	for index := range digestIDs {
		if digestIDs[index] != compactIDs[index] || digestIDs[index] != packageIDs[index] {
			t.Fatalf("the two shapes disagree on candidate identity:\n digest=%v\n compact=%v\n package=%v", digestIDs, compactIDs, packageIDs)
		}
	}
	if fromPackage[0].EvidenceRefs == nil || len(fromPackage[0].EvidenceRefs) != 1 {
		t.Fatalf("the package route lost the observation evidence refs: %+v", fromPackage[0])
	}
}
