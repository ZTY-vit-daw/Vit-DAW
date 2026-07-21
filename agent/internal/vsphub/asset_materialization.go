package vsphub

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaterializeMaxTiles  = 64
	defaultMaterializeMaxFloats = 262144
	maxMaterializedTiles        = 8192
	materializedTileTTL         = 10 * time.Minute
)

type AssetMaterializationStore struct {
	mu sync.RWMutex

	tiles map[string]AudioFeatureTile
	order []string

	shmReadOK      int64
	shmReadMiss    int64
	storedTiles    int64
	requestCount   int64
	hitCount       int64
	notFoundCount  int64
	unsupported    int64
	lastError      string
	lastStoredClip string
}

type AudioFeatureTile struct {
	Key              string
	ClipID           string
	KernelTrackID    string
	Kind             string
	FeatureType      string
	TileIndex        int
	TileStartSeconds float64
	TileDuration     float64
	FloatCount       int
	Data             []float32
	Metadata         map[string]any
	CreatedAt        time.Time
	MaterializedFrom string
}

func NewAssetMaterializationStore() *AssetMaterializationStore {
	return &AssetMaterializationStore{
		tiles: map[string]AudioFeatureTile{},
		order: []string{},
	}
}

func (s *AssetMaterializationStore) RecordTelemetry(telemetry any) {
	if s == nil {
		return
	}
	item := mapFromAny(telemetry)
	if strings.ToLower(cleanString(item["command"])) != "audio_feature_data_ready" {
		return
	}
	featureType := strings.ToLower(cleanString(item["feature_type"]))
	if featureType != "waveform_envelope" && featureType != "time_energy" {
		return
	}
	clipID := cleanString(item["clip_id"])
	trackID := cleanString(item["track_id"])
	shmName := cleanString(item["shared_memory"])
	floatCount := intFromAny(item["float_count"], 0)
	if clipID == "" || trackID == "" || shmName == "" || floatCount <= 0 {
		return
	}
	data, err := readSharedMemoryFloat32(shmName, floatCount)
	if err != nil || len(data) != floatCount {
		s.mu.Lock()
		s.shmReadMiss++
		if err != nil {
			s.lastError = err.Error()
		} else {
			s.lastError = fmt.Sprintf("shared memory returned %d floats, expected %d", len(data), floatCount)
		}
		s.mu.Unlock()
		return
	}
	metadata := cloneMap(item)
	delete(metadata, "shared_memory")
	tile := AudioFeatureTile{
		ClipID:           clipID,
		KernelTrackID:    trackID,
		Kind:             "waveform_peak",
		FeatureType:      featureType,
		TileIndex:        intFromAny(item["tile_index"], -1),
		TileStartSeconds: floatFromAny(item["tile_content_start_seconds"], 0.0),
		TileDuration:     floatFromAny(item["tile_duration"], 0.0),
		FloatCount:       floatCount,
		Data:             data,
		Metadata:         metadata,
		CreatedAt:        time.Now().UTC(),
		MaterializedFrom: "kernel.shared_memory",
	}
	tile.Key = materializedTileKey(tile)
	s.storeTile(tile)
}

func (s *AssetMaterializationStore) storeTile(tile AudioFeatureTile) {
	if s == nil || tile.ClipID == "" || tile.KernelTrackID == "" || tile.FeatureType == "" || len(tile.Data) == 0 {
		return
	}
	if tile.Kind == "" {
		tile.Kind = "waveform_peak"
	}
	if tile.CreatedAt.IsZero() {
		tile.CreatedAt = time.Now().UTC()
	}
	if tile.FloatCount <= 0 {
		tile.FloatCount = len(tile.Data)
	}
	if tile.Key == "" {
		tile.Key = materializedTileKey(tile)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now().UTC())
	if _, exists := s.tiles[tile.Key]; !exists {
		s.order = append(s.order, tile.Key)
	}
	tile.Data = append([]float32(nil), tile.Data...)
	tile.Metadata = cloneMap(tile.Metadata)
	s.tiles[tile.Key] = tile
	s.shmReadOK++
	s.storedTiles++
	s.lastError = ""
	s.lastStoredClip = tile.ClipID
	for len(s.order) > maxMaterializedTiles {
		old := s.order[0]
		s.order = s.order[1:]
		delete(s.tiles, old)
	}
}

