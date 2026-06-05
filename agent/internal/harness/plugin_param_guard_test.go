package harness

import (
	"context"
	"testing"
)

func TestPluginSnapshotCacheUpdate(t *testing.T) {
	c := NewPluginSnapshotCache()

	reply := map[string]any{
		"track_id":  "track_1",
		"plugin_id": "plugin_1",
		"parameters": []map[string]any{
			{"id": "param_1", "name": "Gain", "value": 0.5},
			{"id": "param_2", "name": "Frequency", "value": 1000.0},
			{"id": "param_3", "name": "Q", "value": 1.5},
		},
	}

	if !c.Update(reply) {
		t.Fatal("Update should return true for valid reply")
	}

	if !c.HasParamID("track_1", "plugin_1", "param_1") {
		t.Error("HasParamID should find param_1 after update")
	}
	if !c.HasParamID("track_1", "plugin_1", "param_2") {
		t.Error("HasParamID should find param_2 after update")
	}
	if c.HasParamID("track_1", "plugin_1", "param_99") {
		t.Error("HasParamID should not find non-existent param_99")
	}
	if c.HasParamID("track_2", "plugin_1", "param_1") {
		t.Error("HasParamID should not find param for different track")
	}
}

func TestPluginSnapshotCacheUpdateMissingFields(t *testing.T) {
	c := NewPluginSnapshotCache()

	if c.Update(map[string]any{}) {
		t.Error("Update should return false for empty reply")
	}
	if c.Update(map[string]any{"track_id": "t1"}) {
		t.Error("Update should return false when missing plugin_id")
	}
	if c.Update(map[string]any{"track_id": "t1", "plugin_id": "p1"}) {
		t.Error("Update should return false when no parameters")
	}
	if c.GetSnapshot("t1", "p1") != nil {
		t.Error("GetSnapshot should return nil for non-existent entry")
	}
}

func TestPluginSnapshotCacheStaleParams(t *testing.T) {
	c := NewPluginSnapshotCache()

	reply := map[string]any{
		"track_id":                "t1",
		"plugin_id":               "p1",
		"profile_stale_param_ids": []any{"param_2", "param_3"},
		"parameters": []map[string]any{
			{"id": "param_1"},
			{"id": "param_2"},
			{"id": "param_3"},
			{"id": "param_4"},
		},
	}

	if !c.Update(reply) {
		t.Fatal("Update should succeed")
	}

	if c.IsStaleParam("t1", "p1", "param_1") {
		t.Error("param_1 should not be stale")
	}
	if !c.IsStaleParam("t1", "p1", "param_2") {
		t.Error("param_2 should be stale")
	}
	if !c.IsStaleParam("t1", "p1", "param_3") {
		t.Error("param_3 should be stale")
	}
	if c.IsStaleParam("t1", "p1", "param_99") {
		t.Error("non-existent param should not be stale")
	}
}

func TestValidateSetPluginParamNoCache(t *testing.T) {
	h := New(nil, nil, nil)

	err := h.validateSetPluginParam(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"param_id":  "param_1",
		"value":     0.5,
	})
	if err == nil {
		t.Error("Should reject set when no snapshot cached")
	}
	if err != nil && !containsText(err.Error(), "have not been read") {
		t.Errorf("Error should mention parameters not read, got: %v", err)
	}
}

func TestValidateSetPluginParamValidParam(t *testing.T) {
	h := New(nil, nil, nil)

	h.snapshotCache.Update(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"parameters": []map[string]any{
			{"id": "param_1"},
			{"id": "param_2"},
		},
	})

	err := h.validateSetPluginParam(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"param_id":  "param_1",
		"value":     0.5,
	})
	if err != nil {
		t.Errorf("Valid param should pass, got: %v", err)
	}
}

func TestValidateSetPluginParamInvalidParamID(t *testing.T) {
	h := New(nil, nil, nil)

	h.snapshotCache.Update(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"parameters": []map[string]any{
			{"id": "param_1"},
			{"id": "param_2"},
		},
	})

	err := h.validateSetPluginParam(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"param_id":  "param_99",
		"value":     0.5,
	})
	if err == nil {
		t.Error("Should reject invalid param_id")
	}
	if err != nil && !containsText(err.Error(), "not in the current plugin parameter set") {
		t.Errorf("Error should mention invalid param, got: %v", err)
	}
}

