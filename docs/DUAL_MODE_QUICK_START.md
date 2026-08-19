# Dual-Mode Architecture - Quick Start Guide

**目标读者：** 开发团队  
**预计时间：** 周一上午团队会议（1小时）+ 周一下午开始编码  
**文档版本：** 1.0  
**日期：** 2026-08-15

---

## 📋 周一团队会议议程（1小时）

### 第一部分：问题对齐（15分钟）

**讨论要点：**
1. 当前问题：证据在 HTTP 边界被压缩，导致 LLM 无法进入执行层
2. 根本原因：两个模式（自由对话 + 固定编排）在同一个 context 上竞争
3. 为什么现在修 bug 会陷入循环：改 A 破 B，改 B 破 A

**决策点：**
- ✅ 确认问题诊断正确
- ✅ 同意需要架构重构，而不是继续打补丁

### 第二部分：新架构讲解（20分钟）

**核心概念：**
```
Evidence Store (不可变事实数据库)
     ↓
Mode Router (根据任务选择模式)
     ↓
Context Projector (为每个模式生成定制视图)
     ↓
┌─────────────┐        ┌──────────────┐
│ 自由对话模式 │        │ 固定编排模式 │
│ (丰富上下文) │        │ (精简参数)   │
└─────────────┘        └──────────────┘
```

**关键决策：**
- Evidence Store 只增不改（append-only）
- 两个模式共享底层证据，但维护各自的工作状态
- 每个模式看到定制的投影，互不干扰

**白板绘图：**
```
┌──────────────────────────────────────┐
│  Conversation Evidence (Server 持久化) │
├──────────────────────────────────────┤
│                                       │
│  Observations: {                      │
│    "obs_001": { /* 完整观察 */ }      │
│    "obs_002": { /* 完整观察 */ }      │
│  }                                    │
│                                       │
│  Decisions: {                         │
│    "dec_001": { /* 完整决策 */ }      │
│  }                                    │
│                                       │
│  ActiveMode: "orchestrated_mix"       │
│                                       │
│  ModeStates: {                        │
│    "free_dialogue": {                 │
│      RecentObservations: [...]        │
│      CandidatePool: [...]             │
│    },                                 │
│    "orchestrated_mix": {              │
│      HypothesisFrontier: [...]        │
│      CurrentPhase: "..."              │
│    }                                  │
│  }                                    │
└──────────────────────────────────────┘
```

### 第三部分：实施计划（15分钟）

**4周时间表：**
- Week 1: Evidence Store + 集成到现有代码
- Week 2: Mode Router + Context Projector
- Week 3: 修复证据完整性问题（刚才讨论的 Priority 1-3）
- Week 4: 控制逻辑 + 验收标准收紧

**本周目标（Week 1）：**
- Day 1-2: 设计数据结构 + 代码审查
- Day 3-4: 实现 Evidence Store
- Day 5: 集成到 `recordFreeStateDecision`

### 第四部分：任务分配（10分钟）

**角色分工：**
- **架构负责人**：Review 数据结构设计，把控整体方向
- **后端开发 A**：实现 Evidence Store 核心逻辑
- **后端开发 B**：编写单元测试
- **前端/集成**：准备集成点，确保向后兼容

**今天（周一下午）要完成：**
- [ ] 所有人 Read `DUAL_MODE_ARCHITECTURE_V2.md` 完整文档
- [ ] 在白板或 Miro 上画出 `ConversationEvidence` 数据结构
- [ ] 团队 Review 并达成一致
- [ ] 创建 feature branch: `feat/dual-mode-architecture-v2`

---

## 🚀 Day 1-2 具体任务：设计数据结构

### 任务 1.1：创建 evidence 包（30分钟）

```bash
# 创建新包
mkdir -p D:/Vit_DAW/agent/internal/evidence
cd D:/Vit_DAW/agent/internal/evidence

# 创建基础文件
touch types.go
touch store.go
touch router.go
touch projector.go
touch frontier.go
touch progression.go
```

### 任务 1.2：定义核心类型（2小时）

**文件：`agent/internal/evidence/types.go`**

