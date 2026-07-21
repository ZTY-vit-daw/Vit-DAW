package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/vps"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

type vpsConformanceTestKernel struct {
	params       map[string]float64
	commands     []map[string]any
	pluginPath   string
	dropReadback bool
}

func (k *vpsConformanceTestKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	k.commands = append(k.commands, cloneTestCommand(command))
	switch cleanContextText(command["cmd"]) {
	case "n_project_profile":
		return map[string]any{"status": "ok", "message": "profile saved"}, "{}", nil
	case "get_plugin_parameters":
		reply := k.parameterReply()
		return reply, "{}", nil
	case "set_plugin_param":
		id := cleanContextText(command["param_id"])
		value, ok := vpsFiniteNumber(command["normalized_value"])
		if !ok {
			return map[string]any{"status": "error", "message": "missing normalized value"}, "{}", nil
		}
		if _, exists := k.params[id]; !exists {
			return map[string]any{"status": "error", "message": "unknown parameter"}, "{}", nil
		}
		if !(k.dropReadback && id == "gain" && math.Abs(value-0.25) < 0.001) {
			k.params[id] = value
		}
		return map[string]any{
			"status": "ok", "param_id": id, "new_normalised_value": k.params[id],
		}, "{}", nil
	default:
		return map[string]any{"status": "error", "message": "unexpected command " + cleanContextText(command["cmd"])}, "{}", nil
	}
}

func (k *vpsConformanceTestKernel) parameterReply() map[string]any {
	ids := make([]string, 0, len(k.params))
	for id := range k.params {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parameters := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		parameters = append(parameters, vpsTestParameter(id, k.params[id]))
	}
	return map[string]any{
		"status": "ok", "track_id": "track_vps", "plugin_id": "plugin_vps",
		"plugin_identity": map[string]any{
			"manufacturer": "Example Audio", "plugin_name": "Conformance EQ", "plugin_format": "VST3",
			"version": "1.0.0", "profile_key": "conformance_eq", "plugin_path": k.pluginPath,
		},
		"parameters": parameters,
	}
}

func vpsTestParameter(id string, normalized float64) map[string]any {
	row := map[string]any{
		"id": id, "param_id": id, "name": id, "raw_param_name": id,
		"normalized_value": normalized, "host_controllable": true,
	}
	switch id {
	case "frequency":
		value := 20 * math.Pow(1000, normalized)
		row["value"], row["min"], row["max"] = value, 20.0, 20000.0
		row["value_text"] = fmt.Sprintf("%.1f Hz", value)
		row["display_probe"] = map[string]any{"samples": []map[string]any{
			{"normalized_value": 0.0, "value": 20.0, "text": "20 Hz"},
			{"normalized_value": 0.5, "value": 632.5, "text": "632.5 Hz"},
			{"normalized_value": 1.0, "value": 20000.0, "text": "20000 Hz"},
		}}
	case "gain":
		value := -18 + normalized*36
		// The kernel transport exposes the normalized host range. The physical
		// dB range is supplied separately as fresh display-domain evidence.
		minimum, maximum := -18.0, 18.0
		row["value"], row["min"], row["max"] = normalized, 0.0, 1.0
		row["value_text"] = fmt.Sprintf("%.1f", value)
		row["display_domain_candidate"] = map[string]any{
			"text": "-18~18 dB", "unit": "dB", "min": minimum, "max": maximum, "scale": "linear",
		}
		row["display_probe"] = map[string]any{"current_text": fmt.Sprintf("%.1f", value), "label": "dB", "samples": []map[string]any{
			{"normalized_value": 0.0, "value": -18.0, "text": "-18 dB"},
			{"normalized_value": 0.5, "value": 0.0, "text": "0 dB"},
			{"normalized_value": 1.0, "value": 18.0, "text": "18 dB"},
		}}
	case "q":
		value := 0.1 * math.Pow(120, normalized)
		row["value"], row["min"], row["max"] = value, 0.1, 12.0
		row["value_text"] = fmt.Sprintf("%.2f Q", value)
		row["display_probe"] = map[string]any{"samples": []map[string]any{
			{"normalized_value": 0.0, "value": 0.1, "text": "0.1 Q"},
			{"normalized_value": 0.5, "value": 1.1, "text": "1.1 Q"},
			{"normalized_value": 1.0, "value": 12.0, "text": "12 Q"},
		}}
	case "enable":
		row["value"], row["min"], row["max"] = normalized, 0.0, 1.0
		row["is_boolean"], row["is_discrete"], row["num_steps"] = true, true, 2
		if normalized >= 0.5 {
			row["value_text"] = "On"
		} else {
			row["value_text"] = "Off"
		}
		row["display_probe"] = map[string]any{"discrete_labels": []map[string]any{{"index": 0, "value": 0.0, "label": "Off"}, {"index": 1, "value": 1.0, "label": "On"}}}
	case "type":
		row["value"], row["min"], row["max"] = normalized, 0.0, 2.0
		row["is_discrete"], row["num_steps"], row["value_text"] = true, 3, "Bell"
		row["display_probe"] = map[string]any{"discrete_labels": []map[string]any{{"index": 0, "value": 0.0, "label": "Low Shelf"}, {"index": 1, "value": 1.0, "label": "Bell"}, {"index": 2, "value": 2.0, "label": "High Shelf"}}}
	}
	return row
}

