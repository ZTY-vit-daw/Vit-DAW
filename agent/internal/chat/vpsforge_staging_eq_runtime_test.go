package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

type vpsForgeStagingEQTestKernel struct {
	params   map[string]float64
	commands []map[string]any
}

func (k *vpsForgeStagingEQTestKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	copy := map[string]any{}
	for key, value := range command {
		copy[key] = value
	}
	k.commands = append(k.commands, copy)
	switch cleanContextText(command["cmd"]) {
	case "get_plugin_parameters":
		return k.parameterReply(), "{}", nil
	case "set_plugin_param":
		id := cleanContextText(command["param_id"])
		value, valid := vpsFiniteNumber(command["normalized_value"])
		if !valid {
			return map[string]any{"status": "error", "message": "missing normalized value"}, "{}", nil
		}
		if _, found := k.params[id]; !found {
			return map[string]any{"status": "error", "message": "unknown parameter"}, "{}", nil
		}
		k.params[id] = value
		return map[string]any{"status": "ok", "param_id": id, "new_normalised_value": value}, "{}", nil
	default:
		return map[string]any{"status": "error", "message": "unexpected command"}, "{}", nil
	}
}

func (k *vpsForgeStagingEQTestKernel) parameterReply() map[string]any {
	ids := make([]string, 0, len(k.params))
	for id := range k.params {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parameters := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		parameters = append(parameters, map[string]any{
			"id": id, "param_id": id, "name": "Pro-Q 3 " + id,
			"normalized_value": k.params[id], "host_controllable": true,
			"value_text": fmt.Sprintf("%.6f", k.params[id]), "min": 0.0, "max": 1.0,
			"display_probe": map[string]any{"label": "Pro-Q 3 " + id},
		})
	}
	return map[string]any{
		"status": "ok", "track_id": "track_proq", "plugin_id": "plugin_proq",
		"plugin_identity": map[string]any{
			"manufacturer": "FabFilter", "plugin_name": "Pro-Q 3", "plugin_format": "VST3", "version": "3.2.3.0",
		},
		"parameters": parameters,
	}
}

func vpsForgeStagingEQWriteCount(commands []map[string]any) int {
	count := 0
	for _, command := range commands {
		if cleanContextText(command["cmd"]) == "set_plugin_param" {
			count++
		}
	}
	return count
}

func vpsForgeStagingEQTestActionModel() (spal.EQV2Binding, []string, []vps.VPSActionImplementation) {
	allocation := spal.ParameterBinding{ParameterID: "0", Unit: "toggle", Min: 0, Max: 1, Scale: "linear"}
	linear := func(id, unit string, minimum, maximum float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: minimum, Max: maximum, Scale: "linear"}
	}
	log := func(id, unit string, minimum, maximum float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: minimum, Max: maximum, Scale: "log"}
	}
	binding := spal.EQV2Binding{Bands: map[string]spal.EQV2BandBinding{"b1": {
		ComponentID: "b1", Allocated: &allocation, Enabled: linear("1", "toggle", 0, 1),
		ResponseShape: spal.EnumParameterBinding{ParameterID: "8", Values: map[string]float64{"bell": 0}},
		FrequencyHz:   log("2", "Hz", 10, 30000), GainDB: linear("3", "dB", -30, 30), Q: log("7", "Q", .025, 40),
	}}}
	implementations := []vps.VPSActionImplementation{{
		BadgeID: vps.EqualizerCapabilityID, BadgeVersion: vps.EqualizerProfileVersion,
		ActionID: "eq.static_band.patch", SchemaID: spal.EQBandPatchControlID, Status: vps.BadgeFeatureStatusStagingReady, BindingRef: "spal.eq_v2.staging_binding",
		Features: []vps.BadgeFeatureMatrixEntry{
			{ActionID: "eq.static_band.patch", FeatureID: "static_band", Status: vps.BadgeFeatureStatusStagingReady},
			{ActionID: "eq.static_band.patch", FeatureID: "bell", Status: vps.BadgeFeatureStatusStagingReady},
		},
	}}
	return binding, []string{spal.EQBandPatchControlID}, implementations
}

