# ContextRuntime 修复开发文档

**版本**: v1.0  
**日期**: 2026-08-11  
**目标**: 修复上下文重复装配问题，将上下文体积从 60k+ 降至 10k 以下

---

## 一、问题诊断总结

### 1.1 核心问题

**同一份 ToolResult 数据被重复装配到 4 个位置：**

1. `Snapshot.ToolResultSummary` (顶层字段) - 完整工具结果数组
2. `GoalTraceSummary.recent_events[].tool_result` - 最近 12 个 events 中每个都重复
3. `DAWSemanticSummary.recent_execution_result` - 最后一个 ToolResult
4. `RecentGoalContext.recent_tool_result` - 从 trace 倒查的最后一个 ToolResult

### 1.2 具体影响

**实测数据：**
- 初始请求：6,916 字符
- 执行 ccb.observation_catalog 后：63,592 字符 (+56,676, 约 9.2x 增长)
- CCB catalog 响应：约 15.5k

**放大机制：**
- CCB Bundle 包含多个投影模型（COM, DOM, MOM 等）
- 每个投影模型有完整数据（20-30 KB）+ LLMContext（2-3 KB）
- contextruntime 错误地展开了完整数据，并装配了 4 次

### 1.3 根本原因

**contextruntime 没有理解 CCB Bundle 的复合结构：**
- CCB Bundle 是一个"购物袋"，里面装了多个投影模型
- 每个投影模型都有 LLMContext 字段（专门为 LLM 准备的紧凑摘要）
- contextruntime 应该提取 LLMContext，而不是展开完整数据


---

## 二、修复方案

### 2.1 修复原则

1. **尊重投影模型的自描述能力**：优先使用投影模型提供的 LLMContext
2. **消除重复装配**：同一数据在 Snapshot 中只保留一份
3. **引入引用机制**：其他位置只保留 tool_call_id 引用
4. **向后兼容**：尽量保持对外接口不变

### 2.2 修复内容

分为 3 个修理项（建议全部完成）：

- **修理项 1**: 增加 CCB Bundle 识别和拆包逻辑
- **修理项 2**: 消除重复装配
- **修理项 3**: 引入轻量 Ledger 机制

---

## 三、修理项 1：CCB Bundle 识别和拆包

### 3.1 目标

让 summarizeToolResult() 能够：
1. 识别 ToolResult 是否为 CCB Bundle
2. 从 Bundle 中提取每个投影模型的 LLMContext
3. 不展开投影模型的完整数据

### 3.2 实现步骤

#### Step 1：增加 CCB Bundle 识别函数

**位置**: agent/internal/contextruntime/context.go

**新增函数**:

```go
// isCCBBundle 检查 ToolResult 是否为 CCB FreeStateObservationBundle
func isCCBBundle(result map[string]any) bool {
    schema, _ := result["schema_version"].(string)
    return schema == "ccb_observation_bundle.v1"
}
```

#### Step 2：增加投影模型 LLMContext 提取函数

**新增函数**:

```go
// extractLLMContext 从投影模型中提取 llm_context 字段
// 支持所有投影模型：COM, DOM, MOM, TOM, TIM, FXM, EPM, RLM
func extractLLMContext(projection map[string]any) map[string]any {
    llmCtx, ok := projection["llm_context"].(map[string]any)
    if !ok {
        return nil
    }
    
    // 检查是否标记了 do_not_include_raw_package
    // 如果为 true，说明投影模型明确要求只使用 LLMContext
    if doNotInclude, _ := llmCtx["do_not_include_raw_package"].(bool); doNotInclude {
        return llmCtx
    }
    
    // 即使没有明确标记，只要有 llm_context 就优先使用
    return llmCtx
}
```


#### Step 3：增加 CCB Bundle 专用装配函数

**新增函数**:

```go
// summarizeCCBBundle 专门处理 CCB FreeStateObservationBundle
// 从 Bundle.Views 中提取每个投影模型的 LLMContext
func summarizeCCBBundle(result planner.ToolResult, opts Options) map[string]any {
    bundle := result.Result
    
    out := map[string]any{
        "tool_call_id": result.ToolCallID,
        "tool":         result.Tool,
        "status":       result.Status,
        "bundle_type":  "ccb_observation_bundle",
    }
    
    // 提取 Bundle 元信息
    if obsID := bundle["observation_id"]; obsID != nil {
        out["observation_id"] = obsID
    }
    if reqViews := bundle["requested_views"]; reqViews != nil {
        out["requested_views"] = reqViews
    }
    if disclosureBytes := bundle["disclosure_bytes"]; disclosureBytes != nil {
        out["disclosure_bytes"] = disclosureBytes
    }
    
    // 遍历 Views，提取每个投影模型的 LLMContext
    views, ok := bundle["views"].(map[string]any)
    if !ok {
        // 如果 Views 不存在或格式错误，回退到通用处理
        out["result_preview"] = compactValue(bundle, opts, 0)
        return out
    }
    
    viewContexts := make(map[string]any)
    viewSummaries := []string{}
    
    for viewID, viewData := range views {
        projection, ok := viewData.(map[string]any)
        if !ok {
            continue
        }
        
        // 优先提取 LLMContext
        if llmCtx := extractLLMContext(projection); llmCtx != nil {
            viewContexts[viewID] = llmCtx
            
            // 收集 summary 用于生成整体 digest
            if summary, ok := llmCtx["summary_md"].(string); ok {
                viewSummaries = append(viewSummaries, summary)
            }
        } else {
            // 如果投影模型没有 LLMContext，使用通用压缩
            viewContexts[viewID] = compactValue(projection, opts, 0)
        }
    }
    
    if len(viewContexts) > 0 {
        out["view_contexts"] = viewContexts
    }
    
    // 生成整体 digest
    if len(viewSummaries) > 0 {
        out["digest"] = fmt.Sprintf("CCB Bundle 包含 %d 个观察视角: %s",
            len(viewSummaries), strings.Join(viewSummaries, "; "))
    } else {
        out["digest"] = fmt.Sprintf("CCB Bundle 包含 %d 个观察视角", len(viewContexts))
    }
    
    return out
}
```


