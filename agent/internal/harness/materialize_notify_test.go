package harness

// materialize_notify_test.go — MAT-B 纯通知锁定（红先行：先于三挂点实现落库）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §3.1（三挂点 if-n-nil 尾挂、nil 时
// 与现状逐字节一致的纯通知承诺）+§5.5（TestNotifierNilIsByteIdentical）。
//
// 两态对拍（MAT-0 零行为锁同款）：off=零接线（现状等价：Notifier nil、shadow 观察者
// 未安装）与 on=全接线（noop 记录 Notifier，三挂点通知真实触发）跑同一输入电池，
// 三挂点路径的全部可观察输出 canonical JSON 逐字节一致。on 态（通知确实发生）仍
// 逐字节一致 ⟹ nil 态（做的严格更少）必然一致——纯通知承诺由此锁定；时间戳字段
// 为唯一归一化项（墙钟差与本卡无关）。非空性守卫：on 态三挂点通知计数必须>0，
// 否则对拍空洞（MAT-0 同款纪律）。

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/materialize"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/shadow"
)

// matBTruncateForDiff 截断超长对拍差分输出（诊断用）。
func matBTruncateForDiff(data []byte) string {
	const limit = 400
	if len(data) <= limit {
		return string(data)
	}
	return string(data[:limit]) + "..."
}

// matBRecorder 是 noop 记录型 Notifier：只计数不做事——它的存在使 on 态走完
// 三挂点的完整通知调用链，同时保证通知本身零副作用。
type matBRecorder struct {
	shadowN    int
	telemetryN int
	renderN    int
}

func (r *matBRecorder) NotifyShadowChange(shadow.ChangeReceipt) { r.shadowN++ }
func (r *matBRecorder) NotifyTelemetry(map[string]any)          { r.telemetryN++ }
func (r *matBRecorder) NotifyRenderJob(jobID, status, filePath string) {
	r.renderN++
}

// 编译期断言：测试 recorder 满足物化层 Notifier 契约（挂点侧持接口，§3.1）。
var _ materialize.Notifier = (*matBRecorder)(nil)

// matBIsVolatileKey 判定墙钟字段：键以 _at / At 结尾（updated_at、reused_at、
// FirstReceivedAt 等）或名为 timestamp/last_timestamp——两态间唯一允许的差。
func matBIsVolatileKey(key string) bool {
	return key == "timestamp" || key == "last_timestamp" ||
		strings.HasSuffix(key, "_at") || strings.HasSuffix(key, "At")
}

func matBStripVolatile(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		for k, item := range value {
			if matBIsVolatileKey(k) {
				continue
			}
			out[k] = matBStripVolatile(item)
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = matBStripVolatile(item)
		}
		return out
	default:
		return v
	}
}

// matBBatteryTmp 是当前电池的临时目录（runMatBBattery 设置）：canonical 输出中
// 替换为占位符——两态各用各的 TempDir 是测试隔离需要，路径差异非通知效应。
var matBBatteryTmp string