func newVPSForgeStagingEQRuntimeServer(t *testing.T) (*Server, *vpsForgeStagingEQTestKernel, *vps.Library, vps.VPSDocument) {
	t.Helper()
	root := t.TempDir()
	canonicalLibraryPath := filepath.Join(root, "canonical_vps_library.json")
	libraryPath := filepath.Join(root, "staging_vps_library.json")
	reportPath := filepath.Join(root, "reports")
	artifactPath := filepath.Join(root, "pro_q_3_static_eq_staging_vps.json")
	// The normal Agent keeps this canonical Library. The staging document is
	// deliberately stored in a separate library named by the artifact.
	t.Setenv("VIT_VPS_LIBRARY_V3_PATH", canonicalLibraryPath)
	t.Setenv(vpsDraftTestReportsDirectoryEnv, reportPath)
	t.Setenv(vpsForgeStagingVPSPathEnv, artifactPath)
	t.Setenv("VIT_AGENT_JOURNAL_PATH", "memory")
	kernel := &vpsForgeStagingEQTestKernel{params: map[string]float64{
		"0": 0, "1": 1, "2": 0.575188457965851, "3": 0.5, "7": 0.5, "8": 0, "9": 1.0 / 9.0, "other": 0.25,
	}}
	digest := buildPluginParameterDigest(kernel.parameterReply())
	fingerprint, err := vps.BuildPluginFingerprintFromDigest("draft-test", digest)
	if err != nil {
		t.Fatal(err)
	}
	library, err := vps.NewLibrary(libraryPath)
	if err != nil {
		t.Fatal(err)
	}
	document := vps.NewDraft(vps.PluginIdentity{
		Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "3.2.3.0",
		Fingerprint: vps.PluginFingerprint{ParameterSurface: fingerprint.ParameterSurface},
	}, time.Now().UTC())
	document.ID = "vps_pro_q_3_static_eq_staging_test"
	binding, schemas, implementations := vpsForgeStagingEQTestActionModel()
	document.BadgeActionImplementations = implementations
	for _, mapping := range vpsForgeProQ3Band1StagingMappings {
		parts := strings.SplitN(mapping.Key, ".", 2)
		document.ControlSurface.Mappings = append(document.ControlSurface.Mappings, vps.ControlSurfaceMapping{
			ComponentID: parts[0], SemanticSlot: parts[1], ParameterID: mapping.ParameterID,
			BindingStatus: "user-confirmed_staging_executable", ExecutionScope: "forge_staging_actual_test",
		})
	}
	stored, added, err := library.ImportDraft(document)
	if err != nil || !added {
		t.Fatalf("ImportDraft added=%v err=%v", added, err)
	}
	document = stored
	artifact := vpsForgeStagingVPSArtifact{
		SchemaVersion: vpsForgeStagingVPSSchema, Trust: "user-confirmed", Status: vpsForgeStagingVPSStatus,
		VPSID: document.ID, VPSRevision: document.Revision,
		BadgeActionImplementations: implementations, ImplementedSchemas: schemas, StagingBinding: &binding,
		Runtime: vpsForgeStagingVPSArtifactRuntime{LibraryPath: "staging_vps_library.json"},
	}
	for _, mapping := range vpsForgeProQ3Band1StagingMappings {
		artifact.ImplementedMappings = append(artifact.ImplementedMappings, vpsForgeStagingArtifactMapping{
			Key: mapping.Key, ParameterID: mapping.ParameterID, BindingStatus: "user-confirmed_staging_executable", ExecutionScope: "forge_staging_actual_test",
		})
	}
	payload, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	server := New(nil, shadow.New(nil), nil)
	server.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	return server, kernel, library, document
}

func proQ3StagingBellRequest() spalEQV2Request {
	maxGain := 18.0
	return spalEQV2Request{
		TargetRef: "track_proq", PluginID: "plugin_proq",
		Instruction: spal.Instruction{
			SchemaID: spal.EQBandPatchControlID, TargetRef: "track_proq",
			Parameters:       map[string]float64{"frequency_hz": 3188.1, "gain_db": 7.5, "q": 3.024, "enabled": 1},
			StringParameters: map[string]string{"band_ref": "b1", "response_shape": "bell"},
			SafetyBounds:     spal.SafetyBounds{MaxAbsoluteGainDB: &maxGain},
		},
	}
}

