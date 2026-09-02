// Package experimentplugins loads the machine-local experiment plugin
// whitelist used by the D1/D2 free-state experiment channel and validates its
// PCA admission. It only loads and validates; wiring into plan/ports/smoke
// tests belongs to the executing slice.
//
// Schema v2 adds the broadband_compression section beside static_eq; every
// static_eq loading/validation behavior is byte-identical to the v1 loader.
// Schema v3 adds the de_esser section (FAM1-S1): one shared threshold
// parameter instead of the PA-style ch pair, per the 2026-08-31 pluginprobe.
// Schema v4 adds the transient_shaper section (FAM2-S1): one shared attack
// parameter, per the 2026-09-01 pluginprobe. Schema v5 adds the limiter
// section (FAM4-S1): one shared ceiling parameter (FabFilter Pro-L 2,
// pluginprobe 2026-09-02), per the same single-shared-parameter form.
package experimentplugins

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"vit-daw-agent/internal/processorattestation"
)

const SchemaVersion = "vit.free_state_experiment_plugins.v5"

const freeStateExperimentPluginsFileName = "free_state_experiment_plugins.json"

// Band 是 static_eq 域的一个可用 band：中心频点 + 双通道 gain 参数（PA 系插件
// ch1/ch2 为独立参数，一次动作单批同写两通道 = 单 revision 前进）。
type Band struct {
	CenterHz       float64 `json:"center_hz"`
	GainParamIDCH1 string  `json:"gain_param_id_ch1"`
	GainParamIDCH2 string  `json:"gain_param_id_ch2"`
}

type StaticEQPlugin struct {
	PluginName       string `json:"plugin_name"`
	Manufacturer     string `json:"manufacturer"`
	Format           string `json:"format"`
	PluginIdentifier string `json:"plugin_identifier"`
	PluginPath       string `json:"plugin_path"`
	Bands            []Band `json:"bands"`
}

// BroadbandCompressionPlugin 是 broadband_compression 域的白名单插件：双通道
// threshold 参数（PA 系 ch A/B 为独立参数，一次动作单批同写两通道 = 单
// revision 前进）。与 static_eq 的 band 概念不同，这里没有频点维度。
type BroadbandCompressionPlugin struct {
	PluginName          string `json:"plugin_name"`
	Manufacturer        string `json:"manufacturer"`
	Format              string `json:"format"`
	PluginIdentifier    string `json:"plugin_identifier"`
	PluginPath          string `json:"plugin_path"`
	ThresholdParamIDCH1 string `json:"threshold_param_id_ch1"`
	ThresholdParamIDCH2 string `json:"threshold_param_id_ch2"`
}

// DeEsserPlugin 是 de_esser 域的白名单插件：单个共享 threshold 参数（FabFilter
// Pro-DS 2026-08-31 pluginprobe 实测：立体声 in/out 拓扑下 threshold 为全表面
// 唯一一个共享参数，非 ch 对），一次动作单批单通道写。
type DeEsserPlugin struct {
	PluginName       string `json:"plugin_name"`
	Manufacturer     string `json:"manufacturer"`
	Format           string `json:"format"`
	PluginIdentifier string `json:"plugin_identifier"`
	PluginPath       string `json:"plugin_path"`
	ThresholdParamID string `json:"threshold_param_id"`
}

// TransientShaperPlugin 是 transient_shaper 域的白名单插件：单个共享 attack
// 参数（SPL Transient Designer Plus 2026-09-01 pluginprobe 实测：全表面 13
// 参数中 attack/sustain 均为单共享连续 dB 参数，非 ch 对，Link 默认 On），
// 一次动作单批单通道写。
type TransientShaperPlugin struct {
	PluginName       string `json:"plugin_name"`
	Manufacturer     string `json:"manufacturer"`
	Format           string `json:"format"`
	PluginIdentifier string `json:"plugin_identifier"`
	PluginPath       string `json:"plugin_path"`
	AttackParamID    string `json:"attack_param_id"`
}

