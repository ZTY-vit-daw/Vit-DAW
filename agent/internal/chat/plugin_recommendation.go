package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
)

const (
	pluginRecommendationWorkflow = "plugin_recommendation_selection"
	pluginRecommendationSchema   = "plugin_recommendation_selection.v1"
)

type pluginRecommendationCandidate struct {
	Key               string `json:"candidate_key"`
	ID                string `json:"id,omitempty"`
	Name              string `json:"name"`
	DescriptiveName   string `json:"descriptive_name,omitempty"`
	Manufacturer      string `json:"manufacturer,omitempty"`
	Format            string `json:"format,omitempty"`
	Category          string `json:"category,omitempty"`
	Identifier        string `json:"identifier,omitempty"`
	PluginPath        string `json:"plugin_path"`
	PrimaryType       string `json:"primary_type,omitempty"`
	IsInstrument      bool   `json:"is_instrument,omitempty"`
	SubjectKey        string `json:"-"`
	BinaryFingerprint string `json:"-"`
	ProcessorFamily   string `json:"-"`
	AttestationID     string `json:"-"`
}

type pluginRecommendationChoice struct {
	CandidateKey   string   `json:"candidate_key"`
	Identifier     string   `json:"identifier,omitempty"`
	Role           string   `json:"role"`
	Reason         string   `json:"reason"`
	Tradeoff       string   `json:"tradeoff,omitempty"`
	Confidence     string   `json:"confidence"`
	KnowledgeBasis []string `json:"knowledge_basis,omitempty"`
}

type pluginRecommendationPlan struct {
	SchemaVersion string                       `json:"schema_version"`
	ProcessorType string                       `json:"processor_type"`
	UserGoal      string                       `json:"user_goal"`
	Summary       string                       `json:"summary"`
	Choices       []pluginRecommendationChoice `json:"choices"`
	Limitations   []string                     `json:"limitations,omitempty"`
}

func ordinaryAgentPluginRecommendationIntent(userText string, requestContext map[string]any) bool {
	if ordinaryAgentSemanticEQNeedsPluginSelection(userText, requestContext) {
		return true
	}
	if looksLikePluginGrabberLoadIntent(userText) {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(userText))
	hasRecommendation := agentLoopTextHasAny(text,
		"推荐", "建议用", "适合用", "适合加载", "该用什么", "用什么效果器", "用哪个", "选哪个", "哪款",
		"recommend", "suggest", "which plugin", "what plugin", "best plugin", "suitable plugin", "appropriate plugin",
	)
	hasPluginSubject := agentLoopTextHasAny(text,
		"插件", "效果器", "均衡", "压缩", "混响", "延迟", "限制器", "激励器", "饱和", "失真",
		"plugin", "effect", "eq", "equalizer", "compressor", "reverb", "delay", "limiter", "de-esser", "deesser", "gate", "expander", "transient", "multiband", "exciter", "saturation", "distortion",
	)
	return hasRecommendation && hasPluginSubject
}

func explicitPluginRecommendationProcessorType(userText string) string {
	text := strings.ToLower(strings.TrimSpace(userText))
	switch {
	case strings.Contains(text, "spectral dynamics") || strings.Contains(text, "spectral-dynamics"):
		return "spectral_dynamics"
	case strings.Contains(text, "clipper"):
		return "clipper"
	case agentLoopTextHasAny(text, "均衡", "eq", "equalizer"):
		return "eq"
	case agentLoopTextHasAny(text, "压缩", "compressor", "compression"):
		return "compressor"
	case agentLoopTextHasAny(text, "混响", "reverb"):
		return "reverb"
	case agentLoopTextHasAny(text, "延迟", "delay", "echo"):
		return "delay"
	case agentLoopTextHasAny(text, "限制器", "limiter", "maximizer"):
		return "limiter"
	case agentLoopTextHasAny(text, "de-esser", "deesser", "sibilance", "齿音", "齒音"):
		return "de_esser"
	case agentLoopTextHasAny(text, "gate", "expander", "noise gate", "噪声门", "噪音门", "扩展器", "擴展器"):
		return "gate_expander"
	case agentLoopTextHasAny(text, "transient shaper", "transient", "瞬态", "瞬變"):
		return "transient_shaper"
	case agentLoopTextHasAny(text, "multiband", "multi-band", "多段", "多频段", "多頻段"):
		return "multiband_dynamics"
	case agentLoopTextHasAny(text, "激励器", "饱和", "失真", "exciter", "saturation", "distortion", "overdrive"):
		return "distortion"
	case agentLoopTextHasAny(text, "调制", "合唱", "chorus", "flanger", "phaser", "modulation"):
		return "modulation"
	case agentLoopTextHasAny(text, "滤波器", "filter"):
		return "filter"
	case agentLoopTextHasAny(text, "音高", "修音", "pitch", "tune"):
		return "pitch"
	case agentLoopTextHasAny(text, "分析仪", "频谱仪", "analyzer", "spectrum", "meter"):
		return "analyzer"
	default:
		return ""
	}
}

