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
// pluginprobe 2026-09-02), per the same single-shared-parameter form; the
// same v5 revision then gained the gate_expander section (FAM5-S1) and the
// multiband section (FAM6-S1): a per-band threshold id list (Lindell MBC,
// pluginprobe 2026-09-02), per the same reuse-one-version coordination.
//
// Schema v6 (FIX-PLUGIN-SELECT-1) evolves every family value from one object
// to a candidate entry array — the entry shape is identical to v5
// field-for-field. The loader accepts v5 (a single object normalizes to a
// single-element list, preserving the historical single-candidate behavior)
// and v6, and fails closed on every other schema_version. Duplicate
// plugin_identifier values inside one family are corrupt: admission
// membership must always resolve exactly one entry.
//
// FIX-BROADBAND-SHARED-1 (PORT-PCA-FULL-CANDIDATES-1 decision A) widens only
// the broadband_compression entry: beside the historical dual ch1/ch2 pair it
// also accepts the single shared threshold_param_id form (field name aligned
// with the de_esser v5 precedent; mac-side Waves comps expose one shared
// Threshold). The two forms are mutually exclusive — both pinned or neither
// pinned fails closed.
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
	"strings"

	"vit-daw-agent/internal/processorattestation"
)

// SchemaVersion is the canonical in-memory (and v6 wire) schema version: a
// loaded whitelist always reports this version regardless of whether it came
// from a v5 or a v6 file.
const SchemaVersion = "vit.free_state_experiment_plugins.v6"

// SchemaVersionV5 is the legacy single-object-per-family wire form the loader
// still accepts (each configured family normalizes to a one-element list).
const SchemaVersionV5 = "vit.free_state_experiment_plugins.v5"

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

// BroadbandCompressionPlugin 是 broadband_compression 域的白名单条目，两种
// 互斥形态（FIX-BROADBAND-SHARED-1，决策点 A）：双通道 threshold 参数对
// （threshold_param_id_ch1/ch2，PA 系 ch A/B 为独立参数，一次动作单批同写
// 两通道 = 单 revision 前进）或单个共享 threshold 参数
// （threshold_param_id，字段名与 de_esser 对齐；mac 12 个 Waves comp 实测
// 全为单共享 Threshold），一次动作单条目批写（FAM1-S1 语义）。两种形态都
// 缺或都有 fail-closed。与 static_eq 的 band 概念不同，这里没有频点维度。
type BroadbandCompressionPlugin struct {
	PluginName          string `json:"plugin_name"`
	Manufacturer        string `json:"manufacturer"`
	Format              string `json:"format"`
	PluginIdentifier    string `json:"plugin_identifier"`
	PluginPath          string `json:"plugin_path"`
	ThresholdParamID    string `json:"threshold_param_id,omitempty"`
	ThresholdParamIDCH1 string `json:"threshold_param_id_ch1,omitempty"`
	ThresholdParamIDCH2 string `json:"threshold_param_id_ch2,omitempty"`
}

