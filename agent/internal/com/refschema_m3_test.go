package com

import (
	"encoding/hex"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// TestProjectionIDVitRefForm REFSCHEMA-M3：构造器产出 vit://com 形态，
// ParseRef 落 parsed 态且各段承载核对；hash 段=种子前 16 hex；内容身份回归
// （F6 com=内容身份：种子置空 GeneratedAt → 变化不动 ID）；legacy 形态兼容读
// 回归（旧记录 com_<20hex> 经注册表 legacy 条目可解析）。
func TestProjectionIDVitRefForm(t *testing.T) {
	projection := Build(readySourceInput())
	if !strings.HasPrefix(projection.ProjectionID, "vit://com/") {
		t.Fatalf("projection id form = %q", projection.ProjectionID)
	}
	parsed, err := agentprotocol.ParseRef(projection.ProjectionID)
	if err != nil || parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("parse state = %v err=%v", parsed.State, err)
	}
	if parsed.Ref.Kind != "com" || parsed.Ref.ScopeKind != "track" || parsed.Ref.ScopeValue != "track-1" || parsed.Ref.Snapshot != "obs-1" {
		t.Fatalf("segments = %+v", parsed.Ref)
	}
	if parsed.Ref.Window == nil || !parsed.Ref.Window.AllTime {
		t.Fatalf("window = %+v", parsed.Ref.Window)
	}
	sum := projectionSeedHash(projection)
	if want := "sha256:" + hex.EncodeToString(sum[:])[:16]; parsed.Ref.Hash != want {
		t.Fatalf("hash segment = %q want %q", parsed.Ref.Hash, want)
	}
	shifted := readySourceInput()
	shifted.CreatedAt = "2027-01-01T00:00:00Z"
	if regen := Build(shifted); regen.ProjectionID != projection.ProjectionID {
		t.Fatalf("content identity drift on GeneratedAt change: %q vs %q", regen.ProjectionID, projection.ProjectionID)
	}
	legacy, err := agentprotocol.ParseRef("com_" + strings.Repeat("c", 20))
	if err != nil || legacy.State != agentprotocol.RefStateLegacy {
		t.Fatalf("legacy parse state = %v err=%v", legacy.State, err)
	}
	if legacy.Legacy == nil || legacy.Legacy.TargetKind != "com" || legacy.Legacy.Slot != agentprotocol.RefSlotHash {
		t.Fatalf("legacy translation = %+v", legacy.Legacy)
	}
}

// TestChangeDeltaProjectionIDCarriesVitChildIDs REFSCHEMA-M3：change_delta 投
// 影的 BeforeProjectionID/AfterProjectionID 与其自身 ProjectionID 同轮同形态
// （vit://），change 投影自身也落 parsed 态。
func TestChangeDeltaProjectionIDCarriesVitChildIDs(t *testing.T) {
	before := Build(readySourceInput())
	afterInput := readySourceInput()
	afterInput.CreatedAt = "2026-08-04T00:00:01Z"
	after := Build(afterInput)
	change := Build(Input{Mode: ModeChangeDelta, ObservationID: "obs-1", TargetRef: afterInput.TargetRef,
		Change: &ChangeDeltaInput{Before: &before, After: &after}})
	if !strings.HasPrefix(change.EvidenceInputs.BeforeProjectionID, "vit://com/") || !strings.HasPrefix(change.EvidenceInputs.AfterProjectionID, "vit://com/") {
		t.Fatalf("child ids = %q / %q", change.EvidenceInputs.BeforeProjectionID, change.EvidenceInputs.AfterProjectionID)
	}
	parsed, err := agentprotocol.ParseRef(change.ProjectionID)
	if err != nil || parsed.State != agentprotocol.RefStateParsed || parsed.Ref.Kind != "com" {
		t.Fatalf("change id parse = %v err=%v", parsed.State, err)
	}
}