func TestVPSBipolarDBDisplayRangeUsesPhysicalDisplayDomain(t *testing.T) {
	minimum, maximum := -18.0, 18.0
	parameter := pluginParameterInfo{
		Min:       0.0,
		Max:       1.0,
		Value:     0.25,
		ValueText: "−9.0",
		DisplayProbe: &plugingrabber.ParameterDisplayProbe{
			CurrentText: "−9.0",
			Label:       "dB",
		},
		DisplayDomainCandidate: &plugingrabber.PluginDisplayDomain{
			Text: "-18~18 dB", Unit: "dB", Min: &minimum, Max: &maximum, Scale: "linear",
		},
	}
	actualMin, actualMax, ok := vpsBipolarDBDisplayRange(parameter)
	if !ok || actualMin != -18 || actualMax != 18 {
		t.Fatalf("physical display range = (%v, %v, %v)", actualMin, actualMax, ok)
	}
	value, ok := vpsDBValueFromReadback(parameter)
	if !ok || math.Abs(value-(-9.0)) > 0.0001 {
		t.Fatalf("dB readback = (%v, %v)", value, ok)
	}
}

func TestPluginLearningVPSV3ConformanceIssuesCredentialAndRestoresState(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "Conformance EQ.vst3")
	if err := os.WriteFile(pluginPath, []byte("test plug-in package"), 0o600); err != nil {
		t.Fatal(err)
	}
	libraryPath := filepath.Join(t.TempDir(), "vps_library.json")
	t.Setenv("VIT_VPS_LIBRARY_V3_PATH", libraryPath)
	kernel := &vpsConformanceTestKernel{
		pluginPath: pluginPath,
		params:     map[string]float64{"frequency": 0.42, "gain": 0.50, "q": 0.34, "enable": 0.0, "type": 1.0},
	}
	original := map[string]float64{}
	for key, value := range kernel.params {
		original[key] = value
	}
	server := New(nil, shadow.New(nil), nil)
	server.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	plan := PendingPlan{
		Workflow: "plugin_grabber_auto_learn",
		WorkflowData: map[string]any{
			"mode":         "auto_learn",
			"target":       map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps", "plugin_name": "Conformance EQ"},
			"plugin_skill": vpsTestConfirmedSkill(pluginPath),
		},
		Context: map[string]any{"conversation_id": "chat_vps"},
	}
	commit, err := server.commitPluginLearningVPSV3(context.Background(), plan)
	if err != nil {
		t.Fatalf("commitPluginLearningVPSV3: %v", err)
	}
	if commit.CredentialStatus != vps.CredentialVerified || !commit.CatalogVisible || commit.CatalogEntry == nil {
		t.Fatalf("commit = %#v", commit)
	}
	if commit.Conformance["status"] != "pass" || commit.Conformance["rollback"] != true {
		t.Fatalf("conformance summary = %#v", commit.Conformance)
	}
	for id, wanted := range original {
		if actual := kernel.params[id]; math.Abs(actual-wanted) > 0.0001 {
			t.Fatalf("parameter %s remained altered: got=%v want=%v commands=%+v", id, actual, wanted, kernel.commands)
		}
	}
	setCount := 0
	for _, command := range kernel.commands {
		if cleanContextText(command["cmd"]) == "set_plugin_param" {
			setCount++
			if _, ok := command["normalized_value"]; !ok {
				t.Fatalf("conformance write did not use normalized_value: %+v", command)
			}
		}
	}
	if setCount < 16 {
		t.Fatalf("expected patch, boundary and rollback writes; got %d commands=%+v", setCount, kernel.commands)
	}
	library, err := vps.NewLibrary(libraryPath)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := library.Catalog()
	if err != nil || len(catalog.Entries) != 1 || catalog.Entries[0].CredentialID != commit.CredentialID {
		t.Fatalf("catalog = %#v err=%v", catalog, err)
	}
	recorder := httptest.NewRecorder()
	server.handleVPSCatalog(recorder, httptest.NewRequest(http.MethodGet, "/agent/vps/catalog", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("catalog endpoint status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var endpoint map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &endpoint); err != nil {
		t.Fatal(err)
	}
	if endpoint["status"] != "ok" || len(mapRowsValue(mapValue(endpoint["catalog"])["entries"])) != 1 {
		t.Fatalf("catalog endpoint = %#v", endpoint)
	}
}

func TestPluginLearningVPSV3ConformanceFailureStaysOutOfCatalog(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "Conformance EQ.vst3")
	if err := os.WriteFile(pluginPath, []byte("test plug-in package"), 0o600); err != nil {
		t.Fatal(err)
	}
	libraryPath := filepath.Join(t.TempDir(), "vps_library.json")
	t.Setenv("VIT_VPS_LIBRARY_V3_PATH", libraryPath)
	kernel := &vpsConformanceTestKernel{
		pluginPath: pluginPath, dropReadback: true,
		params: map[string]float64{"frequency": 0.42, "gain": 0.50, "q": 0.34, "enable": 0.0, "type": 1.0},
	}
	server := New(nil, shadow.New(nil), nil)
	server.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	commit, err := server.commitPluginLearningVPSV3(context.Background(), PendingPlan{
		Workflow: "plugin_grabber_auto_learn",
		WorkflowData: map[string]any{
			"mode": "auto_learn", "target": map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
			"plugin_skill": vpsTestConfirmedSkill(pluginPath),
		},
	})
	if err != nil {
		t.Fatalf("failed conformance should retain a mapped VPS, got %v", err)
	}
	if commit.CredentialID != "" || commit.CatalogVisible || commit.VPSStatus != vps.VPSStatusMapped || commit.Conformance["status"] != "fail" {
		t.Fatalf("failure commit = %#v", commit)
	}
	library, err := vps.NewLibrary(libraryPath)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := library.Catalog()
	if err != nil || len(catalog.Entries) != 0 {
		t.Fatalf("failed conformance entered catalog: %#v err=%v", catalog, err)
	}
	if len(commit.Warnings) == 0 || !strings.Contains(strings.Join(commit.Warnings, " "), "conformance") {
		t.Fatalf("failed conformance warning missing: %#v", commit.Warnings)
	}
}

