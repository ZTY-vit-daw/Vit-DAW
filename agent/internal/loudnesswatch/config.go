// Package loudnesswatch implements the project-level loudness watch
// configuration surface (card LOUDNESS-WATCH-CONFIG-1, design file 02 of
// LOUDNESS-WATCH-DESIGN-1): a per-project switch plus threshold vector that
// later cards (TRIGGER/ALERT) compare measured loudness rows against.
//
// Persistence compatibility (AGENTS.md §11):
//   - the config is a new additive field on chat's projectAgentRuntimeState;
//     records without it restore to disabled with zero behavior change;
//   - unknown schema versions and unknown enum values fail closed (dropped
//     and counted at restore, never silently rewritten).
package loudnesswatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"vit-daw-agent/internal/rlm"
)

const (
	// ConfigSchemaVersion is the only accepted config schema version.
	ConfigSchemaVersion = "loudness_watch_config.v0"
	// VerdictSchemaVersion stamps every verdict Compare produces.
	VerdictSchemaVersion = "loudness_watch.v0"

	// TriggerAfterProcessing is the default trigger mode (design 01 §3 main case).
	TriggerAfterProcessing = "after_processing"
	// TriggerPeriodic re-uses the background probe cadence; interval >= 60s.
	TriggerPeriodic = "periodic"
	// TriggerRealtime is accepted for storage; activation reports unavailable
	// because no kernel realtime loudness producer exists (design 01 §3).
	TriggerRealtime = "realtime"

	// ProfileSourceCustom selects the user-defined threshold object.
	ProfileSourceCustom = "custom"

	// RealtimeUnavailableStatus / RealtimeUnavailableReason are the honest
	// capability-gap disclosure for realtime mode (design 01 §3: a missing
	// producer, not a performance trade-off — no pseudo-realtime polling).
	RealtimeUnavailableStatus = "unavailable"
	RealtimeUnavailableReason = "no_realtime_producer"

	// DefaultPeriodicIntervalSeconds applies when periodic mode omits the
	// interval; MinPeriodicIntervalSeconds is the accepted lower bound.
	DefaultPeriodicIntervalSeconds = 300
	MinPeriodicIntervalSeconds     = 60

	// MeasurementGradeApproximate is the only v1 grade: the kernel publishes
	// an RMS-offset approximate integrated loudness, no K-weighting/gating
	// (design 04 §5). Compare never claims bs1770-grade results.
	MeasurementGradeApproximate = "approximate"

	customThresholdsProfileID = "custom:project_watch"
)

// Config is the project-level loudness watch configuration. It is an
// independent schema and deliberately does not ride rlm.RenderProfileBinding
// (that binding face resolves builtin ids only; custom thresholds live only
// inside the watch config, design 02 §2.2).
type Config struct {
	SchemaVersion string `json:"schema_version"`
	Enabled       bool   `json:"enabled"`
	// TriggerMode: after_processing (default) | periodic | realtime.
	TriggerMode string `json:"trigger_mode,omitempty"`
	// PeriodicIntervalSeconds is only used by periodic mode; default 300,
	// accepted lower bound 60.
	PeriodicIntervalSeconds int `json:"periodic_interval_seconds,omitempty"`
	// ProfileSource is "builtin:<id>" (one of the five rlm built-ins) or
	// "custom" (Custom must be present).
	ProfileSource string `json:"profile_source,omitempty"`
	// Custom carries the user-defined thresholds; only read when
	// ProfileSource is custom.
	Custom *CustomThresholds `json:"custom,omitempty"`
}

// CustomThresholds borrows the rlm.DeliveryProfile field shapes (LoudnessBand
// band, ST/M max pointers, true-peak ceiling) so validation can be delegated
// to rlm.ValidateDeliveryProfile instead of a second implementation
// (design 02 §2.2: reuse, not parallel).
type CustomThresholds struct {
	Integrated      rlm.LoudnessBand `json:"integrated_lufs"`
	TruePeakMaxDBTP float64          `json:"true_peak_max_dbtp"`
	ShortTermMax    *float64         `json:"short_term_max_lufs,omitempty"`
	MomentaryMax    *float64         `json:"momentary_max_lufs,omitempty"`
}

// Normalize returns a copy with defaults applied: schema version pinned,
// empty trigger mode -> after_processing, periodic interval 0 -> 300.
func Normalize(config Config) Config {
	config.SchemaVersion = ConfigSchemaVersion
	if strings.TrimSpace(config.TriggerMode) == "" {
		config.TriggerMode = TriggerAfterProcessing
	}
	if config.TriggerMode == TriggerPeriodic && config.PeriodicIntervalSeconds == 0 {
		config.PeriodicIntervalSeconds = DefaultPeriodicIntervalSeconds
	}
	return config
}