#### Step 4：修改 summarizeToolResult 函数

**位置**: context.go line 511-529

**修改说明**：在函数开始处增加 CCB Bundle 和 LLMContext 的识别逻辑

**修改后的完整函数**:

```go
func summarizeToolResult(result planner.ToolResult, opts Options) map[string]any {
    out := map[string]any{
        "tool_call_id": result.ToolCallID,
        "tool":         result.Tool,
        "status":       result.Status,
    }
    
    if strings.TrimSpace(result.Error) != "" {
        out["error"] = compactText(result.Error, opts.MaxTextRunes)
    }
    
    if len(result.Result) > 0 {
        // ===== 新增：识别并特殊处理 CCB Bundle =====
        if isCCBBundle(result.Result) {
            return summarizeCCBBundle(result, opts)
        }
        
        // ===== 新增：对于其他投影模型，优先提取 LLMContext =====
        if llmCtx := extractLLMContext(result.Result); llmCtx != nil {
            out["llm_context"] = llmCtx
            if summary, ok := llmCtx["summary_md"].(string); ok {
                out["digest"] = summary
            }
            // 不生成 result_preview，因为 llm_context 已经是最优表达
            return out
        }
        
        // ===== 原有的通用压缩逻辑（兜底）=====
        out["result_keys"] = sortedKeys(result.Result)
        important := map[string]any{}
        collectImportantFields(important, "", result.Result, 0, opts.MaxListItems)
        if len(important) > 0 {
            out["important_fields"] = important
        }
        out["result_preview"] = compactResultPreview(result.Result, compactValue(result.Result, opts, 0), opts)
    }
    
    return out
}
```

### 3.3 测试验证

创建测试文件 `context_ccb_test.go`：

```go
package contextruntime

import (
    "testing"
    "vit-daw-agent/internal/planner"
    "github.com/stretchr/testify/assert"
)

func TestIsCCBBundle(t *testing.T) {
    // CCB Bundle
    ccbBundle := map[string]any{
        "schema_version": "ccb_observation_bundle.v1",
        "views": map[string]any{},
    }
    assert.True(t, isCCBBundle(ccbBundle))
    
    // 普通 ToolResult
    normalResult := map[string]any{
        "status": "ok",
    }
    assert.False(t, isCCBBundle(normalResult))
}

func TestExtractLLMContext(t *testing.T) {
    // COM Projection with LLMContext
    comProjection := map[string]any{
        "schema_version": "com.projection.v1",
        "llm_context": map[string]any{
            "summary_md": "COM ready projection",
            "compact_facts": []map[string]any{},
            "do_not_include_raw_package": true,
        },
    }
    
    llmCtx := extractLLMContext(comProjection)
    assert.NotNil(t, llmCtx)
    assert.Equal(t, "COM ready projection", llmCtx["summary_md"])
}

func TestSummarizeCCBBundle(t *testing.T) {
    bundle := map[string]any{
        "schema_version": "ccb_observation_bundle.v1",
        "observation_id": "obs_123",
        "requested_views": []string{"com_source_dynamics", "dom_paired_io"},
        "disclosure_bytes": 35000,
        "views": map[string]any{
            "com_source_dynamics": map[string]any{
                "llm_context": map[string]any{
                    "summary_md": "COM ready source_only projection",
                    "do_not_include_raw_package": true,
                },
            },
            "dom_paired_io": map[string]any{
                "llm_context": map[string]any{
                    "summary_md": "DOM ready paired_io projection",
                    "do_not_include_raw_package": true,
                },
            },
        },
    }
    
    result := planner.ToolResult{
        ToolCallID: "call_123",
        Tool: "ccb.observation_request",
        Status: "ok",
        Result: bundle,
    }
    
    opts := normalizeOptions(Options{})
    summary := summarizeCCBBundle(result, opts)
    
    assert.Equal(t, "ccb_observation_bundle", summary["bundle_type"])
    assert.Equal(t, "obs_123", summary["observation_id"])
    
    viewContexts := summary["view_contexts"].(map[string]any)
    assert.Len(t, viewContexts, 2)
    
    digest := summary["digest"].(string)
    assert.Contains(t, digest, "CCB Bundle 包含 2 个观察视角")
}
```


---

## 四、修理项 2：消除重复装配

### 4.1 目标

删除 ToolResult 的 3 个重复副本，只保留一个唯一入口。

### 4.2 实现步骤

#### Step 1：删除 Snapshot.ToolResultSummary 字段

**位置**: context.go line 53-72

