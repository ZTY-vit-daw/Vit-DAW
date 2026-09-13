package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-zeromq/zmq4"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/shadow"
)

// PCA-FULLACCESS-1 (transport half): the deterministic journey probe posts a
// bare rack.add_node with no authority in the body, exactly like an agent tool
// call. The authority mode is server-owned, so /agent/invoke binds the server's
// own state onto the harness context; without that binding the PCA load gate
// cannot tell a granted full-access session from a manual one and dead-ends the
// load. Manual mode must stay untouched.

func pcaFullAccessInvokeFixture(t *testing.T, identifier string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "pca-v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "pca-v2.json"))
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", filepath.Join(root, "missing_plugin_semantics.json"))

	bundle := filepath.Join(root, "PCA Transport EQ.vst3")
	if err := os.WriteFile(bundle, []byte("pca-transport-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := processorattestation.FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: processorattestation.Subject{Name: "PCA Transport EQ", Manufacturer: "Vendor", Format: "VST3", Identifier: identifier, InstalledPath: bundle},
		BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{{
			ReceiptID: "pca-transport", Kind: "test", SHA256: "sha256:" + strings.Repeat("b", 64), ObservedAt: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
		}},
	}, "pca-transport-test"); err != nil {
		t.Fatal(err)
	}
	return bundle
}

// startReplyKernel stands in for the kernel on the real transport so the test
// covers the whole HTTP -> chat server -> harness -> kernel path rather than a
// mocked harness.
func startReplyKernel(t *testing.T, reply map[string]any) string {
	t.Helper()
	socket := zmq4.NewRep(context.Background())
	if err := socket.Listen("tcp://127.0.0.1:0"); err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	address := socket.Addr()
	if address == nil {
		t.Fatal("reply kernel has no bound address")
	}
	endpoint := "tcp://" + address.String()
	payload, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			if _, err := socket.Recv(); err != nil {
				return
			}
			if err := socket.Send(zmq4.NewMsg(payload)); err != nil {
				return
			}
		}
	}()
	return endpoint
}

func pcaFullAccessProbeBody(identifier string) *bytes.Reader {
	body, _ := json.Marshal(map[string]any{
		"tool":      "rack_add_node",
		"source":    "journey1_driver",
		"confirmed": true,
		"command": map[string]any{
			"cmd": "rack_add_node", "plugin_identifier": identifier, "track_id": "1007", "x": 0, "y": 0, "zone_id": "journey1_zone",
		},
	})
	return bytes.NewReader(body)
}

func invokeProbe(t *testing.T, server *Server, identifier string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	server.handleInvoke(recorder, httptest.NewRequest(http.MethodPost, "/agent/invoke", pcaFullAccessProbeBody(identifier)))
	return recorder.Code, recorder.Body.String()
}

func TestInvokeStampsServerAuthorityModeForFullAccessLoad(t *testing.T) {
	const identifier = "VST3-PCA-Transport-Probe-1"
	pcaFullAccessInvokeFixture(t, identifier)
	endpoint := startReplyKernel(t, map[string]any{"status": "ok", "plugin_id": "plugin-1", "plugin_identifier": identifier})

	granted := New(kernel.New(endpoint, 10*time.Second), shadow.New(nil), nil)
	granted.authorityMode = authorityModeFull
	grantedCode, grantedBody := invokeProbe(t, granted, identifier)
	if grantedCode != http.StatusOK || strings.Contains(grantedBody, "pca_load_gate") {
		t.Fatalf("granted full access invoke status=%d body=%s", grantedCode, grantedBody)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(grantedBody), &response); err != nil {
		t.Fatal(err)
	}
	if firstStringFromMap(response, "status") != "ok" {
		t.Fatalf("granted full access invoke response=%s", grantedBody)
	}

	manual := New(kernel.New(endpoint, 10*time.Second), shadow.New(nil), nil)
	manualCode, manualBody := invokeProbe(t, manual, identifier)
	if !strings.Contains(manualBody, "missing non-forgeable exact selection authorization") {
		t.Fatalf("manual mode invoke must stay byte-identical: status=%d body=%s", manualCode, manualBody)
	}
}

func TestInvokeAuthorityStampLeavesCallerAssertedModeInPlace(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.authorityMode = authorityModeFull

	manualRequest := harness.InvokeRequest{Context: map[string]any{"authority_mode": authorityModeManual, "authority_mode_explicit": true}}
	if err := server.validateInvokeAuthority(manualRequest.Context); err != nil {
		t.Fatalf("manual assertion must be accepted: %v", err)
	}
	server.stampInvokeAuthorityMode(&manualRequest)
	if firstStringFromMap(manualRequest.Context, "authority_mode") != authorityModeManual {
		t.Fatalf("caller-asserted manual mode was overwritten: %+v", manualRequest.Context)
	}

	empty := harness.InvokeRequest{}
	server.stampInvokeAuthorityMode(&empty)
	if firstStringFromMap(empty.Context, "authority_mode") != authorityModeFull || !boolValue(empty.Context["authority_mode_explicit"]) {
		t.Fatalf("server mode was not stamped: %+v", empty.Context)
	}

	forged := harness.InvokeRequest{Context: map[string]any{"authority_mode": authorityModeFull, "authority_mode_explicit": true}}
	if err := server.validateInvokeAuthority(forged.Context); err != nil {
		t.Fatalf("granted full access assertion: %v", err)
	}
	ungranted := New(nil, shadow.New(nil), nil)
	ungranted.authorityMode = authorityModeManual
	if err := ungranted.validateInvokeAuthority(forged.Context); err == nil {
		t.Fatal("full access asserted while the authority control is manual must be rejected")
	}
}
