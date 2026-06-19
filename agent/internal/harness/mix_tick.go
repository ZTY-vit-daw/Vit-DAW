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
	mixTickOpTrackPanAdjust  = "track_pan_adjust"
	mixTickOpTrackPanSet     = "track_pan_set"
	mixTickMaxAbsDeltaDB     = 2.0
	mixTickMaxAbsDeltaPan    = 0.15
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
	DeltaPan      float64
	TargetPan     *float64
	BeforeDB      *float64
	AfterDB       *float64
	BeforePan     *float64
	AfterPan      *float64
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
	if !mixTickSupportedOperation(op) {
		return map[string]any{"status": "error", "error": "unsupported mix tick operation", "supported_operations": []string{mixTickOpTrackGainAdjust, mixTickOpTrackPanAdjust, mixTickOpTrackPanSet}}, nil
	}
	trackID := firstString(cmd, "track_id")
	if trackID == "" {
		return map[string]any{"status": "error", "error": "track_id is required"}, nil
	}
	state := h.UserStateSummary(context.Background())
	trackRow := firstTrackRow(state, trackID)
	if trackRow == nil {
		return map[string]any{"status": "error", "error": "track not found in current project state", "track_id": trackID}, nil
	}
	if op == mixTickOpTrackGainAdjust {
		return h.proposeTrackGainMixTick(cmd, state, trackRow, trackID, op)
	}
	return h.proposeTrackPanMixTick(cmd, state, trackRow, trackID, op)
}