// ThresholdParamPair returns the entry's normalized write shape: the shared
// parameter id with an empty ch2 for single-form entries (one-entry batch
// write, FAM1-S1 semantics), or the ch1/ch2 pair for dual-form entries. Load
// validation guarantees exactly one form is pinned, so the empty-shared
// discriminator is total.
func (p BroadbandCompressionPlugin) ThresholdParamPair() (paramID, paramIDCH2 string) {
	if p.ThresholdParamID != "" {
		return p.ThresholdParamID, ""
	}
	return p.ThresholdParamIDCH1, p.ThresholdParamIDCH2
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

// MultibandPlugin is the multiband domain's whitelisted plugin: a per-band
// threshold surface (Lindell MBC 2026-09-02 pluginprobe actual test: all 47
// parameters carry no ch pair; Low/Mid/High each expose their own continuous
// Threshold parameter, so the whitelist pins one threshold id per band and
// the admission's band_index picks one for a single-batch single-channel
// write).
type MultibandPlugin struct {
	PluginName            string   `json:"plugin_name"`
	Manufacturer          string   `json:"manufacturer"`
	Format                string   `json:"format"`
	PluginIdentifier      string   `json:"plugin_identifier"`
	PluginPath            string   `json:"plugin_path"`
	BandThresholdParamIDs []string `json:"band_threshold_param_ids"`
}

// StaticEQPlugins is the v6 static_eq candidate list. Its decoder accepts the
// v6 array form and the v5 single-object form (one-element list) with the
// same unknown-field strictness the v5 loader enforced.
type StaticEQPlugins []StaticEQPlugin

// BroadbandCompressionPlugins is the v6 broadband_compression candidate list.
type BroadbandCompressionPlugins []BroadbandCompressionPlugin

// DeEsserPlugins is the v6 de_esser candidate list.
type DeEsserPlugins []DeEsserPlugin

// TransientShaperPlugins is the v6 transient_shaper candidate list.
type TransientShaperPlugins []TransientShaperPlugin

// LimiterPlugins is the v6 limiter candidate list.
type LimiterPlugins []LimiterPlugin

// GateExpanderPlugins is the v6 gate_expander candidate list.
type GateExpanderPlugins []GateExpanderPlugin

// MultibandPlugins is the v6 multiband candidate list.
type MultibandPlugins []MultibandPlugin

func (p *StaticEQPlugins) UnmarshalJSON(data []byte) error {
	entries, err := unmarshalFamilyEntries[StaticEQPlugin](data)
	*p = entries
	return err
}

func (p *BroadbandCompressionPlugins) UnmarshalJSON(data []byte) error {
	entries, err := unmarshalFamilyEntries[BroadbandCompressionPlugin](data)
	*p = entries
	return err
}

func (p *DeEsserPlugins) UnmarshalJSON(data []byte) error {
	entries, err := unmarshalFamilyEntries[DeEsserPlugin](data)
	*p = entries
	return err
}

func (p *TransientShaperPlugins) UnmarshalJSON(data []byte) error {
	entries, err := unmarshalFamilyEntries[TransientShaperPlugin](data)
	*p = entries
	return err
}

func (p *LimiterPlugins) UnmarshalJSON(data []byte) error {
	entries, err := unmarshalFamilyEntries[LimiterPlugin](data)
	*p = entries
	return err
}

func (p *GateExpanderPlugins) UnmarshalJSON(data []byte) error {
	entries, err := unmarshalFamilyEntries[GateExpanderPlugin](data)
	*p = entries
	return err
}

func (p *MultibandPlugins) UnmarshalJSON(data []byte) error {
	entries, err := unmarshalFamilyEntries[MultibandPlugin](data)
	*p = entries
	return err
}

// unmarshalFamilyEntries decodes one whitelist family in either wire form:
// the v6 entry array or the v5 single object (normalized to a one-element
// list). null decodes to a nil list (section absent). Unknown fields stay
// rejected inside every entry.
func unmarshalFamilyEntries[T any](data []byte) ([]T, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var entries []T
		if err := decoder.Decode(&entries); err != nil {
			return nil, err
		}
		return entries, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var single T
	if err := decoder.Decode(&single); err != nil {
		return nil, err
	}
	return []T{single}, nil
}

type Whitelist struct {
	SchemaVersion        string                      `json:"schema_version"`
	StaticEQ             StaticEQPlugins             `json:"static_eq,omitempty"`
	BroadbandCompression BroadbandCompressionPlugins `json:"broadband_compression,omitempty"`
	DeEsser              DeEsserPlugins              `json:"de_esser,omitempty"`
	TransientShaper      TransientShaperPlugins      `json:"transient_shaper,omitempty"`
	Limiter              LimiterPlugins              `json:"limiter,omitempty"`
	GateExpander         GateExpanderPlugins         `json:"gate_expander,omitempty"`
	Multiband            MultibandPlugins            `json:"multiband,omitempty"`
}

var ErrNotConfigured = errors.New("experiment plugin whitelist: static_eq plugin is not configured")

var ErrCompressionNotConfigured = errors.New("experiment plugin whitelist: broadband_compression plugin is not configured")

var ErrDeEsserNotConfigured = errors.New("experiment plugin whitelist: de_esser plugin is not configured")

var ErrTransientShaperNotConfigured = errors.New("experiment plugin whitelist: transient_shaper plugin is not configured")

var ErrLimiterNotConfigured = errors.New("experiment plugin whitelist: limiter plugin is not configured")

var ErrGateExpanderNotConfigured = errors.New("experiment plugin whitelist: gate_expander plugin is not configured")

var ErrMultibandNotConfigured = errors.New("experiment plugin whitelist: multiband plugin is not configured")

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
	switch whitelist.SchemaVersion {
	case SchemaVersion:
	case SchemaVersionV5:
		// Compatibility valve: the v5 wire form loads with identical
		// semantics (each single object becomes a one-element candidate
		// list) and the loaded whitelist is canonicalized to v6.
		whitelist.SchemaVersion = SchemaVersion
	default:
		return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: schema_version must be %q or %q but got %q", path, SchemaVersion, SchemaVersionV5, whitelist.SchemaVersion)
	}
	for _, plugin := range whitelist.StaticEQ {
		if err := validateStaticEQPlugin(plugin); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	for _, plugin := range whitelist.BroadbandCompression {
		if err := validateBroadbandCompressionPlugin(plugin); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	for _, plugin := range whitelist.DeEsser {
		if err := validateDeEsserPlugin(plugin); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	for _, plugin := range whitelist.TransientShaper {
		if err := validateTransientShaperPlugin(plugin); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	for _, plugin := range whitelist.Limiter {
		if err := validateLimiterPlugin(plugin); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	for _, plugin := range whitelist.GateExpander {
		if err := validateGateExpanderPlugin(plugin); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	for _, plugin := range whitelist.Multiband {
		if err := validateMultibandPlugin(plugin); err != nil {
			return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
		}
	}
	for _, family := range []struct {
		label       string
		identifiers []string
	}{
		{"static_eq", staticEQIdentifiers(whitelist.StaticEQ)},
		{"broadband_compression", broadbandCompressionIdentifiers(whitelist.BroadbandCompression)},
		{"de_esser", deEsserIdentifiers(whitelist.DeEsser)},
		{"transient_shaper", transientShaperIdentifiers(whitelist.TransientShaper)},
		{"limiter", limiterIdentifiers(whitelist.Limiter)},
		{"gate_expander", gateExpanderIdentifiers(whitelist.GateExpander)},
		{"multiband", multibandIdentifiers(whitelist.Multiband)},
	} {
		seen := map[string]bool{}
		for index, identifier := range family.identifiers {
			if seen[identifier] {
				return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %s entry %d duplicates plugin_identifier %q", path, family.label, index, identifier)
			}
			seen[identifier] = true
		}
	}
	return whitelist, nil
}

func staticEQIdentifiers(entries StaticEQPlugins) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.PluginIdentifier)
	}
	return out
}

func broadbandCompressionIdentifiers(entries BroadbandCompressionPlugins) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.PluginIdentifier)
	}
	return out
}

