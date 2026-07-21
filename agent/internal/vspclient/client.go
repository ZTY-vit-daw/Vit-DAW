package vspclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const VSPVersion = "1.0"

var idCounter atomic.Uint64

type Client struct {
	URL           string
	HTTPClient    *http.Client
	ClientID      string
	Role          string
	ClientName    string
	ClientVersion string
	SessionID     string
}

func New(url, clientID, role, name, version string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Client{
		URL:           strings.TrimSpace(url),
		HTTPClient:    &http.Client{Timeout: timeout},
		ClientID:      strings.TrimSpace(clientID),
		Role:          strings.TrimSpace(role),
		ClientName:    strings.TrimSpace(name),
		ClientVersion: strings.TrimSpace(version),
	}
}

func (c *Client) Hello(ctx context.Context, wants []string, transports []string) (map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("vsp client is nil")
	}
	payload := map[string]any{
		"client_name":        firstNonEmpty(c.ClientName, c.ClientID),
		"client_version":     firstNonEmpty(c.ClientVersion, "unknown"),
		"protocol_min":       VSPVersion,
		"protocol_max":       VSPVersion,
		"wants":              append([]string(nil), wants...),
		"transport_bindings": append([]string(nil), transports...),
	}
	reply, err := c.Send(ctx, "session_pending", "session", "session.hello", "vsp.session.hello.v1", payload)
	if err != nil {
		return reply, err
	}
	if sessionID := strings.TrimSpace(fmt.Sprint(reply["session_id"])); sessionID != "" && sessionID != "<nil>" {
		c.SessionID = sessionID
	}
	return reply, nil
}

func (c *Client) Send(ctx context.Context, sessionID, channel, messageType, schema string, payload map[string]any) (map[string]any, error) {
	envelope := map[string]any{
		"vsp_version": VSPVersion,
		"schema":      schema,
		"message_id":  newID("msg"),
		"session_id":  firstNonEmpty(sessionID, c.SessionID, "session_pending"),
		"client_id":   firstNonEmpty(c.ClientID, "vit.agent"),
		"role":        firstNonEmpty(c.Role, "agent"),
		"channel":     strings.TrimSpace(channel),
		"type":        strings.TrimSpace(messageType),
		"created_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"trace_id":    newID("trace"),
		"payload":     cloneMap(payload),
	}
	return c.SendEnvelope(ctx, envelope)
}

func (c *Client) SendEnvelope(ctx context.Context, envelope map[string]any) (map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("vsp client is nil")
	}
	if strings.TrimSpace(c.URL) == "" {
		return nil, fmt.Errorf("vsp hub URL is empty")
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal VSP envelope: %w", err)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var reply map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
		return nil, fmt.Errorf("decode VSP reply: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return reply, fmt.Errorf("vsp hub returned HTTP %d: %s", resp.StatusCode, firstNonEmpty(stringFromAny(reply["error"]), stringFromAny(reply["message"])))
	}
	return reply, nil
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	data, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func newID(prefix string) string {
	prefix = strings.TrimRight(strings.TrimSpace(prefix), "_")
	if prefix == "" {
		prefix = "id"
	}
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), idCounter.Add(1))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}
