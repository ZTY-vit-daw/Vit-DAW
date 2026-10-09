package rlm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// vitRefTestInput 构造 ready 态参考电平输入（同现有 strict metric 测试口径）。
func vitRefTestInput(generatedAt string) Input {
	return Input{
		GeneratedAt: generatedAt,
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_c", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_a", "active_rms_dbfs": -24.0, "rms_dbfs": -35.0},
					{"track_id": "track_b", "active_rms_dbfs": -20.0, "rms_dbfs": -30.0},
					{"track_id": "track_c", "active_rms_dbfs": -20.0, "rms_dbfs": -30.0},
				},
			},
		},
	}
}

// TestProjectionIDVitRefForm REFSCHEMA-M3：构造器产出 vit://rlm 形态，
// ParseRef 落 parsed 态且各段承载核对（scope=project:current 单例投影；
// snapshot=GeneratedAt 实例代，冒号经 canonical 转义往返还原）；hash 段=种子
// 前 16 hex；实例身份回归（F6 rlm=实例身份：GeneratedAt 参与 → 变化即 ID
// 变）；legacy 形态兼容读回归（旧记录 rlm_<16hex> 经注册表 legacy 条目可解
// 析）。
func TestProjectionIDVitRefForm(t *testing.T) {
	projection := Build(vitRefTestInput("2026-07-10T00:00:00Z"))
	if !strings.HasPrefix(projection.ProjectionID, "vit://rlm/project:") {
		t.Fatalf("projection id form = %q", projection.ProjectionID)
	}
	parsed, err := agentprotocol.ParseRef(projection.ProjectionID)
	if err != nil || parsed.State != agentprotocol.RefStateParsed {
		t.Fatalf("parse state = %v err=%v", parsed.State, err)
	}
	if parsed.Ref.Kind != "rlm" || parsed.Ref.ScopeKind != "project" || parsed.Ref.ScopeValue != "current" || parsed.Ref.Snapshot != "2026-07-10T00:00:00Z" {
		t.Fatalf("segments = %+v", parsed.Ref)
	}
	if parsed.Ref.Window == nil || !parsed.Ref.Window.AllTime {
		t.Fatalf("window = %+v", parsed.Ref.Window)
	}
	seed := strings.Join([]string{
		projection.SchemaVersion,
		projection.SelectedMetric,
		projection.Mode,
		fmt.Sprintf("%d", projection.Summary.TrackCount),
		fmt.Sprintf("%d", projection.Summary.ActionCount),
		strings.Join(projection.EvidenceRefs, "|"),
		projection.GeneratedAt,
	}, "\x00")
	sum := sha256.Sum256([]byte(seed))
	if want := "sha256:" + hex.EncodeToString(sum[:])[:16]; parsed.Ref.Hash != want {
		t.Fatalf("hash segment = %q want %q", parsed.Ref.Hash, want)
	}
	if regen := Build(vitRefTestInput("2026-07-11T00:00:00Z")); regen.ProjectionID == projection.ProjectionID {
		t.Fatalf("instance identity lost: GeneratedAt change must move the rlm id")
	}
	legacy, err := agentprotocol.ParseRef("rlm_" + strings.Repeat("d", 16))
	if err != nil || legacy.State != agentprotocol.RefStateLegacy {
		t.Fatalf("legacy parse state = %v err=%v", legacy.State, err)
	}
	if legacy.Legacy == nil || legacy.Legacy.TargetKind != "rlm" || legacy.Legacy.Slot != agentprotocol.RefSlotHash {
		t.Fatalf("legacy translation = %+v", legacy.Legacy)
	}
}
