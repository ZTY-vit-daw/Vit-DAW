package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	semanticTreatmentWorkflow = "semantic_treatment_strategy"
	semanticTreatmentSchema   = "semantic_treatment_strategy.v1"
)

type semanticTreatmentInstance struct {
	Key                  string                                     `json:"instance_key"`
	TrackID              string                                     `json:"track_id"`
	PluginID             string                                     `json:"plugin_id"`
	PluginName           string                                     `json:"plugin_name,omitempty"`
	ProcessorType        string                                     `json:"processor_type"`
	QualificationStatus  string                                     `json:"qualification_status"`
	NextPlanner          string                                     `json:"next_planner"`
	Topology             map[string]any                             `json:"generic_eq_topology,omitempty"`
	IdentityCard         *semanticeffect.AudioProcessorIdentityCard `json:"processor_identity_card,omitempty"`
	QualifiedSurfaces    []semanticProcessorSurface                 `json:"qualified_surfaces,omitempty"`
	Limitation           string                                     `json:"limitation,omitempty"`
	PCAReviewed          bool                                       `json:"-"`
	PCAEligible          bool                                       `json:"-"`
	PCAStatus            string                                     `json:"-"`
	PCAReason            string                                     `json:"-"`
	PCASubjectKey        string                                     `json:"-"`
	PCABinaryFingerprint string                                     `json:"-"`
	PCAAttestationID     string                                     `json:"-"`
	PCARequiredCoverage  []processorattestation.Coverage            `json:"-"`
}

// semanticProcessorSurface is an independent qualified capability exposed by
// one loaded plugin instance. A mixed plugin is represented by multiple rows;
// no deterministic family priority is allowed to collapse them into one.
type semanticProcessorSurface struct {
	SurfaceKey           string                                     `json:"surface_key"`
	Family               string                                     `json:"family"`
	PCAFamily            string                                     `json:"pca_family"`
	QualificationStatus  string                                     `json:"qualification_status"`
	NextPlanner          string                                     `json:"next_planner"`
	CoverageVocabulary   []string                                   `json:"coverage_vocabulary,omitempty"`
	ObservationViews     []string                                   `json:"observation_views,omitempty"`
	OwnedParameterIDs    []string                                   `json:"owned_parameter_ids,omitempty"`
	Topology             map[string]any                             `json:"topology,omitempty"`
	IdentityCard         *semanticeffect.AudioProcessorIdentityCard `json:"processor_identity_card,omitempty"`
	Limitation           string                                     `json:"limitation,omitempty"`
	InspectOnly          bool                                       `json:"inspect_only,omitempty"`
	PCAReviewed          bool                                       `json:"-"`
	PCAEligible          bool                                       `json:"-"`
	PCAStatus            string                                     `json:"-"`
	PCAReason            string                                     `json:"-"`
	PCASubjectKey        string                                     `json:"-"`
	PCABinaryFingerprint string                                     `json:"-"`
	PCAAttestationID     string                                     `json:"-"`
	PCARequiredCoverage  []processorattestation.Coverage            `json:"-"`
}

type semanticTreatmentChoice struct {
	ChoiceKey      string `json:"choice_key"`
	Role           string `json:"role"`
	Title          string `json:"title"`
	ProcessorType  string `json:"processor_type"`
	TargetMode     string `json:"target_mode"`
	InstanceKey    string `json:"instance_key,omitempty"`
	Reason         string `json:"reason"`
	ExpectedEffect string `json:"expected_effect"`
	Tradeoff       string `json:"tradeoff,omitempty"`
	Confidence     string `json:"confidence"`
	NextPlanner    string `json:"next_planner"`
	MateriallyDiff string `json:"material_difference,omitempty"`
}

type semanticTreatmentPlan struct {
	SchemaVersion     string                    `json:"schema_version"`
	DecisionMode      string                    `json:"decision_mode"`
	UserGoal          string                    `json:"user_goal"`
	Summary           string                    `json:"summary"`
	Choices           []semanticTreatmentChoice `json:"choices"`
	GlobalConstraints []string                  `json:"global_constraints,omitempty"`
	EvidenceRefs      []string                  `json:"evidence_refs,omitempty"`
	Limitations       []string                  `json:"limitations,omitempty"`
}

func ordinaryAgentSemanticEQMutationRequest(userText string, requestContext map[string]any) bool {
	return contextHasAnyValue(requestContext, "selected_track_id", "selected_plugin_track_id") &&
		ordinaryAgentSemanticEQTextTopic(userText) && ordinaryAgentSemanticEQMutationIntent(userText, true)
}

// This is deliberately only a routing gate. It identifies open-ended audible
// change requests; the LLM, not these words, chooses the treatment method.
func ordinaryAgentTreatmentStrategyIntent(userText string, requestContext map[string]any) bool {
	if !contextHasAnyValue(requestContext, "selected_track_id", "selected_plugin_track_id") ||
		ordinaryAgentPluginRecommendationIntent(userText, requestContext) ||
		ordinaryAgentSemanticEQMutationRequest(userText, requestContext) {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || agentLoopTextHasAny(text,
		"为什么", "為什麼", "什么原因", "什麼原因", "分析一下", "解释一下", "解釋一下",
		"why", "what causes", "how do you know", "analyse", "analyze", "explain") {
		return false
	}
	goal := agentLoopTextHasAny(text,
		"靠前", "靠后", "靠後", "有力量", "更有力", "温暖", "溫暖", "更厚", "更薄", "更贴", "更貼",
		"更稳", "更穩", "更松", "更紧", "更緊", "更清晰", "更通透", "更自然", "更有空间", "更有空間",
		"站到前面", "站出来", "站出來", "清楚", "稳定", "穩定", "不稳", "不穩", "时大时小", "時大時小",
		"均匀", "均勻", "收稳", "收穩", "突出", "盖住", "蓋住", "发闷", "發悶", "糊", "存在感",
		"forward", "up front", "powerful", "stronger", "warmer", "thicker", "thinner", "closer", "stable", "tighter", "clearer", "open", "natural", "depth")
	action := agentLoopTextHasAny(text,
		"让", "讓", "使", "变", "變", "更", "弄得", "处理", "處理", "调整", "調整", "改善", "增加", "减少", "減少",
		"把", "帮", "幫", "整理", "收稳", "收穩",
		"make", "bring", "move", "sound", "increase", "reduce", "improve", "adjust")
	return goal && action
}

func (s *Server) semanticTreatmentInstances(ctx context.Context, trackID string, pcaInputs ...any) ([]semanticTreatmentInstance, string) {
	if s == nil || s.harness == nil || strings.TrimSpace(trackID) == "" {
		return nil, ""
	}
	state := s.harness.UserStateSummary(ctx)
	client := s.eqKernelClient()
	if client != nil {
		if liveState, _, err := client.SendCommand(ctx, map[string]any{"cmd": "get_project_state"}); err == nil && kernelReplyOK(liveState) {
			state = liveState
		}
	}
	refs := chatVisiblePluginRefs(state, trackID)
	if len(refs) > 16 {
		refs = refs[:16]
	}
	out := make([]semanticTreatmentInstance, 0, len(refs))
	var pcaInput semanticTreatmentPCAInput
	var pcaInputErr error
	if len(pcaInputs) > 0 && pcaInputs[0] != nil {
		switch value := pcaInputs[0].(type) {
		case map[string]any:
			pcaInput, pcaInputErr = semanticTreatmentPCAInputFromIntent(value)
		case semanticTreatmentPCAInput:
			pcaInput = value
		default:
			pcaInput, pcaInputErr = semanticTreatmentPCAInputFromRequirement(value)
		}
	}
	for index, ref := range refs {
		instance := semanticTreatmentInstance{
			Key:                 fmt.Sprintf("loaded_instance_%d", index+1),
			TrackID:             ref.TrackID,
			PluginID:            ref.ID,
			PluginName:          ref.Name,
			ProcessorType:       "unknown",
			QualificationStatus: "identity_only",
			NextPlanner:         "capability_boundary",
			Limitation:          "The loaded identity is real, but no ordinary-Agent abstract parameter planner has qualified it.",
		}
		refIdentity := semanticTreatmentSubjectFromRef(ref)
		if pcaInputErr != nil {
			instance.PCAReviewed = true
			instance.PCAStatus = "rejected"
			instance.PCAReason = pcaInputErr.Error()
			instance.Limitation = appendSemanticTreatmentLimitation(instance.Limitation, "PCA admission: "+pcaInputErr.Error())
		}
		if client == nil {
			instance.Limitation = "kernel client is unavailable"
			out = append(out, instance)
			continue
		}
		reply, _, err := client.SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": ref.TrackID,
			"plugin_id": ref.ID, "include_parameters": true})
		if err != nil || !kernelReplyOK(reply) {
			instance.Limitation = firstNonEmpty(errorText(err), firstNonEmptyText(reply, "message", "error"), "parameter read failed")
			out = append(out, instance)
			continue
		}
		s.observePluginParametersReply(reply)
		digest := plugingrabber.BuildParameterDigest(reply)
		if digest.TrackID == "" {
			digest.TrackID = ref.TrackID
		}
		if digest.PluginID == "" {
			digest.PluginID = ref.ID
		}
		if digest.PluginName == "" {
			digest.PluginName = ref.Name
		}
		if digest.PluginIdentifier == "" {
			digest.PluginIdentifier = ref.Identifier
		}
		if digest.PluginPath == "" {
			digest.PluginPath = ref.Path
		}
		if digest.PluginFormat == "" {
			digest.PluginFormat = ref.Format
		}
		if digest.PluginManufacturer == "" {
			digest.PluginManufacturer = ref.Manufacturer
		}
		instance.QualifiedSurfaces, instance.Limitation = semanticTreatmentBuildSurfaces(ref.TrackID, ref.ID, digest)
		if pcaInputErr != nil {
			for index := range instance.QualifiedSurfaces {
				instance.QualifiedSurfaces[index].PCAReviewed = true
				instance.QualifiedSurfaces[index].PCAEligible = false
				instance.QualifiedSurfaces[index].PCAStatus = "rejected"
				instance.QualifiedSurfaces[index].PCAReason = pcaInputErr.Error()
				instance.QualifiedSurfaces[index].Limitation = appendSemanticTreatmentLimitation(instance.QualifiedSurfaces[index].Limitation, "PCA admission: "+pcaInputErr.Error())
			}
		}
		if pcaInputErr == nil && len(pcaInputs) > 0 && pcaInputs[0] != nil {
			instance.QualifiedSurfaces = qualifySemanticTreatmentSurfaces(instance.QualifiedSurfaces, digest, refIdentity, pcaInput)
			instance.PCAReviewed = true
			instance.PCARequiredCoverage = append([]processorattestation.Coverage(nil), pcaInput.RequiredCoverage...)
			for _, surface := range instance.QualifiedSurfaces {
				if surface.PCAReviewed {
					instance.PCAEligible = instance.PCAEligible || surface.PCAEligible
					instance.PCAStatus = firstNonEmpty(instance.PCAStatus, surface.PCAStatus)
					instance.PCAReason = firstNonEmpty(instance.PCAReason, surface.PCAReason)
					instance.PCASubjectKey = firstNonEmpty(instance.PCASubjectKey, surface.PCASubjectKey)
					instance.PCABinaryFingerprint = firstNonEmpty(instance.PCABinaryFingerprint, surface.PCABinaryFingerprint)
					instance.PCAAttestationID = firstNonEmpty(instance.PCAAttestationID, surface.PCAAttestationID)
				}
			}
		}
		if len(instance.QualifiedSurfaces) == 1 {
			surface := instance.QualifiedSurfaces[0]
			instance.ProcessorType = legacyProcessorTypeForFamily(surface.Family)
			instance.QualificationStatus = surface.QualificationStatus
			instance.NextPlanner = surface.NextPlanner
			instance.Topology = surface.Topology
			instance.IdentityCard = surface.IdentityCard
		} else if len(instance.QualifiedSurfaces) > 1 {
			instance.ProcessorType = "mixed"
			instance.QualificationStatus = "multiple_surfaces"
			instance.NextPlanner = "semantic_family_selection"
			instance.Topology = nil
			instance.IdentityCard = nil
			instance.Limitation = ""
		}
		out = append(out, instance)
	}
	return out, semanticTreatmentStateToken(state, trackID)
}

