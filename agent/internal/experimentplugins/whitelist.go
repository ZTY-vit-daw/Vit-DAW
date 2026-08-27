// Package experimentplugins loads the machine-local experiment plugin
// whitelist used by the D1/D2 free-state experiment channel and validates its
// PCA v1 admission. It only loads and validates; wiring into plan/ports/
// smoke tests belongs to the evening S1 slice.
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

const SchemaVersion = "vit.free_state_experiment_plugins.v1"

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

type Whitelist struct {
	SchemaVersion string          `json:"schema_version"`
	StaticEQ      *StaticEQPlugin `json:"static_eq,omitempty"`
}

var ErrNotConfigured = errors.New("experiment plugin whitelist: static_eq plugin is not configured")

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
	if whitelist.StaticEQ == nil {
		return whitelist, nil
	}
	if err := validateStaticEQPlugin(*whitelist.StaticEQ); err != nil {
		return Whitelist{}, fmt.Errorf("experiment plugin whitelist: invalid %s: %w", path, err)
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
	subject := processorattestation.Subject{
		Name:          w.StaticEQ.PluginName,
		Manufacturer:  w.StaticEQ.Manufacturer,
		Format:        w.StaticEQ.Format,
		Identifier:    w.StaticEQ.PluginIdentifier,
		InstalledPath: w.StaticEQ.PluginPath,
	}
	key, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: static_eq plugin %q: %w", w.StaticEQ.PluginName, err)
	}
	fingerprint, err := processorattestation.FingerprintPath(w.StaticEQ.PluginPath)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: static_eq plugin %q: %w", w.StaticEQ.PluginName, err)
	}
	result, err := processorattestation.QueryLibraryAdmission(lib, key, fingerprint, processorattestation.FamilyStaticEQ)
	if err != nil {
		return fmt.Errorf("experiment plugin whitelist: static_eq plugin %q: %w", w.StaticEQ.PluginName, err)
	}
	if !result.Eligible {
		return fmt.Errorf("experiment plugin whitelist: static_eq plugin is not PCA-promoted: plugin %q reason %q", w.StaticEQ.PluginName, result.Reason)
	}
	return nil
}
