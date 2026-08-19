package chat

import (
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
)

// typedPluginApplyConfirmationResponse protects the HTTP handlers that dispatch
// straight into typed controllers before the generic Harness confirmation path.
// It deliberately covers only parameter-writing typed applies; load and generic
// writes continue through their existing Harness gates.
func typedPluginApplyConfirmationResponse(req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if req.Confirmed {
		return harness.InvokeResponse{}, false
	}
	for _, rawName := range invokeCommandNames(req) {
		tool, command := canonicalTypedPluginApply(strings.ToLower(strings.TrimSpace(rawName)))
		if tool == "" {
			continue
		}
		return harness.InvokeResponse{
			Status:               "needs_confirmation",
			Tool:                 tool,
			CommandName:          command,
			RiskLevel:            tools.RiskUndoable,
			RequiresConfirmation: true,
			Preview:              "This typed processor control will write plugin parameters atomically and requires explicit confirmation.",
			Result: map[string]any{
				"status":                "needs_confirmation",
				"mutation_performed":    false,
				"confirmation_required": true,
			},
		}, true
	}
	return harness.InvokeResponse{}, false
}

func canonicalTypedPluginApply(name string) (tool, command string) {
	switch name {
	case "plugin_grabber.apply_eq_edits", "plugin_grabber_apply_eq_edits":
		return pluginGrabberApplyEQEditsTool, pluginGrabberApplyEQEditsCommand
	case "plugin_grabber.set_eq_point", "plugin_grabber_set_eq_point":
		return pluginGrabberSetEQPointTool, pluginGrabberSetEQPointCommand
	case "plugin_grabber.apply_compressor_controls", "plugin_grabber_apply_compressor_controls":
		return pluginGrabberApplyCompressorTool, pluginGrabberApplyCompressorCommand
	case "plugin_grabber.apply_limiter_controls", "plugin_grabber_apply_limiter_controls":
		return pluginGrabberApplyLimiterTool, pluginGrabberApplyLimiterCommand
	case "plugin_grabber.apply_gate_expander_controls", "plugin_grabber_apply_gate_expander_controls":
		return pluginGrabberApplyGateExpanderTool, pluginGrabberApplyGateExpanderCommand
	case "plugin_grabber.apply_de_esser_controls", "plugin_grabber_apply_de_esser_controls":
		return pluginGrabberApplyDeEsserTool, pluginGrabberApplyDeEsserCommand
	case "plugin_grabber.apply_transient_shaper_controls", "plugin_grabber_apply_transient_shaper_controls":
		return pluginGrabberApplyTransientShaperTool, pluginGrabberApplyTransientShaperCommand
	case "plugin_grabber.apply_multiband_controls", "plugin_grabber_apply_multiband_controls":
		return pluginGrabberApplyMultibandTool, pluginGrabberApplyMultibandCommand
	default:
		return "", ""
	}
}
