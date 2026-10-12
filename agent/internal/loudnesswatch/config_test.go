package loudnesswatch

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"vit-daw-agent/internal/rlm"
)

func bandPtr(value float64) *float64 { return &value }

func validBuiltinConfig() Config {
	return Normalize(Config{Enabled: true, ProfileSource: "builtin:ebu_r128"})
}

func TestValidateAcceptsAllFiveBuiltinProfiles(t *testing.T) {
	for _, id := range []string{"builtin:ebu_r128", "builtin:aes_td1004", "builtin:apple_music", "builtin:spotify", "builtin:gy_282_2014"} {
		config := Normalize(Config{Enabled: true, ProfileSource: id})
		if err := ValidateConfig(config); err != nil {
			t.Fatalf("builtin %q must validate: %v", id, err)
		}
		thresholds, err := ResolveThresholds(config)
		if err != nil {
			t.Fatalf("builtin %q must resolve: %v", id, err)
		}
		if thresholds.ProfileID != id {
			t.Fatalf("resolved profile id = %q want %q", thresholds.ProfileID, id)
		}
	}
}

// Test ④ (card acceptance list): an unknown builtin id fails closed.
func TestUnknownBuiltinIDFailsClosed(t *testing.T) {
	config := Normalize(Config{Enabled: true, ProfileSource: "builtin:no_such_profile"})
	err := ValidateConfig(config)
	if err == nil {
		t.Fatal("unknown builtin id must be rejected")
	}
	if !strings.Contains(err.Error(), "builtin:no_such_profile") {
		t.Fatalf("error must name the offending id, got: %v", err)
	}
	if _, resolveErr := ResolveThresholds(config); resolveErr == nil {
		t.Fatal("ResolveThresholds must fail closed on unknown builtin id")
	}
}

// Test ⑤ (card acceptance list): custom threshold violations are rejected one
// by one through the borrowed rlm validation (band order, cross-window order,
// true-peak sanity), with the field name and value in the message.
func TestCustomThresholdViolationsRejected(t *testing.T) {
	cases := []struct {
		name       string
		custom     CustomThresholds
		wantSubstr string
	}{
		{
			name:       "band min above max",
			custom:     CustomThresholds{Integrated: rlmBand(-22.5, -23.5), TruePeakMaxDBTP: -1},
			wantSubstr: "min",
		},
		{
			name: "short-term max above momentary max",
			custom: CustomThresholds{
				Integrated:      rlmBand(-23.5, -22.5),
				TruePeakMaxDBTP: -1,
				ShortTermMax:    bandPtr(-8),
				MomentaryMax:    bandPtr(-10),
			},
			wantSubstr: "momentary max",
		},
		{
			name:       "true peak above full scale",
			custom:     CustomThresholds{Integrated: rlmBand(-23.5, -22.5), TruePeakMaxDBTP: 3},
			wantSubstr: "true peak max",
		},
		{
			name:       "non-finite integrated max",
			custom:     CustomThresholds{Integrated: rlmBand(-23.5, math.Inf(1)), TruePeakMaxDBTP: -1},
			wantSubstr: "finite",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			config := Normalize(Config{Enabled: true, ProfileSource: "custom", Custom: cloneCustom(&testCase.custom)})
			err := ValidateConfig(config)
			if err == nil {
				t.Fatalf("custom violation must be rejected: %+v", testCase.custom)
			}
			if !strings.Contains(err.Error(), testCase.wantSubstr) {
				t.Fatalf("error %q must mention %q", err, testCase.wantSubstr)
			}
		})
	}
}

func TestCustomWithoutObjectRejected(t *testing.T) {
	config := Normalize(Config{Enabled: true, ProfileSource: "custom"})
	if err := ValidateConfig(config); err == nil {
		t.Fatal("profile_source=custom without the custom object must be rejected")
	}
}

