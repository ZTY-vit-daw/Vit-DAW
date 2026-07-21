package chat

import (
	"bytes"
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
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/vps"
)

type vpsDraftTestKernel struct {
	params                    map[string]float64
	commands                  []map[string]any
	readCount                 int
	nonControllableAfterReads int
	surfaceChangesAfterReads  int
	failSetParameterID        string
}

func (k *vpsDraftTestKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	copy := map[string]any{}
	for key, value := range command {
		copy[key] = value
	}
	k.commands = append(k.commands, copy)
	switch cleanContextText(command["cmd"]) {
	case "get_plugin_parameters":
		k.readCount++
		return k.parameterReply(), "{}", nil
	case "set_plugin_param":
		id := cleanContextText(command["param_id"])
		if id != "" && id == k.failSetParameterID {
			return map[string]any{"status": "error", "message": "forced test write failure"}, "{}", nil
		}
		value, ok := vpsFiniteNumber(command["normalized_value"])
		if !ok {
			return map[string]any{"status": "error", "message": "missing normalized value"}, "{}", nil
		}
		if _, found := k.params[id]; !found {
			return map[string]any{"status": "error", "message": "unknown parameter"}, "{}", nil
		}
		k.params[id] = value
		return map[string]any{"status": "ok", "param_id": id, "new_normalised_value": value}, "{}", nil
	default:
		return map[string]any{"status": "error", "message": "unexpected command " + cleanContextText(command["cmd"])}, "{}", nil
	}
}

func (k *vpsDraftTestKernel) parameterReply() map[string]any {
	ids := make([]string, 0, len(k.params))
	for id := range k.params {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parameters := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		probeLabel := id
		if k.surfaceChangesAfterReads > 0 && k.readCount >= k.surfaceChangesAfterReads {
			probeLabel += " changed"
		}
		isDiscrete := k.surfaceChangesAfterReads > 0 && k.readCount >= k.surfaceChangesAfterReads && id == "gain"
		row := map[string]any{
			"id":                id,
			"param_id":          id,
			"name":              id,
			"normalized_value":  k.params[id],
			"host_controllable": true,
			"is_discrete":       isDiscrete,
			"value_text":        fmt.Sprintf("%.3f", k.params[id]),
			"min":               0.0,
			"max":               1.0,
			"display_probe":     map[string]any{"label": probeLabel},
		}
		if id == "gain" && k.nonControllableAfterReads > 0 && k.readCount >= k.nonControllableAfterReads {
			row["host_controllable"] = false
		}
		parameters = append(parameters, row)
	}
	return map[string]any{
		"status":    "ok",
		"track_id":  "track_vps",
		"plugin_id": "plugin_vps",
		"plugin_identity": map[string]any{
			"manufacturer":  "Example Audio",
			"plugin_name":   "Draft Test EQ",
			"plugin_format": "VST3",
			"version":       "1.0.0",
			"profile_key":   "draft_test_eq",
		},
		"parameters": parameters,
	}
}

