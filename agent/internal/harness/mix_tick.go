package harness

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	mixTickOpTrackGainAdjust = "track_gain_adjust"
	mixTickMaxAbsDeltaDB     = 2.0
	mixTickRollbackWindow    = 16
)

type mixTickRecord struct {
	TickID        string
	Operation     string
	TrackID       string
	TrackName     string
	ObservationID string
	Evidence      map[string]any
	DeltaDB       float64
	BeforeDB      *float64
	AfterDB       *float64
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type mixTickStore struct {
	mu    sync.RWMutex
	byID  map[string]*mixTickRecord
	order []string
}

func newMixTickStore() *mixTickStore {
	return &mixTickStore{byID: map[string]*mixTickRecord{}}
}

func (s *mixTickStore) put(rec *mixTickRecord) {
	if s == nil || rec == nil || strings.TrimSpace(rec.TickID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID == nil {
		s.byID = map[string]*mixTickRecord{}
	}
	now := time.Now()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	clone := cloneMixTickRecord(rec)
	s.byID[rec.TickID] = clone
	s.order = append(s.order, rec.TickID)
	if len(s.order) > mixTickRollbackWindow {
		trimmed := append([]string(nil), s.order[len(s.order)-mixTickRollbackWindow:]...)
		s.order = trimmed
	}
}

func (s *mixTickStore) get(id string) (*mixTickRecord, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.byID[strings.TrimSpace(id)]
	if !ok || rec == nil {
		return nil, false
	}
	return cloneMixTickRecord(rec), true
}

func (s *mixTickStore) latest() (*mixTickRecord, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.order) - 1; i >= 0; i-- {
		if rec, ok := s.byID[s.order[i]]; ok && rec != nil {
			return cloneMixTickRecord(rec), true
		}
	}
	return nil, false
}

func cloneMixTickRecord(in *mixTickRecord) *mixTickRecord {
	if in == nil {
		return nil
	}
	out := *in
	out.Evidence = cloneAnyMap(in.Evidence)
	return &out
}

func (h *Harness) proposeMixTick(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	_ = ctx
	if h != nil && h.mixTicks == nil {
		h.mixTicks = newMixTickStore()
	}
	op := strings.TrimSpace(firstString(cmd, "operation", "type"))
	if op == "" {
		op = mixTickOpTrackGainAdjust
	}
	if op != mixTickOpTrackGainAdjust {
		return map[string]any{"status": "error", "error": "mix_tick v1 only supports track_gain_adjust", "supported_operations": []string{mixTickOpTrackGainAdjust}}, nil
	}
	trackID := firstString(cmd, "track_id")
	if trackID == "" {
		return map[string]any{"status": "error", "error": "track_id is required"}, nil
	}
	deltaValue, hasDelta := cmd["delta_db"]
	if !hasDelta || isEmptyValue(deltaValue) {
		return map[string]any{"status": "error", "error": "delta_db is required for mix_tick proposal; do not use an implicit default for acoustic mix execution"}, nil
	}
	delta := numberFromAnyWithDefault(deltaValue, 0)
	if delta == 0 {
		return map[string]any{"status": "error", "error": "delta_db must be non-zero"}, nil
	}
	if math.Abs(delta) > mixTickMaxAbsDeltaDB {
		delta = math.Copysign(mixTickMaxAbsDeltaDB, delta)
	}
	state := h.UserStateSummary(context.Background())
	var currentDB float64
	found := false
	for _, row := range mapRowsFromAny(state["tracks"]) {
		if firstString(row, "track_id", "id") == trackID {
			currentDB = numberFromAnyWithDefault(firstPresentAny(row, "volume_db", "gain_db", "fader_db"), 0)
			found = true
			break
		}
	}
	if !found {
		return map[string]any{"status": "error", "error": "track not found in current project state", "track_id": trackID}, nil
	}
	projectPeak := numberFromAnyWithDefault(firstPresentAny(state, "master_peak_dbfs", "peak_dbfs"), -120)
	projectHeadroom := numberFromAnyWithDefault(firstPresentAny(state, "project_headroom_db", "headroom_db"), 12)
	proposed := clampFloat(currentDB+delta, -60, 12)
	tickID := "mix_tick_" + randomID()
	rec := &mixTickRecord{
		TickID:        tickID,
		Operation:     op,
		TrackID:       trackID,
		TrackName:     firstNonEmpty(firstString(firstTrackRow(state, trackID), "track_name", "name"), trackID),
		ObservationID: firstString(cmd, "observation_id"),
		Evidence:      cloneAnyMap(mapAnyFromAny(cmd["evidence"])),
		DeltaDB:       delta,
		BeforeDB:      &currentDB,
		AfterDB:       &proposed,
		Status:        "proposed",
	}
	if h.mixTicks != nil {
		h.mixTicks.put(rec)
	}
	result := map[string]any{
		"status":                "ok",
		"tick_id":               tickID,
		"operation":             op,
		"requires_confirmation": true,
		"track_id":              trackID,
		"track_name":            rec.TrackName,
		"delta_db":              delta,
		"before_db":             currentDB,
		"after_db":              proposed,
		"project_peak_dbfs":     projectPeak,
		"project_headroom_db":   projectHeadroom,
		"preview":               fmt.Sprintf("track_gain_adjust %s %+0.2f dB -> %+0.2f dB", trackID, delta, proposed),
	}
	return result, nil
}

func (h *Harness) applyMixTick(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return map[string]any{"status": "error", "error": "kernel client is required"}, nil
	}
	if !boolFromAnyDefault(cmd["confirmation"], false) && !boolFromAnyDefault(cmd["confirmed"], false) {
		return map[string]any{"status": "error", "error": "confirmation is required"}, nil
	}
	tickID := strings.TrimSpace(firstString(cmd, "tick_id"))
	if tickID == "" {
		return map[string]any{"status": "error", "error": "tick_id is required"}, nil
	}
	var rec *mixTickRecord
	var ok bool
	if tickID != "" && h.mixTicks != nil {
		rec, ok = h.mixTicks.get(tickID)
	}
	if !ok || rec == nil {
		return map[string]any{"status": "error", "error": "unknown mix tick", "tick_id": tickID}, nil
	}
	if rec.Status != "proposed" {
		if rec.Status == "applied" {
			return map[string]any{"status": "error", "error": "mix tick already applied", "tick_id": rec.TickID}, nil
		}
		return map[string]any{"status": "error", "error": "mix tick is not pending", "tick_id": rec.TickID, "tick_status": rec.Status}, nil
	}
	if rec.Operation != mixTickOpTrackGainAdjust {
		return map[string]any{"status": "error", "error": "unsupported mix tick operation", "tick_id": rec.TickID}, nil
	}
	before := 0.0
	if rec.BeforeDB != nil {
		before = *rec.BeforeDB
	}
	after := before + rec.DeltaDB
	if rec.AfterDB != nil {
		after = *rec.AfterDB
	}
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{
		"cmd":      "set_volume",
		"track_id": rec.TrackID,
		"db":       after,
	})
	if err != nil || !kernelReplySucceeded(reply) {
		return map[string]any{
			"status":   "error",
			"error":    mixTickKernelError(err, reply, "set_volume failed"),
			"tick_id":  rec.TickID,
			"track_id": rec.TrackID,
		}, err
	}
	if h.catalog != nil {
		if volumeSpec, ok := h.catalog.LookupCommand("set_volume"); ok {
			h.afterKernelReply(ctx, volumeSpec, reply)
		}
	}
	now := time.Now()
	rec.Status = "applied"
	rec.AfterDB = &after
	rec.UpdatedAt = now
	h.mixTicks.put(rec)
	return map[string]any{
		"status":           "ok",
		"tick_id":          rec.TickID,
		"operation":        rec.Operation,
		"track_id":         rec.TrackID,
		"track_name":       rec.TrackName,
		"before_db":        before,
		"after_db":         after,
		"kernel_command":   "set_volume",
		"kernel_reply":     reply,
		"requires_refresh": true,
	}, nil
}