func deEsserIdentifiers(entries DeEsserPlugins) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.PluginIdentifier)
	}
	return out
}

func transientShaperIdentifiers(entries TransientShaperPlugins) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.PluginIdentifier)
	}
	return out
}

func limiterIdentifiers(entries LimiterPlugins) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.PluginIdentifier)
	}
	return out
}

func gateExpanderIdentifiers(entries GateExpanderPlugins) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.PluginIdentifier)
	}
	return out
}

func multibandIdentifiers(entries MultibandPlugins) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.PluginIdentifier)
	}
	return out
}

// selectFamilyEntry is the shared membership core behind every per-family
// Select method: one candidate resolves with or without a pin (a wrong pin
// keeps the historical exact-equality refusal wording); more than one
// candidate requires the pin and names the admitted set on refusal.
func selectFamilyEntry[T any](label, pinned string, entries []T, notConfigured error, identifierOf func(T) string) (T, error) {
	var zero T
	if len(entries) == 0 {
		return zero, notConfigured
	}
	if len(entries) == 1 {
		identifier := identifierOf(entries[0])
		if pinned == "" || strings.EqualFold(strings.TrimSpace(pinned), identifier) {
			return entries[0], nil
		}
		return zero, fmt.Errorf("%s admission pinned plugin_identifier %q but the experiment plugin whitelist admits %q", label, pinned, identifier)
	}
	identifiers := make([]string, 0, len(entries))
	for _, entry := range entries {
		identifiers = append(identifiers, identifierOf(entry))
	}
	if pinned == "" {
		return zero, fmt.Errorf("%s admission requires a pinned plugin_identifier to select among %d admitted candidates", label, len(entries))
	}
	for _, entry := range entries {
		if strings.EqualFold(strings.TrimSpace(pinned), identifierOf(entry)) {
			return entry, nil
		}
	}
	quoted := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		quoted = append(quoted, fmt.Sprintf("%q", identifier))
	}
	return zero, fmt.Errorf("%s admission pinned plugin_identifier %q but the experiment plugin whitelist admits one of: %s", label, pinned, strings.Join(quoted, ", "))
}