// semanticTreatmentSubjectFromRef recovers missing PCA subject fields from
// the deterministic local semantic index. Kernel rack snapshots commonly
// expose only the live node name/path/format; this lookup keeps identity
// binding server-owned and exact without leaking plugin identity to the LLM
// or weakening PCA admission.
func semanticTreatmentSubjectFromRef(ref chatPluginRef) processorattestation.Subject {
	subject := processorattestation.Subject{
		Name: ref.Name, Identifier: ref.Identifier, InstalledPath: ref.Path,
		Format: ref.Format, Manufacturer: ref.Manufacturer,
	}
	if subject.Identifier != "" && subject.Manufacturer != "" {
		return subject
	}
	index, err := pluginsemantics.Load("")
	if err != nil {
		return subject
	}
	matches := make([]pluginsemantics.Entry, 0, 2)
	for _, entry := range index.Entries {
		if subject.Name != "" && !strings.EqualFold(strings.TrimSpace(entry.Name), strings.TrimSpace(subject.Name)) {
			continue
		}
		if subject.InstalledPath != "" && !strings.EqualFold(strings.TrimSpace(entry.PluginPath), strings.TrimSpace(subject.InstalledPath)) {
			continue
		}
		matches = append(matches, entry)
	}
	if len(matches) != 1 {
		return subject
	}
	entry := matches[0]
	if subject.Identifier == "" {
		subject.Identifier = entry.Identifier
	}
	if subject.Manufacturer == "" {
		subject.Manufacturer = entry.Manufacturer
	}
	if subject.Format == "" {
		subject.Format = entry.Format
	}
	if subject.InstalledPath == "" {
		subject.InstalledPath = entry.PluginPath
	}
	return subject
}

func semanticTreatmentBuildSurfaces(trackID, pluginID string, digest plugingrabber.ParameterDigest) ([]semanticProcessorSurface, string) {
	registry, registryErr := processorregistry.Default()
	if registryErr != nil || registry == nil {
		return nil, "processor registry unavailable: " + firstNonEmpty(errorText(registryErr), "unknown registry error")
	}
	surfaces := make([]semanticProcessorSurface, 0, 7)
	limitations := make([]string, 0, 7)
	add := func(family, status, summaryBoundary string, topology map[string]any, card *semanticeffect.AudioProcessorIdentityCard) {
		if len(topology) == 0 && card == nil {
			if strings.TrimSpace(summaryBoundary) != "" {
				limitations = append(limitations, family+": "+summaryBoundary)
			}
			return
		}
		definition, ok := registry.Resolve(family)
		if !ok {
			limitations = append(limitations, family+": processor family is not registered")
			return
		}
		row := semanticProcessorSurface{
			SurfaceKey:          fmt.Sprintf("%s:%s:%s", trackID, pluginID, family),
			Family:              family,
			PCAFamily:           definition.PCAFamily,
			QualificationStatus: status,
			NextPlanner:         definition.Planner,
			CoverageVocabulary:  append([]string(nil), definition.CoverageVocabulary...),
			ObservationViews:    append([]string(nil), definition.ObservationViews...),
			OwnedParameterIDs:   semanticSurfaceOwnedParameterIDs(topology),
			Topology:            topology,
			IdentityCard:        card,
			Limitation:          summaryBoundary,
			InspectOnly:         definition.InspectOnly,
		}
		surfaces = append(surfaces, row)
	}

	compressorSummary, compressorBoundary := plugingrabber.BuildCompressorSummaryWithBoundary(digest)
	var compressorCard *semanticeffect.AudioProcessorIdentityCard
	cardBoundary := ""
	if len(compressorSummary) > 0 {
		compressorCard, cardBoundary = plugingrabber.BuildAudioProcessorIdentityCard(digest)
	}
	add(processorintent.FamilyBroadbandCompressor, "broadband_compressor_qualified", firstNonEmpty(cardBoundary, compressorBoundary), compressorSummary, compressorCard)

	eqSummary := plugingrabber.BuildEQBandSummary(digest)
	if len(eqSummary) > 0 && semanticTreatmentEQSurfaceIsIndependent(eqSummary, len(compressorSummary) > 0) {
		add(processorintent.FamilyStaticEQ, "generic_static_eq_qualified", "", semanticEQTopologyPromptSummary(trackID, pluginID, eqSummary), nil)
		// The model-facing EQ projection intentionally omits parameter IDs, but
		// the server still needs the private ownership set to detect cross-family
		// conflicts and fail closed before any typed write.
		for index := range surfaces {
			if surfaces[index].Family == processorintent.FamilyStaticEQ {
				surfaces[index].OwnedParameterIDs = semanticSurfaceOwnedParameterIDs(eqSummary)
			}
		}
	}

	limiterSummary, limiterBoundary := plugingrabber.BuildLimiterSummaryWithBoundary(digest)
	add(processorintent.FamilyLimiter, "limiter_topology_qualified", limiterBoundary, limiterSummary, nil)

	gateSummary, gateBoundary := plugingrabber.BuildGateExpanderSummaryWithBoundary(digest)
	add(processorintent.FamilyGateExpander, "gate_expander_topology_qualified", gateBoundary, gateSummary, nil)

	deEsserSummary, deEsserBoundary := plugingrabber.BuildDeEsserSummaryWithBoundary(digest)
	add(processorintent.FamilyDeEsser, "de_esser_topology_qualified", deEsserBoundary, deEsserSummary, nil)

	transientSummary, transientBoundary := plugingrabber.BuildTransientShaperSummaryWithBoundary(digest)
	add(processorintent.FamilyTransientShaper, "transient_shaper_topology_qualified", transientBoundary, transientSummary, nil)

	multibandSummary, multibandBoundary := plugingrabber.BuildMultibandSummaryWithBoundary(digest)
	add(processorintent.FamilyMultibandDynamics, "multiband_dynamics_topology_qualified", multibandBoundary, multibandSummary, nil)

	owners := map[string][]int{}
	for index, surface := range surfaces {
		for _, parameterID := range surface.OwnedParameterIDs {
			owners[parameterID] = append(owners[parameterID], index)
		}
	}
	for parameterID, indexes := range owners {
		if len(indexes) < 2 {
			continue
		}
		families := make([]string, 0, len(indexes))
		for _, index := range indexes {
			families = append(families, surfaces[index].Family)
		}
		for _, index := range indexes {
			surfaces[index].QualificationStatus = "ownership_conflict"
			surfaces[index].Limitation = fmt.Sprintf("parameter %s is claimed by multiple processor surfaces (%s)", parameterID, strings.Join(uniqueStrings(families), ", "))
		}
		limitations = append(limitations, surfaces[indexes[0]].Limitation)
	}

	return surfaces, strings.Join(uniqueStrings(limitations), "; ")
}

func semanticTreatmentEQSurfaceIsIndependent(summary map[string]any, compressorPresent bool) bool {
	if !compressorPresent {
		return true
	}
	sections := mapRowsValue(summary["sections"])
	if len(sections) == 0 {
		return false
	}
	for _, section := range sections {
		label := strings.ToLower(firstNonEmptyText(section, "section", "name", "role"))
		if label == "" || (!strings.Contains(label, "sidechain") && !strings.Contains(label, "side chain") && !strings.Contains(label, "detector")) {
			return true
		}
	}
	return false
}