func newVPSDraftTestServer(t *testing.T, marked bool, nonControllableAfterReads int, expectedParameterSurface ...string) (*Server, *vpsDraftTestKernel, *vps.Library, vps.VPSDocument) {
	t.Helper()
	t.Setenv("VIT_VPS_LIBRARY_V3_PATH", filepath.Join(t.TempDir(), "vps_library_v3.json"))
	t.Setenv(vpsDraftTestReportsDirectoryEnv, filepath.Join(t.TempDir(), "draft_test_reports"))
	t.Setenv("VIT_AGENT_JOURNAL_PATH", "memory")
	library, err := vps.OpenDefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	document := vps.NewDraft(vps.PluginIdentity{
		Manufacturer: "Example Audio",
		Name:         "Draft Test EQ",
		Format:       "VST3",
		Version:      "1.0.0",
		ProfileKey:   "draft_test_eq",
	}, time.Now().UTC())
	document.ID = "vps_draft_test_candidate"
	if len(expectedParameterSurface) > 0 {
		document.PluginIdentity.Fingerprint.ParameterSurface = expectedParameterSurface[0]
	}
	mapping := vps.ControlSurfaceMapping{
		ComponentID:    "b2",
		SemanticSlot:   "gain",
		ParameterID:    "gain",
		Label:          "Band 2 Gain",
		BindingStatus:  "",
		ExecutionScope: "",
	}
	if marked {
		mapping.BindingStatus = vpsDraftTestOnlyBindingStatus
		mapping.ExecutionScope = vpsDraftTestOnlyScope
	}
	frequencyMapping := vps.ControlSurfaceMapping{
		ComponentID:    "b2",
		SemanticSlot:   "frequency",
		ParameterID:    "frequency",
		Label:          "Band 2 Frequency",
		BindingStatus:  mapping.BindingStatus,
		ExecutionScope: mapping.ExecutionScope,
	}
	qMapping := vps.ControlSurfaceMapping{
		ComponentID:    "b2",
		SemanticSlot:   "q",
		ParameterID:    "q",
		Label:          "Band 2 Q",
		BindingStatus:  mapping.BindingStatus,
		ExecutionScope: mapping.ExecutionScope,
	}
	document.ControlSurface.Mappings = []vps.ControlSurfaceMapping{mapping, frequencyMapping, qMapping}
	if _, added, err := library.ImportDraft(document); err != nil || !added {
		t.Fatalf("ImportDraft added=%v err=%v", added, err)
	}
	kernel := &vpsDraftTestKernel{
		params:                    map[string]float64{"gain": 0.50, "frequency": 0.42, "q": 0.34, "enable": 1.0},
		nonControllableAfterReads: nonControllableAfterReads,
	}
	server := New(nil, shadow.New(nil), nil)
	server.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	server.vpsLibrary = library
	return server, kernel, library, document
}

func postVPSDraftTestJSON(t *testing.T, handler http.Handler, path string, payload map[string]any) (int, map[string]any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	var result map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode %s response: %v body=%s", path, err, recorder.Body.String())
	}
	return recorder.Code, result
}

func vpsDraftTestSetWriteCount(commands []map[string]any) int {
	count := 0
	for _, command := range commands {
		if cleanContextText(command["cmd"]) == "set_plugin_param" {
			count++
		}
	}
	return count
}

func TestVPSDraftTestRequiresConfirmationAndRollsBack(t *testing.T) {
	server, kernel, library, document := newVPSDraftTestServer(t, true, 0)
	handler := server.Routes()
	prepare := map[string]any{
		"vps_id":               document.ID,
		"target":               map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":          "b2.gain",
		"requested_normalized": 0.25,
	}
	status, prepared := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", prepare)
	if status != http.StatusAccepted || prepared["status"] != "needs_explicit_confirmation" {
		t.Fatalf("prepare status=%d result=%#v", status, prepared)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("read-only preparation wrote %d parameters: %#v", writes, kernel.commands)
	}
	ticketID := cleanContextText(prepared["ticket_id"])
	if ticketID == "" {
		t.Fatalf("prepare omitted ticket: %#v", prepared)
	}
	status, denied := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/execute", map[string]any{
		"ticket_id":           ticketID,
		"confirmation_phrase": "no",
	})
	if status != http.StatusForbidden || !stringsContains(cleanContextText(denied["error"]), "no write") {
		t.Fatalf("denied execute status=%d result=%#v", status, denied)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("denied execution wrote %d parameters: %#v", writes, kernel.commands)
	}
	status, executed := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/execute", map[string]any{
		"ticket_id":           ticketID,
		"confirmation_phrase": vpsDraftTestConfirmationPhrase,
	})
	if status != http.StatusOK || executed["status"] != "passed_transport_only" {
		t.Fatalf("execute status=%d result=%#v", status, executed)
	}
	if math.Abs(kernel.params["gain"]-0.50) > 0.0001 {
		t.Fatalf("gain was not restored: %#v", kernel.params)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes < 2 {
		t.Fatalf("expected probe and rollback writes, got %d commands=%#v", writes, kernel.commands)
	}
	if reportPath := cleanContextText(executed["report_path"]); reportPath == "" {
		t.Fatalf("execution omitted report path: %#v", executed)
	} else if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report path %s: %v", reportPath, err)
	}
	catalog, err := library.Catalog()
	if err != nil || len(catalog.Entries) != 0 {
		t.Fatalf("Draft test must not create a Catalog entry: catalog=%#v err=%v", catalog, err)
	}
}

