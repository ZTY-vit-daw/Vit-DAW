package shadow

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
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

	var retainedNodes map[string]any
	if p.initialized {
		if oldNodes, _ := p.state["nodes_by_uid"].(map[string]any); len(oldNodes) > 0 && shouldRetainNodeDeltas(p.state["engine_snapshot"], full) {
			retainedNodes = retainNodeDeltasForSnapshot(full, oldNodes)
		}
	}
	if retainedNodes == nil {
		retainedNodes = map[string]any{}
	}
	p.state = map[string]any{
		"engine_snapshot": clone(full),
		"nodes_by_uid":    retainedNodes,
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
			len(pending)+len(retainedNodes),
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
	userTracks := compactUserTracks(tracks, nodes)
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

func shouldRetainNodeDeltas(oldEngine any, nextEngine map[string]any) bool {
	old, _ := oldEngine.(map[string]any)
	if len(old) == 0 || len(nextEngine) == 0 {
		return false
	}
	oldPath := snapshotProjectPath(old)
	nextPath := snapshotProjectPath(nextEngine)
	return oldPath == "" || nextPath == "" || oldPath == nextPath
}

func snapshotProjectPath(snapshot map[string]any) string {
	value := firstPresent(snapshot, "project_path", "current_project_path", "file_path")
	if value == nil {
		return ""
	}
	path := strings.TrimSpace(fmt.Sprint(value))
	if path == "<nil>" {
		return ""
	}
	return path
}

func retainNodeDeltasForSnapshot(full map[string]any, oldNodes map[string]any) map[string]any {
	validUIDs := collectSnapshotUIDs(full)
	if len(validUIDs) == 0 || len(oldNodes) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	for uid, entry := range oldNodes {
		if validUIDs[uid] {
			out[uid] = cloneAny(entry)
		}
	}
	return out
}

func collectSnapshotUIDs(full map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, it := range asArray(full["tracks"]) {
		row, ok := it.(map[string]any)
		if !ok {
			continue
		}
		addUID(out, trackIDFromRow(row))
		for _, plugin := range asArray(firstPresent(row, "plugins", "rack_nodes")) {
			if pluginRow, ok := plugin.(map[string]any); ok {
				addUID(out, trackIDFromPluginRow(pluginRow))
			}
		}
		for _, clip := range asArray(firstPresent(row, "clips", "clip_summaries")) {
			if clipRow, ok := clip.(map[string]any); ok {
				addUID(out, strings.TrimSpace(fmt.Sprint(firstPresent(clipRow, "clip_id", "id", "uid"))))
			}
		}
	}
	return out
}

func addUID(out map[string]bool, uid string) {
	uid = strings.TrimSpace(uid)
	if uid != "" && uid != "<nil>" {
		out[uid] = true
	}
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

func compactUserTracks(tracks []any, nodes map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(tracks))
	for _, it := range tracks {
		row, ok := it.(map[string]any)
		if !ok || !isUserTrack(row) {
			continue
		}
		out = append(out, compactTrack(trackRowWithDeltaProperties(row, nodes), len(out)+1))
	}
	return out
}

func trackRowWithDeltaProperties(row map[string]any, nodes map[string]any) map[string]any {
	if len(nodes) == 0 {
		return row
	}
	out := clone(row)
	applyTrackDeltaProperties(out, nodeDeltaProperties(nodes, trackIDFromRow(row)))
	for _, plugin := range asArray(firstPresent(row, "plugins", "rack_nodes")) {
		pluginRow, ok := plugin.(map[string]any)
		if !ok || !isTrackVolumePanPlugin(pluginRow) {
			continue
		}
		applyTrackVolumePluginDeltaProperties(out, nodeDeltaProperties(nodes, trackIDFromPluginRow(pluginRow)))
	}
	return out
}

func nodeDeltaProperties(nodes map[string]any, uid string) map[string]any {
	if uid == "" || nodes == nil {
		return nil
	}
	entry, _ := nodes[uid].(map[string]any)
	props, _ := entry["delta_properties"].(map[string]any)
	return props
}

func applyTrackDeltaProperties(row map[string]any, props map[string]any) {
	if len(props) == 0 {
		return
	}
	if v, ok := firstProperty(props, "name", "track_name"); ok {
		row["name"] = cloneAny(v)
		row["track_name"] = cloneAny(v)
	}
	if v, ok := firstProperty(props, "mute", "muted"); ok {
		row["mute"] = cloneAny(v)
	}
	if v, ok := firstProperty(props, "solo", "is_solo"); ok {
		row["solo"] = cloneAny(v)
	}
	if v, ok := firstProperty(props, "is_armed", "armed", "record_armed"); ok {
		row["is_armed"] = cloneAny(v)
		row["armed"] = cloneAny(v)
	}
	if v, ok := firstProperty(props, "volume_db", "volumeDb", "fader_db", "faderDb", "gain_db", "gainDb", "db"); ok {
		row["volume_db"] = cloneAny(v)
		row["fader_db"] = cloneAny(v)
		row["gain_db"] = cloneAny(v)
	}
	if v, ok := firstProperty(props, "level_db", "levelDb", "peak_db", "peakDb", "meter_peak_db", "meter_level_db"); ok {
		row["level_db"] = cloneAny(v)
	}
	if v, ok := firstProperty(props, "left_level_db", "leftLevelDb", "left_peak_db", "leftPeakDb", "level_l_db", "peak_l_db"); ok {
		row["left_level_db"] = cloneAny(v)
	}
	if v, ok := firstProperty(props, "right_level_db", "rightLevelDb", "right_peak_db", "rightPeakDb", "level_r_db", "peak_r_db"); ok {
		row["right_level_db"] = cloneAny(v)
	}
}

func applyTrackVolumePluginDeltaProperties(row map[string]any, props map[string]any) {
	if len(props) == 0 {
		return
	}
	if v, ok := firstProperty(props, "volume_db", "volumeDb", "fader_db", "faderDb", "gain_db", "gainDb", "db"); ok {
		row["volume_db"] = cloneAny(v)
		row["fader_db"] = cloneAny(v)
		row["gain_db"] = cloneAny(v)
		return
	}
	if v, ok := firstProperty(props, "volume"); ok {
		if db, ok := volumeFaderPositionToDB(v); ok {
			row["volume_db"] = db
			row["fader_db"] = db
			row["gain_db"] = db
		}
	}
}

func firstProperty(row map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if v, ok := row[key]; ok && !isEmptyValue(v) {
			return v, true
		}
	}
	return nil, false
}