// LimiterPlugin is the limiter domain's whitelisted plugin: a single shared ceiling parameter
// (FabFilter Pro-L 2 2026-09-02 pluginprobe actual test: among all 32 parameters on the surface, the Output Level (ceiling) is a single shared continuous dBTP parameter,
// not a ch pair), one action per single-batch single-channel write.
type LimiterPlugin struct {
	PluginName       string `json:"plugin_name"`
	Manufacturer     string `json:"manufacturer"`
	Format           string `json:"format"`
	PluginIdentifier string `json:"plugin_identifier"`
	PluginPath       string `json:"plugin_path"`
	CeilingParamID   string `json:"ceiling_param_id"`
}

// GateExpanderPlugin 是 gate_expander 域的白名单插件：单个共享 range（attenuation
// floor）参数（FabFilter Pro-G 2026-09-02 pluginprobe 实测：全表面 35 实参中
// Range 为单共享连续 dB 参数，非 ch 对），一次动作单批单通道写。
type GateExpanderPlugin struct {
	PluginName       string `json:"plugin_name"`
	Manufacturer     string `json:"manufacturer"`
	Format           string `json:"format"`
	PluginIdentifier string `json:"plugin_identifier"`
	PluginPath       string `json:"plugin_path"`
	RangeParamID     string `json:"range_param_id"`
}

type Whitelist struct {
	SchemaVersion        string                      `json:"schema_version"`
	StaticEQ             *StaticEQPlugin             `json:"static_eq,omitempty"`
	BroadbandCompression *BroadbandCompressionPlugin `json:"broadband_compression,omitempty"`
	DeEsser              *DeEsserPlugin              `json:"de_esser,omitempty"`
	TransientShaper      *TransientShaperPlugin      `json:"transient_shaper,omitempty"`
	Limiter              *LimiterPlugin              `json:"limiter,omitempty"`
	GateExpander         *GateExpanderPlugin          `json:"gate_expander,omitempty"`
}

var ErrNotConfigured = errors.New("experiment plugin whitelist: static_eq plugin is not configured")

var ErrCompressionNotConfigured = errors.New("experiment plugin whitelist: broadband_compression plugin is not configured")

var ErrDeEsserNotConfigured = errors.New("experiment plugin whitelist: de_esser plugin is not configured")

var ErrTransientShaperNotConfigured = errors.New("experiment plugin whitelist: transient_shaper plugin is not configured")

var ErrLimiterNotConfigured = errors.New("experiment plugin whitelist: limiter plugin is not configured")

var ErrGateExpanderNotConfigured = errors.New("experiment plugin whitelist: gate_expander plugin is not configured")

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("experiment plugin whitelist: user home: %w", err)
	}
	return filepath.Join(home, ".vit", freeStateExperimentPluginsFileName), nil
}

