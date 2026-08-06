package chat

import (
	"context"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/tools"
)

func TestPluginProfileToolsAreAbsentFromCatalog(t *testing.T) {
	catalog := tools.DefaultCatalog()
	for _, name := range []string{
		"plugin_grabber.get_project_profiles",
		"plugin_grabber.learn_project_profile",
		"plugin_grabber.upsert_project_profile",
		"plugin_grabber.remove_project_profile",
		"plugin_grabber.apply_control",
	} {
		if _, ok := catalog.LookupTool(name); ok {
			t.Fatalf("retired plugin profile tool %q remains callable", name)
		}
	}
	for _, name := range []string{"plugin.get_parameters", "plugin_grabber.explain_controls", "plugin_grabber.apply_eq_edits"} {
		if _, ok := catalog.LookupTool(name); !ok {
			t.Fatalf("retained deterministic/read-only tool %q is missing", name)
		}
	}
}

func TestPluginTreatmentIsObservationOnlyAfterProfileRetirement(t *testing.T) {
	s := &Server{}
	decision := s.resolveMixTreatment(context.Background(), agentloop.MixTreatmentPending{
		ActionKind:    "plugin_treatment",
		ProcessorType: "compressor",
		TargetRef:     "track:1007",
	}, nil)
	if decision.Status != "observation_only" {
		t.Fatalf("plugin treatment status = %q, want observation_only", decision.Status)
	}
	if len(decision.ToolRoute) != 1 || decision.ToolRoute[0] != "plugin_grabber.explain_controls" || len(decision.Command) != 0 {
		t.Fatalf("retired plugin treatment retained execution authority: %+v", decision)
	}
}