func TestVPSDraftTestRejectsMappingsWithoutTestOnlyMarkers(t *testing.T) {
	server, kernel, _, document := newVPSDraftTestServer(t, false, 0)
	status, result := postVPSDraftTestJSON(t, server.Routes(), "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":               document.ID,
		"target":               map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":          "b2.gain",
		"requested_normalized": 0.25,
	})
	if status != http.StatusBadRequest || !stringsContains(cleanContextText(result["error"]), "explicitly marked") {
		t.Fatalf("unmarked mapping response=%d %#v", status, result)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("unmarked mapping wrote %d parameters: %#v", writes, kernel.commands)
	}
}

func TestVPSDraftTestHardFailureRejectsCurrentRevisionWithoutWriting(t *testing.T) {
	// Preparation sees a valid parameter.  The execution-side fresh read then
	// sees it as non-controllable, proving that the executor rejects the
	// mapping rather than attempting a stale write.
	server, kernel, _, document := newVPSDraftTestServer(t, true, 2)
	handler := server.Routes()
	status, prepared := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":               document.ID,
		"target":               map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":          "b2.gain",
		"requested_normalized": 0.25,
	})
	if status != http.StatusAccepted {
		t.Fatalf("prepare status=%d result=%#v", status, prepared)
	}
	status, failed := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/execute", map[string]any{
		"ticket_id":           cleanContextText(prepared["ticket_id"]),
		"confirmation_phrase": vpsDraftTestConfirmationPhrase,
	})
	if status != http.StatusConflict {
		t.Fatalf("hard failure status=%d result=%#v", status, failed)
	}
	report := mapValue(failed["report"])
	if cleanContextText(report["failure_code"]) != "mapped_parameter_not_host_controllable" || cleanContextText(report["mapping_disposition"]) != "rejected_for_this_vps_revision" {
		t.Fatalf("hard failure report=%#v", report)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("hard failed mapping wrote %d parameters: %#v", writes, kernel.commands)
	}
	status, rejected := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":               document.ID,
		"target":               map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":          "b2.gain",
		"requested_normalized": 0.25,
	})
	if status != http.StatusConflict || rejected["status"] != "rejected" {
		t.Fatalf("rejected revision status=%d result=%#v", status, rejected)
	}
}

func TestVPSDraftTestRequiresExactSurfaceMismatchAcknowledgment(t *testing.T) {
	server, kernel, _, document := newVPSDraftTestServer(t, true, 0, "sha256:draft-observation-does-not-match-live-host")
	handler := server.Routes()
	request := map[string]any{
		"vps_id":                          document.ID,
		"target":                          map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":                     "b2.gain",
		"requested_normalized":            0.25,
		"surface_mismatch_acknowledgment": vpsDraftTestSurfaceMismatchAcknowledgmentPhrase + " ",
	}
	status, denied := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", request)
	if status != http.StatusConflict || cleanContextText(denied["surface_compatibility"]) != vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged {
		t.Fatalf("unacknowledged mismatch response=%d %#v", status, denied)
	}
	if cleanContextText(denied["expected_parameter_surface"]) != "sha256:draft-observation-does-not-match-live-host" || cleanContextText(denied["observed_parameter_surface"]) == "" {
		t.Fatalf("unacknowledged mismatch omitted fingerprints: %#v", denied)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("unacknowledged mismatch wrote %d parameters: %#v", writes, kernel.commands)
	}
	request["surface_mismatch_acknowledgment"] = vpsDraftTestSurfaceMismatchAcknowledgmentPhrase
	status, prepared := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", request)
	if status != http.StatusAccepted || prepared["status"] != "needs_explicit_confirmation" {
		t.Fatalf("acknowledged mismatch prepare response=%d %#v", status, prepared)
	}
	if cleanContextText(prepared["surface_compatibility"]) != vpsDraftTestSurfaceCompatibilityMismatchAcknowledged || prepared["surface_mismatch_acknowledged"] != true {
		t.Fatalf("acknowledged mismatch was not marked Draft-only: %#v", prepared)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("acknowledged mismatch preparation wrote %d parameters: %#v", writes, kernel.commands)
	}
}