func trackIDFromRow(row map[string]any) string {
	return strings.TrimSpace(fmt.Sprint(firstPresent(row, "track_id", "id", "uid", "kernel_track_id")))
}

func trackIDFromPluginRow(row map[string]any) string {
	return strings.TrimSpace(fmt.Sprint(firstPresent(row, "plugin_item_id", "item_id", "plugin_id", "id", "node_id")))
}

func isTrackVolumePanPlugin(row map[string]any) bool {
	label := normalizedIdentityText(fmt.Sprint(firstPresent(row, "plugin_name", "name", "display_name", "label")))
	kind := normalizedIdentityText(fmt.Sprint(firstPresent(row, "type", "plugin_type", "kind", "role", "slot")))
	return kind == "volume" ||
		strings.Contains(kind, "volumeandpan") ||
		strings.Contains(label, "volumeandpan") ||
		strings.Contains(label, "volumepan") ||
		(strings.Contains(label, "volume") && strings.Contains(label, "pan"))
}

func normalizedIdentityText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func volumeFaderPositionToDB(v any) (float64, bool) {
	position, ok := float64From(v)
	if !ok {
		return 0, false
	}
	if position <= 0 {
		return -100, true
	}
	return math.Max(-100, 20*math.Log(position)+6), true
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
		"mute":             firstPresent(row, "mute", "muted"),
		"solo":             firstPresent(row, "solo", "is_solo"),
		"is_armed":         firstPresent(row, "is_armed", "armed"),
		"armed":            firstPresent(row, "armed", "is_armed"),
		"volume_db":        firstPresent(row, "volume_db", "volumeDb"),
		"fader_db":         firstPresent(row, "fader_db", "faderDb"),
		"gain_db":          firstPresent(row, "gain_db", "gainDb"),
		"pan":              firstPresent(row, "pan", "pan_value", "panValue"),
		"level_db":         firstPresent(row, "level_db", "levelDb", "peak_db", "peakDb", "meter_peak_db", "meter_level_db"),
		"left_level_db":    firstPresent(row, "left_level_db", "leftLevelDb", "left_peak_db", "leftPeakDb", "level_l_db", "peak_l_db"),
		"right_level_db":   firstPresent(row, "right_level_db", "rightLevelDb", "right_peak_db", "rightPeakDb", "level_r_db", "peak_r_db"),
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

func float64From(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		n, err := x.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n, err == nil
	default:
		return 0, false
	}
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