func semanticSurfaceOwnedParameterIDs(topology map[string]any) []string {
	owned := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				key = strings.ToLower(strings.TrimSpace(key))
				if key == "param_id" || key == "parameter_id" || strings.HasSuffix(key, "_param_id") {
					if id := strings.TrimSpace(fmt.Sprint(child)); id != "" && id != "<nil>" {
						owned[id] = true
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case []map[string]any:
			for _, child := range typed {
				visit(child)
			}
		case map[string]string:
			for key, child := range typed {
				key = strings.ToLower(strings.TrimSpace(key))
				if key == "param_id" || key == "parameter_id" || strings.HasSuffix(key, "_param_id") {
					if id := strings.TrimSpace(child); id != "" {
						owned[id] = true
					}
				}
			}
		}
	}
	visit(topology)
	out := make([]string, 0, len(owned))
	for id := range owned {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func legacyProcessorTypeForFamily(family string) string {
	switch family {
	case processorintent.FamilyStaticEQ:
		return "eq"
	case processorintent.FamilyBroadbandCompressor:
		return "compressor"
	default:
		return family
	}
}

func processorFamilyForTreatmentProcessorType(processorType string) string {
	raw := strings.ToLower(strings.TrimSpace(processorType))
	if raw == processorintent.FamilyStaticEQ {
		return processorintent.FamilyStaticEQ
	}
	if raw == processorintent.FamilyBroadbandCompressor {
		return processorintent.FamilyBroadbandCompressor
	}
	switch canonicalPluginRecommendationProcessorType(processorType) {
	case "eq":
		return processorintent.FamilyStaticEQ
	case "compressor":
		return processorintent.FamilyBroadbandCompressor
	case "limiter":
		return processorintent.FamilyLimiter
	case "gate_expander", "gate", "expander":
		return processorintent.FamilyGateExpander
	case "de_esser", "deesser", "de-esser":
		return processorintent.FamilyDeEsser
	case "transient_shaper", "transient":
		return processorintent.FamilyTransientShaper
	case "multiband_dynamics", "multiband":
		return processorintent.FamilyMultibandDynamics
	default:
		return ""
	}
}

func semanticTreatmentStateToken(state map[string]any, trackID string) string {
	refs := chatVisiblePluginRefs(state, trackID)
	rows := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		rows = append(rows, map[string]any{"track_id": ref.TrackID, "plugin_id": ref.ID, "plugin_name": ref.Name})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left := firstStringFromMap(rows[i], "track_id") + "\x00" + firstStringFromMap(rows[i], "plugin_id")
		right := firstStringFromMap(rows[j], "track_id") + "\x00" + firstStringFromMap(rows[j], "plugin_id")
		return left < right
	})
	payload := map[string]any{
		"track_id": trackID, "plugins": rows,
		"project_uuid":   firstStringFromMap(state, "project_uuid", "project_id"),
		"project_path":   firstStringFromMap(state, "project_path", "current_project_path"),
		"graph_revision": state["graph_revision"], "project_revision": state["project_revision"],
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return "treatment_state_" + hex.EncodeToString(sum[:12])
}

func semanticTreatmentQualifiedEQ(instances []semanticTreatmentInstance) []semanticTreatmentInstance {
	out := make([]semanticTreatmentInstance, 0, len(instances))
	for _, instance := range instances {
		if semanticTreatmentInstanceHasFamilySurface(instance, processorintent.FamilyStaticEQ) ||
			(!instance.PCAReviewed && instance.ProcessorType == "eq" && instance.QualificationStatus == "generic_static_eq_qualified" && instance.NextPlanner == "semantic_eq") {
			out = append(out, instance)
		}
	}
	return out
}

func semanticTreatmentInstanceHasFamilySurface(instance semanticTreatmentInstance, family string) bool {
	for _, surface := range instance.QualifiedSurfaces {
		if surface.Family == family && semanticTreatmentSurfacePCAEligible(surface) {
			return true
		}
	}
	return false
}

func semanticTreatmentInstanceSurface(instance semanticTreatmentInstance, family string) (semanticProcessorSurface, bool) {
	for _, surface := range instance.QualifiedSurfaces {
		if surface.Family == family {
			return surface, true
		}
	}
	return semanticProcessorSurface{}, false
}

func semanticTreatmentSurfaceModelEnvelope(surface semanticProcessorSurface) semanticProcessorSurface {
	out := surface
	out.SurfaceKey = ""
	out.OwnedParameterIDs = nil
	out.Topology = nil
	out.IdentityCard = nil
	return out
}

func semanticTreatmentFindInstance(instances []semanticTreatmentInstance, key string) (semanticTreatmentInstance, bool) {
	for _, instance := range instances {
		if instance.Key == key {
			return instance, true
		}
	}
	return semanticTreatmentInstance{}, false
}

func semanticTreatmentFindPlugin(instances []semanticTreatmentInstance, pluginID string) (semanticTreatmentInstance, bool) {
	for _, instance := range instances {
		if instance.PluginID == pluginID {
			return instance, true
		}
	}
	return semanticTreatmentInstance{}, false
}

// semanticTreatmentModelInstances is the progressive-disclosure boundary.
// Family selection never receives loaded-instance identity or topology. Once
// the free-state model has selected a family, only candidates in that family
// are disclosed; topology/identity cards remain server-side until the exact
// instance key has been selected.
func semanticTreatmentModelInstances(instances []semanticTreatmentInstance, requiredProcessorType string) []semanticTreatmentInstance {
	requiredProcessorType = canonicalPluginRecommendationProcessorType(requiredProcessorType)
	if requiredProcessorType == "" {
		return nil
	}
	out := make([]semanticTreatmentInstance, 0, len(instances))
	for _, instance := range instances {
		family := processorFamilyForTreatmentProcessorType(requiredProcessorType)
		if family != "" {
			if !semanticTreatmentInstanceHasFamilySurface(instance, family) && canonicalPluginRecommendationProcessorType(instance.ProcessorType) != requiredProcessorType {
				continue
			}
		} else if canonicalPluginRecommendationProcessorType(instance.ProcessorType) != requiredProcessorType {
			continue
		}
		row := instance
		if family != "" {
			if surface, ok := semanticTreatmentInstanceSurface(instance, family); ok {
				row.ProcessorType = legacyProcessorTypeForFamily(surface.Family)
				row.QualificationStatus = surface.QualificationStatus
				row.NextPlanner = surface.NextPlanner
			}
		}
		row.PluginID = ""
		row.Topology = nil
		row.IdentityCard = nil
		if family != "" {
			if surface, ok := semanticTreatmentInstanceSurface(instance, family); ok {
				// The family has already been selected, so expose only the
				// matching capability envelope. Keep topology and identity for
				// the later exact-instance handoff, never for another family.
				row.QualifiedSurfaces = []semanticProcessorSurface{semanticTreatmentSurfaceModelEnvelope(surface)}
			} else {
				row.QualifiedSurfaces = nil
			}
		}
		out = append(out, row)
	}
	return out
}

func (s *Server) planSemanticTreatment(ctx context.Context, conversationID, userText, trackID, trackName string,
	observation *agentloop.RecentObservation, instances []semanticTreatmentInstance, forceInstanceChoice, nativeHandoffAvailable bool,
	cfg config.EngineConfig, requiredProcessorTypes ...any) (semanticTreatmentPlan, error) {
	if s == nil || s.llm == nil {
		return semanticTreatmentPlan{}, fmt.Errorf("semantic treatment LLM is unavailable")
	}
	if forceInstanceChoice && len(instances) == 0 {
		return semanticTreatmentPlan{}, fmt.Errorf("no qualified loaded EQ instance is available")
	}
	requiredProcessorType := ""
	var semanticIntent map[string]any
	for _, value := range requiredProcessorTypes {
		switch typed := value.(type) {
		case string:
			if requiredProcessorType == "" {
				requiredProcessorType = canonicalPluginRecommendationProcessorType(typed)
			}
		case map[string]any:
			if semanticIntent == nil {
				semanticIntent = typed
			}
		}
	}
	disclosureFamily := requiredProcessorType
	if forceInstanceChoice && disclosureFamily == "" {
		disclosureFamily = "eq"
	}
	modelInstances := semanticTreatmentModelInstances(instances, disclosureFamily)
	input := map[string]any{
		"user_request":                          userText,
		"target_scope":                          map[string]any{"kind": "track", "track_refs": []string{trackID}, "label": trackName},
		"relationship_refs":                     []string{},
		"observation_context":                   semanticEQPlannerObservation(observation),
		"loaded_plugin_instances":               modelInstances,
		"allowed_load_required_processor_types": []string{"eq", "compressor", "limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics"},
		"native_agent_result_handoff_available": nativeHandoffAvailable,
		"planning_mode":                         map[bool]string{true: "loaded_eq_instance_arbitration", false: "treatment_method_arbitration"}[forceInstanceChoice],
	}
	if requiredProcessorType != "" {
		input["required_processor_type"] = requiredProcessorType
		input["allowed_load_required_processor_types"] = []string{requiredProcessorType}
	}
	if len(semanticIntent) > 0 {
		input["semantic_processor_intent"] = cloneContext(semanticIntent)
	}
	inputJSON, _ := json.Marshal(input)
	system := `You are the treatment-strategy phase of an ordinary DAW Agent. This is a horizontal capability, not B4 or any A-F specialist workflow.
The user request, exact target scope, optional observation evidence, and real loaded instances are supplied as JSON. You own the acoustic and musical method judgement. Deterministic code owns identity binding, qualification, capability boundaries, confirmation, execution, readback, rollback, and verification.

Return ONLY one semantic_treatment_strategy.v1 JSON object:
	{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct|choice_required","user_goal":"copy user goal","summary":"short summary in the user's language","choices":[{"choice_key":"stable unique key","role":"recommended|alternative","title":"short method label","processor_type":"eq|compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics|native","target_mode":"existing_plugin|load_required|native","instance_key":"exact supplied key when existing_plugin","reason":"task-specific reason","expected_effect":"audible intent","tradeoff":"meaningful difference or limitation","confidence":"low|medium|high","next_planner":"semantic_eq|semantic_compressor|semantic_limiter|semantic_gate_expander|semantic_de_esser|semantic_transient_shaper|semantic_multiband|plugin_recommendation|existing_agent_result|capability_boundary","material_difference":"why this is not a duplicate"}],"global_constraints":[],"evidence_refs":[],"limitations":[]}

Rules:
- Do not invent a track, plugin, instance_key, observation, or executable capability.
- An existing_plugin choice must use an exact supplied instance_key. Use the supplied qualified surface's registry planner (for example semantic_eq, semantic_compressor, semantic_limiter, semantic_gate_expander, semantic_de_esser, semantic_transient_shaper, or semantic_multiband) only when that surface is PCA-eligible. An identity-only or inspect-only instance must use capability_boundary.
- A load_required choice omits instance_key and uses next_planner=plugin_recommendation. It means recommend/select/load first, not that parameters are already controllable.
- A native choice is allowed only when native_agent_result_handoff_available=true. It must be the sole direct recommendation and use next_planner=existing_agent_result; never place native in a three-way selection because that result cannot be frozen across this choice interaction.
- Use decision_mode=direct with exactly one recommended choice when one method is clearly appropriate and no meaningful user decision remains.
- Use decision_mode=choice_required only when methods have substantively different audible results, workflows, or tradeoffs. Then return exactly one recommended choice and two materially different alternatives.
- Do not pad alternatives with cosmetic variations. EQ versus compression versus saturation/space can be material; two copies of the same strategy are not.
- Respect an explicitly requested processor family as a hard method constraint.
- Preserve shared negative constraints in global_constraints.
- Do not output parameter values. A downstream planner owns concrete parameters after strategy selection.
- Do not use stored mappings, B4, plugin-specific control rules, or network search.`
	if forceInstanceChoice {
		system += `
- This request is loaded-EQ instance arbitration. Use only existing_plugin choices from supplied generic_static_eq_qualified instances, processor_type=eq, next_planner=semantic_eq, and decision_mode=choice_required. Offer up to three real instances (one recommended, remaining alternatives); when only one is supplied, return that one recommended choice without inventing alternatives.`
	}
	if requiredProcessorType != "" {
		system += `
- required_processor_type is the upstream free-state reasoning decision. Treat it as a hard family constraint, use decision_mode=direct with exactly one recommended choice of that processor_type, and decide only whether a qualified existing instance or load_required can materialize it. Do not offer or recommend another processor family.`
	}
	if len(semanticIntent) > 0 {
		system += `
- semantic_processor_intent.v1 is the upstream model-owned semantic handoff. Preserve its family, open intent, scope, evidence references, and required coverage exactly; deterministic code will validate them again. This strategy stage may choose only an exact qualified instance or a governed load-required handoff for that family.`
	}
	request := llm.Request{
		Messages:   []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(inputJSON)}},
		Metadata:   llm.RequestMetadata{Source: "ordinary_agent_semantic_treatment_strategy", ConversationID: conversationID},
		PreferJSON: true,
	}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return semanticTreatmentPlan{}, err
	}
	plan, decodeErr := decodeAndValidateSemanticTreatmentPlan(response.Text, userText, modelInstances, forceInstanceChoice, nativeHandoffAvailable)
	if decodeErr == nil {
		decodeErr = semanticTreatmentRequiredProcessorIssue(plan, requiredProcessorType)
	}
	if decodeErr == nil {
		return plan, nil
	}
	repair := fmt.Sprintf("Your previous semantic treatment JSON was invalid: %s\nReturn ONLY a corrected semantic_treatment_strategy.v1 object using the same supplied identities and capabilities.", decodeErr)
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: repair})
	response, err = s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return semanticTreatmentPlan{}, err
	}
	plan, err = decodeAndValidateSemanticTreatmentPlan(response.Text, userText, modelInstances, forceInstanceChoice, nativeHandoffAvailable)
	if err == nil {
		err = semanticTreatmentRequiredProcessorIssue(plan, requiredProcessorType)
	}
	if err != nil {
		return semanticTreatmentPlan{}, fmt.Errorf("semantic treatment strategy remained invalid after repair: %w", err)
	}
	return plan, nil
}