func TestVPSDraftTestRejectsChangedObservedSurfaceBeforeWriting(t *testing.T) {
	server, kernel, _, document := newVPSDraftTestServer(t, true, 0, "sha256:draft-observation-does-not-match-live-host")
	handler := server.Routes()
	status, prepared := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":                          document.ID,
		"target":                          map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":                     "b2.gain",
		"requested_normalized":            0.25,
		"surface_mismatch_acknowledgment": vpsDraftTestSurfaceMismatchAcknowledgmentPhrase,
	})
	if status != http.StatusAccepted {
		t.Fatalf("prepare status=%d result=%#v", status, prepared)
	}
	// The ticket's fresh execute read will now produce a new surface hash.  The
	// executor must reject that change before issuing even the probe write.
	kernel.surfaceChangesAfterReads = kernel.readCount + 1
	status, failed := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/execute", map[string]any{
		"ticket_id":           cleanContextText(prepared["ticket_id"]),
		"confirmation_phrase": vpsDraftTestConfirmationPhrase,
	})
	if status != http.StatusConflict {
		t.Fatalf("changed surface status=%d result=%#v", status, failed)
	}
	report := mapValue(failed["report"])
	if cleanContextText(report["failure_code"]) != "target_parameter_surface_changed_after_prepare" {
		t.Fatalf("changed surface report=%#v", report)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("changed surface wrote %d parameters: %#v", writes, kernel.commands)
	}
}

func TestVPSDraftTestManualWitnessHoldsUntilExplicitRollbackAndRecoversAfterRestart(t *testing.T) {
	server, kernel, library, document := newVPSDraftTestServer(t, true, 0)
	handler := server.Routes()
	status, prepared := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":               document.ID,
		"target":               map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":          "b2.gain",
		"requested_normalized": 0.25,
		"observation_mode":     vpsDraftTestObservationModeManualWitness,
	})
	if status != http.StatusAccepted || cleanContextText(prepared["observation_mode"]) != vpsDraftTestObservationModeManualWitness {
		t.Fatalf("manual prepare status=%d result=%#v", status, prepared)
	}
	status, observing := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/execute", map[string]any{
		"ticket_id":           cleanContextText(prepared["ticket_id"]),
		"confirmation_phrase": vpsDraftTestConfirmationPhrase,
	})
	if status != http.StatusAccepted || cleanContextText(observing["status"]) != vpsDraftTestObservationStateAwaiting {
		t.Fatalf("manual execute status=%d result=%#v", status, observing)
	}
	if math.Abs(kernel.params["gain"]-0.25) > 0.0001 {
		t.Fatalf("manual witness did not hold probe value: %#v", kernel.params)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 1 {
		t.Fatalf("manual witness must only issue the probe write before rollback, got %d commands=%#v", writes, kernel.commands)
	}
	sessionID := cleanContextText(observing["observation_session_id"])
	if sessionID == "" {
		t.Fatalf("manual execute omitted observation session: %#v", observing)
	}
	reportRoot := vpsDraftTestReportRoot(library.Path())
	if session, err := vpsDraftTestLoadObservationSession(reportRoot, sessionID); err != nil || session.State != vpsDraftTestObservationStateAwaiting || session.Report.Preimage == nil {
		t.Fatalf("manual preimage recovery record err=%v session=%#v", err, session)
	}
	status, blocked := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":               document.ID,
		"target":               map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"mapping_key":          "b2.gain",
		"requested_normalized": 0.75,
	})
	if status != http.StatusConflict || cleanContextText(blocked["status"]) != "active_observation_requires_rollback" {
		t.Fatalf("active manual observation did not block another prepare: %d %#v", status, blocked)
	}
	writesBeforeDeniedRollback := vpsDraftTestSetWriteCount(kernel.commands)
	status, denied := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/rollback", map[string]any{
		"observation_session_id": sessionID,
		"confirmation_phrase":    "no",
	})
	if status != http.StatusForbidden || vpsDraftTestSetWriteCount(kernel.commands) != writesBeforeDeniedRollback || math.Abs(kernel.params["gain"]-0.25) > 0.0001 {
		t.Fatalf("denied manual rollback changed state: status=%d result=%#v params=%#v", status, denied, kernel.params)
	}
	// A fresh Server instance deliberately has no in-memory session.  Successful
	// rollback proves that the persisted preimage survives an Agent restart.
	restarted := New(nil, shadow.New(nil), nil)
	restarted.harness = harness.NewWithSender(kernel, shadow.New(nil), nil)
	restarted.vpsLibrary = library
	status, rolledBack := postVPSDraftTestJSON(t, restarted.Routes(), "/agent/vps/draft-tests/rollback", map[string]any{
		"observation_session_id": sessionID,
		"confirmation_phrase":    vpsDraftTestManualRollbackConfirmationPhrase,
	})
	if status != http.StatusOK || cleanContextText(rolledBack["status"]) != "passed_transport_only" {
		t.Fatalf("manual rollback status=%d result=%#v", status, rolledBack)
	}
	if math.Abs(kernel.params["gain"]-0.50) > 0.0001 {
		t.Fatalf("manual rollback did not restore preimage: %#v", kernel.params)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes < writesBeforeDeniedRollback+1 {
		t.Fatalf("manual rollback did not issue its restore write: %#v", kernel.commands)
	}
	if sessions, err := vpsDraftTestListObservationSessions(reportRoot); err != nil || len(sessions) != 0 {
		t.Fatalf("manual recovery record remained after a passed rollback: err=%v sessions=%#v", err, sessions)
	}
	if reportPath := cleanContextText(rolledBack["report_path"]); reportPath == "" {
		t.Fatalf("manual rollback omitted final report: %#v", rolledBack)
	} else if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("manual rollback report %s: %v", reportPath, err)
	}
}

