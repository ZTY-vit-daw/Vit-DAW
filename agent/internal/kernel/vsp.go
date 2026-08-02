package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

const (
	vspVersion = "1.0"
	vspClient  = "agent.main"
	vspRole    = "agent"
)

var vspIDCounter atomic.Uint64

type VSPCommandResult struct {
	Response      map[string]any
	Raw           string
	Payload       map[string]any
	LegacyReply   map[string]any
	Command       string
	LegacyCommand string
	TransactionID string
	Revision      int64
	ResyncHint    bool
}

type VSPStateResult struct {
	Response      map[string]any
	Raw           string
	Payload       map[string]any
	LegacyState   map[string]any
	Revision      int64
	BaseRevision  int64
	ProjectEpoch  string
	SnapshotHash  string
	Scope         string
	Resync        bool
	ResyncHint    bool
	Ops           []any
	ChangedTracks []any
	ChangedClips  []any
}

func (c *Client) VSPHello(ctx context.Context) (map[string]any, string, error) {
	if c == nil {
		return nil, "", fmt.Errorf("kernel client is nil")
	}
	req := c.vspEnvelope("session_pending", "session", "session.hello", "vsp.session.hello.v1", map[string]any{
		"client_name":        "Vit Agent",
		"client_version":     "phase5-agent-vsp",
		"protocol_min":       vspVersion,
		"protocol_max":       vspVersion,
		"wants":              []string{"command.request", "state.snapshot", "state.delta", "state.resync", "event.progress"},
		"transport_bindings": []string{"legacy.zmq_reqrep"},
	}, "", "")
	reply, raw, err := c.SendCommand(ctx, req)
	if err != nil {
		return reply, raw, err
	}
	if !strings.EqualFold(stringFromAny(reply["type"]), "session.hello_ack") {
		return reply, raw, fmt.Errorf("vsp hello rejected: %s", firstNonEmptyString(stringFromAny(reply["type"]), stringFromAny(reply["message"]), stringFromAny(reply["error"]), "unexpected response"))
	}
	sessionID := stringFromAny(reply["session_id"])
	if sessionID == "" {
		return reply, raw, fmt.Errorf("vsp hello did not return session_id")
	}
	c.mu.Lock()
	c.sessionID = sessionID
	c.featureFlags = boolMapFromAny(reply["feature_flags"])
	c.mu.Unlock()
	return reply, raw, nil
}

// VSPFeature performs capability negotiation before callers upgrade a bounded
// adapter snapshot to a strong ProjectCut guarantee.
func (c *Client) VSPFeature(ctx context.Context, name string) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("kernel client is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("feature name is required")
	}
	if _, err := c.VSPSessionID(ctx); err != nil {
		return false, err
	}
	c.mu.Lock()
	enabled := c.featureFlags[name]
	c.mu.Unlock()
	return enabled, nil
}

func (c *Client) VSPSessionID(ctx context.Context) (string, error) {
	if c == nil {
		return "", fmt.Errorf("kernel client is nil")
	}
	c.mu.Lock()
	sessionID := c.sessionID
	c.mu.Unlock()
	if sessionID != "" {
		return sessionID, nil
	}
	reply, _, err := c.VSPHello(ctx)
	if err != nil {
		return "", err
	}
	sessionID = stringFromAny(reply["session_id"])
	if sessionID == "" {
		return "", fmt.Errorf("vsp hello did not return session_id")
	}
	return sessionID, nil
}

func (c *Client) SendVSPCommand(ctx context.Context, command string, args map[string]any) (*VSPCommandResult, error) {
	return c.SendVSPCommandWithIDs(ctx, command, args, "", "")
}

// SendVSPCommandWithIDs is the typed-command counterpart to
// SendVSPLegacyCommandWithIDs. Callers that implement a compensating
// transaction need a stable request and transaction identity for every
// physical write and its rollback.
func (c *Client) SendVSPCommandWithIDs(ctx context.Context, command string, args map[string]any, requestID, transactionID string) (*VSPCommandResult, error) {
	sessionID, err := c.VSPSessionID(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"command": strings.TrimSpace(command),
		"args":    cloneMap(args),
	}
	return c.sendVSPCommandPayloadWithIDs(ctx, sessionID, payload, requestID, transactionID)
}

func (c *Client) SendVSPLegacyCommand(ctx context.Context, cmd map[string]any) (*VSPCommandResult, error) {
	return c.SendVSPLegacyCommandWithIDs(ctx, cmd, "", "")
}