**修改**:
```go
type Snapshot struct {
    SchemaVersion         string           `json:"schema_version"`
    CreatedAt             string           `json:"created_at,omitempty"`
    ConversationID        string           `json:"conversation_id,omitempty"`
    GoalID                string           `json:"goal_id,omitempty"`
    RunID                 string           `json:"run_id,omitempty"`
    UserText              string           `json:"user_text,omitempty"`
    GoalSummary           string           `json:"goal_summary,omitempty"`
    ConversationSummary   map[string]any   `json:"conversation_summary"`
    GoalTraceSummary      map[string]any   `json:"goal_trace_summary,omitempty"`
    RecentTurns           []llm.Message    `json:"recent_turns,omitempty"`
    CurrentSelection      map[string]any   `json:"current_selection"`
    DAWStateSummary       map[string]any   `json:"daw_state_summary"`
    DAWSemanticSummary    map[string]any   `json:"daw_semantic_summary,omitempty"`
    RecentGoalContext     map[string]any   `json:"recent_goal_context,omitempty"`
    ProjectHistorySummary map[string]any   `json:"project_history_summary,omitempty"`
    PluginContextSummary  map[string]any   `json:"plugin_context_summary,omitempty"`
    // ToolResultSummary 已删除
    Warnings              []string         `json:"warnings,omitempty"`
}
```

**注意**：这是 breaking change，需要检查消费侧代码。

#### Step 2：修改 compactTraceEvent，只保留引用

**位置**: context.go line 448-500

**修改 tool_result 处理逻辑** (line 466-468):

```go
if ev.ToolResult != nil {
    // 只保留引用，不包含完整结果
    out["tool_result_ref"] = map[string]any{
        "tool_call_id": ev.ToolResult.ToolCallID,
        "tool":         ev.ToolResult.Tool,
        "status":       ev.ToolResult.Status,
    }
    if strings.TrimSpace(ev.ToolResult.Error) != "" {
        out["tool_result_ref"].(map[string]any)["error"] = compactText(ev.ToolResult.Error, opts.MaxTextRunes/2)
    }
}
```

#### Step 3：删除 DAWSemanticSummary.recent_execution_result

**位置**: context.go line 654-693

**修改 summarizeDAWSemantic 函数**，删除以下代码 (line 687-689):

```go
// 删除以下代码：
// if len(toolResultSummary) > 0 {
//     out["recent_execution_result"] = toolResultSummary[len(toolResultSummary)-1]
// }
```

**同时修改函数签名**，删除 `toolResultSummary` 参数:

```go
// 修改前：
func summarizeDAWSemantic(selection, dawStateSummary, projectHistorySummary, rawProjectHistory, pluginContextSummary map[string]any, toolResultSummary []map[string]any, ctx map[string]any, opts Options) map[string]any

// 修改后：
func summarizeDAWSemantic(selection, dawStateSummary, projectHistorySummary, rawProjectHistory, pluginContextSummary map[string]any, ctx map[string]any, opts Options) map[string]any
```

#### Step 4：修改 summarizeRecentGoalContext，只保留引用

**位置**: context.go line 802-836

**修改 recent_tool_result 逻辑** (line 826-828):

```go
// 修改前：
if result := latestToolResultFromTrace(trace, opts); len(result) > 0 {
    out["recent_tool_result"] = result
}

// 修改后：
if toolCallID := latestToolCallIDFromTrace(trace); toolCallID != "" {
    out["recent_tool_call_id"] = toolCallID
}
```

**新增辅助函数**:

```go
// latestToolCallIDFromTrace 从 trace 中提取最新的 tool_call_id
func latestToolCallIDFromTrace(trace []planner.TraceEvent) string {
    for i := len(trace) - 1; i >= 0; i-- {
        if trace[i].ToolResult != nil {
            return trace[i].ToolResult.ToolCallID
        }
    }
    return ""
}
```

#### Step 5：修改 Build 函数调用

**位置**: context.go line 74-148

**修改 Build 函数**，删除 toolResultSummary 的生成和传递:

```go
func Build(in Input, opts Options) Snapshot {
    opts = normalizeOptions(opts)
    now := opts.Now()
    recentTurns, conversationSummary := summarizeConversation(in.Conversation, opts)
    
    // 修改：summarizeTrace 不再返回 toolResultSummary
    goalTraceSummary := summarizeTrace(in.GoalTrace, opts)
    
    selection := summarizeSelection(in.Context, opts)
    dawStateSummary := summarizeDAWState(in.State, opts)
    projectHistorySummary := summarizeProjectHistory(in.ProjectHistorySummary, in.State, opts)
    
    // ... plugin context 相关代码保持不变
    
    // 修改：不再传递 toolResultSummary
    dawSemanticSummary := summarizeDAWSemantic(selection, dawStateSummary, projectHistorySummary, in.ProjectHistorySummary, pluginContextSummary, in.Context, opts)
    recentGoalContext := summarizeRecentGoalContext(in.GoalTrace, in.PlanItems, in.PendingToolCall, in.PendingToolQueue, in.ExecutionMemory, in.RecentObservation, in.PreviousSnapshot, opts)
    
    return Snapshot{
        // ... 字段保持不变，但不包含 ToolResultSummary
    }
}
```

**修改 summarizeTrace 函数签名** (line 387):

```go
// 修改前：
func summarizeTrace(trace []planner.TraceEvent, opts Options) (map[string]any, []map[string]any)

// 修改后：
func summarizeTrace(trace []planner.TraceEvent, opts Options) map[string]any
```

