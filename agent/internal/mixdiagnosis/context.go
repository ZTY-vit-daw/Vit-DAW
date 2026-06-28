package mixdiagnosis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const SchemaVersion = "mix_diagnosis_context.v0"

type Input struct {
	ConversationID     string
	GoalID             string
	RunID              string
	UserIntent         string
	TargetRef          string
	ActionKind         string
	ProcessorType      string
	ObservationSummary map[string]any
	RequestContext     map[string]any
	CreatedAt          string
}

type Fact struct {
	Key     string         `json:"key,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Ref     string         `json:"ref,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type MissingEvidence struct {
	Key      string `json:"key,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Blocking bool   `json:"blocking,omitempty"`
}

type Recommendation struct {
	Strategy      string `json:"strategy,omitempty"`
	NextStep      string `json:"next_step,omitempty"`
	ActionKind    string `json:"action_kind,omitempty"`
	ProcessorType string `json:"processor_type,omitempty"`
	Risk          string `json:"risk,omitempty"`
	Confidence    string `json:"confidence,omitempty"`
}

type Context struct {
	SchemaVersion     string            `json:"schema_version"`
	ID                string            `json:"id"`
	ConversationID    string            `json:"conversation_id,omitempty"`
	GoalID            string            `json:"goal_id,omitempty"`
	RunID             string            `json:"run_id,omitempty"`
	ObservationID     string            `json:"observation_id,omitempty"`
	MixSessionID      string            `json:"mix_session_id,omitempty"`
	UserIntent        string            `json:"user_intent,omitempty"`
	TargetRef         string            `json:"target_ref,omitempty"`
	ProblemKind       string            `json:"problem_kind"`
	ObservedFacts     []Fact            `json:"observed_facts,omitempty"`
	UserReportedFacts []Fact            `json:"user_reported_facts,omitempty"`
	InferredFacts     []Fact            `json:"inferred_facts,omitempty"`
	MissingEvidence   []MissingEvidence `json:"missing_evidence,omitempty"`
	Recommendation    Recommendation    `json:"recommendation"`
	LowRiskNextStep   string            `json:"low_risk_next_step,omitempty"`
	EvidenceStatus    map[string]any    `json:"evidence_status,omitempty"`
	EvidenceRefs      []string          `json:"evidence_refs,omitempty"`
	SourceRefs        map[string]string `json:"source_refs,omitempty"`
	CreatedAt         string            `json:"created_at,omitempty"`
}

func (ctx Context) Map() map[string]any {
	data, _ := json.Marshal(ctx)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}

func Build(in Input) Context {
	intent := firstNonEmpty(strings.TrimSpace(in.UserIntent), findTextByKey(in.RequestContext, "intent", "goal_text"), findTextByKey(in.ObservationSummary, "goal_text"))
	targetRef := extractTargetRef(in.TargetRef, in.ObservationSummary, in.RequestContext)
	observationID := firstNonEmpty(findTextByKey(in.ObservationSummary, "observation_id"), findTextByKey(in.RequestContext, "observation_id"))
	mixSessionID := firstNonEmpty(findTextByKey(in.ObservationSummary, "mix_session_id"), findTextByKey(in.RequestContext, "mix_session_id"))
	actionKind := strings.TrimSpace(in.ActionKind)
	processorType := strings.TrimSpace(in.ProcessorType)
	problemKind := classifyProblemKind(intent, actionKind, processorType)

	createdAt := strings.TrimSpace(in.CreatedAt)
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	ctx := Context{
		SchemaVersion:  SchemaVersion,
		ID:             stableID(in.ConversationID, in.GoalID, in.RunID, observationID, intent, targetRef, actionKind, processorType),
		ConversationID: strings.TrimSpace(in.ConversationID),
		GoalID:         strings.TrimSpace(in.GoalID),
		RunID:          strings.TrimSpace(in.RunID),
		ObservationID:  observationID,
		MixSessionID:   mixSessionID,
		UserIntent:     intent,
		TargetRef:      targetRef,
		ProblemKind:    problemKind,
		CreatedAt:      createdAt,
		SourceRefs:     map[string]string{},
	}
	if observationID != "" {
		ctx.SourceRefs["observation_id"] = observationID
	}
	if mixSessionID != "" {
		ctx.SourceRefs["mix_session_id"] = mixSessionID
	}
	if targetRef != "" {
		ctx.SourceRefs["target_ref"] = targetRef
	}
	ctx.addEvidenceRef(ctx.ID)
	if observationID != "" {
		ctx.addEvidenceRef(observationID)
	}
	if intent != "" {
		ctx.UserReportedFacts = append(ctx.UserReportedFacts, Fact{
			Key:     "user_intent",
			Summary: userIntentSummary(problemKind, intent),
			Ref:     ctx.ID + ".user_reported.intent",
		})
		ctx.addEvidenceRef(ctx.ID + ".user_reported.intent")
	}

	band := bandEvidence(in.ObservationSummary, in.RequestContext)
	waveform := waveformEvidence(in.ObservationSummary, in.RequestContext)
	stereo := stereoEvidence(in.ObservationSummary, in.RequestContext)
	spectralStatus := spectralEvidenceStatus(in.ObservationSummary, in.RequestContext)
	missingMetrics := stringSliceFromAny(firstValueByKey(in.ObservationSummary, "missing_metrics"))

	if evidenceReady(cleanText(waveform["status"])) {
		ctx.ObservedFacts = append(ctx.ObservedFacts, Fact{
			Key:     "waveform_summary",
			Summary: "waveform/headroom metrics were observed in the latest mix observation",
			Ref:     sourceRef(ctx, "observed.waveform_summary"),
			Data:    cloneMap(waveform),
		})
		ctx.addEvidenceRef(sourceRef(ctx, "observed.waveform_summary"))
	}
	if band.Ready || band.Partial {
		ctx.ObservedFacts = append(ctx.ObservedFacts, Fact{
			Key:     "band_energy_summary",
			Summary: band.summary(),
			Ref:     sourceRef(ctx, "observed.band_energy_summary"),
			Data:    cloneMap(band.Row),
		})
		ctx.addEvidenceRef(sourceRef(ctx, "observed.band_energy_summary"))
		if targetRef != "" {
			ctx.addEvidenceRef(strings.TrimPrefix(targetRef, "track:") + ".slow.band_energy.summary")
		}
	}
	if evidenceObserved(cleanText(stereo["status"])) {
		ctx.ObservedFacts = append(ctx.ObservedFacts, Fact{
			Key:     "stereo_relation_summary",
			Summary: stereoSummary(stereo),
			Ref:     sourceRef(ctx, "observed.stereo_relation_summary"),
			Data:    cloneMap(stereo),
		})
		ctx.addEvidenceRef(sourceRef(ctx, "observed.stereo_relation_summary"))
	}

	switch problemKind {
	case "low_mud":
		buildLowMudContext(&ctx, band, spectralStatus, missingMetrics)
	case "vocal_forward":
		buildVocalForwardContext(&ctx, band, targetRef)
	case "pan_balance":
		buildPanContext(&ctx, targetRef)
	case "gain_balance":
		buildGainContext(&ctx, targetRef)
	default:
		buildGenericContext(&ctx, band)
	}
	ctx.EvidenceStatus = map[string]any{
		"problem_kind":       ctx.ProblemKind,
		"band_energy":        band.readinessLabel(),
		"spectral":           firstNonEmpty(spectralStatus, "missing"),
		"strategy":           ctx.Recommendation.Strategy,
		"confidence_ceiling": ctx.Recommendation.Confidence,
		"missing":            missingEvidenceKeys(ctx.MissingEvidence),
	}
	return ctx
}

type bandState struct {
	Row     map[string]any
	Status  string
	Ready   bool
	Partial bool
}

func buildLowMudContext(ctx *Context, band bandState, spectralStatus string, missingMetrics []string) {
	ctx.ProblemKind = "low_mud"
	if band.Ready {
		ctx.InferredFacts = append(ctx.InferredFacts, Fact{
			Key:     "low_mud_candidate",
			Summary: "low/low-mid cleanup can be considered with band_energy_summary available; the observed band table should drive the exact EQ choice",
			Ref:     sourceRef(*ctx, "inferred.low_mud_candidate"),
			Data:    compactBandExplanation(band.Row),
		})
		ctx.Recommendation = Recommendation{
			Strategy:      "eq_cut_low_mid",
			NextStep:      "prepare a small low/low-mid EQ cut candidate and keep it pending for user confirmation",
			ActionKind:    firstNonEmpty(ctxActionKind(ctx), "plugin_treatment"),
			ProcessorType: "eq",
			Risk:          "low",
			Confidence:    "medium_high",
		}
		ctx.LowRiskNextStep = "eq_cut_low_mid"
		return
	}
	if band.Partial {
		ctx.InferredFacts = append(ctx.InferredFacts, Fact{
			Key:     "low_mud_partial_evidence",
			Summary: "partial band_energy_summary is available from returned spectral tiles; low-frequency cleanup should remain a conservative probe and must not be framed as a full-song conclusion",
			Ref:     sourceRef(*ctx, "inferred.low_mud_partial_evidence"),
			Data:    compactBandExplanation(band.Row),
		})
		ctx.addMissing("band_energy_summary", "band evidence is partial_from_spectral_tiles, so full-song low-frequency buildup remains unverified", false)
		ctx.Recommendation = Recommendation{
			Strategy:      "conservative_probe",
			NextStep:      "prepare only a small reversible EQ probe, citing the partial spectral-tile coverage before asking for confirmation",
			ActionKind:    firstNonEmpty(ctxActionKind(ctx), "plugin_treatment"),
			ProcessorType: "eq",
			Risk:          "low",
			Confidence:    "medium",
		}
		ctx.LowRiskNextStep = "conservative_probe"
		return
	}
	ctx.InferredFacts = append(ctx.InferredFacts, Fact{
		Key:     "low_mud_hypothesis",
		Summary: "the low-frequency muddiness is user-reported; no reliable band_energy_summary is available, so low-frequency buildup must not be treated as observed",
		Ref:     sourceRef(*ctx, "inferred.low_mud_hypothesis"),
	})
	ctx.addMissing("band_energy_summary", "low-mud diagnosis needs band energy before claiming observed low-frequency buildup", true)
	if !evidenceReady(spectralStatus) {
		ctx.addMissing("spectrogram_tiles", "spectral detail is unavailable, so frequency-specific explanation must stay conservative", false)
	}
	for _, key := range missingMetrics {
		if strings.Contains(strings.ToLower(key), "band_energy") {
			ctx.addMissing("band_energy_summary", "mix_package.missing_metrics reported band_energy_summary", true)
		}
	}
	ctx.Recommendation = Recommendation{
		Strategy:      "conservative_probe",
		NextStep:      "prepare only a small reversible EQ probe, or request a deeper observation before making a stronger claim",
		ActionKind:    firstNonEmpty(ctxActionKind(ctx), "plugin_treatment"),
		ProcessorType: "eq",
		Risk:          "low",
		Confidence:    "medium",
	}
	ctx.LowRiskNextStep = "conservative_probe"
}

func buildVocalForwardContext(ctx *Context, band bandState, targetRef string) {
	if ambiguousTarget(targetRef) {
		ctx.addMissing("target_track_identity", "vocal-forward treatment needs the vocal track identity before suggesting a level or processing move", true)
		ctx.InferredFacts = append(ctx.InferredFacts, Fact{
			Key:     "vocal_target_ambiguous",
			Summary: "the user wants the vocal forward, but the current target is not a confirmed vocal track",
			Ref:     sourceRef(*ctx, "inferred.vocal_target_ambiguous"),
		})
		ctx.Recommendation = Recommendation{
			Strategy:      "request_target_clarification",
			NextStep:      "ask which track is the vocal before preparing a mix action",
			ActionKind:    "request_clarification",
			ProcessorType: "none",
			Risk:          "none",
			Confidence:    "low",
		}
		ctx.LowRiskNextStep = "request_target_clarification"
		return
	}
	if !band.Ready {
		ctx.addMissing("band_energy_summary", "vocal-forward decisions should avoid brightness or presence assumptions without band/masking evidence", false)
		ctx.addMissing("masking_depth", "masking depth is unavailable, so do not infer a precise presence-band conflict", false)
	}
	ctx.InferredFacts = append(ctx.InferredFacts, Fact{
		Key:     "vocal_forward_safe_route",
		Summary: "prefer a conservative level or peer rebalance move; avoid tonal-brightness boosts without masking evidence",
		Ref:     sourceRef(*ctx, "inferred.vocal_forward_safe_route"),
	})
	ctx.Recommendation = Recommendation{
		Strategy:      "conservative_level_or_peer_rebalance",
		NextStep:      "prepare a small level/rebalance candidate instead of a brightness boost",
		ActionKind:    "gain_balance",
		ProcessorType: "utility",
		Risk:          "low",
		Confidence:    "medium",
	}
	ctx.LowRiskNextStep = "conservative_level_or_peer_rebalance"
}

func buildPanContext(ctx *Context, targetRef string) {
	if ambiguousTarget(targetRef) {
		ctx.addMissing("target_track_identity", "pan/balance changes require a concrete target track", true)
		ctx.Recommendation = Recommendation{
			Strategy:      "request_target_clarification",
			NextStep:      "ask which track should move in the stereo field",
			ActionKind:    "request_clarification",
			ProcessorType: "none",
			Risk:          "none",
			Confidence:    "low",
		}
		ctx.LowRiskNextStep = "request_target_clarification"
		return
	}
	ctx.InferredFacts = append(ctx.InferredFacts, Fact{
		Key:     "pan_band_independent",
		Summary: "a small pan/balance adjustment can be prepared from the user's direction and confirmed target; band_energy is not a blocking dependency",
		Ref:     sourceRef(*ctx, "inferred.pan_band_independent"),
	})
	ctx.Recommendation = Recommendation{
		Strategy:      "small_pan_adjust",
		NextStep:      "prepare a small reversible pan move and wait for explicit confirmation",
		ActionKind:    "pan_balance",
		ProcessorType: "utility",
		Risk:          "low",
		Confidence:    "high",
	}
	ctx.LowRiskNextStep = "small_pan_adjust"
}

func buildGainContext(ctx *Context, targetRef string) {
	if ambiguousTarget(targetRef) {
		ctx.addMissing("target_track_identity", "gain changes require a concrete target track", true)
		ctx.LowRiskNextStep = "request_target_clarification"
		ctx.Recommendation = Recommendation{Strategy: "request_target_clarification", NextStep: "ask which track should move in level", ActionKind: "request_clarification", ProcessorType: "none", Risk: "none", Confidence: "low"}
		return
	}
	ctx.Recommendation = Recommendation{Strategy: "small_gain_adjust", NextStep: "prepare a small reversible gain move and wait for explicit confirmation", ActionKind: "gain_balance", ProcessorType: "utility", Risk: "low", Confidence: "medium_high"}
	ctx.LowRiskNextStep = "small_gain_adjust"
}

func buildGenericContext(ctx *Context, band bandState) {
	if !band.Ready {
		ctx.addMissing("band_energy_summary", "frequency-specific diagnosis is unavailable without band energy evidence", false)
	}
	ctx.Recommendation = Recommendation{Strategy: "request_more_observation", NextStep: "summarize available facts and request more observation before a specific mix move", ActionKind: "observation_only", ProcessorType: "none", Risk: "none", Confidence: "low"}
	ctx.LowRiskNextStep = "request_more_observation"
}

func (ctx *Context) addMissing(key, reason string, blocking bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	for i := range ctx.MissingEvidence {
		if ctx.MissingEvidence[i].Key == key {
			if blocking {
				ctx.MissingEvidence[i].Blocking = true
			}
			if ctx.MissingEvidence[i].Reason == "" {
				ctx.MissingEvidence[i].Reason = reason
			}
			return
		}
	}
	ctx.MissingEvidence = append(ctx.MissingEvidence, MissingEvidence{Key: key, Reason: reason, Blocking: blocking})
	ctx.addEvidenceRef(sourceRef(*ctx, "missing."+key))
}

func (ctx *Context) addEvidenceRef(ref string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return
	}
	for _, existing := range ctx.EvidenceRefs {
		if existing == ref {
			return
		}
	}
	ctx.EvidenceRefs = append(ctx.EvidenceRefs, ref)
}

func bandEvidence(rows ...map[string]any) bandState {
	row := map[string]any{}
	for _, candidate := range rows {
		if found := findMapByKey(candidate, "band_energy_summary", "band_energy"); len(found) > 0 {
			row = found
			break
		}
	}
	status := cleanText(row["status"])
	for _, candidate := range rows {
		if status != "" {
			break
		}
		status = findTextByKey(candidate, "band_energy_status")
	}
	ready := evidenceReady(status)
	partial := strings.EqualFold(strings.TrimSpace(status), "partial")
	if status == "" && len(row) > 0 {
		status = "ready"
		ready = true
	}
	return bandState{Row: row, Status: status, Ready: ready, Partial: partial}
}

func (band bandState) summary() string {
	if len(band.Row) == 0 {
		return "band_energy_summary is unavailable"
	}
	detail := bandText(band.Row)
	if band.Partial {
		if detail == "" {
			return "band_energy_summary is partial; coverage is limited to returned spectral tiles"
		}
		return "band_energy_summary is partial from returned spectral tiles: " + detail
	}
	if detail == "" {
		return "band_energy_summary is ready"
	}
	return "band_energy_summary is ready: " + detail
}

func (band bandState) readinessLabel() string {
	if band.Ready {
		return "ready"
	}
	if band.Partial {
		return "partial"
	}
	return "missing"
}

func waveformEvidence(rows ...map[string]any) map[string]any {
	for _, row := range rows {
		if found := findMapByKey(row, "waveform"); len(found) > 0 {
			return found
		}
	}
	return nil
}

func stereoEvidence(rows ...map[string]any) map[string]any {
	for _, row := range rows {
		if found := findMapByKey(row, "stereo_relation", "stereo_relation_summary"); len(found) > 0 {
			return found
		}
	}
	return nil
}

func stereoSummary(row map[string]any) string {
	if strings.EqualFold(cleanText(row["status"]), "partial") {
		return "stereo balance/correlation metrics were partially observed from returned spectral tiles"
	}
	return "stereo balance/correlation metrics were observed in the latest mix observation"
}

func spectralEvidenceStatus(rows ...map[string]any) string {
	for _, row := range rows {
		if text := findTextByKey(row, "deep_band_observation", "spectrogram_tiles", "spectrogram_status"); text != "" {
			return strings.ToLower(text)
		}
	}
	return "missing"
}

func compactBandExplanation(row map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"status", "source", "derivation_status", "track_id", "target_ref", "bands", "tile_count_seen", "tile_count_expected", "tile_count_parsed", "coverage_seconds", "total_duration", "updated_at"} {
		if value, ok := row[key]; ok {
			out[key] = value
		}
	}
	return out
}

func classifyProblemKind(intent, actionKind, processorType string) string {
	text := strings.ToLower(strings.TrimSpace(intent + " " + actionKind + " " + processorType))
	action := strings.ToLower(strings.TrimSpace(actionKind))
	if mixDiagnosisObservationOnlyKind(intent, actionKind, processorType) {
		return "generic_mix_diagnosis"
	}
	if action == "pan_balance" {
		return "pan_balance"
	}
	switch {
	case containsAny(text, "声像", "pan ", "pan_", "pan-", "panning", "靠左", "靠右", "左边", "右边") || strings.EqualFold(strings.TrimSpace(actionKind), "pan_balance"):
		return "pan_balance"
	case containsAny(text, "人声", "主唱", "vocal", "forward", "靠前", "更前", "突出"):
		return "vocal_forward"
	case containsAny(text, "低频", "低中频", "糊", "浑浊", "mud", "muddy", "low end", "low-end", "low mid", "low-mid"):
		return "low_mud"
	case strings.EqualFold(strings.TrimSpace(actionKind), "gain_balance"):
		return "gain_balance"
	default:
		return "generic_mix_diagnosis"
	}
}

func mixDiagnosisObservationOnlyKind(intent, actionKind, processorType string) bool {
	intentText := strings.ToLower(strings.TrimSpace(intent))
	action := strings.ToLower(strings.TrimSpace(actionKind))
	processor := strings.ToLower(strings.TrimSpace(processorType))
	if action == "observation_only" || processor == "none" {
		return true
	}
	if containsAny(intentText,
		"read only", "read-only", "readonly", "observe only", "observation only", "analysis only", "analyze only", "analyse only",
		"no changes", "do not modify", "don't modify", "do not execute", "do not apply",
		"\u53ea\u8bfb", "\u53ea\u89c2\u5bdf", "\u53ea\u5206\u6790", "\u4e0d\u8981\u6267\u884c", "\u4e0d\u6267\u884c",
		"\u4e0d\u8981\u4fee\u6539", "\u4e0d\u4fee\u6539", "\u4e0d\u505a\u4efb\u4f55\u6539\u52a8", "\u4e0d\u6539\u5de5\u7a0b",
	) {
		return true
	}
	hasObservationVerb := containsAny(intentText,
		"observe", "check", "inspect", "analyze", "analyse", "analysis", "look at", "status",
		"\u89c2\u5bdf", "\u770b\u4e00\u4e0b", "\u770b\u770b", "\u5206\u6790", "\u68c0\u67e5", "\u72b6\u6001",
	)
	hasStereoStatusSubject := containsAny(intentText,
		"stereo status", "stereo image", "stereo field", "phase", "correlation", "width", "panning status", "pan status", "pan position", "leaning left", "leaning right",
		"\u58f0\u50cf\u72b6\u6001", "\u58f0\u76f8\u72b6\u6001", "\u58f0\u573a\u72b6\u6001", "\u76f8\u4f4d", "\u76f8\u5173\u5ea6", "\u5bbd\u5ea6", "\u504f\u5de6", "\u504f\u53f3",
	)
	hasPanMove := containsAny(intentText,
		"move left", "move right", "pan left", "pan right", "adjust pan", "set pan", "center it", "centre it",
		"\u79fb\u5230\u5de6", "\u79fb\u5230\u53f3", "\u5f80\u5de6", "\u5f80\u53f3", "\u8c03\u6574\u58f0\u50cf", "\u8c03\u58f0\u50cf", "\u6539\u58f0\u50cf", "\u6446\u5230",
	)
	return hasObservationVerb && hasStereoStatusSubject && !hasPanMove
}

func userIntentSummary(problemKind, intent string) string {
	switch problemKind {
	case "low_mud":
		return "user reported a low-frequency or low-mid muddiness issue: " + intent
	case "vocal_forward":
		return "user asked for a vocal-forward mix result: " + intent
	case "pan_balance":
		return "user requested a pan/balance move: " + intent
	default:
		return "user reported mix intent: " + intent
	}
}

func extractTargetRef(explicit string, rows ...map[string]any) string {
	if normalized := normalizeTargetRef(explicit); normalized != "" && normalized != "unknown" {
		return normalized
	}
	for _, row := range rows {
		if target := findMapByKey(row, "target_ref", "target"); len(target) > 0 {
			kind := strings.ToLower(firstNonEmpty(cleanText(target["kind"]), cleanText(target["type"])))
			id := firstNonEmpty(cleanText(target["id"]), cleanText(target["track_id"]))
			if kind == "track" && id != "" {
				return "track:" + id
			}
			if kind != "" {
				return kind
			}
		}
		if target := normalizeTargetRef(firstNonEmpty(findTextByKey(row, "target_ref"), findTextByKey(row, "target"))); target != "" && target != "unknown" {
			return target
		}
	}
	return normalizeTargetRef(explicit)
}

func normalizeTargetRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	lower := strings.ToLower(ref)
	if lower == "unknown" || lower == "project" {
		return lower
	}
	if strings.HasPrefix(lower, "track:") {
		id := strings.TrimSpace(ref[len("track:"):])
		if id == "" {
			return "unknown"
		}
		return "track:" + id
	}
	return ref
}

func ambiguousTarget(targetRef string) bool {
	targetRef = strings.ToLower(strings.TrimSpace(targetRef))
	return targetRef == "" || targetRef == "unknown" || targetRef == "project" || targetRef == "track:"
}

func evidenceReady(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "available", "ok":
		return true
	default:
		return false
	}
}

func evidenceObserved(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "available", "ok", "partial":
		return true
	default:
		return false
	}
}

func readinessLabel(ready bool) string {
	if ready {
		return "ready"
	}
	return "missing"
}

func missingEvidenceKeys(rows []MissingEvidence) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Key) != "" {
			out = append(out, row.Key)
		}
	}
	return out
}

func sourceRef(ctx Context, suffix string) string {
	if ctx.ID == "" {
		return strings.TrimSpace(suffix)
	}
	return ctx.ID + "." + strings.TrimSpace(suffix)
}

func ctxActionKind(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	switch ctx.ProblemKind {
	case "pan_balance":
		return "pan_balance"
	case "vocal_forward":
		return "gain_balance"
	case "low_mud":
		return "plugin_treatment"
	default:
		return ""
	}
}

func stableID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(joined))
	return "diag_" + hex.EncodeToString(sum[:])[:16]
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstValueByKey(row map[string]any, keys ...string) any {
	if len(row) == 0 {
		return nil
	}
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	for _, value := range row {
		if nested := firstValueByKey(mapValue(value), keys...); nested != nil {
			return nested
		}
	}
	return nil
}

func findTextByKey(row map[string]any, keys ...string) string {
	if len(row) == 0 {
		return ""
	}
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if text := cleanText(value); text != "" {
				return text
			}
		}
	}
	for _, value := range row {
		if text := findTextByKey(mapValue(value), keys...); text != "" {
			return text
		}
		for _, nested := range rowsValue(value) {
			if text := findTextByKey(nested, keys...); text != "" {
				return text
			}
		}
	}
	return ""
}

func findMapByKey(row map[string]any, keys ...string) map[string]any {
	return findMapByKeyDepth(row, 0, keys...)
}

func findMapByKeyDepth(row map[string]any, depth int, keys ...string) map[string]any {
	if len(row) == 0 || depth > 6 {
		return nil
	}
	for _, key := range keys {
		if found := mapValue(row[key]); len(found) > 0 {
			return found
		}
	}
	for _, value := range row {
		if found := findMapByKeyDepth(mapValue(value), depth+1, keys...); len(found) > 0 {
			return found
		}
		for _, nested := range rowsValue(value) {
			if found := findMapByKeyDepth(nested, depth+1, keys...); len(found) > 0 {
				return found
			}
		}
	}
	return nil
}

func mapValue(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case map[string]string:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = value
		}
		return out
	default:
		return nil
	}
}

func rowsValue(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if mapped := mapValue(row); len(mapped) > 0 {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return nil
	}
}

func cleanText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	data, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}

func stringSliceFromAny(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := cleanText(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	default:
		return nil
	}
}

func bandText(row map[string]any) string {
	bands := mapValue(row["bands"])
	if len(bands) == 0 {
		return ""
	}
	keys := make([]string, 0, len(bands))
	for key := range bands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		if text := cleanText(bands[key]); text != "" {
			parts = append(parts, key+"="+text)
		}
	}
	return strings.Join(parts, "; ")
}
