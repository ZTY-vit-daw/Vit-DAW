package harness

// Paired pins for the third-layer (harness) barewrite seam of
// PLUGIN-CONTROL-EVAL-BAREFREE-T1-1 (card 319). The env key
// VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE is the same key the agentloop and
// chat halves (card 318) already honor; these tests pin BOTH directions of
// the harness half: seam open = the T1-type raw plugin parameter write that
// the 318 run recorded as pre_journal_validation_failed
// stage=broad_mix_observe_first_write_guard passes the guard; seam
// unset/other values = the interception is byte-for-byte the pre-seam
// behavior. The seam must not leak to any other broad-mix write command and
// must not exempt retired control surfaces.

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/tools"
)

// t1BarewriteSeamContext reproduces the 318 T1 corpus shape: a natural
// acoustic-mix request with no explicit-plugin-request verb (so
// broadMixNaturalRequest matches via "mix" while ExplicitPluginRequest
// stays false) — exactly the text the 318 payload capture recorded being
// blocked at this guard.
func t1BarewriteSeamContext() map[string]any {
	return map[string]any{"user_message": "请把 Pro-R 混响的 Mix 设为 35%。"}
}

func t1BarewriteParamWriteVariants() []struct {
	name string
	spec tools.CommandSpec
	cmd  map[string]any
} {
	return []struct {
		name string
		spec tools.CommandSpec
		cmd  map[string]any
	}{
		{"set_plugin_param", tools.CommandSpec{CommandName: "set_plugin_param"}, map[string]any{
			"cmd": "set_plugin_param", "track_id": "1007", "plugin_id": "1015", "param_id": "6", "value": 0.35}},
		{"plugin.set_parameter", tools.CommandSpec{CommandName: "plugin.set_parameter"}, map[string]any{
			"cmd": "plugin.set_parameter", "track_id": "1007", "plugin_id": "1015", "param_id": "6", "value": 0.35}},
		{"plugin_set_parameter", tools.CommandSpec{CommandName: "plugin_set_parameter"}, map[string]any{
			"cmd": "plugin_set_parameter", "track_id": "1007", "plugin_id": "1015", "param_id": "6", "value": 0.35}},
	}
}

func TestBarefreeT1HarnessGuardBlocksPluginParamWriteWhenSeamClosed(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "")
	ctx := t1BarewriteSeamContext()
	for _, variant := range t1BarewriteParamWriteVariants() {
		t.Run(variant.name, func(t *testing.T) {
			err := broadMixObserveFirstWriteGuard(ctx, variant.spec, variant.cmd)
			if err == nil {
				t.Fatalf("seam closed: T1-type plugin parameter write escaped the observe-first guard (spec=%+v)", variant.spec)
			}
			if !strings.Contains(err.Error(), "mix.request_observation") {
				t.Fatalf("seam closed: expected the broad-mix observe-first rejection, got %q", err.Error())
			}
		})
	}
}

func TestBarefreeT1HarnessGuardAllowsPluginParamWriteWhenSeamOpen(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	ctx := t1BarewriteSeamContext()
	for _, variant := range t1BarewriteParamWriteVariants() {
		t.Run(variant.name, func(t *testing.T) {
			if err := broadMixObserveFirstWriteGuard(ctx, variant.spec, variant.cmd); err != nil {
				t.Fatalf("seam open: T1-type plugin parameter write still blocked: %v", err)
			}
		})
	}
}

func TestBarefreeT1HarnessSeamStillBlocksOtherBroadMixWrites(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	ctx := t1BarewriteSeamContext()
	blocked := []struct {
		name string
		spec tools.CommandSpec
		cmd  map[string]any
	}{
		{"plugin load", tools.CommandSpec{CommandName: "rack_add_node"}, map[string]any{
			"cmd": "rack_add_node", "track_id": "1007", "plugin_path": `C:\VST3\Other.vst3`}},
		{"volume", tools.CommandSpec{CommandName: "set_volume"}, map[string]any{
			"cmd": "set_volume", "track_id": "1007", "value": -3.0}},
		{"pan", tools.CommandSpec{CommandName: "set_pan"}, map[string]any{
			"cmd": "set_pan", "track_id": "1007", "value": 0.2}},
		{"send amount", tools.CommandSpec{CommandName: "track.send.set_amount"}, map[string]any{
			"cmd": "track_send_set_amount", "track_id": "1007", "target_track_id": "1008", "amount": 0.5}},
	}
	for _, test := range blocked {
		t.Run(test.name, func(t *testing.T) {
			if err := broadMixObserveFirstWriteGuard(ctx, test.spec, test.cmd); err == nil {
				t.Fatalf("seam open leaked past the observe-first gate for a non-parameter write: spec=%+v", test.spec)
			}
		})
	}
}