func (s *Server) inferPluginRecommendationProcessorType(ctx context.Context, conversationID, userText string, requestContext map[string]any, observation *agentloop.RecentObservation, cfg config.EngineConfig) (string, error) {
	if s == nil || s.llm == nil {
		return "", fmt.Errorf("plugin recommendation LLM is unavailable")
	}
	input := map[string]any{
		"user_request":         userText,
		"conversation_context": s.pluginRecommendationConversationContext(conversationID),
		"target_context": map[string]any{
			"track_id":   firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id"),
			"track_name": firstStringFromMap(requestContext, "selected_track_name"),
		},
		"observation_context": semanticEQPlannerObservation(observation),
	}
	inputJSON, _ := json.Marshal(input)
	system := `You choose one processor family for a horizontal ordinary-DAW-Agent plugin recommendation request.
This is not an A-F specialist capability. Infer the user's primary requested processing method from their words and supplied context.
Return ONLY JSON: {"schema_version":"plugin_processor_choice.v1","processor_type":"eq|compressor|limiter|gate_expander|de_esser|transient_shaper|multiband_dynamics|reverb|delay|distortion|modulation|filter|pitch|analyzer","reason":"brief reason"}.
Choose exactly one family. Do not name or rank a plugin, do not propose a chain, do not invent observations, and do not return prose.`
	request := llm.Request{
		Messages:   []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(inputJSON)}},
		Metadata:   llm.RequestMetadata{Source: "ordinary_agent_plugin_processor_choice", ConversationID: conversationID},
		PreferJSON: true,
	}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return "", err
	}
	var decoded struct {
		SchemaVersion string `json:"schema_version"`
		ProcessorType string `json:"processor_type"`
	}
	if err := decodePluginRecommendationJSON(response.Text, &decoded); err != nil {
		return "", fmt.Errorf("decode plugin processor choice: %w", err)
	}
	typ := canonicalPluginRecommendationProcessorType(decoded.ProcessorType)
	if decoded.SchemaVersion != "plugin_processor_choice.v1" || typ == "" {
		return "", fmt.Errorf("plugin processor choice returned an unsupported processor_type")
	}
	return typ, nil
}

func canonicalPluginRecommendationProcessorType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "eq", "compressor", "limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics", "spectral_dynamics", "clipper", "reverb", "delay", "distortion", "modulation", "filter", "pitch", "analyzer":
		return value
	case "equalizer":
		return "eq"
	case "compression", "dynamics":
		return "compressor"
	case "gate", "expander":
		return "gate_expander"
	case "deesser", "de-esser":
		return "de_esser"
	case "transient":
		return "transient_shaper"
	case "multiband":
		return "multiband_dynamics"
	case "saturation", "exciter", "overdrive":
		return "distortion"
	default:
		return ""
	}
}

func (s *Server) localPluginRecommendationCandidates(ctx context.Context, processorType string) ([]pluginRecommendationCandidate, error) {
	if s == nil || s.harness == nil {
		return nil, fmt.Errorf("plugin inventory is unavailable")
	}
	searchType := pluginRecommendationCatalogSearchType(processorType)
	resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:   "plugin.semantic_search",
		Args:   map[string]any{"query": searchType, "type": searchType, "limit": 200},
		Source: "ordinary_agent_plugin_recommendation",
	})
	if err != nil {
		return nil, err
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("plugin semantic search failed: %s", firstNonEmpty(resp.Error, "unknown error"))
	}
	rows := mapRowsValue(resp.Result["entries"])
	if len(rows) == 0 {
		rows = mapRowsValue(resp.Result["plugins"])
	}
	return pluginRecommendationCandidatesFromRows(rows), nil
}

func pluginRecommendationCatalogSearchType(processorType string) string {
	switch canonicalPluginRecommendationProcessorType(processorType) {
	case "compressor", "limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics":
		// Catalog categories often expose compressor, limiter, gate and expander
		// products under the hard host category "Dynamics". The LLM performs the
		// task-specific selection within that complete local family.
		return "dynamics"
	default:
		return canonicalPluginRecommendationProcessorType(processorType)
	}
}

func pluginRecommendationCandidatesFromRows(rows []map[string]any) []pluginRecommendationCandidate {
	out := make([]pluginRecommendationCandidate, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		path := firstStringFromMap(row, "plugin_path", "path", "file_or_identifier", "file_path")
		name := firstStringFromMap(row, "name", "descriptive_name", "display_name")
		if path == "" || name == "" {
			continue
		}
		dedupeKey := strings.ToLower(strings.TrimSpace(firstNonEmpty(firstStringFromMap(row, "id", "identifier"), path)))
		if dedupeKey != "" && seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		out = append(out, pluginRecommendationCandidate{
			Key:             fmt.Sprintf("plugin_candidate_%d", len(out)+1),
			ID:              firstStringFromMap(row, "id"),
			Name:            name,
			DescriptiveName: firstStringFromMap(row, "descriptive_name"),
			Manufacturer:    firstStringFromMap(row, "manufacturer", "vendor"),
			Format:          firstStringFromMap(row, "format"),
			Category:        firstStringFromMap(row, "category"),
			Identifier:      firstStringFromMap(row, "identifier"),
			PluginPath:      path,
			PrimaryType:     firstStringFromMap(row, "primary_type"),
			IsInstrument:    boolValue(row["is_instrument"]),
		})
	}
	return out
}