func semanticTreatmentRequiredProcessorIssue(plan semanticTreatmentPlan, requiredProcessorType string) error {
	requiredProcessorType = canonicalPluginRecommendationProcessorType(requiredProcessorType)
	if requiredProcessorType == "" {
		return nil
	}
	if plan.DecisionMode != "direct" || len(plan.Choices) != 1 {
		return fmt.Errorf("required processor type %s requires one direct materialization choice", requiredProcessorType)
	}
	if canonicalPluginRecommendationProcessorType(plan.Choices[0].ProcessorType) != requiredProcessorType {
		return fmt.Errorf("strategy processor type %s does not match required processor type %s", plan.Choices[0].ProcessorType, requiredProcessorType)
	}
	return nil
}

func decodeAndValidateSemanticTreatmentPlan(text, userText string, instances []semanticTreatmentInstance, forceInstanceChoice bool, nativeHandoffAvailable ...bool) (semanticTreatmentPlan, error) {
	var plan semanticTreatmentPlan
	if err := decodePluginRecommendationJSON(text, &plan); err != nil {
		return plan, err
	}
	if plan.SchemaVersion != semanticTreatmentSchema {
		return plan, fmt.Errorf("schema_version must be %s", semanticTreatmentSchema)
	}
	if strings.TrimSpace(plan.UserGoal) == "" {
		plan.UserGoal = strings.TrimSpace(userText)
	}
	plan.DecisionMode = strings.ToLower(strings.TrimSpace(plan.DecisionMode))
	if forceInstanceChoice {
		if plan.DecisionMode != "choice_required" {
			return plan, fmt.Errorf("loaded instance arbitration requires decision_mode=choice_required")
		}
		if len(plan.Choices) < 1 || len(plan.Choices) > 3 || len(plan.Choices) > len(instances) {
			return plan, fmt.Errorf("instance arbitration choices must contain 1-3 real instances")
		}
	} else {
		switch plan.DecisionMode {
		case "direct":
			if len(plan.Choices) != 1 {
				return plan, fmt.Errorf("direct strategy requires exactly one choice")
			}
		case "choice_required":
			if len(plan.Choices) != 3 {
				return plan, fmt.Errorf("choice_required strategy requires one recommendation and two alternatives")
			}
		default:
			return plan, fmt.Errorf("decision_mode must be direct or choice_required")
		}
	}
	seenChoice := map[string]bool{}
	seenInstance := map[string]bool{}
	seenSignature := map[string]bool{}
	nativeAllowed := len(nativeHandoffAvailable) > 0 && nativeHandoffAvailable[0]
	for index := range plan.Choices {
		choice := &plan.Choices[index]
		choice.ChoiceKey = strings.TrimSpace(choice.ChoiceKey)
		choice.Role = strings.ToLower(strings.TrimSpace(choice.Role))
		choice.ProcessorType = semanticTreatmentProcessorType(choice.ProcessorType)
		choice.TargetMode = strings.ToLower(strings.TrimSpace(choice.TargetMode))
		choice.InstanceKey = strings.TrimSpace(choice.InstanceKey)
		choice.NextPlanner = strings.ToLower(strings.TrimSpace(choice.NextPlanner))
		choice.Confidence = strings.ToLower(strings.TrimSpace(choice.Confidence))
		if choice.ChoiceKey == "" || seenChoice[choice.ChoiceKey] {
			return plan, fmt.Errorf("choice %d requires a unique choice_key", index+1)
		}
		seenChoice[choice.ChoiceKey] = true
		wantRole := "alternative"
		if index == 0 {
			wantRole = "recommended"
		}
		if choice.Role != wantRole {
			return plan, fmt.Errorf("choice %d role must be %s", index+1, wantRole)
		}
		if strings.TrimSpace(choice.Reason) == "" || strings.TrimSpace(choice.ExpectedEffect) == "" {
			return plan, fmt.Errorf("choice %d requires reason and expected_effect", index+1)
		}
		switch choice.Confidence {
		case "low", "medium", "high":
		default:
			return plan, fmt.Errorf("choice %d confidence is invalid", index+1)
		}
		signature := choice.TargetMode + ":" + choice.ProcessorType + ":" + choice.InstanceKey
		if seenSignature[signature] {
			return plan, fmt.Errorf("choice %d duplicates another treatment strategy", index+1)
		}
		seenSignature[signature] = true
		switch choice.TargetMode {
		case "existing_plugin":
			instance, ok := semanticTreatmentFindInstance(instances, choice.InstanceKey)
			if !ok || seenInstance[choice.InstanceKey] {
				return plan, fmt.Errorf("choice %d uses an unknown or duplicate instance_key", index+1)
			}
			seenInstance[choice.InstanceKey] = true
			if instance.ProcessorType != "unknown" && choice.ProcessorType != instance.ProcessorType {
				return plan, fmt.Errorf("choice %d processor_type does not match the bound instance", index+1)
			}
			expectedPlanner := "capability_boundary"
			if family := processorFamilyForTreatmentProcessorType(choice.ProcessorType); family != "" {
				if surface, surfaceOK := semanticTreatmentInstanceSurface(instance, family); surfaceOK && semanticTreatmentSurfacePCAEligible(surface) {
					expectedPlanner = surface.NextPlanner
				}
			}
			if expectedPlanner == "capability_boundary" && !instance.PCAReviewed {
				if instance.QualificationStatus == "generic_static_eq_qualified" && choice.ProcessorType == "eq" {
					expectedPlanner = "semantic_eq"
				} else if instance.QualificationStatus == "broadband_compressor_qualified" && choice.ProcessorType == "compressor" {
					expectedPlanner = "semantic_compressor"
				}
			}
			if choice.NextPlanner != expectedPlanner {
				return plan, fmt.Errorf("choice %d next_planner exceeds qualified instance capability", index+1)
			}
		case "load_required":
			if choice.InstanceKey != "" || choice.NextPlanner != "plugin_recommendation" || choice.ProcessorType == "" || choice.ProcessorType == "native" {
				return plan, fmt.Errorf("choice %d has an invalid load_required handoff", index+1)
			}
		case "native":
			if choice.InstanceKey != "" || choice.NextPlanner != "existing_agent_result" || !nativeAllowed || plan.DecisionMode != "direct" {
				return plan, fmt.Errorf("choice %d has an invalid native capability boundary", index+1)
			}
		default:
			return plan, fmt.Errorf("choice %d target_mode is invalid", index+1)
		}
		if forceInstanceChoice && (choice.TargetMode != "existing_plugin" || choice.ProcessorType != "eq" || choice.NextPlanner != "semantic_eq") {
			return plan, fmt.Errorf("instance arbitration may only select qualified existing EQ instances")
		}
	}
	return plan, nil
}

func semanticTreatmentProcessorType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "native") {
		return "native"
	}
	return canonicalPluginRecommendationProcessorType(value)
}

func semanticTreatmentObservationPayload(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	bounded := semanticEQPlannerObservation(observation)
	out := map[string]any{
		"tool_call_id": observation.ToolCallID, "tool": observation.Tool, "command_name": observation.CommandName,
		"status": observation.Status, "error": observation.Error,
	}
	if summary := firstMapFromAny(bounded["summary"]); len(summary) > 0 {
		out["summary"] = summary
	} else {
		out["summary"] = bounded
	}
	return out
}

func semanticTreatmentObservationFromPayload(payload map[string]any) *agentloop.RecentObservation {
	row := firstMapFromAny(payload["observation_context"])
	if len(row) == 0 {
		return nil
	}
	return &agentloop.RecentObservation{
		ToolCallID: firstStringFromMap(row, "tool_call_id"), Tool: firstStringFromMap(row, "tool"),
		CommandName: firstStringFromMap(row, "command_name"), Status: firstStringFromMap(row, "status"),
		Error: firstStringFromMap(row, "error"), Summary: cloneContext(firstMapFromAny(row["summary"])),
	}
}

func (s *Server) semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken string,
	requestContext map[string]any, res agentloop.Result, plan semanticTreatmentPlan, instances []semanticTreatmentInstance) ChatResponse {
	instanceByKey := map[string]semanticTreatmentInstance{}
	for _, instance := range instances {
		instanceByKey[instance.Key] = instance
	}
	rows := make([]map[string]any, 0, len(plan.Choices))
	reviews := make([]AgentInteractionReview, 0, len(plan.Choices))
	actions := make([]AgentInteractionAction, 0, len(plan.Choices)+1)
	lines := []string{firstNonEmpty(strings.TrimSpace(plan.Summary), "我已经根据目标、观察证据和当前可用实例形成处理策略。")}
	for index, choice := range plan.Choices {
		row := semanticTreatmentChoiceRow(choice, instanceByKey[choice.InstanceKey])
		rows = append(rows, row)
		roleLabel := "备选"
		if index == 0 {
			roleLabel = "主推荐"
		}
		lines = append(lines, fmt.Sprintf("%s：%s——%s", roleLabel, firstNonEmpty(choice.Title, pluginRecommendationProcessorLabel(choice.ProcessorType)), choice.Reason))
		body := strings.TrimSpace(choice.Reason) + "\n预期：" + strings.TrimSpace(choice.ExpectedEffect)
		if strings.TrimSpace(choice.Tradeoff) != "" {
			body += "\n取舍：" + strings.TrimSpace(choice.Tradeoff)
		}
		reviews = append(reviews, AgentInteractionReview{ID: choice.ChoiceKey, Title: roleLabel + " · " + firstNonEmpty(choice.Title, pluginRecommendationProcessorLabel(choice.ProcessorType)), Body: body, Status: choice.Role, Payload: row})
		actions = append(actions, AgentInteractionAction{
			ID: "select_" + choice.ChoiceKey, Label: "选择 " + firstNonEmpty(choice.Title, pluginRecommendationProcessorLabel(choice.ProcessorType)),
			Style: map[bool]string{true: "primary", false: "secondary"}[index == 0], Description: choice.Reason, Recommended: index == 0,
			Value: map[string]any{"choice_key": choice.ChoiceKey},
		})
	}
	actions = append(actions, AgentInteractionAction{ID: "cancel", Label: "暂不处理", Style: "secondary"})
	payload := map[string]any{
		"schema_version": semanticTreatmentSchema, "status": "awaiting_selection", "decision_mode": plan.DecisionMode,
		"listening_goal": plan.UserGoal, "summary": plan.Summary,
		"target_scope":      map[string]any{"kind": "track", "track_refs": []string{trackID}, "label": trackName},
		"relationship_refs": []string{}, "loaded_instances": instances, "choices": rows,
		"global_constraints": plan.GlobalConstraints, "evidence_refs": plan.EvidenceRefs, "limitations": plan.Limitations,
		"state_token": stateToken, "selection_performed": false, "mutation_performed": false,
		"request_context": cloneContext(requestContext), "observation_context": semanticTreatmentObservationPayload(res.RecentObservation),
	}
	res.ExecutionMemory.PendingMixTreatment = nil
	res.Status = agentruntime.StatusWaitingClarification
	res.StopReason = "semantic_treatment_selection_required"
	res.Error = ""
	res.Continuation = nil
	res.Reply = strings.Join(lines, "\n") + "\n选择只会进入对应的参数计划或插件推荐阶段；此时不会修改工程。"
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	resp.NeedsConfirmation = false
	resp.MessageKind = "proposal"
	resp.Workflow = semanticTreatmentWorkflow
	resp.WorkflowData = payload
	req := AgentInteractionRequest{
		ID: "interaction_" + randomID(), Kind: "semantic_treatment_question", Type: "semantic_treatment_selection",
		Source: "semantic_treatment", Workflow: semanticTreatmentWorkflow, Stage: "awaiting_selection",
		Title: "处理策略与插件实例选择", Body: res.Reply, Status: "waiting_for_user",
		ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID,
		ReviewItems: reviews, Payload: payload, Data: payload, Actions: actions,
	}
	s.storePendingInteraction(req, payload)
	resp.InteractionRequests = []AgentInteractionRequest{req}
	s.attachInteractionRequests(&resp)
	return resp
}

