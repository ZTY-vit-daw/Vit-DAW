package vsphub

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type EventStore struct {
	mu            sync.RWMutex
	subscriptions map[string]*EventSubscription
	recent        []CachedTelemetryEvent
	nextSeq       int64
	replayed      int64
}

type EventSubscription struct {
	ID        string
	SessionID string
	ClientID  string
	Role      string
	CreatedAt time.Time
	Topics    []string
}

type CachedTelemetryEvent struct {
	Seq       int64
	CreatedAt time.Time
	Telemetry any
}

const (
	eventReplayMax = 2048
	eventReplayTTL = 2 * time.Minute
)

func NewEventStore() *EventStore {
	return &EventStore{subscriptions: map[string]*EventSubscription{}}
}

func (s *EventStore) RegisterSubscriptionFromReply(env *Envelope, replyBytes []byte) {
	if s == nil || env == nil {
		return
	}
	var reply map[string]any
	if err := json.Unmarshal(replyBytes, &reply); err != nil {
		return
	}
	replyType := cleanString(reply["type"])
	if replyType != "event.notification" && replyType != "event.progress" {
		return
	}
	payload := mapFromAny(reply["payload"])
	status := strings.ToLower(cleanString(payload["status"]))
	if status == "error" || status == "failed" {
		return
	}
	requestPayload := env.Payload()
	subID := firstNonEmpty(
		cleanString(payload["subscription_id"]),
		cleanString(requestPayload["subscription_id"]),
		NewID("sub_evt"),
	)
	sub := &EventSubscription{
		ID:        subID,
		SessionID: env.SessionID(),
		ClientID:  env.ClientID(),
		Role:      env.Role(),
		CreatedAt: time.Now().UTC(),
		Topics: firstStringSlice(
			stringSliceFromAny(payload["topics"]),
			stringSliceFromAny(requestPayload["topics"]),
		),
	}
	if sub.SessionID == "" || sub.SessionID == "session_pending" {
		return
	}
	s.mu.Lock()
	s.subscriptions[sub.ID] = sub
	s.mu.Unlock()
}

func (s *EventStore) ShouldDeliverTelemetry(sessionID string, telemetry any) bool {
	sessionID = strings.TrimSpace(sessionID)
	if s == nil || sessionID == "" || sessionID == "session_pending" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sub := range s.subscriptions {
		if sub == nil || sub.SessionID != sessionID {
			continue
		}
		if eventTopicsMatch(sub.Topics, telemetry) {
			return true
		}
	}
	return false
}

func (s *EventStore) RecordTelemetry(telemetry any) {
	if s == nil || !shouldCacheTelemetryEvent(telemetry) {
		return
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneRecentLocked(now)
	s.nextSeq++
	s.recent = append(s.recent, CachedTelemetryEvent{
		Seq:       s.nextSeq,
		CreatedAt: now,
		Telemetry: telemetry,
	})
	if len(s.recent) > eventReplayMax {
		s.recent = append([]CachedTelemetryEvent(nil), s.recent[len(s.recent)-eventReplayMax:]...)
	}
}

func (s *EventStore) CachedTelemetryForSession(sessionID string, limit int) []CachedTelemetryEvent {
	sessionID = strings.TrimSpace(sessionID)
	if s == nil || sessionID == "" || sessionID == "session_pending" {
		return nil
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneRecentLocked(now)
	out := make([]CachedTelemetryEvent, 0, len(s.recent))
	for _, cached := range s.recent {
		if !s.hasMatchingSubscriptionLocked(sessionID, cached.Telemetry) {
			continue
		}
		out = append(out, cached)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	s.replayed += int64(len(out))
	return out
}

func (s *EventStore) Snapshot() map[string]any {
	if s == nil {
		return map[string]any{"subscription_count": 0}
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneRecentLocked(now)
	return map[string]any{
		"subscription_count":              len(s.subscriptions),
		"telemetry_requires_subscription": true,
		"cached_replay_events":            len(s.recent),
		"replayed_events":                 s.replayed,
	}
}

func (s *EventStore) hasMatchingSubscriptionLocked(sessionID string, telemetry any) bool {
	for _, sub := range s.subscriptions {
		if sub == nil || sub.SessionID != sessionID {
			continue
		}
		if eventTopicsMatch(sub.Topics, telemetry) {
			return true
		}
	}
	return false
}

func (s *EventStore) pruneRecentLocked(now time.Time) {
	if len(s.recent) == 0 {
		return
	}
	cutoff := now.Add(-eventReplayTTL)
	first := 0
	for first < len(s.recent) && s.recent[first].CreatedAt.Before(cutoff) {
		first++
	}
	if first > 0 {
		s.recent = append([]CachedTelemetryEvent(nil), s.recent[first:]...)
	}
}

func eventTopicsMatch(topics []string, telemetry any) bool {
	if len(topics) == 0 {
		return true
	}
	labels := telemetryTopicLabels(telemetry)
	for _, rawTopic := range topics {
		topic := strings.ToLower(strings.TrimSpace(rawTopic))
		if topic == "" || topic == "*" || topic == "telemetry" || topic == "kernel.telemetry" {
			return true
		}
		for _, label := range labels {
			if label == topic || strings.HasPrefix(label, topic+".") || strings.HasPrefix(label, topic+"_") {
				return true
			}
		}
	}
	return false
}

func telemetryTopicLabels(telemetry any) []string {
	item := mapFromAny(telemetry)
	if len(item) == 0 {
		return []string{"telemetry"}
	}
	out := []string{}
	for _, key := range []string{"topic", "subtopic", "type", "command", "cmd", "action"} {
		value := strings.ToLower(cleanString(item[key]))
		if value != "" {
			out = append(out, value)
		}
	}
	topic := strings.ToLower(cleanString(item["topic"]))
	subtopic := strings.ToLower(cleanString(item["subtopic"]))
	if topic != "" && subtopic != "" {
		out = append(out, topic+"."+subtopic, topic+"_"+subtopic)
	}
	if len(out) == 0 {
		out = append(out, strings.ToLower(strings.TrimSpace(fmt.Sprint(telemetry))))
	}
	cleaned := make([]string, 0, len(out))
	seen := map[string]bool{}
	for _, label := range out {
		label = strings.TrimSpace(label)
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		cleaned = append(cleaned, label)
	}
	if len(cleaned) == 0 {
		return []string{"telemetry"}
	}
	return cleaned
}

func shouldCacheTelemetryEvent(telemetry any) bool {
	for _, label := range telemetryTopicLabels(telemetry) {
		switch label {
		case "audio_feature_data_ready", "tile_ready", "track_duration_ready":
			return true
		}
	}
	return false
}
