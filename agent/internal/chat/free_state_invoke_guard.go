package chat

import (
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
)

// freeStateInvokeMutationReason closes the HTTP invoke boundary for an active
// abstract reasoning loop. Governed materialization uses the internal typed
// workflow after family/instance validation; the public invoke entry cannot
// be used to inject a direct apply, load, or raw parameter write.
func freeStateInvokeMutationReason(req harness.InvokeRequest) string {
	if !freeStateLoopActiveContext(req.Context) {
		return ""
	}
	for _, name := range invokeCommandNames(req) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if name == "daw.invoke" || name == "daw_invoke" {
			return "active free-state reasoning may use only read-only CCB observations; daw.invoke is forbidden"
		}
		if spec, ok := tools.DefaultCatalog().LookupTool(name); ok {
			if spec.MutatesProject || spec.RefreshAfter {
				return "active free-state reasoning may use only read-only CCB observations; mutation tool " + name + " is forbidden"
			}
			continue
		}
		if spec, ok := tools.DefaultCatalog().LookupCommand(name); ok {
			if spec.MutatesProject || spec.RefreshAfter {
				return "active free-state reasoning may use only read-only CCB observations; mutation command " + name + " is forbidden"
			}
			continue
		}
		if freeStateKnownMutationName(name) {
			return "active free-state reasoning may use only read-only CCB observations; mutation tool " + name + " is forbidden"
		}
	}
	return ""
}

func invokeCommandNames(req harness.InvokeRequest) []string {
	seen := map[string]bool{}
	var out []string
	add := func(value any) {
		text, ok := value.(string)
		if !ok {
			return
		}
		text = strings.TrimSpace(text)
		if text != "" && !seen[strings.ToLower(text)] {
			seen[strings.ToLower(text)] = true
			out = append(out, text)
		}
	}
	add(req.Tool)
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				lower := strings.ToLower(strings.TrimSpace(key))
				if lower == "cmd" || lower == "command" || lower == "action" || lower == "tool" {
					add(child)
				}
				if lower == "args" || lower == "payload" || lower == "command" {
					visit(child)
				}
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(req.Command)
	visit(req.Args)
	return out
}

func freeStateKnownMutationName(name string) bool {
	switch name {
	case "plugin.set_parameter", "plugin_set_parameter", "set_plugin_param",
		"plugin.load_to_rack", "rack.add_node", "rack_add_node", "rack.load_plugin", "rack_load_plugin",
		"instantiate_plugin", "plugin.instantiate", "plugin_grabber_load_and_get_params", "plugin_grabber.load_and_get_params",
		"plugin_grabber.apply_control", "plugin_grabber_apply_control",
		"plugin_grabber.apply_eq_edits", "plugin_grabber_apply_eq_edits", "plugin_grabber.set_eq_point", "plugin_grabber_set_eq_point",
		"plugin_grabber.apply_compressor_controls", "plugin_grabber_apply_compressor_controls",
		"plugin_grabber.apply_limiter_controls", "plugin_grabber_apply_limiter_controls",
		"plugin_grabber.apply_gate_expander_controls", "plugin_grabber_apply_gate_expander_controls",
		"plugin_grabber.apply_de_esser_controls", "plugin_grabber_apply_de_esser_controls",
		"plugin_grabber.apply_transient_shaper_controls", "plugin_grabber_apply_transient_shaper_controls",
		"plugin_grabber.apply_multiband_controls", "plugin_grabber_apply_multiband_controls":
		return true
	default:
		return false
	}
}
