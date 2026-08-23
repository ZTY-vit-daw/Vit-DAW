package audioclosure

import (
	"fmt"
	"strings"
)

// Free-state diagnostic round and priority queue schemas
// (docs/FREE_STATE_DIAGNOSTIC_ROUND_AND_PRIORITY_QUEUE_SCHEMA_V1.md).

const (
	DiagnosticRoundSchema   = "free_state_diagnostic_round.v1"
	PriorityQueueSchema     = "free_state_priority_queue.v1"
	diagnosticRoundIDPrefix = "r_"
)

// DiagnosticDimension is one axis of the diagnostic spine.
type DiagnosticDimension string

const (
	DimensionLevelHeadroom      DiagnosticDimension = "level_headroom"
	DimensionFrequencyOccupancy DiagnosticDimension = "frequency_occupancy"
	DimensionDynamics           DiagnosticDimension = "dynamics"
	DimensionStereoSpace        DiagnosticDimension = "stereo_space"
	DimensionTransientEvent     DiagnosticDimension = "transient_event"
)

// DefaultDimensionOrder is the contract §2 default order (an ordering input,
// not an assertion).
var DefaultDimensionOrder = []DiagnosticDimension{
	DimensionLevelHeadroom, DimensionFrequencyOccupancy, DimensionDynamics,
	DimensionStereoSpace, DimensionTransientEvent,
}

// dimensionViews is the contract §3 dimension → view mapping. Views are peers,
// never a pipeline.
var dimensionViews = map[DiagnosticDimension]struct{ Primary, Supporting []string }{
	DimensionLevelHeadroom: {
		Primary:    []string{"track.basic_energy"},
		Supporting: []string{"track.peak_structure", "mix.multitrack_relationship", "project.structure"},
	},
	DimensionFrequencyOccupancy: {
		Primary:    []string{"track.timbre_frequency"},
		Supporting: []string{"mix.frequency_relationship", "mix.masking_relationship", "track.frequency_time_events"},
	},
	DimensionDynamics: {
		Primary:    []string{"track.time_dynamics"},
		Supporting: []string{"track.band_dynamics", "processor.behavior", "track.activity_structure"},
	},
	DimensionStereoSpace: {
		Primary:    []string{"track.stereo_space"},
		Supporting: []string{"mix.multitrack_relationship"},
	},
	DimensionTransientEvent: {
		Primary:    []string{"track.transient_structure"},
		Supporting: []string{"track.frequency_time_events", "track.peak_structure"},
	},
}

// DimensionPrimaryViews returns the primary (main) views of a dimension.
func DimensionPrimaryViews(dim DiagnosticDimension) []string {
	return append([]string(nil), dimensionViews[dim].Primary...)
}

func ValidDimensionViews(dim DiagnosticDimension) []string {
	views := dimensionViews[dim]
	out := append([]string(nil), views.Primary...)
	return append(out, views.Supporting...)
}

// PriorityReason explains a deviation from the default dimension order.
type PriorityReason string

const (
	PriorityDefaultOrder    PriorityReason = "default_order"
	PriorityProjectEvidence PriorityReason = "project_evidence"
	PriorityFreshness       PriorityReason = "freshness"
	PriorityCost            PriorityReason = "cost"
	PriorityContradiction   PriorityReason = "contradiction"
)

// SkipReason is the closed enumeration for skipping a dimension.
type SkipReason string

const (
	SkipNotApplicable       SkipReason = "not_applicable"
	SkipFreshEvidenceExists SkipReason = "fresh_evidence_exists"
	SkipAlreadyCovered      SkipReason = "already_covered"
	SkipDependencyMissing   SkipReason = "dependency_unavailable"
	SkipCostLimited         SkipReason = "cost_limited"
)

func validSkipReason(reason SkipReason) bool {
	switch reason {
	case SkipNotApplicable, SkipFreshEvidenceExists, SkipAlreadyCovered, SkipDependencyMissing, SkipCostLimited:
		return true
	default:
		return false
	}
}