// genericStaticEQRecommendationCandidates is the pre-load admission boundary
// for B4/C1. Their governed executor can materialize only ordinary static EQ
// atoms, so catalog entries explicitly classified or named as dynamic EQ must
// not become an exact load Proposal. Post-load topology qualification remains
// authoritative for every admitted candidate.
func genericStaticEQRecommendationCandidates(candidates []pluginRecommendationCandidate) []pluginRecommendationCandidate {
	out := make([]pluginRecommendationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		category := strings.ToLower(candidate.Category)
		name := strings.ToLower(firstNonEmpty(candidate.DescriptiveName, candidate.Name))
		dynamicCategory := false
		for _, token := range strings.FieldsFunc(category, func(r rune) bool { return r == '|' || r == ',' || r == ';' || r == '/' }) {
			if strings.TrimSpace(token) == "dynamics" {
				dynamicCategory = true
				break
			}
		}
		dynamicName := strings.Contains(name, "dynamic eq") || strings.Contains(name, "dynamic-eq") || strings.Contains(name, "dyneq") || strings.Contains(name, "dyn eq")
		if dynamicCategory || dynamicName {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func (s *Server) planPluginRecommendation(ctx context.Context, conversationID, userText, processorType string, requestContext map[string]any, observation *agentloop.RecentObservation, candidates []pluginRecommendationCandidate, cfg config.EngineConfig) (pluginRecommendationPlan, error) {
	if s == nil || s.llm == nil {
		return pluginRecommendationPlan{}, fmt.Errorf("plugin recommendation LLM is unavailable")
	}
	if len(candidates) == 0 {
		return pluginRecommendationPlan{}, fmt.Errorf("no loadable local %s candidates", processorType)
	}
	input := map[string]any{
		"user_request":            userText,
		"conversation_context":    s.pluginRecommendationConversationContext(conversationID),
		"required_processor_type": processorType,
		"target_context": map[string]any{
			"track_id":   firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id"),
			"track_name": firstStringFromMap(requestContext, "selected_track_name"),
		},
		"observation_context":       semanticEQPlannerObservation(observation),
		"loadable_local_candidates": pluginRecommendationCandidatesForLLM(candidates),
	}
	inputJSON, _ := json.Marshal(input)
	system := `You are the plugin recommendation phase of an ordinary DAW Agent.
The local catalog supplies hard facts and exact loadable candidates. You own the musical and product judgement: use the user's goal, evidence, and your pretrained knowledge of named products to decide which available plugin is the best fit.

Return ONLY one JSON object:
{"schema_version":"plugin_recommendation.v1","processor_type":"copy required type","user_goal":"copy user goal","summary":"short recommendation summary in the user's language","choices":[{"candidate_key":"exact supplied key","identifier":"exact supplied identifier","role":"recommended|alternative","reason":"task-specific reason in the user's language","tradeoff":"meaningful difference or limitation","confidence":"low|medium|high","knowledge_basis":["catalog_metadata","model_knowledge","user_request","observation"]}],"limitations":[]}

Rules:
- Select only exact candidate_key values from loadable_local_candidates; never invent a plugin, path, manufacturer, capability, or availability.
- Return one primary recommendation. Return up to two alternatives only when they offer a materially different workflow, character, or tradeoff. At most three choices total.
- The first choice must be role=recommended; all others role=alternative. Candidate keys must be unique.
- LLM pretrained knowledge may inform soft suitability, workflow, era/model/style, and likely character. Clearly keep that reasoning probabilistic; do not present it as a measured property or a local catalog fact.
- Catalog match scores, type confidence, alphabetical order, and the first candidate are not recommendation reasons.
- Do not use offline measurement, stored plugin mappings, B4, or parameter/topology claims.
- This phase recommends and selects only. It must not load a plugin or propose parameter writes.
- If product-specific knowledge is weak, prefer a versatile, well-known candidate and disclose the limitation rather than fabricating detail.`
	request := llm.Request{
		Messages:   []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(inputJSON)}},
		Metadata:   llm.RequestMetadata{Source: "ordinary_agent_plugin_recommendation", ConversationID: conversationID},
		PreferJSON: true,
	}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return pluginRecommendationPlan{}, err
	}
	plan, decodeErr := decodeAndValidatePluginRecommendationPlan(response.Text, processorType, userText, candidates)
	if decodeErr == nil {
		return plan, nil
	}
	repair := fmt.Sprintf("Your previous plugin recommendation JSON was invalid: %s\nReturn ONLY a corrected plugin_recommendation.v1 object using the same exact candidate keys.", decodeErr)
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: repair})
	response, err = s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return pluginRecommendationPlan{}, err
	}
	plan, err = decodeAndValidatePluginRecommendationPlan(response.Text, processorType, userText, candidates)
	if err != nil {
		return pluginRecommendationPlan{}, fmt.Errorf("plugin recommendation remained invalid after repair: %w", err)
	}
	return plan, nil
}

// pluginRecommendationCandidatesForLLM deliberately exposes only the opaque
// candidate key, exact PCA-admitted identifier, and a display-only name.
// Executable paths, vendor metadata, and PCA internals stay server-side and
// are resolved after the model has selected one of these exact candidates.
func pluginRecommendationCandidatesForLLM(candidates []pluginRecommendationCandidate) []map[string]any {
	out := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, map[string]any{
			"candidate_key": candidate.Key,
			"identifier":    candidate.Identifier,
			"name":          candidate.Name,
		})
	}
	return out
}

func decodePluginRecommendationJSON(text string, target any) error {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return fmt.Errorf("response does not contain a JSON object")
	}
	return json.Unmarshal([]byte(text[start:end+1]), target)
}

