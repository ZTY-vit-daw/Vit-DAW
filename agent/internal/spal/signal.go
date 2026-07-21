package spal

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const SignalProbeEvidenceSchema = "vit.spal_signal_evidence.v0"

// RenderProbe is intentionally compact. The real L2/DAD layer may carry much
// more data; SPAL only needs same-tap AB identity and a band-energy value.
type RenderProbe struct {
	TapPoint       string       `json:"tap_point"`
	RenderMode     string       `json:"render_mode"`
	RenderRevision string       `json:"render_revision"`
	TrackID        string       `json:"track_id,omitempty"`
	ClipID         string       `json:"clip_id,omitempty"`
	SourceRevision string       `json:"source_revision,omitempty"`
	ClipRevision   string       `json:"clip_revision,omitempty"`
	EvidenceRef    string       `json:"evidence_ref,omitempty"`
	Bands          []BandEnergy `json:"bands"`
}

type BandEnergy struct {
	MinHz    float64 `json:"min_hz"`
	MaxHz    float64 `json:"max_hz"`
	EnergyDB float64 `json:"energy_db"`
}

type SignalVerification struct {
	Status       string   `json:"status"` // pass | mismatch | inconclusive
	Summary      string   `json:"summary,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	BeforeDB     float64  `json:"before_db,omitempty"`
	AfterDB      float64  `json:"after_db,omitempty"`
}

// SignalProbeEvidence is stored with the Action Receipt. The before/after
// probes are evidence of a bounded signal-direction observation only; they do
// not express musical or user acceptance.
type SignalProbeEvidence struct {
	SchemaVersion string           `json:"schema_version"`
	Scope         SignalProbeScope `json:"scope,omitempty"`
	Before        *RenderProbe     `json:"before,omitempty"`
	After         *RenderProbe     `json:"after,omitempty"`
	CaptureErrors []string         `json:"capture_errors,omitempty"`
	EvidenceRefs  []string         `json:"evidence_refs,omitempty"`
}

func NewSignalProbeEvidence(scope SignalProbeScope) SignalProbeEvidence {
	return SignalProbeEvidence{SchemaVersion: SignalProbeEvidenceSchema, Scope: cloneSignalProbeScope(scope)}
}

func (e *SignalProbeEvidence) AddCaptureError(err error) {
	if e == nil || err == nil {
		return
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return
	}
	for _, existing := range e.CaptureErrors {
		if existing == message {
			return
		}
	}
	e.CaptureErrors = append(e.CaptureErrors, message)
}

func (e *SignalProbeEvidence) RefreshEvidenceRefs() {
	if e == nil {
		return
	}
	refs := append([]string(nil), e.EvidenceRefs...)
	if e.Before != nil {
		refs = append(refs, e.Before.EvidenceRef)
	}
	if e.After != nil {
		refs = append(refs, e.After.EvidenceRef)
	}
	e.EvidenceRefs = uniqueSorted(refs)
}

func (e SignalProbeEvidence) Verify(expectation SignalExpectation) SignalVerification {
	e.RefreshEvidenceRefs()
	result := SignalVerification{Status: "inconclusive", EvidenceRefs: append([]string(nil), e.EvidenceRefs...)}
	if len(e.CaptureErrors) > 0 {
		result.Summary = "same-tap L2 signal capture unavailable: " + strings.Join(e.CaptureErrors, "; ")
		return result
	}
	if e.Before == nil || e.After == nil {
		result.Summary = "same-tap L2 signal capture is incomplete"
		return result
	}
	result = VerifySignalDirection(expectation, *e.Before, *e.After)
	result.EvidenceRefs = uniqueSorted(append(result.EvidenceRefs, e.EvidenceRefs...))
	return result
}

// SignalProbeEvidenceFromAny restores the durable receipt representation.
// File-backed orchestration stores round-trip Details through JSON maps, so
// this intentionally accepts either a typed value or an untyped map.
func SignalProbeEvidenceFromAny(value any) (SignalProbeEvidence, error) {
	if evidence, ok := value.(SignalProbeEvidence); ok {
		if evidence.SchemaVersion == "" {
			evidence.SchemaVersion = SignalProbeEvidenceSchema
		}
		if evidence.SchemaVersion != SignalProbeEvidenceSchema {
			return SignalProbeEvidence{}, fmt.Errorf("unsupported SPAL signal evidence schema %q", evidence.SchemaVersion)
		}
		evidence.RefreshEvidenceRefs()
		return evidence, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return SignalProbeEvidence{}, fmt.Errorf("encode SPAL signal evidence: %w", err)
	}
	var evidence SignalProbeEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return SignalProbeEvidence{}, fmt.Errorf("decode SPAL signal evidence: %w", err)
	}
	if evidence.SchemaVersion == "" {
		return SignalProbeEvidence{}, fmt.Errorf("SPAL signal evidence schema is required")
	}
	if evidence.SchemaVersion != SignalProbeEvidenceSchema {
		return SignalProbeEvidence{}, fmt.Errorf("unsupported SPAL signal evidence schema %q", evidence.SchemaVersion)
	}
	if err := evidence.Scope.Validate(); err != nil {
		return SignalProbeEvidence{}, fmt.Errorf("SPAL signal evidence scope: %w", err)
	}
	evidence.RefreshEvidenceRefs()
	return evidence, nil
}

func VerifySignalDirection(expectation SignalExpectation, before, after RenderProbe) SignalVerification {
	result := SignalVerification{Status: "inconclusive"}
	if err := expectation.Validate(); err != nil {
		result.Summary = err.Error()
		return result
	}
	if strings.TrimSpace(expectation.Direction) == "" {
		result.Summary = "no signal direction was requested"
		return result
	}
	if strings.TrimSpace(before.TapPoint) == "" || before.TapPoint != after.TapPoint {
		result.Summary = "same tap point is required for signal verification"
		return result
	}
	if strings.TrimSpace(before.RenderMode) == "" || before.RenderMode != after.RenderMode {
		result.Summary = "same render mode is required for signal verification"
		return result
	}
	if strings.TrimSpace(before.TrackID) != "" && strings.TrimSpace(after.TrackID) != "" && before.TrackID != after.TrackID {
		result.Summary = "same track is required for signal verification"
		return result
	}
	if strings.TrimSpace(before.RenderRevision) == "" || before.RenderRevision == after.RenderRevision {
		result.Summary = "a changed render revision is required for signal verification"
		return result
	}
	beforeBand, beforeOK := coveringBand(before.Bands, expectation.BandLowHz, expectation.BandHighHz)
	afterBand, afterOK := coveringBand(after.Bands, expectation.BandLowHz, expectation.BandHighHz)
	if !beforeOK || !afterOK {
		result.Summary = "before/after L2 probes do not cover the expected frequency band"
		return result
	}
	result.BeforeDB, result.AfterDB = beforeBand.EnergyDB, afterBand.EnergyDB
	result.EvidenceRefs = uniqueSorted([]string{before.EvidenceRef, after.EvidenceRef})
	delta := afterBand.EnergyDB - beforeBand.EnergyDB
	matched := (strings.EqualFold(expectation.Direction, "decrease") && delta < -0.001) ||
		(strings.EqualFold(expectation.Direction, "increase") && delta > 0.001)
	if matched {
		result.Status = "pass"
		result.Summary = fmt.Sprintf("same-tap L2 render probe changed %.2f dB in the expected %s direction", math.Abs(delta), strings.ToLower(expectation.Direction))
		return result
	}
	result.Status = "mismatch"
	result.Summary = fmt.Sprintf("same-tap L2 render probe changed %.2f dB in the opposite or neutral direction", delta)
	return result
}

func coveringBand(bands []BandEnergy, low, high float64) (BandEnergy, bool) {
	var best BandEnergy
	bestWidth := math.Inf(1)
	found := false
	for _, band := range bands {
		if !finite(band.MinHz) || !finite(band.MaxHz) || !finite(band.EnergyDB) {
			continue
		}
		if band.MinHz <= low && band.MaxHz >= high {
			width := band.MaxHz - band.MinHz
			if !found || width < bestWidth {
				best = band
				bestWidth = width
				found = true
			}
		}
	}
	return best, found
}