func Load(path string) (Whitelist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: %s does not exist: %w", path, ErrNotConfigured)
		}
		return Whitelist{}, fmt.Errorf("experiment plugin whitelist: read %s: %w", path, err)
	}
	var whitelist Whitelist
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&whitelist); err != nil {
		return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: trailing JSON content", path)
	}
	if whitelist.SchemaVersion != SchemaVersion {
		return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: schema_version must be %q but got %q", path, SchemaVersion, whitelist.SchemaVersion)
	}
	if whitelist.StaticEQ != nil {
		if err := validateStaticEQPlugin(*whitelist.StaticEQ); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	if whitelist.BroadbandCompression != nil {
		if err := validateBroadbandCompressionPlugin(*whitelist.BroadbandCompression); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	if whitelist.DeEsser != nil {
		if err := validateDeEsserPlugin(*whitelist.DeEsser); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	if whitelist.TransientShaper != nil {
		if err := validateTransientShaperPlugin(*whitelist.TransientShaper); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	if whitelist.Limiter != nil {
		if err := validateLimiterPlugin(*whitelist.Limiter); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	if whitelist.GateExpander != nil {
		if err := validateGateExpanderPlugin(*whitelist.GateExpander); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	return whitelist, nil
}

func validateStaticEQPlugin(plugin StaticEQPlugin) error {
	for _, missing := range []struct{ field, value string }{
		{"plugin_name", plugin.PluginName},
		{"manufacturer", plugin.Manufacturer},
		{"format", plugin.Format},
		{"plugin_identifier", plugin.PluginIdentifier},
		{"plugin_path", plugin.PluginPath},
	} {
		if missing.value == "" {
			return fmt.Errorf("static_eq %s must be non-empty", missing.field)
		}
	}
	if len(plugin.Bands) < 1 {
		return fmt.Errorf("static_eq bands must contain at least one band")
	}
	seenCH1 := map[string]bool{}
	previousHz := math.Inf(-1)
	for index, band := range plugin.Bands {
		if !(band.CenterHz >= 20 && band.CenterHz <= 20000) {
			return fmt.Errorf("static_eq band %d center_hz %v outside [20, 20000]", index, band.CenterHz)
		}
		if band.GainParamIDCH1 == "" {
			return fmt.Errorf("static_eq band %d gain_param_id_ch1 must be non-empty", index)
		}
		if band.GainParamIDCH2 == "" {
			return fmt.Errorf("static_eq band %d gain_param_id_ch2 must be non-empty", index)
		}
		if seenCH1[band.GainParamIDCH1] {
			return fmt.Errorf("static_eq band %d duplicates gain_param_id_ch1 %q", index, band.GainParamIDCH1)
		}
		seenCH1[band.GainParamIDCH1] = true
		if band.CenterHz <= previousHz {
			return fmt.Errorf("static_eq band %d center_hz %v must be strictly greater than previous %v", index, band.CenterHz, previousHz)
		}
		previousHz = band.CenterHz
	}
	return nil
}

func (w Whitelist) NearestStaticEQBand(frequencyHz float64) (Band, error) {
	if w.StaticEQ == nil {
		return Band{}, ErrNotConfigured
	}
	best := w.StaticEQ.Bands[0]
	bestDistance := math.Abs(best.CenterHz - frequencyHz)
	for _, band := range w.StaticEQ.Bands[1:] {
		distance := math.Abs(band.CenterHz - frequencyHz)
		if distance < bestDistance || (distance == bestDistance && band.CenterHz < best.CenterHz) {
			best = band
			bestDistance = distance
		}
	}
	return best, nil
}

func (w Whitelist) ValidateStaticEQAdmission(lib processorattestation.Library) error {
	if w.StaticEQ == nil {
		return ErrNotConfigured
	}
	return w.validateSectionAdmission(
		"static_eq",
		w.StaticEQ.PluginName, w.StaticEQ.Manufacturer, w.StaticEQ.Format, w.StaticEQ.PluginIdentifier, w.StaticEQ.PluginPath,
		processorattestation.FamilyStaticEQ, lib,
	)
}

// BroadbandThresholdParams returns the whitelisted dual-channel threshold
// parameter ids for one broadband_compression execution.
func (w Whitelist) BroadbandThresholdParams() (thresholdCH1, thresholdCH2 string, err error) {
	if w.BroadbandCompression == nil {
		return "", "", ErrCompressionNotConfigured
	}
	return w.BroadbandCompression.ThresholdParamIDCH1, w.BroadbandCompression.ThresholdParamIDCH2, nil
}

// ValidateCompressionAdmission mirrors ValidateStaticEQAdmission for the
// broadband_compression whitelist section: same subject construction, same
// v1 library predicate family path, and a distinguishable not-PCA-promoted
// prefix naming this domain.
func (w Whitelist) ValidateCompressionAdmission(lib processorattestation.Library) error {
	if w.BroadbandCompression == nil {
		return ErrCompressionNotConfigured
	}
	return w.validateSectionAdmission(
		"broadband_compression",
		w.BroadbandCompression.PluginName, w.BroadbandCompression.Manufacturer, w.BroadbandCompression.Format, w.BroadbandCompression.PluginIdentifier, w.BroadbandCompression.PluginPath,
		processorattestation.FamilyBroadbandCompressor, lib,
	)
}

// ValidateDeEsserAdmission mirrors ValidateCompressionAdmission for the
// de_esser whitelist section. de_esser is a PCA v2 family, so per the GLM
// ruling on D2-FAM1-S1 (Form A dispatch) this predicate takes the v2
// attestation library; the subject construction and boundary wording stay
// identical to the v1 sections.
func (w Whitelist) ValidateDeEsserAdmission(lib processorattestation.LibraryV2) error {
	if w.DeEsser == nil {
		return ErrDeEsserNotConfigured
	}
	return w.validateSectionAdmissionV2(
		"de_esser",
		w.DeEsser.PluginName, w.DeEsser.Manufacturer, w.DeEsser.Format, w.DeEsser.PluginIdentifier, w.DeEsser.PluginPath,
		processorattestation.FamilyDeEsser, lib,
	)
}

// ValidateTransientShaperAdmission mirrors ValidateDeEsserAdmission for the
// transient_shaper whitelist section: same subject construction, same v2
// library predicate path (transient_shaper is a PCA v2 family), and a
// distinguishable not-PCA-promoted prefix naming this domain.
func (w Whitelist) ValidateTransientShaperAdmission(lib processorattestation.LibraryV2) error {
	if w.TransientShaper == nil {
		return ErrTransientShaperNotConfigured
	}
	return w.validateSectionAdmissionV2(
		"transient_shaper",
		w.TransientShaper.PluginName, w.TransientShaper.Manufacturer, w.TransientShaper.Format, w.TransientShaper.PluginIdentifier, w.TransientShaper.PluginPath,
		processorattestation.FamilyTransient, lib,
	)
}

// ValidateLimiterAdmission mirrors ValidateTransientShaperAdmission for the
// limiter whitelist section: same subject construction, same v2 library
// predicate path (limiter is a PCA v2 family), and a distinguishable
// not-PCA-promoted prefix naming this domain.
func (w Whitelist) ValidateLimiterAdmission(lib processorattestation.LibraryV2) error {
	if w.Limiter == nil {
		return ErrLimiterNotConfigured
	}
	return w.validateSectionAdmissionV2(
		"limiter",
		w.Limiter.PluginName, w.Limiter.Manufacturer, w.Limiter.Format, w.Limiter.PluginIdentifier, w.Limiter.PluginPath,
		processorattestation.FamilyLimiter, lib,
	)
}

// ValidateGateExpanderAdmission mirrors ValidateLimiterAdmission for the
// gate_expander whitelist section: same subject construction, same v2 library
// predicate path (gate_expander is a PCA v2 family), and a distinguishable
// not-PCA-promoted prefix naming this domain.
func (w Whitelist) ValidateGateExpanderAdmission(lib processorattestation.LibraryV2) error {
	if w.GateExpander == nil {
		return ErrGateExpanderNotConfigured
	}
	return w.validateSectionAdmissionV2(
		"gate_expander",
		w.GateExpander.PluginName, w.GateExpander.Manufacturer, w.GateExpander.Format, w.GateExpander.PluginIdentifier, w.GateExpander.PluginPath,
		processorattestation.FamilyGateExpander, lib,
	)
}

// validateSectionAdmission is the shared admission predicate core for both
// whitelist sections. The label appears verbatim in the returned boundary
// prefix so each domain stays distinguishable upstream.
func (w Whitelist) validateSectionAdmission(label, pluginName, manufacturer, formatV, identifier, pluginPath string, family string, lib processorattestation.Library) error {
	subject := processorattestation.Subject{
		Name:          pluginName,
		Manufacturer:  manufacturer,
		Format:        formatV,
		Identifier:    identifier,
		InstalledPath: pluginPath,
	}
	key, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: %s plugin %q: %w", label, pluginName, err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: %s plugin %q: %w", label, pluginName, err)
	}
	result, err := processorattestation.QueryLibraryAdmission(lib, key, fingerprint, family)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: %s plugin %q: %w", label, pluginName, err)
	}
	if !result.Eligible {
		return fmt.Errorf("experiment plugin whitelist: %s plugin is not PCA-promoted: plugin %q reason %q", label, pluginName, result.Reason)
	}
	return nil
}

// validateSectionAdmissionV2 is the v2-generation twin of
// validateSectionAdmission: identical boundary wording and identical subject
// and fingerprint construction, differing only in the attestation library
// generation it queries (GLM ruling on D2-FAM1-S1: Form A dispatch keeps the
// predicates generation-typed and the callers store-choosing).
func (w Whitelist) validateSectionAdmissionV2(label, pluginName, manufacturer, formatV, identifier, pluginPath string, family string, lib processorattestation.LibraryV2) error {
	subject := processorattestation.Subject{
		Name:          pluginName,
		Manufacturer:  manufacturer,
		Format:        formatV,
		Identifier:    identifier,
		InstalledPath: pluginPath,
	}
	key, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: %s plugin %q: %w", label, pluginName, err)
	}
	fingerprint, err := processorattestation.FingerprintPath(pluginPath)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: %s plugin %q: %w", label, pluginName, err)
	}
	result, err := processorattestation.QueryLibraryAdmissionV2(lib, key, fingerprint, family)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: %s plugin %q: %w", label, pluginName, err)
	}
	if !result.Eligible {
		return fmt.Errorf("experiment plugin whitelist: %s plugin is not PCA-promoted: plugin %q reason %q", label, pluginName, result.Reason)
	}
	return nil
}

