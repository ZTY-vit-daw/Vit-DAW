package pluginvps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"vit-daw-agent/internal/spal"
)

type SurfaceSnapshot struct {
	PluginIdentity struct {
		Manufacturer string `json:"manufacturer"`
		Name         string `json:"name"`
		Format       string `json:"format"`
		Version      string `json:"version"`
		InstallPath  string `json:"install_path"`
		Fingerprint  struct {
			Installation string `json:"installation"`
		} `json:"fingerprint"`
	} `json:"plugin_identity"`
	Parameters []SurfaceParameter `json:"parameters"`
}
type SurfaceParameter struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Unit             string `json:"unit"`
	HostControllable bool   `json:"host_controllable"`
	DisplayDomain    struct {
		Unit  string   `json:"unit"`
		Min   *float64 `json:"min"`
		Max   *float64 `json:"max"`
		Scale string   `json:"scale"`
	} `json:"display_domain"`
	Observed map[string]any `json:"observed"`
}

func DraftFromSurfaceFile(path string) (Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	var s SurfaceSnapshot
	if err = json.Unmarshal(raw, &s); err != nil {
		return Document{}, err
	}
	return DraftFromSurface(s)
}
func DraftFromSurface(s SurfaceSnapshot) (Document, error) {
	d := Document{SchemaVersion: SchemaVersion, Plugin: PluginIdentity{Manufacturer: s.PluginIdentity.Manufacturer, Name: s.PluginIdentity.Name, Format: s.PluginIdentity.Format, Version: s.PluginIdentity.Version, InstallPath: s.PluginIdentity.InstallPath, InstallationHash: s.PluginIdentity.Fingerprint.Installation}, Capability: EqualizerCapability, Bindings: spal.EQV2Binding{}}
	candidates := map[string][]SurfaceParameter{}
	for _, p := range s.Parameters {
		if !p.HostControllable || strings.TrimSpace(p.ID) == "" {
			continue
		}
		slot := inferSlot(p.Name)
		if slot != "" {
			candidates[slot] = append(candidates[slot], p)
		}
	}
	band := spal.EQV2BandBinding{ComponentID: "b1"}
	ok := true
	var take = func(slot string) (SurfaceParameter, bool) {
		v := candidates[slot]
		if len(v) == 0 {
			return SurfaceParameter{}, false
		}
		sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
		return v[0], true
	}
	if p, yes := take("enabled"); yes {
		band.Enabled = toBinding(p, 0, 1, "linear")
	} else {
		ok = false
	}
	if p, yes := take("response_shape"); yes {
		band.ResponseShape = spal.EnumParameterBinding{ParameterID: p.ID, Values: enumValues(p)}
	} else {
		ok = false
	}
	if p, yes := take("frequency_hz"); yes {
		band.FrequencyHz = toBinding(p, 10, 40000, "log")
	} else {
		ok = false
	}
	if p, yes := take("gain_db"); yes {
		band.GainDB = toBinding(p, -18, 18, "linear")
	} else {
		ok = false
	}
	if p, yes := take("q"); yes {
		band.Q = toBinding(p, .1, 6, "log")
	} else {
		ok = false
	}
	if ok {
		d.Bindings.Bands = map[string]spal.EQV2BandBinding{"b1": band}
		d.Bindings.ConformedSchemas = []string{spal.EQBandPatchControlID}
	}
	if err := d.Validate(false); err != nil {
		return d, fmt.Errorf("draft needs manual semantic mapping: %w", err)
	}
	return d, nil
}

var tokenRE = regexp.MustCompile(`[^a-z0-9]+`)

func inferSlot(name string) string {
	s := strings.TrimSpace(tokenRE.ReplaceAllString(strings.ToLower(name), " "))
	tokens := map[string]bool{}
	for _, token := range strings.Fields(s) {
		tokens[token] = true
	}
	switch {
	case strings.Contains(s, "enable") || tokens["active"] || tokens["on"]:
		return "enabled"
	case strings.Contains(s, "type") || strings.Contains(s, "shape"):
		return "response_shape"
	case strings.Contains(s, "freq") || strings.Contains(s, "cutoff"):
		return "frequency_hz"
	case strings.Contains(s, "gain"):
		return "gain_db"
	case tokens["q"] || strings.Contains(s, "quality"):
		return "q"
	}
	return ""
}
func toBinding(p SurfaceParameter, min, max float64, scale string) spal.ParameterBinding {
	if p.DisplayDomain.Min != nil {
		min = *p.DisplayDomain.Min
	}
	if p.DisplayDomain.Max != nil {
		max = *p.DisplayDomain.Max
	}
	if strings.TrimSpace(p.DisplayDomain.Scale) != "" && p.DisplayDomain.Scale != "normalized" {
		scale = p.DisplayDomain.Scale
	}
	unit := firstNonEmpty(p.DisplayDomain.Unit, p.Unit)
	return spal.ParameterBinding{ParameterID: p.ID, Unit: unit, Min: min, Max: max, Scale: scale}
}
func enumValues(p SurfaceParameter) map[string]float64 {
	var labels []string
	if raw, ok := p.Observed["display_choices"]; ok {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &labels)
	}
	out := map[string]float64{}
	if len(labels) == 1 {
		out[labels[0]] = 0
	} else {
		for i, l := range labels {
			out[l] = float64(i) / float64(len(labels)-1)
		}
	}
	if len(out) == 0 {
		out["bell"] = 0
	}
	return out
}
func Slug(name string) string {
	s := strings.Trim(tokenRE.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if s == "" {
		s = "plugin"
	}
	return s
}
func ResolveSurface(pluginArg, surfaceFlag string) (string, error) {
	if strings.TrimSpace(surfaceFlag) != "" {
		return surfaceFlag, nil
	}
	p := strings.TrimSpace(pluginArg)
	if info, err := os.Stat(p); err == nil {
		if info.IsDir() {
			return filepath.Join(p, "surface_snapshot.json"), nil
		}
		return p, nil
	}
	return "", fmt.Errorf("draft %q requires --surface or a surface/workspace path", pluginArg)
}
