package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/mixboard"
)

type audioAsset struct {
	TrackID string
	ClipID  string
	Name    string
	Path    string
}

type wavInfo struct {
	SampleRate int
	Channels   int
	Frames     int
	Samples    []float64
}

func main() {
	repoRoot := flag.String("repo-root", defaultRepoRoot(), "Vit_DAW repository root")
	outRoot := flag.String("out-root", "", "output root for lab artifacts")
	sessionID := flag.String("session-id", "mixlab_full_project", "mixboard session id")
	segmentSeconds := flag.Float64("segment-seconds", 2.0, "time segment size for waveform rows")
	flag.Parse()

	root := filepath.Clean(*repoRoot)
	outputRoot := strings.TrimSpace(*outRoot)
	if outputRoot == "" {
		outputRoot = filepath.Join(root, "VitApp", "Workspace", "Artifacts", "mixlab", time.Now().Format("20060102_150405"))
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		fatalf("create output root: %v", err)
	}

	assets := discoverAssets(root)
	if len(assets) == 0 {
		fatalf("no lab assets found under %s", root)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	projectTracks := make([]any, 0, len(assets))
	waveformRows := make([]map[string]any, 0, len(assets))
	for i, asset := range assets {
		projectTracks = append(projectTracks, map[string]any{
			"track_id":         asset.TrackID,
			"track_name":       asset.Name,
			"user_track_index": i + 1,
			"track_type":       "audio",
			"is_audio_track":   true,
			"clips": []any{
				map[string]any{
					"clip_id":          asset.ClipID,
					"name":             asset.Name,
					"file_path":        asset.Path,
					"start_seconds":    0,
					"length_seconds":   assetDuration(asset.Path),
					"duration_seconds": assetDuration(asset.Path),
				},
			},
		})
		row, err := waveformRowForAsset(asset, *segmentSeconds, now)
		if err != nil {
			row = blockedWaveformRow(asset, err.Error(), now)
		}
		waveformRows = append(waveformRows, row)
	}

	snapshotPath := filepath.Join(outputRoot, "mixlab_feature_snapshot.json")
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"updated_at":     now,
		"latest_request": map[string]any{
			"schema_version": "mixboard_feature_request.v1",
			"request_id":     "mixlab_offline_" + time.Now().UTC().Format("20060102T150405"),
			"status":         "ready",
			"scope":          "full_project",
			"reason":         "offline_mixlab_fixture",
			"created_at":     now,
		},
		"waveform_envelope":        map[string]any{"status": "missing", "reason": "full_project_lab_uses_track_waveform_envelopes"},
		"track_waveform_envelopes": waveformRows,
		"spectrogram_tiles":        map[string]any{"status": "missing"},
		"band_energy_summary":      map[string]any{"status": "missing"},
		"stereo_relation_summary":  map[string]any{"status": "missing"},
	}
	writeJSON(snapshotPath, snapshot)

	storeRoot := filepath.Join(outputRoot, "mixboard")
	store := mixboard.NewStore(storeRoot)
	result, err := store.RequestObservation(mixboard.Request{
		MixSessionID: *sessionID,
		Round:        1,
		GoalText:     "offline full-project acoustic lab",
		TargetRef: mixboard.TargetRef{
			Kind:       "project",
			ID:         "current",
			Label:      "MixLab offline project",
			Source:     "mixlab",
			Confidence: "high",
		},
		MixObjects: []mixboard.MixObject{{
			Mode:   "scope",
			Kind:   "project",
			ID:     "current",
			Label:  "MixLab offline project",
			Source: "mixlab",
		}},
		ListenScope: mixboard.ListenScope{
			Time:   mixboard.ListenTimeScope{Mode: "full_song", Source: "mixlab"},
			Source: mixboard.ListenSourceScope{Mode: "full_project"},
		},
		ProjectState: map[string]any{
			"duration_seconds": projectDuration(projectTracks),
			"tracks":           projectTracks,
		},
		Args: map[string]any{
			"scope":                 "full_project",
			"segment_seconds":       *segmentSeconds,
			"feature_snapshot_path": snapshotPath,
		},
	})
	if err != nil {
		fatalf("request observation: %v", err)
	}

	summary := buildSummary(result, snapshotPath, outputRoot)
	summaryPath := filepath.Join(outputRoot, "summary.json")
	writeJSON(summaryPath, summary)
	fmt.Printf("ok: mix acoustic lab generated\n")
	fmt.Printf("output_root=%s\n", outputRoot)
	fmt.Printf("snapshot_path=%s\n", snapshotPath)
	fmt.Printf("observation_path=%s\n", result.ObservationPath)
	fmt.Printf("summary_path=%s\n", summaryPath)
	printSummary(summary)
}

