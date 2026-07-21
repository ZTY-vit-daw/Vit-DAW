package vsphub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type RealtimeStore struct {
	mu             sync.RWMutex
	subscriptions  map[string]*RealtimeSubscription
	latest         map[string]*RealtimeFrame
	published      int64
	cacheServed    int64
	cacheMisses    int64
	subscribeLocal int64
}

type RealtimeSubscription struct {
	ID        string
	SessionID string
	ClientID  string
	Role      string
	LocalOnly bool
	CreatedAt time.Time
	Streams   map[string]RealtimeStreamSpec
}

type RealtimeStreamSpec struct {
	Stream       string
	StreamID     string
	Key          string
	TrackIDs     []string
	MaxHz        int
	Mode         string
	VisibleRange map[string]any
}

type RealtimeFrame struct {
	Stream     string
	Key        string
	FrameIndex int64
	CreatedAt  time.Time
	Payload    map[string]any
}

type RealtimeDeliveryFrame struct {
	SessionID string
	Payload   map[string]any
	MaxHz     int
	Key       string
}

func NewRealtimeStore() *RealtimeStore {
	return &RealtimeStore{
		subscriptions: map[string]*RealtimeSubscription{},
		latest:        map[string]*RealtimeFrame{},
	}
}

func (s *RealtimeStore) Subscribe(env *Envelope, localOnly bool) map[string]any {
	payload := env.Payload()
	streamSpecs := specsFromAny(payload["streams"])
	subID := strings.TrimSpace(fmt.Sprint(payload["subscription_id"]))
	if subID == "" || subID == "<nil>" {
		subID = NewID("sub_rt")
	}
	sub := &RealtimeSubscription{
		ID:        subID,
		SessionID: env.SessionID(),
		ClientID:  env.ClientID(),
		Role:      env.Role(),
		LocalOnly: localOnly,
		CreatedAt: time.Now().UTC(),
		Streams:   map[string]RealtimeStreamSpec{},
	}
	for i := range streamSpecs {
		spec := streamSpecs[i]
		if spec.StreamID == "" {
			spec.StreamID = NewID("rt_stream")
		}
		if spec.Key == "" {
			spec.Key = realtimeStreamKey(spec.Stream, spec.TrackIDs)
		}
		sub.Streams[spec.StreamID] = spec
		streamSpecs[i] = spec
	}
	s.mu.Lock()
	s.subscriptions[sub.ID] = sub
	if localOnly {
		s.subscribeLocal++
	}
	s.mu.Unlock()
	return streamStatusPayload("subscribed", sub.ID, streamSpecs, localOnly)
}

func (s *RealtimeStore) RegisterSubscriptionFromStatus(env *Envelope, replyBytes []byte) {
	var reply map[string]any
	if err := json.Unmarshal(replyBytes, &reply); err != nil {
		return
	}
	if strings.TrimSpace(fmt.Sprint(reply["type"])) != "realtime.stream_status" {
		return
	}
	payload := mapFromAny(reply["payload"])
	subID := strings.TrimSpace(fmt.Sprint(payload["subscription_id"]))
	if subID == "" || subID == "<nil>" {
		return
	}
	streamSpecs := specsFromAny(payload["streams"])
	sub := &RealtimeSubscription{
		ID:        subID,
		SessionID: env.SessionID(),
		ClientID:  env.ClientID(),
		Role:      env.Role(),
		LocalOnly: false,
		CreatedAt: time.Now().UTC(),
		Streams:   map[string]RealtimeStreamSpec{},
	}
	for _, spec := range streamSpecs {
		if spec.StreamID == "" || spec.Stream == "" {
			continue
		}
		if spec.Key == "" {
			spec.Key = realtimeStreamKey(spec.Stream, spec.TrackIDs)
		}
		sub.Streams[spec.StreamID] = spec
	}
	if len(sub.Streams) == 0 {
		return
	}
	s.mu.Lock()
	s.subscriptions[sub.ID] = sub
	s.mu.Unlock()
}

func (s *RealtimeStore) Unsubscribe(subscriptionID string) (RealtimeSubscription, bool) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return RealtimeSubscription{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subscriptions[subscriptionID]
	if !ok || sub == nil {
		return RealtimeSubscription{}, false
	}
	delete(s.subscriptions, subscriptionID)
	return *sub, true
}