**修改 summarizeTrace 函数实现**，删除 toolResults 的收集和返回:

```go
func summarizeTrace(trace []planner.TraceEvent, opts Options) map[string]any {
    summary := map[string]any{
        "event_count":         len(trace),
        "omitted_event_count": 0,
    }
    if len(trace) > opts.RecentTraceEvents {
        summary["omitted_event_count"] = len(trace) - opts.RecentTraceEvents
    }
    toolCounts := map[string]int{}
    errorCount := 0
    var lastAssistant, lastError string
    
    // 删除以下代码：
    // toolResults := []map[string]any{}
    
    for _, ev := range trace {
        if strings.TrimSpace(ev.Reply) != "" {
            lastAssistant = compactText(ev.Reply, opts.MaxTextRunes/2)
        }
        if ev.ToolCall != nil {
            tool := strings.TrimSpace(ev.ToolCall.Tool)
            if tool != "" {
                toolCounts[tool]++
            }
        }
        if ev.ToolResult != nil {
            if strings.TrimSpace(ev.ToolResult.Tool) != "" {
                toolCounts[ev.ToolResult.Tool]++
            }
            if strings.TrimSpace(ev.ToolResult.Error) != "" || strings.EqualFold(ev.ToolResult.Status, "error") || strings.EqualFold(ev.ToolResult.Status, "kernel_error") {
                errorCount++
                lastError = compactText(firstNonEmpty(ev.ToolResult.Error, ev.ToolResult.Status), opts.MaxTextRunes/2)
            }
            // 删除以下代码：
            // toolResults = append(toolResults, summarizeToolResult(*ev.ToolResult, opts))
        }
        if strings.EqualFold(ev.Kind, "planner_error") && strings.TrimSpace(ev.Message) != "" {
            errorCount++
            lastError = compactText(ev.Message, opts.MaxTextRunes/2)
        }
    }
    
    if len(toolCounts) > 0 {
        summary["tool_counts"] = toolCounts
    }
    if errorCount > 0 {
        summary["error_count"] = errorCount
        summary["last_error"] = lastError
    }
    if lastAssistant != "" {
        summary["last_assistant_reply"] = lastAssistant
    }
    if len(trace) > 0 {
        start := 0
        if len(trace) > opts.RecentTraceEvents {
            start = len(trace) - opts.RecentTraceEvents
        }
        recent := make([]map[string]any, 0, len(trace)-start)
        for _, ev := range trace[start:] {
            recent = append(recent, compactTraceEvent(ev, opts))
        }
        summary["recent_events"] = recent
    }
    
    // 修改：只返回 summary
    return summary
}
```


### 4.3 测试验证

创建测试文件 `context_dedup_test.go`：

```go
package contextruntime

import (
    "testing"
    "vit-daw-agent/internal/planner"
    "github.com/stretchr/testify/assert"
)

func TestNoToolResultDuplication(t *testing.T) {
    // 构造输入
    largeBundle := map[string]any{
        "schema_version": "ccb_observation_bundle.v1",
        "observation_id": "obs_large",
        "views": map[string]any{
            "com_source_dynamics": map[string]any{
                "llm_context": map[string]any{
                    "summary_md": "COM projection",
                    "do_not_include_raw_package": true,
                },
            },
        },
    }
    
    input := Input{
        GoalTrace: []planner.TraceEvent{
            {
                Kind: "tool_result",
                ToolResult: &planner.ToolResult{
                    ToolCallID: "call_123",
                    Tool: "ccb.observation_request",
                    Status: "ok",
                    Result: largeBundle,
                },
            },
        },
    }
    
    opts := Options{}
    snapshot := Build(input, opts)
    
    // 1. 顶层不应有 ToolResultSummary
    _, hasToolResultSummary := snapshot.Map()["tool_result_summary"]
    assert.False(t, hasToolResultSummary, "ToolResultSummary should be removed")
    
    // 2. recent_events 中只有引用
    events := snapshot.GoalTraceSummary["recent_events"].([]map[string]any)
    if len(events) > 0 {
        ref, hasRef := events[0]["tool_result_ref"]
        assert.True(t, hasRef, "Should have tool_result_ref")
        
        refMap := ref.(map[string]any)
        assert.Equal(t, "call_123", refMap["tool_call_id"])
        
        // 不应有完整的 tool_result
        _, hasResult := events[0]["tool_result"]
        assert.False(t, hasResult, "Should not have full tool_result in events")
    }
    
    // 3. DAWSemanticSummary 不应有 recent_execution_result
    _, hasExecResult := snapshot.DAWSemanticSummary["recent_execution_result"]
    assert.False(t, hasExecResult, "Should not have recent_execution_result")
    
    // 4. RecentGoalContext 只有 tool_call_id
    toolCallID, hasID := snapshot.RecentGoalContext["recent_tool_call_id"]
    assert.True(t, hasID, "Should have recent_tool_call_id")
    assert.Equal(t, "call_123", toolCallID)
    
    _, hasFullResult := snapshot.RecentGoalContext["recent_tool_result"]
    assert.False(t, hasFullResult, "Should not have full recent_tool_result")
}
```

---

## 五、修理项 3：引入轻量 Ledger 机制

### 5.1 目标

引入 ToolResultLedger，作为 ToolResult 的唯一存储位置，其他地方通过 tool_call_id 引用。

### 5.2 实现步骤

#### Step 1：定义 ToolResultEntry 结构

**位置**: context.go，在 Snapshot 定义之前