func discoverAssets(root string) []audioAsset {
	candidates := []struct {
		id   string
		name string
		path string
	}{
		{"lab_100hz", "100 Hz Test Tone", filepath.Join(root, "test_100hz_10s.wav")},
		{"lab_peak", "Peak Risk Test Target", filepath.Join(root, "test_target_3s.wav")},
		{"lab_paper_crown", "Paper Crown AIGC Song", filepath.Join(root, "Paper Crown.mp3")},
		{"lab_jasmine", "Jasmine AIGC Instrumental", filepath.Join(root, "茉莉花四重奏_爱给网_aigei_com.mp3")},
	}
	out := []audioAsset{}
	for _, c := range candidates {
		if _, err := os.Stat(c.path); err != nil {
			continue
		}
		out = append(out, audioAsset{
			TrackID: c.id,
			ClipID:  "clip_" + c.id,
			Name:    c.name,
			Path:    filepath.Clean(c.path),
		})
	}
	return out
}

func waveformRowForAsset(asset audioAsset, segmentSeconds float64, now string) (map[string]any, error) {
	if !strings.EqualFold(filepath.Ext(asset.Path), ".wav") {
		return nil, fmt.Errorf("offline decoder supports wav only; use live kernel bake for %s", filepath.Ext(asset.Path))
	}
	info, err := readPCM16WAV(asset.Path)
	if err != nil {
		return nil, err
	}
	peak, rms := peakAndRMS(info.Samples)
	duration := float64(info.Frames) / float64(info.SampleRate)
	return map[string]any{
		"status":              "ready",
		"track_id":            asset.TrackID,
		"clip_id":             asset.ClipID,
		"file_path":           asset.Path,
		"request_id":          "mixlab_offline",
		"source":              "mixlab_offline_wav_scan",
		"total_duration":      round3(duration),
		"rms":                 round6(rms),
		"peak_abs":            round6(peak),
		"rms_dbfs":            dbfsValue(rms),
		"peak_dbfs":           dbfsValue(peak),
		"headroom_db":         headroomDBValue(peak),
		"crest_db":            crestDBValue(peak, rms),
		"float_count":         len(info.Samples),
		"tile_count_seen":     len(timeSegments(info, segmentSeconds)),
		"tile_count_expected": len(timeSegments(info, segmentSeconds)),
		"time_segments":       timeSegments(info, segmentSeconds),
		"updated_at":          now,
	}, nil
}

func blockedWaveformRow(asset audioAsset, reason, now string) map[string]any {
	return map[string]any{
		"status":     "blocked",
		"track_id":   asset.TrackID,
		"clip_id":    asset.ClipID,
		"file_path":  asset.Path,
		"request_id": "mixlab_offline",
		"reason":     reason,
		"updated_at": now,
	}
}

func readPCM16WAV(path string) (wavInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return wavInfo{}, err
	}
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return wavInfo{}, fmt.Errorf("not a RIFF/WAVE file")
	}
	var channels, bitsPerSample, audioFormat uint16
	var sampleRate uint32
	var pcm []byte
	for offset := 12; offset+8 <= len(data); {
		id := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := offset + 8
		end := start + size
		if end > len(data) {
			return wavInfo{}, fmt.Errorf("invalid wav chunk %s", id)
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return wavInfo{}, fmt.Errorf("invalid fmt chunk")
			}
			audioFormat = binary.LittleEndian.Uint16(data[start : start+2])
			channels = binary.LittleEndian.Uint16(data[start+2 : start+4])
			sampleRate = binary.LittleEndian.Uint32(data[start+4 : start+8])
			bitsPerSample = binary.LittleEndian.Uint16(data[start+14 : start+16])
		case "data":
			pcm = data[start:end]
		}
		offset = end
		if offset%2 == 1 {
			offset++
		}
	}
	if audioFormat != 1 {
		return wavInfo{}, fmt.Errorf("unsupported wav format %d; expected PCM", audioFormat)
	}
	if channels == 0 || sampleRate == 0 {
		return wavInfo{}, fmt.Errorf("missing wav format metadata")
	}
	if bitsPerSample != 16 {
		return wavInfo{}, fmt.Errorf("unsupported bits_per_sample %d; expected 16", bitsPerSample)
	}
	if len(pcm) == 0 {
		return wavInfo{}, fmt.Errorf("missing wav data chunk")
	}
	sampleCount := len(pcm) / 2
	samples := make([]float64, 0, sampleCount)
	reader := bytes.NewReader(pcm)
	for reader.Len() >= 2 {
		var sample int16
		if err := binary.Read(reader, binary.LittleEndian, &sample); err != nil {
			return wavInfo{}, err
		}
		samples = append(samples, float64(sample)/32768.0)
	}
	return wavInfo{
		SampleRate: int(sampleRate),
		Channels:   int(channels),
		Frames:     len(samples) / int(channels),
		Samples:    samples,
	}, nil
}

func peakAndRMS(samples []float64) (float64, float64) {
	if len(samples) == 0 {
		return 0, 0
	}
	peak := 0.0
	sumSquares := 0.0
	for _, sample := range samples {
		abs := math.Abs(sample)
		if abs > peak {
			peak = abs
		}
		sumSquares += sample * sample
	}
	return peak, math.Sqrt(sumSquares / float64(len(samples)))
}