func (s *RealtimeStore) Subscription(subscriptionID string) (RealtimeSubscription, bool) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return RealtimeSubscription{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subscriptions[subscriptionID]
	if !ok || sub == nil {
		return RealtimeSubscription{}, false
	}
	return *sub, true
}

func (s *RealtimeStore) Publish(env *Envelope) []map[string]any {
	payload := env.Payload()
	rawFrames := framesFromPublishPayload(payload)
	if len(rawFrames) == 0 {
		return nil
	}
	now := time.Now().UTC()
	frames := make([]map[string]any, 0, len(rawFrames))
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, raw := range rawFrames {
		frame := normaliseRealtimeFrame(raw, now, s.published+1)
		if frame.Stream == "" {
			continue
		}
		s.latest[frame.Key] = frame
		if frame.Key != frame.Stream {
			s.latest[frame.Stream] = frame
		}
		s.published++
		frames = append(frames, cloneMap(frame.Payload))
	}
	return frames
}

func (s *RealtimeStore) FrameForRequest(env *Envelope) (map[string]any, bool) {
	payload := env.Payload()
	subID := strings.TrimSpace(fmt.Sprint(payload["subscription_id"]))
	streamID := strings.TrimSpace(fmt.Sprint(payload["stream_id"]))
	streamName := strings.TrimSpace(fmt.Sprint(payload["stream"]))
	if subID == "<nil>" {
		subID = ""
	}
	if streamID == "<nil>" {
		streamID = ""
	}
	if streamName == "<nil>" {
		streamName = ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := []string{}
	var matchedSpec RealtimeStreamSpec
	hasMatchedSpec := false
	requestedTrackIDs := firstStringSlice(
		stringSliceFromAny(payload["track_ids"]),
		stringSliceFromAny(payload["visible_track_ids"]),
	)
	if subID != "" && streamID != "" {
		if sub, ok := s.subscriptions[subID]; ok && sub != nil {
			if spec, ok := sub.Streams[streamID]; ok {
				keys = append(keys, spec.Key, spec.Stream)
				streamName = spec.Stream
				matchedSpec = spec
				hasMatchedSpec = true
				if len(spec.TrackIDs) > 0 {
					requestedTrackIDs = append([]string{}, spec.TrackIDs...)
				}
			}
		}
	}
	if streamName != "" {
		keys = append(keys, streamName)
	}
	for _, key := range keys {
		if frame, ok := s.latest[key]; ok && frame != nil {
			payload := cloneMap(frame.Payload)
			if hasMatchedSpec {
				payload = filterRealtimePayloadForSpec(payload, matchedSpec)
			} else if len(requestedTrackIDs) > 0 {
				payload = filterRealtimePayloadForSpec(payload, RealtimeStreamSpec{
					Stream:   firstNonEmpty(streamName, cleanString(payload["stream"])),
					TrackIDs: requestedTrackIDs,
				})
			}
			if streamID != "" {
				payload["stream_id"] = streamID
			}
			if subID != "" {
				payload["subscription_id"] = subID
			}
			if _, ok := payload["stream"]; !ok && streamName != "" {
				payload["stream"] = streamName
			}
			s.cacheServed++
			return payload, true
		}
	}
	s.cacheMisses++
	return nil, false
}

func (s *RealtimeStore) FramesForSessionBroadcast(sessionID string, frame map[string]any) []RealtimeDeliveryFrame {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || sessionID == "session_pending" {
		return nil
	}
	streamName := cleanString(frame["stream"])
	if streamName == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []RealtimeDeliveryFrame{}
	for _, sub := range s.subscriptions {
		if sub == nil || sub.SessionID != sessionID {
			continue
		}
		for _, spec := range sub.Streams {
			if spec.Stream != streamName {
				continue
			}
			payload := filterRealtimePayloadForSpec(cloneMap(frame), spec)
			if visibleRealtimePayloadEmptyForSpec(payload, spec) {
				continue
			}
			payload["subscription_id"] = sub.ID
			if spec.StreamID != "" {
				payload["stream_id"] = spec.StreamID
			}
			if _, ok := payload["stream"]; !ok {
				payload["stream"] = spec.Stream
			}
			payload["delivery"] = "hub_websocket"
			out = append(out, RealtimeDeliveryFrame{
				SessionID: sessionID,
				Payload:   payload,
				MaxHz:     spec.MaxHz,
				Key:       sub.ID + ":" + spec.StreamID,
			})
		}
	}
	return out
}

func visibleRealtimePayloadEmptyForSpec(payload map[string]any, spec RealtimeStreamSpec) bool {
	if len(spec.TrackIDs) == 0 {
		return false
	}
	stream := firstNonEmpty(spec.Stream, cleanString(payload["stream"]))
	if stream != "meters.visible_tracks" && stream != "spectrum.visible_tracks" {
		return false
	}
	data := mapFromAny(payload["data"])
	if len(data) == 0 {
		return false
	}
	if rows, ok := data["tracks"]; ok && len(arrayFromAny(rows)) == 0 {
		return true
	}
	if rows, ok := data["asset_refs"]; ok && len(arrayFromAny(rows)) == 0 {
		return true
	}
	return false
}

func filterRealtimePayloadForSpec(payload map[string]any, spec RealtimeStreamSpec) map[string]any {
	if len(spec.TrackIDs) == 0 {
		return payload
	}
	stream := firstNonEmpty(spec.Stream, cleanString(payload["stream"]))
	if stream != "meters.visible_tracks" && stream != "spectrum.visible_tracks" {
		return payload
	}
	data := mapFromAny(payload["data"])
	if len(data) == 0 {
		return payload
	}
	filteredAny := false
	filteredCount := 0
	filteredIDs := []string{}
	if _, ok := data["tracks"]; ok {
		rows, ids := filterRealtimeRowsByTrackIDs(data["tracks"], spec.TrackIDs)
		data["tracks"] = rows
		filteredAny = true
		filteredCount = len(rows)
		filteredIDs = ids
	}
	if _, ok := data["asset_refs"]; ok {
		rows, ids := filterRealtimeRowsByTrackIDs(data["asset_refs"], spec.TrackIDs)
		data["asset_refs"] = rows
		filteredAny = true
		filteredCount = len(rows)
		if len(filteredIDs) == 0 {
			filteredIDs = ids
		}
	}
	if !filteredAny {
		return payload
	}
	data["visible_track_count"] = filteredCount
	payload["data"] = data
	if len(filteredIDs) > 0 {
		payload["track_ids"] = filteredIDs
	} else {
		payload["track_ids"] = []string{}
	}
	payload["stream_key"] = realtimeStreamKey(stream, spec.TrackIDs)
	return payload
}

func filterRealtimeRowsByTrackIDs(value any, trackIDs []string) ([]any, []string) {
	rows := arrayFromAny(value)
	if len(rows) == 0 || len(trackIDs) == 0 {
		return []any{}, []string{}
	}
	byID := map[string]map[string]any{}
	for _, row := range rows {
		item := mapFromAny(row)
		id := firstNonEmpty(cleanString(item["track_id"]), cleanString(item["id"]))
		if id == "" {
			continue
		}
		if _, exists := byID[id]; !exists {
			byID[id] = item
		}
	}
	out := make([]any, 0, len(trackIDs))
	outIDs := make([]string, 0, len(trackIDs))
	seen := map[string]bool{}
	for _, rawID := range trackIDs {
		id := strings.TrimSpace(rawID)
		if id == "" || seen[id] {
			continue
		}
		item, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, item)
		outIDs = append(outIDs, id)
		seen[id] = true
	}
	return out, outIDs
}