func decodeAndValidatePluginRecommendationPlan(text, processorType, userText string, candidates []pluginRecommendationCandidate) (pluginRecommendationPlan, error) {
	var plan pluginRecommendationPlan
	if err := decodePluginRecommendationJSON(text, &plan); err != nil {
		return plan, err
	}
	if plan.SchemaVersion != "plugin_recommendation.v1" {
		return plan, fmt.Errorf("schema_version must be plugin_recommendation.v1")
	}
	if canonicalPluginRecommendationProcessorType(plan.ProcessorType) != processorType {
		return plan, fmt.Errorf("processor_type changed from %s", processorType)
	}
	if strings.TrimSpace(plan.UserGoal) == "" {
		plan.UserGoal = strings.TrimSpace(userText)
	}
	if len(plan.Choices) < 1 || len(plan.Choices) > 3 {
		return plan, fmt.Errorf("choices must contain 1-3 candidates")
	}
	known := map[string]bool{}
	for _, candidate := range candidates {
		known[candidate.Key] = true
	}
	seen := map[string]bool{}
	for i := range plan.Choices {
		choice := &plan.Choices[i]
		choice.CandidateKey = strings.TrimSpace(choice.CandidateKey)
		choice.Identifier = strings.TrimSpace(choice.Identifier)
		choice.Role = strings.ToLower(strings.TrimSpace(choice.Role))
		choice.Confidence = strings.ToLower(strings.TrimSpace(choice.Confidence))
		if !known[choice.CandidateKey] {
			return plan, fmt.Errorf("choice %d invented candidate_key %q", i+1, choice.CandidateKey)
		}
		for _, candidate := range candidates {
			if candidate.Key != choice.CandidateKey {
				continue
			}
			if choice.Identifier == "" {
				choice.Identifier = candidate.Identifier
			}
			if !strings.EqualFold(choice.Identifier, candidate.Identifier) {
				return plan, fmt.Errorf("choice %d identifier does not match supplied candidate", i+1)
			}
		}
		if seen[choice.CandidateKey] {
			return plan, fmt.Errorf("candidate_key %q was selected more than once", choice.CandidateKey)
		}
		seen[choice.CandidateKey] = true
		wantRole := "alternative"
		if i == 0 {
			wantRole = "recommended"
		}
		if choice.Role != wantRole {
			return plan, fmt.Errorf("choice %d role must be %s", i+1, wantRole)
		}
		if strings.TrimSpace(choice.Reason) == "" {
			return plan, fmt.Errorf("choice %d reason is required", i+1)
		}
		switch choice.Confidence {
		case "low", "medium", "high":
		default:
			return plan, fmt.Errorf("choice %d confidence must be low, medium, or high", i+1)
		}
	}
	return plan, nil
}

func (s *Server) ordinaryAgentPluginRecommendationResponse(ctx context.Context, conversationID, mode, userText string, requestContext map[string]any, res agentloop.Result, cfg config.EngineConfig) ChatResponse {
	processorType := explicitPluginRecommendationProcessorType(userText)
	if ordinaryAgentSemanticEQNeedsPluginSelection(userText, requestContext) {
		processorType = "eq"
		requestContext = mergeContext(requestContext, map[string]any{
			"semantic_eq_post_load_handoff": true,
			"semantic_eq_post_load_goal":    strings.TrimSpace(userText),
		})
	}
	if processorType == "" {
		inferred, err := s.inferPluginRecommendationProcessorType(ctx, conversationID, userText, requestContext, res.RecentObservation, cfg)
		if err != nil {
			return pluginRecommendationErrorResponse(conversationID, res, "processor_choice_failed", err)
		}
		processorType = inferred
	}
	return s.ordinaryAgentPluginRecommendationResponseForProcessor(ctx, conversationID, mode, userText, requestContext, res, processorType, cfg)
}

func (s *Server) ordinaryAgentPluginRecommendationResponseForProcessor(ctx context.Context, conversationID, mode, userText string, requestContext map[string]any, res agentloop.Result, processorType string, cfg config.EngineConfig) ChatResponse {
	requestContext = s.bindFreeStateAuthoritativeTrack(conversationID, requestContext)
	processorType = canonicalPluginRecommendationProcessorType(processorType)
	if processorType == "" {
		return pluginRecommendationErrorResponse(conversationID, res, "processor_choice_invalid", fmt.Errorf("processor type is unavailable"))
	}
	if processorType == "spectral_dynamics" || processorType == "clipper" {
		return s.pluginRecommendationNoCandidatesResponse(conversationID, mode, userText, requestContext, res, processorType)
	}
	if processorAttestationFamily(processorType) == "" {
		return s.pluginRecommendationNoCandidatesResponse(conversationID, mode, userText, requestContext, res, processorType)
	}
	candidates, err := s.localPluginRecommendationCandidates(ctx, processorType)
	if err != nil {
		return pluginRecommendationErrorResponse(conversationID, res, "catalog_search_failed", err)
	}
	if len(candidates) == 0 {
		return s.pluginRecommendationNoCandidatesResponse(conversationID, mode, userText, requestContext, res, processorType)
	}
	if processorAttestationFamily(processorType) != "" {
		candidates, err = admittedPluginRecommendationCandidates(candidates, processorType)
		if err != nil {
			return pluginRecommendationErrorResponse(conversationID, res, "attestation_query_failed", err)
		}
		if len(candidates) == 0 {
			return s.pluginRecommendationNoCandidatesResponse(conversationID, mode, userText, requestContext, res, processorType)
		}
	}
	plan, err := s.planPluginRecommendation(ctx, conversationID, userText, processorType, requestContext, res.RecentObservation, candidates, cfg)
	if err != nil {
		return pluginRecommendationErrorResponse(conversationID, res, "recommendation_failed", err)
	}
	return s.pluginRecommendationSelectionResponse(conversationID, mode, requestContext, res, plan, candidates)
}

