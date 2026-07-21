package vsphub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
)

type KernelRawSender interface {
	SendRaw(context.Context, string) (string, error)
}

type Hub struct {
	cfg      Config
	kernel   KernelRawSender
	logger   *logx.Logger
	sessions *SessionManager
	registry *CapabilityRegistry
	streams  *StreamRegistry
	realtime *RealtimeStore
	events   *EventStore
	assets   *AssetMaterializationStore
	started  time.Time

	requests atomic.Int64
	errors   atomic.Int64
}

func New(cfg Config, kernelClient KernelRawSender, logger *logx.Logger) *Hub {
	cfg = cfg.WithDefaults()
	if kernelClient == nil {
		kernelClient = kernel.New(cfg.KernelReqURL, cfg.RequestTimeout)
	}
	return &Hub{
		cfg:      cfg,
		kernel:   kernelClient,
		logger:   logger,
		sessions: NewSessionManager(),
		registry: DefaultCapabilityRegistry(),
		streams:  NewStreamRegistry(),
		realtime: NewRealtimeStore(),
		events:   NewEventStore(),
		assets:   NewAssetMaterializationStore(),
		started:  time.Now().UTC(),
	}
}

func (h *Hub) Config() Config {
	if h == nil {
		return Config{}.WithDefaults()
	}
	return h.cfg
}

func (h *Hub) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/vsp/status", h.handleStatus)
	mux.HandleFunc("/vsp", h.handleHTTPVSP)
	mux.HandleFunc("/vsp/stream", h.handleStream)
	return mux
}

func (h *Hub) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:              h.cfg.HTTPAddr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: h.cfg.ReadHeaderTimeout,
	}
	errCh := make(chan error, 2)
	go func() {
		if h.logger != nil {
			h.logger.Info("VspHub HTTP listening on http://%s", h.cfg.HTTPAddr)
		}
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	if h.cfg.EnableTelemetry && strings.TrimSpace(h.cfg.KernelSubURL) != "" {
		go func() {
			errCh <- h.RunTelemetryRelay(ctx)
		}()
	}

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	}
}

func (h *Hub) Dispatch(ctx context.Context, env *Envelope, transport string) HubResponse {
	h.requests.Add(1)
	if env == nil {
		h.errors.Add(1)
		return ErrorResponse(nil, http.StatusBadRequest, "session", "session.close", "validation_error", "missing VSP envelope")
	}
	if transport == "" {
		transport = TransportHTTP
	}
	if env.SessionID() != "" && env.SessionID() != "session_pending" {
		h.sessions.Touch(env.SessionID())
	}
	if isLocalExtensionMessage(env) {
		if err := h.checkPermission(env); err != nil {
			h.errors.Add(1)
			return ErrorResponse(env, http.StatusForbidden, env.Channel(), env.Channel()+".error", "permission_denied", err.Error())
		}
		return h.handleExtensionMessage(env, transport)
	}
	if err := h.checkPermission(env); err != nil {
		h.errors.Add(1)
		return ErrorResponse(env, http.StatusForbidden, env.Channel(), env.Channel()+".error", "permission_denied", err.Error())
	}
	if env.Channel() == "asset" && env.Type() == "asset.materialize_request" {
		return h.handleAssetMaterializeLocal(env)
	}
	if env.Channel() == "realtime" {
		if resp, handled := h.handleRealtimeLocal(env, transport); handled {
			return resp
		}
	}
	reply, err := h.kernel.SendRaw(ctx, string(env.Raw))
	if err != nil {
		h.errors.Add(1)
		return ErrorResponse(env, gatewayStatus(err), env.Channel(), env.Channel()+".error", "kernel_unavailable", "kernel gateway: "+err.Error())
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		h.errors.Add(1)
		return ErrorResponse(env, http.StatusBadGateway, env.Channel(), env.Channel()+".error", "invalid_kernel_reply", "kernel returned an empty VSP reply")
	}
	if !json.Valid([]byte(reply)) {
		h.errors.Add(1)
		return ErrorResponse(env, http.StatusBadGateway, env.Channel(), env.Channel()+".error", "invalid_kernel_reply", "kernel returned invalid JSON")
	}
	if env.Channel() == "session" && env.Type() == "session.hello" {
		return h.handleKernelHelloReply(env, []byte(reply), transport)
	}
	if env.Channel() == "realtime" {
		switch env.Type() {
		case "realtime.subscribe":
			h.realtime.RegisterSubscriptionFromStatus(env, []byte(reply))
		case "realtime.unsubscribe":
			h.realtime.Unsubscribe(cleanString(env.Payload()["subscription_id"]))
		}
	}
	if env.Channel() == "event" && env.Type() == "event.subscribe" {
		h.events.RegisterSubscriptionFromReply(env, []byte(reply))
	}
	return HubResponse{Body: []byte(reply + "\n"), HTTPStatus: http.StatusOK}
}

