package loudnesswatch

// Pure comparator (card LOUDNESS-WATCH-CONFIG-1 item ②): rows in, verdict
// out, no I/O and no clock. v1 pinned measurement semantics (design 04 §5):
//   - measurement source is the existing approximate LoudnessSummary rows
//     (RMS-offset integrated + sample peak); no new producers;
//   - every verdict is measurement_grade="approximate" and the summary text
//     carries the qualifier — user-visible text never claims BS.1770 grade;
//   - short-term/momentary thresholds are storable but their compare face is
//     always unmeasurable (the kernel has no ST/M producer) — disclosed, not
//     faked;
//   - the peak dimension is a necessary-condition check: sample peak above
//     the true-peak ceiling is a breach; at-or-below is "no breach seen",
//     never a pass (inter-sample peaks are invisible at sample accuracy);
//   - unmeasurable is a first-class outcome (missing rows / unresolvable
//     config / ST-M configured) and produces neither pass nor breach.

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/rlm"
)

// Verdict states (three-state, design 01 §2.4).
const (
	VerdictPass         = "pass"
	VerdictBreach       = "breach"
	VerdictUnmeasurable = "unmeasurable"
)

// Peak dimension states: the peak check only ever produces a breach or the
// honest "no breach seen" — never a pass (design 04 §5.2).
const (
	PeakCheckBreach       = "breach"
	PeakCheckNoBreachSeen = "no_breach_seen"
)