**新增类型**:

```go
// ToolResultEntry 是 ToolResult 在 Ledger 中的存储单元
type ToolResultEntry struct {
    ToolCallID string         `json:"tool_call_id"`
    Tool       string         `json:"tool"`
    Status     string         `json:"status"`
    Error      string         `json:"error,omitempty"`
    
    // 对于 CCB Bundle，保留 view_contexts
    BundleType   string         `json:"bundle_type,omitempty"`
    ViewContexts map[string]any `json:"view_contexts,omitempty"`
    
    // 对于其他投影模型，保留 llm_context
    LLMContext   map[string]any `json:"llm_context,omitempty"`
    
    // 兜底：通用压缩的 result_preview
    ResultPreview map[string]any `json:"result_preview,omitempty"`
    
    // Digest：一句话摘要
    Digest       string         `json:"digest,omitempty"`
    
    // 元信息
    MetaSummary  map[string]any `json:"meta_summary,omitempty"`
}
```

#### Step 2：修改 Snapshot 结构，增加 ToolResultLedger

**位置**: context.go line 53-72

**修改 Snapshot**:

```go
type Snapshot struct {
    SchemaVersion         string                       `json:"schema_version"`
    CreatedAt             string                       `json:"created_at,omitempty"`
    ConversationID        string                       `json:"conversation_id,omitempty"`
    GoalID                string                       `json:"goal_id,omitempty"`
    RunID                 string                       `json:"run_id,omitempty"`
    UserText              string                       `json:"user_text,omitempty"`
    GoalSummary           string                       `json:"goal_summary,omitempty"`
    ConversationSummary   map[string]any               `json:"conversation_summary"`
    GoalTraceSummary      map[string]any               `json:"goal_trace_summary,omitempty"`
    RecentTurns           []llm.Message                `json:"recent_turns,omitempty"`
    CurrentSelection      map[string]any               `json:"current_selection"`
    DAWStateSummary       map[string]any               `json:"daw_state_summary"`
    DAWSemanticSummary    map[string]any               `json:"daw_semantic_summary,omitempty"`
    RecentGoalContext     map[string]any               `json:"recent_goal_context,omitempty"`
    ProjectHistorySummary map[string]any               `json:"project_history_summary,omitempty"`
    PluginContextSummary  map[string]any               `json:"plugin_context_summary,omitempty"`
    
    // 新增：ToolResult 的唯一存储
    ToolResultLedger      map[string]*ToolResultEntry `json:"tool_result_ledger,omitempty"`
    
    Warnings              []string                     `json:"warnings,omitempty"`
}
```

#### Step 3：修改 Build 函数，先构建 Ledger

**位置**: context.go line 74-148

**在 Build 函数开始处添加**:

```go
func Build(in Input, opts Options) Snapshot {
    opts = normalizeOptions(opts)
    now := opts.Now()
    
    // ===== 新增：先构建 ToolResultLedger =====
    toolResultLedger := buildToolResultLedger(in.GoalTrace, in.ToolResults, opts)
    
    // 原有逻辑
    recentTurns, conversationSummary := summarizeConversation(in.Conversation, opts)
    goalTraceSummary := summarizeTrace(in.GoalTrace, opts)
    
    // ... 其他逻辑保持不变
    
    return Snapshot{
        SchemaVersion:         SchemaVersion,
        CreatedAt:             now.UTC().Format(time.RFC3339Nano),
        ConversationID:        strings.TrimSpace(in.ConversationID),
        GoalID:                strings.TrimSpace(in.GoalID),
        RunID:                 strings.TrimSpace(in.RunID),
        UserText:              compactText(in.UserText, opts.MaxTextRunes),
        GoalSummary:           compactText(in.GoalSummary, opts.MaxTextRunes),
        ConversationSummary:   conversationSummary,
        GoalTraceSummary:      goalTraceSummary,
        RecentTurns:           recentTurns,
        CurrentSelection:      selection,
        DAWStateSummary:       dawStateSummary,
        DAWSemanticSummary:    dawSemanticSummary,
        RecentGoalContext:     recentGoalContext,
        ProjectHistorySummary: projectHistorySummary,
        PluginContextSummary:  pluginContextSummary,
        ToolResultLedger:      toolResultLedger,  // 新增
        Warnings:              warnings,
    }
}
```


#### Step 4：实现 buildToolResultLedger 函数

**新增函数**:

```go
// buildToolResultLedger 构建 ToolResult 的 Ledger
// 从 trace 和 in.ToolResults 收集所有 ToolResult，去重后装配
func buildToolResultLedger(trace []planner.TraceEvent, results []planner.ToolResult, opts Options) map[string]*ToolResultEntry {
    ledger := make(map[string]*ToolResultEntry)
    seen := make(map[string]bool)
    
    // 从 trace 中收集
    for _, ev := range trace {
        if ev.ToolResult != nil {
            toolCallID := ev.ToolResult.ToolCallID
            if seen[toolCallID] {
                continue
            }
            seen[toolCallID] = true
            
            entry := buildToolResultEntry(*ev.ToolResult, opts)
            ledger[toolCallID] = entry
        }
    }
    
    // 从 in.ToolResults 中补充
    for _, result := range results {
        toolCallID := result.ToolCallID
        if seen[toolCallID] {
            continue
        }
        seen[toolCallID] = true
        
        entry := buildToolResultEntry(result, opts)
        ledger[toolCallID] = entry
    }
    
    return ledger
}

// buildToolResultEntry 从 ToolResult 构建 ToolResultEntry
func buildToolResultEntry(result planner.ToolResult, opts Options) *ToolResultEntry {
    entry := &ToolResultEntry{
        ToolCallID: result.ToolCallID,
        Tool:       result.Tool,
        Status:     result.Status,
        Error:      result.Error,
    }
    
    if len(result.Result) == 0 {
        return entry
    }
    
    // 识别 CCB Bundle
    if isCCBBundle(result.Result) {
        bundleSummary := summarizeCCBBundle(result, opts)
        if bundleType, ok := bundleSummary["bundle_type"].(string); ok {
            entry.BundleType = bundleType
        }
        if viewContexts, ok := bundleSummary["view_contexts"].(map[string]any); ok {
            entry.ViewContexts = viewContexts
        }
        if digest, ok := bundleSummary["digest"].(string); ok {
            entry.Digest = digest
        }
        return entry
    }
    
    // 识别其他投影模型
    if llmCtx := extractLLMContext(result.Result); llmCtx != nil {
        entry.LLMContext = llmCtx
        if summary, ok := llmCtx["summary_md"].(string); ok {
            entry.Digest = summary
        }
        return entry
    }
    
    // 通用压缩（兜底）
    entry.ResultPreview = compactValue(result.Result, opts, 0)
    entry.MetaSummary = map[string]any{
        "result_keys": sortedKeys(result.Result),
    }
    
    return entry
}
```

### 5.3 测试验证

创建测试文件 `context_ledger_test.go`：

```go
package contextruntime

import (
    "testing"
    "vit-daw-agent/internal/planner"
    "github.com/stretchr/testify/assert"
)

func TestBuildToolResultLedger(t *testing.T) {
    trace := []planner.TraceEvent{
        {
            Kind: "tool_result",
            ToolResult: &planner.ToolResult{
                ToolCallID: "call_1",
                Tool: "ccb.observation_catalog",
                Status: "ok",
                Result: map[string]any{
                    "schema_version": "ccb_observation_bundle.v1",
                    "views": map[string]any{
                        "com_source_dynamics": map[string]any{
                            "llm_context": map[string]any{
                                "summary_md": "COM projection",
                            },
                        },
                    },
                },
            },
        },
        {
            Kind: "tool_result",
            ToolResult: &planner.ToolResult{
                ToolCallID: "call_2",
                Tool: "daw.get_tracks",
                Status: "ok",
                Result: map[string]any{
                    "tracks": []map[string]any{},
                },
            },
        },
    }
    
    opts := normalizeOptions(Options{})
    ledger := buildToolResultLedger(trace, []planner.ToolResult{}, opts)
    
    assert.Len(t, ledger, 2)
    
    // call_1 应该是 CCB Bundle
    entry1 := ledger["call_1"]
    assert.NotNil(t, entry1)
    assert.Equal(t, "ccb_observation_bundle", entry1.BundleType)
    assert.NotNil(t, entry1.ViewContexts)
    
    // call_2 应该是通用结果
    entry2 := ledger["call_2"]
    assert.NotNil(t, entry2)
    assert.NotNil(t, entry2.ResultPreview)
}

func TestToolResultLedgerDeduplication(t *testing.T) {
    // 同一个 tool_call_id 在 trace 和 results 中都出现
    trace := []planner.TraceEvent{
        {
            ToolResult: &planner.ToolResult{
                ToolCallID: "call_dup",
                Tool: "test",
                Status: "ok",
            },
        },
    }
    
    results := []planner.ToolResult{
        {
            ToolCallID: "call_dup",
            Tool: "test",
            Status: "ok",
        },
    }
    
    opts := normalizeOptions(Options{})
    ledger := buildToolResultLedger(trace, results, opts)
    
    // 应该只有一个 entry
    assert.Len(t, ledger, 1)
}
```

---

## 六、System Prompt 配合修改

### 6.1 说明引用机制

在 system prompt 中需要说明新的上下文结构，让 LLM 理解如何使用引用机制。

**建议在 system prompt 中添加：**

```markdown
## Context Snapshot Structure

The snapshot uses a **ledger + reference** architecture to avoid duplication:

### 1. tool_result_ledger
Complete storage of all tool results, keyed by tool_call_id. Each entry contains:
- For CCB Bundles: `view_contexts` (each view's LLMContext)
- For projection models: `llm_context` (compact facts and summary)
- For other results: `result_preview` (compressed data)
- `digest`: one-line summary of what happened

### 2. References in other sections
Other parts of the snapshot only contain tool_call_id references:
- `goal_trace_summary.recent_events[].tool_result_ref` - only metadata (tool_call_id, tool, status)
- `recent_goal_context.recent_tool_call_id` - only the ID string

### How to use
When you see a tool_call_id reference:
1. Look up the entry in tool_result_ledger using the tool_call_id
2. Read the digest for a quick summary
3. If you need details:
   - For CCB Bundles: check view_contexts for each projection model's LLMContext
   - For projection models: check llm_context.compact_facts
   - For other results: check result_preview

Example:
```json
{
  "recent_goal_context": {
    "recent_tool_call_id": "call_123"
  },
  "tool_result_ledger": {
    "call_123": {
      "tool": "ccb.observation_request",
      "status": "ok",
      "bundle_type": "ccb_observation_bundle",
      "digest": "CCB Bundle 包含 2 个观察视角: COM ready projection; DOM ready projection",
      "view_contexts": {
        "com_source_dynamics": {
          "summary_md": "COM ready source_only projection",
          "compact_facts": [...]
        },
        "dom_paired_io": {
          "summary_md": "DOM ready paired_io projection",
          "compact_facts": [...]
        }
      }
    }
  }
}
```
```