func (h *Hub) handleKernelHelloReply(env *Envelope, replyBytes []byte, transport string) HubResponse {
	var reply map[string]any
	if err := json.Unmarshal(replyBytes, &reply); err != nil {
		h.errors.Add(1)
		return ErrorResponse(env, http.StatusBadGateway, "session", "session.close", "invalid_kernel_reply", "kernel hello reply was invalid JSON")
	}
	role := env.Role()
	caps := h.registry.CapabilitiesForRole(role)
	session := h.sessions.RegisterHello(env, reply, transport, caps)
	reply["hub"] = map[string]any{
		"name":              h.cfg.HubName,
		"version":           h.cfg.HubVersion,
		"session_id":        session.SessionID,
		"transport_binding": transport,
		"capabilities":      caps,
	}
	payload := mapFromAny(reply["payload"])
	payload["hub_name"] = h.cfg.HubName
	payload["hub_version"] = h.cfg.HubVersion
	payload["hub_transport_binding"] = transport
	payload["hub_session_registry"] = "active"
	reply["payload"] = payload
	if h.logger != nil {
		h.logger.Info("VSP session registered role=%s client_id=%s session_id=%s transport=%s", session.Role, session.ClientID, session.SessionID, session.Transport)
	}
	return JSONResponse(http.StatusOK, reply)
}

func (h *Hub) checkPermission(env *Envelope) error {
	if env.Channel() == "session" {
		return nil
	}
	role := env.Role()
	if session, ok := h.sessions.Get(env.SessionID()); ok && session.Role != "" {
		role = session.Role
	}
	capability := capabilityForEnvelope(env)
	if !h.registry.Allows(role, capability) {
		return fmt.Errorf("role %s is not allowed to use %s", role, capability)
	}
	return nil
}

func (h *Hub) handleExtensionMessage(env *Envelope, transport string) HubResponse {
	switch env.Type() {
	case "extension.register":
		caps := h.registry.CapabilitiesForRole("extension")
		session := h.sessions.RegisterLocal(env, transport, caps)
		reply := h.localReply(env, "vsp.extension.register_ack.v1", "extension", "extension.register_ack", map[string]any{
			"status":       "ok",
			"extension_id": firstNonEmpty(env.ClientID(), session.ClientID),
			"capabilities": caps,
		})
		reply["session_id"] = session.SessionID
		return JSONResponse(http.StatusOK, reply)
	case "extension.unregister":
		removed := h.sessions.Unregister(env.SessionID(), env.ClientID())
		payload := map[string]any{
			"status":       "ok",
			"extension_id": env.ClientID(),
			"removed":      removed,
		}
		if !removed {
			payload["status"] = "not_found"
		}
		return JSONResponse(http.StatusOK, h.localReply(env, "vsp.extension.unregister_ack.v1", "extension", "extension.unregister_ack", payload))
	default:
		h.errors.Add(1)
		return ErrorResponse(env, http.StatusBadRequest, "extension", "extension.error", "capability_not_supported", "unsupported extension message type")
	}
}

func (h *Hub) localReply(request *Envelope, schema, channel, messageType string, payload map[string]any) map[string]any {
	out := map[string]any{
		"vsp_version": VSPVersion,
		"schema":      schema,
		"message_id":  NewID("msg_hub"),
		"session_id":  firstNonEmpty(request.SessionID(), "session_unknown"),
		"client_id":   "vsp.hub",
		"role":        "hub",
		"channel":     channel,
		"type":        messageType,
		"created_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"payload":     cloneMap(payload),
		"ack": map[string]any{
			"stage":   "completed",
			"message": "handled by VSP Hub",
		},
	}
	if request.RequestID() != "" {
		out["request_id"] = request.RequestID()
	}
	if request.MessageID() != "" {
		out["correlation_id"] = request.MessageID()
	}
	if request.TraceID() != "" {
		out["trace_id"] = request.TraceID()
	}
	return out
}

func isLocalExtensionMessage(env *Envelope) bool {
	return env.Channel() == "extension"
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