func pluginRecommendationErrorResponse(conversationID string, res agentloop.Result, code string, err error) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID,
		GoalID:         res.GoalID,
		RunID:          res.RunID,
		Reply:          "插件推荐暂时无法形成可靠结论，因此没有选择或加载任何插件。",
		Workflow:       pluginRecommendationWorkflow,
		WorkflowData:   map[string]any{"schema_version": pluginRecommendationSchema, "status": "unavailable", "error_code": code, "mutation_performed": false},
		GoalStatus:     string(agentruntime.StatusFailed),
		Error:          err.Error(),
	}
}

func (s *Server) pluginRecommendationNoCandidatesResponse(conversationID, mode, userText string, requestContext map[string]any, res agentloop.Result, processorType string) ChatResponse {
	requestContext = s.bindFreeStateAuthoritativeTrack(conversationID, requestContext)
	res.ExecutionMemory.PendingMixTreatment = nil
	res.Status = agentruntime.StatusCompleted
	res.StopReason = ""
	res.Error = ""
	res.Continuation = nil
	res.Reply = fmt.Sprintf("当前本机没有找到已通过 PCA 准入、且二进制指纹仍匹配的 %s 候选；这不是插件目录为空，也没有选择、加载或修改工程。", pluginRecommendationProcessorLabel(processorType))
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	resp.Workflow = pluginRecommendationWorkflow
	resp.WorkflowData = map[string]any{
		"schema_version":      pluginRecommendationSchema,
		"status":              "no_candidates",
		"admission":           "pca_promoted_current_binary",
		"processor_type":      processorType,
		"listening_goal":      strings.TrimSpace(userText),
		"target_ref":          map[string]any{"kind": "track", "id": firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id"), "label": firstStringFromMap(requestContext, "selected_track_name")},
		"selection_performed": false,
		"mutation_performed":  false,
	}
	return resp
}

func (s *Server) pluginRecommendationSelectionResponse(conversationID, mode string, requestContext map[string]any, res agentloop.Result, plan pluginRecommendationPlan, candidates []pluginRecommendationCandidate) ChatResponse {
	requestContext = s.bindFreeStateAuthoritativeTrack(conversationID, requestContext)
	candidateByKey := map[string]pluginRecommendationCandidate{}
	for _, candidate := range candidates {
		candidateByKey[candidate.Key] = candidate
	}
	rows := make([]map[string]any, 0, len(plan.Choices))
	reviews := make([]AgentInteractionReview, 0, len(plan.Choices))
	actions := make([]AgentInteractionAction, 0, len(plan.Choices)+1)
	lines := []string{firstNonEmpty(strings.TrimSpace(plan.Summary), "我已经根据当前任务和本机插件目录完成推荐。")}
	for i, choice := range plan.Choices {
		candidate := candidateByKey[choice.CandidateKey]
		row := pluginRecommendationChoiceRow(choice, candidate)
		rows = append(rows, row)
		label := candidate.Name
		roleLabel := "备选"
		if i == 0 {
			roleLabel = "主推荐"
		}
		lines = append(lines, fmt.Sprintf("%s：%s——%s", roleLabel, label, strings.TrimSpace(choice.Reason)))
		body := strings.TrimSpace(choice.Reason)
		if strings.TrimSpace(choice.Tradeoff) != "" {
			body += "\n取舍：" + strings.TrimSpace(choice.Tradeoff)
		}
		reviews = append(reviews, AgentInteractionReview{ID: choice.CandidateKey, Title: roleLabel + " · " + label, Body: body, Status: choice.Role, Payload: row})
		actions = append(actions, AgentInteractionAction{
			ID: "select_" + choice.CandidateKey, Label: "选择 " + label,
			Style:       firstNonEmpty(map[bool]string{true: "primary", false: "secondary"}[i == 0]),
			Description: strings.TrimSpace(choice.Reason), Recommended: i == 0,
			Value: map[string]any{"candidate_key": choice.CandidateKey},
		})
	}
	actions = append(actions, AgentInteractionAction{ID: "cancel", Label: "暂不加载", Style: "secondary"})
	trackID := firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id")
	trackName := firstStringFromMap(requestContext, "selected_track_name")
	payload := map[string]any{
		"schema_version":  pluginRecommendationSchema,
		"status":          "awaiting_selection",
		"processor_type":  plan.ProcessorType,
		"listening_goal":  plan.UserGoal,
		"summary":         plan.Summary,
		"target_ref":      map[string]any{"kind": "track", "id": trackID, "label": trackName},
		"recommendations": rows,
		"limitations":     appendPluginRecommendationCatalogLimitation(plan.Limitations),
		"catalog_evidence": map[string]any{
			"source": "local_scanned_plugin_index", "exact_identity_bound": true,
			"license_and_runtime_load_unverified": true,
		},
		"selection_performed": false,
		"load_proposed":       false,
		"mutation_performed":  false,
		"request_context":     cloneContext(requestContext),
	}
	if requirement, ok := pluginControlRequirementFromAny(requestContext["processor_control_requirement"]); ok {
		payload["processor_control_requirement"] = requirement
	}
	if boolValue(requestContext["semantic_eq_post_load_handoff"]) && plan.ProcessorType == "eq" {
		payload["post_load_planner"] = "semantic_eq"
		payload["post_load_goal"] = firstNonEmpty(firstStringFromMap(requestContext, "semantic_eq_post_load_goal"), plan.UserGoal)
		payload["post_load_observation_context"] = semanticTreatmentObservationPayload(res.RecentObservation)
	} else if boolValue(requestContext["semantic_compressor_post_load_handoff"]) && plan.ProcessorType == "compressor" {
		payload["post_load_planner"] = "semantic_compressor"
		payload["post_load_goal"] = firstNonEmpty(firstStringFromMap(requestContext, "semantic_compressor_post_load_goal"), plan.UserGoal)
		payload["post_load_observation_context"] = semanticTreatmentObservationPayload(res.RecentObservation)
	}
	if family, planner, ok := semanticPostLoadAdapterForProcessorType(plan.ProcessorType); ok {
		payload["post_load_family"] = family
		payload["post_load_planner"] = planner
		payload["post_load_goal"] = firstNonEmpty(firstStringFromMap(requestContext, "semantic_post_load_goal", "semantic_eq_post_load_goal", "semantic_compressor_post_load_goal"), plan.UserGoal)
		payload["post_load_observation_context"] = semanticTreatmentObservationPayload(res.RecentObservation)
		if semanticIntent := firstMapFromAny(requestContext["free_state_semantic_processor_intent"]); len(semanticIntent) > 0 {
			payload["post_load_semantic_processor_intent"] = cloneContext(semanticIntent)
		}
	}
	if res.RecentObservation != nil {
		texts := semanticEQRecursiveText(res.RecentObservation.Summary)
		if observationID := semanticEQFirstRecursive(texts, "observation_id"); observationID != "" {
			payload["observation_id"] = observationID
			payload["evidence_refs"] = []string{"mix.observe:" + observationID}
		}
	}
	res.ExecutionMemory.PendingMixTreatment = nil
	res.Status = agentruntime.StatusWaitingClarification
	res.StopReason = "plugin_selection_required"
	res.Error = ""
	res.Continuation = nil
	res.Reply = strings.Join(lines, "\n") + "\n选择一个候选后，我只会生成加载确认，不会立即加载或修改参数。"
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	resp.NeedsConfirmation = false
	resp.MessageKind = "proposal"
	resp.Workflow = pluginRecommendationWorkflow
	resp.WorkflowData = payload
	req := AgentInteractionRequest{
		ID: "interaction_" + randomID(), Kind: "plugin_recommendation_question", Type: "plugin_recommendation_selection",
		Source: "plugin_recommendation", Workflow: pluginRecommendationWorkflow, Stage: "awaiting_selection",
		Title: pluginRecommendationProcessorLabel(plan.ProcessorType) + "插件推荐", Body: res.Reply, Status: "waiting_for_user",
		ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID,
		ReviewItems: reviews, Payload: payload, Data: payload, Actions: actions,
	}
	s.storePendingInteraction(req, payload)
	resp.InteractionRequests = []AgentInteractionRequest{req}
	s.attachInteractionRequests(&resp)
	return resp
}

func appendPluginRecommendationCatalogLimitation(limitations []string) []string {
	out := append([]string(nil), limitations...)
	const limitation = "本机目录证明候选已扫描且具有精确加载标识；授权状态和实际实例化仍需在确认加载时验证。"
	for _, existing := range out {
		if strings.TrimSpace(existing) == limitation {
			return out
		}
	}
	return append(out, limitation)
}

func (s *Server) pluginRecommendationConversationContext(conversationID string) []map[string]any {
	messages := s.recentConversationMessages(conversationID, 8, nil)
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if runes := []rune(content); len(runes) > 2000 {
			content = string(runes[:2000]) + "…"
		}
		out = append(out, map[string]any{"role": message.Role, "content": content})
	}
	return out
}

