package executionverifiers

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/orchestration"
)

// PluginEffectControl verifies the governed B4 plug-in seam.  Structural
// readback is supplied by the mutation port; acoustic proof is independently
// recollected and can only pass when quantitative and rendered directional
// evidence agree.
type PluginEffectControl struct {
	Invoker               HarnessInvoker
	PreviousObservationID string
	MixSessionID          string
	GoalText              string
	EvidenceDir           string
}

func (v PluginEffectControl) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	result := orchestration.VerificationResult{Status: "inconclusive", Structural: "inconclusive", Acoustic: "not_run", UserAcceptance: "unknown"}
	if len(receipts) != len(actionSet.Actions) || len(actionSet.Actions) != 1 {
		result.Structural = "fail"
		return result, fmt.Errorf("plugin effect receipt coverage mismatch")
	}
	for _, receipt := range receipts {
		if !strings.EqualFold(receipt.Status, "applied") {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s was not applied", receipt.ActionID)
		}
		if !strings.EqualFold(verifierString(receipt.Details, "structural_readback"), "pass") {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s lacks passing structural readback", receipt.ActionID)
		}
		result.EvidenceRefs = appendUniqueVerifierRefs(result.EvidenceRefs, receipt.EvidenceRefs...)
	}
	result.Structural = "pass"
	if v.Invoker == nil {
		return result, fmt.Errorf("harness invoker is required")
	}
	previous := strings.TrimSpace(v.PreviousObservationID)
	if previous == "" {
		result.Summary = "plugin effect verification requires a previous observation id"
		return result, nil
	}

	response, err := v.Invoker.Invoke(ctx, harness.InvokeRequest{
		Tool: "mix.observe", Args: map[string]any{
			"scope": "full_project", "project_context": true, "observation_only": true,
			"disclosure": "digest_catalog", "mom_intent": mom.IntentProjectMultitrackObservation,
			"previous_observation": previous, "mix_session_id": strings.TrimSpace(v.MixSessionID), "goal_text": strings.TrimSpace(v.GoalText),
		},
		Context: map[string]any{"capability_runtime_v1": true, "observation_only": true, "plugin_effect_control_verifier": true},
		Source:  "capability_runtime_v1_plugin_effect_verifier", Confirmed: true,
		ToolCallID: "verify:" + actionSet.ID + ":mix.observe",
	})
	if err != nil || !strings.EqualFold(response.Status, "ok") {
		result.Summary = "fresh mix.observe failed: " + firstVerifierText(response.Error, verifierError(err), response.Status)
		return result, nil
	}
	afterID := verifierString(response.Result, "observation_id")
	if afterID == "" || afterID == previous {
		result.Acoustic = "fail"
		result.Status = "fail"
		result.Summary = "post-execution observation identity is missing or reused"
		return result, nil
	}
	result.EvidenceRefs = appendUniqueVerifierRefs(result.EvidenceRefs, "mix.observe:"+afterID)

	action := actionSet.Actions[0]
	beforeEvidence := verifierMap(action.Args["before_evidence"])
	beforeObservation := verifierMap(beforeEvidence["observation"])
	beforeProjection := pluginEffectMOMProjection(beforeObservation)
	afterProjection := pluginEffectMOMProjection(response.Result)
	if len(beforeProjection) == 0 || len(afterProjection) == 0 {
		result.Acoustic = "inconclusive"
		result.Summary = "before/after MOM projection is incomplete"
		return result, nil
	}
	trackID := verifierString(action.Args, "track_id")
	frequency, direction, directionNote := pluginEffectExpectedDirection(action)
	if direction == 0 {
		result.Acoustic = "inconclusive"
		result.Summary = "semantic control has no verifiable quantitative direction: " + directionNote
		return result, nil
	}
	band := pluginEffectBandForFrequency(frequency)
	beforeValue, beforeOK := pluginEffectBandEnergy(beforeProjection, trackID, band)
	afterValue, afterOK := pluginEffectBandEnergy(afterProjection, trackID, band)
	if !beforeOK || !afterOK {
		result.Acoustic = "inconclusive"
		result.Summary = fmt.Sprintf("MOM band_occupancy lacks comparable %s energy for track %s", band, trackID)
		return result, nil
	}
	delta := afterValue - beforeValue
	const epsilon = 1e-6
	quantitativePass := (direction < 0 && delta < -epsilon) || (direction > 0 && delta > epsilon)
	if !quantitativePass {
		result.Acoustic = "fail"
		result.Status = "fail"
		result.Summary = fmt.Sprintf("quantitative projection contradicts requested direction for %s: before %.6f after %.6f delta %.6f", band, beforeValue, afterValue, delta)
		return result, nil
	}

	beforeBands := pluginEffectTrackBandEnergies(beforeProjection, trackID)
	afterBands := pluginEffectTrackBandEnergies(afterProjection, trackID)
	if len(beforeBands) == 0 || len(afterBands) == 0 {
		result.Acoustic = "inconclusive"
		result.Summary = "frequency evidence cannot be rendered from MOM band_occupancy"
		return result, nil
	}
	evidenceDir, err := v.pluginEffectEvidenceDir()
	if err != nil {
		result.Acoustic = "inconclusive"
		result.Summary = "cannot create runtime evidence directory: " + err.Error()
		return result, nil
	}
	safeID := pluginEffectSafeFilename(actionSet.ID)
	beforePath := filepath.Join(evidenceDir, safeID+"_before_frequency.png")
	afterPath := filepath.Join(evidenceDir, safeID+"_after_frequency.png")
	if err := writePluginEffectBandPNG(beforePath, beforeBands, band); err != nil {
		result.Acoustic = "inconclusive"
		result.Summary = "cannot render before frequency evidence: " + err.Error()
		return result, nil
	}
	if err := writePluginEffectBandPNG(afterPath, afterBands, band); err != nil {
		result.Acoustic = "inconclusive"
		result.Summary = "cannot render after frequency evidence: " + err.Error()
		return result, nil
	}
	result.EvidenceRefs = appendUniqueVerifierRefs(result.EvidenceRefs, "file:"+beforePath, "file:"+afterPath)
	result.Acoustic = "pass"
	result.Status = "pass"
	result.Summary = fmt.Sprintf("structural readback passed; fresh observation %s changed %s energy in the expected direction (%.6f → %.6f); before/after frequency PNG evidence was rendered; musical acceptance remains unknown", afterID, band, beforeValue, afterValue)
	return result, nil
}