func validateBroadbandCompressionPlugin(plugin BroadbandCompressionPlugin) error {
	for _, missing := range []struct{ field, value string }{
		{"plugin_name", plugin.PluginName},
		{"manufacturer", plugin.Manufacturer},
		{"format", plugin.Format},
		{"plugin_identifier", plugin.PluginIdentifier},
		{"plugin_path", plugin.PluginPath},
		{"threshold_param_id_ch1", plugin.ThresholdParamIDCH1},
		{"threshold_param_id_ch2", plugin.ThresholdParamIDCH2},
	} {
		if missing.value == "" {
			return fmt.Errorf("broadband_compression %s must be non-empty", missing.field)
		}
	}
	if plugin.ThresholdParamIDCH1 == plugin.ThresholdParamIDCH2 {
		return fmt.Errorf("broadband_compression threshold_param_id_ch1 must differ from threshold_param_id_ch2")
	}
	return nil
}

func validateDeEsserPlugin(plugin DeEsserPlugin) error {
	for _, missing := range []struct{ field, value string }{
		{"plugin_name", plugin.PluginName},
		{"manufacturer", plugin.Manufacturer},
		{"format", plugin.Format},
		{"plugin_identifier", plugin.PluginIdentifier},
		{"plugin_path", plugin.PluginPath},
		{"threshold_param_id", plugin.ThresholdParamID},
	} {
		if missing.value == "" {
			return fmt.Errorf("de_esser %s must be non-empty", missing.field)
		}
	}
	return nil
}

