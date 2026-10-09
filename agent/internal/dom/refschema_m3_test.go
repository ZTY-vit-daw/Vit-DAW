package dom

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// TestProjectionIDVitRefForm REFSCHEMA-M3：构造器产出 vit://dom 形态，
// ParseRef 落 parsed 态且 kind/scope/snapshot/window/hash 各段按 QUERY_ENGINE
// §3.3 承载核对；hash 段=stableProjectionID 种子前 16 hex（F3）；内容身份
// 回归（GeneratedAt 变化不动 ID，F6 dom=内容身份）；legacy 形态兼容读回归
// （旧记录 dom_<20hex> 经注册表 legacy 条目可解析，REF_SCHEMA_V1 §9 legacy 态）。
func TestProjectionIDVitRefForm(t *testing.T) {
	projection := Build(domTestInput())
	if !strings.HasPrefix(projection.ProjectionID, "vit://dom/") {
		t.Fatalf("projection id form = %q", projection.ProjectionID)
	}
	parsed, err := agentprotocol.ParseRef(projection.ProjectionID)
	if err != nil || parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("parse state = %v err=%v", parsed.State, err)
	}
	if parsed.Ref.Kind != "dom" || parsed.Ref.ScopeKind != "track" || parsed.Ref.ScopeValue != "track-1" || parsed.Ref.Snapshot != "obs-1" {
		t.Fatalf("segments = %+v", parsed.Ref)
	}
	if parsed.Ref.Window == nil || !parsed.Ref.Window.AllTime {
		t.Fatalf("window = %+v", parsed.Ref.Window)
	}
	sum := projectionSeedHash(projection)
	if want := "sha256:" + hex.EncodeToString(sum[:])[:16]; parsed.Ref.Hash != want {
		t.Fatalf("hash segment = %q want %q", parsed.Ref.Hash, want)
	}
	shifted := domTestInput()
	shifted.CreatedAt = "2027-01-01T00:00:00Z"
	if regen := Build(shifted); regen.ProjectionID != projection.ProjectionID {
		t.Fatalf("content identity drift on GeneratedAt change: %q vs %q", regen.ProjectionID, projection.ProjectionID)
	}
	legacy, err := agentprotocol.ParseRef("dom_" + strings.Repeat("a", 20))
	if err != nil || legacy.State != agentprotocol.RefStateLegacy {
		t.Fatalf("legacy parse state = %v err=%v", legacy.State, err)
	}
	if legacy.Legacy == nil || legacy.Legacy.TargetKind != "dom" || legacy.Legacy.Slot != agentprotocol.RefSlotHash {
		t.Fatalf("legacy translation = %+v", legacy.Legacy)
	}
}

// TestProjectionIDScopeEscaping REFSCHEMA-M3：TargetRef 身份含保留字符时经
// canonical 转义入 ref，ParseRef 还原（INV2 canonical 闭环）。
func TestProjectionIDScopeEscaping(t *testing.T) {
	input := domTestInput()
	input.TargetRef = map[string]any{"kind": "track", "id": "track/1#x"}
	projection := Build(input)
	parsed, err := agentprotocol.ParseRef(projection.ProjectionID)
	if err != nil || parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("parse state = %v err=%v", parsed.State, err)
	}
	if parsed.Ref.ScopeValue != "track/1#x" {
		t.Fatalf("scope value roundtrip = %q", parsed.Ref.ScopeValue)
	}
}

// TestLegacyProjectionRecordRoundTrip REFSCHEMA-M3 legacy 兼容读回归：旧持久
// 化记录（legacy projection_id）反序列化往返——Projection 结构零改动下旧记
// 录可加载，ID 字段原样保留且走 legacy 三态解析（四包 types.go 均未动，JSON
// 兼容性同构）。
func TestLegacyProjectionRecordRoundTrip(t *testing.T) {
	legacyID := "dom_" + strings.Repeat("a", 20)
	raw := `{"schema_version":"` + SchemaVersion + `","dom_version":"v0","projection_id":"` + legacyID + `","mode":"source_only","status":"partial"}`
	var loaded Projection
	if err := json.Unmarshal([]byte(raw), &loaded); err != nil {
		t.Fatalf("legacy record load: %v", err)
	}
	if loaded.ProjectionID != legacyID {
		t.Fatalf("legacy projection_id mutated: %q", loaded.ProjectionID)
	}
	parsed, err := agentprotocol.ParseRef(loaded.ProjectionID)
	if err != nil || parsed.State != agentprotocol.RefStateLegacy {
		t.Fatalf("legacy record id parse = %v err=%v", parsed.State, err)
	}
}