```go
package evidence

import (
    "time"
    "vit-daw-agent/internal/processorintent"
)

// ExecutionMode defines which execution path to use
type ExecutionMode string

const (
    ModeFreeDialogue    ExecutionMode = "free_dialogue"
    ModeOrchestratedMix ExecutionMode = "orchestrated_mix"
)

// ConversationEvidence is the root container for all evidence
type ConversationEvidence struct {
    ConversationID string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    
    // Immutable fact database
    Observations map[string]Observation
    Decisions    map[string]Decision
    
    // Current mode
    ActiveMode ExecutionMode
    
    // Mode-specific state
    ModeStates map[ExecutionMode]ModeState
}

// Observation is an immutable CCB observation record
type Observation struct {
    ID           string
    Timestamp    time.Time
    ToolCallID   string
    ViewIDs      []string
    Status       string
    Bundle       map[string]any
    Conclusion   *ObservationConclusion
    TargetRef    map[string]any
    Freshness    map[string]any
    Limitations  []string
    EvidenceRefs []string
}

// ObservationConclusion is extracted structured data
type ObservationConclusion struct {
    Candidates      []Candidate
    Coverage        map[string]any
    IssueFindings   []IssueFinding
    Measurements    []Measurement
    Confidence      float64
    Actionability   string
}

// Decision is an immutable free-state decision record
type Decision struct {
    ID                      string
    Timestamp               time.Time
    Cycle                   int
    Status                  string
    Summary                 string
    RemainingIntent         string
    ProcessorType           string
    SemanticProcessorIntent *processorintent.Intent
    RequestedViewIDs        []string
    ObservationID           string
    EvidenceRefs            []string
    Limitations             []string
}

// ModeState interface for mode-specific working state
type ModeState interface {
    GetMode() ExecutionMode
}

// FreeDialogueState maintains conversation memory
type FreeDialogueState struct {
    Mode                 ExecutionMode
    RecentObservations   []string  // observation IDs
    ReasoningTrace       []string  // decision IDs
    CandidatePool        []Candidate
    LastUserIntent       string
    ExplorationDepth     int
}

func (s *FreeDialogueState) GetMode() ExecutionMode {
    return ModeFreeDialogue
}

// OrchestratedMixState maintains workflow execution state
type OrchestratedMixState struct {
    Mode               ExecutionMode
    CurrentPhase       string
    HypothesisFrontier []Hypothesis
    ActionHistory      []ActionRecord
    MaxActions         int
    MaxCycles          int
    ProgressMetrics    ProgressMetrics
}

func (s *OrchestratedMixState) GetMode() ExecutionMode {
    return ModeOrchestratedMix
}

// Candidate represents a treatment target
type Candidate struct {
    CandidateID  string
    TrackID      string
    TrackName    string
    IssueType    string
    Confidence   float64
    SourceObsID  string
    EvidenceRefs []string
    Details      map[string]any
}

// Hypothesis is an actionable candidate
type Hypothesis struct {
    HypothesisID     string
    Candidate        Candidate
    ProcessorFamily  string
    RequiredCoverage []string
    Confidence       float64
    Status           string
}

// ActionRecord tracks executed actions
type ActionRecord struct {
    Cycle         int
    ProcessorType string
    Workflow      string
    Status        string
    Summary       string
    Receipt       map[string]any
    RecordedAt    time.Time
}

// ProgressMetrics tracks orchestrated mode progress
type ProgressMetrics struct {
    CandidateCount      int
    HypothesisCount     int
    ActionableCount     int
    AverageConfidence   float64
    PhaseProgressOrder  int
}

// IssueFinding for diagnostic views
type IssueFinding struct {
    IssueType    string
    Severity     float64
    Description  string
    EvidenceRefs []string
}

// Measurement for spectral/dynamic views
type Measurement struct {
    Metric       string
    Value        float64
    Unit         string
    EvidenceRefs []string
}
```

**验收标准：**
- [ ] 所有类型定义清晰
- [ ] 团队 code review 通过
- [ ] 没有循环依赖

### 任务 1.3：白板设计评审（1小时）

**在白板上画出：**
1. `ConversationEvidence` 的完整结构
2. `Observation` 如何从 CCB bundle 提取
3. `FreeDialogueState` vs `OrchestratedMixState` 的区别
4. 数据流向：观察 → Evidence Store → 投影 → 模式

**评审检查点：**
- [ ] 所有字段都有明确用途
- [ ] 不可变部分和可变部分区分清楚
- [ ] 两个模式的状态互不干扰
- [ ] 可以支持未来扩展（新模式、新视图）

---

## 📝 Day 3-4 具体任务：实现 Evidence Store

### 任务 2.1：实现基础存储（3小时）

**文件：`agent/internal/evidence/store.go`**

