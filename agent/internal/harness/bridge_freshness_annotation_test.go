package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// FIX-F5-SNAPSHOT-FRESHNESS：声学桥特征快照双写竞态的红测试。
//
// 两条写路径共用 mixboard_feature_snapshot.json：
//   - 路径 B（mixboard 戳记）：newMixboardFeatureRequestPacket → request_id=mixboard_<UTC 纳秒>
//   - 路径 A（kernel telemetry）：kernelFeatureMaterializerRequestID → request_id=kernel_prepared_<feature>_<clip>
//
// 每次写入都无条件覆盖 latest_request，其余桥行按条件替换。在两次写入之间，
// 快照呈 "latest_request=新铃 / 桥行=旧铃" 的分叉。前台铃机制要求：写入层在
// 发布新 latest_request 的同一笔写入里，为不属于新铃的行自写新鲜度标注
// （stale + superseded_by_request），读侧不做推断。断言对象因此从"不存在分叉"
// 收紧为"不存在未标注的分叉"。

// unannotatedForkRows 返回快照中与 latest_request.request_id 分叉且未携带
// freshness 标注的桥行（key:request_id 形式）。missing 占位行与无 request_id
// 的行不算分叉行。
func unannotatedForkRows(snapshot map[string]any) []string {
	latest, _ := snapshot["latest_request"].(map[string]any)
	latestID := firstString(latest, "request_id")
	if latestID == "" {
		return nil
	}
	var forks []string
	for _, key := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		row, _ := snapshot[key].(map[string]any)
		if len(row) == 0 {
			continue
		}
		rowID := firstString(row, "request_id")
		if rowID == "" || rowID == latestID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(firstString(row, "status")), "missing") {
			continue
		}
		if firstString(row, "freshness") == "" {
			forks = append(forks, key+":"+rowID)
		}
	}
	return forks
}

func f5BandEnergyRow(requestID string) map[string]any {
	return map[string]any{
		"feature_type": "band_energy_summary",
		"status":       "ready",
		"request_id":   requestID,
		"track_id":     "1007",
		"clip_id":      "1011",
		"source":       "kernel_l3_offline_analyzer",
		"bands": map[string]any{
			"low":  map[string]any{"rms_dbfs": -18.2},
			"high": map[string]any{"rms_dbfs": -21.7},
		},
		"analyzed_sample_count":  480000,
		"expected_sample_count":  480000,
		"coverage_ratio":         1.0,
		"source_revision":        "rev-f5-band",
	}
}

func f5SpectralRow(requestID string) map[string]any {
	return map[string]any{
		"feature_type":       "spectral_field",
		"status":             "ready",
		"request_id":         requestID,
		"track_id":           "1007",
		"clip_id":            "1011",
		"source":             "kernel_tile_ready_direct_collector",
		"tile_count_seen":    4,
		"tile_count_expected": 4,
	}
}

func f5MixboardPacket(requestID string) map[string]any {
	return map[string]any{
		"schema_version": "mixboard_feature_request.v1",
		"request_id":     requestID,
		"status":         "requested",
		"project_id":     "current",
		"resolved_target": map[string]any{
			"track_id": "1007",
			"clip_id":  "1011",
		},
		"requested_features": []any{
			map[string]any{"feature_type": "waveform_envelope"},
			map[string]any{"feature_type": "spectral_field"},
			map[string]any{"feature_type": "l3_acoustic_summary"},
		},
	}
}

// TestMixboardSnapshotDualWriteForkRowsCarryFreshnessAnnotation 是确定性红测试：
// 路径 B 先落一笔带 band 行的 mixboard 戳记快照，路径 A 随后发布新铃
// （latest_request 换为 kernel_prepared_*，band 行未被替换）。此后快照上
// band_energy_summary 与 latest_request 分叉，该行必须携带写入层自写的新鲜度
// 标注，否则断言红（未标注分叉）。
func TestMixboardSnapshotDualWriteForkRowsCarryFreshnessAnnotation(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}

	mixboardRequestID := "mixboard_20260921T120000.000000001"
	writeMixboardReadyL3SummarySnapshot(cmd, f5MixboardPacket(mixboardRequestID), f5BandEnergyRow(mixboardRequestID))

	// 路径 A：kernel telemetry 到达（spectral），latest_request 无条件换成
	// kernel_prepared 铃；band 行没有 kernel 侧替换路径。
	kernelEvent := map[string]any{
		"track_id":     "1007",
		"clip_id":      "1011",
		"feature_type": "spectral_field",
	}
	kernelPacket := kernelFeatureMaterializerPacket(kernelEvent, kernelFeatureMaterializerTarget(kernelEvent, "1007", "1011"))
	kernelRequestID := firstString(kernelPacket, "request_id")
	if !strings.HasPrefix(kernelRequestID, "kernel_prepared_") {
		t.Fatalf("kernel packet should carry kernel_prepared request id, got %q", kernelRequestID)
	}
	writeMixboardReadySpectralSnapshot(cmd, kernelPacket, f5SpectralRow(kernelRequestID))

	snapshot := readMixboardFeatureSnapshotFile(snapshotPath)
	if len(snapshot) == 0 {
		t.Fatal("snapshot missing after dual writes")
	}
	latest, _ := snapshot["latest_request"].(map[string]any)
	if firstString(latest, "request_id") != kernelRequestID {
		t.Fatalf("latest_request should be the kernel_prepared bell, got %q", firstString(latest, "request_id"))
	}
	if forks := unannotatedForkRows(snapshot); len(forks) > 0 {
		band, _ := snapshot["band_energy_summary"].(map[string]any)
		raw, _ := json.Marshal(band)
		t.Fatalf("unannotated fork rows after dual writes: %v\nlatest_request=%s\nband_energy_summary=%s",
			forks, kernelRequestID, string(raw))
	}
	// 分叉行必须保留其原始 request_id（旧行归属不被洗写），并以
	// superseded_by_request 指向当前铃。
	band, _ := snapshot["band_energy_summary"].(map[string]any)
	if firstString(band, "request_id") == kernelRequestID {
		t.Fatalf("fork row request_id should keep its origin bell (got rewashed to %q)", kernelRequestID)
	}
	if firstString(band, "superseded_by_request") != kernelRequestID {
		t.Fatalf("fork row should disclose superseded_by_request=%s, got %q", kernelRequestID, firstString(band, "superseded_by_request"))
	}
}

