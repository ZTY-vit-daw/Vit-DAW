package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/spallab"
)

func TestSPALReferenceEQNaturalLanguageParserBuildsFiniteSemanticRequest(t *testing.T) {
	request := parseSPALReferenceEQRequest(ChatRequest{Message: "SPAL Reference EQ 测试：目标: track:bass，频率: 92Hz，增益: -2.5dB，Q: 1.2，片段: clip-bass-01"})
	if len(request.Missing) != 0 {
		t.Fatalf("unexpected missing fields: %#v", request.Missing)
	}
	if request.TargetRef != "track:bass" || request.FrequencyHz != 92 || request.GainDB != -2.5 || request.Q != 1.2 || request.Scope.ClipID != "clip-bass-01" {
		t.Fatalf("parser lost explicit semantic fields: %#v", request)
	}
	instruction, err := request.instruction(spallab.ProviderRecord{ID: "record-1", ConformanceEvidence: []string{"conformance:record-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if instruction.SchemaID != spalReferenceEQSchemaID || instruction.Parameters["center_frequency_hz"] != 92 || instruction.Parameters["gain_db"] != -2.5 || instruction.ExpectedSignalChange.Direction != "decrease" {
		t.Fatalf("unexpected semantic instruction: %#v", instruction)
	}
	if instruction.SignalProbeScope.ClipID != "clip-bass-01" || instruction.ExpectedSignalChange.BandLowHz <= 0 || instruction.ExpectedSignalChange.BandHighHz <= instruction.ExpectedSignalChange.BandLowHz {
		t.Fatalf("signal scope/band was not frozen: %#v", instruction)
	}
}

func TestSPALReferenceEQParserAcceptsExplicitTimeRange(t *testing.T) {
	request := parseSPALReferenceEQRequest(ChatRequest{Message: "SPAL Reference EQ 测试：目标: bass，频率: 100Hz，增益: +1.5dB，Q: 0.8，范围: 1.25s-8.5s，尾音: 0.2s"})
	if len(request.Missing) != 0 || request.Scope.StartSeconds == nil || request.Scope.EndSeconds == nil || request.Scope.TailSeconds == nil {
		t.Fatalf("time-range request did not create a frozen probe scope: %#v", request)
	}
	if *request.Scope.StartSeconds != 1.25 || *request.Scope.EndSeconds != 8.5 || *request.Scope.TailSeconds != .2 {
		t.Fatalf("unexpected parsed range: %#v", request.Scope)
	}
}

func TestSPALReferenceEQParserAcceptsBareStaticBellTupleAndGodotTimeContext(t *testing.T) {
	request := parseSPALReferenceEQRequest(ChatRequest{
		Message: "在当前 TDR Nova 上做 92Hz、-2.5dB、Q 1.2 的静态 Bell EQ。",
		Context: map[string]any{
			"selected_track_id":  "track-bass",
			"selected_plugin_id": "plugin-nova",
			"start_seconds":      0,
			"end_seconds":        1,
		},
	})
	if len(request.Missing) != 0 {
		t.Fatalf("bare Static Bell tuple was not complete: %#v", request)
	}
	if request.TargetRef != "track-bass" || request.PluginID != "plugin-nova" || request.FrequencyHz != 92 || request.GainDB != -2.5 || request.Q != 1.2 {
		t.Fatalf("bare Static Bell tuple lost semantic fields: %#v", request)
	}
	if request.Scope.StartSeconds == nil || request.Scope.EndSeconds == nil || *request.Scope.StartSeconds != 0 || *request.Scope.EndSeconds != 1 {
		t.Fatalf("Godot time scope was not retained: %#v", request.Scope)
	}
}

func TestSPALReferenceEQParserUsesExplicitGodotSelectionContext(t *testing.T) {
	request := parseSPALReferenceEQRequest(ChatRequest{
		Message: "SPAL Reference EQ 测试：频率: 92Hz，增益: -2dB，Q: 1.1",
		Context: map[string]any{"selected_track_id": "track-bass", "selected_clip_id": "clip-bass"},
	})
	if len(request.Missing) != 0 || request.TargetRef != "track-bass" || request.Scope.ClipID != "clip-bass" {
		t.Fatalf("selected Godot context was not treated as an explicit target/scope: %#v", request)
	}
}

func TestSPALReferenceEQRejectsSignalClipFromAnotherTrack(t *testing.T) {
	record := spallab.ProviderRecord{ID: "record", Instance: testSPALReferenceEQInstance("track:bass", "bass")}
	state := &kernel.VSPStateResult{LegacyState: map[string]any{"tracks": []any{
		map[string]any{"track_id": "bass", "clips": []any{map[string]any{"clip_id": "bass-clip"}}},
		map[string]any{"track_id": "vocal", "clips": []any{map[string]any{"clip_id": "vocal-clip"}}},
	}}}
	if err := validateSPALReferenceEQSignalScope(state, record, spal.SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe", ClipID: "bass-clip"}); err != nil {
		t.Fatalf("same-track signal scope rejected: %v", err)
	}
	if err := validateSPALReferenceEQSignalScope(state, record, spal.SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe", ClipID: "vocal-clip"}); err == nil {
		t.Fatal("foreign-track clip was accepted as a signal probe scope")
	}
}

func TestSPALReferenceEQRequiresAnExplicitProviderInsteadOfFallback(t *testing.T) {
	records := []spallab.ProviderRecord{
		{ID: "record-a", ProjectUUID: "project-1", Instance: testSPALReferenceEQInstance("track:bass", "bass")},
		{ID: "record-b", ProjectUUID: "project-1", Instance: testSPALReferenceEQInstance("track:bass", "bass")},
	}
	for index := range records {
		records[index].Instance.Metadata = map[string]string{"project_uuid": "project-1"}
	}
	if _, _, err := selectSPALReferenceEQProvider(records, "project-1", "track:missing", ""); err == nil || !strings.Contains(err.Error(), "no_verified_provider") {
		t.Fatalf("missing target became a fallback: %v", err)
	}
	if _, _, err := selectSPALReferenceEQProvider(records, "project-1", "track:bass", ""); err == nil || !strings.Contains(err.Error(), "provider_selection_required") {
		t.Fatalf("multiple instances were selected implicitly: %v", err)
	}
	selected, target, err := selectSPALReferenceEQProvider(records, "project-1", "bass", "record-b")
	if err != nil || selected.ID != "record-b" || target != "track:bass" {
		t.Fatalf("explicit record selection failed: selected=%#v target=%q err=%v", selected, target, err)
	}
}

func TestSPALReferenceEQIsExplicitProductIntentNotB4(t *testing.T) {
	// Now that B4 is implemented, "进行 B4 处理" routes to the B4 capability,
	// not to the SPAL Reference EQ test path.
	if got := inferProjectAwareCapability("进行 B4 处理"); got != lowEndRelationCapabilityID {
		t.Fatalf("B4 intent should route to low-end relation capability, got %q", got)
	}
	if got := inferProjectAwareCapability("请做 SPAL Reference EQ 测试"); got != spalReferenceEQTestCapabilityID {
		t.Fatalf("explicit SPAL Reference EQ intent routed to %q", got)
	}
}

func TestSPALReferenceEQSessionCarriesNoAutoProvisioningConstraint(t *testing.T) {
	runtime := orchestrationruntime.New()
	session, err := runtime.StartSPALReferenceEQTestChatSession("spal-test", "conversation", "project", "Reference EQ", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	if session.Invocation.CapabilityID != spalReferenceEQTestCapabilityID || !containsSPALReferenceEQConstraint(session.Constraints, "no_provider_auto_provisioning") || !containsSPALReferenceEQConstraint(session.Constraints, "reference_eq_test_only") {
		t.Fatalf("SPAL session did not retain constrained product-path ownership: %#v", session)
	}
}

func TestFreshSPALRollbackPhraseDoesNotAttemptToCancelMissingSession(t *testing.T) {
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	response, handled := server.handleCapabilityRuntimeCanary(context.Background(), "conversation", ChatRequest{Message: "SPAL Reference EQ 测试回滚"}, agentruntime.Goal{})
	if !handled || !strings.Contains(response.Reply, "Project-aware Runtime") {
		t.Fatalf("fresh rollback phrase was not delegated to the capability path: handled=%v response=%#v", handled, response)
	}
}

func testSPALReferenceEQInstance(target, track string) spal.ProviderInstance {
	return spal.ProviderInstance{ID: "instance-" + track, ProviderID: "provider", TargetRef: target, TrackID: track, PluginID: "plugin", Status: "verified"}
}

func containsSPALReferenceEQConstraint(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
