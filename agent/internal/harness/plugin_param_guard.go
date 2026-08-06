package harness

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ParamSnapshot holds the cached parameter snapshot for one plugin instance.
type ParamSnapshot struct {
	TrackID     string          `json:"track_id"`
	PluginID    string          `json:"plugin_id"`
	ParamIDs    map[string]bool `json:"param_ids"`
	ParamIDList []string        `json:"param_id_list"`
	LoadedAt    time.Time       `json:"loaded_at"`
}

func cacheKey(trackID, pluginID string) string {
	return trackID + ":" + pluginID
}

// PluginSnapshotCache stores the latest get_plugin_parameters snapshot
// for each (track, plugin) pair, so that set_plugin_param can validate
// param_id before writing.
type PluginSnapshotCache struct {
	mu    sync.RWMutex
	store map[string]*ParamSnapshot
}

func NewPluginSnapshotCache() *PluginSnapshotCache {
	return &PluginSnapshotCache{
		store: make(map[string]*ParamSnapshot),
	}
}

// Update stores a new snapshot from a get_plugin_parameters kernel reply.
func (c *PluginSnapshotCache) Update(reply map[string]any) bool {
	trackID := firstNonEmpty(firstString(reply, "track_id"), "")
	pluginID := firstNonEmpty(firstString(reply, "plugin_id", "plugin_item_id"), "")
	if trackID == "" || pluginID == "" {
		return false
	}
	params := mapRowsFromAny(reply["parameters"])
	if len(params) == 0 {
		return false
	}
	key := cacheKey(trackID, pluginID)
	ids := make(map[string]bool, len(params))
	idList := make([]string, 0, len(params))
	for _, row := range params {
		id := firstNonEmpty(firstString(row, "id", "param_id", "raw_param_id"), "")
		if id != "" {
			ids[id] = true
			idList = append(idList, id)
		}
	}
	if len(ids) == 0 {
		return false
	}

	sort.Strings(idList)

	snap := &ParamSnapshot{
		TrackID:     trackID,
		PluginID:    pluginID,
		ParamIDs:    ids,
		ParamIDList: idList,
		LoadedAt:    time.Now(),
	}
	c.mu.Lock()
	c.store[key] = snap
	c.mu.Unlock()
	return true
}

func (c *PluginSnapshotCache) GetSnapshot(trackID, pluginID string) *ParamSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap, ok := c.store[cacheKey(trackID, pluginID)]
	if !ok {
		return nil
	}
	return snap
}

func (c *PluginSnapshotCache) HasParamID(trackID, pluginID, paramID string) bool {
	snap := c.GetSnapshot(trackID, pluginID)
	if snap == nil {
		return false
	}
	return snap.ParamIDs[paramID]
}

func (h *Harness) ObservePluginParametersReply(reply map[string]any) bool {
	if h == nil || h.snapshotCache == nil || !kernelReplySucceeded(reply) {
		return false
	}
	return h.snapshotCache.Update(reply)
}

func (h *Harness) validateSetPluginParam(cmd map[string]any) error {
	trackID := firstNonEmpty(firstString(cmd, "track_id"), "")
	pluginID := firstNonEmpty(firstString(cmd, "plugin_id"), "")
	paramID := firstNonEmpty(firstString(cmd, "param_id"), "")

	if h == nil || h.snapshotCache == nil {
		return nil
	}
	if trackID == "" || pluginID == "" {
		return nil
	}

	snap := h.snapshotCache.GetSnapshot(trackID, pluginID)
	if snap == nil {
		validIDs := h.formatAvailablePluginSnapshots(trackID, pluginID)
		hint := ""
		if validIDs != "" {
			hint = ". " + validIDs
		}
		return fmt.Errorf(
			"Plugin parameters have not been read for track=%s plugin=%s. "+
				"Use get_plugin_parameters to read the current parameter set before writing%s",
			trackID, pluginID, hint,
		)
	}

	if paramID == "" {
		return fmt.Errorf("set_plugin_param requires a non-empty param_id")
	}

	if !snap.ParamIDs[paramID] {
		return fmt.Errorf(
			"Parameter %q is not in the current plugin parameter set for track=%s plugin=%s. "+
				"Valid parameter IDs: %s. "+
				"If the plugin was reloaded or updated, re-run get_plugin_parameters to refresh.",
			paramID, trackID, pluginID, formatParamIDList(snap.ParamIDList, 20),
		)
	}

	return nil
}

func (h *Harness) formatAvailablePluginSnapshots(trackID, pluginID string) string {
	if h == nil || h.snapshotCache == nil {
		return ""
	}
	h.snapshotCache.mu.RLock()
	defer h.snapshotCache.mu.RUnlock()

	var hints []string
	for key, snap := range h.snapshotCache.store {
		if strings.Contains(key, trackID) || strings.Contains(key, pluginID) {
			hints = append(hints, fmt.Sprintf("snapshot: track=%s plugin=%s (%d params, loaded %s)",
				snap.TrackID, snap.PluginID, len(snap.ParamIDs),
				snap.LoadedAt.Format("15:04:05")))
		}
		if len(hints) >= 3 {
			break
		}
	}
	if len(hints) > 0 {
		return "Available snapshots: " + strings.Join(hints, "; ") + "."
	}
	return ""
}

func formatParamIDList(ids []string, limit int) string {
	if len(ids) == 0 {
		return "(none)"
	}
	shown := ids
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
	}
	out := strings.Join(shown, ", ")
	if len(ids) > limit {
		out += fmt.Sprintf(" ... and %d more", len(ids)-limit)
	}
	return out
}

func boolFromAny(v any) bool {
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