```go
package evidence

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// Store interface defines evidence storage operations
type Store interface {
    RecordObservation(ctx context.Context, obs Observation) error
    RecordDecision(ctx context.Context, dec Decision) error
    GetEvidence(conversationID string) (*ConversationEvidence, error)
    UpdateModeState(conversationID string, mode ExecutionMode, state ModeState) error
}

// InMemoryStore is a simple in-memory implementation
type InMemoryStore struct {
    conversations map[string]*ConversationEvidence
    mu            sync.RWMutex
}

// NewInMemoryStore creates a new in-memory evidence store
func NewInMemoryStore() *InMemoryStore {
    return &InMemoryStore{
        conversations: make(map[string]*ConversationEvidence),
    }
}

// RecordObservation adds an observation (append-only)
func (s *InMemoryStore) RecordObservation(ctx context.Context, obs Observation) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if obs.ID == "" {
        return fmt.Errorf("observation ID is required")
    }
    
    // Get or create conversation evidence
    evidence := s.getOrCreateLocked(obs.ConversationID)
    
    // Check for duplicates
    if _, exists := evidence.Observations[obs.ID]; exists {
        return fmt.Errorf("observation %s already exists", obs.ID)
    }
    
    // Append observation
    evidence.Observations[obs.ID] = obs
    evidence.UpdatedAt = time.Now()
    
    return nil
}

// RecordDecision adds a decision (append-only)
func (s *InMemoryStore) RecordDecision(ctx context.Context, dec Decision) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if dec.ID == "" {
        return fmt.Errorf("decision ID is required")
    }
    
    evidence := s.getOrCreateLocked(dec.ConversationID)
    
    if _, exists := evidence.Decisions[dec.ID]; exists {
        return fmt.Errorf("decision %s already exists", dec.ID)
    }
    
    evidence.Decisions[dec.ID] = dec
    evidence.UpdatedAt = time.Now()
    
    return nil
}

// GetEvidence retrieves evidence for a conversation (read-only snapshot)
func (s *InMemoryStore) GetEvidence(conversationID string) (*ConversationEvidence, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    evidence, exists := s.conversations[conversationID]
    if !exists {
        return nil, fmt.Errorf("conversation %s not found", conversationID)
    }
    
    // Return a deep copy to ensure immutability
    return cloneEvidence(evidence), nil
}

// UpdateModeState updates the working state for a mode
func (s *InMemoryStore) UpdateModeState(conversationID string, mode ExecutionMode, state ModeState) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    evidence := s.getOrCreateLocked(conversationID)
    evidence.ModeStates[mode] = state
    evidence.UpdatedAt = time.Now()
    
    return nil
}

// getOrCreateLocked gets or creates evidence (caller must hold lock)
func (s *InMemoryStore) getOrCreateLocked(conversationID string) *ConversationEvidence {
    evidence, exists := s.conversations[conversationID]
    if !exists {
        now := time.Now()
        evidence = &ConversationEvidence{
            ConversationID: conversationID,
            CreatedAt:      now,
            UpdatedAt:      now,
            Observations:   make(map[string]Observation),
            Decisions:      make(map[string]Decision),
            ActiveMode:     ModeFreeDialogue,  // Default mode
            ModeStates: map[ExecutionMode]ModeState{
                ModeFreeDialogue: &FreeDialogueState{
                    Mode:               ModeFreeDialogue,
                    RecentObservations: []string{},
                    ReasoningTrace:     []string{},
                    CandidatePool:      []Candidate{},
                },
                ModeOrchestratedMix: &OrchestratedMixState{
                    Mode:               ModeOrchestratedMix,
                    CurrentPhase:       "processor_selection",
                    HypothesisFrontier: []Hypothesis{},
                    ActionHistory:      []ActionRecord{},
                    MaxActions:         6,
                    MaxCycles:          6,
                },
            },
        }
        s.conversations[conversationID] = evidence
    }
    return evidence
}

// cloneEvidence creates a deep copy
func cloneEvidence(src *ConversationEvidence) *ConversationEvidence {
    // TODO: Implement deep copy
    // For now, return the original (acceptable for Phase 1)
    return src
}
```

**验收标准：**
- [ ] 所有接口方法实现
- [ ] 并发安全（mutex 保护）
- [ ] append-only 语义（不允许覆盖）

### 任务 2.2：编写单元测试（2小时）

**文件：`agent/internal/evidence/store_test.go`**

