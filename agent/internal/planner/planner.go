package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/promptruntime"
)

type ToolCall struct {
	ID         string         `json:"id,omitempty"`
	Tool       string         `json:"tool,omitempty"`
	Args       map[string]any `json:"args,omitempty"`
	Command    map[string]any `json:"command,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	PlanItemID string         `json:"plan_item_id,omitempty"`
}

type ToolResult struct {
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Tool       string         `json:"tool,omitempty"`
	Status     string         `json:"status,omitempty"`
	Error      string         `json:"error,omitempty"`
	Result     map[string]any `json:"result,omitempty"`
}

type TraceEvent struct {
	Kind         string              `json:"kind"`
	Message      string              `json:"message,omitempty"`
	ToolCall     *ToolCall           `json:"tool_call,omitempty"`
	ToolResult   *ToolResult         `json:"tool_result,omitempty"`
	Verification *VerificationResult `json:"verification,omitempty"`
	PlanItems    []PlanItem          `json:"plan_items,omitempty"`
	Reply        string              `json:"reply,omitempty"`
}

type PlanItem struct {
	ID          string         `json:"id,omitempty"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	Evidence    string         `json:"evidence,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Completion struct {
	SatisfiedPlanIDs []string `json:"satisfied_plan_ids,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
	Uncertain        bool     `json:"uncertain,omitempty"`
}

type VerificationResult struct {
	ToolCallID    string         `json:"tool_call_id,omitempty"`
	PlanItemID    string         `json:"plan_item_id,omitempty"`
	Tool          string         `json:"tool,omitempty"`
	CommandName   string         `json:"command_name,omitempty"`
	Status        string         `json:"status,omitempty"`
	Postcondition string         `json:"postcondition,omitempty"`
	Message       string         `json:"message,omitempty"`
	Evidence      map[string]any `json:"evidence,omitempty"`
}

type Input struct {
	GoalID                string         `json:"goal_id,omitempty"`
	RunID                 string         `json:"run_id,omitempty"`
	UserText              string         `json:"user_text"`
	Context               map[string]any `json:"context,omitempty"`
	ContextSnapshot       map[string]any `json:"context_snapshot,omitempty"`
	State                 map[string]any `json:"state,omitempty"`
	CatalogSummary        string         `json:"catalog_summary,omitempty"`
	AllowedTools          []string       `json:"allowed_tools,omitempty"`
	Trace                 []TraceEvent   `json:"trace,omitempty"`
	PlanItems             []PlanItem     `json:"plan_items,omitempty"`
	MaxToolCallsRemaining int            `json:"max_tool_calls_remaining,omitempty"`
}

type Output struct {
	Done                  bool                `json:"done"`
	Reply                 string              `json:"reply,omitempty"`
	NeedsClarification    bool                `json:"needs_clarification,omitempty"`
	ClarificationQuestion string              `json:"clarification_question,omitempty"`
	FailureReason         string              `json:"failure_reason,omitempty"`
	PlanItems             []PlanItem          `json:"plan_items,omitempty"`
	Completion            *Completion         `json:"completion,omitempty"`
	Verification          *VerificationResult `json:"verification,omitempty"`
	ToolCalls             []ToolCall          `json:"tool_calls,omitempty"`
	Raw                   string              `json:"-"`
}

type Completer interface {
	Complete(context.Context, config.EngineConfig, []llm.Message) (string, error)
}

type LLMPlanner struct {
	Client Completer
	Config config.EngineConfig
}

func (p LLMPlanner) Next(ctx context.Context, in Input) (Output, error) {
	client := p.Client
	if client == nil {
		client = &llm.Client{}
	}
	if !p.Config.Complete() {
		return Output{}, fmt.Errorf("planner LLM config incomplete")
	}
	assembly := p.assembly(in)
	raw, err := llm.CompleteText(ctx, client, p.Config, llm.Request{
		Messages: assembly.Messages,
		Metadata: llm.RequestMetadata{
			Source:            "planner",
			GoalID:            in.GoalID,
			PromptFingerprint: assembly.Fingerprint,
			PromptStats:       assembly.Stats.Map(),
		},
	})
	if err != nil {
		return Output{}, err
	}
	out, err := ParseOutput(raw)
	if err != nil {
		return Output{Raw: raw}, err
	}
	out.Raw = raw
	for i := range out.ToolCalls {
		out.ToolCalls[i].Tool = strings.TrimSpace(out.ToolCalls[i].Tool)
		out.ToolCalls[i].PlanItemID = strings.TrimSpace(out.ToolCalls[i].PlanItemID)
		if out.ToolCalls[i].Args == nil {
			out.ToolCalls[i].Args = map[string]any{}
		}
	}
	return out, nil
}

func (p LLMPlanner) messages(in Input) []llm.Message {
	return p.assembly(in).Messages
}