// Verdict is the loudness_watch.v0 record. Project identity, trigger and
// CheckedAt are caller-supplied context (the trigger wiring card fills them);
// Compare leaves them empty rather than fabricating values.
type Verdict struct {
	SchemaVersion   string `json:"schema_version"`
	ProjectUUID     string `json:"project_uuid,omitempty"`
	ProjectRevision string `json:"project_revision,omitempty"`
	TriggerKind     string `json:"trigger_kind,omitempty"`
	TriggerRef      string `json:"trigger_ref,omitempty"`
	// MeasurementGrade is forced to approximate in v1 (design 04 §5.2).
	MeasurementGrade string         `json:"measurement_grade"`
	ConfigDigest     string         `json:"config_digest"`
	Verdict          string         `json:"verdict"`
	Measured         []MeasuredClip `json:"measured,omitempty"`
	BreachDetails    []BreachDetail `json:"breach_details,omitempty"`
	// PeakCheck: "breach" or "no_breach_seen" when peak values were present;
	// empty when no row carried a peak.
	PeakCheck string `json:"peak_check,omitempty"`
	// UnmeasurableDimensions lists configured dimensions that could not be
	// measured (e.g. short_term_max, momentary_max).
	UnmeasurableDimensions []string `json:"unmeasurable_dimensions,omitempty"`
	UnmeasurableReason     string   `json:"unmeasurable_reason,omitempty"`
	// Summary is the user-visible line; always carries the approximate-grade
	// qualifier (approximate口径限定词).
	Summary   string `json:"summary"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// MeasuredClip is one consumed measurement row.
type MeasuredClip struct {
	ClipID         string   `json:"clip_id"`
	IntegratedLUFS *float64 `json:"integrated_lufs,omitempty"`
	SamplePeakDBFS *float64 `json:"sample_peak_dbfs,omitempty"`
	// Approximate is always true in v1: both consumed metrics are
	// approximate-grade (design 04 §2).
	Approximate bool `json:"approximate"`
	// Note carries honest caveats for partial rows (e.g. missing integrated
	// value on an otherwise present row).
	Note string `json:"note,omitempty"`
}

// BreachDetail is one out-of-bound fact.
type BreachDetail struct {
	ClipID    string  `json:"clip_id,omitempty"`
	Metric    string  `json:"metric"`
	Measured  float64 `json:"measured"`
	Threshold float64 `json:"threshold"`
	// DeltaDB is signed: measured minus threshold (positive = above).
	DeltaDB float64 `json:"delta_db"`
}

// row accessors mirror the existing consumption forms (integrated fallback
// chain per capabilitycontext/gain_staging.go; peak per the LoudnessSummary
// row keys published by the kernel).
func rowNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			switch typed := value.(type) {
			case float64:
				return typed, true
			case int:
				return float64(typed), true
			}
		}
	}
	return 0, false
}

func rowString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func rowPtr(value float64) *float64 { return &value }

// Compare evaluates the measurement rows against the configured thresholds.
// rows are loudness_summary-shaped maps (clip_id, integrated_lufs /
// approximate_lufs / lufs / lufs_estimate, sample_peak_dbfs / peak_dbfs).
func Compare(rows []map[string]any, config Config) Verdict {
	verdict := Verdict{SchemaVersion: VerdictSchemaVersion, MeasurementGrade: MeasurementGradeApproximate}
	thresholds, err := ResolveThresholds(config)
	if err != nil {
		verdict.Verdict = VerdictUnmeasurable
		verdict.UnmeasurableReason = "config_invalid: " + err.Error()
		verdict.Summary = approximateSummary(verdict.Verdict, verdict.UnmeasurableReason, 0, "")
		return verdict
	}
	verdict.ConfigDigest = ConfigDigest(config)
	if len(rows) == 0 {
		verdict.Verdict = VerdictUnmeasurable
		verdict.UnmeasurableReason = "no_measurement_rows: no loudness_summary rows available for this project"
		verdict.Summary = approximateSummary(verdict.Verdict, verdict.UnmeasurableReason, 0, "")
		return verdict
	}

	unmeasurableDims := map[string]bool{}
	if thresholds.ShortTermMax != nil {
		unmeasurableDims["short_term_max"] = true
	}
	if thresholds.MomentaryMax != nil {
		unmeasurableDims["momentary_max"] = true
	}

	breaches := []BreachDetail{}
	peakPresent, peakBreach := false, false
	measurable := 0
	for index, row := range rows {
		clip := MeasuredClip{
			ClipID:      rowString(row, "clip_id", "track_id"),
			Approximate: true,
		}
		if clip.ClipID == "" {
			clip.ClipID = fmt.Sprintf("(clip %d)", index)
		}
		if value, ok := rowNumber(row, "integrated_lufs", "approximate_lufs", "lufs", "lufs_estimate"); ok {
			clip.IntegratedLUFS = rowPtr(value)
		}
		if value, ok := rowNumber(row, "sample_peak_dbfs", "peak_dbfs"); ok {
			clip.SamplePeakDBFS = rowPtr(value)
		}
		if clip.IntegratedLUFS == nil {
			clip.Note = "integrated_lufs missing on this row"
		} else {
			measurable++
			breaches = append(breaches, integratedBreaches(clip.ClipID, *clip.IntegratedLUFS, thresholds.Integrated)...)
		}
		if clip.SamplePeakDBFS != nil {
			peakPresent = true
			if *clip.SamplePeakDBFS > thresholds.TruePeakMaxDBTP {
				peakBreach = true
				breaches = append(breaches, BreachDetail{
					ClipID:    clip.ClipID,
					Metric:    "sample_peak_dbfs",
					Measured:  *clip.SamplePeakDBFS,
					Threshold: thresholds.TruePeakMaxDBTP,
					DeltaDB:   *clip.SamplePeakDBFS - thresholds.TruePeakMaxDBTP,
				})
			}
		}
		verdict.Measured = append(verdict.Measured, clip)
	}

	if peakPresent {
		if peakBreach {
			verdict.PeakCheck = PeakCheckBreach
		} else {
			verdict.PeakCheck = PeakCheckNoBreachSeen
		}
	}

	switch {
	case len(breaches) > 0:
		verdict.Verdict = VerdictBreach
		verdict.BreachDetails = breaches
	case measurable == 0:
		verdict.Verdict = VerdictUnmeasurable
		verdict.UnmeasurableReason = "integrated_lufs_missing: rows present but none carries an integrated loudness value"
	case len(unmeasurableDims) > 0:
		verdict.Verdict = VerdictUnmeasurable
		verdict.UnmeasurableReason = "no_producer: configured short-term/momentary ceilings have no kernel producer (v1 publishes approximate integrated + sample peak only)"
	default:
		verdict.Verdict = VerdictPass
	}
	for dim := range unmeasurableDims {
		verdict.UnmeasurableDimensions = append(verdict.UnmeasurableDimensions, dim)
	}
	sort.Strings(verdict.UnmeasurableDimensions)
	verdict.Summary = approximateSummary(verdict.Verdict, verdict.UnmeasurableReason, len(breaches), verdict.PeakCheck)
	return verdict
}

// integratedBreaches checks one clip against the target band. nil Min means
// the standard sets no floor (Apple/Spotify shape): only the ceiling applies.
func integratedBreaches(clipID string, integrated float64, band rlm.LoudnessBand) []BreachDetail {
	breaches := []BreachDetail{}
	if band.Min != nil && integrated < *band.Min {
		breaches = append(breaches, BreachDetail{
			ClipID:    clipID,
			Metric:    "integrated_lufs",
			Measured:  integrated,
			Threshold: *band.Min,
			DeltaDB:   integrated - *band.Min,
		})
	}
	if integrated > band.Max {
		breaches = append(breaches, BreachDetail{
			ClipID:    clipID,
			Metric:    "integrated_lufs",
			Measured:  integrated,
			Threshold: band.Max,
			DeltaDB:   integrated - band.Max,
		})
	}
	return breaches
}

func approximateSummary(verdict, reason string, breachCount int, peakCheck string) string {
	text := "近似口径（approximate）：integrated 为 RMS 偏移近似值（approximate_rms_lufs_v1，未加 K 加权/门控），非 BS.1770 认证口径；peak 为 sample 精度必要条件检查。"
	if peakCheck == PeakCheckNoBreachSeen {
		text += "peak 未见 breach（inter-sample 峰不可见，不构成 TP 合规判定）。"
	}
	switch verdict {
	case VerdictPass:
		text += "本次未见越界（近似级结论，不构成交付级判定）。"
	case VerdictBreach:
		text += fmt.Sprintf("检出 %d 处越界（近似级）。", breachCount)
	case VerdictUnmeasurable:
		text += "无法测量：" + reason + "。"
	}
	return text
}
