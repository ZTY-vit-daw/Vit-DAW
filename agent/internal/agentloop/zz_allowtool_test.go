package agentloop

import "testing"

func TestAllowedToolSetPluginParamAlias(t *testing.T) {
	allowed := []string{"plugin.set_parameter", "get_plugin_parameters"}
	for _, name := range []string{"set_plugin_param", "plugin.set_parameter", "plugin_set_parameter"} {
		if !allowedTool(name, allowed) {
			t.Errorf("allowedTool(%q) = false, want true", name)
		} else {
			t.Logf("allowedTool(%q) = true", name)
		}
	}
}