// RoundEvidenceStatus is derived only from CCB observation receipts.
type RoundEvidenceStatus string

const (
	RoundEvidenceOpen     RoundEvidenceStatus = "open"
	RoundEvidenceReady    RoundEvidenceStatus = "ready"
	RoundEvidencePartial  RoundEvidenceStatus = "partial"
	RoundEvidenceStale    RoundEvidenceStatus = "stale"
	RoundEvidenceRejected RoundEvidenceStatus = "rejected"
)

// SkippedDimension records one skip with its mandatory enum reason.
type SkippedDimension struct {
	Dimension DiagnosticDimension `json:"dimension"`
	Reason    SkipReason          `json:"reason"`
}

// DiagnosticRoundRecord is `free_state_diagnostic_round.v1`: ten fields plus
// the three identity keys from the ADR §10 persistence map.
type DiagnosticRoundRecord struct {
	SchemaVersion       string              `json:"schema_version"`
	RoundID             string              `json:"round_id"`
	GoalID              string              `json:"goal_id,omitempty"`
	RunID               string              `json:"run_id,omitempty"`
	ConversationID      string              `json:"conversation_id,omitempty"`
	PrimaryDimension    DiagnosticDimension `json:"primary_dimension"`
	PriorityReason      PriorityReason      `json:"priority_reason"`
	ViewsRequested      []string            `json:"views_requested"`
	EvidenceStatus      RoundEvidenceStatus `json:"evidence_status"`
	CandidateRefs       []string            `json:"candidate_refs,omitempty"`
	UnresolvedQuestions []string            `json:"unresolved_questions,omitempty"`
	SkippedDimensions   []SkippedDimension  `json:"skipped_dimensions,omitempty"`
	NextPriority        []string            `json:"next_priority,omitempty"`
	ProjectRevision     string              `json:"project_revision"`
	CreatedAt           string              `json:"created_at,omitempty"`
	ClosedAt            string              `json:"closed_at,omitempty"`
}

// Closed reports whether this dimension is closed: usable evidence and no open
// unresolved question (contract G4).
func (r DiagnosticRoundRecord) Closed() bool {
	switch r.EvidenceStatus {
	case RoundEvidenceReady:
		return len(r.UnresolvedQuestions) == 0
	case RoundEvidencePartial:
		return false
	default:
		return false
	}
}

func (r DiagnosticRoundRecord) Validate() error {
	if strings.TrimSpace(r.SchemaVersion) != DiagnosticRoundSchema {
		return fmt.Errorf("round schema_version must be %s", DiagnosticRoundSchema)
	}
	if !strings.HasPrefix(strings.TrimSpace(r.RoundID), diagnosticRoundIDPrefix) {
		return fmt.Errorf("round_id must use the %s prefix", diagnosticRoundIDPrefix)
	}
	if r.PrimaryDimension == "" {
		return fmt.Errorf("primary_dimension is required (exactly one per round)")
	}
	views, known := dimensionViews[r.PrimaryDimension]
	if !known {
		return fmt.Errorf("unknown primary_dimension %q", r.PrimaryDimension)
	}
	allowed := map[string]bool{}
	for _, viewID := range append(append([]string(nil), views.Primary...), views.Supporting...) {
		allowed[viewID] = true
	}
	if len(r.ViewsRequested) == 0 {
		return fmt.Errorf("views_requested is required")
	}
	for _, viewID := range r.ViewsRequested {
		if !allowed[strings.TrimSpace(viewID)] {
			return fmt.Errorf("views_requested contains %q which is outside the allowed views of dimension %s", viewID, r.PrimaryDimension)
		}
	}
	switch r.PriorityReason {
	case PriorityDefaultOrder, PriorityProjectEvidence, PriorityFreshness, PriorityCost, PriorityContradiction:
	default:
		return fmt.Errorf("unknown priority_reason %q", r.PriorityReason)
	}
	switch r.EvidenceStatus {
	case RoundEvidenceOpen, RoundEvidenceReady, RoundEvidencePartial, RoundEvidenceStale, RoundEvidenceRejected:
	default:
		return fmt.Errorf("unknown evidence_status %q", r.EvidenceStatus)
	}
	for i, skip := range r.SkippedDimensions {
		if _, known := dimensionViews[skip.Dimension]; !known {
			return fmt.Errorf("skipped_dimensions[%d] has unknown dimension %q", i, skip.Dimension)
		}
		if !validSkipReason(skip.Reason) {
			return fmt.Errorf("skipped_dimensions[%d] reason %q is outside the skip-reason enumeration", i, skip.Reason)
		}
	}
	return nil
}

