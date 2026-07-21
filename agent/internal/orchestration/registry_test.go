package orchestration

import "testing"

func TestRegistryRequiresExplicitCapabilityVersion(t *testing.T) {
	r, err := NewRegistry(CapabilityDefinition{
		ID:      "static_mix.static_balance.v0",
		Version: "v0",
		Family:  "parametric_transform",
		Effects: []string{"project_mutation", "project_mutation"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Resolve("static_mix.static_balance.v0", "v1"); ok {
		t.Fatal("unregistered capability version must not resolve")
	}
	definition, ok := r.Resolve("static_mix.static_balance.v0", "v0")
	if !ok || len(definition.Effects) != 1 {
		t.Fatalf("unexpected definition: %#v", definition)
	}
	if err := r.Register(definition); err == nil {
		t.Fatal("duplicate capability version must be rejected")
	}
}
