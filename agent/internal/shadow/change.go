package shadow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ProjectChangeReceiptSchema = "project_change_receipt.v1"
	changeHistoryLimit         = 24

	ChangeSourceAuthoritativeSnapshot = "authoritative_snapshot"
	ChangeSourceTelemetryDelta        = "telemetry_delta"
	ChangeSourceExecutorDelta         = "executor_delta"
)

// ChangeReceipt records a deterministic state transition observed by the
// Shadow Project. It reports engineering changes only; it never claims an
// acoustic consequence.
type ChangeReceipt struct {
	SchemaVersion          string         `json:"schema_version"`
	ChangeID               string         `json:"change_id"`
	Source                 string         `json:"source"`
	ObservedAt             string         `json:"observed_at"`
	Authoritative          bool           `json:"authoritative"`
	Freshness              string         `json:"freshness"`
	FromStateEpoch         int64          `json:"from_state_epoch"`
	ToStateEpoch           int64          `json:"to_state_epoch"`
	FromProject            map[string]any `json:"from_project,omitempty"`
	ToProject              map[string]any `json:"to_project,omitempty"`
	ChangedEntities        []ChangeEntity `json:"changed_entities,omitempty"`
	AffectedScopes         []string       `json:"affected_scopes,omitempty"`
	UnresolvedRefreshScope []string       `json:"unresolved_refresh_scopes,omitempty"`
}

type ChangeEntity struct {
	Kind   string        `json:"kind"`
	ID     string        `json:"id"`
	Fields []ChangeField `json:"fields,omitempty"`
}