func (s *RealtimeStore) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	streams := make([]map[string]any, 0, len(s.latest))
	keys := make([]string, 0, len(s.latest))
	for key := range s.latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		frame := s.latest[key]
		if frame == nil || key != frame.Key {
			continue
		}
		streams = append(streams, map[string]any{
			"stream":      frame.Stream,
			"key":         frame.Key,
			"frame_index": frame.FrameIndex,
			"updated_at":  frame.CreatedAt.Format(time.RFC3339Nano),
		})
	}
	return map[string]any{
		"published_frames":          s.published,
		"cache_served":              s.cacheServed,
		"cache_misses":              s.cacheMisses,
		"local_subscriptions":       s.subscribeLocal,
		"subscription_count":        len(s.subscriptions),
		"latest_stream_count":       len(streams),
		"latest_streams":            streams,
		"cache_first_frame_request": true,
	}
}

func (h *Hub) handleRealtimeLocal(env *Envelope, transport string) (HubResponse, bool) {
	switch env.Type() {
	case "realtime.publish":
		frames := h.realtime.Publish(env)
		for _, frame := range frames {
			h.broadcastRealtimeFrame(env, frame)
		}
		payload := map[string]any{
			"status":          "ok",
			"accepted_frames": len(frames),
			"delivery":        "hub_cache",
		}
		return JSONResponse(http.StatusOK, h.localReply(env, "vsp.realtime.publish_ack.v1", "realtime", "realtime.publish_ack", payload)), true
	case "realtime.subscribe":
		if !wantsHubCacheOnly(env.Payload()) {
			return HubResponse{}, false
		}
		payload := h.realtime.Subscribe(env, true)
		return JSONResponse(http.StatusOK, h.localReply(env, "vsp.realtime.stream_status.v1", "realtime", "realtime.stream_status", payload)), true
	case "realtime.frame_request":
		if payload, ok := h.realtime.FrameForRequest(env); ok {
			return JSONResponse(http.StatusOK, h.localReply(env, "vsp.realtime.frame.v1", "realtime", "realtime.frame", payload)), true
		}
		if wantsHubCacheOnly(env.Payload()) {
			return ErrorResponse(env, http.StatusNotFound, "realtime", "realtime.error", "realtime_unavailable", "no realtime frame is cached for this stream"), true
		}
		return HubResponse{}, false
	case "realtime.unsubscribe":
		payload := env.Payload()
		subID := strings.TrimSpace(fmt.Sprint(payload["subscription_id"]))
		sub, ok := h.realtime.Subscription(subID)
		if ok && sub.LocalOnly {
			h.realtime.Unsubscribe(subID)
			status := map[string]any{
				"status":          "unsubscribed",
				"subscription_id": subID,
				"delivery":        "hub_cache",
			}
			return JSONResponse(http.StatusOK, h.localReply(env, "vsp.realtime.stream_status.v1", "realtime", "realtime.stream_status", status)), true
		}
		return HubResponse{}, false
	default:
		return HubResponse{}, false
	}
}