// matBNeutralizePaths 把 canonical 字节串里的电池临时目录（含 JSON 转义形式）
// 归一为占位符。
func matBNeutralizePaths(data []byte) []byte {
	if matBBatteryTmp == "" {
		return data
	}
	escaped := strings.ReplaceAll(matBBatteryTmp, `\`, `\\`)
	data = bytes.ReplaceAll(data, []byte(escaped), []byte("<TMP>"))
	return bytes.ReplaceAll(data, []byte(matBBatteryTmp), []byte("<TMP>"))
}

// matBCanonical 做 JSON 往返+墙钟字段剔除后序列化（map 键序由 encoding/json 排序，
// 两态可比）。
func matBCanonical(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var plain any
	if err := json.Unmarshal(data, &plain); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(matBStripVolatile(plain))
	if err != nil {
		t.Fatalf("canonical marshal: %v", err)
	}
	return matBNeutralizePaths(out)
}

func matBReadFileCanonical(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return []byte("<missing:" + path + ">")
	}
	var plain any
	if err := json.Unmarshal(data, &plain); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	out, err := json.Marshal(matBStripVolatile(plain))
	if err != nil {
		t.Fatalf("canonical marshal %s: %v", path, err)
	}
	return matBNeutralizePaths(out)
}

// matBBattery 输入（两态字面同一；文件路径用相对值，避免临时目录差异进内容）。
func matBProjectFull(gainDB float64, revision string) map[string]any {
	return map[string]any{
		"project_uuid":     "matb_nil_identity",
		"project_epoch":    1,
		"project_revision": revision,
		"tracks": []any{map[string]any{
			"track_id":   "T1",
			"track_name": "lead",
			"gain_db":    gainDB,
			"plugins":    []any{},
			"clips":      []any{},
		}},
	}
}

func matBDelta() map[string]any {
	return map[string]any{
		"type":       "delta_update",
		"target_uid": "T1",
		"action":     "property_changed:gain_db",
		"value":      -9.0,
		"seq_id":     1,
	}
}

func matBTelemetryBattery() []map[string]any {
	return []map[string]any{
		{"command": "audio_feature_data_ready", "feature_type": "band_energy_summary",
			"track_id": "T3", "clip_id": "C1", "file_path": "audio.wav", "status": "ready",
			"quality_status": "ready", "source_revision": "sr1", "project_uuid": "matb_nil_identity",
			"bands": map[string]any{"low": map[string]any{"energy_db": -12.0}}},
		{"command": "audio_feature_data_ready", "feature_type": "waveform_envelope",
			"track_id": "T3", "clip_id": "C1", "file_path": "w.wav", "request_id": "wf-1",
			"float_count": 4, "shared_memory": "shm://w", "tile_index": 0},
		{"command": "tile_ready", "feature_type": "spectral_field",
			"track_id": "T3", "clip_id": "C1", "file_path": "s.wav", "request_id": "sp-1",
			"tile_index": 0, "expected_tiles": 1, "float_count": 4, "shared_memory": "shm://s"},
		{"command": "l2_render_probe_ready", "feature_type": "l2_render_probe",
			"request_id": "l2-1", "track_id": "T3", "clip_id": "C1",
			"tap_point": "track_post_fader", "render_revision": "rr1"},
		{"topic": "levels", "meters": []any{1.0, 2.0}},
		{"topic": "render", "subtopic": "render_done", "job_id": "job-matb", "status": "ok", "file_path": "out.wav"},
	}
}

// runMatBBattery 跑一遍完整电池，返回 canonical 可观察面 + 该态的 recorder 计数。
func runMatBBattery(t *testing.T, wired bool) (map[string][]byte, matBRecorder) {
	t.Helper()
	tmp := t.TempDir()
	matBBatteryTmp = tmp
	t.Cleanup(func() { matBBatteryTmp = "" })
	previousRoot, hadRoot := os.LookupEnv("VIT_MIXBOARD_ROOT")
	if err := os.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(tmp, "mixboard")); err != nil {
		t.Fatal(err)
	}
	if hadRoot {
		t.Cleanup(func() { _ = os.Setenv("VIT_MIXBOARD_ROOT", previousRoot) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("VIT_MIXBOARD_ROOT") })
	}

	projectPath := filepath.Join(tmp, "proj", "song.vit")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}

	projectShadow := shadow.New(nil)
	h := NewWithSender(nil, projectShadow, nil)
	recorder := matBRecorder{}
	if wired {
		h.SetMaterializeNotifier(&recorder)
	}
	if _, err := h.ActivateProjectStore(projectPath, "matb_nil_identity"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(projectstore.Deactivate)

	// 挂点 1：shadow ApplyDelta/Initialize 的 ChangeReceipt 路径。
	projectShadow.Initialize(matBProjectFull(-6.0, "r1"))
	projectShadow.ApplyDelta(matBDelta())
	projectShadow.Initialize(matBProjectFull(-12.0, "r2"))

	// 挂点 2：IngestKernelTelemetry 四分支+#9 分支+未知事件。
	for _, event := range matBTelemetryBattery() {
		h.IngestKernelTelemetry(event)
	}

	observables := map[string][]byte{
		"shadow_snapshot": matBCanonical(t, projectShadow.Snapshot()),
		"shadow_summary":  matBCanonical(t, projectShadow.Summary()),
		"shadow_receipt":  matBCanonical(t, projectShadow.LatestChangeReceipt()),
		"shadow_window":   matBCanonical(t, projectShadow.ChangeWindow(2)),
	}

	l3Rows, _, err := projectworkspace.ReadL3Features(projectPath, "matb_nil_identity")
	if err != nil {
		t.Fatalf("ReadL3Features: %v", err)
	}
	observables["l3_rows"] = matBCanonical(t, l3Rows)

	observables["feature_snapshot"] = matBReadFileCanonical(t,
		filepath.Join(tmp, "mixboard_feature_snapshot.json"))

	h.featureMu.Lock()
	observables["waveform_collectors"] = matBCanonical(t, h.waveforms)
	observables["spectral_collectors"] = matBCanonical(t, h.spectrals)
	h.featureMu.Unlock()

	// 挂点 3：ingestKernelRenderTelemetry 唤醒 waiters 前的 render 终态。
	renderResult, err := h.WaitRender(context.Background(), "job-matb")
	if err != nil {
		t.Fatalf("WaitRender: %v", err)
	}
	observables["render_result"] = matBCanonical(t, renderResult)

	return observables, recorder
}

// TestNotifierNilIsByteIdentical：off（现状等价零接线）vs on（noop 记录 Notifier
// 全接线）两态，三挂点全部可观察输出逐字节一致；on 态通知计数>0（非空性）。
func TestNotifierNilIsByteIdentical(t *testing.T) {
	offObs, offRecorder := runMatBBattery(t, false)
	onObs, onRecorder := runMatBBattery(t, true)

	if len(offObs) != len(onObs) {
		t.Fatalf("两态可观察面键数不一致 off=%d on=%d", len(offObs), len(onObs))
	}
	for key, offBytes := range offObs {
		onBytes, ok := onObs[key]
		if !ok {
			t.Fatalf("on 态缺可观察面 %q", key)
		}
		if !bytes.Equal(offBytes, onBytes) {
			t.Fatalf("纯通知承诺被破坏：%s 两态不一致\noff=%s\non=%s",
				key, matBTruncateForDiff(offBytes), matBTruncateForDiff(onBytes))
		}
	}

	// 非空性守卫：on 态三挂点必须真的通知过，否则上面的对拍什么都没证明。
	if onRecorder.shadowN == 0 {
		t.Fatalf("on 态 shadow 挂点零通知（收据≥2 笔预期）")
	}
	if onRecorder.telemetryN < len(matBTelemetryBattery()) {
		t.Fatalf("on 态遥测挂点通知不足：got=%d want≥%d", onRecorder.telemetryN, len(matBTelemetryBattery()))
	}
	if onRecorder.renderN == 0 {
		t.Fatalf("on 态 render 挂点零通知")
	}
	if offRecorder != (matBRecorder{}) {
		t.Fatalf("off 态不应有任何通知（现状等价）：%+v", offRecorder)
	}
}