func (h *Hub) handleAssetMaterializeLocal(env *Envelope) HubResponse {
	if h == nil || h.assets == nil {
		return ErrorResponse(env, http.StatusServiceUnavailable, "asset", "asset.error", "asset_materializer_unavailable", "asset materializer is not available")
	}
	payload := env.Payload()
	clipID := cleanString(payload["clip_id"])
	if clipID == "" {
		h.errors.Add(1)
		return ErrorResponse(env, http.StatusBadRequest, "asset", "asset.error", "validation_error", "asset.materialize_request requires clip_id")
	}
	replyPayload := h.assets.Materialize(payload)
	return JSONResponse(http.StatusOK, h.localReply(env, "vsp.asset.materialized.v1", "asset", "asset.materialized", replyPayload))
}

func (s *AssetMaterializationStore) Materialize(request map[string]any) map[string]any {
	if s == nil {
		return map[string]any{"status": "error", "code": "asset_materializer_unavailable"}
	}
	clipID := cleanString(request["clip_id"])
	trackID := firstNonEmpty(cleanString(request["kernel_track_id"]), cleanString(request["track_id"]))
	kind := strings.ToLower(firstNonEmpty(cleanString(request["kind"]), "waveform_peak"))
	featureType := strings.ToLower(firstNonEmpty(cleanString(request["feature_type"]), "waveform_envelope"))
	maxTiles := intFromAny(request["max_tiles"], defaultMaterializeMaxTiles)
	if maxTiles <= 0 {
		maxTiles = defaultMaterializeMaxTiles
	}
	if maxTiles > 256 {
		maxTiles = 256
	}
	maxFloats := intFromAny(request["max_float_count"], defaultMaterializeMaxFloats)
	if maxFloats <= 0 {
		maxFloats = defaultMaterializeMaxFloats
	}
	rangeStart, rangeEnd, hasRange := materializeRange(request)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestCount++
	s.pruneLocked(now)
	if kind != "waveform_peak" {
		s.unsupported++
		return map[string]any{
			"status":  "unsupported",
			"code":    "asset_kind_not_materialized",
			"kind":    kind,
			"clip_id": clipID,
		}
	}

	matches := make([]AudioFeatureTile, 0, 8)
	for _, tile := range s.tiles {
		if tile.ClipID != clipID {
			continue
		}
		if trackID != "" && tile.KernelTrackID != trackID {
			continue
		}
		if featureType != "" && tile.FeatureType != featureType {
			continue
		}
		if hasRange && !tileOverlapsRange(tile, rangeStart, rangeEnd) {
			continue
		}
		matches = append(matches, tile)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].KernelTrackID != matches[j].KernelTrackID {
			return matches[i].KernelTrackID < matches[j].KernelTrackID
		}
		if matches[i].TileStartSeconds != matches[j].TileStartSeconds {
			return matches[i].TileStartSeconds < matches[j].TileStartSeconds
		}
		return matches[i].TileIndex < matches[j].TileIndex
	})

	outTiles := make([]any, 0, len(matches))
	totalFloats := 0
	truncated := false
	for _, tile := range matches {
		if len(outTiles) >= maxTiles || totalFloats+len(tile.Data) > maxFloats {
			truncated = true
			break
		}
		totalFloats += len(tile.Data)
		meta := cloneMap(tile.Metadata)
		if _, ok := meta["feature_type"]; !ok {
			meta["feature_type"] = tile.FeatureType
		}
		if _, ok := meta["tile_index"]; !ok {
			meta["tile_index"] = tile.TileIndex
		}
		if _, ok := meta["tile_content_start_seconds"]; !ok {
			meta["tile_content_start_seconds"] = tile.TileStartSeconds
		}
		if _, ok := meta["tile_duration"]; !ok {
			meta["tile_duration"] = tile.TileDuration
		}
		outTiles = append(outTiles, map[string]any{
			"clip_id":         tile.ClipID,
			"kernel_track_id": tile.KernelTrackID,
			"track_id":        tile.KernelTrackID,
			"kind":            tile.Kind,
			"feature_type":    tile.FeatureType,
			"float_count":     len(tile.Data),
			"data":            append([]float32(nil), tile.Data...),
			"metadata":        meta,
			"materialized_at": tile.CreatedAt.Format(time.RFC3339Nano),
			"source":          tile.MaterializedFrom,
		})
	}
	if len(outTiles) == 0 {
		s.notFoundCount++
		return map[string]any{
			"status":          "not_found",
			"code":            "asset_materialization_cache_miss",
			"clip_id":         clipID,
			"kernel_track_id": trackID,
			"kind":            kind,
			"feature_type":    featureType,
			"tile_count":      0,
			"cached":          false,
		}
	}
	s.hitCount++
	return map[string]any{
		"status":              "ok",
		"clip_id":             clipID,
		"kernel_track_id":     trackID,
		"kind":                kind,
		"feature_type":        featureType,
		"format":              "f32_array_tiles.v1",
		"inline_payload":      true,
		"no_big_json_payload": false,
		"cached":              true,
		"tile_count":          len(outTiles),
		"float_count":         totalFloats,
		"truncated":           truncated,
		"max_tiles":           maxTiles,
		"max_float_count":     maxFloats,
		"tiles":               outTiles,
	}
}

