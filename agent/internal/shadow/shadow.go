package shadow

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"vit-daw-agent/internal/logx"
)

const orphanHistoryMax = 20

type Project struct {
	logger *logx.Logger

	mu             sync.RWMutex
	state          map[string]any
	initialized    bool
	preInitDeltas  []map[string]any
	orphanDeltas   map[string][]map[string]any
	bootstrapUIDs  map[string]bool
	lastDeltaSeqID int64
	seqGapCount    int64
}

func New(logger *logx.Logger) *Project {
	return &Project{
		logger:        logger,
		state:         map[string]any{},
		orphanDeltas:  map[string][]map[string]any{},
		bootstrapUIDs: map[string]bool{},
	}
}

func (p *Project) Initialize(full map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state = map[string]any{
		"engine_snapshot": clone(full),
		"nodes_by_uid":    map[string]any{},
	}
	p.bootstrapUIDs = collectTrackUIDs(full)
	p.initialized = true
	pending := append([]map[string]any(nil), p.preInitDeltas...)
	p.preInitDeltas = nil

	for _, d := range pending {
		p.applyDeltaLocked(d, true)
	}

	if p.logger != nil {
		p.logger.Info("[shadow] initialized tracks=%d project_path=%q replayed_deltas=%d",
			len(asArray(full["tracks"])),
			fmt.Sprint(full["project_path"]),
			len(pending),
		)
	}
}

func (p *Project) ApplyDelta(delta map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.applyDeltaLocked(delta, false)
}

func (p *Project) Initialized() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initialized
}

func (p *Project) Snapshot() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return clone(p.state)
}

func (p *Project) Summary() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	engine, _ := p.state["engine_snapshot"].(map[string]any)
	nodes, _ := p.state["nodes_by_uid"].(map[string]any)
	tracks := asArray(engine["tracks"])
	userTracks := compactUserTracks(tracks)
	observability := sanitizedObservability(engine["observability"], len(userTracks))

	out := map[string]any{
		"initialized":          p.initialized,
		"project_path":         fmt.Sprint(engine["project_path"]),
		"track_count":          len(userTracks),
		"user_track_count":     len(userTracks),
		"engine_track_count":   len(tracks),
		"internal_track_count": len(tracks) - len(userTracks),
		"nodes_by_uid":         len(nodes),
		"orphan_uid_keys":      len(p.orphanDeltas),
		"last_delta_seq":       p.lastDeltaSeqID,
		"delta_seq_gaps":       p.seqGapCount,
		"tracks":               userTracks,
		"observability":        observability,
		"project_health":       cloneMap(engine["project_health"]),
		"graph_revision":       engine["graph_revision"],
		"pending_job_data":     cloneMap(engine["jobs"]),
	}
	return out
}

func (p *Project) applyDeltaLocked(delta map[string]any, fromReplay bool) {
	if strings.TrimSpace(fmt.Sprint(delta["type"])) != "delta_update" {
		return
	}
	if !p.initialized {
		p.preInitDeltas = append(p.preInitDeltas, clone(delta))
		return
	}

	uid := strings.TrimSpace(fmt.Sprint(delta["target_uid"]))
	if uid == "" {
		return
	}
	action := strings.TrimSpace(fmt.Sprint(delta["action"]))
	seqID := int64From(delta["seq_id"])
	if seqID > 0 {
		if p.lastDeltaSeqID > 0 && seqID != p.lastDeltaSeqID+1 {
			p.seqGapCount++
			if p.logger != nil {
				p.logger.Warn("[shadow] delta seq gap #%d last=%d current=%d", p.seqGapCount, p.lastDeltaSeqID, seqID)
			}
		}
		p.lastDeltaSeqID = seqID
	}

	nodes, _ := p.state["nodes_by_uid"].(map[string]any)
	if nodes == nil {
		nodes = map[string]any{}
		p.state["nodes_by_uid"] = nodes
	}
	entry, _ := nodes[uid].(map[string]any)
	if entry == nil {
		entry = map[string]any{
			"delta_properties": map[string]any{},
		}
		nodes[uid] = entry
	}
	if prop := propertyKey(action); prop != "" {
		props, _ := entry["delta_properties"].(map[string]any)
		if props == nil {
			props = map[string]any{}
			entry["delta_properties"] = props
		}
		props[prop] = cloneAny(delta["value"])
	}
	entry["last_seq_id"] = delta["seq_id"]
	entry["last_timestamp"] = delta["timestamp"]
	entry["last_action"] = action

	if !p.bootstrapUIDs[uid] {
		h := append(p.orphanDeltas[uid], clone(delta))
		if len(h) > orphanHistoryMax {
			h = h[len(h)-orphanHistoryMax:]
		}
		p.orphanDeltas[uid] = h
	}

	if p.logger != nil && !isTransportPositionNoise(uid, action) {
		tag := "live"
		if fromReplay {
			tag = "replay"
		}
		p.logger.Debug("[shadow] %s delta uid=%q action=%q", tag, uid, action)
	}
}