// SelectStaticEQ resolves the static_eq candidate the admission pinned: the
// pinned identifier must be a list member (case-insensitive); an empty pin
// resolves the only entry of a single-candidate family and is ambiguous
// (refused) when the family carries more than one candidate.
func (w Whitelist) SelectStaticEQ(pluginIdentifier string) (StaticEQPlugin, error) {
	return selectFamilyEntry("static_eq", pluginIdentifier, w.StaticEQ, ErrNotConfigured, func(p StaticEQPlugin) string { return p.PluginIdentifier })
}

// SelectBroadbandCompression mirrors SelectStaticEQ for broadband_compression.
func (w Whitelist) SelectBroadbandCompression(pluginIdentifier string) (BroadbandCompressionPlugin, error) {
	return selectFamilyEntry("broadband_compression", pluginIdentifier, w.BroadbandCompression, ErrCompressionNotConfigured, func(p BroadbandCompressionPlugin) string { return p.PluginIdentifier })
}

// SelectDeEsser mirrors SelectStaticEQ for de_esser.
func (w Whitelist) SelectDeEsser(pluginIdentifier string) (DeEsserPlugin, error) {
	return selectFamilyEntry("de_esser", pluginIdentifier, w.DeEsser, ErrDeEsserNotConfigured, func(p DeEsserPlugin) string { return p.PluginIdentifier })
}

// SelectTransientShaper mirrors SelectStaticEQ for transient_shaper.
func (w Whitelist) SelectTransientShaper(pluginIdentifier string) (TransientShaperPlugin, error) {
	return selectFamilyEntry("transient_shaper", pluginIdentifier, w.TransientShaper, ErrTransientShaperNotConfigured, func(p TransientShaperPlugin) string { return p.PluginIdentifier })
}

// SelectLimiter mirrors SelectStaticEQ for limiter.
func (w Whitelist) SelectLimiter(pluginIdentifier string) (LimiterPlugin, error) {
	return selectFamilyEntry("limiter", pluginIdentifier, w.Limiter, ErrLimiterNotConfigured, func(p LimiterPlugin) string { return p.PluginIdentifier })
}

// SelectGateExpander mirrors SelectStaticEQ for gate_expander.
func (w Whitelist) SelectGateExpander(pluginIdentifier string) (GateExpanderPlugin, error) {
	return selectFamilyEntry("gate_expander", pluginIdentifier, w.GateExpander, ErrGateExpanderNotConfigured, func(p GateExpanderPlugin) string { return p.PluginIdentifier })
}

// SelectMultiband mirrors SelectStaticEQ for multiband.
func (w Whitelist) SelectMultiband(pluginIdentifier string) (MultibandPlugin, error) {
	return selectFamilyEntry("multiband", pluginIdentifier, w.Multiband, ErrMultibandNotConfigured, func(p MultibandPlugin) string { return p.PluginIdentifier })
}

// NearestBand maps one admitted frequency onto this plugin's nearest band
// center (ties prefer the lower center).
func (p StaticEQPlugin) NearestBand(frequencyHz float64) (Band, error) {
	if len(p.Bands) == 0 {
		return Band{}, ErrNotConfigured
	}
	best := p.Bands[0]
	bestDistance := math.Abs(best.CenterHz - frequencyHz)
	for _, band := range p.Bands[1:] {
		distance := math.Abs(band.CenterHz - frequencyHz)
		if distance < bestDistance || (distance == bestDistance && band.CenterHz < best.CenterHz) {
			best = band
			bestDistance = distance
		}
	}
	return best, nil
}

