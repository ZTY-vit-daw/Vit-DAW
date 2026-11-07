package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/shadow"
)

// VSTI-WRITEGUARD-1 paired tests. Three directions:
//  1. provably non-dynamics surfaces (VSTi instrument role, proven delay /
//     reverb topology) pass the typed generic-write guards;
//  2. owned dynamics surfaces keep the exact pre-fix guard semantics;
//  3. unknown families and read failures stay fail-closed.

// fakeFamilySurfaceKernel answers get_plugin_parameters with a captured
// surface reply (fixture replay).
type fakeFamilySurfaceKernel struct {
	reply map[string]any
	reads int
}

func (fake *fakeFamilySurfaceKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	fake.reads++
	return fake.reply, "", nil
}

func (fake *fakeFamilySurfaceKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func writeguardFixtureServer(t *testing.T, fixture string) (*Server, *fakeFamilySurfaceKernel, harness.InvokeRequest) {
	t.Helper()
	var reply map[string]any
	if err := json.Unmarshal([]byte(fixture), &reply); err != nil {
		t.Fatalf("fixture reply: %v", err)
	}
	fake := &fakeFamilySurfaceKernel{reply: reply}
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	req := harness.InvokeRequest{Tool: "plugin.set_parameter", Args: map[string]any{
		"track_id": firstNonEmptyText(reply, "track_id"), "plugin_id": firstNonEmptyText(reply, "plugin_id"),
		"param_id": "0", "value": .25}}
	return server, fake, req
}

// Direction 1: the three negative-anchor surfaces pass every typed
// generic-write guard after the family exemption.
func TestGenericWriteGuardExemptsInstrumentAndProvenTimeEffectFamilies(t *testing.T) {
	cases := []struct {
		name     string
		fixture  string
		exemption string
	}{
		{name: "grand rhapsody instrument (298 rejection replay)", fixture: writeguardGrandRhapsodyReply, exemption: "template_role=instrument"},
		{name: "h-delay delay (300 s0 run2/run6 t4 forensics replay)", fixture: writeguardHDelayReply, exemption: "proven delay topology"},
		{name: "plates reverb (300 s0 run1 t3 forensics replay)", fixture: writeguardPlatesReply, exemption: "proven reverb topology"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, fake, req := writeguardFixtureServer(t, tc.fixture)
			if resp, blocked := server.guardDeEsserOwnedGenericParameterWrite(context.Background(), req); blocked {
				t.Fatalf("de-esser guard blocked a %s surface: %v", tc.exemption, resp.Error)
			}
			if resp, blocked := server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), req); blocked {
				t.Fatalf("gate/expander guard blocked a %s surface: %v", tc.exemption, resp.Error)
			}
			if resp, blocked := server.guardLimiterOwnedGenericParameterWrite(context.Background(), req); blocked {
				t.Fatalf("limiter guard blocked a %s surface: %v", tc.exemption, resp.Error)
			}
			if fake.reads == 0 {
				t.Fatal("guards never consulted the kernel")
			}
		})
	}
}

// Negative-anchor pins: the replayed surfaces reproduce the exact
// unresolved_* classification that produced the pre-fix rejections, so the
// passes above come from the family exemption and not from fixture drift.
func TestGenericWriteGuardFixturesReproduceUnresolvedClassification(t *testing.T) {
	pins := []struct {
		name    string
		fixture string
		read    func(*Server, context.Context, string, string) error
		code    string
	}{
		{name: "grand rhapsody trips unresolved de-esser", fixture: writeguardGrandRhapsodyReply,
			read: func(s *Server, ctx context.Context, trackID, pluginID string) error {
				_, _, err := s.readLiveDeEsserControlSurface(ctx, trackID, pluginID)
				return err
			}, code: "unresolved_de_esser_surface"},
		{name: "h-delay trips unresolved gate/expander", fixture: writeguardHDelayReply,
			read: func(s *Server, ctx context.Context, trackID, pluginID string) error {
				_, _, err := s.readLiveGateExpanderControlSurface(ctx, trackID, pluginID)
				return err
			}, code: "unresolved_gate_expander_surface"},
		{name: "plates trips unresolved limiter", fixture: writeguardPlatesReply,
			read: func(s *Server, ctx context.Context, trackID, pluginID string) error {
				_, _, err := s.readLiveLimiterControlSurface(ctx, trackID, pluginID)
				return err
			}, code: "unresolved_limiter_surface"},
	}
	for _, pin := range pins {
		t.Run(pin.name, func(t *testing.T) {
			server, _, req := writeguardFixtureServer(t, pin.fixture)
			trackID := firstNonEmptyText(req.Args, "track_id")
			pluginID := firstNonEmptyText(req.Args, "plugin_id")
			err := pin.read(server, context.Background(), trackID, pluginID)
			if err == nil || !strings.Contains(err.Error(), pin.code) {
				t.Fatalf("fixture no longer reproduces the negative anchor %q: err=%v", pin.code, err)
			}
		})
	}
}