---

## 七、测试与验证

### 7.1 单元测试清单

| 测试文件 | 测试内容 | 状态 |
|---------|---------|------|
| `context_ccb_test.go` | CCB Bundle 识别和拆包 | 待创建 |
| `context_dedup_test.go` | 重复装配消除验证 | 待创建 |
| `context_ledger_test.go` | Ledger 构建和去重 | 待创建 |

### 7.2 集成测试

**测试场景 1：CCB Catalog 执行**
```
输入：执行 ccb.observation_catalog
预期：
- Snapshot 大小 < 10k
- tool_result_ledger 包含 1 个 entry (CCB Bundle)
- recent_events 只有 tool_result_ref
- 没有重复的完整数据
```

**测试场景 2：多个工具结果**
```
输入：执行多个工具 (daw.get_tracks + ccb.observation_request + plugin.set_parameter)
预期：
- tool_result_ledger 包含 3 个 entry
- 每个 entry 只出现一次
- CCB Bundle 的 view_contexts 正确提取
```

### 7.3 回归测试

确保修改不影响现有功能：

1. **Planner 测试**：确认 planner 仍能正常访问上下文
2. **Verifier 测试**：确认 verifier 仍能正常读取工具结果
3. **Goal Runner 测试**：确认 goal runner 的上下文传递正常


---

## 八、实施计划

### 8.1 阶段划分

**Phase 1: 核心修复（3-4 天）**
- Day 1: 实现修理项 1（CCB Bundle 识别和拆包）
  - 新增 isCCBBundle、extractLLMContext、summarizeCCBBundle
  - 修改 summarizeToolResult
  - 单元测试
- Day 2: 实现修理项 2（消除重复装配）
  - 删除 ToolResultSummary 字段
  - 修改 compactTraceEvent、summarizeDAWSemantic、summarizeRecentGoalContext
  - 修改 summarizeTrace 函数签名
  - 单元测试
- Day 3-4: 实现修理项 3（Ledger 机制）
  - 新增 ToolResultEntry 结构
  - 实现 buildToolResultLedger
  - 修改 Snapshot 和 Build 函数
  - 单元测试和集成测试

**Phase 2: 测试和验证（1-2 天）**
- Day 5: 回归测试
  - 检查消费侧代码（planner、verifier、goalrunner）
  - 修复 breaking changes 导致的问题
- Day 6: 集成测试和烟测
  - 使用实际 CCB 数据验证
  - 确认上下文体积降到 10k 以下

### 8.2 风险点和应对

**风险 1：Breaking Change 影响消费侧**
- 影响：删除 ToolResultSummary 可能导致其他代码访问失败
- 应对：
  1. 全局搜索 `ToolResultSummary` 的引用
  2. 替换为 `ToolResultLedger` 访问
  3. 提供兼容性辅助函数（如果需要）

**风险 2：投影模型没有 LLMContext**
- 影响：某些投影模型可能还没有实现 LLMContext
- 应对：
  1. extractLLMContext 会返回 nil，回退到通用压缩
  2. 不影响功能，只是优化程度不同

**风险 3：上下文引用机制 LLM 不理解**
- 影响：LLM 可能不会通过 tool_call_id 查找 ledger
- 应对：
  1. 在 system prompt 中清晰说明
  2. 如果问题严重，考虑在 CurrentDecisionContext 中内联最新结果

### 8.3 回滚策略

如果修复后出现严重问题，提供快速回滚方案：

**回滚步骤：**
1. 恢复 `Snapshot.ToolResultSummary` 字段
2. 恢复 `summarizeTrace` 的原始实现（返回 toolResultSummary）
3. 恢复 `compactTraceEvent` 中的完整 tool_result
4. 恢复 `summarizeDAWSemantic` 的 recent_execution_result
5. 恢复 `summarizeRecentGoalContext` 的 recent_tool_result

**Git 标签：**
- 修复前：`git tag contextruntime-fix-before`
- 修复后：`git tag contextruntime-fix-after`

---

## 九、预期效果

### 9.1 体积对比

| 场景 | 修复前 | 修复后 | 压缩比 |
|------|--------|--------|--------|
| 初始请求 | 7k | 7k | 100% |
| CCB catalog (15.5k) | 64k | ~8k | **88% ↓** |
| CCB bundle (35k) | 140k+ | ~12k | **91% ↓** |
| 多个投影模型 (50k) | 200k+ | ~15k | **92% ↓** |

### 9.2 具体改进

**修复前的结构：**
```json
{
  "tool_result_summary": [
    { "tool_call_id": "call_123", "result_preview": { /* 35 KB */ } }
  ],
  "goal_trace_summary": {
    "recent_events": [
      { "tool_result": { "tool_call_id": "call_123", "result_preview": { /* 35 KB */ } } }
    ]
  },
  "daw_semantic_summary": {
    "recent_execution_result": { "tool_call_id": "call_123", "result_preview": { /* 35 KB */ } }
  },
  "recent_goal_context": {
    "recent_tool_result": { "tool_call_id": "call_123", "result_preview": { /* 35 KB */ } }
  }
}
```
**总体积：140 KB（35 KB × 4）**