func TestValidateSetPluginParamStaleParam(t *testing.T) {
	h := New(nil, nil, nil)

	h.snapshotCache.Update(map[string]any{
		"track_id":                "t1",
		"plugin_id":               "p1",
		"profile_stale_param_ids": []any{"param_2"},
		"parameters": []map[string]any{
			{"id": "param_1"},
			{"id": "param_2"},
		},
	})

	err := h.validateSetPluginParam(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"param_id":  "param_2",
		"value":     0.5,
	})
	if err == nil {
		t.Error("Should reject stale param")
	}
	if err != nil && !containsText(err.Error(), "marked as stale") {
		t.Errorf("Error should mention stale, got: %v", err)
	}
}

func TestValidateSetPluginParamMissingFields(t *testing.T) {
	h := New(nil, nil, nil)

	err := h.validateSetPluginParam(map[string]any{})
	if err != nil {
		// Missing track/plugin should pass validation (can"t validate if we don"t know the target)
	}

	// Add cache first
	h.snapshotCache.Update(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"parameters": []map[string]any{
			{"id": "param_1"},
		},
	})

	err = h.validateSetPluginParam(map[string]any{
		"track_id":  "t1",
		"plugin_id": "p1",
		"value":     0.5,
	})
	if err == nil {
		t.Error("Should reject empty param_id when cache exists")
	}
}

func TestInvokeGetParametersCachesSnapshotForSetParam(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{
		{
			"status":    "ok",
			"track_id":  "t1",
			"plugin_id": "p1",
			"parameters": []any{
				map[string]any{"id": "param_1"},
				map[string]any{"id": "param_2"},
			},
		},
		{
			"status":    "ok",
			"plugin_id": "p1",
			"param_id":  "param_1",
			"new_value": 0.5,
		},
	}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	if _, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.get_parameters",
		Args: map[string]any{
			"track_id":  "t1",
			"plugin_id": "p1",
		},
	}); err != nil {
		t.Fatalf("get parameters invoke: %v", err)
	}

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.set_parameter",
		Args: map[string]any{
			"track_id":  "t1",
			"plugin_id": "p1",
			"param_id":  "param_1",
			"value":     0.5,
		},
	})
	if err != nil {
		t.Fatalf("set parameter invoke: %v; resp=%+v", err, resp)
	}
	if len(kernel.commands) < 2 || kernel.commands[1]["cmd"] != "set_plugin_param" {
		t.Fatalf("kernel commands = %+v", kernel.commands)
	}
}

func TestInvokeSetParamRejectsBeforeKernelWithoutSnapshot(t *testing.T) {
	kernel := &fakeKernelClient{}
	h := New(nil, nil, nil)
	h.kernel = kernel

	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.set_parameter",
		Args: map[string]any{
			"track_id":  "t1",
			"plugin_id": "p1",
			"param_id":  "param_1",
			"value":     0.5,
		},
	})
	if err == nil {
		t.Fatalf("expected set parameter to be rejected; resp=%+v", resp)
	}
	if !containsText(err.Error(), "have not been read") {
		t.Fatalf("error = %v", err)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("set_plugin_param should not reach kernel, commands=%+v", kernel.commands)
	}
}

func TestFormatParamIDList(t *testing.T) {
	if s := formatParamIDList(nil, 10); s != "(none)" {
		t.Errorf("Expected (none), got %q", s)
	}
	if s := formatParamIDList([]string{}, 10); s != "(none)" {
		t.Errorf("Expected (none), got %q", s)
	}
	if s := formatParamIDList([]string{"a", "b", "c"}, 10); s != "a, b, c" {
		t.Errorf("Expected 'a, b, c', got %q", s)
	}
	if s := formatParamIDList([]string{"a", "b", "c", "d"}, 2); s != "a, b ... and 2 more" {
		t.Errorf("Expected truncated, got %q", s)
	}
}

func containsText(s, substr string) bool {
	return len(s) > 0 && substr != "" && containsTextAnyFold(s, substr)
}