// Direction 3: a surface that stays unresolved but carries no provable family
// evidence keeps the fail-closed rejection on all three guarded families.
func TestGenericWriteGuardUnknownFamilyStaysFailClosed(t *testing.T) {
	server, _, req := writeguardFixtureServer(t, writeguardUnknownFamilyReply)
	trackID := firstNonEmptyText(req.Args, "track_id")
	pluginID := firstNonEmptyText(req.Args, "plugin_id")
	if server.genericWriteGuardFamilyExemption(context.Background(), trackID, pluginID) {
		t.Fatal("family exemption rescued a surface with no provable family evidence")
	}
	blocks := []struct {
		name  string
		guard func() (harness.InvokeResponse, bool)
		code  string
	}{
		{name: "de-esser", guard: func() (harness.InvokeResponse, bool) {
			return server.guardDeEsserOwnedGenericParameterWrite(context.Background(), req)
		}, code: "typed_de_esser_surface_unavailable"},
		{name: "gate/expander", guard: func() (harness.InvokeResponse, bool) {
			return server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), req)
		}, code: "typed_gate_expander_surface_unavailable"},
		{name: "limiter", guard: func() (harness.InvokeResponse, bool) {
			return server.guardLimiterOwnedGenericParameterWrite(context.Background(), req)
		}, code: "typed_limiter_surface_unavailable"},
	}
	for _, block := range blocks {
		resp, blocked := block.guard()
		if !blocked {
			t.Fatalf("%s guard let an unknown family through", block.name)
		}
		if got := firstNonEmptyText(resp.Result, "rejection_code"); got != block.code {
			t.Fatalf("%s guard rejection_code=%q want %q", block.name, got, block.code)
		}
		if !strings.Contains(resp.Error, "unresolved_") {
			t.Fatalf("%s guard rejection lost the unresolved classification: %v", block.name, resp.Error)
		}
	}
}

// Hard read failures stay fail-closed: the exemption only applies to the
// unresolved-classification branch, never to kernel read errors.
func TestGenericWriteGuardReadFailureStaysFailClosed(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = &fakeFamilySurfaceKernel{reply: map[string]any{"status": "error", "message": "kernel exploded"}}
	req := harness.InvokeRequest{Tool: "plugin.set_parameter", Args: map[string]any{
		"track_id": "track-1", "plugin_id": "plugin-1", "param_id": "0", "value": .25}}
	resp, blocked := server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), req)
	if !blocked {
		t.Fatal("gate/expander guard passed a surface it could not read")
	}
	if got := firstNonEmptyText(resp.Result, "rejection_code"); got != "typed_gate_expander_surface_unavailable" {
		t.Fatalf("rejection_code=%q", got)
	}
	if !strings.Contains(resp.Error, "parameter_read_failed") {
		t.Fatalf("read failure lost in rejection: %v", resp.Error)
	}
}

// Direction 2: owned dynamics surfaces keep the exact pre-fix guard
// semantics (typed control required for owned params, clean pass for
// unsupported capabilities).
func TestGenericWriteGuardOwnedDynamicsSemanticsUnchanged(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = newFakeGateExpanderKernel()
	ownedRequest := harness.InvokeRequest{Tool: "plugin.set_parameter", Args: map[string]any{
		"track_id": "track-1", "plugin_id": "gate-1", "param_id": "threshold", "value": .2}}
	owned, blocked := server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), ownedRequest)
	if !blocked || firstNonEmptyText(owned.Result, "rejection_code") != "typed_gate_expander_control_required" {
		t.Fatalf("owned=%+v blocked=%v", owned, blocked)
	}
	for _, paramID := range []string{"midi", "channel"} {
		_, blocked := server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), harness.InvokeRequest{Tool: "plugin.set_parameter",
			Args: map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "param_id": paramID, "value": .2}})
		if blocked {
			t.Fatalf("unsupported capability %s leaked into typed ownership", paramID)
		}
	}
}
