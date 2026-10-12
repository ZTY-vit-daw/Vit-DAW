package loudnesswatch

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/rlm"
)

func loudnessRow(clipID string, integrated float64) map[string]any {
	return map[string]any{"clip_id": clipID, "integrated_lufs": integrated, "sample_peak_dbfs": -6.0}
}

// Test ⑦ (card acceptance list): band breaches on both sides, nil-min bands
// skip the lower check, and unmeasurable is honest for missing rows and for
// ST/M dimensions that have no kernel producer.
func TestCompareBandSemantics(t *testing.T) {
	r128 := validBuiltinConfig()

	above := Compare([]map[string]any{loudnessRow("a", -20)}, r128)
	if above.Verdict != VerdictBreach || len(above.BreachDetails) != 1 {
		t.Fatalf("-20 LUFS vs R128 must breach above max: %+v", above)
	}
	if above.BreachDetails[0].Metric != "integrated_lufs" || above.BreachDetails[0].Threshold != -22.5 {
		t.Fatalf("breach detail drifted: %+v", above.BreachDetails[0])
	}

	below := Compare([]map[string]any{loudnessRow("b", -26)}, r128)
	if below.Verdict != VerdictBreach || len(below.BreachDetails) != 1 {
		t.Fatalf("-26 LUFS vs R128 must breach below min: %+v", below)
	}
	if below.BreachDetails[0].Threshold != -23.5 || below.BreachDetails[0].DeltaDB >= 0 {
		t.Fatalf("below-min breach must carry the floor threshold and a negative delta: %+v", below.BreachDetails[0])
	}

	inBand := Compare([]map[string]any{loudnessRow("c", -23)}, r128)
	if inBand.Verdict != VerdictPass {
		t.Fatalf("-23 LUFS vs R128 must pass: %+v", inBand)
	}

	// nil-min band (apple_music: no floor): only the ceiling is checked.
	apple := Normalize(Config{Enabled: true, ProfileSource: "builtin:apple_music"})
	belowFloor := Compare([]map[string]any{loudnessRow("d", -30)}, apple)
	if belowFloor.Verdict != VerdictPass {
		t.Fatalf("nil-min band must not check the lower bound (-30 vs apple cap -16): %+v", belowFloor)
	}
	overCap := Compare([]map[string]any{loudnessRow("e", -12)}, apple)
	if overCap.Verdict != VerdictBreach {
		t.Fatalf("-12 LUFS vs apple cap -16 must breach: %+v", overCap)
	}
}

func TestCompareUnmeasurableCases(t *testing.T) {
	// Missing rows.
	empty := Compare(nil, validBuiltinConfig())
	if empty.Verdict != VerdictUnmeasurable || !strings.Contains(empty.UnmeasurableReason, "no_measurement_rows") {
		t.Fatalf("missing rows must be unmeasurable: %+v", empty)
	}

	// Rows present but none measurable.
	blind := Compare([]map[string]any{{"clip_id": "x", "peak_dbfs": -6.0}}, validBuiltinConfig())
	if blind.Verdict != VerdictUnmeasurable || !strings.Contains(blind.UnmeasurableReason, "integrated_lufs_missing") {
		t.Fatalf("rows without integrated values must be unmeasurable: %+v", blind)
	}
	if blind.PeakCheck != PeakCheckNoBreachSeen {
		t.Fatalf("peak dimension must still be checked: %+v", blind)
	}

	// ST/M configured but no producer: dimension stays unmeasurable, never faked.
	stMax := -8.0
	mMax := -6.0
	config := Normalize(Config{Enabled: true, ProfileSource: "custom", Custom: &CustomThresholds{
		Integrated:      rlm.LoudnessBand{Min: bandPtr(-23.5), Max: -22.5},
		TruePeakMaxDBTP: -1,
		ShortTermMax:    &stMax,
		MomentaryMax:    &mMax,
	}})
	withST := Compare([]map[string]any{loudnessRow("f", -23)}, config)
	if withST.Verdict != VerdictUnmeasurable {
		t.Fatalf("configured ST/M must force unmeasurable when no producer exists: %+v", withST)
	}
	if len(withST.UnmeasurableDimensions) != 2 || withST.UnmeasurableDimensions[0] != "momentary_max" || withST.UnmeasurableDimensions[1] != "short_term_max" {
		t.Fatalf("unmeasurable dimensions drifted: %+v", withST.UnmeasurableDimensions)
	}

	// Breach dominates: an integrated breach is reported even when ST/M is
	// also configured (the unmeasurable dimensions stay disclosed).
	breached := Compare([]map[string]any{loudnessRow("g", -20)}, config)
	if breached.Verdict != VerdictBreach || len(breached.BreachDetails) == 0 {
		t.Fatalf("breach must dominate: %+v", breached)
	}
	if len(breached.UnmeasurableDimensions) != 2 {
		t.Fatalf("unmeasurable dimensions must stay disclosed on breach: %+v", breached.UnmeasurableDimensions)
	}
}

