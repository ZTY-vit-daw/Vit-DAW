package vsphub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type Envelope struct {
	Raw    []byte
	Object map[string]any
}

var idCounter atomic.Uint64

func ParseEnvelopeBytes(body []byte) (*Envelope, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("empty VSP body")
	}
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("VSP envelope must be a JSON object")
	}
	env := &Envelope{Raw: append([]byte(nil), body...), Object: object}
	if err := env.Validate(); err != nil {
		return nil, err
	}
	return env, nil
}

func (e *Envelope) Validate() error {
	if e == nil || e.Object == nil {
		return fmt.Errorf("VSP envelope must be a JSON object")
	}
	for _, field := range []string{"vsp_version", "schema", "message_id", "session_id", "client_id", "role", "channel", "type", "created_at"} {
		if e.StringField(field) == "" {
			return fmt.Errorf("VSP envelope missing %s", field)
		}
	}
	if e.StringField("vsp_version") != VSPVersion {
		return fmt.Errorf("unsupported VSP version: %s", e.StringField("vsp_version"))
	}
	payload, ok := e.Object["payload"]
	if !ok {
		return fmt.Errorf("VSP envelope missing payload")
	}
	if _, ok := payload.(map[string]any); !ok {
		return fmt.Errorf("VSP envelope payload must be a JSON object")
	}
	return nil
}

func (e *Envelope) StringField(field string) string {
	if e == nil || e.Object == nil {
		return ""
	}
	value, ok := e.Object[field]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func (e *Envelope) Payload() map[string]any {
	if e == nil || e.Object == nil {
		return map[string]any{}
	}
	return mapFromAny(e.Object["payload"])
}

func (e *Envelope) CloneObject() map[string]any {
	if e == nil {
		return map[string]any{}
	}
	return cloneMap(e.Object)
}

func (e *Envelope) Channel() string {
	return e.StringField("channel")
}

func (e *Envelope) Type() string {
	return e.StringField("type")
}

func (e *Envelope) SessionID() string {
	return e.StringField("session_id")
}

func (e *Envelope) ClientID() string {
	return e.StringField("client_id")
}

func (e *Envelope) Role() string {
	return normalizeRole(e.StringField("role"))
}

func (e *Envelope) RequestID() string {
	return e.StringField("request_id")
}

func (e *Envelope) TraceID() string {
	return e.StringField("trace_id")
}

func (e *Envelope) MessageID() string {
	return e.StringField("message_id")
}

func NewID(prefix string) string {
	prefix = strings.TrimRight(strings.TrimSpace(prefix), "_")
	if prefix == "" {
		prefix = "id"
	}
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), idCounter.Add(1))
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
	data, err := json.Marshal(in)
	if err != nil {
		return in
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return in
	}
	return out
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if row, ok := value.(map[string]any); ok {
		return cloneMap(row)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func stringSliceFromAny(value any) []string {
	rows, ok := value.([]any)
	if !ok {
		var decoded []any
		data, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil
		}
		rows = decoded
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		text := strings.TrimSpace(fmt.Sprint(row))
		if text != "" && text != "<nil>" {
			out = append(out, text)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "", "ui":
		return "gui"
	case "agent", "gui", "extension", "tool", "controller", "analyzer", "kernel":
		return role
	default:
		return role
	}
}