func semanticTreatmentChoiceRow(choice semanticTreatmentChoice, instance semanticTreatmentInstance) map[string]any {
	row := map[string]any{
		"choice_key": choice.ChoiceKey, "role": choice.Role, "title": choice.Title,
		"processor_type": choice.ProcessorType, "target_mode": choice.TargetMode, "instance_key": choice.InstanceKey,
		"reason": choice.Reason, "expected_effect": choice.ExpectedEffect, "tradeoff": choice.Tradeoff,
		"confidence": choice.Confidence, "next_planner": choice.NextPlanner, "material_difference": choice.MateriallyDiff,
	}
	if instance.PluginID != "" {
		row["bound_instance"] = instance
	}
	return row
}

func recoverSemanticTreatmentInteractionFromPayload(interactionID string, payload map[string]any) (PendingInteraction, bool) {
	if firstStringFromMap(payload, "schema_version") != semanticTreatmentSchema ||
		firstStringFromMap(payload, "status") != "awaiting_selection" || len(mapRowsValue(payload["choices"])) == 0 {
		return PendingInteraction{}, false
	}
	requestContext := cloneContext(firstMapFromAny(payload["request_context"]))
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	requestContext["semantic_treatment_recovered"] = true
	return PendingInteraction{
		ID: interactionID, Kind: "semantic_treatment_question", Type: "semantic_treatment_selection",
		Source: "semantic_treatment", Workflow: semanticTreatmentWorkflow, Stage: "awaiting_selection",
		ConversationID: firstStringFromMap(requestContext, "conversation_id"), GoalID: firstStringFromMap(requestContext, "goal_id"),
		RunID: firstStringFromMap(requestContext, "run_id"), RequestContext: requestContext, Payload: payload, Data: payload,
	}, true
}

func semanticTreatmentTarget(payload map[string]any) (string, string) {
	target := firstMapFromAny(payload["target_scope"])
	trackRefs := stringListValue(target["track_refs"])
	trackID := ""
	if len(trackRefs) > 0 {
		trackID = trackRefs[0]
	}
	return trackID, firstStringFromMap(target, "label")
}

func (s *Server) continueSemanticTreatmentInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	if strings.EqualFold(decision, "cancel") || strings.EqualFold(decision, "cancel_semantic_treatment") {
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply: "已取消这次处理策略选择；没有加载插件，也没有修改任何参数。", Workflow: semanticTreatmentWorkflow,
			WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "cancelled", "selection_performed": false, "mutation_performed": false}),
			GoalStatus:   string(agentruntime.StatusCancelled)}
	}
	choiceKey := strings.TrimPrefix(strings.TrimSpace(decision), "select_")
	var selected map[string]any
	for _, row := range mapRowsValue(interaction.Payload["choices"]) {
		if firstStringFromMap(row, "choice_key") == choiceKey {
			selected = row
			break
		}
	}
	if len(selected) == 0 {
		return semanticTreatmentInvalidSelectionResponse(interaction, "没有识别到有效的处理策略；工程没有被修改。", "invalid_treatment_choice")
	}
	trackID, trackName := semanticTreatmentTarget(interaction.Payload)
	if trackID == "" || s == nil || s.harness == nil {
		return semanticTreatmentInvalidSelectionResponse(interaction, "处理策略缺少当前精确目标轨道；工程没有被修改。", "treatment_target_unavailable")
	}
	// A stored interaction is not PCA authority. Re-read every selected surface
	// against the same model-owned family and coverage before materialization.
	var pcaInput any
	if intent := firstMapFromAny(interaction.RequestContext["free_state_semantic_processor_intent"]); len(intent) > 0 {
		pcaInput = intent
	}
	instances, currentToken := s.semanticTreatmentInstances(ctx, trackID, pcaInput)
	if expected := firstStringFromMap(interaction.Payload, "state_token"); expected == "" || currentToken == "" || expected != currentToken {
		return semanticTreatmentInvalidSelectionResponse(interaction, "选择期间工程或插件实例状态已经改变，旧策略已失效；请重新发起处理请求。", "stale_treatment_strategy")
	}
	requestContext := mergeContext(interaction.RequestContext, map[string]any{
		"selected_track_id": trackID, "selected_track_name": trackName, "conversation_id": interaction.ConversationID,
		"goal_id": interaction.GoalID, "run_id": interaction.RunID,
		"semantic_treatment_strategy": map[string]any{
			"schema_version": semanticTreatmentSchema, "summary": firstStringFromMap(interaction.Payload, "summary"),
			"global_constraints": stringListValue(interaction.Payload["global_constraints"]), "selected_choice": cloneContext(selected),
		},
	})
	goal := firstStringFromMap(interaction.Payload, "listening_goal")
	processorType := canonicalPluginRecommendationProcessorType(firstStringFromMap(selected, "processor_type"))
	switch firstStringFromMap(selected, "target_mode") {
	case "existing_plugin":
		bound := firstMapFromAny(selected["bound_instance"])
		pluginID := firstStringFromMap(bound, "plugin_id")
		instance, ok := semanticTreatmentFindPlugin(instances, pluginID)
		if !ok || instance.Key == "" || !semanticTreatmentInstanceExecutable(instance, processorType, firstStringFromMap(selected, "next_planner")) {
			return semanticTreatmentCapabilityBoundaryResponse(interaction, selected, "所选已加载实例没有通过对应普通 Agent 参数规划器的实时资格确认，因此没有生成参数动作。")
		}
		requestContext["selected_plugin_track_id"] = instance.TrackID
		requestContext["selected_plugin_id"] = instance.PluginID
		requestContext["selected_plugin_name"] = instance.PluginName
		observation := semanticTreatmentObservationFromPayload(interaction.Payload)
		requestContext["semantic_treatment_observation_context"] = semanticTreatmentObservationPayload(observation)
		if processorType == "eq" {
			requestContext["generic_eq_topology"] = instance.Topology
			return s.semanticTreatmentPlanEQResponse(ctx, interaction.ConversationID, goal, requestContext,
				observation, interaction.GoalID, interaction.RunID)
		}
		if processorType == "compressor" {
			return s.semanticTreatmentPlanCompressorResponse(ctx, interaction.ConversationID, goal, requestContext,
				interaction.GoalID, interaction.RunID)
		}
		family, planner, adapterOK := semanticPostLoadAdapterForProcessorType(processorType)
		if !adapterOK {
			return semanticTreatmentCapabilityBoundaryResponse(interaction, selected,
				"所选 family 没有可用的受治理 adapter；没有修改工程。")
		}
		cfg, _, cfgErr := config.Load()
		if cfgErr != nil || !cfg.Complete() {
			if cfgErr == nil {
				cfgErr = fmt.Errorf("AI configuration is incomplete")
			}
			return semanticTreatmentCapabilityBoundaryResponse(interaction, selected, cfgErr.Error())
		}
		return s.planBoundSemanticDynamic(ctx, interaction.ConversationID, goal, requestContext, family,
			semanticDynamicObservationFromRecent(observation), cfg)
		return semanticTreatmentCapabilityBoundaryResponse(interaction, selected,
			fmt.Sprintf("已确认 %s 的现有实例具备 PCA 资格，但 %s adapter（%s）尚未提供具体语义 planner；没有写入参数。", family, family, planner))
	case "load_required":
		requestContext["semantic_treatment_selection"] = true
		if processorType == "eq" {
			requestContext["semantic_eq_post_load_handoff"] = true
			requestContext["semantic_eq_post_load_goal"] = goal
		} else if processorType == "compressor" {
			requestContext["semantic_compressor_post_load_handoff"] = true
			requestContext["semantic_compressor_post_load_goal"] = goal
		}
		if family, planner, ok := semanticPostLoadAdapterForProcessorType(processorType); ok {
			requestContext["semantic_post_load_handoff"] = true
			requestContext["semantic_post_load_family"] = family
			requestContext["semantic_post_load_planner"] = planner
			requestContext["semantic_post_load_goal"] = goal
			requestContext["semantic_post_load_semantic_processor_intent"] = cloneContext(firstMapFromAny(interaction.RequestContext["free_state_semantic_processor_intent"]))
		}
		cfg, _, err := config.Load()
		if err != nil || !cfg.Complete() {
			if err == nil {
				err = fmt.Errorf("AI configuration is incomplete")
			}
			return semanticTreatmentInvalidSelectionResponse(interaction, "插件推荐阶段当前无法读取完整 AI 配置；没有加载插件或修改工程。", err.Error())
		}
		return s.ordinaryAgentPluginRecommendationResponseForProcessor(ctx, interaction.ConversationID,
			agentModeFromContext(requestContext), goal, requestContext, agentloop.Result{GoalID: interaction.GoalID, RunID: interaction.RunID,
				Status: agentruntime.StatusCompleted, RecentObservation: semanticTreatmentObservationFromPayload(interaction.Payload)}, processorType, cfg)
	default:
		return semanticTreatmentCapabilityBoundaryResponse(interaction, selected, "这个处理方法目前没有普通 Agent 的可治理参数 planner；策略已保留，但没有加载插件或修改工程。")
	}
}

func semanticTreatmentInvalidSelectionResponse(interaction PendingInteraction, reply, code string) ChatResponse {
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply: reply, Workflow: semanticTreatmentWorkflow,
		WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "stale_or_invalid", "mutation_performed": false}),
		GoalStatus:   string(agentruntime.StatusWaitingClarification), Error: code}
}

func semanticTreatmentCapabilityBoundaryResponse(interaction PendingInteraction, selected map[string]any, reason string) ChatResponse {
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply: reason, Workflow: semanticTreatmentWorkflow,
		WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "capability_boundary", "selected_choice": selected,
			"selection_performed": true, "mutation_performed": false}),
		GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_treatment_capability_boundary"}
}

