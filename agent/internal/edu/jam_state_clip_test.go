package edu

// W02-A1a：Observation 的 clip 绑定披露字段（clip_ids/clip_scope）是 additive
// （omitempty）——旧观察 JSON 的消费者面不变，新字段只在读取面确实组装后出现。

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestObservationClipFieldsAdditiveJSON(t *testing.T) {
	obs := Observation{
		At:        "2026-11-19T12:00:00Z",
		Slots:     map[string][]Note{"melody_imported": {{ID: "n1", Pitch: 60, Start: 0, Length: 1}}},
		ClipIDs:   map[string]string{"melody_imported": "c2"},
		ClipScope: ClipScopeSelected,
	}
	raw, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("marshal observation: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal observation: %v", err)
	}
	if decoded["clip_scope"] != ClipScopeSelected {
		t.Fatalf("clip_scope=%v want %s (raw=%s)", decoded["clip_scope"], ClipScopeSelected, raw)
	}
	ids, ok := decoded["clip_ids"].(map[string]any)
	if !ok || ids["melody_imported"] != "c2" {
		t.Fatalf("clip_ids round trip wrong: %v (raw=%s)", decoded["clip_ids"], raw)
	}
	// 既有字段契约不变。
	if _, ok := decoded["slots"].(map[string]any); !ok {
		t.Fatalf("legacy slots field lost in JSON: %s", raw)
	}
	if decoded["at"] != "2026-11-19T12:00:00Z" {
		t.Fatalf("legacy at field lost in JSON: %s", raw)
	}

	// 未组装 clip 绑定（旧观察形状）→ 新键不出现（omitempty；旧记录反序列化兼容）。
	emptyRaw, err := json.Marshal(Observation{At: "t"})
	if err != nil {
		t.Fatalf("marshal empty observation: %v", err)
	}
	if strings.Contains(string(emptyRaw), "clip_") {
		t.Fatalf("additive clip fields must be omitted when unset: %s", emptyRaw)
	}
	var restored Observation
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("round trip decode: %v", err)
	}
	if restored.ClipScope != ClipScopeSelected || restored.ClipIDs["melody_imported"] != "c2" {
		t.Fatalf("decoded observation lost clip binding: %+v", restored)
	}
}