func (p LLMPlanner) assembly(in Input) promptruntime.Assembly {
	allowed := append([]string(nil), in.AllowedTools...)
	sort.Strings(allowed)
	state, _ := json.MarshalIndent(compactValue(in.State), "", "  ")
	ctx, _ := json.MarshalIndent(compactValue(in.Context), "", "  ")
	trace, _ := json.MarshalIndent(compactValue(in.Trace), "", "  ")
	planItems, _ := json.MarshalIndent(compactValue(in.PlanItems), "", "  ")
	snapshot, _ := json.MarshalIndent(compactValue(in.ContextSnapshot), "", "  ")
	system := fmt.Sprintf(`You are Ask Vit's controlled ReAct planner inside Vit DAW.
Return ONLY strict JSON:
{"done":true,"reply":"short final user-facing reply","completion":{"satisfied_plan_ids":["id"],"evidence":["state/tool evidence"],"uncertain":false},"tool_calls":[]}
or
{"done":false,"needs_clarification":true,"clarification_question":"ask the user exactly what target/choice is missing","reply":"same question in user-facing wording","tool_calls":[]}
or
{"done":false,"reply":"short progress note","plan_items":[{"id":"short_id","description":"observable subtask","status":"pending"}],"tool_calls":[{"tool":"workspace.grep","args":{"root_id":"agent","query":"text"},"plan_item_id":"short_id","reason":"why"}]}

Rules:
- Use only tools from Allowed tools. For low-level DAW commands, use tool:"daw.invoke" only when daw.invoke is explicitly listed in Allowed tools, with args containing cmd.
- Do not invent tool names or IDs.
- Use at most 3 tool calls per turn.
- Prefer read-only observation before confirmed writes.
- The current Goal line is the only executable user instruction for this run. Conversation/history/recent_turns are historical context only; do not add actions from prior turns unless the current Goal explicitly says to continue, repeat, reuse, or base work on earlier context.
- The runner enforces a mutation barrier: after any project-mutating tool runs, the current batch stops, state is refreshed, execution_memory is updated, and you will be called again. Do not rely on later mutating calls from the same batch using IDs created by an earlier call.
- For compound tasks such as "create a track then load an EQ", keep both outcomes as plan_items, but after the create step use execution_memory.last_created_track_id / active_work_target_track_id from the next context snapshot before loading the plugin.
- Target resolution priority is: explicit user-named/indexed target, then object created in this run, then user-explicit "current/selected/this" UI target, then a single unambiguous default target, otherwise ask for clarification.
- Do not use the selected/current track as a hidden fallback when the user did not refer to it and multiple editable tracks exist.
- Mutating or external tools may pause for user confirmation; do not claim they already ran.
- If the requested clip, track, plugin, or selected object is ambiguous or missing, set needs_clarification:true and ask a concise clarification question. Do not guess object IDs.
- If multiple clips, tracks, or plugins match the request and the user did not specify which one, ask for clarification before calling tools.
- Keep plan_items small and objective: one item per requested observable outcome. Reuse existing plan item ids when continuing.
- Mark done:true only when all requested outcomes are evidenced by tool results or the current context snapshot. If a write has not been verified, continue with an observation or repair tool call.
- If enough information is available, set done:true and provide a concise reply.
- Never expose internal IDs in reply unless the user asks for technical details.
- When Context snapshot is present, use it as the primary source for conversation, DAW, plugin, and tool-result context. Raw Trace is only a small recency window.
- Do not proactively plan version.checkpoint for ordinary write operations; VitAgent runtime creates one automatic Project History safety checkpoint before the first mutating tool in a goal. Use Project History tools only when the user explicitly asks to list history, create/check out a branch, create a worktree, restore, or inspect checkpoints.
- For ordinary plugin/effect loading, search the semantic plugin library when needed, then load the chosen plugin with plugin.load_to_rack or rack.add_node so it appears inside the rack workflow.
- Rack zone rules: synths, samplers, MIDI instruments, and plugins marked is_instrument must be loaded to zone_id "Z2"; audio effects such as EQ, compressor, delay, reverb, analyzer, meter, or distortion belong in zone_id "Z3". If plugin metadata is unavailable, omit zone_id and let the tool resolve it.
- For plugin library inventory, search, recommendation, and type/category questions, use read-only plugin tools such as plugin.list_available, plugin.search, or plugin.semantic_search before answering unless the context snapshot already contains current evidence. Preserve compound requests, for example "what plugins are available and what types are they", as separate plan items or a single plan item with both observable outcomes.
- Do not use plugin.instantiate for normal plugin loading; use it only when the user explicitly asks for a track-level plugin outside the rack.
- If the user asks to load multiple plugin types, such as EQ and reverb, plan every requested plugin. Do not stop after loading or confirming only the first one.
- For runtime acoustic plugin adjustments on an already loaded/learned plugin, such as cutting mud near 500Hz, boosting presence, or reducing harshness, use plugin_grabber.apply_control with control eq.cut_region/eq.boost_region/eq.set_region and target freq_hz/gain_db/q when known. Do not use plugin.set_parameter/set_plugin_param for acoustic targets unless the user explicitly gives an exact param_id and raw value.
- For explicit one-parameter plugin control where the user names a concrete param_id and display value/unit, and get_plugin_parameters display_probe evidence is high confidence, plugin.set_parameter may use value_text such as "1000 ms" or "28 percent" without a saved Plugin Skill. Long-term semantic control, multi-parameter control, and automatic mixing still require a learned Plugin Skill.

Available tool catalog:
%s

Allowed tools:
%s`, in.CatalogSummary, strings.Join(allowed, ", "))
	user := ""
	if len(in.ContextSnapshot) > 0 {
		user = fmt.Sprintf("Goal: %s\nGoalID: %s\nRunID: %s\nRemaining tool calls this run: %d\nCurrent plan items JSON:\n%s\nContext snapshot JSON:\n%s\nRecent raw trace JSON:\n%s", strings.TrimSpace(in.UserText), in.GoalID, in.RunID, in.MaxToolCallsRemaining, string(planItems), string(snapshot), string(trace))
	} else {
		user = fmt.Sprintf("Goal: %s\nGoalID: %s\nRunID: %s\nRemaining tool calls this run: %d\nCurrent plan items JSON:\n%s\nSelected/context JSON:\n%s\nCurrent DAW state JSON:\n%s\nPrior trace JSON:\n%s", strings.TrimSpace(in.UserText), in.GoalID, in.RunID, in.MaxToolCallsRemaining, string(planItems), string(ctx), string(state), string(trace))
	}
	return promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "planner_system", "", system, true),
		},
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionRuntime, "planner_runtime", "", user, false),
		},
	})
}