// NearestStaticEQBand selects the static_eq entry with the same membership
// rule the admission uses, then maps the frequency onto that entry's nearest
// band center.
func (w Whitelist) NearestStaticEQBand(frequencyHz float64, pluginIdentifier string) (Band, error) {
	selected, err := w.SelectStaticEQ(pluginIdentifier)
	if err != nil {
		return Band{}, err
	}
	return selected.NearestBand(frequencyHz)
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

// ValidateStaticEQAdmission runs the PCA admission predicate for the entry
// the pluginIdentifier selects (empty pin = the only entry of a
// single-candidate family, exactly the pre-v6 behavior).
func (w Whitelist) ValidateStaticEQAdmission(lib processorattestation.Library, pluginIdentifier string) error {
	if len(w.StaticEQ) == 0 {
		return ErrNotConfigured
	}
	selected, err := w.SelectStaticEQ(pluginIdentifier)
	if err != nil {
		return err
	}
	return w.validateSectionAdmission(
		"static_eq",
		selected.PluginName, selected.Manufacturer, selected.Format, selected.PluginIdentifier, selected.PluginPath,
		processorattestation.FamilyStaticEQ, lib,
	)
}

// BroadbandThresholdParams returns the selected whitelisted plugin's
// threshold write shape in pair terms: the ch1/ch2 ids for dual-form entries,
// or the shared parameter id with an empty ch2 for single-form entries
// (FIX-BROADBAND-SHARED-1).
func (w Whitelist) BroadbandThresholdParams(pluginIdentifier string) (thresholdCH1, thresholdCH2 string, err error) {
	selected, err := w.SelectBroadbandCompression(pluginIdentifier)
	if err != nil {
		return "", "", err
	}
	thresholdCH1, thresholdCH2 = selected.ThresholdParamPair()
	return thresholdCH1, thresholdCH2, nil
}

// ValidateCompressionAdmission mirrors ValidateStaticEQAdmission for the
// broadband_compression whitelist section: same subject construction, same
// v1 library predicate family path, and a distinguishable not-PCA-promoted
// prefix naming this domain.
func (w Whitelist) ValidateCompressionAdmission(lib processorattestation.Library, pluginIdentifier string) error {
	if len(w.BroadbandCompression) == 0 {
		return ErrCompressionNotConfigured
	}
	selected, err := w.SelectBroadbandCompression(pluginIdentifier)
	if err != nil {
		return err
	}
	return w.validateSectionAdmission(
		"broadband_compression",
		selected.PluginName, selected.Manufacturer, selected.Format, selected.PluginIdentifier, selected.PluginPath,
		processorattestation.FamilyBroadbandCompressor, lib,
	)
}

// ValidateDeEsserAdmission mirrors ValidateCompressionAdmission for the
// de_esser whitelist section. de_esser is a PCA v2 family, so per the GLM
// ruling on D2-FAM1-S1 (Form A dispatch) this predicate takes the v2
// attestation library; the subject construction and boundary wording stay
// identical to the v1 sections.
func (w Whitelist) ValidateDeEsserAdmission(lib processorattestation.LibraryV2, pluginIdentifier string) error {
	if len(w.DeEsser) == 0 {
		return ErrDeEsserNotConfigured
	}
	selected, err := w.SelectDeEsser(pluginIdentifier)
	if err != nil {
		return err
	}
	return w.validateSectionAdmissionV2(
		"de_esser",
		selected.PluginName, selected.Manufacturer, selected.Format, selected.PluginIdentifier, selected.PluginPath,
		processorattestation.FamilyDeEsser, lib,
	)
}

// ValidateTransientShaperAdmission mirrors ValidateDeEsserAdmission for the
// transient_shaper whitelist section: same subject construction, same v2
// library predicate path (transient_shaper is a PCA v2 family), and a
// distinguishable not-PCA-promoted prefix naming this domain.
func (w Whitelist) ValidateTransientShaperAdmission(lib processorattestation.LibraryV2, pluginIdentifier string) error {
	if len(w.TransientShaper) == 0 {
		return ErrTransientShaperNotConfigured
	}
	selected, err := w.SelectTransientShaper(pluginIdentifier)
	if err != nil {
		return err
	}
	return w.validateSectionAdmissionV2(
		"transient_shaper",
		selected.PluginName, selected.Manufacturer, selected.Format, selected.PluginIdentifier, selected.PluginPath,
		processorattestation.FamilyTransient, lib,
	)
}

// ValidateLimiterAdmission mirrors ValidateTransientShaperAdmission for the
// limiter whitelist section: same subject construction, same v2 library
// predicate path (limiter is a PCA v2 family), and a distinguishable
// not-PCA-promoted prefix naming this domain.
func (w Whitelist) ValidateLimiterAdmission(lib processorattestation.LibraryV2, pluginIdentifier string) error {
	if len(w.Limiter) == 0 {
		return ErrLimiterNotConfigured
	}
	selected, err := w.SelectLimiter(pluginIdentifier)
	if err != nil {
		return err
	}
	return w.validateSectionAdmissionV2(
		"limiter",
		selected.PluginName, selected.Manufacturer, selected.Format, selected.PluginIdentifier, selected.PluginPath,
		processorattestation.FamilyLimiter, lib,
	)
}

// ValidateGateExpanderAdmission mirrors ValidateLimiterAdmission for the
// gate_expander whitelist section: same subject construction, same v2 library
// predicate path (gate_expander is a PCA v2 family), and a distinguishable
// not-PCA-promoted prefix naming this domain.
func (w Whitelist) ValidateGateExpanderAdmission(lib processorattestation.LibraryV2, pluginIdentifier string) error {
	if len(w.GateExpander) == 0 {
		return ErrGateExpanderNotConfigured
	}
	selected, err := w.SelectGateExpander(pluginIdentifier)
	if err != nil {
		return err
	}
	return w.validateSectionAdmissionV2(
		"gate_expander",
		selected.PluginName, selected.Manufacturer, selected.Format, selected.PluginIdentifier, selected.PluginPath,
		processorattestation.FamilyGateExpander, lib,
	)
}

// ValidateMultibandAdmission mirrors ValidateGateExpanderAdmission for the
// multiband whitelist section: same subject construction, same v2 library
// predicate path (multiband_dynamics is a PCA v2 family), and a
// distinguishable not-PCA-promoted prefix naming this domain.
func (w Whitelist) ValidateMultibandAdmission(lib processorattestation.LibraryV2, pluginIdentifier string) error {
	if len(w.Multiband) == 0 {
		return ErrMultibandNotConfigured
	}
	selected, err := w.SelectMultiband(pluginIdentifier)
	if err != nil {
		return err
	}
	return w.validateSectionAdmissionV2(
		"multiband",
		selected.PluginName, selected.Manufacturer, selected.Format, selected.PluginIdentifier, selected.PluginPath,
		processorattestation.FamilyMultiband, lib,
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
	} {
		if missing.value == "" {
			return fmt.Errorf("broadband_compression %s must be non-empty", missing.field)
		}
	}
	switch {
	case plugin.ThresholdParamID != "" && (plugin.ThresholdParamIDCH1 != "" || plugin.ThresholdParamIDCH2 != ""):
		return fmt.Errorf("broadband_compression entry must pin either threshold_param_id (shared) or threshold_param_id_ch1/threshold_param_id_ch2 (dual), not both forms")
	case plugin.ThresholdParamID != "":
		return nil
	case plugin.ThresholdParamIDCH1 == "" && plugin.ThresholdParamIDCH2 == "":
		return fmt.Errorf("broadband_compression entry must pin threshold_param_id (shared) or both threshold_param_id_ch1 and threshold_param_id_ch2 (dual)")
	case plugin.ThresholdParamIDCH1 == "":
		return fmt.Errorf("broadband_compression threshold_param_id_ch1 must be non-empty")
	case plugin.ThresholdParamIDCH2 == "":
		return fmt.Errorf("broadband_compression threshold_param_id_ch2 must be non-empty")
	case plugin.ThresholdParamIDCH1 == plugin.ThresholdParamIDCH2:
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

func validateMultibandPlugin(plugin MultibandPlugin) error {
	for _, missing := range []struct{ field, value string }{
		{"plugin_name", plugin.PluginName},
		{"manufacturer", plugin.Manufacturer},
		{"format", plugin.Format},
		{"plugin_identifier", plugin.PluginIdentifier},
		{"plugin_path", plugin.PluginPath},
	} {
		if missing.value == "" {
			return fmt.Errorf("multiband %s must be non-empty", missing.field)
		}
	}
	// A multiband topology is at least two bands (the recognizer refuses
	// fewer); each band must carry its own non-empty threshold id.
	if len(plugin.BandThresholdParamIDs) < 2 {
		return fmt.Errorf("multiband band_threshold_param_ids must list at least two band threshold ids")
	}
	for index, id := range plugin.BandThresholdParamIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("multiband band_threshold_param_ids[%d] must be non-empty", index)
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
