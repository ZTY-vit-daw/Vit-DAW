package audioclosure

// observation_key_test.go — MAT-A（MATERIALIZATION_V1_DESIGN §6.3，承接 G1
// ruling #5）ObservationKey 扩段的旧记录往返测试（红先行）。
//
// 兼容纪律（AGENTS §11）：新字段 omitempty，旧记录反序列化缺省空串=未物化；
// 扩段不得改变旧形 key 的 JSON 形状与 ObservationFingerprint 输出。
// 下方金标值由扩段前代码实测采集（2026-09-27，port/mat-a@84ad82ba，
// types.go 未改动时的 ObservationFingerprint/json.Marshal 输出）。

import (
	"encoding/json"
	"testing"
)

const legacyObservationKeyJSON = `{"project_uuid":"proj-uuid-1","project_revision":"42","scope":{"kind":"track","id":"T3","label":"Lead Vox"},"target_ref":"vit://dom/track:T3/t=all@obs1#sha256:0123456789abcdef","view_ids":["v1","v2"],"observation_mode":"duty","tap":"pre","time_window":"t=all"}`

const legacyObservationKeyFingerprint = "audio_observation:b119df864429e71bdd16819817a4c2a1"

func TestObservationKeyRefSegmentsLegacyRoundTrip(t *testing.T) {
	var key ObservationKey
	if err := json.Unmarshal([]byte(legacyObservationKeyJSON), &key); err != nil {
		t.Fatalf("旧记录反序列化: %v", err)
	}
	if key.RefKind != "" || key.RefHash != "" {
		t.Fatalf("旧记录缺省语义：扩段字段应为空串（=未物化），got ref_kind=%q ref_hash=%q", key.RefKind, key.RefHash)
	}

	remarshaled, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("序列化: %v", err)
	}
	if string(remarshaled) != legacyObservationKeyJSON {
		t.Fatalf("omitempty 违约：空扩段字段泄漏进旧形状 JSON\n got: %s\nwant: %s", remarshaled, legacyObservationKeyJSON)
	}

	fp, err := ObservationFingerprint(key)
	if err != nil {
		t.Fatalf("ObservationFingerprint: %v", err)
	}
	if fp != legacyObservationKeyFingerprint {
		t.Fatalf("金标指纹漂移：扩段改变了旧形 key 的观察身份\ngot:  %s\nwant: %s", fp, legacyObservationKeyFingerprint)
	}

	// 扩段后往返：物化坐标可持久化、可恢复（RefKind 对齐 refschema 注册
	// kind，RefHash=L0 hash 段同源内容身份）。
	key.RefKind = "dom"
	key.RefHash = "sha256:0123456789abcdef"
	raw, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("序列化扩段 key: %v", err)
	}
	var back ObservationKey
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("反序列化扩段 key: %v", err)
	}
	if back.RefKind != "dom" || back.RefHash != "sha256:0123456789abcdef" {
		t.Fatalf("扩段字段往返失真：ref_kind=%q ref_hash=%q", back.RefKind, back.RefHash)
	}

	// 可分辨性：物化坐标参与观察身份——观察记录与物化行可互查（§6.3 目的）。
	fpMaterialized, err := ObservationFingerprint(back)
	if err != nil {
		t.Fatalf("ObservationFingerprint(扩段): %v", err)
	}
	if fpMaterialized == legacyObservationKeyFingerprint {
		t.Fatalf("携带物化坐标的 key 与旧形 key 指纹相同——互查可分辨性丧失")
	}
}