func TestPluginLearningConfirmationPersistsVPSAndIssuesCredential(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "Conformance EQ.vst3")
	if err := os.WriteFile(pluginPath, []byte("test plug-in package"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_VPS_LIBRARY_V3_PATH", filepath.Join(t.TempDir(), "vps_library.json"))
	kernel := &vpsConformanceTestKernel{
		pluginPath: pluginPath,
		params:     map[string]float64{"frequency": 0.42, "gain": 0.50, "q": 0.34, "enable": 0.0, "type": 1.0},
	}
	server := New(nil, shadow.New(nil), nil)
	server.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	commands := []map[string]any{{
		"cmd": "plugin_grabber_upsert_project_profile", "track_id": "track_vps", "plugin_id": "plugin_vps",
		"plugin_skill": vpsTestConfirmedSkill(pluginPath),
	}}
	decisions := policy.Analyze(commands)
	if len(decisions) != 1 {
		t.Fatalf("profile decisions = %#v", decisions)
	}
	plan := PendingPlan{
		ID: "plan_vps_learning", Workflow: "plugin_grabber_auto_learn", Decisions: decisions,
		WorkflowData: map[string]any{
			"mode": "auto_learn", "conversation_id": "chat_vps_confirm",
			"target":       map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps", "plugin_name": "Conformance EQ"},
			"plugin_skill": vpsTestConfirmedSkill(pluginPath),
		},
		Context: map[string]any{"conversation_id": "chat_vps_confirm"},
	}
	server.pending[plan.ID] = plan
	status, response := server.resolvePendingPlanDecision(context.Background(), plan.ID, "approve")
	if status != http.StatusOK || response["status"] != "ok" {
		t.Fatalf("confirmation response = status:%d %#v", status, response)
	}
	vpsData := mapValue(response["vps_v3"])
	if cleanContextText(vpsData["credential_status"]) != string(vps.CredentialVerified) || !boolValue(vpsData["catalog_visible"]) {
		t.Fatalf("VPS result was not attached to saved learning response: %#v", response)
	}
	learning := mapValue(response["plugin_learning"])
	if mapValue(learning["vps_v3"])["credential_status"] != string(vps.CredentialVerified) {
		t.Fatalf("Plugin Learning completion did not retain VPS result: %#v", learning)
	}
}

func vpsTestConfirmedSkill(path string) plugingrabber.PluginSkillDocument {
	frequencyMin, frequencyMax := 20.0, 20000.0
	gainMin, gainMax := -18.0, 18.0
	qMin, qMax := 0.1, 12.0
	toggleMin, toggleMax := 0.0, 1.0
	typeMin, typeMax := 0.0, 2.0
	mapping := func(id, label string, minimum, maximum *float64, unit, scale string) plugingrabber.PluginSkillParamMap {
		return plugingrabber.PluginSkillParamMap{
			ParamID: id, Label: label, Source: "user_demonstrated", Confidence: 1, Confirmed: true,
			DisplayDomain: &plugingrabber.PluginDisplayDomain{Text: label, Unit: unit, Min: minimum, Max: maximum, Scale: scale},
		}
	}
	return plugingrabber.PluginSkillDocument{
		SchemaVersion: plugingrabber.PluginSkillSchemaVersion,
		Identity: plugingrabber.PluginSkillIdentity{
			Manufacturer: "Example Audio", Name: "Conformance EQ", Format: "VST3", Version: "1.0.0", ProfileKey: "conformance_eq", Path: path,
			ParamSignatureHash: "legacy-conformance-eq",
		},
		Components: []plugingrabber.PluginSkillComponent{{
			ID: "b1", Role: "eq_band", Label: "Band 1",
			Params: map[string]plugingrabber.PluginSkillParamMap{
				"type":      mapping("type", "Filter type", &typeMin, &typeMax, "enum", "enum"),
				"frequency": mapping("frequency", "Frequency", &frequencyMin, &frequencyMax, "Hz", "log"),
				"gain":      mapping("gain", "Gain", &gainMin, &gainMax, "dB", "linear"),
				"q":         mapping("q", "Q", &qMin, &qMax, "Q", "log"),
				"enable":    mapping("enable", "Enable", &toggleMin, &toggleMax, "toggle", "linear"),
			},
		}},
	}
}