func pluginEffectMOMProjection(result map[string]any) map[string]any {
	projection := verifierMap(result["mom_projection"])
	if len(projection) == 0 {
		projection = verifierMap(verifierMap(result["observation"])["mom_projection"])
	}
	return projection
}

func pluginEffectExpectedDirection(action orchestration.Action) (float64, int, string) {
	apply := verifierMap(action.Args["apply_args"])
	target := verifierMap(apply["target"])
	if len(target) == 0 {
		target = apply
	}
	frequency, _ := pluginEffectVerifierNumber(target, "frequency_hz", "freq_hz", "hz", "center_hz")
	gain, gainOK := pluginEffectVerifierNumber(target, "gain_db", "db", "amount_db")
	control := strings.ToLower(firstVerifierText(verifierString(action.Args, "control"), verifierString(apply, "control", "operation", "name")))
	if gainOK && gain < 0 {
		return frequency, -1, "negative gain"
	}
	if gainOK && gain > 0 {
		return frequency, 1, "positive gain"
	}
	if strings.Contains(control, "cut") || strings.Contains(control, "highpass") || strings.Contains(control, "low_cut") {
		return frequency, -1, control
	}
	if strings.Contains(control, "boost") {
		return frequency, 1, control
	}
	return frequency, 0, control
}

func pluginEffectBandForFrequency(hz float64) string {
	switch {
	case hz <= 0:
		return "bass"
	case hz < 60:
		return "sub"
	case hz < 250:
		return "bass"
	case hz < 500:
		return "low_mid"
	case hz < 2000:
		return "mid"
	case hz < 6000:
		return "high_mid"
	case hz < 12000:
		return "presence"
	default:
		return "air"
	}
}

func pluginEffectBandEnergy(projection map[string]any, trackID, desiredBand string) (float64, bool) {
	values := pluginEffectTrackBandEnergies(projection, trackID)
	if value, ok := values[desiredBand]; ok {
		return value, true
	}
	aliases := map[string][]string{"bass": {"low", "low_end"}, "low_mid": {"low-mid", "lowmid"}, "high_mid": {"high-mid", "highmid"}, "presence": {"treble", "high"}, "air": {"ultra_high"}}
	for _, alias := range aliases[desiredBand] {
		if value, ok := values[alias]; ok {
			return value, true
		}
	}
	return 0, false
}

func pluginEffectTrackBandEnergies(projection map[string]any, trackID string) map[string]float64 {
	relation := verifierMap(projection["multitrack_relation"])
	out := map[string]float64{}
	for _, bandRow := range verifierRows(relation["band_occupancy"]) {
		band := strings.ToLower(strings.TrimSpace(verifierString(bandRow, "band", "name", "id")))
		if band == "" {
			continue
		}
		candidates := append(verifierRows(bandRow["leaders"]), verifierRows(bandRow["tracks"])...)
		for _, row := range candidates {
			if verifierString(row, "track_id", "id") != trackID {
				continue
			}
			if value, ok := pluginEffectVerifierNumber(row, "unit_energy", "energy", "value", "ratio"); ok {
				out[band] = value
				break
			}
		}
	}
	return out
}

func (v PluginEffectControl) pluginEffectEvidenceDir() (string, error) {
	dir := strings.TrimSpace(v.EvidenceDir)
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cache, "VitDAW", "agent", "evidence", "plugin_effect_control")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func writePluginEffectBandPNG(path string, values map[string]float64, highlight string) error {
	order := []string{"sub", "bass", "low_mid", "mid", "high_mid", "presence", "air"}
	img := image.NewRGBA(image.Rect(0, 0, 560, 220))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 18, G: 22, B: 28, A: 255}}, image.Point{}, draw.Src)
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		max = 1
	}
	barW := 52
	gap := 20
	x := 28
	for _, band := range order {
		value := values[band]
		h := int((value / max) * 170)
		if h < 1 && value > 0 {
			h = 1
		}
		c := color.RGBA{R: 64, G: 160, B: 220, A: 255}
		if band == highlight {
			c = color.RGBA{R: 244, G: 166, B: 62, A: 255}
		}
		draw.Draw(img, image.Rect(x, 195-h, x+barW, 195), &image.Uniform{c}, image.Point{}, draw.Src)
		x += barW + gap
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func pluginEffectVerifierNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case float32:
			return float64(typed), true
		case int:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			return parsed, err == nil
		}
	}
	return 0, false
}
func pluginEffectSafeFilename(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "plugin_effect"
	}
	return b.String()
}
func verifierError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

var _ = sort.Strings
