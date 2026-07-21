package pluginvps

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/spal"
)

const SchemaVersion = "vit.plugin_vps.v1"
const EqualizerCapability = "equalizer.v2"

type PluginIdentity struct {
	Manufacturer     string `json:"manufacturer,omitempty"`
	Name             string `json:"name"`
	Format           string `json:"format"`
	Version          string `json:"version,omitempty"`
	InstallPath      string `json:"install_path"`
	InstallationHash string `json:"installation_hash"`
}

type Verification struct {
	InstallationHash     string    `json:"installation_hash"`
	ParameterSurfaceHash string    `json:"parameter_surface_hash"`
	VerifiedAt           time.Time `json:"verified_at"`
	WorkerProtocol       string    `json:"worker_protocol,omitempty"`
	Checks               int       `json:"checks,omitempty"`
}

type Bounds struct {
	MaxAbsoluteGainDB *float64 `json:"max_absolute_gain_db,omitempty"`
}

type Document struct {
	SchemaVersion string           `json:"schema_version"`
	Plugin        PluginIdentity   `json:"plugin"`
	Capability    string           `json:"capability"`
	Bindings      spal.EQV2Binding `json:"bindings"`
	Verified      bool             `json:"verified"`
	Verification  *Verification    `json:"verification,omitempty"`
	Bounds        Bounds           `json:"bounds,omitempty"`
}

func (d Document) Validate(requireVerified bool) error {
	if d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %s", SchemaVersion)
	}
	if strings.TrimSpace(d.Plugin.Name) == "" || strings.TrimSpace(d.Plugin.Format) == "" || strings.TrimSpace(d.Plugin.InstallPath) == "" {
		return fmt.Errorf("plugin name, format and install_path are required")
	}
	if !strings.EqualFold(strings.TrimSpace(d.Plugin.Format), "VST3") {
		return fmt.Errorf("only VST3 is supported")
	}
	if !validSHA256(d.Plugin.InstallationHash) {
		return fmt.Errorf("plugin installation_hash must be sha256:<64 hex>")
	}
	if d.Capability != EqualizerCapability {
		return fmt.Errorf("capability must be %s", EqualizerCapability)
	}
	def := spal.EQV2ProviderDefinition{Descriptor: spal.ProviderDescriptor{ID: "pluginvps-validation", AdapterVersion: "v1", Status: spal.ProviderVerified, SupportedSchemas: append([]string(nil), d.Bindings.ConformedSchemas...)}, VPSID: "pluginvps-validation", CredentialID: "pluginvps-validation", Binding: d.Bindings}
	if _, err := spal.NewVPSEQV2Adapter(def); err != nil {
		return fmt.Errorf("bindings: %w", err)
	}
	if requireVerified {
		if !d.Verified || d.Verification == nil {
			return fmt.Errorf("VPS is not verified")
		}
		if d.Verification.InstallationHash != d.Plugin.InstallationHash || !validSHA256(d.Verification.ParameterSurfaceHash) || d.Verification.VerifiedAt.IsZero() {
			return fmt.Errorf("verified stamp is incomplete or stale")
		}
	} else if d.Verified {
		if d.Verification == nil || d.Verification.InstallationHash != d.Plugin.InstallationHash || !validSHA256(d.Verification.ParameterSurfaceHash) || d.Verification.VerifiedAt.IsZero() {
			return fmt.Errorf("verified document has an invalid verification stamp")
		}
	}
	return nil
}

func validSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "sha256:") {
		return false
	}
	raw := strings.TrimSpace(value[len("sha256:"):])
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func Load(path string) (Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	var d Document
	if err = json.Unmarshal(raw, &d); err != nil {
		return Document{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err = d.Validate(false); err != nil {
		return Document{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return d, nil
}

func Save(path string, d Document) error {
	if err := d.Validate(false); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func DefaultDirectory() string {
	if v := strings.TrimSpace(os.Getenv("VIT_PLUGIN_VPS_DIR")); v != "" {
		return v
	}
	if app := strings.TrimSpace(os.Getenv("APPDATA")); app != "" {
		return filepath.Join(app, "Vit", "Agent", "vps")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".vit", "agent", "vps")
}

type Registry struct{ documents []Document }

func LoadDirectory(dir string) (*Registry, []error) {
	r := &Registry{}
	var warnings []error
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return r, []error{err}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".vps.json") {
			continue
		}
		d, loadErr := Load(filepath.Join(dir, e.Name()))
		if loadErr != nil {
			warnings = append(warnings, loadErr)
			continue
		}
		if err = d.Validate(true); err != nil {
			warnings = append(warnings, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		r.documents = append(r.documents, d)
	}
	sort.Slice(r.documents, func(i, j int) bool {
		return strings.ToLower(r.documents[i].Plugin.Name) < strings.ToLower(r.documents[j].Plugin.Name)
	})
	return r, warnings
}
func (r *Registry) Documents() []Document {
	if r == nil {
		return nil
	}
	out := make([]Document, len(r.documents))
	copy(out, r.documents)
	return out
}
func (r *Registry) RuntimeProfiles() []map[string]any {
	if r == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(r.documents))
	for _, d := range r.documents {
		out = append(out, d.RuntimeProfile())
	}
	return out
}

func (d Document) RuntimeProfile() map[string]any {
	groups := make([]any, 0, len(d.Bindings.Bands)+2)
	keys := make([]string, 0, len(d.Bindings.Bands))
	for k := range d.Bindings.Bands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b := d.Bindings.Bands[k]
		params := map[string]any{"enabled": runtimeParameter(b.Enabled), "response_shape": runtimeEnum(b.ResponseShape), "frequency": runtimeParameter(b.FrequencyHz), "gain": runtimeParameter(b.GainDB), "q": runtimeParameter(b.Q)}
		if b.Allocated != nil {
			params["allocated"] = runtimeParameter(*b.Allocated)
		}
		groups = append(groups, map[string]any{"id": firstNonEmpty(b.ComponentID, k), "params": params})
	}
	if d.Bindings.HighPass != nil {
		groups = append(groups, passGroup("hp", *d.Bindings.HighPass))
	}
	if d.Bindings.LowPass != nil {
		groups = append(groups, passGroup("lp", *d.Bindings.LowPass))
	}
	profile := map[string]any{"schema_version": SchemaVersion, "verified": d.Verified, "capability": d.Capability, "plugin": d.Plugin, "verification": d.Verification, "class": "equalizer", "groups": groups, "source": "verified_vps"}
	if d.Bindings.Output != nil {
		params := map[string]any{}
		if d.Bindings.Output.Bypass != nil {
			params["bypass"] = runtimeParameter(*d.Bindings.Output.Bypass)
		}
		if d.Bindings.Output.DryMix != nil {
			params["dry_mix_percent"] = runtimeParameter(*d.Bindings.Output.DryMix)
		}
		if d.Bindings.Output.OutputGainDB != nil {
			params["output_gain_db"] = runtimeParameter(*d.Bindings.Output.OutputGainDB)
		}
		groups = append(groups, map[string]any{"id": firstNonEmpty(d.Bindings.Output.ComponentID, "output"), "params": params})
		profile["groups"] = groups
	}
	if d.Bounds.MaxAbsoluteGainDB != nil {
		profile["safety"] = map[string]any{"max_abs_gain_db": *d.Bounds.MaxAbsoluteGainDB}
	}
	return profile
}
func passGroup(fallback string, b spal.EQV2PassFilterBinding) map[string]any {
	p := map[string]any{"enabled": runtimeParameter(b.Enabled), "cutoff": runtimeParameter(b.CutoffFrequencyHz), "slope": runtimeEnum(b.SlopeDBPerOctave)}
	if b.Allocated != nil {
		p["allocated"] = runtimeParameter(*b.Allocated)
	}
	if b.ResponseShape != nil {
		p["response_shape"] = runtimeEnum(*b.ResponseShape)
	}
	return map[string]any{"id": firstNonEmpty(b.ComponentID, fallback), "params": p}
}
func runtimeParameter(b spal.ParameterBinding) map[string]any {
	return map[string]any{"parameter_id": b.ParameterID, "unit": b.Unit, "confirmed": true, "display_domain": map[string]any{"unit": b.Unit, "min": b.Min, "max": b.Max, "scale": b.Scale, "status": "verified"}}
}
func runtimeEnum(b spal.EnumParameterBinding) map[string]any {
	type pair struct {
		label string
		value float64
	}
	pairs := make([]pair, 0, len(b.Values))
	verifiedValues := make(map[string]float64, len(b.Values))
	for semanticLabel, value := range b.Values {
		displayLabel := strings.TrimSpace(b.DisplayLabels[semanticLabel])
		if displayLabel == "" {
			displayLabel = semanticLabel
		}
		pairs = append(pairs, pair{displayLabel, value})
		verifiedValues[displayLabel] = value
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].value == pairs[j].value {
			return pairs[i].label < pairs[j].label
		}
		return pairs[i].value < pairs[j].value
	})
	labels := make([]string, 0, len(pairs))
	for _, p := range pairs {
		labels = append(labels, p.label)
	}
	return map[string]any{"parameter_id": b.ParameterID, "confirmed": true, "verified_values": verifiedValues, "display_domain": map[string]any{"text": "enum:" + strings.Join(labels, "/"), "scale": "enum", "status": "verified"}}
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func HashSurface(parameterIDs []string) string {
	ids := append([]string(nil), parameterIDs...)
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func BindingParameterIDs(b spal.EQV2Binding) []string {
	set := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = true
		}
	}
	addP := func(p spal.ParameterBinding) { add(p.ParameterID) }
	addE := func(p spal.EnumParameterBinding) { add(p.ParameterID) }
	for _, x := range b.Bands {
		if x.Allocated != nil {
			addP(*x.Allocated)
		}
		addP(x.Enabled)
		addE(x.ResponseShape)
		addP(x.FrequencyHz)
		addP(x.GainDB)
		addP(x.Q)
		if x.Dynamic != nil {
			addE(x.Dynamic.Mode)
			addE(x.Dynamic.Routing)
			addP(x.Dynamic.ThresholdDB)
			if x.Dynamic.Ratio != nil {
				addP(*x.Dynamic.Ratio)
			}
			if x.Dynamic.AttackMS != nil {
				addP(*x.Dynamic.AttackMS)
			}
			if x.Dynamic.ReleaseMS != nil {
				addP(*x.Dynamic.ReleaseMS)
			}
		}
	}
	for _, x := range []*spal.EQV2PassFilterBinding{b.HighPass, b.LowPass} {
		if x == nil {
			continue
		}
		if x.Allocated != nil {
			addP(*x.Allocated)
		}
		if x.ResponseShape != nil {
			addE(*x.ResponseShape)
		}
		addP(x.Enabled)
		addP(x.CutoffFrequencyHz)
		addE(x.SlopeDBPerOctave)
	}
	if x := b.Output; x != nil {
		if x.Bypass != nil {
			addP(*x.Bypass)
		}
		if x.DryMix != nil {
			addP(*x.DryMix)
		}
		if x.OutputGainDB != nil {
			addP(*x.OutputGainDB)
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
