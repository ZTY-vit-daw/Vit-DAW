package agentloop

import "testing"

func TestAllowedToolAcceptsCommandNameForAllowedTool(t *testing.T) {
	if !allowedTool("get_project_state", []string{"project.state"}) {
		t.Fatal("get_project_state should be allowed when project.state is allowed")
	}
	if !allowedTool("control_add_binding", []string{"control.add_binding"}) {
		t.Fatal("control_add_binding should be allowed when control.add_binding is allowed")
	}
	if allowedTool("plugin.search", []string{"project.state"}) {
		t.Fatal("unrelated tool should not be allowed")
	}
}