```go
package evidence

import (
    "context"
    "testing"
    "time"
)

func TestInMemoryStore_RecordObservation(t *testing.T) {
    store := NewInMemoryStore()
    ctx := context.Background()
    
    obs := Observation{
        ID:           "obs_001",
        ConversationID: "conv_001",
        Timestamp:    time.Now(),
        ViewIDs:      []string{"track.frequency"},
        Status:       "ready",
        Bundle:       map[string]any{"test": "data"},
    }
    
    // First write should succeed
    err := store.RecordObservation(ctx, obs)
    if err != nil {
        t.Fatalf("RecordObservation failed: %v", err)
    }
    
    // Duplicate write should fail
    err = store.RecordObservation(ctx, obs)
    if err == nil {
        t.Fatal("Expected error for duplicate observation")
    }
    
    // Verify stored
    evidence, err := store.GetEvidence("conv_001")
    if err != nil {
        t.Fatalf("GetEvidence failed: %v", err)
    }
    
    if len(evidence.Observations) != 1 {
        t.Fatalf("Expected 1 observation, got %d", len(evidence.Observations))
    }
    
    stored := evidence.Observations["obs_001"]
    if stored.ID != obs.ID {
        t.Errorf("Expected ID %s, got %s", obs.ID, stored.ID)
    }
}

func TestInMemoryStore_Concurrency(t *testing.T) {
    store := NewInMemoryStore()
    ctx := context.Background()
    
    // Write 100 observations concurrently
    done := make(chan bool, 100)
    for i := 0; i < 100; i++ {
        go func(idx int) {
            obs := Observation{
                ID:           fmt.Sprintf("obs_%03d", idx),
                ConversationID: "conv_001",
                Timestamp:    time.Now(),
                Status:       "ready",
            }
            store.RecordObservation(ctx, obs)
            done <- true
        }(i)
    }
    
    // Wait for all
    for i := 0; i < 100; i++ {
        <-done
    }
    
    // Verify all stored
    evidence, _ := store.GetEvidence("conv_001")
    if len(evidence.Observations) != 100 {
        t.Errorf("Expected 100 observations, got %d", len(evidence.Observations))
    }
}

func TestInMemoryStore_ModeStateIsolation(t *testing.T) {
    store := NewInMemoryStore()
    
    // Create evidence
    store.getOrCreateLocked("conv_001")
    
    // Update free dialogue state
    freeState := &FreeDialogueState{
        Mode: ModeFreeDialogue,
        RecentObservations: []string{"obs_001"},
    }
    store.UpdateModeState("conv_001", ModeFreeDialogue, freeState)
    
    // Update orchestrated state
    orchState := &OrchestratedMixState{
        Mode: ModeOrchestratedMix,
        CurrentPhase: "materialization",
    }
    store.UpdateModeState("conv_001", ModeOrchestratedMix, orchState)
    
    // Verify isolation
    evidence, _ := store.GetEvidence("conv_001")
    
    free := evidence.ModeStates[ModeFreeDialogue].(*FreeDialogueState)
    if len(free.RecentObservations) != 1 {
        t.Error("Free dialogue state was affected")
    }
    
    orch := evidence.ModeStates[ModeOrchestratedMix].(*OrchestratedMixState)
    if orch.CurrentPhase != "materialization" {
        t.Error("Orchestrated state was affected")
    }
}
```

**验收标准：**
- [ ] 所有测试通过
- [ ] 并发测试无 race condition
- [ ] 模式隔离验证通过

---

## 🔌 Day 5 具体任务：集成到现有代码

### 任务 3.1：在 Server 添加 Evidence Store（30分钟）

**文件：`agent/internal/chat/server.go`**

```go
type Server struct {
    // ... 现有字段
    
    // NEW: Evidence Store
    evidenceStore *evidence.InMemoryStore
}

// 在 NewServer 或初始化函数中
func (s *Server) initEvidenceStore() {
    s.evidenceStore = evidence.NewInMemoryStore()
}
```

### 任务 3.2：集成到观察记录点（2小时）

**文件：`agent/internal/chat/free_state_reasoning_loop.go`**