func (h *Hub) broadcastRealtimeFrame(request *Envelope, frame map[string]any) {
	if h.streams.Count() == 0 {
		return
	}
	now := time.Now().UTC()
	for _, stream := range h.streams.Clients() {
		sessionID := stream.BoundSessionID()
		deliveries := h.realtime.FramesForSessionBroadcast(sessionID, frame)
		for _, delivery := range deliveries {
			if !stream.ShouldSendRealtime(delivery.Key, delivery.MaxHz, now) {
				continue
			}
			env := h.localReply(request, "vsp.realtime.frame.v1", "realtime", "realtime.frame", delivery.Payload)
			env["session_id"] = delivery.SessionID
			env["client_id"] = "vsp.hub"
			env["role"] = "hub"
			data, _ := json.Marshal(env)
			stream.Write(append(data, '\n'))
		}
	}
}

func specsFromAny(value any) []RealtimeStreamSpec {
	rows, ok := value.([]any)
	if !ok {
		data, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		_ = json.Unmarshal(data, &rows)
	}
	out := make([]RealtimeStreamSpec, 0, len(rows))
	for _, row := range rows {
		item := mapFromAny(row)
		stream := strings.TrimSpace(fmt.Sprint(item["stream"]))
		if stream == "" || stream == "<nil>" {
			continue
		}
		spec := RealtimeStreamSpec{
			Stream:       stream,
			StreamID:     cleanString(item["stream_id"]),
			TrackIDs:     stringSliceFromAny(item["track_ids"]),
			MaxHz:        intFromAny(item["max_hz"], 30),
			Mode:         cleanString(item["mode"]),
			VisibleRange: mapFromAny(item["visible_range"]),
		}
		if spec.Mode == "" {
			spec.Mode = "latest_only"
		}
		spec.Key = realtimeStreamKey(spec.Stream, spec.TrackIDs)
		out = append(out, spec)
	}
	return out
}