func ParseOutput(raw string) (Output, error) {
	var out Output
	text := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(text), &out); err == nil {
		return normalizeOutput(out), nil
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(text[start:end+1]), &out); err == nil {
			return normalizeOutput(out), nil
		}
	}
	return Output{}, fmt.Errorf("planner returned invalid JSON")
}

func normalizeOutput(out Output) Output {
	out.Reply = strings.TrimSpace(out.Reply)
	out.ClarificationQuestion = strings.TrimSpace(out.ClarificationQuestion)
	out.FailureReason = strings.TrimSpace(out.FailureReason)
	out.PlanItems = normalizePlanItems(out.PlanItems)
	if out.Completion != nil {
		out.Completion.SatisfiedPlanIDs = cleanStringList(out.Completion.SatisfiedPlanIDs)
		out.Completion.Evidence = cleanStringList(out.Completion.Evidence)
	}
	if out.Verification != nil {
		normalizeVerification(out.Verification)
	}
	clean := make([]ToolCall, 0, len(out.ToolCalls))
	for _, call := range out.ToolCalls {
		call.ID = strings.TrimSpace(call.ID)
		call.Tool = strings.TrimSpace(call.Tool)
		call.Reason = strings.TrimSpace(call.Reason)
		call.PlanItemID = strings.TrimSpace(call.PlanItemID)
		if call.Tool == "" && len(call.Command) == 0 {
			continue
		}
		if call.Args == nil {
			call.Args = map[string]any{}
		}
		clean = append(clean, call)
	}
	out.ToolCalls = clean
	if out.Done || out.NeedsClarification {
		out.ToolCalls = nil
	}
	return out
}

func normalizeVerification(ver *VerificationResult) {
	if ver == nil {
		return
	}
	ver.ToolCallID = strings.TrimSpace(ver.ToolCallID)
	ver.PlanItemID = strings.TrimSpace(ver.PlanItemID)
	ver.Tool = strings.TrimSpace(ver.Tool)
	ver.CommandName = strings.TrimSpace(ver.CommandName)
	ver.Status = strings.TrimSpace(ver.Status)
	ver.Postcondition = strings.TrimSpace(ver.Postcondition)
	ver.Message = strings.TrimSpace(ver.Message)
	if len(ver.Evidence) == 0 {
		ver.Evidence = nil
	}
}

func normalizePlanItems(items []PlanItem) []PlanItem {
	out := make([]PlanItem, 0, len(items))
	for _, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Description = strings.TrimSpace(item.Description)
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		item.Evidence = strings.TrimSpace(item.Evidence)
		if item.ID == "" && item.Description == "" {
			continue
		}
		if item.Status == "" {
			item.Status = "pending"
		}
		if len(item.Metadata) == 0 {
			item.Metadata = nil
		}
		out = append(out, item)
	}
	return out
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func compactValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]any:
		out := map[string]any{}
		for key, val := range v {
			out[key] = compactValue(val)
		}
		return out
	case []TraceEvent:
		limit := len(v)
		if limit > 12 {
			v = v[limit-12:]
		}
		return v
	case []any:
		limit := len(v)
		if limit > 20 {
			limit = 20
		}
		out := make([]any, 0, limit)
		for i := 0; i < limit; i++ {
			out = append(out, compactValue(v[i]))
		}
		return out
	case string:
		r := []rune(v)
		if len(r) > 900 {
			return string(r[:900]) + "...<truncated>"
		}
		return v
	default:
		return v
	}
}