func (s *Server) semanticTreatmentPlanEQResponse(ctx context.Context, conversationID, userText string, requestContext map[string]any,
	observation *agentloop.RecentObservation, goalID, runID string) ChatResponse {
	cfg, _, err := config.Load()
	if err != nil {
		return semanticEQRejectedResponse(conversationID, agentruntime.Goal{GoalID: goalID, RunID: runID, Summary: userText, Status: agentruntime.StatusFailed}, "config_unavailable", err.Error(), nil)
	}
	action, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, requestContext, observation, cfg, nil)
	if err != nil {
		return semanticEQRejectedResponse(conversationID, agentruntime.Goal{GoalID: goalID, RunID: runID, Summary: userText, Status: agentruntime.StatusFailed}, "acoustic_plan_failed", err.Error(), nil)
	}
	res := agentloop.Result{GoalID: goalID, RunID: runID, Status: agentruntime.StatusCompleted, RecentObservation: observation, SemanticAction: action}
	return s.materializeAgentSemanticEQAction(ctx, conversationID, ChatRequest{ConversationID: conversationID, Message: userText, Context: requestContext}, agentModeFromContext(requestContext), res)
}

func semanticTreatmentInstanceExecutable(instance semanticTreatmentInstance, processorType, nextPlanner string) bool {
	if canonical := canonicalPluginRecommendationProcessorType(processorType); canonical != "" {
		processorType = canonical
	}
	if family := processorFamilyForTreatmentProcessorType(processorType); family != "" && len(instance.QualifiedSurfaces) > 0 {
		for _, surface := range instance.QualifiedSurfaces {
			if surface.Family == family && semanticTreatmentSurfacePCAEligible(surface) && surface.NextPlanner == nextPlanner {
				return true
			}
		}
		return false
	}
	switch processorType {
	case "eq":
		return instance.ProcessorType == "eq" && instance.QualificationStatus == "generic_static_eq_qualified" && nextPlanner == "semantic_eq"
	case "compressor":
		return instance.ProcessorType == "compressor" && instance.QualificationStatus == "broadband_compressor_qualified" && nextPlanner == "semantic_compressor"
	default:
		return false
	}
}

func (s *Server) semanticTreatmentPlanCompressorResponse(ctx context.Context, conversationID, userText string,
	requestContext map[string]any, goalID, runID string) ChatResponse {
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		if err == nil {
			err = fmt.Errorf("AI configuration is incomplete")
		}
		return semanticCompressorPlanningFailure(conversationID,
			mergeContext(requestContext, map[string]any{"goal_id": goalID, "run_id": runID}), "config_unavailable", err)
	}
	bound := mergeContext(requestContext, map[string]any{"goal_id": goalID, "run_id": runID})
	return s.planBoundSemanticCompressor(ctx, conversationID, userText, bound, cfg)
}

func (s *Server) routeOrdinaryAgentSemanticEQ(ctx context.Context, conversationID, mode, userText string,
	requestContext map[string]any, res agentloop.Result, cfg config.EngineConfig) (ChatResponse, bool) {
	if contextBool(requestContext, "semantic_entry_unavailable") {
		return ChatResponse{}, false
	}
	if freeStateRouteAuthorized(requestContext) || !ordinaryAgentSemanticEQMutationRequest(userText, requestContext) {
		return ChatResponse{}, false
	}
	trackID := firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id")
	trackName := firstStringFromMap(requestContext, "selected_track_name")
	selectedPluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	if selectedPluginID != "" && len(firstMapFromAny(requestContext["generic_eq_topology"])) > 0 {
		planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, requestContext, res.RecentObservation, cfg, res.SemanticAction)
		if err != nil {
			goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
			return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
		}
		res.SemanticAction = planned
		return s.materializeAgentSemanticEQAction(ctx, conversationID,
			ChatRequest{ConversationID: conversationID, Message: userText, Context: requestContext}, mode, res), true
	}
	instances, stateToken := s.semanticTreatmentInstances(ctx, trackID)
	qualified := semanticTreatmentQualifiedEQ(instances)
	if selectedPluginID != "" {
		if instance, ok := semanticTreatmentFindPlugin(qualified, selectedPluginID); ok {
			bound := mergeContext(requestContext, map[string]any{
				"selected_plugin_track_id": instance.TrackID, "selected_plugin_id": instance.PluginID,
				"selected_plugin_name": instance.PluginName, "generic_eq_topology": instance.Topology,
			})
			planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, bound, res.RecentObservation, cfg, res.SemanticAction)
			if err != nil {
				goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
				return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
			}
			res.SemanticAction = planned
			return s.materializeAgentSemanticEQAction(ctx, conversationID,
				ChatRequest{ConversationID: conversationID, Message: userText, Context: bound}, mode, res), true
		}
		// A selected but ineligible processor must never be silently replaced.
		// If another real EQ exists, present that explicit instance decision.
		if len(qualified) > 0 {
			plan, err := s.planSemanticTreatment(ctx, conversationID, userText, trackID, trackName, res.RecentObservation, qualified, true, false, cfg)
			if err != nil {
				return semanticTreatmentPlannerErrorResponse(conversationID, res, err), true
			}
			plan.Limitations = append(plan.Limitations, "当前选中的插件没有通过通用静态 EQ 资格确认，因此不会在未征得选择的情况下改用其他实例。")
			return s.semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken, requestContext, res, plan, qualified), true
		}
	}
	if selectedPluginID == "" {
		switch len(qualified) {
		case 1:
			instance := qualified[0]
			bound := mergeContext(requestContext, map[string]any{
				"selected_plugin_track_id": instance.TrackID, "selected_plugin_id": instance.PluginID,
				"selected_plugin_name": instance.PluginName, "generic_eq_topology": instance.Topology,
			})
			planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, bound, res.RecentObservation, cfg, nil)
			if err != nil {
				goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
				return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
			}
			res.SemanticAction = planned
			return s.materializeAgentSemanticEQAction(ctx, conversationID,
				ChatRequest{ConversationID: conversationID, Message: userText, Context: bound}, mode, res), true
		default:
			if len(qualified) > 1 {
				plan, err := s.planSemanticTreatment(ctx, conversationID, userText, trackID, trackName, res.RecentObservation, qualified, true, false, cfg)
				if err != nil {
					return semanticTreatmentPlannerErrorResponse(conversationID, res, err), true
				}
				return s.semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken, requestContext, res, plan, qualified), true
			}
		}
	}
	// No loaded instance is qualified. Enter target 3 with an explicit
	// post-load EQ handoff marker; recommendation and loading remain separate.
	handoff := mergeContext(requestContext, map[string]any{
		"semantic_eq_post_load_handoff": true, "semantic_eq_post_load_goal": strings.TrimSpace(userText),
	})
	return s.ordinaryAgentPluginRecommendationResponseForProcessor(ctx, conversationID, mode, userText, handoff, res, "eq", cfg), true
}

func (s *Server) routeOrdinaryAgentTreatmentStrategy(ctx context.Context, conversationID, mode, userText string,
	requestContext map[string]any, res agentloop.Result, cfg config.EngineConfig) (ChatResponse, bool) {
	if contextBool(requestContext, "semantic_entry_unavailable") {
		return ChatResponse{}, false
	}
	if !freeStateRouteAuthorized(requestContext) {
		return ChatResponse{}, false
	}
	trackID := firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id")
	trackName := firstStringFromMap(requestContext, "selected_track_name")
	nativeHandoff := semanticTreatmentNativeHandoffAvailable(res)
	requiredProcessorType := ""
	if freeStateRouteAuthorized(requestContext) {
		requiredProcessorType = firstStringFromMap(requestContext, "free_state_processor_type")
	}
	semanticIntent := firstMapFromAny(requestContext["free_state_semantic_processor_intent"])
	if freeStateRouteAuthorized(requestContext) && res.FreeStateDecision != nil &&
		strings.EqualFold(strings.TrimSpace(res.FreeStateDecision.Status), agentloop.FreeStateNeedsAction) &&
		strings.ToLower(strings.TrimSpace(requiredProcessorType)) != "native" &&
		processorFamilyForTreatmentProcessorType(requiredProcessorType) != "" && len(semanticIntent) == 0 {
		return semanticTreatmentDirectBoundaryResponse(conversationID, res, semanticTreatmentPlan{}, semanticTreatmentChoice{},
			"free-state needs_action requires the model-owned semantic_processor_intent; no family, coverage, or parameter action was materialized"), true
	}
	if len(semanticIntent) > 0 {
		rawIntent, _ := json.Marshal(semanticIntent)
		decodedIntent, intentErr := processorintent.Decode(string(rawIntent))
		if intentErr != nil || decodedIntent.Status != processorintent.StatusResolved ||
			(processorFamilyForTreatmentProcessorType(requiredProcessorType) != "" && decodedIntent.Family != processorFamilyForTreatmentProcessorType(requiredProcessorType)) {
			return semanticTreatmentDirectBoundaryResponse(conversationID, res, semanticTreatmentPlan{}, semanticTreatmentChoice{},
				"free-state semantic_processor_intent failed deterministic validation; no processor family or parameter action was materialized"), true
		}
		_, registryErr := processorregistry.Default()
		if registryErr != nil {
			return semanticTreatmentPlannerErrorResponse(conversationID, res, registryErr), true
		}
		if orchestratorErr := semanticProgressiveDisclosureAccept(requestContext, decodedIntent, res, res.RecentObservation); orchestratorErr != nil {
			return semanticTreatmentDirectBoundaryResponse(conversationID, res, semanticTreatmentPlan{}, semanticTreatmentChoice{},
				"free-state semantic intent cannot enter the progressive-disclosure orchestrator: "+orchestratorErr.Error()), true
		}
	}
	var instancePCAInput any
	if len(semanticIntent) > 0 {
		instancePCAInput = semanticIntent
	}
	instances, stateToken := s.semanticTreatmentInstances(ctx, trackID, instancePCAInput)
	planArgs := []any{}
	if requiredProcessorType != "" {
		planArgs = append(planArgs, requiredProcessorType)
	}
	if len(semanticIntent) > 0 {
		planArgs = append(planArgs, semanticIntent)
	}
	plan, err := s.planSemanticTreatment(ctx, conversationID, userText, trackID, trackName, res.RecentObservation, instances, false, nativeHandoff, cfg, planArgs...)
	if err != nil {
		return semanticTreatmentPlannerErrorResponse(conversationID, res, err), true
	}
	if plan.DecisionMode == "choice_required" {
		return s.semanticTreatmentSelectionResponse(conversationID, mode, trackID, trackName, stateToken, requestContext, res, plan, instances), true
	}
	if len(plan.Choices) != 1 {
		return semanticTreatmentPlannerErrorResponse(conversationID, res, fmt.Errorf("direct strategy omitted its single choice")), true
	}
	choice := plan.Choices[0]
	requestContext = mergeContext(requestContext, map[string]any{
		"semantic_treatment_strategy": map[string]any{"schema_version": semanticTreatmentSchema, "summary": plan.Summary,
			"global_constraints": plan.GlobalConstraints, "selected_choice": semanticTreatmentChoiceRow(choice, semanticTreatmentInstance{})},
	})
	switch choice.TargetMode {
	case "existing_plugin":
		instance, ok := semanticTreatmentFindInstance(instances, choice.InstanceKey)
		if !ok || !semanticTreatmentInstanceExecutable(instance, choice.ProcessorType, choice.NextPlanner) {
			return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
				"主推荐实例目前没有通过对应普通 Agent 参数规划器的实时资格确认；没有修改工程。"), true
		}
		bound := mergeContext(requestContext, map[string]any{
			"selected_plugin_track_id": instance.TrackID, "selected_plugin_id": instance.PluginID,
			"selected_plugin_name": instance.PluginName, "goal_id": res.GoalID, "run_id": res.RunID,
			"semantic_treatment_observation_context": semanticTreatmentObservationPayload(res.RecentObservation),
		})
		if choice.ProcessorType == "compressor" {
			return s.planBoundSemanticCompressor(ctx, conversationID, userText, bound, cfg), true
		}
		if choice.ProcessorType != "eq" {
			family, planner, ok := semanticPostLoadAdapterForProcessorType(choice.ProcessorType)
			if !ok {
				return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
					"所选 family 没有可用的受治理 adapter；没有修改工程。"), true
			}
			return s.planBoundSemanticDynamic(ctx, conversationID, userText, bound, family,
				semanticDynamicObservationFromRecent(res.RecentObservation), cfg), true
			return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
				fmt.Sprintf("已确认 %s 的 PCA 资格，但 %s adapter（%s）尚未提供具体语义 planner；没有写入参数。", family, planner, choice.NextPlanner)), true
		}
		bound["generic_eq_topology"] = instance.Topology
		planned, err := s.ensureOrdinaryAgentSemanticEQExecutable(ctx, conversationID, userText, bound, res.RecentObservation, cfg, nil)
		if err != nil {
			goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Summary: userText, Status: agentruntime.StatusFailed}
			return semanticEQRejectedResponse(conversationID, goal, "acoustic_plan_failed", err.Error(), nil), true
		}
		res.SemanticAction = planned
		return s.materializeAgentSemanticEQAction(ctx, conversationID,
			ChatRequest{ConversationID: conversationID, Message: userText, Context: bound}, mode, res), true
	case "load_required":
		if choice.ProcessorType == "eq" {
			requestContext["semantic_eq_post_load_handoff"] = true
			requestContext["semantic_eq_post_load_goal"] = strings.TrimSpace(userText)
		} else if choice.ProcessorType == "compressor" {
			requestContext["semantic_compressor_post_load_handoff"] = true
			requestContext["semantic_compressor_post_load_goal"] = strings.TrimSpace(userText)
		}
		if family, planner, ok := semanticPostLoadAdapterForProcessorType(choice.ProcessorType); ok {
			requestContext["semantic_post_load_handoff"] = true
			requestContext["semantic_post_load_family"] = family
			requestContext["semantic_post_load_planner"] = planner
			requestContext["semantic_post_load_goal"] = strings.TrimSpace(userText)
			requestContext["semantic_post_load_semantic_processor_intent"] = cloneContext(semanticIntent)
		}
		return s.ordinaryAgentPluginRecommendationResponseForProcessor(ctx, conversationID, mode, userText, requestContext, res, choice.ProcessorType, cfg), true
	case "native":
		if nativeHandoff && choice.NextPlanner == "existing_agent_result" {
			return s.chatResponseFromAgentLoopResult(conversationID, mode, res), true
		}
		return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
			"原生控制策略已经失去可继续的 Agent 结果；没有修改工程，请重新发起请求。"), true
	default:
		return semanticTreatmentDirectBoundaryResponse(conversationID, res, plan, choice,
			"主推荐方法目前没有可治理的普通 Agent 参数 planner；策略可以讨论，但没有加载插件或修改工程。"), true
	}
}