**修复后的结构：**
```json
{
  "tool_result_ledger": {
    "call_123": {
      "tool": "ccb.observation_request",
      "bundle_type": "ccb_observation_bundle",
      "digest": "CCB Bundle 包含 2 个观察视角: COM ready projection; DOM ready projection",
      "view_contexts": {
        "com_source_dynamics": { "summary_md": "...", "compact_facts": [...] },  // 2 KB
        "dom_paired_io": { "summary_md": "...", "compact_facts": [...] }         // 1.5 KB
      }
    }
  },
  "goal_trace_summary": {
    "recent_events": [
      { "tool_result_ref": { "tool_call_id": "call_123", "tool": "ccb.observation_request", "status": "ok" } }
    ]
  },
  "recent_goal_context": {
    "recent_tool_call_id": "call_123"
  }
}
```
**总体积：~4 KB（只保留 LLMContext）**

---

## 十、后续优化建议

### 10.1 短期优化（1-2 个月内）

1. **为所有投影模型添加 LLMContext**
   - 确保 TIM, FXM, EPM, RLM 都实现 LLMContext
   - 统一 LLMContext 的格式规范

2. **优化 Digest 生成策略**
   - 为不同类型的工具定制 digest 生成
   - 考虑使用模板或规则引擎

3. **监控上下文体积**
   - 在日志中记录每次装配的体积
   - 设置告警阈值（如超过 20k）

### 10.2 中期优化（2-3 个月内）

如果混音能力继续扩展，考虑：

1. **引入 Hot/Warm/Cold 分层**
   - Hot：当前决策必需（< 10k）
   - Warm：最近几轮的 digest（< 5k）
   - Cold：历史数据（按需加载）

2. **Observation 外部化存储**
   - 超大体积的 observation bundle 写入文件
   - Ledger 中只保留文件路径

3. **上下文预算管理**
   - 设置总预算（如 50k tokens）
   - 自动降级策略

### 10.3 长期优化（3+ 个月）

1. **观察视角管理器**
   - 注册机制
   - 按需激活
   - 依赖管理

2. **Context View 机制**
   - Planning View（规划阶段需要的上下文）
   - Execution View（执行阶段需要的上下文）
   - Verification View（验证阶段需要的上下文）

3. **升级为 ContextManager**
   - 显式的上下文生命周期管理
   - 增量更新机制
   - 上下文老化策略

---

## 十一、附录

### 11.1 关键文件清单

| 文件路径 | 修改内容 | 影响范围 |
|---------|---------|---------|
| `agent/internal/contextruntime/context.go` | 主要修改 | 高 |
| `agent/internal/planner/*.go` | 可能需要适配 | 中 |
| `agent/internal/goalrunner/*.go` | 可能需要适配 | 中 |
| System Prompt 文件 | 增加引用机制说明 | 中 |

### 11.2 相关资源

- **投影模型文档**：
  - COM: `agent/internal/com/`
  - DOM: `agent/internal/dom/`
  - MOM: `agent/internal/mom/`
  - TOM: `agent/internal/tom/`

- **CCB 文档**：
  - `agent/internal/capabilitycontext/free_state_observation.go`

- **测试数据**：
  - 使用当前烟测中的 CCB catalog/bundle 数据

### 11.3 参考链接

- 诊断分析文档：`context_duplication_analysis.md`
- 架构讨论记录：本次对话

---

## 十二、检查清单

### 开发前检查
- [ ] 阅读完整开发文档
- [ ] 理解重复装配的 4 个路径
- [ ] 理解 CCB Bundle 的复合结构
- [ ] 理解投影模型的 LLMContext 设计
- [ ] 创建 feature 分支：`git checkout -b fix/contextruntime-duplication`

### 开发中检查
- [ ] 修理项 1 完成，单元测试通过
- [ ] 修理项 2 完成，单元测试通过
- [ ] 修理项 3 完成，单元测试通过
- [ ] 所有新函数都有单元测试
- [ ] 代码符合项目规范（gofmt, golint）

### 开发后检查
- [ ] 回归测试全部通过
- [ ] 集成测试通过
- [ ] 使用实际 CCB 数据验证
- [ ] 上下文体积 < 10k（目标达成）
- [ ] 消费侧代码适配完成
- [ ] System Prompt 更新完成
- [ ] 文档更新完成

### 发布前检查
- [ ] Code Review 完成
- [ ] 性能测试通过
- [ ] 没有内存泄漏
- [ ] 日志记录完整
- [ ] 回滚方案准备好
- [ ] 合并到 main 分支

---

## 结束语

本次修复的核心思想是：**尊重投影模型的自描述能力，消除重复装配，引入轻量引用机制。**

关键收获：
1. 投影模型（COM, DOM, MOM, TOM）的设计是正确的，它们已经提供了 LLMContext
2. CCB 作为"手推车"的职责是明确的，不需要改动
3. 问题出在 contextruntime 没有正确"拆包" CCB Bundle
4. 解决方案不是重构，而是修理：识别 + 提取 + 去重

**预期效果：上下文体积从 60k+ 降至 10k 以下，压缩 80-90%。**

祝开发顺利！

---

**文档版本**: v1.0  
**创建日期**: 2026-08-11  
**作者**: Kiro AI Assistant  
**审核状态**: 待审核
