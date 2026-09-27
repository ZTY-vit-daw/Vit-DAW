package materialize

// label_parity_test.go — MAT-D3 红先行：DOMAdapter 合成包 TargetRef 的 label
// 奇偶性（G2-D 闭环证据）。
//
// MAT-D2 真栈取证（coord/runs/MAT-D2-1/forensics/）：S2 分歧行的唯一非 volatile
// 差异键是 target_ref.label——真实观察包带 label（harness.requestMixObservation
// 从轨行解析 track_name 填 target.Label，harness.go resolveMixObservation
// TargetContext→visibleTrackName 同源链），适配器合成包不带→内容身份 hash 恒
// 不等。本文件用取证的最小复现形态钉死该奇偶性：物化侧从 deps.ProjectState
// 同源取轨名填 Label；轨名缺失=NE（不猜名字）。

import (
	"testing"

	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/mixboard"
)

// matD3Fixture 是最小复现形态：单轨工程快照（轨名 lead vox）+ 单行 waveform
// 特征（MAT-D2 取证 label_diff_minimal_repro 同构，去 TimeRuler/MixPackage——
// 取证已证明其在场未被消费时不影响 hash）。
func matD3Fixture() DepInputs {
	waveformRow := map[string]any{
		"status": "ready", "track_id": "T3", "source_revision": "sr1",
		"clip_revision": "cr1", "rms_dbfs": -20.5, "peak_dbfs": -3.2,
		"headroom_db": 3.2, "duration_seconds": 12.0, "sample_rate": 48000.0,
		"channel_count": 2, "analyzed_sample_count": 576000,
		"window_ms": 200.0, "hop_ms": 100.0,
		"time_segments": []any{map[string]any{
			"start_seconds": 0.0, "end_seconds": 12.0,
			"rms_dbfs": -20.5, "peak_dbfs": -3.2,
		}},
	}
	return DepInputs{
		ProjectState: map[string]any{
			"project_revision": "r1",
			"tracks": []any{map[string]any{
				"track_id": "T3", "track_name": "lead vox", "gain_db": -6.0,
			}},
		},
		FeatureSnapshot: map[string]any{
			"schema_version":           "mixboard_feature_snapshot.v1",
			"track_waveform_envelopes": []any{waveformRow},
		},
		ProjectRevision: "r1",
	}
}

// matD3ObservationRow 构造观察侧基准行：真实观察包形态（TargetRef 带轨名
// label——真栈 harness 解析链的输出形态）。label 空串=无 label 形态（NE 对照）。
func matD3ObservationRow(label string) Row {
	obs := mixboard.ObservationPacket{
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3", Label: label},
		GlobalSummary:  map[string]any{"feature_snapshot": matD3Fixture().FeatureSnapshot},
		ProjectPackage: map[string]any{"project_revision": "r1"},
	}
	projection := dom.Build(mixboard.DOMInputFromObservation(obs, mixboard.Request{}))
	row, ok := DOMRowFromProjection("T3", projection)
	if !ok {
		panic("matD3 观察侧未产行（dom.Build 零值）")
	}
	return row
}

// TestDOMAdapterLabelParityWithObservation：物化行与带 label 的真实观察包
// 行化后 hash 相等——修前红（合成包缺 Label→hash 恒不等，MAT-D2 取证形态）。
func TestDOMAdapterLabelParityWithObservation(t *testing.T) {
	rows, err := DOMAdapter().Build(matD3Fixture())
	if err != nil {
		t.Fatalf("DOMAdapter.Build: %v", err)
	}
	var materialized *Row
	for i := range rows {
		if rows[i].Ref.Kind == "dom" && rows[i].Ref.ScopeValue == "T3" {
			materialized = &rows[i]
			break
		}
	}
	if materialized == nil {
		t.Fatalf("DOMAdapter 未产出行 dom/T3（rows=%v）", rows)
	}
	observed := matD3ObservationRow("lead vox")
	if materialized.Ref.Hash != observed.Ref.Hash {
		t.Fatalf("label 奇偶性缺口：物化行 hash=%s != 观察行 hash=%s（观察侧 label=lead vox）\n物化行=%+v\n观察行=%+v",
			materialized.Ref.Hash, observed.Ref.Hash, *materialized, observed)
	}
}

// TestDOMAdapterLabelNEWhenTrackNameMissing：ProjectState 无该轨名时 NE 语义
// ——Label 留空（不猜名字），与无 label 观察行 hash 相等；也不得把其他轨的
// 名字误填进来。
func TestDOMAdapterLabelNEWhenTrackNameMissing(t *testing.T) {
	deps := matD3Fixture()
	deps.ProjectState = map[string]any{
		"project_revision": "r1",
		"tracks": []any{
			map[string]any{"track_id": "T7", "track_name": "drums"},
			map[string]any{"track_id": "T3"}, // 目标轨行在场但无名字
		},
	}
	rows, err := DOMAdapter().Build(deps)
	if err != nil {
		t.Fatalf("DOMAdapter.Build: %v", err)
	}
	var materialized *Row
	for i := range rows {
		if rows[i].Ref.Kind == "dom" && rows[i].Ref.ScopeValue == "T3" {
			materialized = &rows[i]
			break
		}
	}
	if materialized == nil {
		t.Fatalf("DOMAdapter 未产出行 dom/T3（rows=%v）", rows)
	}
	observed := matD3ObservationRow("")
	if materialized.Ref.Hash != observed.Ref.Hash {
		t.Fatalf("NE 语义破坏：轨名缺失时物化行 hash=%s != 无 label 观察行 hash=%s（物化行=%+v）",
			materialized.Ref.Hash, observed.Ref.Hash, *materialized)
	}
}