```go
func (s *Server) recordFreeStateDecision(conversationID string, res agentloop.Result) (freeStateReasoningLoop, bool) {
    // ... 现有逻辑
    
    // NEW: Record observation to Evidence Store
    if latestObs := freeStateCCBObservation(res); latestObs != nil {
        obs := evidence.Observation{
            ID:             firstStringFromMap(latestObs.Summary, "observation_id"),
            ConversationID: conversationID,
            Timestamp:      time.Now(),
            ToolCallID:     latestObs.ToolCallID,
            ViewIDs:        messageLoopStringList(latestObs.Summary["requested_views"]),
            Status:         firstStringFromMap(latestObs.Summary, "status"),
            Bundle:         cloneMap(latestObs.Summary),
            Conclusion:     extractObservationConclusion(latestObs.Summary),
            TargetRef:      messageLoopMapValue(latestObs.Summary["target_ref"]),
            Limitations:    messageLoopStringList(latestObs.Summary["limitations"]),
            EvidenceRefs:   messageLoopStringList(latestObs.Summary["evidence_refs"]),
        }
        
        if err := s.evidenceStore.RecordObservation(context.Background(), obs); err != nil {
            s.logger.Error("Failed to record observation: %v", err)
        } else {
            s.logger.Info("Recorded observation %s to evidence store", obs.ID)
        }
    }
    
    // NEW: Record decision to Evidence Store
    if res.FreeStateDecision != nil {
        dec := evidence.Decision{
            ID:                      fmt.Sprintf("dec_%d_%s", loop.Cycle, conversationID[:8]),
            ConversationID:          conversationID,
            Timestamp:               time.Now(),
            Cycle:                   loop.Cycle,
            Status:                  res.FreeStateDecision.Status,
            Summary:                 res.FreeStateDecision.Summary,
            RemainingIntent:         res.FreeStateDecision.RemainingIntent,
            ProcessorType:           res.FreeStateDecision.ProcessorType,
            SemanticProcessorIntent: res.FreeStateDecision.SemanticProcessorIntent,
            RequestedViewIDs:        res.FreeStateDecision.RequestedViewIDs,
            ObservationID:           res.FreeStateDecision.ObservationID,
            Limitations:             res.FreeStateDecision.Limitations,
        }
        
        if err := s.evidenceStore.RecordDecision(context.Background(), dec); err != nil {
            s.logger.Error("Failed to record decision: %v", err)
        } else {
            s.logger.Info("Recorded decision %s to evidence store", dec.ID)
        }
    }
    
    // ... 现有逻辑继续
}

// Helper function to extract conclusion
func extractObservationConclusion(bundle map[string]any) *evidence.ObservationConclusion {
    // Extract candidates, coverage, etc. from CCB bundle
    // TODO: Implement based on actual bundle structure
    return nil
}
```

### 任务 3.3：验证集成（1小时）

**验证步骤：**
1. 启动 agent
2. 发送一个触发观察的请求
3. 检查日志中的 "Recorded observation ... to evidence store"
4. 使用调试器验证 `s.evidenceStore.conversations` 中有数据

**验收标准：**
- [ ] 观察被记录到 Evidence Store
- [ ] 决策被记录到 Evidence Store
- [ ] 没有崩溃或错误
- [ ] 现有功能不受影响（向后兼容）

---

## ✅ Week 1 完成检查清单

**Day 1-2:**
- [ ] `evidence/types.go` 创建并通过 code review
- [ ] 团队在白板上对齐数据结构
- [ ] 所有人理解 Evidence Store 的设计目标

**Day 3-4:**
- [ ] `evidence/store.go` 实现完成
- [ ] 所有单元测试通过
- [ ] 并发测试无 race condition

**Day 5:**
- [ ] Evidence Store 集成到 `chat.Server`
- [ ] 观察和决策被正确记录
- [ ] 日志中可以看到记录成功的消息

**Week 1 Demo（周五下午）：**
- [ ] 展示 Evidence Store 数据结构
- [ ] 展示实际运行中的证据记录
- [ ] 回答团队疑问

---

## 🚨 常见问题

**Q1: Evidence Store 会不会消耗太多内存？**
A: Phase 1 是 in-memory，单个对话通常 < 10MB。如果担心，可以在 Phase 2 加持久化。

**Q2: 如果中途崩溃，Evidence Store 的数据会丢失吗？**
A: Phase 1 会丢失。这是已知限制。Phase 2 会加 SQLite 持久化。

**Q3: 现有代码会受影响吗？**
A: Week 1 只是"旁路记录"，不影响现有流程。现有代码继续用原来的 context 传递。

**Q4: 如果 Week 1 发现设计问题怎么办？**
A: 立即停下来，团队讨论，调整设计。不要带着问题进入 Week 2。

**Q5: Week 1 结束后还不能执行怎么办？**
A: 正常！Week 1 只是建立数据层。Week 2 才加路由和投影。Week 3 才能真正执行。

---

## 📚 参考资料

**必读文档：**
- `DUAL_MODE_ARCHITECTURE_V2.md` — 完整架构设计（本文档的详细版）
- 第 3 节：数据模型
- 第 6 节：实施计划 Phase 1

**代码参考：**
- `agent/internal/agentloop/free_state_reasoning.go:816` — 现有的观察处理
- `agent/internal/chat/free_state_reasoning_loop.go:225` — 现有的 ledger 写入（我们要替换的）

**下周预告：**
- Week 2: Mode Router + Context Projector
- Week 2 结束后，两个模式应该能正确路由并看到各自的上下文

---

**祝开发顺利！有问题随时沟通。**