// QueueEntryStatus is one dimension's state inside the queue.
type QueueEntryStatus string

const (
	QueueOpen    QueueEntryStatus = "open"
	QueueClosed  QueueEntryStatus = "closed"
	QueueSkipped QueueEntryStatus = "skipped"
)

// PriorityQueueEntry is one dimension slot in `free_state_priority_queue.v1`.
type PriorityQueueEntry struct {
	Dimension      DiagnosticDimension `json:"dimension"`
	PriorityReason PriorityReason      `json:"priority_reason"`
	Status         QueueEntryStatus    `json:"status"`
	SkipReason     SkipReason          `json:"skip_reason,omitempty"`
}

// PriorityQueue is `free_state_priority_queue.v1`.
type PriorityQueue struct {
	SchemaVersion string               `json:"schema_version"`
	Entries       []PriorityQueueEntry `json:"entries"`
}

func DefaultPriorityQueue() PriorityQueue {
	entries := make([]PriorityQueueEntry, 0, len(DefaultDimensionOrder))
	for _, dim := range DefaultDimensionOrder {
		entries = append(entries, PriorityQueueEntry{Dimension: dim, PriorityReason: PriorityDefaultOrder, Status: QueueOpen})
	}
	return PriorityQueue{SchemaVersion: PriorityQueueSchema, Entries: entries}
}

func (q PriorityQueue) Validate() error {
	if strings.TrimSpace(q.SchemaVersion) != PriorityQueueSchema {
		return fmt.Errorf("queue schema_version must be %s", PriorityQueueSchema)
	}
	if len(q.Entries) == 0 {
		return fmt.Errorf("queue entries are required")
	}
	seen := map[DiagnosticDimension]bool{}
	for i, entry := range q.Entries {
		if _, known := dimensionViews[entry.Dimension]; !known {
			return fmt.Errorf("queue entry %d has unknown dimension %q", i, entry.Dimension)
		}
		if seen[entry.Dimension] {
			return fmt.Errorf("queue entry %d duplicates dimension %s", i, entry.Dimension)
		}
		seen[entry.Dimension] = true
		switch entry.PriorityReason {
		case PriorityDefaultOrder, PriorityProjectEvidence, PriorityFreshness, PriorityCost, PriorityContradiction:
		default:
			return fmt.Errorf("queue entry %d has unknown priority_reason %q", i, entry.PriorityReason)
		}
		switch entry.Status {
		case QueueOpen, QueueClosed:
		case QueueSkipped:
			if !validSkipReason(entry.SkipReason) {
				return fmt.Errorf("queue entry %d skips %s without a valid enum reason", i, entry.Dimension)
			}
		default:
			return fmt.Errorf("queue entry %d has unknown status %q", i, entry.Status)
		}
	}
	// A queue ordered exactly as the default needs no deviation reason; any
	// other order must carry at least one non-default reason.
	sameOrder := len(q.Entries) == len(DefaultDimensionOrder)
	hasDeviation := false
	for i, entry := range q.Entries {
		if sameOrder && entry.Dimension != DefaultDimensionOrder[i] {
			sameOrder = false
		}
		if entry.PriorityReason != PriorityDefaultOrder {
			hasDeviation = true
		}
	}
	if !sameOrder && !hasDeviation {
		return fmt.Errorf("queue order deviates from the default order without a priority_reason")
	}
	return nil
}

