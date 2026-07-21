package vsphub

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type HubResponse struct {
	Body       []byte
	HTTPStatus int
}

func JSONResponse(status int, body map[string]any) HubResponse {
	data, _ := json.Marshal(body)
	return HubResponse{Body: append(data, '\n'), HTTPStatus: status}
}

func ErrorResponse(request *Envelope, status int, channel, messageType, code, message string) HubResponse {
	return JSONResponse(status, ErrorEnvelope(request, channel, messageType, code, message))
}

func ErrorEnvelope(request *Envelope, channel, messageType, code, message string) map[string]any {
	if channel == "" {
		channel = "session"
	}
	if messageType == "" {
		messageType = channel + ".error"
	}
	out := map[string]any{
		"vsp_version": VSPVersion,
		"schema":      "vsp." + channel + ".error.v1",
		"message_id":  NewID("msg_hub"),
		"session_id":  "session_unknown",
		"client_id":   "vsp.hub",
		"role":        "hub",
		"channel":     channel,
		"type":        messageType,
		"created_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"ack": map[string]any{
			"stage":   "rejected",
			"message": message,
		},
		"error": map[string]any{
			"code":      code,
			"message":   message,
			"retryable": code == "timeout" || code == "transport_error" || code == "kernel_unavailable",
		},
		"payload": map[string]any{
			"status": "error",
		},
	}
	if request != nil {
		out["session_id"] = firstNonEmpty(request.SessionID(), "session_unknown")
		if request.RequestID() != "" {
			out["request_id"] = request.RequestID()
		}
		if request.MessageID() != "" {
			out["correlation_id"] = request.MessageID()
		}
		if request.TraceID() != "" {
			out["trace_id"] = request.TraceID()
		}
	}
	return out
}

func gatewayStatus(err error) int {
	if err == nil {
		return http.StatusBadGateway
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "deadline exceeded") || strings.Contains(text, "timeout") {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}