func TestVPSDraftTestManualSceneHoldsMultipleChangesAndRollsBack(t *testing.T) {
	server, kernel, _, document := newVPSDraftTestServer(t, true, 0)
	handler := server.Routes()
	changes := []any{
		map[string]any{"mapping_key": "b2.frequency", "requested_normalized": 0.65},
		map[string]any{"mapping_key": "b2.q", "requested_normalized": 0.75},
		map[string]any{"mapping_key": "b2.gain", "requested_normalized": 0.25},
	}
	status, prepared := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":           document.ID,
		"target":           map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"scenario_id":      "b2_bell_scene",
		"changes":          changes,
		"observation_mode": vpsDraftTestObservationModeManualWitness,
	})
	if status != http.StatusAccepted || cleanContextText(prepared["scenario_id"]) != "b2_bell_scene" || len(mapRowsValue(prepared["planned_changes"])) != 3 {
		t.Fatalf("manual scene prepare status=%d result=%#v", status, prepared)
	}
	status, observing := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/execute", map[string]any{
		"ticket_id":           cleanContextText(prepared["ticket_id"]),
		"confirmation_phrase": vpsDraftTestConfirmationPhrase,
	})
	if status != http.StatusAccepted || cleanContextText(observing["status"]) != vpsDraftTestObservationStateAwaiting {
		t.Fatalf("manual scene execute status=%d result=%#v", status, observing)
	}
	if math.Abs(kernel.params["frequency"]-0.65) > 0.0001 || math.Abs(kernel.params["q"]-0.75) > 0.0001 || math.Abs(kernel.params["gain"]-0.25) > 0.0001 {
		t.Fatalf("manual scene did not hold all requested values: %#v", kernel.params)
	}
	receipts := mapRowsValue(mapValue(observing["write_readback"])["receipts"])
	if len(receipts) != 3 || cleanContextText(receipts[0]["mapping_key"]) != "b2.frequency" || cleanContextText(receipts[1]["mapping_key"]) != "b2.q" || cleanContextText(receipts[2]["mapping_key"]) != "b2.gain" {
		t.Fatalf("manual scene omitted ordered per-change readbacks: %#v", receipts)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 3 {
		t.Fatalf("manual scene expected exactly three held writes before rollback, got %d commands=%#v", writes, kernel.commands)
	}
	status, rolledBack := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/rollback", map[string]any{
		"observation_session_id": cleanContextText(observing["observation_session_id"]),
		"confirmation_phrase":    vpsDraftTestManualRollbackConfirmationPhrase,
	})
	if status != http.StatusOK || cleanContextText(rolledBack["status"]) != "passed_transport_only" {
		t.Fatalf("manual scene rollback status=%d result=%#v", status, rolledBack)
	}
	if math.Abs(kernel.params["frequency"]-0.42) > 0.0001 || math.Abs(kernel.params["q"]-0.34) > 0.0001 || math.Abs(kernel.params["gain"]-0.50) > 0.0001 {
		t.Fatalf("manual scene rollback did not restore all values: %#v", kernel.params)
	}
	report := mapValue(rolledBack["report"])
	if cleanContextText(report["scenario_id"]) != "b2_bell_scene" || len(mapRowsValue(report["requested_changes"])) != 3 || len(mapRowsValue(report["mappings"])) != 3 {
		t.Fatalf("manual scene report omitted the full scene plan: %#v", report)
	}
}