func TestVPSForgeStagingEQCandidateDoesNotOwnEqualizerPlanExecution(t *testing.T) {
	server, kernel, _, _ := newVPSForgeStagingEQRuntimeServer(t)
	server.pluginEffectControlRuntimeOverride = func(_ context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
		apply := pluginEffectApplyArgs(req.Context)
		if cleanContextText(apply["control"]) != "eq.boost_region" {
			t.Fatalf("equalizer plan did not delegate to B4: %#v", apply)
		}
		return pluginEffectProposalTestResponse(conversationID, goal)
	}
	out, invokeErr := server.invokeEqualizerCapabilityTool(context.Background(), executorpkg.Input{
		GoalID: "goal_staging", RunID: "run_staging",
		ToolCall: planner.ToolCall{ID: "call_staging", Tool: equalizerCapabilityPlanTool, Args: map[string]any{
			"task": "spectral_region_adjust", "response_shape": "bell",
			"frequency_hz": 3188.1, "gain_db": 7.5, "q": 3.024,
		}},
		Context: map[string]any{
			"conversation_id": "staging_conversation", "user_message": "Pro-Q 3 Bell staging test",
			"selected_track_id": "track_proq", "selected_plugin_id": "plugin_proq",
		},
	})
	if invokeErr != nil || out.Status != "ok" {
		t.Fatalf("B4 delegated tool err=%v result=%#v", invokeErr, out)
	}
	if cleanContextText(out.Result["execution_route"]) != pluginEffectControlCapabilityID {
		t.Fatalf("staging path retained execution authority: %#v", out.Result)
	}
	if writes := vpsForgeStagingEQWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("proposal planning wrote %d plugin parameters", writes)
	}
}

func TestEqualizerHTTPInvokeDelegatesPastStagingToB4(t *testing.T) {
	server, kernel, _, _ := newVPSForgeStagingEQRuntimeServer(t)
	server.pluginEffectControlRuntimeOverride = func(_ context.Context, conversationID string, _ ChatRequest, goal agentruntime.Goal) ChatResponse {
		return pluginEffectProposalTestResponse(conversationID, goal)
	}
	body, err := json.Marshal(harness.InvokeRequest{
		Tool: equalizerCapabilityPlanCommand,
		Args: map[string]any{
			"task": "spectral_region_adjust", "track_id": "track_proq", "plugin_id": "plugin_proq",
			"frequency_hz": 3188.1, "gain_db": 7.5, "q": 3.024, "enabled": true,
		},
		Source: "vpsforge_http_test",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/agent/invoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.handleInvoke(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response harness.InvokeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode HTTP response: %v body=%s", err, recorder.Body.String())
	}
	if response.Status != "ok" || response.Tool != equalizerCapabilityPlanTool || response.CommandName != equalizerCapabilityPlanCommand {
		t.Fatalf("HTTP capability response=%#v", response)
	}
	if cleanContextText(response.Result["execution_route"]) != pluginEffectControlCapabilityID {
		t.Fatalf("HTTP invoke did not create a B4 proposal: %#v", response.Result)
	}
	if writes := vpsForgeStagingEQWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("HTTP planning wrote %d plug-in parameters", writes)
	}
}

func TestVPSForgeStagingEQBridgeReturnsExplicitHighpassGap(t *testing.T) {
	server, kernel, _, _ := newVPSForgeStagingEQRuntimeServer(t)
	request := proQ3StagingBellRequest()
	request.Instruction.SchemaID = spal.EQPassFilterPatchControlID
	request.Instruction.StringParameters = map[string]string{"filter_kind": "highpass"}
	request.Instruction.Parameters = map[string]float64{"enabled": 1, "cutoff_frequency_hz": 80, "slope_db_per_octave": 24}
	resp := server.handleSPALEQV2StructuredRuntime(context.Background(), "staging_gap", ChatRequest{ConversationID: "staging_gap"}, agentruntime.Goal{}, request)
	if resp.NeedsConfirmation || resp.GoalStatus != string(agentruntime.StatusWaitingClarification) || !strings.Contains(strings.ToLower(resp.Reply), "feature matrix") {
		t.Fatalf("highpass gap response=%#v", resp)
	}
	if writes := vpsForgeStagingEQWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("unsupported highpass issued %d writes", writes)
	}
}