func (h *Harness) rollbackMixTick(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return map[string]any{"status": "error", "error": "kernel client is required"}, nil
	}
	tickID := strings.TrimSpace(firstString(cmd, "tick_id"))
	var rec *mixTickRecord
	var ok bool
	if tickID != "" && h.mixTicks != nil {
		rec, ok = h.mixTicks.get(tickID)
	}
	if !ok && h.mixTicks != nil {
		rec, ok = h.mixTicks.latest()
	}
	if !ok || rec == nil {
		return map[string]any{"status": "error", "error": "unknown mix tick", "tick_id": tickID}, nil
	}
	if rec.BeforeDB == nil {
		return map[string]any{"status": "error", "error": "mix tick has no rollback state", "tick_id": rec.TickID}, nil
	}
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{
		"cmd":      "set_volume",
		"track_id": rec.TrackID,
		"db":       *rec.BeforeDB,
	})
	if err != nil || !kernelReplySucceeded(reply) {
		return map[string]any{
			"status":   "error",
			"error":    mixTickKernelError(err, reply, "rollback failed"),
			"tick_id":  rec.TickID,
			"track_id": rec.TrackID,
		}, err
	}
	if h.catalog != nil {
		if volumeSpec, ok := h.catalog.LookupCommand("set_volume"); ok {
			h.afterKernelReply(ctx, volumeSpec, reply)
		}
	}
	rec.Status = "rolled_back"
	rec.UpdatedAt = time.Now()
	if h.mixTicks != nil {
		h.mixTicks.put(rec)
	}
	return map[string]any{
		"status":           "ok",
		"tick_id":          rec.TickID,
		"operation":        rec.Operation,
		"track_id":         rec.TrackID,
		"track_name":       rec.TrackName,
		"restored_db":      *rec.BeforeDB,
		"kernel_command":   "set_volume",
		"kernel_reply":     reply,
		"requires_refresh": true,
	}, nil
}

func firstTrackRow(state map[string]any, trackID string) map[string]any {
	for _, row := range mapRowsFromAny(state["tracks"]) {
		if firstString(row, "track_id", "id") == trackID {
			return row
		}
	}
	return nil
}

func firstPresentAny(row map[string]any, keys ...string) any {
	if row == nil {
		return nil
	}
	for _, key := range keys {
		if value, ok := row[key]; ok && !isEmptyValue(value) {
			return value
		}
	}
	return nil
}

func mixTickKernelError(err error, reply map[string]any, fallback string) string {
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		return err.Error()
	}
	return firstNonEmpty(firstString(reply, "error", "message"), fallback)
}