func (c *Client) SendVSPLegacyCommandWithIDs(ctx context.Context, cmd map[string]any, requestID, transactionID string) (*VSPCommandResult, error) {
	sessionID, err := c.VSPSessionID(ctx)
	if err != nil {
		return nil, err
	}
	legacy := cloneMap(cmd)
	legacyCommand := strings.TrimSpace(stringFromAny(legacy["cmd"]))
	if legacyCommand == "" {
		return nil, fmt.Errorf("legacy command requires cmd")
	}
	args := cloneMap(legacy)
	delete(args, "cmd")
	payload := map[string]any{
		"command": "legacy.command",
		"legacy": map[string]any{
			"cmd":  legacyCommand,
			"args": args,
		},
	}
	return c.sendVSPCommandPayloadWithIDs(ctx, sessionID, payload, requestID, transactionID)
}

func (c *Client) VSPStateSnapshot(ctx context.Context, scope string) (*VSPStateResult, error) {
	return c.sendVSPState(ctx, "state.snapshot_request", "vsp.state.snapshot_request.v1", scope, 0)
}

func (c *Client) VSPStateDelta(ctx context.Context, baseRevision int64, scope string) (*VSPStateResult, error) {
	if baseRevision <= 0 {
		return nil, fmt.Errorf("base_revision is required")
	}
	return c.sendVSPState(ctx, "state.delta_request", "vsp.state.delta_request.v1", scope, baseRevision)
}

func (c *Client) VSPStateResync(ctx context.Context, scope string) (*VSPStateResult, error) {
	return c.sendVSPState(ctx, "state.resync_request", "vsp.state.resync_request.v1", scope, 0)
}

func (c *Client) sendVSPCommandPayload(ctx context.Context, sessionID string, payload map[string]any) (*VSPCommandResult, error) {
	return c.sendVSPCommandPayloadWithIDs(ctx, sessionID, payload, "", "")
}

func (c *Client) sendVSPCommandPayloadWithIDs(ctx context.Context, sessionID string, payload map[string]any, requestID, transactionID string) (*VSPCommandResult, error) {
	if strings.TrimSpace(requestID) == "" {
		requestID = newVSPID("req")
	}
	if strings.TrimSpace(transactionID) == "" {
		transactionID = newVSPID("tx")
	}
	req := c.vspEnvelope(sessionID, "command", "command.request", "vsp.command.request.v1", payload, requestID, transactionID)
	reply, raw, err := c.SendCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	return parseVSPCommandResult(reply, raw), nil
}