func (s *AssetMaterializationStore) Snapshot() map[string]any {
	if s == nil {
		return map[string]any{"available": false}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now().UTC())
	return map[string]any{
		"available":         true,
		"cached_tiles":      len(s.tiles),
		"shm_read_ok":       s.shmReadOK,
		"shm_read_miss":     s.shmReadMiss,
		"stored_tiles":      s.storedTiles,
		"request_count":     s.requestCount,
		"hit_count":         s.hitCount,
		"not_found_count":   s.notFoundCount,
		"unsupported_count": s.unsupported,
		"last_error":        s.lastError,
		"last_stored_clip":  s.lastStoredClip,
	}
}

func (s *AssetMaterializationStore) pruneLocked(now time.Time) {
	if len(s.order) == 0 {
		return
	}
	cutoff := now.Add(-materializedTileTTL)
	next := s.order[:0]
	for _, key := range s.order {
		tile, ok := s.tiles[key]
		if !ok {
			continue
		}
		if tile.CreatedAt.Before(cutoff) {
			delete(s.tiles, key)
			continue
		}
		next = append(next, key)
	}
	s.order = next
}

func materializedTileKey(tile AudioFeatureTile) string {
	return fmt.Sprintf("%s|%s|%s|%d|%.6f",
		tile.ClipID,
		tile.KernelTrackID,
		tile.FeatureType,
		tile.TileIndex,
		tile.TileStartSeconds,
	)
}

func materializeRange(request map[string]any) (float64, float64, bool) {
	rangeObj := mapFromAny(request["visible_range"])
	if len(rangeObj) == 0 {
		rangeObj = mapFromAny(request["range"])
	}
	if len(rangeObj) == 0 {
		return 0, 0, false
	}
	start, hasStart := floatFromAnyOK(firstPresent(rangeObj, "start_seconds", "start", "range_start_seconds"))
	end, hasEnd := floatFromAnyOK(firstPresent(rangeObj, "end_seconds", "end", "range_end_seconds"))
	if !hasEnd {
		duration, hasDuration := floatFromAnyOK(firstPresent(rangeObj, "duration_seconds", "duration", "length_seconds", "length"))
		if hasStart && hasDuration && duration >= 0 {
			end = start + duration
			hasEnd = true
		}
	}
	if !hasStart || !hasEnd || end < start {
		return 0, 0, false
	}
	return start, end, true
}

func tileOverlapsRange(tile AudioFeatureTile, start, end float64) bool {
	tileStart := tile.TileStartSeconds
	tileEnd := tileStart + tile.TileDuration
	if tile.TileDuration <= 0 {
		tileEnd = tileStart
	}
	return tileEnd >= start && tileStart <= end
}

func firstPresent(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func floatFromAny(value any, fallback float64) float64 {
	out, ok := floatFromAnyOK(value)
	if !ok {
		return fallback
	}
	return out
}

func floatFromAnyOK(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case jsonNumber:
		f, err := v.Float64()
		return f, err == nil
	}
	text := cleanString(value)
	if text == "" {
		return 0, false
	}
	var out float64
	if _, err := fmt.Sscanf(text, "%f", &out); err == nil {
		return out, true
	}
	return 0, false
}

type jsonNumber interface {
	Float64() (float64, error)
}
