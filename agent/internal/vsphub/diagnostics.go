package vsphub

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const diagnosticsVersion = "vsp.hub.diagnostics.v1"

func (h *Hub) Health() map[string]any {
	cfg := h.Config()
	return map[string]any{
		"status":              "ok",
		"service":             "VspHub",
		"diagnostics_version": diagnosticsVersion,
		"hub_name":            cfg.HubName,
		"hub_version":         cfg.HubVersion,
		"pid":                 os.Getpid(),
		"checked_at":          time.Now().UTC().Format(time.RFC3339Nano),
		"started_at":          h.started.Format(time.RFC3339Nano),
		"uptime_ms":           time.Since(h.started).Milliseconds(),
		"listen":              cfg.HTTPAddr,
		"kernel":              h.kernelDiagnostics(cfg),
		"transports":          []string{TransportHTTP, TransportWebSocket},
		"transport_bindings":  h.transportDiagnostics(cfg),
		"session_count":       h.sessions.Count(),
		"stream_count":        h.streams.Count(),
		"realtime":            h.realtime.Snapshot(),
		"events":              h.events.Snapshot(),
		"assets":              h.assets.Snapshot(),
		"requests":            h.requests.Load(),
		"errors":              h.errors.Load(),
	}
}

func (h *Hub) Status() map[string]any {
	cfg := h.Config()
	return map[string]any{
		"status":              "ok",
		"service":             "VspHub",
		"diagnostics_version": diagnosticsVersion,
		"hub_name":            cfg.HubName,
		"hub_version":         cfg.HubVersion,
		"pid":                 os.Getpid(),
		"checked_at":          time.Now().UTC().Format(time.RFC3339Nano),
		"started_at":          h.started.Format(time.RFC3339Nano),
		"uptime_ms":           time.Since(h.started).Milliseconds(),
		"listen":              cfg.HTTPAddr,
		"kernel_req":          cfg.KernelReqURL,
		"kernel_sub":          cfg.KernelSubURL,
		"kernel":              h.kernelDiagnostics(cfg),
		"transports":          []string{TransportHTTP, TransportWebSocket},
		"transport_bindings":  h.transportDiagnostics(cfg),
		"sessions":            h.sessions.Snapshot(),
		"session_count":       h.sessions.Count(),
		"stream_count":        h.streams.Count(),
		"realtime":            h.realtime.Snapshot(),
		"events":              h.events.Snapshot(),
		"requests":            h.requests.Load(),
		"errors":              h.errors.Load(),
		"capabilities": map[string]any{
			"gui":       h.registry.CapabilitiesForRole("gui"),
			"agent":     h.registry.CapabilitiesForRole("agent"),
			"extension": h.registry.CapabilitiesForRole("extension"),
			"kernel":    h.registry.CapabilitiesForRole("kernel"),
		},
	}
}

func (h *Hub) kernelDiagnostics(cfg Config) map[string]any {
	status := "configured"
	if strings.TrimSpace(cfg.KernelReqURL) == "" {
		status = "missing_req_endpoint"
	}
	return map[string]any{
		"status":             status,
		"req_url":            cfg.KernelReqURL,
		"sub_url":            cfg.KernelSubURL,
		"telemetry_enabled":  cfg.EnableTelemetry,
		"request_timeout_ms": cfg.RequestTimeout.Milliseconds(),
	}
}

func (h *Hub) transportDiagnostics(cfg Config) []map[string]any {
	return []map[string]any{
		{
			"name":     TransportHTTP,
			"endpoint": hubEndpoint("http", cfg.HTTPAddr, "/vsp"),
			"mode":     "request_reply",
			"status":   "enabled",
		},
		{
			"name":         TransportWebSocket,
			"endpoint":     hubEndpoint("ws", cfg.HTTPAddr, "/vsp/stream"),
			"mode":         "duplex_stream",
			"status":       "enabled",
			"stream_count": h.streams.Count(),
		},
	}
}

func hubEndpoint(scheme, addr, path string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = DefaultHTTPAddr
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s://%s%s", scheme, addr, path)
}