func collectTrackUIDs(full map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, it := range asArray(full["tracks"]) {
		row, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(row["track_id"]))
		if id == "" || id == "<nil>" {
			id = strings.TrimSpace(fmt.Sprint(row["id"]))
		}
		if id != "" && id != "<nil>" {
			out[id] = true
		}
	}
	return out
}

func compactTracks(tracks []any) []map[string]any {
	out := make([]map[string]any, 0, len(tracks))
	for _, it := range tracks {
		row, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"id":       row["id"],
			"track_id": row["track_id"],
			"name":     row["name"],
			"type":     row["type"],
			"plugins":  row["plugins"],
			"clips":    row["clips"],
		})
	}
	return out
}

func compactUserTracks(tracks []any) []map[string]any {
	out := make([]map[string]any, 0, len(tracks))
	for _, it := range tracks {
		row, ok := it.(map[string]any)
		if !ok || !isUserTrack(row) {
			continue
		}
		out = append(out, compactTrack(row, len(out)+1))
	}
	return out
}

func compactTrack(row map[string]any, userIndex int) map[string]any {
	return map[string]any{
		"user_track_index": userIndex,
		"is_user_visible":  true,
		"id":               firstPresent(row, "id", "track_id"),
		"track_id":         firstPresent(row, "track_id", "id"),
		"name":             firstPresent(row, "name", "track_name"),
		"track_name":       firstPresent(row, "track_name", "name"),
		"type":             firstPresent(row, "type", "track_type"),
		"track_type":       firstPresent(row, "track_type", "type"),
		"is_audio_track":   row["is_audio_track"],
		"plugins":          row["plugins"],
		"clips":            row["clips"],
		"rack":             row["rack"],
	}
}

func isUserTrack(row map[string]any) bool {
	trackType := strings.ToLower(strings.TrimSpace(fmt.Sprint(firstPresent(row, "track_type", "type"))))
	if trackType == "master" || trackType == "arranger" || trackType == "chord" || trackType == "marker" || trackType == "tempo" {
		return false
	}
	trackName := strings.ToLower(strings.TrimSpace(fmt.Sprint(firstPresent(row, "track_name", "name"))))
	if trackName == "master" || trackName == "arranger" || trackName == "chord" || trackName == "marker" || trackName == "tempo" {
		return false
	}
	if boolFrom(row["is_audio_track"]) || boolFrom(row["is_audio"]) {
		return true
	}
	vitType := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["vit_type"])))
	return vitType == "audio" || vitType == "midi" || vitType == "bus" || vitType == "ghost"
}

func sanitizedObservability(v any, userTrackCount int) any {
	out := cloneMap(v)
	m, ok := out.(map[string]any)
	if !ok {
		return out
	}
	profile, ok := m["profile"].(map[string]any)
	if !ok {
		return m
	}
	profile["track_count"] = userTrackCount
	profile["user_track_count"] = userTrackCount
	return m
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := row[key]; ok && !isEmptyValue(v) {
			return v
		}
	}
	return nil
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || s == "<nil>"
}

func boolFrom(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}

func propertyKey(action string) string {
	const prefix = "property_changed:"
	if strings.HasPrefix(action, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(action, prefix))
	}
	return ""
}

func isTransportPositionNoise(uid, action string) bool {
	return uid == "TRANSPORT" && action == "property_changed:position"
}

func cloneMap(v any) any {
	if v == nil {
		return nil
	}
	return cloneAny(v)
}

func clone(in map[string]any) map[string]any {
	var out map[string]any
	b, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func cloneAny(in any) any {
	var out any
	b, err := json.Marshal(in)
	if err != nil {
		return in
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return in
	}
	return out
}

func asArray(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return nil
}

func int64From(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	default:
		return 0
	}
}