func (h *Harness) proposeTrackGainMixTick(cmd map[string]any, state map[string]any, trackRow map[string]any, trackID, op string) (map[string]any, error) {
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
	currentDB := numberFromAnyWithDefault(firstPresentAny(trackRow, "volume_db", "gain_db", "fader_db"), 0)
	projectPeak := numberFromAnyWithDefault(firstPresentAny(state, "master_peak_dbfs", "peak_dbfs"), -120)
	projectHeadroom := numberFromAnyWithDefault(firstPresentAny(state, "project_headroom_db", "headroom_db"), 12)
	proposed := clampFloat(currentDB+delta, -60, 12)
	tickID := "mix_tick_" + randomID()
	rec := &mixTickRecord{
		TickID:        tickID,
		Operation:     op,
		TrackID:       trackID,
		TrackName:     firstNonEmpty(firstString(trackRow, "track_name", "name"), trackID),
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

func (h *Harness) proposeTrackPanMixTick(cmd map[string]any, state map[string]any, trackRow map[string]any, trackID, op string) (map[string]any, error) {
	currentPan := numberFromAnyWithDefault(firstPresentAny(trackRow, "pan", "pan_value", "balance"), 0)
	var proposed float64
	var deltaPan float64
	var targetPan *float64
	if op == mixTickOpTrackPanSet {
		targetValue, hasTarget := firstPresentAnyWithOK(cmd, "pan", "pan_value", "target_pan")
		if !hasTarget || isEmptyValue(targetValue) {
			return map[string]any{"status": "error", "error": "pan is required for track_pan_set proposal"}, nil
		}
		target := clampFloat(numberFromAnyWithDefault(targetValue, currentPan), -1, 1)
		targetPan = &target
		proposed = target
		deltaPan = proposed - currentPan
	} else {
		deltaValue, hasDelta := firstPresentAnyWithOK(cmd, "delta_pan", "pan_delta")
		if !hasDelta || isEmptyValue(deltaValue) {
			return map[string]any{"status": "error", "error": "delta_pan is required for track_pan_adjust proposal"}, nil
		}
		deltaPan = numberFromAnyWithDefault(deltaValue, 0)
		if deltaPan == 0 {
			return map[string]any{"status": "error", "error": "delta_pan must be non-zero"}, nil
		}
		if math.Abs(deltaPan) > mixTickMaxAbsDeltaPan {
			deltaPan = math.Copysign(mixTickMaxAbsDeltaPan, deltaPan)
		}
		proposed = clampFloat(currentPan+deltaPan, -1, 1)
	}
	tickID := "mix_tick_" + randomID()
	rec := &mixTickRecord{
		TickID:        tickID,
		Operation:     op,
		TrackID:       trackID,
		TrackName:     firstNonEmpty(firstString(trackRow, "track_name", "name"), trackID),
		ObservationID: firstString(cmd, "observation_id"),
		Evidence:      cloneAnyMap(mapAnyFromAny(cmd["evidence"])),
		DeltaPan:      deltaPan,
		TargetPan:     targetPan,
		BeforePan:     &currentPan,
		AfterPan:      &proposed,
		Status:        "proposed",
	}
	if h.mixTicks != nil {
		h.mixTicks.put(rec)
	}
	return map[string]any{
		"status":                "ok",
		"tick_id":               tickID,
		"operation":             op,
		"requires_confirmation": true,
		"track_id":              trackID,
		"track_name":            rec.TrackName,
		"delta_pan":             deltaPan,
		"target_pan":            targetPanValue(targetPan),
		"before_pan":            currentPan,
		"after_pan":             proposed,
		"preview":               fmt.Sprintf("%s %s pan %+0.2f -> %+0.2f", op, trackID, currentPan, proposed),
	}, nil
}

func mixTickSupportedOperation(op string) bool {
	switch strings.TrimSpace(op) {
	case mixTickOpTrackGainAdjust, mixTickOpTrackPanAdjust, mixTickOpTrackPanSet:
		return true
	default:
		return false
	}
}

func targetPanValue(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func firstPresentAnyWithOK(row map[string]any, keys ...string) (any, bool) {
	if row == nil {
		return nil, false
	}
	for _, key := range keys {
		if value, ok := row[key]; ok && !isEmptyValue(value) {
			return value, true
		}
	}
	return nil, false
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
	switch rec.Operation {
	case mixTickOpTrackGainAdjust:
		return h.applyTrackGainMixTick(ctx, rec)
	case mixTickOpTrackPanAdjust, mixTickOpTrackPanSet:
		return h.applyTrackPanMixTick(ctx, rec)
	default:
		return map[string]any{"status": "error", "error": "unsupported mix tick operation", "tick_id": rec.TickID}, nil
	}
}

func (h *Harness) applyTrackGainMixTick(ctx context.Context, rec *mixTickRecord) (map[string]any, error) {
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

func (h *Harness) applyTrackPanMixTick(ctx context.Context, rec *mixTickRecord) (map[string]any, error) {
	before := 0.0
	if rec.BeforePan != nil {
		before = *rec.BeforePan
	}
	after := clampFloat(before+rec.DeltaPan, -1, 1)
	if rec.AfterPan != nil {
		after = *rec.AfterPan
	}
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{
		"cmd":      "set_pan",
		"track_id": rec.TrackID,
		"pan":      after,
	})
	if err != nil || !kernelReplySucceeded(reply) {
		return map[string]any{
			"status":   "error",
			"error":    mixTickKernelError(err, reply, "set_pan failed"),
			"tick_id":  rec.TickID,
			"track_id": rec.TrackID,
		}, err
	}
	if h.catalog != nil {
		if panSpec, ok := h.catalog.LookupCommand("set_pan"); ok {
			h.afterKernelReply(ctx, panSpec, reply)
		}
	}
	observed := h.observedMixTickTrackRow(ctx, rec.TrackID)
	observedPan, observedPanOK := observedTrackPan(observed)
	now := time.Now()
	rec.Status = "applied"
	rec.AfterPan = &after
	rec.UpdatedAt = now
	h.mixTicks.put(rec)
	result := map[string]any{
		"status":           "ok",
		"tick_id":          rec.TickID,
		"operation":        rec.Operation,
		"track_id":         rec.TrackID,
		"track_name":       rec.TrackName,
		"before_pan":       before,
		"after_pan":        after,
		"delta_pan":        after - before,
		"kernel_command":   "set_pan",
		"kernel_reply":     reply,
		"requires_refresh": true,
	}
	if observed != nil {
		result["observed_track"] = observed
	}
	if observedPanOK {
		result["observed_pan"] = observedPan
		result["observed_pan_matches"] = math.Abs(observedPan-after) <= 0.0001
	}
	return result, nil
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
	switch rec.Operation {
	case mixTickOpTrackGainAdjust:
		return h.rollbackTrackGainMixTick(ctx, rec)
	case mixTickOpTrackPanAdjust, mixTickOpTrackPanSet:
		return h.rollbackTrackPanMixTick(ctx, rec)
	default:
		return map[string]any{"status": "error", "error": "unsupported mix tick operation", "tick_id": rec.TickID}, nil
	}
}

func (h *Harness) rollbackTrackGainMixTick(ctx context.Context, rec *mixTickRecord) (map[string]any, error) {
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

func (h *Harness) rollbackTrackPanMixTick(ctx context.Context, rec *mixTickRecord) (map[string]any, error) {
	if rec.BeforePan == nil {
		return map[string]any{"status": "error", "error": "mix tick has no rollback state", "tick_id": rec.TickID}, nil
	}
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{
		"cmd":      "set_pan",
		"track_id": rec.TrackID,
		"pan":      *rec.BeforePan,
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
		if panSpec, ok := h.catalog.LookupCommand("set_pan"); ok {
			h.afterKernelReply(ctx, panSpec, reply)
		}
	}
	rec.Status = "rolled_back"
	rec.UpdatedAt = time.Now()
	if h.mixTicks != nil {
		h.mixTicks.put(rec)
	}
	observed := h.observedMixTickTrackRow(ctx, rec.TrackID)
	observedPan, observedPanOK := observedTrackPan(observed)
	result := map[string]any{
		"status":           "ok",
		"tick_id":          rec.TickID,
		"operation":        rec.Operation,
		"track_id":         rec.TrackID,
		"track_name":       rec.TrackName,
		"restored_pan":     *rec.BeforePan,
		"kernel_command":   "set_pan",
		"kernel_reply":     reply,
		"requires_refresh": true,
	}
	if observed != nil {
		result["observed_track"] = observed
	}
	if observedPanOK {
		result["observed_pan"] = observedPan
		result["observed_pan_matches"] = math.Abs(observedPan-*rec.BeforePan) <= 0.0001
	}
	return result, nil
}

func firstTrackRow(state map[string]any, trackID string) map[string]any {
	for _, row := range mapRowsFromAny(state["tracks"]) {
		if firstString(row, "track_id", "id") == trackID {
			return row
		}
	}
	return nil
}

func (h *Harness) observedMixTickTrackRow(ctx context.Context, trackID string) map[string]any {
	if h == nil {
		return nil
	}
	if h.kernel != nil && h.shadow != nil {
		reply, _, err := h.kernel.SendCommand(ctx, map[string]any{"cmd": "get_project_state"})
		if err == nil && kernelReplySucceeded(reply) {
			h.shadow.Initialize(reply)
			if row := firstTrackRow(h.UserStateSummary(ctx), trackID); row != nil {
				return row
			}
			if row := firstTrackRow(reply, trackID); row != nil {
				return row
			}
		}
	}
	return firstTrackRow(h.UserStateSummary(ctx), trackID)
}

func observedTrackPan(row map[string]any) (float64, bool) {
	value := firstPresentAny(row, "pan", "pan_value", "balance")
	if value == nil {
		return 0, false
	}
	return numberFromAnyWithDefault(value, 0), true
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