func pluginRecommendationChoiceRow(choice pluginRecommendationChoice, candidate pluginRecommendationCandidate) map[string]any {
	return map[string]any{
		"candidate_key":      candidate.Key,
		"id":                 candidate.ID,
		"name":               candidate.Name,
		"descriptive_name":   candidate.DescriptiveName,
		"manufacturer":       candidate.Manufacturer,
		"format":             candidate.Format,
		"category":           candidate.Category,
		"identifier":         candidate.Identifier,
		"plugin_path":        candidate.PluginPath,
		"primary_type":       candidate.PrimaryType,
		"is_instrument":      candidate.IsInstrument,
		"subject_key":        candidate.SubjectKey,
		"binary_fingerprint": candidate.BinaryFingerprint,
		"processor_family":   candidate.ProcessorFamily,
		"attestation_id":     candidate.AttestationID,
		"role":               choice.Role,
		"reason":             strings.TrimSpace(choice.Reason),
		"tradeoff":           strings.TrimSpace(choice.Tradeoff),
		"confidence":         choice.Confidence,
		"knowledge_basis":    append([]string(nil), choice.KnowledgeBasis...),
	}
}

func pluginRecommendationProcessorLabel(processorType string) string {
	switch canonicalPluginRecommendationProcessorType(processorType) {
	case "eq":
		return "EQ"
	case "compressor":
		return "压缩器"
	case "reverb":
		return "混响"
	case "delay":
		return "延迟"
	case "limiter":
		return "限制器"
	case "gate_expander":
		return "Gate/Expander"
	case "de_esser":
		return "De-esser"
	case "transient_shaper":
		return "Transient Shaper"
	case "multiband_dynamics":
		return "Multiband Dynamics"
	case "distortion":
		return "饱和/失真"
	case "modulation":
		return "调制"
	case "filter":
		return "滤波器"
	case "pitch":
		return "音高处理"
	case "analyzer":
		return "分析器"
	default:
		return "效果器"
	}
}