// QueueFromRounds derives the live priority queue from the persisted
// diagnostic round records: a closed round closes its primary dimension, and
// each recorded skip marks that dimension skipped with its enum reason.
// Open/partial rounds leave their dimension open. The queue is therefore a
// projection of the event stream, never a second source of truth.
func QueueFromRounds(rounds []DiagnosticRoundRecord) PriorityQueue {
	queue := DefaultPriorityQueue()
	index := make(map[DiagnosticDimension]int, len(queue.Entries))
	for i, entry := range queue.Entries {
		index[entry.Dimension] = i
	}
	for _, round := range rounds {
		if round.Validate() != nil {
			continue
		}
		if i, ok := index[round.PrimaryDimension]; ok && round.Closed() && queue.Entries[i].Status == QueueOpen {
			queue.Entries[i].Status = QueueClosed
		}
		for _, skip := range round.SkippedDimensions {
			if i, ok := index[skip.Dimension]; ok && queue.Entries[i].Status == QueueOpen {
				queue.Entries[i].Status = QueueSkipped
				queue.Entries[i].SkipReason = skip.Reason
			}
		}
	}
	return queue
}

// HasOpen reports whether any dimension remains diagnosable.
func (q PriorityQueue) HasOpen() bool {
	for _, entry := range q.Entries {
		if entry.Status == QueueOpen {
			return true
		}
	}
	return false
}

// ReObservationRequest is the input to the same-target/view-set re-request
// admission rule (contract §2): a repeat is admitted only by new evidence, a
// new project revision, or a recorded contradiction.
type ReObservationRequest struct {
	RequestedViews         []string
	PriorViews             []string
	PriorEvidenceStatus    RoundEvidenceStatus
	PriorUnresolved        []string
	NewUnresolvedQuestions []string
	PriorProjectRevision   string
	NewProjectRevision     string
	NewPriorityReason      PriorityReason
	DeclaredContradiction  bool
}

func sameViewSet(a, b []string) bool {
	na, nb := normalizedStrings(a), normalizedStrings(b)
	if len(na) != len(nb) {
		return false
	}
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

// AdmitReObservation returns (admitted, reason). A non-repeating view set is
// always admitted.
func AdmitReObservation(req ReObservationRequest) (bool, string) {
	if !sameViewSet(req.RequestedViews, req.PriorViews) {
		return true, "new_view_set"
	}
	// Admission 1: new evidence — the new round must surface a question that
	// was not open in the prior round, with a contradiction-priority revisit
	// or a discriminating new view.
	if len(req.PriorUnresolved) > 0 && req.NewPriorityReason == PriorityContradiction &&
		hasNewUnresolvedQuestion(req.PriorUnresolved, req.NewUnresolvedQuestions) {
		return true, "new_evidence_contradiction"
	}
	// Admission 2: new project revision.
	if strings.TrimSpace(req.NewProjectRevision) != "" &&
		strings.TrimSpace(req.PriorProjectRevision) != strings.TrimSpace(req.NewProjectRevision) {
		return true, "new_project_revision"
	}
	// Admission 3: recorded contradiction — prior status stale/partial plus an
	// explicit conflict declaration in the new round.
	if (req.PriorEvidenceStatus == RoundEvidenceStale || req.PriorEvidenceStatus == RoundEvidencePartial) &&
		req.DeclaredContradiction {
		return true, "recorded_contradiction"
	}
	return false, "repeat_without_admission"
}

func hasNewUnresolvedQuestion(prior, next []string) bool {
	seen := map[string]bool{}
	for _, question := range prior {
		seen[strings.TrimSpace(question)] = true
	}
	for _, question := range next {
		if question = strings.TrimSpace(question); question != "" && !seen[question] {
			return true
		}
	}
	return false
}
