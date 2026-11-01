package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
)

// M3-NEUTRAL-ACTION-PATH-1（235）measurement face: env-gated full payload
// capture. The 234 forensics gap was that neither telemetry (metadata only)
// nor prompt-render (assembly only, no response body) could prove what the
// model actually received and answered — this capture closes that gap for
// every LLM call: the complete request messages (system + user + history,
// full bytes) plus the response content (reply text and the raw HTTP body,
// which carries any tool_calls form the engine used).
//
// Gating (default OFF, ruling on the card):
//   - env unset            -> no capture, no writes, and NO fallback path —
//     unlike telemetry there is no default location, so a capture file can
//     never land in the source tree or workspace unless a caller explicitly
//     supplies a run directory;
//   - VIT_AGENT_LLM_PAYLOAD_CAPTURE=<dir> -> writes <dir>/agent_llm_payload_capture.jsonl
//     (dir created as needed; same explicit-off vocabulary as
//     VIT_AGENT_LLM_TELEMETRY_PATH: "off"/"disabled").
//
// The capture is measurement only: append errors are swallowed so a capture
// problem can never break a production call path.

const PayloadCaptureSchemaVersion = "vit_llm_payload_capture.v1"

const payloadCaptureFileName = "agent_llm_payload_capture.jsonl"

// PayloadCaptureRequest carries the full request surface as sent (post
// assembly): every message with complete role/content bytes, the tool
// surface, and the shaping flags.
type PayloadCaptureRequest struct {
	Messages        []Message        `json:"messages"`
	Tools           []map[string]any `json:"tools,omitempty"`
	ToolChoice      any              `json:"tool_choice,omitempty"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	PreferJSON      bool             `json:"prefer_json,omitempty"`
}

// PayloadCaptureResponse carries the response content: the parsed reply
// text, the raw HTTP response body (last attempt — the authoritative bytes
// for tool_calls and any engine-side detail), the call error if any, and
// usage.
type PayloadCaptureResponse struct {
	Text  string `json:"text,omitempty"`
	Raw   string `json:"raw,omitempty"`
	Error string `json:"error,omitempty"`
	Usage Usage  `json:"usage"`
}

type PayloadCaptureRecord struct {
	SchemaVersion  string                `json:"schema_version"`
	CreatedAt      string                `json:"created_at"`
	Source         string                `json:"source,omitempty"`
	ConversationID string                `json:"conversation_id,omitempty"`
	GoalID         string                `json:"goal_id,omitempty"`
	Model          string                `json:"model,omitempty"`
	EndpointKind   string                `json:"endpoint_kind,omitempty"`
	Request        PayloadCaptureRequest `json:"request"`
	Response       PayloadCaptureResponse `json:"response"`
}

// payloadCapturePath resolves the capture file path. Empty string means
// capture off. Default is off with no fallback directory (§10: nothing may
// be written on default paths).
func payloadCapturePath() string {
	override := strings.TrimSpace(os.Getenv("VIT_AGENT_LLM_PAYLOAD_CAPTURE"))
	if override == "" {
		return ""
	}
	if strings.EqualFold(override, "off") || strings.EqualFold(override, "disabled") {
		return ""
	}
	return filepath.Join(override, payloadCaptureFileName)
}

// recordPayloadCapture appends one full-payload row for a completed (or
// failed) LLM call. rawBody is the last HTTP response body when one was
// received (nil on transport errors).
func recordPayloadCapture(cfg config.EngineConfig, req Request, resp Response, callErr error, rawBody []byte, endpointKind string) {
	path := payloadCapturePath()
	if path == "" {
		return
	}
	record := PayloadCaptureRecord{
		SchemaVersion:  PayloadCaptureSchemaVersion,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		Source:         strings.TrimSpace(req.Metadata.Source),
		ConversationID: strings.TrimSpace(req.Metadata.ConversationID),
		GoalID:         strings.TrimSpace(req.Metadata.GoalID),
		Model:          strings.TrimSpace(cfg.DefaultModel),
		EndpointKind:   strings.TrimSpace(endpointKind),
		Request: PayloadCaptureRequest{
			Messages:        req.Messages,
			Tools:           req.Tools,
			ToolChoice:      req.ToolChoice,
			MaxOutputTokens: req.MaxOutputTokens,
			PreferJSON:      req.PreferJSON,
		},
		Response: PayloadCaptureResponse{
			Text:  resp.Text,
			Raw:   string(rawBody),
			Usage: resp.Usage,
		},
	}
	if callErr != nil {
		record.Response.Error = callErr.Error()
	}
	_ = appendPayloadCaptureJSONL(path, record)
}

func appendPayloadCaptureJSONL(path string, record PayloadCaptureRecord) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(record)
}