func TestVPSDraftTestSceneFailureRollsBackEarlierWrites(t *testing.T) {
	server, kernel, _, document := newVPSDraftTestServer(t, true, 0)
	handler := server.Routes()
	status, prepared := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":           document.ID,
		"target":           map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"changes":          []any{map[string]any{"mapping_key": "b2.gain", "requested_normalized": 0.25}, map[string]any{"mapping_key": "b2.frequency", "requested_normalized": 0.65}},
		"observation_mode": vpsDraftTestObservationModeManualWitness,
	})
	if status != http.StatusAccepted {
		t.Fatalf("scene failure prepare status=%d result=%#v", status, prepared)
	}
	kernel.failSetParameterID = "frequency"
	status, failed := postVPSDraftTestJSON(t, handler, "/agent/vps/draft-tests/execute", map[string]any{
		"ticket_id":           cleanContextText(prepared["ticket_id"]),
		"confirmation_phrase": vpsDraftTestConfirmationPhrase,
	})
	if status != http.StatusConflict {
		t.Fatalf("scene failure execute status=%d result=%#v", status, failed)
	}
	report := mapValue(failed["report"])
	if cleanContextText(report["failure_code"]) != "write_readback_failed" || !boolValue(mapValue(report["rollback"])["passed"]) {
		t.Fatalf("scene failure report=%#v", report)
	}
	if math.Abs(kernel.params["gain"]-0.50) > 0.0001 || math.Abs(kernel.params["frequency"]-0.42) > 0.0001 {
		t.Fatalf("scene failure did not restore earlier write: %#v", kernel.params)
	}
}

func TestVPSDraftTestRejectsBypassInMultiParameterSceneBeforeWriting(t *testing.T) {
	server, kernel, _, document := newVPSDraftTestServer(t, true, 0)
	status, result := postVPSDraftTestJSON(t, server.Routes(), "/agent/vps/draft-tests/prepare", map[string]any{
		"vps_id":           document.ID,
		"target":           map[string]any{"track_id": "track_vps", "plugin_id": "plugin_vps"},
		"changes":          []any{map[string]any{"mapping_key": "b2.gain", "requested_normalized": 0.25}, map[string]any{"mapping_key": "io.bypass", "requested_normalized": 1.0}},
		"observation_mode": vpsDraftTestObservationModeManualWitness,
	})
	if status != http.StatusBadRequest || !stringsContains(cleanContextText(result["error"]), "io.bypass must be tested") {
		t.Fatalf("bypass scene response=%d %#v", status, result)
	}
	if writes := vpsDraftTestSetWriteCount(kernel.commands); writes != 0 {
		t.Fatalf("bypass scene preparation wrote %d parameters: %#v", writes, kernel.commands)
	}
}

func stringsContains(text, fragment string) bool {
	return strings.Contains(text, fragment)
}