// Test ⑥ (card acceptance list): realtime is accepted for storage and the
// honest unavailable disclosure constants exist; the round trip does not
// rewrite the user's choice.
func TestRealtimeStoredNotActivated(t *testing.T) {
	config := Normalize(Config{Enabled: true, TriggerMode: TriggerRealtime, ProfileSource: "builtin:spotify"})
	if err := ValidateConfig(config); err != nil {
		t.Fatalf("realtime must be accepted for storage: %v", err)
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	decoded := Config{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Enabled || decoded.TriggerMode != TriggerRealtime {
		t.Fatalf("realtime round trip lost the user choice: %+v", decoded)
	}
	if RealtimeUnavailableStatus != "unavailable" || RealtimeUnavailableReason != "no_realtime_producer" {
		t.Fatalf("realtime disclosure constants drifted: %q/%q", RealtimeUnavailableStatus, RealtimeUnavailableReason)
	}
}

// Test (design 02 §2.3): periodic interval defaults to 300 when omitted and
// values below 60 are rejected.
func TestPeriodicIntervalDefaultAndFloor(t *testing.T) {
	config := Normalize(Config{Enabled: true, TriggerMode: TriggerPeriodic, ProfileSource: "builtin:ebu_r128"})
	if config.PeriodicIntervalSeconds != DefaultPeriodicIntervalSeconds {
		t.Fatalf("omitted periodic interval = %d want %d", config.PeriodicIntervalSeconds, DefaultPeriodicIntervalSeconds)
	}
	if err := ValidateConfig(config); err != nil {
		t.Fatalf("default interval must validate: %v", err)
	}
	tooFast := Normalize(Config{Enabled: true, TriggerMode: TriggerPeriodic, PeriodicIntervalSeconds: 59, ProfileSource: "builtin:ebu_r128"})
	if err := ValidateConfig(tooFast); err == nil {
		t.Fatal("periodic interval 59 must be rejected")
	}
	atFloor := Normalize(Config{Enabled: true, TriggerMode: TriggerPeriodic, PeriodicIntervalSeconds: 60, ProfileSource: "builtin:ebu_r128"})
	if err := ValidateConfig(atFloor); err != nil {
		t.Fatalf("periodic interval 60 must validate: %v", err)
	}
}

// Test ③ support (package half): bad schema versions and unknown enums fail
// closed with the offending value named.
func TestBadSchemaVersionAndUnknownEnumsFailClosed(t *testing.T) {
	badSchema := Normalize(Config{Enabled: true, ProfileSource: "builtin:ebu_r128"})
	badSchema.SchemaVersion = "loudness_watch_config.v9"
	if err := ValidateConfig(badSchema); err == nil || !strings.Contains(err.Error(), "loudness_watch_config.v9") {
		t.Fatalf("bad schema version must fail closed naming the value, got %v", err)
	}
	badTrigger := Normalize(Config{Enabled: true, TriggerMode: "every_render", ProfileSource: "builtin:ebu_r128"})
	if err := ValidateConfig(badTrigger); err == nil || !strings.Contains(err.Error(), "every_render") {
		t.Fatalf("unknown trigger_mode must fail closed naming the value, got %v", err)
	}
	badSource := Normalize(Config{Enabled: true, ProfileSource: "user_guess"})
	if err := ValidateConfig(badSource); err == nil || !strings.Contains(err.Error(), "user_guess") {
		t.Fatalf("unknown profile_source must fail closed naming the value, got %v", err)
	}
}

// Test ② support (package half): full config with custom pointer thresholds
// survives a JSON round trip with pointer semantics intact (nil stays nil,
// set stays set) and ApplyUpdate merges only the provided keys.
func TestApplyUpdateMergeAndPointerRoundTrip(t *testing.T) {
	current := validBuiltinConfig()
	updated, err := ApplyUpdate(current, map[string]any{"enabled": false})
	if err != nil {
		t.Fatalf("enabled-only update rejected: %v", err)
	}
	if updated.Enabled || updated.ProfileSource != "builtin:ebu_r128" {
		t.Fatalf("omitted keys must keep current values: %+v", updated)
	}

	minLUFS := -23.5
	maxLUFS := -22.5
	stMax := -8.0
	custom := CustomThresholds{
		Integrated:      rlm.LoudnessBand{Min: &minLUFS, Max: maxLUFS},
		TruePeakMaxDBTP: -1,
		ShortTermMax:    &stMax,
	}
	updated, err = ApplyUpdate(current, map[string]any{
		"trigger_mode":   "periodic",
		"profile_source": "custom",
		"custom":         custom,
	})
	if err != nil {
		t.Fatalf("custom update rejected: %v", err)
	}
	if updated.PeriodicIntervalSeconds != DefaultPeriodicIntervalSeconds {
		t.Fatalf("periodic default interval = %d want %d", updated.PeriodicIntervalSeconds, DefaultPeriodicIntervalSeconds)
	}
	data, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	decoded := Config{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(updated, decoded) {
		t.Fatalf("round trip not equal:\n sent: %+v\n got:  %+v", updated, decoded)
	}
	if decoded.Custom == nil || decoded.Custom.Integrated.Min == nil || *decoded.Custom.Integrated.Min != minLUFS {
		t.Fatalf("custom integrated min pointer lost: %+v", decoded.Custom)
	}
	if decoded.Custom.ShortTermMax == nil || *decoded.Custom.ShortTermMax != stMax {
		t.Fatalf("custom short-term pointer lost: %+v", decoded.Custom)
	}
	if decoded.Custom.MomentaryMax != nil {
		t.Fatalf("nil momentary max must stay nil: %+v", decoded.Custom)
	}

	if _, err := ApplyUpdate(current, map[string]any{"enabeld": true}); err == nil || !strings.Contains(err.Error(), "enabeld") {
		t.Fatalf("unknown key must be rejected naming it, got %v", err)
	}
	if _, err := ApplyUpdate(current, map[string]any{"enabled": "yes"}); err == nil {
		t.Fatal("mistyped enabled must be rejected")
	}
	if _, err := ApplyUpdate(current, map[string]any{"periodic_interval_seconds": 12.5}); err == nil {
		t.Fatal("fractional interval must be rejected")
	}
}

// The config digest must be stable for identical configs and sensitive to
// threshold changes (verdict records key dedupe on it, design 01 §2.4).
func TestConfigDigestStableAndContentSensitive(t *testing.T) {
	first := ConfigDigest(validBuiltinConfig())
	second := ConfigDigest(validBuiltinConfig())
	if first == "" || first != second {
		t.Fatalf("digest must be stable, got %q vs %q", first, second)
	}
	other := ConfigDigest(Normalize(Config{Enabled: true, ProfileSource: "builtin:spotify"}))
	if other == first {
		t.Fatal("digest must be content sensitive")
	}
}

func rlmBand(min, max float64) rlm.LoudnessBand {
	return rlm.LoudnessBand{Min: bandPtr(min), Max: max}
}

func cloneCustom(custom *CustomThresholds) *CustomThresholds {
	if custom == nil {
		return nil
	}
	cloned := *custom
	return &cloned
}
