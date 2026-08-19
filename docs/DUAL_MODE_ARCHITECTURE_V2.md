# Dual-Mode Architecture V2: Free Dialogue + Orchestrated Mix

**Document Status:** Active  
**Version:** 2.0  
**Created:** 2026-08-15  
**Authors:** Architecture Review Team  
**Supersedes:** None (New architectural direction)

---

## Executive Summary

This document defines the architectural redesign required to support two execution modes in the Vit DAW Agent:

1. **Free Dialogue Mode** — Flexible, exploratory LLM-driven interactions for creative tasks
2. **Orchestrated Mix Mode** — Efficient, deterministic workflow orchestration for standard multi-track mixing

**Current Problem:** The two modes compete for the same context data structure, causing evidence loss and execution failures.

**Solution:** Introduce an Evidence Store layer that maintains all observations as immutable facts, then project mode-specific views for each execution path.

**Timeline:** 4 weeks total (2 weeks architecture, 2 weeks refinement)

---

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Core Components](#2-core-components)
3. [Data Model](#3-data-model)
4. [Mode Routing Logic](#4-mode-routing-logic)
5. [Context Projection Strategy](#5-context-projection-strategy)
6. [Implementation Plan](#6-implementation-plan)
7. [Migration Strategy](#7-migration-strategy)
8. [Acceptance Criteria](#8-acceptance-criteria)
9. [Appendix](#9-appendix)

---

## 1. Architecture Overview

### 1.1 Current Architecture (Problem)

```
┌─────────────────────────────────────┐
│   Free-State Reasoning Loop (LLM)   │
│    - Observation → Decision          │
└──────────────┬──────────────────────┘
               │
               ▼
    ┌──────────────────────┐
    │  Shared Context Map  │ ◄── Both modes read/write here
    │  (Passed by value)   │     Evidence gets overwritten!
    └──────────┬───────────┘
               │
      ┌────────┴────────┐
      ▼                 ▼
┌──────────┐    ┌────────────────┐
│ Mode A:  │    │ Mode B:        │
│ Free     │    │ Orchestrated   │
│ Dialogue │    │ Mix            │
└──────────┘    └────────────────┘
```

**Issues:**
- Evidence from observations gets compressed when passed between HTTP boundaries
- `ledger["available_views"][viewID]` overwrites rich conclusions with thin rows
- Mode A needs full reasoning chains; Mode B needs compact parameters
- Both modes mutate the same `map[string]any`, causing conflicts

### 1.2 Target Architecture (Solution)

```
┌─────────────────────────────────────┐
│   Free-State Reasoning Loop (LLM)   │
│    - Observation → Decision          │
└──────────────┬──────────────────────┘
               │
               ▼
    ┌──────────────────────────────┐
    │   Evidence Store (Server)     │
    │   - Immutable fact database   │
    │   - Monotonic accumulation    │
    │   - Never deletes evidence    │
    └──────────┬───────────────────┘
               │
        ┌──────┴───────┐
        │ Mode Router  │
        └──────┬───────┘
               │
      ┌────────┴────────┐
      ▼                 ▼
┌─────────────┐   ┌──────────────────┐
│ Mode A:     │   │ Mode B:          │
│ Free        │   │ Orchestrated     │
│ Dialogue    │   │ Mix              │
│             │   │                  │
│ Projection: │   │ Projection:      │
│ - Full obs  │   │ - Compact params │
│ - Reasoning │   │ - Frontier only  │
│   trace     │   │ - Phase state    │
└─────────────┘   └──────────────────┘
```

**Key Improvements:**
- **Evidence Store** maintains all facts (observations, decisions) as immutable records
- **Mode Router** selects execution path based on task characteristics
- **Projection Layer** generates mode-specific contexts from the evidence store
- Each mode gets exactly the data it needs, in the format it expects

---

## 2. Core Components

### 2.1 Evidence Store

**Responsibility:** Persistent, immutable storage of all conversation evidence.

**Key Properties:**
- **Monotonic:** Can only add, never delete or overwrite
- **Versioned:** Each observation/decision has a unique ID and timestamp
- **Queryable:** Can retrieve evidence by ID, time range, or type

**Interface:**

```go
type EvidenceStore interface {
    // Write operations (append-only)
    RecordObservation(ctx context.Context, obs Observation) error
    RecordDecision(ctx context.Context, dec Decision) error
    
    // Read operations (return immutable snapshots)
    GetEvidence(conversationID string) (*ConversationEvidence, error)
    QueryObservations(conversationID string, filter ObservationFilter) ([]Observation, error)
    
    // Mode state operations
    GetModeState(conversationID string, mode ExecutionMode) (ModeState, error)
    UpdateModeState(conversationID string, mode ExecutionMode, state ModeState) error
}
```

### 2.2 Mode Router

**Responsibility:** Determine which execution mode to use based on task characteristics.

**Routing Decision Factors:**
1. **Task Scope:** Single-track vs. multi-track (≥4 tracks)
2. **Task Type:** Standard mixing vs. creative/exploratory
3. **User Preference:** Explicit mode selection if provided
4. **Current Context:** Available observations and their coverage

**Interface:**

```go
type ModeRouter interface {
    SelectMode(
        decision *agentloop.FreeStateDecision,
        evidence *ConversationEvidence,
        context map[string]any,
    ) (mode ExecutionMode, reason string, err error)
}

type ExecutionMode string

const (
    ModeFreeDialogue    ExecutionMode = "free_dialogue"
    ModeOrchestratedMix ExecutionMode = "orchestrated_mix"
)
```

### 2.3 Context Projector

**Responsibility:** Generate mode-specific context from evidence store.

**Key Principle:** Each mode sees a tailored view of the same underlying evidence.

**Interface:**

```go
type ContextProjector interface {
    // For Free Dialogue: Rich context with full reasoning chains
    ProjectFreeDialogueContext(
        evidence *ConversationEvidence,
        baseContext map[string]any,
    ) (map[string]any, error)
    
    // For Orchestrated Mix: Compact parameters for deterministic execution
    ProjectOrchestratedMixContext(
        evidence *ConversationEvidence,
        baseContext map[string]any,
    ) (map[string]any, error)
}
```

---

## 3. Data Model

### 3.1 Evidence Store Schema

```go
// ConversationEvidence is the root container for all evidence in a conversation
type ConversationEvidence struct {
    ConversationID string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    
    // Immutable fact database (append-only)
    Observations map[string]Observation  // key = observation_id
    Decisions    map[string]Decision     // key = decision_id
    
    // Current active mode
    ActiveMode ExecutionMode
    
    // Mode-specific working state (mutable)
    ModeStates map[ExecutionMode]ModeState
}

// Observation is an immutable record of a CCB observation result
type Observation struct {
    ID              string                 `json:"observation_id"`
    Timestamp       time.Time              `json:"timestamp"`
    ToolCallID      string                 `json:"tool_call_id"`
    ViewIDs         []string               `json:"view_ids"`
    Status          string                 `json:"status"`
    
    // Complete observation bundle (never compressed)
    Bundle          map[string]any         `json:"bundle"`
    
    // Extracted structured conclusion (for fast access)
    Conclusion      *ObservationConclusion `json:"conclusion,omitempty"`
    
    // Metadata
    TargetRef       map[string]any         `json:"target_ref,omitempty"`
    Freshness       map[string]any         `json:"freshness,omitempty"`
    Limitations     []string               `json:"limitations,omitempty"`
    EvidenceRefs    []string               `json:"evidence_refs,omitempty"`
}

// ObservationConclusion is the structured digest extracted from observation bundle
type ObservationConclusion struct {
    // For multi-track relationship views
    Candidates      []RelationshipCandidate `json:"candidates,omitempty"`
    Coverage        map[string]any          `json:"coverage,omitempty"`
    
    // For track-level diagnostic views
    IssueFindings   []IssueFinding          `json:"issue_findings,omitempty"`
    
    // For spectral/dynamic views
    Measurements    []Measurement           `json:"measurements,omitempty"`
    
    // Confidence and actionability
    Confidence      float64                 `json:"confidence"`
    Actionability   string                  `json:"actionability"` // "direct" | "requires_target" | "diagnostic_only"
}

// Decision is an immutable record of a Free-State decision
type Decision struct {
    ID                      string                       `json:"decision_id"`
    Timestamp               time.Time                    `json:"timestamp"`
    Cycle                   int                          `json:"cycle"`
    Status                  string                       `json:"status"` // needs_observation | needs_action | satisfied | blocked
    
    // Decision content (never modified)
    Summary                 string                       `json:"summary"`
    RemainingIntent         string                       `json:"remaining_intent,omitempty"`
    ProcessorType           string                       `json:"processor_type,omitempty"`
    SemanticProcessorIntent *processorintent.Intent      `json:"semantic_processor_intent,omitempty"`
    RequestedViewIDs        []string                     `json:"requested_view_ids,omitempty"`
    ObservationID           string                       `json:"observation_id,omitempty"`
    
    // Context at decision time (for audit)
    EvidenceRefs            []string                     `json:"evidence_refs,omitempty"`
    Limitations             []string                     `json:"limitations,omitempty"`
}

// ModeState is the mutable working state for each execution mode
type ModeState interface {
    GetMode() ExecutionMode
}

// FreeDialogueState maintains conversation memory for exploratory reasoning
type FreeDialogueState struct {
    Mode ExecutionMode `json:"mode"` // always "free_dialogue"
    
    // Recent observation history (for LLM context)
    RecentObservations []string `json:"recent_observations"` // observation IDs
    
    // Reasoning trace (for multi-turn coherence)
    ReasoningTrace []string `json:"reasoning_trace"` // decision IDs
    
    // Candidate pool (accumulated from observations)
    CandidatePool []Candidate `json:"candidate_pool"`
    
    // Conversation flow state
    LastUserIntent string    `json:"last_user_intent"`
    ExplorationDepth int     `json:"exploration_depth"`
}

// OrchestratedMixState maintains execution state for deterministic workflow
type OrchestratedMixState struct {
    Mode ExecutionMode `json:"mode"` // always "orchestrated_mix"
    
    // Current workflow phase
    CurrentPhase string `json:"current_phase"` // processor_selection | materialization | post_action_evaluation
    
    // Hypothesis frontier (actionable candidates)
    HypothesisFrontier []Hypothesis `json:"hypothesis_frontier"`
    
    // Execution history (for progress tracking)
    ActionHistory []ActionRecord `json:"action_history"`
    
    // Workflow constraints
    MaxActions int `json:"max_actions"`
    MaxCycles  int `json:"max_cycles"`
    
    // Progress metrics
    ProgressMetrics ProgressMetrics `json:"progress_metrics"`
}

// Candidate represents a potential treatment target identified from observations
type Candidate struct {
    CandidateID   string   `json:"candidate_id"`
    TrackID       string   `json:"track_id"`
    TrackName     string   `json:"track_name"`
    IssueType     string   `json:"issue_type"` // frequency_conflict | dynamic_imbalance | transient_issue
    Confidence    float64  `json:"confidence"`
    SourceObsID   string   `json:"source_observation_id"`
    EvidenceRefs  []string `json:"evidence_refs"`
    Details       map[string]any `json:"details,omitempty"`
}

// Hypothesis is an actionable candidate with processor family selection
type Hypothesis struct {
    HypothesisID  string   `json:"hypothesis_id"`
    Candidate     Candidate `json:"candidate"`
    ProcessorFamily string  `json:"processor_family"` // static_eq | broadband_compressor | etc.
    RequiredCoverage []string `json:"required_coverage"`
    Confidence    float64  `json:"confidence"`
    Status        string   `json:"status"` // pending | selected | executed | verified | rejected
}

// ProgressMetrics tracks meaningful progress in orchestrated mode
type ProgressMetrics struct {
    CandidateCount      int     `json:"candidate_count"`
    HypothesisCount     int     `json:"hypothesis_count"`
    ActionableCount     int     `json:"actionable_count"`
    AverageConfidence   float64 `json:"average_confidence"`
    PhaseProgressOrder  int     `json:"phase_progress_order"` // 0=selection, 1=materialization, 2=evaluation
}
```

### 3.2 Storage Implementation

**Phase 1 (MVP):** In-memory map with mutex
```go
type InMemoryEvidenceStore struct {
    conversations map[string]*ConversationEvidence
    mu            sync.RWMutex
}
```

**Phase 2 (Production):** Persistent storage
- SQLite for small deployments
- PostgreSQL for multi-user scenarios
- Redis for hot data caching

---

## 4. Mode Routing Logic

### 4.1 Routing Decision Tree

```
┌─ User Preference Explicit? ─────────────────┐
│  Yes → Use user's choice                    │
│  No  → Continue to automated selection      │
└─────────────────────────────────────────────┘
           │
           ▼
┌─ Task Scope Analysis ───────────────────────┐
│  Track Count:                                │
│  • 1-3 tracks   → Lean toward Free Dialogue  │
│  • 4+ tracks    → Lean toward Orchestrated   │
│                                              │
│  Visible Track Coverage:                     │
│  • < 50% observed → Need more observation    │
│  • ≥ 80% observed → Can orchestrate          │
└──────────────────────────────────────────────┘
           │
           ▼
┌─ Intent Classification ─────────────────────┐
│  Standard Mix Keywords:                      │
│  • "混音", "mix", "balance"                   │
│  • "adjust levels", "均衡音量"                 │
│  → Orchestrated Mix                          │
│                                              │
│  Creative/Exploratory Keywords:              │
│  • "空间感", "spatial", "creative"             │
│  • "试试", "experiment", "explore"            │
│  → Free Dialogue                             │
└──────────────────────────────────────────────┘
           │
           ▼
┌─ Semantic Entry Decision ───────────────────┐
│  If semantic_entry_decision present:         │
│  • route="open_semantic" → Continue          │
│  • route="other" → Free Dialogue (fallback)  │
│                                              │
│  control_mode="semantic_loop" → Match!       │
└──────────────────────────────────────────────┘
           │
           ▼
┌─ Final Decision ─────────────────────────────┐
│  Weighted Score:                             │
│  score = 0                                   │
│  + (trackCount >= 4) ? 2 : 0                 │
│  + (coverage >= 0.8) ? 2 : 0                 │
│  + hasStandardMixIntent ? 2 : 0              │
│  + hasMultiTrackCandidates ? 1 : 0           │
│                                              │
│  if score >= 4:                              │
│    → Orchestrated Mix                        │
│  else:                                       │
│    → Free Dialogue                           │
└──────────────────────────────────────────────┘
```

### 4.2 Implementation

```go
type DefaultModeRouter struct {
    logger *logx.Logger
}

func (r *DefaultModeRouter) SelectMode(
    decision *agentloop.FreeStateDecision,
    evidence *ConversationEvidence,
    context map[string]any,
) (ExecutionMode, string, error) {
    
    // 1. Check user preference
    if userMode := firstStringFromMap(context, "user_execution_mode", "orchestration_preference"); userMode != "" {
        switch strings.ToLower(userMode) {
        case "manual", "free", "dialogue":
            return ModeFreeDialogue, "user_preference", nil
        case "auto", "orchestrated", "workflow":
            return ModeOrchestratedMix, "user_preference", nil
        }
    }
    
    // 2. Analyze task scope
    trackCount := len(contextStringSlice(context["visible_track_ids"]))
    coverage := computeCoverageRatio(evidence, context)
    
    // 3. Classify intent
    hasStandardMixIntent := detectStandardMixIntent(decision, context)
    hasCreativeIntent := detectCreativeIntent(decision, context)
    
    // 4. Check semantic entry
    entryDecision, hasEntry := semanticEntryDecisionFromContext(context)
    if hasEntry && entryDecision.Route != "open_semantic" {
        return ModeFreeDialogue, "semantic_entry_fallback", nil
    }
    
    // 5. Compute weighted score
    score := 0
    reasons := []string{}
    
    if trackCount >= 4 {
        score += 2
        reasons = append(reasons, fmt.Sprintf("multi_track(%d)", trackCount))
    }
    
    if coverage >= 0.8 {
        score += 2
        reasons = append(reasons, fmt.Sprintf("high_coverage(%.2f)", coverage))
    }
    
    if hasStandardMixIntent {
        score += 2
        reasons = append(reasons, "standard_mix_intent")
    }
    
    if hasMultiTrackCandidates(evidence) {
        score += 1
        reasons = append(reasons, "multi_track_candidates")
    }
    
    if hasCreativeIntent {
        score -= 2
        reasons = append(reasons, "creative_intent(-2)")
    }
    
    // 6. Decision threshold
    if score >= 4 {
        reason := "orchestrated: " + strings.Join(reasons, ", ")
        return ModeOrchestratedMix, reason, nil
    }
    
    reason := "free_dialogue: " + strings.Join(reasons, ", ")
    return ModeFreeDialogue, reason, nil
}
```

---

## 5. Context Projection Strategy

### 5.1 Free Dialogue Projection

**Goal:** Provide rich context for LLM reasoning

**Included Data:**
- Full observation bundles (not just summaries)
- Complete reasoning trace (all prior decisions)
- Accumulated candidate pool
- Evidence references for citation

**Example Output:**

```go
{
    "mode": "free_dialogue",
    
    "recent_observations": [
        {
            "observation_id": "obs_001",
            "view_ids": ["track.frequency_time_events"],
            "conclusion": {
                "candidates": [...],  // Full candidate details
                "coverage": {...},
                "measurements": [...]
            },
            "bundle": {...}  // Complete CCB bundle
        },
        // ... up to last 3 observations
    ],
    
    "reasoning_trace": [
        {
            "decision_id": "dec_001",
            "cycle": 1,
            "status": "needs_observation",
            "summary": "需要观察人声轨道的频率分布",
            "requested_view_ids": ["track.frequency_time_events"]
        },
        {
            "decision_id": "dec_002",
            "cycle": 2,
            "status": "needs_observation",
            "summary": "发现人声在 3-5kHz 有突出，需要确认与伴奏的关系",
            "requested_view_ids": ["mix.multitrack_relationship"]
        }
    ],
    
    "candidate_pool": [
        {
            "candidate_id": "cand_vocals_1032",
            "track_id": "1032",
            "track_name": "vocals",
            "issue_type": "frequency_conflict",
            "confidence": 0.82,
            "source_observation_id": "obs_001",
            "details": {
                "conflict_with": "piano (1027)",
                "frequency_range": "2500-4000 Hz",
                "magnitude": "6.2 dB overlap"
            }
        }
    ],
    
    "last_user_intent": "把人声做得更清晰",
    "exploration_depth": 2
}
```

### 5.2 Orchestrated Mix Projection

**Goal:** Provide compact parameters for deterministic execution

**Included Data:**
- Current phase state
- Hypothesis frontier (actionable candidates only)
- Observation references (not full bundles)
- Progress metrics

**Example Output:**

```go
{
    "mode": "orchestrated_mix",
    
    "current_phase": "processor_selection",
    
    "hypothesis_frontier": [
        {
            "hypothesis_id": "hyp_001",
            "candidate": {
                "track_id": "1032",
                "track_name": "vocals",
                "issue_type": "frequency_conflict"
            },
            "processor_family": "static_eq",
            "required_coverage": ["frequency_targeting", "gain_control"],
            "confidence": 0.85,
            "status": "pending"
        }
    ],
    
    "action_history": [
        {
            "cycle": 1,
            "processor_type": "eq",
            "workflow": "semantic_dynamic_execution",
            "status": "applied",
            "summary": "Applied EQ to vocals (1032)"
        }
    ],
    
    "progress_metrics": {
        "candidate_count": 3,
        "hypothesis_count": 2,
        "actionable_count": 1,
        "average_confidence": 0.78,
        "phase_progress_order": 0
    },
    
    "observation_refs": [
        {
            "observation_id": "obs_001",
            "view_ids": ["mix.multitrack_relationship"],
            "status": "ready",
            "actionability": "requires_target"
        }
    ],
    
    "max_actions": 6,
    "max_cycles": 6
}
```

### 5.3 Implementation

```go
type DefaultContextProjector struct {
    logger *logx.Logger
}

func (p *DefaultContextProjector) ProjectFreeDialogueContext(
    evidence *ConversationEvidence,
    baseContext map[string]any,
) (map[string]any, error) {
    
    state, ok := evidence.ModeStates[ModeFreeDialogue].(*FreeDialogueState)
    if !ok {
        return nil, fmt.Errorf("free_dialogue state not initialized")
    }
    
    // Retrieve recent observations (full bundles)
    recentObs := []map[string]any{}
    for _, obsID := range lastN(state.RecentObservations, 3) {
        if obs, found := evidence.Observations[obsID]; found {
            recentObs = append(recentObs, map[string]any{
                "observation_id": obs.ID,
                "view_ids": obs.ViewIDs,
                "conclusion": obs.Conclusion,
                "bundle": obs.Bundle,  // Full bundle included
                "timestamp": obs.Timestamp,
            })
        }
    }
    
    // Retrieve reasoning trace
    reasoningTrace := []map[string]any{}
    for _, decID := range state.ReasoningTrace {
        if dec, found := evidence.Decisions[decID]; found {
            reasoningTrace = append(reasoningTrace, map[string]any{
                "decision_id": dec.ID,
                "cycle": dec.Cycle,
                "status": dec.Status,
                "summary": dec.Summary,
                "remaining_intent": dec.RemainingIntent,
                "processor_type": dec.ProcessorType,
                "requested_view_ids": dec.RequestedViewIDs,
            })
        }
    }
    
    // Build projection
    projection := cloneMap(baseContext)
    projection["mode"] = string(ModeFreeDialogue)
    projection["recent_observations"] = recentObs
    projection["reasoning_trace"] = reasoningTrace
    projection["candidate_pool"] = state.CandidatePool
    projection["last_user_intent"] = state.LastUserIntent
    projection["exploration_depth"] = state.ExplorationDepth
    
    return projection, nil
}

func (p *DefaultContextProjector) ProjectOrchestratedMixContext(
    evidence *ConversationEvidence,
    baseContext map[string]any,
) (map[string]any, error) {
    
    state, ok := evidence.ModeStates[ModeOrchestratedMix].(*OrchestratedMixState)
    if !ok {
        return nil, fmt.Errorf("orchestrated_mix state not initialized")
    }
    
    // Extract observation references (not full bundles)
    obsRefs := []map[string]any{}
    for _, obs := range evidence.Observations {
        obsRefs = append(obsRefs, map[string]any{
            "observation_id": obs.ID,
            "view_ids": obs.ViewIDs,
            "status": obs.Status,
            "actionability": obs.Conclusion.Actionability,
            // No bundle, no full conclusion
        })
    }
    
    // Build projection
    projection := cloneMap(baseContext)
    projection["mode"] = string(ModeOrchestratedMix)
    projection["current_phase"] = state.CurrentPhase
    projection["hypothesis_frontier"] = state.HypothesisFrontier
    projection["action_history"] = state.ActionHistory
    projection["progress_metrics"] = state.ProgressMetrics
    projection["observation_refs"] = obsRefs
    projection["max_actions"] = state.MaxActions
    projection["max_cycles"] = state.MaxCycles
    
    return projection, nil
}
```

---

## 6. Implementation Plan

### Phase 1: Foundation (Week 1)

**Goal:** Establish Evidence Store and basic mode routing

#### Day 1-2: Design Review & Data Model

**Deliverables:**
- [ ] Team review of this document
- [ ] Finalize `ConversationEvidence` schema
- [ ] Finalize `ModeState` interfaces
- [ ] Create `agent/internal/evidence/` package

**Code:**
```go
// agent/internal/evidence/types.go
package evidence

// Define all types from Section 3.1
```

#### Day 3-4: Evidence Store Implementation

**Deliverables:**
- [ ] Implement `InMemoryEvidenceStore`
- [ ] Implement read/write methods
- [ ] Add mutex protection
- [ ] Write unit tests

**Code:**
```go
// agent/internal/evidence/store.go
type InMemoryEvidenceStore struct {
    conversations map[string]*ConversationEvidence
    mu            sync.RWMutex
}

func NewInMemoryStore() *InMemoryEvidenceStore {
    return &InMemoryEvidenceStore{
        conversations: make(map[string]*ConversationEvidence),
    }
}

func (s *InMemoryEvidenceStore) RecordObservation(ctx context.Context, obs Observation) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    convID := obs.ConversationID
    evidence := s.getOrCreateLocked(convID)
    
    if _, exists := evidence.Observations[obs.ID]; exists {
        return fmt.Errorf("observation %s already exists", obs.ID)
    }
    
    evidence.Observations[obs.ID] = obs
    evidence.UpdatedAt = time.Now()
    
    return nil
}

// ... other methods
```

#### Day 5: Integration Point 1 - Record Observations

**Deliverables:**
- [ ] Integrate Evidence Store into `chat.Server`
- [ ] Update `recordFreeStateDecision` to write to Evidence Store
- [ ] Verify observations are recorded correctly

**Code:**
```go
// agent/internal/chat/server.go
type Server struct {
    // ... existing fields
    evidenceStore *evidence.InMemoryEvidenceStore
}

// agent/internal/chat/free_state_reasoning_loop.go
func (s *Server) recordFreeStateDecision(...) {
    // ... existing logic
    
    // NEW: Write to Evidence Store
    if latestObs := freeStateCCBObservation(res); latestObs != nil {
        obs := evidence.Observation{
            ID:          latestObs.Summary["observation_id"].(string),
            Timestamp:   time.Now(),
            ToolCallID:  latestObs.ToolCallID,
            ViewIDs:     latestObs.Summary["requested_views"].([]string),
            Status:      latestObs.Summary["status"].(string),
            Bundle:      latestObs.Summary,
            Conclusion:  extractConclusion(latestObs.Summary),
        }
        
        if err := s.evidenceStore.RecordObservation(ctx, obs); err != nil {
            s.logger.Error("Failed to record observation: %v", err)
        }
    }
    
    // ... rest of existing logic
}
```

### Phase 2: Mode Routing & Projection (Week 2)

#### Day 1-2: Mode Router Implementation

**Deliverables:**
- [ ] Implement `DefaultModeRouter`
- [ ] Add routing decision tree logic
- [ ] Write unit tests with various scenarios

**Code:**
```go
// agent/internal/evidence/router.go
type DefaultModeRouter struct {
    logger *logx.Logger
}

func (r *DefaultModeRouter) SelectMode(...) (ExecutionMode, string, error) {
    // Implementation from Section 4.2
}

// Helper functions
func detectStandardMixIntent(decision *agentloop.FreeStateDecision, context map[string]any) bool {
    text := strings.ToLower(decision.Summary + " " + decision.RemainingIntent)
    keywords := []string{"混音", "mix", "balance", "adjust levels", "均衡音量"}
    for _, kw := range keywords {
        if strings.Contains(text, kw) {
            return true
        }
    }
    return false
}

func detectCreativeIntent(decision *agentloop.FreeStateDecision, context map[string]any) bool {
    text := strings.ToLower(decision.Summary + " " + decision.RemainingIntent)
    keywords := []string{"空间感", "spatial", "creative", "试试", "experiment", "explore"}
    for _, kw := range keywords {
        if strings.Contains(text, kw) {
            return true
        }
    }
    return false
}
```

#### Day 3-4: Context Projector Implementation

**Deliverables:**
- [ ] Implement `DefaultContextProjector`
- [ ] Implement `ProjectFreeDialogueContext`
- [ ] Implement `ProjectOrchestratedMixContext`
- [ ] Write unit tests

**Code:**
```go
// agent/internal/evidence/projector.go
type DefaultContextProjector struct {
    logger *logx.Logger
}

func (p *DefaultContextProjector) ProjectFreeDialogueContext(...) (map[string]any, error) {
    // Implementation from Section 5.3
}

func (p *DefaultContextProjector) ProjectOrchestratedMixContext(...) (map[string]any, error) {
    // Implementation from Section 5.3
}
```

#### Day 5: Integration Point 2 - Route and Project

**Deliverables:**
- [ ] Update `goalrunner_chat.go` to use Mode Router
- [ ] Apply appropriate projection before execution
- [ ] Verify both modes get correct contexts

**Code:**
```go
// agent/internal/chat/goalrunner_chat.go
func (s *Server) runAgentLoopChat(...) {
    // ... existing setup
    
    // After detecting awaiting_action
    if strings.EqualFold(loop.Status, "awaiting_action") {
        evidence, err := s.evidenceStore.GetEvidence(conversationID)
        if err != nil {
            return errorResponse(err), true
        }
        
        // NEW: Select execution mode
        router := evidence.NewDefaultModeRouter(s.logger)
        targetMode, reason, err := router.SelectMode(res.FreeStateDecision, evidence, chatContext)
        if err != nil {
            return errorResponse(err), true
        }
        
        s.logger.Info("Selected mode: %s (reason: %s)", targetMode, reason)
        
        // NEW: Update active mode
        evidence.ActiveMode = targetMode
        s.evidenceStore.UpdateEvidence(evidence)
        
        // NEW: Project mode-specific context
        projector := evidence.NewDefaultContextProjector(s.logger)
        var projectedContext map[string]any
        
        switch targetMode {
        case evidence.ModeFreeDialogue:
            projectedContext, err = projector.ProjectFreeDialogueContext(evidence, chatContext)
            if err != nil {
                return errorResponse(err), true
            }
            return s.handleFreeDialogueTreatment(ctx, conversationID, projectedContext, res, cfg)
            
        case evidence.ModeOrchestratedMix:
            projectedContext, err = projector.ProjectOrchestratedMixContext(evidence, chatContext)
            if err != nil {
                return errorResponse(err), true
            }
            return s.routeOrdinaryAgentTreatmentStrategy(ctx, conversationID, mode, userText, projectedContext, res, cfg)
        }
    }
}
```

### Phase 3: Evidence Integrity (Week 3)

**Goal:** Fix evidence loss issues identified in original diagnosis

#### Day 1-2: Fix Ledger Merging

**Deliverables:**
- [ ] Implement monotonic ledger merge in `free_state_reasoning_loop.go`
- [ ] Read from Evidence Store instead of passed context
- [ ] Never delete `conclusion` or `decision_digest`

**Code:**
```go
// agent/internal/chat/free_state_reasoning_loop.go
func mergeLedgerRow(existing, new map[string]any) map[string]any {
    merged := cloneMap(existing)
    
    // Base fields always update
    for _, key := range []string{"status", "observation_id", "freshness", "limitations"} {
        if new[key] != nil {
            merged[key] = new[key]
        }
    }
    
    // Conclusion is monotonic (never delete, only enrich)
    if existingConclusion := existing["conclusion"]; existingConclusion != nil {
        merged["conclusion"] = existingConclusion
    }
    if newConclusion := new["conclusion"]; newConclusion != nil {
        merged["conclusion"] = deepMergeConclusion(
            existing["conclusion"],
            newConclusion,
        )
    }
    
    return merged
}

// Update the ledger write logic
func (s *Server) recordFreeStateCCBObservation(...) {
    // OLD: ledger["available_views"][viewID] = compactSelectedKeys(...)
    
    // NEW: Read from Evidence Store
    evidence, _ := s.evidenceStore.GetEvidence(conversationID)
    obs, found := evidence.Observations[observationID]
    if !found {
        return
    }
    
    // Merge with existing ledger entry
    existing := ledger["available_views"][viewID]
    newRow := map[string]any{
        "view_id": viewID,
        "status": obs.Status,
        "observation_id": obs.ID,
        "freshness": obs.Freshness,
        "conclusion": obs.Conclusion,  // Always include conclusion
    }
    
    ledger["available_views"][viewID] = mergeLedgerRow(existing, newRow)
}
```

#### Day 3: Fix Active Observation Projection

**Deliverables:**
- [ ] Update `model_projection.go` to NOT delete conclusion
- [ ] Read from Evidence Store for active observations
- [ ] Maintain structured candidate facts

**Code:**
```go
// agent/internal/contextruntime/model_projection.go
func projectActiveObservation(conversationID, obsID string, evidenceStore *evidence.InMemoryEvidenceStore) map[string]any {
    evidence, err := evidenceStore.GetEvidence(conversationID)
    if err != nil {
        return compactSummary(observation)
    }
    
    obs, found := evidence.Observations[obsID]
    if !found {
        return compactSummary(observation)
    }
    
    // NEW: Always include conclusion if present
    if obs.Conclusion == nil {
        return compactSummary(obs.Bundle)
    }
    
    // Extract and limit candidates for context efficiency
    candidates := obs.Conclusion.Candidates
    if len(candidates) > 5 {
        candidates = candidates[:5]  // Top 5 only
    }
    
    return map[string]any{
        "observation_id": obs.ID,
        "view_id": obs.ViewIDs,
        "status": obs.Status,
        "candidates": candidates,  // Structured facts preserved
        "coverage": obs.Conclusion.Coverage,
        "actionability": obs.Conclusion.Actionability,
        "evidence_refs": obs.EvidenceRefs,
    }
}
```

#### Day 4-5: Candidate Frontier Management

**Deliverables:**
- [ ] Implement candidate extraction from observations
- [ ] Update `OrchestratedMixState.HypothesisFrontier`
- [ ] Ensure frontier is mode-specific (doesn't pollute free dialogue)

**Code:**
```go
// agent/internal/evidence/frontier.go
func UpdateCandidateFrontier(state *OrchestratedMixState, obs Observation) {
    if obs.Conclusion == nil || len(obs.Conclusion.Candidates) == 0 {
        return
    }
    
    for _, candidate := range obs.Conclusion.Candidates {
        hypothesis := Hypothesis{
            HypothesisID: fmt.Sprintf("hyp_%s_%s", obs.ID, candidate.TrackID),
            Candidate: Candidate{
                CandidateID: candidate.CandidateID,
                TrackID: candidate.TrackID,
                TrackName: candidate.TrackName,
                IssueType: candidate.IssueType,
                Confidence: candidate.Confidence,
                SourceObsID: obs.ID,
                EvidenceRefs: obs.EvidenceRefs,
            },
            ProcessorFamily: inferProcessorFamily(candidate.IssueType),
            RequiredCoverage: inferRequiredCoverage(candidate.IssueType),
            Confidence: candidate.Confidence,
            Status: "pending",
        }
        
        state.HypothesisFrontier = appendOrMergeHypothesis(state.HypothesisFrontier, hypothesis)
    }
}

// agent/internal/chat/free_state_reasoning_loop.go
func (s *Server) recordFreeStateDecision(...) {
    // ... existing observation recording
    
    // NEW: Update mode-specific state
    evidence, _ := s.evidenceStore.GetEvidence(conversationID)
    
    if evidence.ActiveMode == evidence.ModeOrchestratedMix {
        state := evidence.ModeStates[evidence.ModeOrchestratedMix].(*OrchestratedMixState)
        UpdateCandidateFrontier(state, obs)
        s.evidenceStore.UpdateModeState(conversationID, evidence.ModeOrchestratedMix, state)
    }
}
```

### Phase 4: Control Logic & Validation (Week 4)

#### Day 1-2: Phase Progression Constraints

**Deliverables:**
- [ ] Implement phase transition validation for orchestrated mode
- [ ] Prevent invalid observation requests after candidates discovered
- [ ] Add progress assessment logic

**Code:**
```go
// agent/internal/evidence/progression.go
func ValidatePhaseTransition(state *OrchestratedMixState, decision agentloop.FreeStateDecision) error {
    // If candidates discovered, next step must be:
    // 1. Select candidate and request target observation
    // 2. Output typed insufficient settlement
    
    if len(state.HypothesisFrontier) > 0 && decision.Status == agentloop.FreeStateNeedsObservation {
        requestedViews := decision.RequestedViewIDs
        
        // Check if it's a valid target-level observation
        if !isTargetLevelObservation(requestedViews, state.HypothesisFrontier) {
            // Forbid re-requesting project.structure
            if containsView(requestedViews, "project.structure") {
                return fmt.Errorf("cannot re-request project.structure after candidates discovered; must select a candidate and request track-level observation or provide typed settlement")
            }
        }
    }
    
    return nil
}

func AssessProgress(oldState, newState *OrchestratedMixState) bool {
    oldMetrics := oldState.ProgressMetrics
    newMetrics := newState.ProgressMetrics
    
    // Progress criteria:
    // 1. Candidates appeared
    if oldMetrics.CandidateCount == 0 && newMetrics.CandidateCount > 0 {
        return true
    }
    
    // 2. Candidates narrowed
    if newMetrics.CandidateCount < oldMetrics.CandidateCount && newMetrics.CandidateCount > 0 {
        return true
    }
    
    // 3. Actionability increased
    if newMetrics.ActionableCount > oldMetrics.ActionableCount {
        return true
    }
    
    // 4. Phase progressed
    if newMetrics.PhaseProgressOrder > oldMetrics.PhaseProgressOrder {
        return true
    }
    
    // NEW observation fingerprint alone doesn't count as progress
    return false
}
```

#### Day 3: Tighten Acceptance Criteria

**Deliverables:**
- [ ] Update `semantic_processor_project_smoke_runner.py`
- [ ] Require candidate selection for minimal closure
- [ ] Split smoke tests by mode

**Code:**
```python
# scripts/semantic_processor_project_smoke_runner.py
def validate_minimal_closure_loop(run_report, expected_mode):
    """Validate that a minimal execution closure actually formed."""
    
    if expected_mode == "orchestrated_mix":
        # Strict criteria for orchestrated mode
        required = {
            "candidate_discovered": False,
            "candidate_selected": False,
            "needs_action_reached": False,
            "not_round_limit_only": False,
        }
        
        for round_data in run_report.get("rounds", []):
            # Check for candidate discovery
            frontier = round_data.get("hypothesis_frontier", [])
            if len(frontier) > 0:
                required["candidate_discovered"] = True
            
            # Check for candidate selection (track-level observation after candidates)
            if required["candidate_discovered"]:
                target_ref = round_data.get("context", {}).get("target_ref", {})
                if target_ref.get("kind") == "track" and target_ref.get("id"):
                    required["candidate_selected"] = True
            
            # Check for needs_action
            decision = round_data.get("decision", {})
            if decision.get("status") == "needs_action":
                required["needs_action_reached"] = True
        
        # Check termination reason
        if run_report.get("stop_reason") not in ["round_limit", "cycle_limit"]:
            required["not_round_limit_only"] = True
        
        passed = sum(required.values())
        if passed >= 3:  # At least 3 of 4 milestones
            return "PASS"
        else:
            return f"FAIL: orchestrated_mix minimal closure not formed ({required})"
    
    elif expected_mode == "free_dialogue":
        # Different criteria for free dialogue
        has_multi_turn_reasoning = len(run_report.get("rounds", [])) >= 2
        has_observation = run_report.get("observation_receipt_count", 0) > 0
        has_decision = run_report.get("decision_count", 0) > 0
        
        if has_multi_turn_reasoning and has_observation and has_decision:
            return "PASS"
        else:
            return "FAIL: free_dialogue lacks coherent reasoning"
```

#### Day 4-5: End-to-End Testing

**Deliverables:**
- [ ] Create smoke test for free dialogue mode
- [ ] Create smoke test for orchestrated mix mode
- [ ] Validate mode isolation (changing A doesn't break B)
- [ ] Document test results

**Test Scenarios:**

```python
# scripts/test_dual_mode_isolation.py
def test_free_dialogue_mode():
    """Test that free dialogue mode works with rich context."""
    response = agent.chat("把人声做得更有空间感")
    
    assert response["mode"] == "free_dialogue"
    assert "recent_observations" in response["context"]
    assert len(response["context"]["recent_observations"]) > 0
    assert "reasoning_trace" in response["context"]

def test_orchestrated_mix_mode():
    """Test that orchestrated mix mode uses compact context."""
    response = agent.chat("帮我混音这12轨的工程")
    
    assert response["mode"] == "orchestrated_mix"
    assert "hypothesis_frontier" in response["context"]
    assert "observation_refs" in response["context"]
    assert "bundle" not in response["context"]  # Should not have full bundles

def test_mode_isolation():
    """Test that modifying one mode doesn't affect the other."""
    # Modify free dialogue projection
    modify_free_dialogue_projection()
    
    # Run both mode tests
    free_result = test_free_dialogue_mode()
    orch_result = test_orchestrated_mix_mode()
    
    assert free_result == "PASS"
    assert orch_result == "PASS"  # Should still pass
```

---

## 7. Migration Strategy

### 7.1 Backward Compatibility

**Principle:** New architecture must coexist with existing code during migration.

**Strategy:**
1. **Evidence Store as opt-in:** Add feature flag `use_evidence_store`
2. **Fallback to old path:** If Evidence Store is disabled, use existing context passing
3. **Gradual migration:** Migrate one workflow at a time

**Code:**
```go
// agent/internal/chat/server.go
func (s *Server) runAgentLoopChat(...) {
    // Feature flag check
    if !config.UseEvidenceStore() {
        // OLD: Existing implementation
        return s.runAgentLoopChatLegacy(ctx, conversationID, req, cfg)
    }
    
    // NEW: Evidence Store implementation
    return s.runAgentLoopChatWithEvidence(ctx, conversationID, req, cfg)
}
```

### 7.2 Migration Checklist

**Phase 1 (Week 1-2):**
- [ ] Evidence Store implemented and tested
- [ ] Mode Router implemented and tested
- [ ] Context Projector implemented and tested
- [ ] Integration tests pass
- [ ] Feature flag `use_evidence_store=false` (disabled by default)

**Phase 2 (Week 3):**
- [ ] Fix evidence integrity issues
- [ ] All existing smoke tests pass with new architecture
- [ ] Feature flag `use_evidence_store=true` for internal testing

**Phase 3 (Week 4):**
- [ ] Control logic and validation complete
- [ ] New smoke tests pass
- [ ] Performance benchmarks acceptable
- [ ] Feature flag `use_evidence_store=true` by default

**Phase 4 (Week 5-6, optional):**
- [ ] Remove legacy code paths
- [ ] Remove feature flag
- [ ] Update all documentation

### 7.3 Rollback Plan

**If critical issues discovered:**
1. Set `use_evidence_store=false` globally
2. System reverts to existing implementation
3. Fix issues in new architecture
4. Re-enable when ready

**Rollback triggers:**
- P0 bug affecting user workflows
- Performance regression > 50%
- Data corruption in Evidence Store

---

## 8. Acceptance Criteria

### 8.1 Functional Requirements

**FR-1: Evidence Persistence**
- [ ] All observations recorded to Evidence Store
- [ ] All decisions recorded to Evidence Store
- [ ] Evidence survives HTTP boundaries
- [ ] Evidence is never deleted or overwritten

**FR-2: Mode Routing**
- [ ] Router correctly identifies multi-track orchestrated tasks
- [ ] Router correctly identifies single-track creative tasks
- [ ] User preference overrides automatic selection
- [ ] Routing decision is logged with reason

**FR-3: Context Projection**
- [ ] Free dialogue receives full observation bundles
- [ ] Orchestrated mix receives compact parameters
- [ ] Projections are mode-isolated (no cross-contamination)

**FR-4: Execution Paths**
- [ ] Free dialogue mode can execute semantic EQ
- [ ] Orchestrated mix mode can execute MinimalAudioClosure workflow
- [ ] Both modes can transition to execution layer
- [ ] Both modes can complete successfully

### 8.2 Quality Requirements

**QR-1: Performance**
- [ ] Evidence Store read latency < 10ms (p95)
- [ ] Evidence Store write latency < 20ms (p95)
- [ ] Context projection latency < 50ms (p95)
- [ ] Total overhead < 100ms per request

**QR-2: Reliability**
- [ ] Evidence Store has no data loss
- [ ] Mode Router has no routing failures
- [ ] Context Projector handles missing data gracefully
- [ ] System recovers from transient errors

**QR-3: Testability**
- [ ] Unit tests for all core components
- [ ] Integration tests for both modes
- [ ] Smoke tests for end-to-end workflows
- [ ] Mode isolation tests pass

### 8.3 Smoke Test Criteria

**Orchestrated Mix Mode:**
- [ ] Project with 4+ tracks routes to orchestrated mode
- [ ] Observation discovers ≥2 candidates
- [ ] At least one candidate is selected
- [ ] `needs_action` is reached with valid `semantic_processor_intent`
- [ ] PCA handoff succeeds
- [ ] Does NOT terminate on `round_limit` alone

**Free Dialogue Mode:**
- [ ] Single-track creative task routes to free dialogue
- [ ] LLM receives full observation context
- [ ] Multi-turn reasoning is coherent
- [ ] Execution layer is reached
- [ ] User intent is satisfied

---

## 9. Appendix

### 9.1 Key Files to Modify

**New Files (to create):**
```
agent/internal/evidence/
├── types.go              # ConversationEvidence, Observation, Decision, ModeState
├── store.go              # InMemoryEvidenceStore implementation
├── router.go             # DefaultModeRouter implementation
├── projector.go          # DefaultContextProjector implementation
├── frontier.go           # Candidate/Hypothesis management
├── progression.go        # Phase validation and progress assessment
└── store_test.go         # Unit tests
```

**Existing Files (to modify):**
```
agent/internal/chat/
├── server.go                           # Add evidenceStore field
├── goalrunner_chat.go                  # Add mode routing logic
├── free_state_reasoning_loop.go        # Record to Evidence Store
└── semantic_treatment_strategy.go      # Read from Evidence Store

agent/internal/contextruntime/
└── model_projection.go                 # Fix conclusion deletion

scripts/
├── semantic_processor_project_smoke_runner.py  # Update validation
└── test_dual_mode_isolation.py                 # New test suite
```

### 9.2 Glossary

**Evidence Store:** Immutable fact database containing all observations and decisions

**Mode Router:** Component that selects execution mode based on task characteristics

**Context Projector:** Component that generates mode-specific views from evidence

**Free Dialogue Mode:** Flexible, exploratory execution path for creative tasks

**Orchestrated Mix Mode:** Deterministic workflow execution path for standard mixing

**Hypothesis Frontier:** Set of actionable candidates with processor family selection

**Progress Metrics:** Quantitative measures of meaningful advancement toward execution

### 9.3 References

**Internal Documents:**
- `agent_action_workflow_v1_master_plan.md` — Overall agent workflow authority
- `PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1.md` — Orchestration architecture
- `ADR-AGENT-CLEANUP-0001` — Agent cleanup and no-remote policy

**Code References:**
- `agent/internal/agentloop/free_state_reasoning.go` — Free-state loop implementation
- `agent/internal/chat/semantic_dynamic_workflow.go` — Semantic dynamic execution
- `agent/internal/audioclosure/driver.go` — MinimalAudioClosure driver

### 9.4 Open Questions

**Q1: Should Evidence Store be persistent across application restarts?**
- **Phase 1:** In-memory only (lost on restart)
- **Phase 2:** Consider SQLite for persistence
- **Decision:** Defer to Phase 2 based on user feedback

**Q2: How to handle evidence garbage collection?**
- **Option A:** Keep all evidence forever (disk space concern)
- **Option B:** Archive old conversations after N days
- **Option C:** Compress old evidence to summary only
- **Decision:** Start with no GC, add if needed

**Q3: Should mode switching be allowed mid-conversation?**
- **Current:** Once mode is selected, it stays for the conversation
- **Future:** Allow explicit mode switching with user command
- **Decision:** Phase 1 locks mode, Phase 2 allows switching

---

## Summary

This architecture redesign solves the evidence loss problem by:
1. **Separating concerns:** Evidence persistence vs. execution parameters
2. **Mode isolation:** Each mode gets tailored context without polluting the other
3. **Immutable facts:** Observations and decisions are never overwritten
4. **Clear ownership:** Evidence Store owns facts, modes own projections

**Timeline:** 4 weeks
- Week 1: Evidence Store foundation
- Week 2: Mode routing and projection
- Week 3: Evidence integrity fixes
- Week 4: Control logic and validation

**Success Criteria:**
- Both modes execute successfully
- Evidence is preserved across HTTP boundaries
- Mode isolation tests pass
- Performance overhead < 100ms per request

**Next Steps:**
1. Team review and approval of this document
2. Create feature branch `feat/dual-mode-architecture-v2`
3. Begin Week 1 implementation
4. Daily standups to track progress

---

**Document End**