func (s *Server) continuePluginRecommendationInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	if strings.EqualFold(decision, "cancel") || strings.EqualFold(decision, "cancel_plugin_recommendation") {
		return ChatResponse{
			ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply:        "已取消插件选择；没有加载插件，也没有修改工程。",
			Workflow:     pluginRecommendationWorkflow,
			WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "cancelled", "selection_performed": false, "mutation_performed": false}),
			GoalStatus:   string(agentruntime.StatusCancelled),
		}
	}
	candidateKey := strings.TrimPrefix(strings.TrimSpace(decision), "select_")
	var selected map[string]any
	for _, row := range mapRowsValue(interaction.Payload["recommendations"]) {
		if firstStringFromMap(row, "candidate_key") == candidateKey {
			selected = row
			break
		}
	}
	if len(selected) == 0 {
		return ChatResponse{
			ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply:        "没有识别到有效的插件候选；没有加载插件或修改工程。",
			Workflow:     pluginRecommendationWorkflow,
			WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "invalid_selection", "mutation_performed": false}),
			GoalStatus:   string(agentruntime.StatusWaitingClarification), Error: "invalid_plugin_candidate_selection",
		}
	}
	if boolValue(interaction.RequestContext["plugin_recommendation_recovered"]) {
		verified, err := s.verifyRecoveredPluginRecommendationCandidate(ctx, firstStringFromMap(interaction.Payload, "processor_type"), selected)
		if err != nil {
			return ChatResponse{
				ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
				Reply:        "插件候选无法与当前本机目录重新核对，因此没有生成加载动作。请重新发起推荐。",
				Workflow:     pluginRecommendationWorkflow,
				WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "stale_selection", "mutation_performed": false}),
				GoalStatus:   string(agentruntime.StatusWaitingClarification), Error: err.Error(),
			}
		}
		selected = verified
	}
	processorFamily := firstStringFromMap(selected, "processor_family")
	hasPCAAdmission := processorFamily != "" && firstStringFromMap(selected, "subject_key") != "" &&
		firstStringFromMap(selected, "binary_fingerprint") != "" && firstStringFromMap(selected, "attestation_id") != ""
	if hasPCAAdmission {
		currentCandidate, err := currentPCAAdmittedPluginRecommendationCandidate(selected, processorFamily)
		if err != nil {
			return ChatResponse{
				ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
				Reply:        "The selected processor control attestation is no longer current; no load action was created.",
				Workflow:     pluginRecommendationWorkflow,
				WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "stale_selection", "mutation_performed": false}),
				GoalStatus:   string(agentruntime.StatusWaitingClarification), Error: err.Error(),
			}
		}
		choice := pluginRecommendationChoice{CandidateKey: firstStringFromMap(selected, "candidate_key"), Role: firstStringFromMap(selected, "role"), Reason: firstStringFromMap(selected, "reason"), Tradeoff: firstStringFromMap(selected, "tradeoff"), Confidence: firstStringFromMap(selected, "confidence")}
		choice.Identifier = firstStringFromMap(selected, "identifier")
		currentCandidate.Key = choice.CandidateKey
		selected = pluginRecommendationChoiceRow(choice, currentCandidate)
	} else if boolValue(interaction.RequestContext["plugin_recommendation_recovered"]) && processorAttestationFamily(firstStringFromMap(interaction.Payload, "processor_type")) != "" {
		return ChatResponse{
			ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply:        "The recovered recommendation predates processor attestation and must be requested again; no load action was created.",
			Workflow:     pluginRecommendationWorkflow,
			WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "stale_selection", "mutation_performed": false}),
			GoalStatus:   string(agentruntime.StatusWaitingClarification), Error: "processor_control_requirement_missing",
		}
	}
	target := firstMapFromAny(interaction.Payload["target_ref"])
	trackID := firstStringFromMap(target, "id")
	if trackID == "" {
		return ChatResponse{
			ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply:        "插件已经选定，但当前没有精确目标轨道，因此没有生成加载动作。请先选择目标轨道。",
			Workflow:     pluginRecommendationWorkflow,
			WorkflowData: mergeContext(interaction.Payload, map[string]any{"status": "target_required", "selected_candidate": selected, "selection_performed": true, "mutation_performed": false}),
			GoalStatus:   string(agentruntime.StatusWaitingClarification),
		}
	}
	requestContext := mergeContext(interaction.RequestContext, map[string]any{
		"selected_track_id":                        trackID,
		"semantic_plugin_recommendation_selection": true,
		"semantic_plugin_recommendation_candidate": cloneContext(selected),
		"plugin_recommendation":                    cloneContext(interaction.Payload),
		"conversation_id":                          interaction.ConversationID,
		"goal_id":                                  interaction.GoalID,
		"run_id":                                   interaction.RunID,
	})
	if firstStringFromMap(interaction.Payload, "post_load_planner") == "semantic_eq" &&
		canonicalPluginRecommendationProcessorType(firstStringFromMap(interaction.Payload, "processor_type")) == "eq" {
		requestContext["semantic_eq_post_load_handoff"] = true
		requestContext["semantic_eq_post_load_goal"] = firstNonEmpty(firstStringFromMap(interaction.Payload, "post_load_goal"), firstStringFromMap(interaction.Payload, "listening_goal"))
		requestContext["semantic_eq_post_load_observation_context"] = cloneContext(firstMapFromAny(interaction.Payload["post_load_observation_context"]))
	} else if firstStringFromMap(interaction.Payload, "post_load_planner") == "semantic_compressor" &&
		canonicalPluginRecommendationProcessorType(firstStringFromMap(interaction.Payload, "processor_type")) == "compressor" {
		requestContext["semantic_compressor_post_load_handoff"] = true
		requestContext["semantic_compressor_post_load_goal"] = firstNonEmpty(firstStringFromMap(interaction.Payload, "post_load_goal"), firstStringFromMap(interaction.Payload, "listening_goal"))
		requestContext["semantic_compressor_post_load_observation_context"] = cloneContext(firstMapFromAny(interaction.Payload["post_load_observation_context"]))
	}
	if family := firstStringFromMap(interaction.Payload, "post_load_family"); family != "" {
		requestContext["semantic_post_load_handoff"] = true
		requestContext["semantic_post_load_family"] = family
		requestContext["semantic_post_load_planner"] = firstStringFromMap(interaction.Payload, "post_load_planner")
		requestContext["semantic_post_load_goal"] = firstStringFromMap(interaction.Payload, "post_load_goal")
		requestContext["semantic_post_load_observation_context"] = cloneContext(firstMapFromAny(interaction.Payload["post_load_observation_context"]))
		requestContext["semantic_post_load_semantic_processor_intent"] = cloneContext(firstMapFromAny(interaction.Payload["post_load_semantic_processor_intent"]))
	}
	workflowCmd := map[string]any{
		"cmd":               "plugin_grabber_load",
		"track_id":          trackID,
		"plugin_query":      firstStringFromMap(selected, "name"),
		"plugin_name":       firstStringFromMap(selected, "name"),
		"plugin_path":       firstStringFromMap(selected, "plugin_path"),
		"plugin_identifier": firstStringFromMap(selected, "identifier"),
		"type":              firstStringFromMap(interaction.Payload, "processor_type"),
		"intent":            firstStringFromMap(interaction.Payload, "listening_goal"),
	}
	resp := s.runPluginGrabberLoadWorkflow(ctx, interaction.ConversationID, firstStringFromMap(interaction.Payload, "listening_goal"), requestContext, workflowCmd)
	resp.GoalID = interaction.GoalID
	resp.RunID = interaction.RunID
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	resp.WorkflowData["recommendation_schema_version"] = pluginRecommendationSchema
	resp.WorkflowData["selected_candidate"] = selected
	resp.WorkflowData["selection_performed"] = true
	resp.WorkflowData["mutation_performed"] = false
	return s.bindFreeStateContextToResponse(resp, requestContext)
}