func semanticTreatmentNativeHandoffAvailable(res agentloop.Result) bool {
	if res.ExecutionMemory.PendingMixTreatment != nil || res.ExecutionMemory.PendingMixTickCandidate != nil {
		return true
	}
	return res.Status == agentruntime.StatusWaitingConfirmation && res.Continuation != nil
}

func semanticTreatmentPlannerErrorResponse(conversationID string, res agentloop.Result, err error) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID,
		Reply:    "处理策略暂时无法形成可靠且可验证的结论，因此没有选择插件或修改工程。",
		Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
			"status": "unavailable", "mutation_performed": false}, GoalStatus: string(agentruntime.StatusFailed), Error: err.Error()}
}

func semanticTreatmentDirectBoundaryResponse(conversationID string, res agentloop.Result, plan semanticTreatmentPlan,
	choice semanticTreatmentChoice, reason string) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID, Reply: reason,
		Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
			"status": "capability_boundary", "summary": plan.Summary, "selected_choice": semanticTreatmentChoiceRow(choice, semanticTreatmentInstance{}),
			"selection_performed": true, "mutation_performed": false}, GoalStatus: string(agentruntime.StatusCompleted),
		StopReason: "semantic_treatment_capability_boundary"}
}

// After a user confirms loading an EQ chosen by target 3, qualify the actual
// returned instance and create a separate governed EQ proposal. The load
// authorization never grants parameter-write authority.
func (s *Server) semanticEQPostLoadHandoff(ctx context.Context, plan PendingPlan, replies []map[string]any) (ChatResponse, bool) {
	if !boolValue(plan.WorkflowData["semantic_eq_post_load_handoff"]) {
		return ChatResponse{}, false
	}
	trackID, pluginID, pluginName := pluginLoadResultIDs(replies)
	if trackID == "" {
		trackID = firstStringFromMap(plan.WorkflowData, "track_id")
	}
	goalID, runID := goalIDsFromContext(plan.Context)
	conversationID := firstStringFromMap(plan.Context, "conversation_id")
	userGoal := firstNonEmpty(firstStringFromMap(plan.WorkflowData, "semantic_eq_post_load_goal"), firstStringFromMap(plan.WorkflowData, "intent"))
	if pluginID == "" {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    "EQ 已加载，但加载结果没有返回精确 plugin_id，无法进行资格确认或生成参数方案；没有写入任何 EQ 参数。",
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "eq", "track_id": trackID, "mutation_performed": false},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_eq_post_load_identity_missing"}, true
	}
	digest, summary, err := s.readLiveEQControlSurface(ctx, trackID, pluginID)
	if err != nil || len(summary) == 0 {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply: fmt.Sprintf("已加载 %s，但该实际实例没有通过普通 Agent 通用静态 EQ 资格确认，因此没有生成或写入参数方案。限制：%s",
				firstNonEmpty(pluginName, pluginID), firstNonEmpty(errorText(err), "没有可证明的通用静态 EQ topology")),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "eq", "track_id": trackID, "plugin_id": pluginID,
				"plugin_name": pluginName, "mutation_performed": false, "limitation": errorText(err)},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_eq_post_load_not_qualified"}, true
	}
	if _, pcaErr := semanticPostLoadPCAQualification(plan, digest, processorattestation.FamilyStaticEQ, trackID, pluginID, pluginName); pcaErr != nil {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    fmt.Sprintf("已加载 %s，但当前 exact identity 未通过 PCA 复核，因此没有生成或写入 EQ 参数方案。限制：%s", firstNonEmpty(pluginName, pluginID), pcaErr),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "eq", "track_id": trackID, "plugin_id": pluginID,
				"mutation_performed": false, "pca_rejection": pcaErr.Error()},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_eq_post_load_pca_rejected"}, true
	}
	pcaReceipt, hasPCAReceipt, receiptErr := semanticPCAAdmissionReceiptFromPlan(plan)
	if receiptErr != nil || !hasPCAReceipt {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    fmt.Sprintf("已加载 %s，但 PCA admission receipt 无法交接到 EQ 执行边界，因此没有生成或写入 EQ 参数方案。", firstNonEmpty(pluginName, pluginID)),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "eq", "track_id": trackID, "plugin_id": pluginID,
				"mutation_performed": false, "pca_rejection": firstNonEmpty(errorText(receiptErr), "pca admission receipt is missing")},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_eq_post_load_receipt_missing"}, true
	}
	requestContext := cloneContext(firstMapFromAny(plan.WorkflowData["semantic_eq_post_load_request_context"]))
	if requestContext == nil {
		requestContext = cloneContext(plan.Context)
	}
	requestContext = mergeContext(requestContext, map[string]any{
		"selected_track_id": trackID, "selected_plugin_track_id": trackID, "selected_plugin_id": pluginID,
		"selected_plugin_name":  firstNonEmpty(pluginName, firstStringFromMap(plan.WorkflowData, "plugin_name")),
		"generic_eq_topology":   semanticEQTopologyPromptSummary(trackID, pluginID, summary),
		"pca_admission_receipt": semanticPCAAdmissionReceiptMap(pcaReceipt),
		"conversation_id":       conversationID, "goal_id": goalID, "run_id": runID,
	})
	observation := semanticTreatmentObservationFromPayload(map[string]any{
		"observation_context": firstMapFromAny(plan.WorkflowData["semantic_eq_post_load_observation_context"]),
	})
	resp := s.semanticTreatmentPlanEQResponse(ctx, conversationID, userGoal, requestContext, observation, goalID, runID)
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	resp.WorkflowData["post_load_qualification"] = map[string]any{
		"status": "qualified", "processor_type": "eq", "track_id": trackID, "plugin_id": pluginID,
		"plugin_name":         firstNonEmpty(pluginName, firstStringFromMap(plan.WorkflowData, "plugin_name")),
		"topology_generation": eqTopologyGenerationFromSummary(summary),
	}
	resp.Reply = fmt.Sprintf("已加载并确认 %s 具备可执行的通用静态 EQ topology。加载授权已经结束，尚未写入 EQ 参数。\n\n%s",
		firstNonEmpty(pluginName, pluginID), resp.Reply)
	return resp, true
}