func TestBarefreeT1HarnessSeamExactValueOnly(t *testing.T) {
	ctx := t1BarewriteSeamContext()
	variant := t1BarewriteParamWriteVariants()[0]
	for _, value := range []string{"", "0", "true", "yes", "on", "1x"} {
		t.Setenv(pluginControlEvalSeamBareWriteEnv, value)
		if pluginControlEvalSeamBareWriteOpen() {
			t.Fatalf("env %q must not open the seam (exact \"1\" contract)", value)
		}
		if err := broadMixObserveFirstWriteGuard(ctx, variant.spec, variant.cmd); err == nil {
			t.Fatalf("env %q: plugin parameter write escaped the guard", value)
		}
	}
	// TrimSpace tolerance mirrors the agentloop/chat halves and the 312/317 seams.
	t.Setenv(pluginControlEvalSeamBareWriteEnv, " 1 ")
	if !pluginControlEvalSeamBareWriteOpen() {
		t.Fatal("env \" 1 \" should open the seam (TrimSpace tolerance)")
	}
	if err := broadMixObserveFirstWriteGuard(ctx, variant.spec, variant.cmd); err != nil {
		t.Fatalf("env \" 1 \": plugin parameter write should pass, got %v", err)
	}
}

func TestBarefreeT1HarnessSeamDoesNotExemptRetiredControlSurfaces(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	spec := tools.CommandSpec{ToolName: "plugin_grabber.apply_control", CommandName: "plugin_grabber_apply_control"}
	cmd := map[string]any{
		"cmd": "plugin_grabber_apply_control", "track_id": "1007", "plugin_id": "1015",
		"control": "eq.cut_region", "target": map[string]any{"freq_hz": 200.0, "gain_db": -2.5},
	}
	ctx := t1BarewriteSeamContext()
	err := broadMixObserveFirstWriteGuard(ctx, spec, cmd)
	if err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("seam open must not exempt retired control surfaces, got: %v", err)
	}
}

// Invoke-level paired pin: the closed seam fails pre-journal at the exact
// stage the 318 T1 corpus recorded (broad_mix_observe_first_write_guard,
// nothing reaches the kernel); the open seam lets the same invoke reach the
// kernel command face.
func TestBarefreeT1HarnessInvokeSeamPairedBothDirections(t *testing.T) {
	invokeReq := InvokeRequest{
		Tool: "set_plugin_param",
		Args: map[string]any{"track_id": "1007", "plugin_id": "1015", "param_id": "6", "value": 0.35},
		Context: t1BarewriteSeamContext(),
		Confirmed: true,
	}

	t.Setenv(pluginControlEvalSeamBareWriteEnv, "")
	closedKernel := &fakeKernelClient{}
	closedHarness := New(nil, nil, nil)
	closedHarness.kernel = closedKernel
	resp, err := closedHarness.Invoke(context.Background(), invokeReq)
	if err == nil || resp.Status != "error" {
		t.Fatalf("seam closed: invoke should fail pre-journal, resp=%+v err=%v", resp, err)
	}
	if !strings.Contains(resp.Error, "mix.request_observation") {
		t.Fatalf("seam closed: expected the broad-mix observe-first rejection, got %+v", resp.Error)
	}
	if len(closedKernel.commands) != 0 {
		t.Fatalf("seam closed: command reached the kernel: %+v", closedKernel.commands)
	}

	t.Setenv(pluginControlEvalSeamBareWriteEnv, "1")
	openKernel := &fakeKernelClient{}
	openHarness := New(nil, nil, nil)
	openHarness.kernel = openKernel
	// seed the read-before-write snapshot the same way a prior
	// get_plugin_parameters kernel reply would (the guard under test sits
	// BEFORE that validation; seeding isolates this test to the seam)
	openHarness.ObservePluginParametersReply(map[string]any{
		"status": "ok", "track_id": "1007", "plugin_id": "1015",
		"parameters": []any{map[string]any{"id": "6", "value_text": "100%", "normalized_value": 1.0}},
	})
	resp, err = openHarness.Invoke(context.Background(), invokeReq)
	if err != nil || resp.Status != "ok" {
		t.Fatalf("seam open: same invoke should reach the kernel face, resp=%+v err=%v", resp, err)
	}
	foundParamWrite := false
	for _, command := range openKernel.commands {
		if name, _ := command["cmd"].(string); name == "set_plugin_param" {
			foundParamWrite = true
		}
	}
	if !foundParamWrite {
		t.Fatalf("seam open: set_plugin_param not in kernel commands: %+v", openKernel.commands)
	}
}