func framesFromPublishPayload(payload map[string]any) []map[string]any {
	if rows, ok := payload["frames"].([]any); ok {
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			frame := mapFromAny(row)
			if len(frame) > 0 {
				out = append(out, frame)
			}
		}
		return out
	}
	if frame := mapFromAny(payload["frame"]); len(frame) > 0 {
		return []map[string]any{frame}
	}
	return []map[string]any{cloneMap(payload)}
}

func normaliseRealtimeFrame(raw map[string]any, now time.Time, fallbackIndex int64) *RealtimeFrame {
	stream := cleanString(raw["stream"])
	if stream == "" {
		return &RealtimeFrame{}
	}
	trackIDs := stringSliceFromAny(raw["track_ids"])
	if len(trackIDs) == 0 {
		trackIDs = trackIDsFromFrameData(raw["data"])
	}
	key := cleanString(raw["stream_key"])
	if key == "" {
		key = realtimeStreamKey(stream, trackIDs)
	}
	frameIndex := int64FromAny(raw["frame_index"], fallbackIndex)
	payload := cloneMap(raw)
	payload["stream"] = stream
	payload["stream_key"] = key
	payload["frame_index"] = frameIndex
	if _, ok := payload["created_at"]; !ok {
		payload["created_at"] = now.Format(time.RFC3339Nano)
	}
	if _, ok := payload["data"]; !ok {
		payload["data"] = map[string]any{}
	}
	return &RealtimeFrame{
		Stream:     stream,
		Key:        key,
		FrameIndex: frameIndex,
		CreatedAt:  now,
		Payload:    payload,
	}
}

func realtimeStreamKey(stream string, trackIDs []string) string {
	stream = strings.TrimSpace(stream)
	if len(trackIDs) == 0 {
		return stream
	}
	cleaned := make([]string, 0, len(trackIDs))
	for _, id := range trackIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			cleaned = append(cleaned, id)
		}
	}
	if len(cleaned) == 0 {
		return stream
	}
	return stream + "|tracks:" + strings.Join(cleaned, ",")
}

func streamStatusPayload(status, subscriptionID string, specs []RealtimeStreamSpec, localOnly bool) map[string]any {
	streams := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		row := map[string]any{
			"stream":      spec.Stream,
			"stream_id":   spec.StreamID,
			"stream_key":  spec.Key,
			"mode":        spec.Mode,
			"max_hz":      spec.MaxHz,
			"latest_only": spec.Mode == "latest_only",
			"drop_old":    true,
			"hub_cached":  true,
		}
		if len(spec.TrackIDs) > 0 {
			row["track_ids"] = spec.TrackIDs
			row["visible_tracks_only"] = strings.Contains(spec.Stream, "visible_tracks")
		}
		if len(spec.VisibleRange) > 0 {
			row["visible_range"] = cloneMap(spec.VisibleRange)
		}
		streams = append(streams, row)
	}
	delivery := "kernel_shadow"
	if localOnly {
		delivery = "hub_cache"
	}
	return map[string]any{
		"status":          status,
		"subscription_id": subscriptionID,
		"streams":         streams,
		"delivery":        delivery,
	}
}

func wantsHubCacheOnly(payload map[string]any) bool {
	if value, ok := payload["hub_cache_only"].(bool); ok && value {
		return true
	}
	mode := strings.ToLower(cleanString(payload["hub_delivery"]))
	return mode == "hub_cache" || mode == "cache_only" || mode == "latest_cache"
}

func trackIDsFromFrameData(value any) []string {
	data := mapFromAny(value)
	rows, ok := data["tracks"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		track := mapFromAny(row)
		id := firstNonEmpty(cleanString(track["track_id"]), cleanString(track["id"]))
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func firstStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return append([]string{}, value...)
		}
	}
	return nil
}

func arrayFromAny(value any) []any {
	if rows, ok := value.([]any); ok {
		return rows
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out []any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func cleanString(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func intFromAny(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i)
		}
	}
	text := cleanString(value)
	if text == "" {
		return fallback
	}
	var out int
	if _, err := fmt.Sscanf(text, "%d", &out); err == nil {
		return out
	}
	return fallback
}

func int64FromAny(value any, fallback int64) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return i
		}
	}
	text := cleanString(value)
	if text == "" {
		return fallback
	}
	var out int64
	if _, err := fmt.Sscanf(text, "%d", &out); err == nil {
		return out
	}
	return fallback
}