type ChangeField struct {
	Path   string `json:"path"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

type comparableProjectState struct {
	project  map[string]any
	entities map[string]map[string]any
}

func comparableProjectIdentityChanged(before, after map[string]any) bool {
	beforeID := strings.TrimSpace(fmt.Sprint(before["project_uuid"]))
	afterID := strings.TrimSpace(fmt.Sprint(after["project_uuid"]))
	return beforeID != "" && beforeID != "<nil>" && afterID != "" && afterID != "<nil>" && beforeID != afterID
}

func comparableProjectStateFromShadow(state map[string]any) comparableProjectState {
	engine, _ := state["engine_snapshot"].(map[string]any)
	nodes, _ := state["nodes_by_uid"].(map[string]any)
	project, _ := engine["project"].(map[string]any)
	identity := map[string]any{
		"project_uuid":     firstPresent(engine, "project_uuid", "project_id"),
		"project_epoch":    firstPresent(engine, "project_epoch"),
		"project_revision": firstPresent(engine, "project_revision", "revision"),
		"snapshot_hash":    firstPresent(engine, "snapshot_hash", "project_state_hash"),
		"graph_revision":   engine["graph_revision"],
	}
	for key, value := range map[string]any{
		"project_uuid":     firstPresent(project, "project_uuid", "project_id"),
		"project_epoch":    project["project_epoch"],
		"project_revision": firstPresent(project, "project_revision", "revision"),
		"snapshot_hash":    firstPresent(project, "snapshot_hash", "project_state_hash"),
	} {
		if isEmptyValue(identity[key]) && !isEmptyValue(value) {
			identity[key] = value
		}
	}
	removeEmptyChangeFields(identity)

	entities := map[string]map[string]any{}
	for _, raw := range asArray(engine["tracks"]) {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := trackIDFromRow(row)
		if id == "" || id == "<nil>" {
			continue
		}
		merged := trackRowWithDeltaProperties(row, nodes)
		entities["track:"+id] = comparableTrackState(id, merged)
	}
	return comparableProjectState{project: identity, entities: entities}
}

func comparableTrackState(id string, row map[string]any) map[string]any {
	return map[string]any{
		"kind":                 "track",
		"id":                   id,
		"name":                 firstPresent(row, "track_name", "name"),
		"track_state_revision": row["track_state_revision"],
		"gain_db":              firstPresent(row, "gain_db", "volume_db", "fader_db", "gainDb", "volumeDb", "faderDb"),
		"pan":                  firstPresent(row, "pan", "pan_value", "panValue", "balance"),
		"mute":                 firstPresent(row, "mute", "muted"),
		"solo":                 firstPresent(row, "solo", "is_solo"),
		"armed":                firstPresent(row, "is_armed", "armed", "record_armed"),
		"plugin_chain":         changeCollectionDigest(firstPresent(row, "plugins", "rack_nodes", "rack")),
		"clips":                changeCollectionDigest(firstPresent(row, "clips", "clip_summaries")),
	}
}

func changeCollectionDigest(value any) map[string]any {
	rows := asArray(value)
	if len(rows) == 0 {
		return map[string]any{"count": 0}
	}
	return map[string]any{"count": len(rows), "fingerprint": stableChangeValue(value)}
}

func diffProjectChange(before, after comparableProjectState) ([]ChangeEntity, []string) {
	entities := make([]ChangeEntity, 0)
	scopes := map[string]bool{}
	if fields := diffChangeMap(before.project, after.project, []string{"project_uuid", "project_epoch", "project_revision", "snapshot_hash", "graph_revision"}); len(fields) > 0 {
		entities = append(entities, ChangeEntity{Kind: "project", ID: "current", Fields: fields})
		for _, field := range fields {
			addChangeScopes(scopes, "project."+field.Path)
		}
	}

	keys := map[string]bool{}
	for key := range before.entities {
		keys[key] = true
	}
	for key := range after.entities {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	for _, key := range ordered {
		beforeEntity, beforeOK := before.entities[key]
		afterEntity, afterOK := after.entities[key]
		kind, id := changeEntityIdentity(beforeEntity, afterEntity)
		var fields []ChangeField
		switch {
		case !beforeOK:
			fields = []ChangeField{{Path: "lifecycle", After: "added"}}
		case !afterOK:
			fields = []ChangeField{{Path: "lifecycle", Before: "present", After: "removed"}}
		default:
			fields = diffChangeMap(beforeEntity, afterEntity, []string{"name", "track_state_revision", "gain_db", "pan", "mute", "solo", "armed", "plugin_chain", "clips"})
		}
		if len(fields) == 0 {
			continue
		}
		entities = append(entities, ChangeEntity{Kind: kind, ID: id, Fields: fields})
		for _, field := range fields {
			addChangeScopes(scopes, kind+"."+field.Path)
		}
	}
	return entities, sortedChangeScopes(scopes)
}

func changeEntityIdentity(before, after map[string]any) (string, string) {
	entity := after
	if len(entity) == 0 {
		entity = before
	}
	return strings.TrimSpace(fmt.Sprint(entity["kind"])), strings.TrimSpace(fmt.Sprint(entity["id"]))
}

func diffChangeMap(before, after map[string]any, keys []string) []ChangeField {
	fields := make([]ChangeField, 0)
	for _, key := range keys {
		beforeValue := before[key]
		afterValue := after[key]
		if stableChangeValue(beforeValue) == stableChangeValue(afterValue) {
			continue
		}
		fields = append(fields, ChangeField{Path: key, Before: cloneAny(beforeValue), After: cloneAny(afterValue)})
	}
	return fields
}

func stableChangeValue(value any) string {
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return fmt.Sprintf("%#v", value)
}

func addChangeScopes(scopes map[string]bool, path string) {
	path = strings.ToLower(strings.TrimSpace(path))
	switch {
	case strings.Contains(path, "gain_db"), strings.Contains(path, "level"), strings.Contains(path, "mute"), strings.Contains(path, "solo"):
		for _, scope := range []string{"track.level", "mix.multitrack_relationship", "mix.frequency_relationship", "mix.masking_relationship", "project.headroom"} {
			scopes[scope] = true
		}
	case strings.Contains(path, "pan"):
		for _, scope := range []string{"track.stereo_space", "mix.multitrack_relationship", "mix.masking_relationship"} {
			scopes[scope] = true
		}
	case strings.Contains(path, "plugin_chain"):
		for _, scope := range []string{"processor.identity_and_controls", "processor.behavior", "processor.change_delta", "comparison.before_after", "mix.multitrack_relationship", "mix.frequency_relationship", "mix.masking_relationship"} {
			scopes[scope] = true
		}
	case strings.Contains(path, "clips"), strings.Contains(path, "lifecycle"):
		for _, scope := range []string{"project.structure", "track.basic_energy", "track.time_dynamics", "track.timbre_frequency", "track.stereo_space", "mix.multitrack_relationship", "mix.frequency_relationship", "mix.masking_relationship"} {
			scopes[scope] = true
		}
	default:
		scopes["project.state"] = true
	}
}

func sortedChangeScopes(scopes map[string]bool) []string {
	out := make([]string, 0, len(scopes))
	for scope := range scopes {
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}

func removeEmptyChangeFields(values map[string]any) {
	for key, value := range values {
		if isEmptyValue(value) {
			delete(values, key)
		}
	}
}

func normalizeChangeSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ChangeSourceTelemetryDelta
	}
	return source
}

func (p *Project) recordChangeLocked(source string, authoritative bool, beforeEpoch, afterEpoch int64, before, after comparableProjectState, changes []ChangeEntity, scopes []string) {
	if len(changes) == 0 {
		return
	}
	p.changeSequence++
	freshness := "pending_authoritative_refresh"
	if authoritative {
		freshness = "current_snapshot"
	}
	receipt := ChangeReceipt{
		SchemaVersion:          ProjectChangeReceiptSchema,
		ChangeID:               fmt.Sprintf("shadow_change_%06d", p.changeSequence),
		Source:                 normalizeChangeSource(source),
		ObservedAt:             time.Now().UTC().Format(time.RFC3339Nano),
		Authoritative:          authoritative,
		Freshness:              freshness,
		FromStateEpoch:         beforeEpoch,
		ToStateEpoch:           afterEpoch,
		FromProject:            clone(before.project),
		ToProject:              clone(after.project),
		ChangedEntities:        changes,
		AffectedScopes:         scopes,
		UnresolvedRefreshScope: append([]string(nil), scopes...),
	}
	p.lastChange = receipt
	p.changeHistory = append(p.changeHistory, receipt)
	if len(p.changeHistory) > changeHistoryLimit {
		p.changeHistory = append([]ChangeReceipt(nil), p.changeHistory[len(p.changeHistory)-changeHistoryLimit:]...)
	}
}

// LatestChangeReceipt returns the latest bounded, deterministic project
// change record for use by Mix Board and context assembly.
func (p *Project) LatestChangeReceipt() map[string]any {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.lastChange.ChangeID == "" {
		return nil
	}
	value, _ := cloneAny(p.lastChange).(map[string]any)
	return value
}

// ChangeWindow returns the newest receipts first. History is kept outside the
// model hot context and remains bounded in the Shadow Project.
func (p *Project) ChangeWindow(limit int) []map[string]any {
	if p == nil || limit == 0 {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if limit < 0 || limit > len(p.changeHistory) {
		limit = len(p.changeHistory)
	}
	out := make([]map[string]any, 0, limit)
	for index := len(p.changeHistory) - 1; index >= 0 && len(out) < limit; index-- {
		value, _ := cloneAny(p.changeHistory[index]).(map[string]any)
		out = append(out, value)
	}
	return out
}