func validateGateExpanderPlugin(plugin GateExpanderPlugin) error {
	for _, missing := range []struct{ field, value string }{
		{"plugin_name", plugin.PluginName},
		{"manufacturer", plugin.Manufacturer},
		{"format", plugin.Format},
		{"plugin_identifier", plugin.PluginIdentifier},
		{"plugin_path", plugin.PluginPath},
		{"range_param_id", plugin.RangeParamID},
	} {
		if missing.value == "" {
			return fmt.Errorf("gate_expander %s must be non-empty", missing.field)
		}
	}
	return nil
}

func validateLimiterPlugin(plugin LimiterPlugin) error {
	for _, missing := range []struct{ field, value string }{
		{"plugin_name", plugin.PluginName},
		{"manufacturer", plugin.Manufacturer},
		{"format", plugin.Format},
		{"plugin_identifier", plugin.PluginIdentifier},
		{"plugin_path", plugin.PluginPath},
		{"ceiling_param_id", plugin.CeilingParamID},
	} {
		if missing.value == "" {
			return fmt.Errorf("limiter %s must be non-empty", missing.field)
		}
	}
	return nil
}

func validateTransientShaperPlugin(plugin TransientShaperPlugin) error {
	for _, missing := range []struct{ field, value string }{
		{"plugin_name", plugin.PluginName},
		{"manufacturer", plugin.Manufacturer},
		{"format", plugin.Format},
		{"plugin_identifier", plugin.PluginIdentifier},
		{"plugin_path", plugin.PluginPath},
		{"attack_param_id", plugin.AttackParamID},
	} {
		if missing.value == "" {
			return fmt.Errorf("transient_shaper %s must be non-empty", missing.field)
		}
	}
	return nil
}
