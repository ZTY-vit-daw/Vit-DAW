package executionports

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

type pluginGrabberCall struct {
	command string
	args    map[string]any
}

type scriptedPluginGrabberClient struct {
	calls   []pluginGrabberCall
	replies map[string][]*kernel.VSPCommandResult
	errors  map[string][]error
}

func (c *scriptedPluginGrabberClient) SendVSPCommandWithIDs(_ context.Context, command string, args map[string]any, _, _ string) (*kernel.VSPCommandResult, error) {
	c.calls = append(c.calls, pluginGrabberCall{command: command, args: args})
	var result *kernel.VSPCommandResult
	if queue := c.replies[command]; len(queue) > 0 {
		result = queue[0]
		c.replies[command] = queue[1:]
	}
	var err error
	if queue := c.errors[command]; len(queue) > 0 {
		err = queue[0]
		c.errors[command] = queue[1:]
	}
	if result == nil && err == nil {
		return nil, fmt.Errorf("unexpected command %s", command)
	}
	return result, err
}

func pluginGrabberTestAction(expectedIDs ...string) orchestration.Action {
	return orchestration.Action{
		ID: "plugin-effect-1", Command: "plugin_grabber.apply_control.governed", Compensatable: true,
		Args: map[string]any{
			"track_id": "track-1", "plugin_id": "plugin-1", "control": "cut mud",
			"apply_args":             map[string]any{"target": map[string]any{"frequency_hz": 200.0}},
			"resolved_parameters":    []map[string]any{{"parameter_id": "gain", "old_normalized_value": 0.25}},
			"expected_parameter_ids": expectedIDs,
		},
	}
}

func pluginResult(payload map[string]any) *kernel.VSPCommandResult {
	return &kernel.VSPCommandResult{LegacyReply: payload, Revision: 42}
}

func pluginAppliedResult(id string, value float64) *kernel.VSPCommandResult {
	return pluginResult(map[string]any{
		"status":             "ok",
		"applied_parameters": []any{map[string]any{"param_id": id, "new_normalised_value": value}},
	})
}

func pluginParametersResult(id string, value float64) *kernel.VSPCommandResult {
	return pluginResult(map[string]any{
		"status":     "ok",
		"parameters": []any{map[string]any{"param_id": id, "normalized_value": value}},
	})
}

func pluginRollbackResult(id string, value float64) *kernel.VSPCommandResult {
	return pluginResult(map[string]any{
		"status": "ok",
		"readback": map[string]any{
			"parameters": []any{map[string]any{"param_id": id, "normalized_value": value}},
		},
	})
}

func TestPluginGrabberApplyUsesFreshReadback(t *testing.T) {
	client := &scriptedPluginGrabberClient{replies: map[string][]*kernel.VSPCommandResult{
		"plugin.apply_control":  {pluginAppliedResult("gain", 0.6)},
		"plugin.parameters.get": {pluginParametersResult("gain", 0.60005)},
	}}
	receipt, err := (PluginGrabberPort{Client: client}).Apply(context.Background(), pluginGrabberTestAction("gain"), "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || !receipt.EffectivelyOnce {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if len(client.calls) != 2 || client.calls[1].command != "plugin.parameters.get" {
		t.Fatalf("fresh readback was not required: %#v", client.calls)
	}
}

func TestPluginGrabberIdentityMismatchRestoresCompletePreimage(t *testing.T) {
	client := &scriptedPluginGrabberClient{replies: map[string][]*kernel.VSPCommandResult{
		"plugin.apply_control":    {pluginAppliedResult("gain", 0.6)},
		"plugin.set_params_batch": {pluginRollbackResult("gain", 0.25)},
	}}
	receipt, err := (PluginGrabberPort{Client: client}).Apply(context.Background(), pluginGrabberTestAction("wrong-id"), "idem-2")
	if err == nil || !strings.Contains(err.Error(), "full preimage restored") {
		t.Fatalf("expected compensated identity mismatch, got receipt=%#v err=%v", receipt, err)
	}
	if receipt.Status != "failed" {
		t.Fatalf("expected compensated failure, got %#v", receipt)
	}
	if len(client.calls) != 2 || client.calls[1].command != "plugin.set_params_batch" {
		t.Fatalf("rollback not invoked: %#v", client.calls)
	}
	params := pluginRows(client.calls[1].args["parameters"])
	if len(params) != 1 || pluginParameterID(params[0]) != "gain" {
		t.Fatalf("rollback did not use complete frozen preimage: %#v", params)
	}
}

func TestPluginGrabberRollbackReadbackMismatchFailsClosed(t *testing.T) {
	client := &scriptedPluginGrabberClient{replies: map[string][]*kernel.VSPCommandResult{
		"plugin.apply_control":    {pluginAppliedResult("gain", 0.6)},
		"plugin.set_params_batch": {pluginRollbackResult("gain", 0.9)},
	}}
	receipt, err := (PluginGrabberPort{Client: client}).Apply(context.Background(), pluginGrabberTestAction("wrong-id"), "idem-3")
	if err == nil || receipt.Status != "failed_closed" {
		t.Fatalf("expected failed-closed rollback mismatch, got receipt=%#v err=%v", receipt, err)
	}
	if !strings.Contains(receipt.Error, "compensation failed") {
		t.Fatalf("missing compensation failure: %#v", receipt)
	}
}

func TestPluginGrabberReconcileNeverTreatsIdentityPresenceAsPostimageProof(t *testing.T) {
	client := &scriptedPluginGrabberClient{replies: map[string][]*kernel.VSPCommandResult{
		"plugin.parameters.get":   {pluginParametersResult("gain", 0.6)},
		"plugin.set_params_batch": {pluginRollbackResult("gain", 0.25)},
	}}
	receipt, err := (PluginGrabberPort{Client: client}).Reconcile(context.Background(), pluginGrabberTestAction("gain"), "idem-4", orchestration.ProjectCut{})
	if err == nil || receipt.Status != "failed" || !strings.Contains(receipt.Error, "ambiguous") {
		t.Fatalf("expected conservative rollback, got receipt=%#v err=%v", receipt, err)
	}
	if len(client.calls) != 2 || client.calls[1].command != "plugin.set_params_batch" {
		t.Fatalf("ambiguous reconcile did not rollback: %#v", client.calls)
	}
}