// Test ⑧ (card acceptance list): the peak dimension is a necessary-condition
// check — sample peak above the TP ceiling breaches; at-or-below yields "no
// breach seen", never a pass.
func TestComparePeakNecessaryCondition(t *testing.T) {
	config := validBuiltinConfig() // TP ceiling -1 dBTP

	over := Compare([]map[string]any{{"clip_id": "h", "integrated_lufs": -23.0, "sample_peak_dbfs": 0.5}}, config)
	if over.Verdict != VerdictBreach || over.PeakCheck != PeakCheckBreach {
		t.Fatalf("sample peak above TP ceiling must breach: %+v", over)
	}
	if len(over.BreachDetails) != 1 || over.BreachDetails[0].Metric != "sample_peak_dbfs" || over.BreachDetails[0].Threshold != -1 {
		t.Fatalf("peak breach detail drifted: %+v", over.BreachDetails)
	}

	under := Compare([]map[string]any{{"clip_id": "i", "integrated_lufs": -23.0, "sample_peak_dbfs": -2.0}}, config)
	if under.PeakCheck != PeakCheckNoBreachSeen {
		t.Fatalf("sample peak at-or-below ceiling must be no_breach_seen, got %q", under.PeakCheck)
	}
	if under.PeakCheck == "pass" || strings.Contains(under.Summary, "peak 合规") {
		t.Fatalf("peak check must never claim a pass: %+v", under)
	}
	if !strings.Contains(under.Summary, "未见 breach") || !strings.Contains(under.Summary, "inter-sample") {
		t.Fatalf("no-breach-seen caveat must be disclosed: %q", under.Summary)
	}
}

// Test ⑨ (card acceptance list): every verdict text carries the
// approximate-grade qualifier (近似口径/approximate).
func TestVerdictTextCarriesApproximateQualifier(t *testing.T) {
	cases := []struct {
		name    string
		verdict Verdict
	}{
		{"pass", Compare([]map[string]any{loudnessRow("j", -23)}, validBuiltinConfig())},
		{"breach", Compare([]map[string]any{loudnessRow("k", -20)}, validBuiltinConfig())},
		{"unmeasurable", Compare(nil, validBuiltinConfig())},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if !strings.Contains(testCase.verdict.Summary, "近似口径") || !strings.Contains(testCase.verdict.Summary, "approximate") {
				t.Fatalf("summary must carry the approximate qualifier: %q", testCase.verdict.Summary)
			}
			if testCase.verdict.MeasurementGrade != MeasurementGradeApproximate {
				t.Fatalf("grade must be forced to approximate, got %q", testCase.verdict.MeasurementGrade)
			}
			if testCase.verdict.SchemaVersion != VerdictSchemaVersion {
				t.Fatalf("verdict schema version drifted: %q", testCase.verdict.SchemaVersion)
			}
		})
	}
}

// Row key compatibility: the integrated fallback chain mirrors the existing
// consumption form (approximate_lufs / lufs / lufs_estimate) and the peak
// row key is the kernel-published peak_dbfs.
func TestCompareConsumesExistingRowKeyShapes(t *testing.T) {
	config := validBuiltinConfig()
	rows := []map[string]any{
		{"clip_id": "approx-form", "approximate_lufs": -23.0},
		{"clip_id": "lufs-form", "lufs": -23.2},
		{"clip_id": "estimate-form", "lufs_estimate": -22.8},
		{"clip_id": "peak-form", "integrated_lufs": -23.0, "peak_dbfs": -3.0},
	}
	verdict := Compare(rows, config)
	if verdict.Verdict != VerdictPass {
		t.Fatalf("all row shapes must be consumed and pass: %+v", verdict)
	}
	if verdict.PeakCheck != PeakCheckNoBreachSeen {
		t.Fatalf("peak_dbfs row key must be consumed: %+v", verdict)
	}
	if len(verdict.Measured) != 4 {
		t.Fatalf("measured must list every row: %+v", verdict.Measured)
	}
	if verdict.ConfigDigest == "" {
		t.Fatal("verdict must pin the config digest")
	}
}