func recoverPluginRecommendationInteractionFromPayload(interactionID string, payload map[string]any) (PendingInteraction, bool) {
	if firstStringFromMap(payload, "schema_version") != pluginRecommendationSchema ||
		firstStringFromMap(payload, "status") != "awaiting_selection" ||
		len(mapRowsValue(payload["recommendations"])) == 0 {
		return PendingInteraction{}, false
	}
	requestContext := cloneContext(firstMapFromAny(payload["request_context"]))
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	requestContext["plugin_recommendation_recovered"] = true
	return PendingInteraction{
		ID: interactionID, Kind: "plugin_recommendation_question", Type: "plugin_recommendation_selection",
		Source: "plugin_recommendation", Workflow: pluginRecommendationWorkflow, Stage: "awaiting_selection",
		ConversationID: firstStringFromMap(requestContext, "conversation_id"),
		GoalID:         firstStringFromMap(requestContext, "goal_id"), RunID: firstStringFromMap(requestContext, "run_id"),
		RequestContext: requestContext, Payload: payload, Data: payload,
	}, true
}

func (s *Server) verifyRecoveredPluginRecommendationCandidate(ctx context.Context, processorType string, selected map[string]any) (map[string]any, error) {
	candidates, err := s.localPluginRecommendationCandidates(ctx, processorType)
	if err != nil {
		return nil, err
	}
	return verifyRecoveredPluginRecommendationCandidateAgainst(processorType, selected, candidates)
}

func verifyRecoveredPluginRecommendationCandidateAgainst(processorType string, selected map[string]any, candidates []pluginRecommendationCandidate) (map[string]any, error) {
	wantKey := strings.TrimSpace(firstStringFromMap(selected, "candidate_key"))
	if wantKey == "" {
		return nil, fmt.Errorf("recovered plugin candidate_key is required")
	}
	wantIdentifier := firstStringFromMap(selected, "identifier")
	wantPath := firstStringFromMap(selected, "plugin_path")
	wantName := firstStringFromMap(selected, "name")
	for _, candidate := range candidates {
		if candidate.Key != wantKey {
			continue
		}
		identifierMatch := wantIdentifier != "" && strings.EqualFold(candidate.Identifier, wantIdentifier)
		pathNameMatch := wantIdentifier == "" && strings.EqualFold(candidate.PluginPath, wantPath) && strings.EqualFold(candidate.Name, wantName)
		if !identifierMatch && !pathNameMatch {
			continue
		}
		if wantPath != "" && !strings.EqualFold(candidate.PluginPath, wantPath) {
			return nil, fmt.Errorf("selected plugin path changed since recommendation")
		}
		candidate.Key = firstStringFromMap(selected, "candidate_key")
		choice := pluginRecommendationChoice{
			CandidateKey: candidate.Key, Role: firstStringFromMap(selected, "role"),
			Reason: firstStringFromMap(selected, "reason"), Tradeoff: firstStringFromMap(selected, "tradeoff"),
			Confidence: firstStringFromMap(selected, "confidence"),
		}
		return pluginRecommendationChoiceRow(choice, candidate), nil
	}
	return nil, fmt.Errorf("selected plugin is no longer present in the local %s catalog", processorType)
}
