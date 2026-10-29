package experiment

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Section automation envelope action identity (M3-C4-AUTO-INTENT-1): the
// D1-S1 domain-table row whose typed action carries a point-array payload
// routed to the kernel's automation.set_points command by the execution port.
const (
	SectionAutomationActionDomain = "section_automation"
	AutomationEnvelopeWriteKind   = "automation_envelope_write"
)

// Envelope structural bounds (the row's own authority; the parameter value
// range itself is port-side — read from the kernel per parameter).
const (
	D1S1AutomationMinPoints      = 2
	D1S1AutomationMaxPoints      = 16
	D1S1AutomationMaxRegionSpanS = 3600.0
	D1S1AutomationReadbackTol    = 1e-6
)

// AutomationEnvelopePoint is one envelope control point in seconds/value/
// curvature space. v is in the target parameter's raw value range; c defaults
// to 0 and stays within [-1,1] (kernel contract).
type AutomationEnvelopePoint struct {
	T float64 `json:"t"`
	V float64 `json:"v"`
	C float64 `json:"c"`
}

// AutomationEnvelopeRegion is the optional region-replacement window. Zero
// means absent (full-curve replacement semantics).
type AutomationEnvelopeRegion struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

func envelopeRows(value any) []map[string]any {
	rows, ok := value.([]map[string]any)
	if ok {
		return rows
	}
	switch typed := value.(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, row := range typed {
			if m, ok := row.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

// AutomationEnvelopePoints parses and structurally validates the typed
// action's "points" array: 2..16 points, t >= 0 strictly ascending, v numeric,
// c within [-1,1] (absent = 0). The parameter value range itself is not known
// here — the execution port's preflight reads it from the kernel and rejects
// out-of-range values fail-closed.
func ParseAutomationEnvelopePoints(typedAction map[string]any) ([]AutomationEnvelopePoint, error) {
	raw := envelopeRows(typedAction["points"])
	if len(raw) < D1S1AutomationMinPoints || len(raw) > D1S1AutomationMaxPoints {
		return nil, fmt.Errorf("D1-S1 section_automation points must carry %d-%d points, got %d",
			D1S1AutomationMinPoints, D1S1AutomationMaxPoints, len(raw))
	}
	points := make([]AutomationEnvelopePoint, 0, len(raw))
	for index, row := range raw {
		t, okT := mapNumber(row, "t")
		v, okV := mapNumber(row, "v")
		if !okT || !okV {
			return nil, fmt.Errorf("D1-S1 section_automation point #%d requires numeric t and v", index)
		}
		if t < 0 {
			return nil, fmt.Errorf("D1-S1 section_automation point #%d t must be >= 0", index)
		}
		c := 0.0
		if value, ok := mapNumber(row, "c"); ok {
			c = value
		}
		if c < -1 || c > 1 {
			return nil, fmt.Errorf("D1-S1 section_automation point #%d c must be within [-1,1]", index)
		}
		points = append(points, AutomationEnvelopePoint{T: t, V: v, C: c})
	}
	for i := 1; i < len(points); i++ {
		if points[i].T <= points[i-1].T {
			return nil, fmt.Errorf("D1-S1 section_automation point #%d t must be strictly ascending", i)
		}
	}
	return points, nil
}

// AutomationEnvelopeRegion parses and validates the optional "region" window:
// 0 <= start < end, span within the bounded ceiling. Absent returns the zero
// region and no error (full-curve replacement). When present, every point
// must already lie inside the window; that cross-check lives in
// AutomationEnvelopePointsWithinRegion so callers can bind it after parsing.
func ParseAutomationEnvelopeRegion(typedAction map[string]any) (AutomationEnvelopeRegion, error) {
	row, _ := typedAction["region"].(map[string]any)
	if row == nil {
		return AutomationEnvelopeRegion{}, nil
	}
	start, okStart := mapNumber(row, "start")
	end, okEnd := mapNumber(row, "end")
	if !okStart || !okEnd {
		return AutomationEnvelopeRegion{}, fmt.Errorf("D1-S1 section_automation region requires numeric start and end seconds")
	}
	if start < 0 || end <= start {
		return AutomationEnvelopeRegion{}, fmt.Errorf("D1-S1 section_automation region must satisfy 0 <= start < end")
	}
	if end-start > D1S1AutomationMaxRegionSpanS {
		return AutomationEnvelopeRegion{}, fmt.Errorf("D1-S1 section_automation region span must stay within %.0fs", D1S1AutomationMaxRegionSpanS)
	}
	return AutomationEnvelopeRegion{Start: start, End: end}, nil
}

// AutomationEnvelopePointsWithinRegion checks that every point lands inside
// the region window (region-replacement semantics: new points must all sit in
// the replaced region).
func AutomationEnvelopePointsWithinRegion(points []AutomationEnvelopePoint, region AutomationEnvelopeRegion) error {
	if region.End <= region.Start {
		return nil
	}
	for index, point := range points {
		if point.T < region.Start || point.T > region.End {
			return fmt.Errorf("D1-S1 section_automation point #%d t=%.3fs lies outside the region [%.3fs, %.3fs]",
				index, point.T, region.Start, region.End)
		}
	}
	return nil
}

// AutomationEnvelopePointsMaps renders parsed points back to the journal /
// command payload shape.
func AutomationEnvelopePointsMaps(points []AutomationEnvelopePoint) []map[string]any {
	out := make([]map[string]any, 0, len(points))
	for _, point := range points {
		out = append(out, map[string]any{"t": point.T, "v": point.V, "c": point.C})
	}
	return out
}

// AutomationEnvelopeCurveMatches asserts a readback curve against the written
// points: same count and per-point t/v/c within tolerance. It is the settle
// assertion skeleton of the choreography chain (deterministic, evidence-only).
func AutomationEnvelopeCurveMatches(written []AutomationEnvelopePoint, readback []map[string]any) error {
	rows := envelopeRows(readback)
	if len(rows) != len(written) {
		return fmt.Errorf("readback point_count %d does not match written %d", len(rows), len(written))
	}
	type readRow struct{ t, v, c float64 }
	got := make([]readRow, 0, len(rows))
	for _, row := range rows {
		t, okT := mapNumber(row, "t")
		v, okV := mapNumber(row, "v")
		if !okT || !okV {
			return fmt.Errorf("readback point carries non-numeric t/v")
		}
		c := 0.0
		if value, ok := mapNumber(row, "c"); ok {
			c = value
		}
		got = append(got, readRow{t: t, v: v, c: c})
	}
	sort.SliceStable(got, func(i, j int) bool { return got[i].t < got[j].t })
	for index, want := range written {
		have := got[index]
		if math.Abs(have.t-want.T) > D1S1AutomationReadbackTol ||
			math.Abs(have.v-want.V) > D1S1AutomationReadbackTol ||
			math.Abs(have.c-want.C) > D1S1AutomationReadbackTol {
			return fmt.Errorf("readback point #%d (t=%.6f v=%.6f c=%.6f) does not match written (t=%.6f v=%.6f c=%.6f)",
				index, have.t, have.v, have.c, want.T, want.V, want.C)
		}
	}
	return nil
}

// AutomationEnvelopeValueRangeHint renders the raw value range for disclosure
// (used by the confirmation card wording when the port has resolved it).
func AutomationEnvelopeValueRangeHint(min, max float64) string {
	return strings.TrimSpace(fmt.Sprintf("[%g, %g]", min, max))
}