// semanticCompressorPostLoadHandoff treats the completed rack load as identity
// evidence only. It re-qualifies the returned instance and creates a separate
// compressor proposal; the preceding load confirmation grants no parameter
// mutation authority.
func (s *Server) semanticCompressorPostLoadHandoff(ctx context.Context, plan PendingPlan, replies []map[string]any) (ChatResponse, bool) {
	if !boolValue(plan.WorkflowData["semantic_compressor_post_load_handoff"]) {
		return ChatResponse{}, false
	}
	trackID, pluginID, pluginName := pluginLoadResultIDs(replies)
	if trackID == "" {
		trackID = firstStringFromMap(plan.WorkflowData, "track_id")
	}
	goalID, runID := goalIDsFromContext(plan.Context)
	conversationID := firstStringFromMap(plan.Context, "conversation_id")
	userGoal := firstNonEmpty(firstStringFromMap(plan.WorkflowData, "semantic_compressor_post_load_goal"), firstStringFromMap(plan.WorkflowData, "intent"))
	if pluginID == "" {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    "压缩器已加载，但加载结果没有返回精确 plugin_id，无法进行资格确认或生成参数方案；没有写入任何压缩器参数。",
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "compressor", "track_id": trackID, "mutation_performed": false},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_compressor_post_load_identity_missing"}, true
	}
	digest, _, err := s.readLiveCompressorControlSurface(ctx, trackID, pluginID)
	if digest.PluginName == "" {
		digest.PluginName = pluginName
	}
	card, boundary := plugingrabber.BuildAudioProcessorIdentityCard(digest)
	if err != nil || card == nil {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply: fmt.Sprintf("已加载 %s，但该真实实例没有通过普通 Agent 的单段宽带压缩器资格确认，因此没有生成或写入参数方案。限制：%s",
				firstNonEmpty(pluginName, pluginID), firstNonEmpty(errorText(err), boundary, "没有可证明的单段宽带压缩器 topology")),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "compressor", "track_id": trackID, "plugin_id": pluginID,
				"plugin_name": pluginName, "mutation_performed": false, "limitation": firstNonEmpty(errorText(err), boundary)},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_compressor_post_load_not_qualified"}, true
	}
	if _, pcaErr := semanticPostLoadPCAQualification(plan, digest, processorattestation.FamilyBroadbandCompressor, trackID, pluginID, pluginName); pcaErr != nil {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    fmt.Sprintf("已加载 %s，但当前 exact identity 未通过 PCA 复核，因此没有生成或写入压缩器参数方案。限制：%s", firstNonEmpty(pluginName, pluginID), pcaErr),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": "compressor", "track_id": trackID, "plugin_id": pluginID,
				"mutation_performed": false, "pca_rejection": pcaErr.Error()},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_compressor_post_load_pca_rejected"}, true
	}
	requestContext := cloneContext(firstMapFromAny(plan.WorkflowData["semantic_compressor_post_load_request_context"]))
	if requestContext == nil {
		requestContext = cloneContext(plan.Context)
	}
	requestContext = mergeContext(requestContext, map[string]any{
		"selected_track_id": trackID, "selected_plugin_track_id": trackID, "selected_plugin_id": pluginID,
		"selected_plugin_name":    firstNonEmpty(pluginName, firstStringFromMap(plan.WorkflowData, "plugin_name")),
		"processor_identity_card": card, "conversation_id": conversationID, "goal_id": goalID, "run_id": runID,
		"semantic_treatment_observation_context": cloneContext(firstMapFromAny(plan.WorkflowData["semantic_compressor_post_load_observation_context"])),
	})
	resp := s.semanticTreatmentPlanCompressorResponse(ctx, conversationID, userGoal, requestContext, goalID, runID)
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	resp.WorkflowData["post_load_qualification"] = map[string]any{
		"status": "qualified", "processor_type": "compressor", "track_id": trackID, "plugin_id": pluginID,
		"plugin_name":         firstNonEmpty(pluginName, firstStringFromMap(plan.WorkflowData, "plugin_name")),
		"topology_generation": card.TopologyEvidence.Generation, "topology_classification": card.TopologyEvidence.Classification,
	}
	resp.Reply = fmt.Sprintf("已加载并确认 %s 具备可执行的单段宽带压缩器 topology。加载授权已经结束，尚未写入压缩器参数。\n\n%s",
		firstNonEmpty(pluginName, pluginID), resp.Reply)
	return resp, true
}

// semanticGenericPostLoadHandoff is the common post-load admission boundary
// for registered dynamic families. It performs the complete exact-identity,
// fingerprint, PCA coverage, and live-topology recheck; it never falls back
// to a generic parameter writer.
func (s *Server) semanticGenericPostLoadHandoff(ctx context.Context, plan PendingPlan, replies []map[string]any) (ChatResponse, bool) {
	if !boolValue(plan.WorkflowData["semantic_post_load_handoff"]) {
		return ChatResponse{}, false
	}
	trackID, pluginID, pluginName := pluginLoadResultIDs(replies)
	if trackID == "" {
		trackID = firstStringFromMap(plan.WorkflowData, "track_id")
	}
	goalID, runID := goalIDsFromContext(plan.Context)
	conversationID := firstStringFromMap(plan.Context, "conversation_id")
	family := firstStringFromMap(plan.WorkflowData, "semantic_post_load_family")
	if family == "" {
		family = firstStringFromMap(plan.Context, "semantic_post_load_family")
	}
	if pluginID == "" || trackID == "" || family == "" {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    "插件已加载，但 post-load PCA 复核缺少精确实例或 family 身份；没有生成或写入参数。",
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": legacyProcessorTypeForFamily(family), "mutation_performed": false},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_post_load_identity_missing"}, true
	}
	client := s.eqKernelClient()
	if client == nil {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    "插件已加载，但无法读取 live topology；没有生成或写入参数。",
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": legacyProcessorTypeForFamily(family), "mutation_performed": false},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_post_load_topology_unavailable"}, true
	}
	reply, _, err := client.SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil || !kernelReplyOK(reply) {
		reason := firstNonEmpty(errorText(err), firstNonEmptyText(reply, "message", "error"), "live parameter read failed")
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    fmt.Sprintf("插件已加载，但 live topology 读取失败：%s；没有生成或写入参数。", reason),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": legacyProcessorTypeForFamily(family), "mutation_performed": false, "limitation": reason},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_post_load_topology_unavailable"}, true
	}
	s.observePluginParametersReply(reply)
	digest := plugingrabber.BuildParameterDigest(reply)
	if digest.PluginName == "" {
		digest.PluginName = pluginName
	}
	surface, pcaErr := semanticPostLoadPCAQualification(plan, digest, family, trackID, pluginID, pluginName)
	if pcaErr != nil {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    fmt.Sprintf("已加载 %s，但没有通过 %s 的 post-load PCA/topology 复核：%s；没有写入参数。", firstNonEmpty(pluginName, pluginID), family, pcaErr),
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "processor_type": legacyProcessorTypeForFamily(family), "track_id": trackID, "plugin_id": pluginID,
				"mutation_performed": false, "pca_rejection": pcaErr.Error()},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_post_load_pca_rejected"}, true
	}
	requestContext := mergeContext(plan.Context, map[string]any{
		"selected_plugin_track_id":               trackID,
		"selected_plugin_id":                     pluginID,
		"selected_plugin_name":                   firstNonEmpty(pluginName, digest.PluginName),
		"semantic_treatment_observation_context": firstMapFromAny(plan.WorkflowData["semantic_post_load_observation_context"]),
	})
	cfg, _, cfgErr := config.Load()
	if cfgErr == nil && cfg.Complete() {
		goal := firstNonEmpty(firstStringFromMap(plan.WorkflowData, "semantic_post_load_goal"), firstStringFromMap(requestContext, "user_goal"), "apply the selected semantic processor intent")
		observation := firstMapFromAny(plan.WorkflowData["semantic_post_load_observation_context"])
		return s.planBoundSemanticDynamic(ctx, conversationID, goal, requestContext, family, observation, cfg), true
	}
	return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Reply:    fmt.Sprintf("已加载 %s，并通过 %s 的 exact identity、指纹、coverage 与 live topology 复核；该 family 的语义 planner 尚未接入，因此没有写入参数。", firstNonEmpty(pluginName, pluginID), family),
		Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
			"status": "qualified_planner_pending", "processor_type": legacyProcessorTypeForFamily(family), "family": family,
			"track_id": trackID, "plugin_id": pluginID, "pca_status": surface.PCAStatus, "pca_reason": surface.PCAReason,
			"mutation_performed": false, "post_load_qualification": map[string]any{"status": "qualified", "family": family}},
		GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_post_load_planner_pending"}, true
}

func (s *Server) semanticProcessorPostLoadHandoff(ctx context.Context, plan PendingPlan, replies []map[string]any) (ChatResponse, bool) {
	if _, _, err := semanticValidatePCAAdmissionReceipt(plan, "", pluginLoadResultIdentifier(replies)); err != nil {
		conversationID := firstStringFromMap(plan.Context, "conversation_id")
		goalID, runID := goalIDsFromContext(plan.Context)
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply:    "加载结果与确认时的 PCA 精确身份不一致；没有生成或写入参数。",
			Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"schema_version": semanticTreatmentSchema,
				"status": "qualification_failed", "mutation_performed": false, "pca_rejection": err.Error()},
			GoalStatus: string(agentruntime.StatusCompleted), StopReason: "semantic_post_load_identity_mismatch"}, true
	}
	if handoff, ok := s.semanticEQPostLoadHandoff(ctx, plan, replies); ok {
		return s.finalizeSemanticProcessorPostLoadHandoff(plan, handoff), true
	}
	handoff, ok := s.semanticCompressorPostLoadHandoff(ctx, plan, replies)
	if !ok {
		handoff, ok = s.semanticGenericPostLoadHandoff(ctx, plan, replies)
		if !ok {
			return ChatResponse{}, false
		}
		return s.finalizeSemanticProcessorPostLoadHandoff(plan, handoff), true
	}
	return s.finalizeSemanticProcessorPostLoadHandoff(plan, handoff), true
}

func (s *Server) finalizeSemanticProcessorPostLoadHandoff(plan PendingPlan, handoff ChatResponse) ChatResponse {
	requestContext := plan.Context
	if state, ok := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); ok {
		if handoff.WorkflowData == nil {
			handoff.WorkflowData = map[string]any{}
		}
		handoff.WorkflowData["semantic_progressive_disclosure"] = state
		handoff.WorkflowData["request_context"] = cloneContext(requestContext)
	}
	if freeStateLoopActiveContext(requestContext) {
		requestContext = mergeContext(requestContext, map[string]any{"free_state_route_authorized": true})
		goalID, runID := goalIDsFromContext(requestContext)
		handoff = s.makeFreeStateMaterializationResumable(handoff.ConversationID, requestContext,
			agentloop.Result{GoalID: goalID, RunID: runID}, handoff)
	}
	return s.bindFreeStateContextToResponse(handoff, requestContext)
}

func applySemanticPostLoadHandoffResponse(base map[string]any, handoff ChatResponse) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	base["message"] = handoff.Reply
	base["reply"] = handoff.Reply
	base["needs_confirmation"] = handoff.NeedsConfirmation
	base["plan_id"] = handoff.PlanID
	base["preview"] = handoff.Preview
	base["workflow"] = handoff.Workflow
	base["workflow_data"] = handoff.WorkflowData
	base["goal_status"] = handoff.GoalStatus
	base["stop_reason"] = handoff.StopReason
	base["error"] = handoff.Error
	if handoff.ProposalPresentation != nil {
		base["proposal_presentation"] = handoff.ProposalPresentation
	}
	if len(handoff.InteractionRequests) > 0 {
		base["interaction_requests"] = handoff.InteractionRequests
	}
	if len(handoff.TypedEvents) > 0 {
		base["typed_events"] = append(mapRowsFromAny(base["typed_events"]), handoff.TypedEvents...)
	}
	base["message_kind"] = firstNonEmpty(handoff.MessageKind, chatResponseMessageKind(handoff))
	return base
}