// TestMixboardSnapshotConcurrentDualWriteNoUnannotatedFork 模拟烟测 ⑤ 断言在
// 分叉窗内读快照：两条写路径并发交替发布，轮询读者持续检查"不存在未标注的
// 分叉"。修前轮询必然捕获无标注分叉窗；修后每笔写入都随铃标注，全程绿。
func TestMixboardSnapshotConcurrentDualWriteNoUnannotatedFork(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	cmd := map[string]any{"feature_snapshot_path": snapshotPath}

	const rounds = 40
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			requestID := fmt.Sprintf("mixboard_20260921T120000.%09d", 1000+i)
			writeMixboardReadyL3SummarySnapshot(cmd, f5MixboardPacket(requestID), f5BandEnergyRow(requestID))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			kernelEvent := map[string]any{
				"track_id":     "1007",
				"clip_id":      "1011",
				"feature_type": "spectral_field",
			}
			packet := kernelFeatureMaterializerPacket(kernelEvent, kernelFeatureMaterializerTarget(kernelEvent, "1007", "1011"))
			writeMixboardReadySpectralSnapshot(cmd, packet, f5SpectralRow(firstString(packet, "request_id")))
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	var lastForks []string
	var reads int
	for {
		if wgDone(&wg) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("dual writers did not finish in time")
		}
		if snapshot := readMixboardFeatureSnapshotFile(snapshotPath); len(snapshot) > 0 {
			reads++
			if forks := unannotatedForkRows(snapshot); len(forks) > 0 {
				lastForks = forks
				// 让写者继续跑完再报错，证据更完整。
				wg.Wait()
				t.Fatalf("unannotated fork observed mid-race (read #%d): %v", reads, lastForks)
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	wg.Wait()
	if reads == 0 {
		t.Fatal("polling reader never observed a snapshot")
	}
}

func wgDone(wg *sync.WaitGroup) bool {
	state := make(chan struct{})
	go func() { wg.Wait(); close(state) }()
	select {
	case <-state:
		return true
	default:
		return false
	}
}

// TestWriteMixboardFeatureSnapshotFileAtomicReplace 锁定原子发布契约：
// 落盘必须经临时文件 + rename 整体替换——任何时刻磁盘上要么是完整旧快照、
// 要么是完整新快照，且不残留 .tmp 工件。
func TestWriteMixboardFeatureSnapshotFileAtomicReplace(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")

	for i := 0; i < 25; i++ {
		requestID := fmt.Sprintf("mixboard_20260921T130000.%09d", 2000+i)
		packet := f5MixboardPacket(requestID)
		snapshot := map[string]any{
			"schema_version": "mixboard_feature_snapshot.v1",
			"updated_at":     time.Now().UTC().Format(time.RFC3339Nano),
			"latest_request": packet,
			"band_energy_summary": f5BandEnergyRow(requestID),
		}
		writeMixboardFeatureSnapshotFile(snapshotPath, snapshot, packet)
		if _, err := os.Stat(snapshotPath); err != nil {
			t.Fatalf("snapshot missing after write #%d: %v", i, err)
		}
		if entries, err := os.ReadDir(root); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".tmp") {
					t.Fatalf("temp artifact %s leaked after write #%d", entry.Name(), i)
				}
			}
		}
		var onDisk map[string]any
		data, err := os.ReadFile(snapshotPath)
		if err != nil {
			t.Fatalf("read snapshot #%d: %v", i, err)
		}
		if err := json.Unmarshal(data, &onDisk); err != nil {
			t.Fatalf("snapshot on disk torn after write #%d: %v", i, err)
		}
		if firstString(onDisk, "schema_version") != "mixboard_feature_snapshot.v1" {
			t.Fatalf("snapshot on disk incomplete after write #%d", i)
		}
	}
}