// ValidateConfig checks the full config fail-closed: schema version, trigger
// enum, periodic interval lower bound, and profile-source resolution (builtin
// id lookup / custom threshold validation through rlm.ValidateDeliveryProfile).
// Defaults apply before validation (empty trigger mode is after_processing;
// periodic without an interval is 300), so only genuinely invalid records are
// rejected. Error messages carry the field name and offending value, matching
// the rlm profile validation error style.
func ValidateConfig(config Config) error {
	if config.SchemaVersion != ConfigSchemaVersion {
		return fmt.Errorf("loudness watch config: schema version %q is not %q (fail closed)", config.SchemaVersion, ConfigSchemaVersion)
	}
	config = Normalize(config)
	switch config.TriggerMode {
	case TriggerAfterProcessing:
	case TriggerPeriodic:
		if config.PeriodicIntervalSeconds < MinPeriodicIntervalSeconds {
			return fmt.Errorf("loudness watch config: periodic_interval_seconds %d is below the %ds minimum", config.PeriodicIntervalSeconds, MinPeriodicIntervalSeconds)
		}
	case TriggerRealtime:
		// Accepted for storage; activation discloses unavailable/no_realtime_producer.
	default:
		return fmt.Errorf("loudness watch config: unknown trigger_mode %q (fail closed)", config.TriggerMode)
	}
	if _, err := ResolveThresholds(config); err != nil {
		return err
	}
	return nil
}

// ResolveThresholds returns the effective delivery-profile vector for the
// config: a resolved builtin profile, or the custom thresholds re-validated
// through rlm.ValidateDeliveryProfile (band ordering, cross-window ordering,
// true-peak sanity and finiteness are all inherited, design 02 §2.2).
func ResolveThresholds(config Config) (rlm.DeliveryProfile, error) {
	source := strings.TrimSpace(config.ProfileSource)
	switch {
	case strings.HasPrefix(source, "builtin:"):
		profile, ok := rlm.LookupDeliveryProfile(source)
		if !ok {
			return rlm.DeliveryProfile{}, fmt.Errorf("loudness watch config: unknown builtin profile %q (fail closed)", source)
		}
		return profile, nil
	case source == ProfileSourceCustom:
		if config.Custom == nil {
			return rlm.DeliveryProfile{}, errors.New("loudness watch config: profile_source=custom requires the custom threshold object")
		}
		profile := rlm.DeliveryProfile{
			SchemaVersion:        rlm.DeliveryProfileSchemaVersion,
			ProfileID:            customThresholdsProfileID,
			DisplayName:          "loudness watch custom thresholds",
			ChannelConfiguration: rlm.ChannelStereo,
			Integrated:           config.Custom.Integrated,
			ShortTermMax:         config.Custom.ShortTermMax,
			MomentaryMax:         config.Custom.MomentaryMax,
			TruePeakMaxDBTP:      config.Custom.TruePeakMaxDBTP,
			HeadroomStrategy:     rlm.HeadroomTruePeakBound,
		}
		if err := rlm.ValidateDeliveryProfile(profile); err != nil {
			return rlm.DeliveryProfile{}, err
		}
		return profile, nil
	default:
		return rlm.DeliveryProfile{}, fmt.Errorf("loudness watch config: unknown profile_source %q (fail closed)", config.ProfileSource)
	}
}

// ApplyUpdate merges the optional configure-tool arguments onto the current
// config: omitted keys keep their current value (omitting enabled does not
// flip the switch, design 02 §3). Unknown keys and mistyped values are
// rejected with the key name in the error (fail-closed input hygiene). The
// merged result is normalized and validated before it is returned.
func ApplyUpdate(current Config, args map[string]any) (Config, error) {
	next := current
	if next.Custom != nil {
		custom := *next.Custom
		next.Custom = &custom
	}
	for key, value := range args {
		switch key {
		case "enabled":
			enabled, ok := value.(bool)
			if !ok {
				return Config{}, fmt.Errorf("loudness watch config: enabled must be a boolean, got %T (%v)", value, value)
			}
			next.Enabled = enabled
		case "trigger_mode":
			mode, ok := value.(string)
			if !ok {
				return Config{}, fmt.Errorf("loudness watch config: trigger_mode must be a string, got %T (%v)", value, value)
			}
			next.TriggerMode = strings.TrimSpace(mode)
		case "periodic_interval_seconds":
			interval, err := intervalFromArg(value)
			if err != nil {
				return Config{}, err
			}
			next.PeriodicIntervalSeconds = interval
		case "profile_source":
			source, ok := value.(string)
			if !ok {
				return Config{}, fmt.Errorf("loudness watch config: profile_source must be a string, got %T (%v)", value, value)
			}
			next.ProfileSource = strings.TrimSpace(source)
		case "custom":
			custom, err := customFromArg(value)
			if err != nil {
				return Config{}, err
			}
			next.Custom = custom
		default:
			return Config{}, fmt.Errorf("loudness watch config: unknown configure key %q (fail closed)", key)
		}
	}
	next = Normalize(next)
	if err := ValidateConfig(next); err != nil {
		return Config{}, err
	}
	return next, nil
}

func intervalFromArg(value any) (int, error) {
	number, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("loudness watch config: periodic_interval_seconds must be a number, got %T (%v)", value, value)
	}
	interval := int(number)
	if float64(interval) != number {
		return 0, fmt.Errorf("loudness watch config: periodic_interval_seconds must be a whole number, got %v", number)
	}
	return interval, nil
}

func customFromArg(value any) (*CustomThresholds, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("loudness watch config: custom thresholds are not encodable: %w", err)
	}
	custom := CustomThresholds{}
	if err := json.Unmarshal(raw, &custom); err != nil {
		return nil, fmt.Errorf("loudness watch config: custom thresholds do not match the schema: %w", err)
	}
	return &custom, nil
}

// ConfigDigest pins the effective config content (sha256 of the canonical
// marshal form) so verdict records can key dedupe on the exact thresholds
// that produced them (design 01 §2.4).
func ConfigDigest(config Config) string {
	normalized := Normalize(config)
	// Drop the volatile-free fields only: the digest pins what Compare used.
	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
