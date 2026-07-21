package mixstyle

import "testing"

func TestBuiltinsValidate(t *testing.T) {
	ids := IDs()
	if len(ids) != 5 {
		t.Fatalf("built-in styles = %v, want 5", ids)
	}
	for _, id := range ids {
		style, err := Builtin(id)
		if err != nil {
			t.Fatalf("Builtin(%q): %v", id, err)
		}
		if style.ID != id {
			t.Fatalf("Builtin(%q).ID = %q", id, style.ID)
		}
	}
}

func TestMatchUsesExplicitStyleAndDefaultsNeutral(t *testing.T) {
	style, explicit := Match("按现代流行做 B2 静态平衡")
	if !explicit || style.ID != "modern_pop" {
		t.Fatalf("explicit match = %q, %v", style.ID, explicit)
	}
	style, explicit = Match("做 B2 静态平衡")
	if explicit || style.ID != "neutral" {
		t.Fatalf("default match = %q, %v", style.ID, explicit)
	}
	style, explicit = Match("use EDM pop for B2")
	if !explicit || style.ID != "edm_pop" {
		t.Fatalf("longest alias match = %q, %v", style.ID, explicit)
	}
}

func TestValidateRejectsOutOfRangeDimension(t *testing.T) {
	style := Default()
	style.Capabilities.StaticBalance.LowEndAnchorWeight = 1.1
	if err := Validate(style); err == nil {
		t.Fatal("expected out-of-range validation error")
	}
}

func TestCanonicalVMSHasStableCapabilityEnvelope(t *testing.T) {
	style := Default()
	if style.Identity.ID != "neutral" || style.Capabilities.StaticBalance.SchemaVersion != StaticBalanceSchemaVersion || style.Capabilities.PanLayout.SchemaVersion != PanLayoutSchemaVersion {
		t.Fatalf("canonical style = %+v", style)
	}
	if Hash(style) == "" || Hash(style) != Hash(style) {
		t.Fatal("style hash is not stable")
	}
}

func TestValidateRejectsUnsafePanDimension(t *testing.T) {
	style := Default()
	style.Capabilities.PanLayout.SpreadAmount = 1.1
	if err := Validate(style); err == nil {
		t.Fatal("expected pan dimension validation error")
	}
}
