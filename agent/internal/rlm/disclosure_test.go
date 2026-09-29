package rlm

import (
	"encoding/json"
	"strings"
	"testing"
)

func disclosureBoundBinding(t *testing.T) RenderProfileBinding {
	t.Helper()
	binding, err := NewRenderProfileBinding("job-disclose-1", "builtin:apple_music", "2026-09-29T00:00:00Z")
	if err != nil {
		t.Fatalf("NewRenderProfileBinding: %v", err)
	}
	return binding
}

func TestBuildDisclosesRenderProfileBindings(t *testing.T) {
	proj := Build(Input{RenderBindings: []RenderProfileBinding{disclosureBoundBinding(t)}})
	if len(proj.RenderBindings) != 1 {
		t.Fatalf("expected 1 render binding disclosure, got %d", len(proj.RenderBindings))
	}
	row := proj.RenderBindings[0]
	if row.RenderID != "job-disclose-1" || row.ProfileID != "builtin:apple_music" {
		t.Fatalf("disclosure row ids: %+v", row)
	}
	if row.Status != RenderBindingStatusBound {
		t.Fatalf("status=%q want %q", row.Status, RenderBindingStatusBound)
	}
	if row.Profile == nil {
		t.Fatal("bound disclosure must carry the resolved profile vector")
	}
	if row.Profile.Integrated.Max != -16 {
		t.Fatalf("apple music integrated max=%v want -16", row.Profile.Integrated.Max)
	}
	if !strings.Contains(strings.ToLower(row.AssertionSemantics), "target band") {
		t.Fatalf("assertion semantics must state target-band semantics, got %q", row.AssertionSemantics)
	}
	if row.BoundAt != "2026-09-29T00:00:00Z" {
		t.Fatalf("bound_at=%q", row.BoundAt)
	}
}

func TestBuildRenderBindingDisclosureUnresolvableFailsClosed(t *testing.T) {
	ghost := RenderProfileBinding{
		SchemaVersion: RenderBindingSchemaVersion,
		RenderID:      "job-ghost",
		ProfileID:     "builtin:removed_standard",
	}
	disclosures := BuildRenderBindingDisclosure([]RenderProfileBinding{ghost, disclosureBoundBinding(t)})
	if len(disclosures) != 2 {
		t.Fatalf("expected 2 disclosure rows, got %d", len(disclosures))
	}
	if disclosures[0].Status != RenderBindingStatusUnresolvable {
		t.Fatalf("ghost binding must disclose as unresolvable, got %q", disclosures[0].Status)
	}
	if disclosures[0].Profile != nil || disclosures[0].Error == "" {
		t.Fatalf("unresolvable row must have nil profile and an error: %+v", disclosures[0])
	}
	if disclosures[0].AssertionSemantics != "" {
		t.Fatalf("unresolvable row must not assert semantics, got %q", disclosures[0].AssertionSemantics)
	}
	if disclosures[1].Status != RenderBindingStatusBound {
		t.Fatalf("valid binding disclosure status=%q", disclosures[1].Status)
	}

	proj := Build(Input{RenderBindings: []RenderProfileBinding{ghost}})
	found := false
	for _, limitation := range proj.Limitations {
		if limitation == LimitationRenderBindingUnresolvable {
			found = true
		}
	}
	if !found {
		t.Fatalf("unresolvable binding must surface limitation %q, got %v", LimitationRenderBindingUnresolvable, proj.Limitations)
	}
}

func TestBuildWithoutRenderBindingsKeepsLegacyJSONShape(t *testing.T) {
	legacy := Build(Input{})
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "render_profile_bindings") {
		t.Fatalf("legacy projection must not carry render_profile_bindings key: %s", data)
	}
	if legacy.RenderBindings != nil {
		t.Fatalf("legacy projection must keep nil disclosure slot, got %+v", legacy.RenderBindings)
	}
}

func TestContextProjectionMapCarriesRenderBindingDisclosure(t *testing.T) {
	proj := Build(Input{RenderBindings: []RenderProfileBinding{disclosureBoundBinding(t)}})
	asMap := ContextProjectionMap(proj)
	rows, ok := asMap["render_profile_bindings"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("context projection map missing render_profile_bindings: %#v", asMap["render_profile_bindings"])
	}
	first, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("disclosure row type: %#v", rows[0])
	}
	if first["render_id"] != "job-disclose-1" || first["status"] != RenderBindingStatusBound {
		t.Fatalf("disclosure row content: %#v", first)
	}
}

func TestRenderBindingIndexBindingsSnapshotIsSorted(t *testing.T) {
	first, err := NewRenderProfileBinding("job-b", "builtin:spotify", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRenderProfileBinding("job-a", "builtin:ebu_r128", "")
	if err != nil {
		t.Fatal(err)
	}
	idx := NewRenderBindingIndex()
	if err := idx.Bind(first); err != nil {
		t.Fatal(err)
	}
	if err := idx.Bind(second); err != nil {
		t.Fatal(err)
	}
	bindings := idx.Bindings()
	if len(bindings) != 2 || bindings[0].RenderID != "job-a" || bindings[1].RenderID != "job-b" {
		t.Fatalf("bindings snapshot not sorted by render id: %+v", bindings)
	}
}