func (c *Client) sendVSPState(ctx context.Context, messageType, schema, scope string, baseRevision int64) (*VSPStateResult, error) {
	sessionID, err := c.VSPSessionID(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if strings.TrimSpace(scope) != "" {
		payload["scope"] = strings.TrimSpace(scope)
	}
	if baseRevision > 0 {
		payload["base_revision"] = baseRevision
	}
	req := c.vspEnvelope(sessionID, "state", messageType, schema, payload, newVSPID("req"), "")
	if baseRevision > 0 {
		req["base_revision"] = baseRevision
	}
	reply, raw, err := c.SendCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	return parseVSPStateResult(reply, raw), nil
}

func (c *Client) vspEnvelope(sessionID, channel, messageType, schema string, payload map[string]any, requestID, transactionID string) map[string]any {
	msg := map[string]any{
		"vsp_version":        vspVersion,
		"schema":             schema,
		"message_id":         newVSPID("msg"),
		"session_id":         sessionID,
		"client_id":          vspClient,
		"role":               vspRole,
		"channel":            channel,
		"type":               messageType,
		"created_at":         time.Now().UTC().Format(time.RFC3339),
		"trace_id":           newVSPID("trace"),
		"payload":            cloneMap(payload),
		"command_timeout_ms": int(c.Timeout / time.Millisecond),
	}
	if msg["command_timeout_ms"].(int) <= 0 {
		msg["command_timeout_ms"] = 120000
	}
	if requestID != "" {
		msg["request_id"] = requestID
	}
	if transactionID != "" {
		msg["transaction_id"] = transactionID
	}
	return msg
}

func parseVSPCommandResult(reply map[string]any, raw string) *VSPCommandResult {
	payload := mapFromAny(reply["payload"])
	out := &VSPCommandResult{
		Response:      cloneMap(reply),
		Raw:           raw,
		Payload:       payload,
		LegacyReply:   mapFromAny(payload["legacy_reply"]),
		Command:       stringFromAny(payload["command"]),
		LegacyCommand: stringFromAny(payload["legacy_command"]),
		TransactionID: stringFromAny(reply["transaction_id"]),
		Revision:      int64FromAny(firstPresent(reply, payload, "revision")),
		ResyncHint:    boolFromAny(payload["resync_hint"]),
	}
	return out
}

func parseVSPStateResult(reply map[string]any, raw string) *VSPStateResult {
	payload := mapFromAny(reply["payload"])
	out := &VSPStateResult{
		Response:      cloneMap(reply),
		Raw:           raw,
		Payload:       payload,
		LegacyState:   legacyStateFromVSPPayload(payload),
		Revision:      int64FromAny(reply["revision"]),
		BaseRevision:  int64FromAny(reply["base_revision"]),
		ProjectEpoch:  stringFromAny(reply["project_epoch"]),
		SnapshotHash:  stringFromAny(payload["snapshot_hash"]),
		Scope:         stringFromAny(payload["scope"]),
		Resync:        boolFromAny(payload["resync"]),
		ResyncHint:    boolFromAny(payload["resync_hint"]) || strings.EqualFold(stringFromAny(payload["status"]), "resync_required"),
		Ops:           sliceFromAny(payload["ops"]),
		ChangedTracks: sliceFromAny(payload["changed_tracks"]),
		ChangedClips:  sliceFromAny(payload["changed_clips"]),
	}
	if out.SnapshotHash != "" {
		out.LegacyState["snapshot_hash"] = out.SnapshotHash
	}
	if out.Revision != 0 {
		out.LegacyState["project_revision"] = out.Revision
	}
	if out.ProjectEpoch != "" {
		out.LegacyState["project_epoch"] = out.ProjectEpoch
	}
	return out
}

func (r *VSPCommandResult) LegacyLikeReply() map[string]any {
	if r == nil {
		return nil
	}
	if len(r.LegacyReply) > 0 {
		return cloneMap(r.LegacyReply)
	}
	out := map[string]any{"status": "ok"}
	if strings.Contains(strings.ToLower(stringFromAny(r.Response["type"])), "error") {
		out["status"] = "error"
	}
	if errObj := mapFromAny(r.Response["error"]); len(errObj) > 0 {
		out["message"] = firstNonEmptyString(stringFromAny(errObj["message"]), stringFromAny(errObj["code"]))
		out["error"] = out["message"]
	}
	if ack := mapFromAny(r.Response["ack"]); len(ack) > 0 && out["message"] == nil {
		out["message"] = stringFromAny(ack["message"])
	}
	return out
}

func (r *VSPStateResult) OK() bool {
	if r == nil {
		return false
	}
	payloadStatus := strings.ToLower(strings.TrimSpace(stringFromAny(r.Payload["status"])))
	if payloadStatus == "ok" {
		return true
	}
	return strings.EqualFold(stringFromAny(r.Response["type"]), "state.snapshot") ||
		strings.EqualFold(stringFromAny(r.Response["type"]), "state.delta")
}

func legacyStateFromVSPPayload(payload map[string]any) map[string]any {
	out := mapFromAny(payload["snapshot"])
	if len(out) == 0 {
		out = map[string]any{}
	}
	if project := mapFromAny(payload["project"]); len(project) > 0 {
		out["project"] = project
		for _, key := range []string{"project_path", "project_uuid", "parent_project_uuid", "analysis_manifest"} {
			if _, ok := out[key]; !ok && project[key] != nil {
				out[key] = cloneAny(project[key])
			}
		}
	}
	if tracks, ok := payload["tracks"]; ok {
		out["tracks"] = cloneAny(tracks)
	}
	if _, ok := out["status"]; !ok {
		out["status"] = "ok"
	}
	return out
}

func newVSPID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", strings.TrimRight(prefix, "_"), time.Now().UnixNano(), vspIDCounter.Add(1))
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	var out map[string]any
	data, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func cloneAny(in any) any {
	var out any
	data, err := json.Marshal(in)
	if err != nil {
		return in
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return in
	}
	return out
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if row, ok := value.(map[string]any); ok {
		return cloneMap(row)
	}
	var row map[string]any
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(data, &row); err != nil {
		return nil
	}
	return row
}

func sliceFromAny(value any) []any {
	switch rows := value.(type) {
	case []any:
		return append([]any(nil), rows...)
	default:
		var out []any
		data, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	}
}

func int64FromAny(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case string:
		var out int64
		_, _ = fmt.Sscan(strings.TrimSpace(v), &out)
		return out
	default:
		return 0
	}
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		return text == "true" || text == "1" || text == "yes"
	default:
		return false
	}
}

func boolMapFromAny(value any) map[string]bool {
	row := mapFromAny(value)
	out := make(map[string]bool, len(row))
	for key, value := range row {
		out[key] = boolFromAny(value)
	}
	return out
}

func stringFromAny(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPresent(primary map[string]any, secondary map[string]any, key string) any {
	if primary != nil {
		if value, ok := primary[key]; ok {
			return value
		}
	}
	if secondary != nil {
		return secondary[key]
	}
	return nil
}