func timeSegments(info wavInfo, segmentSeconds float64) []map[string]any {
	if segmentSeconds <= 0 {
		segmentSeconds = 2
	}
	framesPerSegment := int(math.Round(segmentSeconds * float64(info.SampleRate)))
	if framesPerSegment <= 0 {
		framesPerSegment = info.SampleRate * 2
	}
	rows := []map[string]any{}
	for startFrame := 0; startFrame < info.Frames; startFrame += framesPerSegment {
		endFrame := startFrame + framesPerSegment
		if endFrame > info.Frames {
			endFrame = info.Frames
		}
		startSample := startFrame * info.Channels
		endSample := endFrame * info.Channels
		peak, rms := peakAndRMS(info.Samples[startSample:endSample])
		rows = append(rows, map[string]any{
			"start_seconds": round3(float64(startFrame) / float64(info.SampleRate)),
			"end_seconds":   round3(float64(endFrame) / float64(info.SampleRate)),
			"rms":           round6(rms),
			"rms_dbfs":      dbfsValue(rms),
			"peak_abs":      round6(peak),
			"peak_dbfs":     dbfsValue(peak),
			"crest_db":      crestDBValue(peak, rms),
			"energy_state":  energyState(rms),
		})
	}
	return rows
}

func assetDuration(path string) float64 {
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		info, err := readPCM16WAV(path)
		if err == nil && info.SampleRate > 0 {
			return round3(float64(info.Frames) / float64(info.SampleRate))
		}
	}
	return 0
}

func projectDuration(tracks []any) float64 {
	maxDuration := 0.0
	for _, raw := range tracks {
		track, _ := raw.(map[string]any)
		clips, _ := track["clips"].([]any)
		for _, clipRaw := range clips {
			clip, _ := clipRaw.(map[string]any)
			duration := numberFromAny(clip["duration_seconds"])
			if duration > maxDuration {
				maxDuration = duration
			}
		}
	}
	return maxDuration
}

func buildSummary(result mixboard.WriteResult, snapshotPath, outputRoot string) map[string]any {
	project := result.Observation.ProjectPackage
	return map[string]any{
		"status":                      result.Status,
		"mix_session_id":              result.Observation.MixSessionID,
		"observation_id":              result.Observation.ObservationID,
		"output_root":                 outputRoot,
		"snapshot_path":               snapshotPath,
		"observation_path":            result.ObservationPath,
		"track_count":                 project["track_count"],
		"acoustic_track_count":        project["acoustic_track_count"],
		"active_acoustic_track_count": project["active_acoustic_track_count"],
		"loudness_ranking":            capRows(mapRowsAny(project["loudness_ranking"]), 8),
		"peak_ranking":                capRows(mapRowsAny(project["peak_ranking"]), 8),
		"headroom_risk":               capRows(mapRowsAny(project["headroom_risk"]), 8),
		"likely_first_attention":      project["likely_first_attention_target"],
		"limitations":                 project["limitations"],
	}
}

func printSummary(summary map[string]any) {
	fmt.Printf("status=%v tracks=%v acoustic_ready=%v/%v\n", summary["status"], summary["track_count"], summary["active_acoustic_track_count"], summary["acoustic_track_count"])
	if rows := mapRowsAny(summary["loudness_ranking"]); len(rows) > 0 {
		fmt.Printf("loudness_top=%s %.3f dBFS\n", stringValue(rows[0]["track_id"]), numberFromAny(rows[0]["value"]))
	}
	if rows := mapRowsAny(summary["headroom_risk"]); len(rows) > 0 {
		fmt.Printf("headroom_risk_top=%s %.3f dB risk=%s\n", stringValue(rows[0]["track_id"]), numberFromAny(rows[0]["headroom_db"]), stringValue(rows[0]["risk"]))
	}
	if attn, _ := summary["likely_first_attention"].(map[string]any); len(attn) > 0 {
		fmt.Printf("attention_reason=%s\n", stringValue(attn["reason"]))
	}
}

func writeJSON(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func defaultRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := wd; dir != ""; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "agent" {
			return filepath.Dir(dir)
		}
		if _, err := os.Stat(filepath.Join(dir, "agent", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return wd
}

func dbfsValue(value float64) any {
	if value <= 0 {
		return nil
	}
	return round3(20 * math.Log10(value))
}

func headroomDBValue(peak float64) any {
	if peak <= 0 {
		return nil
	}
	return round3(-20 * math.Log10(peak))
}

func crestDBValue(peak, rms float64) any {
	if peak <= 0 || rms <= 0 {
		return nil
	}
	return round3(20 * math.Log10(peak/rms))
}

func energyState(rms float64) string {
	switch {
	case rms <= 0.0001:
		return "silent"
	case rms < 0.02:
		return "low"
	case rms < 0.12:
		return "medium"
	default:
		return "high"
	}
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func round6(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func numberFromAny(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		var n float64
		_, _ = fmt.Sscanf(strings.TrimSpace(v), "%f", &n)
		return n
	default:
		return 0
	}
}

func stringValue(value any) string {
	return strings.TrimSpace(fmt.Sprint(value))
}

func mapRowsAny(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, raw := range rows {
			if row, ok := raw.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func capRows(rows []map[string]any, max int) []map[string]any {
	if max <= 0 || len(rows) <= max {
		return rows
	}
	return rows[:max]
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
