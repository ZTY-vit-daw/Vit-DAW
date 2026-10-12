package tools

import "testing"

// M3-KERNEL-1：automation 曲线三工具注册面——get=RiskDirect 直读；两写命令
// RiskUndoable+确认+改工程+可撤销+刷新（与 set_plugin_param 同族姿态）；
// 下划线别名可解析；set_points InputSchema 声明 t/c/region 的越界约束。
func TestAutomationCurveToolsRegistered(t *testing.T) {
	catalog := DefaultCatalog()

	get, ok := catalog.LookupCommand("automation.get_curve")
	if !ok {
		t.Fatal("automation.get_curve missing")
	}
	if get.ToolName != "automation.get_curve" || get.RiskLevel != RiskDirect {
		t.Fatalf("automation.get_curve metadata = %+v", get)
	}
	if get.MutatesProject || get.SupportsUndo || get.RequiresConfirmation || get.RefreshAfter {
		t.Fatalf("automation.get_curve must be a pure read: %+v", get)
	}
	if len(get.RequiredTargetIDs) != 2 || get.RequiredTargetIDs[0] != "plugin_id" || get.RequiredTargetIDs[1] != "param_id" {
		t.Fatalf("automation.get_curve required ids = %+v", get.RequiredTargetIDs)
	}

	set, ok := catalog.LookupCommand("automation.set_points")
	if !ok {
		t.Fatal("automation.set_points missing")
	}
	if set.RiskLevel != RiskUndoable || !set.MutatesProject || !set.SupportsUndo || !set.RequiresConfirmation || !set.RefreshAfter {
		t.Fatalf("automation.set_points metadata = %+v", set)
	}

	clear, ok := catalog.LookupCommand("automation.clear_region")
	if !ok {
		t.Fatal("automation.clear_region missing")
	}
	if clear.RiskLevel != RiskUndoable || !clear.MutatesProject || !clear.SupportsUndo || !clear.RequiresConfirmation || !clear.RefreshAfter {
		t.Fatalf("automation.clear_region metadata = %+v", clear)
	}

	for alias, canonical := range map[string]string{
		"automation_get_curve":    "automation.get_curve",
		"automation_set_points":   "automation.set_points",
		"automation_clear_region": "automation.clear_region",
	} {
		spec, ok := catalog.LookupTool(alias)
		if !ok {
			t.Fatalf("alias %s missing", alias)
		}
		if spec.CommandName != canonical {
			t.Fatalf("alias %s resolved to command %q, want %q", alias, spec.CommandName, canonical)
		}
	}
}

func TestAutomationSetPointsInputSchemaDeclaresDomainConstraints(t *testing.T) {
	catalog := DefaultCatalog()
	set, ok := catalog.LookupCommand("automation.set_points")
	if !ok {
		t.Fatal("automation.set_points missing")
	}

	props, ok := set.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("automation.set_points input schema properties missing: %+v", set.InputSchema)
	}
	points, ok := props["points"].(map[string]any)
	if !ok {
		t.Fatal("points property missing")
	}
	if points["minItems"] != 1 {
		t.Fatalf("points minItems = %v, want 1", points["minItems"])
	}
	items, ok := points["items"].(map[string]any)
	if !ok {
		t.Fatal("points items schema missing")
	}
	if required, ok := items["required"].([]string); !ok || len(required) != 2 || required[0] != "t" || required[1] != "v" {
		t.Fatalf("point required fields = %#v, want [t v]", items["required"])
	}
	itemProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatal("point properties missing")
	}
	if tSchema, ok := itemProps["t"].(map[string]any); !ok || tSchema["minimum"] != 0 {
		t.Fatalf("point t minimum = %#v, want 0", itemProps["t"])
	}
	cSchema, ok := itemProps["c"].(map[string]any)
	if !ok || cSchema["minimum"] != -1 || cSchema["maximum"] != 1 {
		t.Fatalf("point c range = %#v, want [-1, 1]", itemProps["c"])
	}

	region, ok := props["region"].(map[string]any)
	if !ok {
		t.Fatal("region property missing")
	}
	regionProps, ok := region["properties"].(map[string]any)
	if !ok {
		t.Fatal("region properties missing")
	}
	if startSchema, ok := regionProps["start"].(map[string]any); !ok || startSchema["minimum"] != 0 {
		t.Fatalf("region start minimum = %#v, want 0", regionProps["start"])
	}

	clear, ok := catalog.LookupCommand("automation.clear_region")
	if !ok {
		t.Fatal("automation.clear_region missing")
	}
	if clearProps, ok := clear.InputSchema["properties"].(map[string]any); !ok {
		t.Fatalf("automation.clear_region properties missing: %+v", clear.InputSchema)
	} else if _, hasRegion := clearProps["region"]; !hasRegion {
		t.Fatal("automation.clear_region must declare a region property")
	}
	if required, ok := clear.InputSchema["required"].([]string); !ok || len(required) != 3 {
		t.Fatalf("automation.clear_region required = %#v, want [plugin_id param_id region]", clear.InputSchema["required"])
	}
}
