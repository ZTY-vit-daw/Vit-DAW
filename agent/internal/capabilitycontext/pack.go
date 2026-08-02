package capabilitycontext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	SchemaVersion                = "capability_context_pack.v0"
	GainStagingCapabilityID      = "static_mix.gain_staging.v0"
	StaticBalanceCapabilityID    = "static_mix.static_balance.v0"
	PanLayoutCapabilityID        = "static_mix.pan_layout.v0"
	LowEndRelationCapabilityID   = "static_mix.low_end_relation.v0"
	FrequencyCleanupCapabilityID = "fine_mix.frequency_cleanup.v1"
)

type Budget struct {
	MaxTracks          int `json:"max_tracks,omitempty"`
	MaxDisclosedTracks int `json:"max_disclosed_tracks,omitempty"`
	MaxRankingRows     int `json:"max_ranking_rows"`
	MaxClipsPerTrack   int `json:"max_clips_per_track"`
	MaxStringRunes     int `json:"max_string_runes"`
}

type Scope struct {
	Kind      string   `json:"kind"`
	IDs       []string `json:"ids,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Source    string   `json:"source,omitempty"`
	TimeRange *Range   `json:"time_range,omitempty"`
}

type Range struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
}

type EvidenceStatus struct {
	Status     string `json:"status"`
	KnownCount int    `json:"known_count,omitempty"`
	TotalCount int    `json:"total_count,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type Pack struct {
	SchemaVersion     string                    `json:"schema_version"`
	PackID            string                    `json:"pack_id"`
	CapabilityID      string                    `json:"capability_id"`
	CapabilityName    string                    `json:"capability_name"`
	ContextManifestID string                    `json:"context_manifest_id,omitempty"`
	ContextBuilder    string                    `json:"context_builder,omitempty"`
	GeneratedAt       string                    `json:"generated_at"`
	UserIntent        string                    `json:"user_intent,omitempty"`
	Scope             Scope                     `json:"scope"`
	Budget            Budget                    `json:"budget"`
	EvidenceRefs      []string                  `json:"evidence_refs,omitempty"`
	EvidenceStatus    map[string]EvidenceStatus `json:"evidence_status,omitempty"`
	Summary           map[string]any            `json:"summary,omitempty"`
	Tracks            []TrackGainRow            `json:"tracks,omitempty"`
	ReferenceLevel    map[string]any            `json:"reference_level,omitempty"`
	Rankings          map[string][]RankRow      `json:"rankings,omitempty"`
	ClipGainSummary   map[string]any            `json:"clip_gain_summary,omitempty"`
	Limitations       []string                  `json:"limitations,omitempty"`
	FollowUpTools     []string                  `json:"follow_up_tools,omitempty"`
	Excluded          []string                  `json:"excluded,omitempty"`
	Guidance          []string                  `json:"guidance,omitempty"`
}

func (p Pack) Map() map[string]any {
	data, _ := json.Marshal(p)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func (p Pack) JSON() string {
	data, err := json.Marshal(p)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func NormalizeBudget(b Budget) Budget {
	if b.MaxTracks <= 0 {
		b.MaxTracks = 24
	}
	if b.MaxRankingRows <= 0 {
		b.MaxRankingRows = 8
	}
	if b.MaxDisclosedTracks <= 0 {
		b.MaxDisclosedTracks = 12
	}
	if b.MaxClipsPerTrack <= 0 {
		b.MaxClipsPerTrack = 4
	}
	if b.MaxStringRunes <= 0 {
		b.MaxStringRunes = 96
	}
	return b
}

func stablePackID(capabilityID, intent string, refs []string, generatedAt time.Time) string {
	sort.Strings(refs)
	seed := strings.Join([]string{capabilityID, strings.TrimSpace(intent), strings.Join(refs, "|"), generatedAt.UTC().Format(time.RFC3339)}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	return "cap_pack_" + hex.EncodeToString(sum[:])[:16]
}

func addUnique(values []string, next ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	for _, value := range next {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func compactText(value any, maxRunes int) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	runes := []rune(text)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return text
}
