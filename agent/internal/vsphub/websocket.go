package vsphub

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"vit-daw-agent/internal/logx"
)

var (
	// streamWriteTimeout bounds a single websocket write; a peer that stops
	// reading (TCP backpressure) cannot wedge the writer forever.
	streamWriteTimeout = 3 * time.Second
	// streamQueueCapacity bounds per-client outbound buffering. A client that
	// falls further behind is evicted (fail-fast) instead of stalling the
	// shared telemetry relay or the publish HTTP path.
	streamQueueCapacity = 4096
	// streamPingInterval drives the application-level hub.heartbeat frame so
	// GUI watchdogs can distinguish "quiet" from "supply cut".
	streamPingInterval = 10 * time.Second
)

var streamHeartbeatBody = append([]byte(`{"vsp_version":"`+VSPVersion+`","schema":"vsp.hub.heartbeat.v1","client_id":"vsp.hub","role":"hub","channel":"hub","type":"hub.heartbeat"}`), '\n')

type StreamRegistry struct {
	mu      sync.RWMutex
	streams map[string]*StreamClient
}

type StreamClient struct {
	ID          string
	RemoteAddr  string
	ConnectedAt time.Time
	SessionID   string
	ClientID    string
	Role        string

	conn             *websocket.Conn
	mu               sync.Mutex
	lastRealtimeSent map[string]time.Time

	outbox     chan []byte
	closeOnce  sync.Once
	closed     chan struct{}
	logger     *logx.Logger
	evicted    atomic.Bool
	evictMsg   atomic.Value // string
	queued     atomic.Int64
	dropped    atomic.Int64
	sent       atomic.Int64
}

func NewStreamRegistry() *StreamRegistry {
	return &StreamRegistry{streams: map[string]*StreamClient{}}
}

func (r *StreamRegistry) Add(conn *websocket.Conn, remoteAddr string, logger *logx.Logger) *StreamClient {
	client := &StreamClient{
		ID:          NewID("stream"),
		RemoteAddr:  remoteAddr,
		ConnectedAt: time.Now().UTC(),
		conn:        conn,
		outbox:      make(chan []byte, streamQueueCapacity),
		closed:      make(chan struct{}),
		logger:      logger,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[client.ID] = client
	return client
}

func (r *StreamRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, id)
}

func (r *StreamRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.streams)
}

func (r *StreamRegistry) Clients() []*StreamClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	streams := make([]*StreamClient, 0, len(r.streams))
	for _, stream := range r.streams {
		streams = append(streams, stream)
	}
	return streams
}

func (r *StreamRegistry) Broadcast(body []byte) {
	for _, stream := range r.Clients() {
		stream.Write(body)
	}
}

func (c *StreamClient) SetSession(sessionID, clientID, role string) {
	if c == nil {
		return
	}
	sessionID = cleanString(sessionID)
	if sessionID == "" || sessionID == "session_pending" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SessionID = sessionID
	c.ClientID = firstNonEmpty(cleanString(clientID), c.ClientID)
	c.Role = firstNonEmpty(cleanString(role), c.Role)
}

func (c *StreamClient) BoundSessionID() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.SessionID
}

func (c *StreamClient) BindFromEnvelopeReply(env *Envelope, replyBody []byte) {
	if c == nil || env == nil {
		return
	}
	sessionID := env.SessionID()
	var reply map[string]any
	if err := json.Unmarshal(replyBody, &reply); err == nil {
		sessionID = firstNonEmpty(cleanString(reply["session_id"]), sessionID)
	}
	c.SetSession(sessionID, env.ClientID(), env.Role())
}

func (c *StreamClient) ShouldSendRealtime(key string, maxHz int, now time.Time) bool {
	if c == nil {
		return false
	}
	key = cleanString(key)
	if key == "" {
		return true
	}
	if maxHz <= 0 {
		maxHz = 30
	}
	minInterval := time.Second / time.Duration(maxHz)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastRealtimeSent == nil {
		c.lastRealtimeSent = map[string]time.Time{}
	}
	last := c.lastRealtimeSent[key]
	if !last.IsZero() && now.Sub(last) < minInterval {
		return false
	}
	c.lastRealtimeSent[key] = now
	return true
}

// Write enqueues body for asynchronous delivery; it never blocks the caller
// (the telemetry relay and publish handlers rely on this). Enqueue onto a full
// queue evicts the lagging client so it can be cleaned up and reconnect.
func (c *StreamClient) Write(body []byte) {
	if c == nil {
		return
	}
	if c.evicted.Load() {
		return
	}
	select {
	case <-c.closed:
		return
	default:
	}
	select {
	case c.outbox <- body:
		c.queued.Add(1)
	default:
		c.dropped.Add(1)
		c.Evict("outbox_overflow")
	}
}

// Evict closes the underlying connection once; the read loop in handleStream
// then unblocks and removes the client from the registry.
func (c *StreamClient) Evict(reason string) {
	if c == nil {
		return
	}
	first := false
	c.closeOnce.Do(func() {
		c.evictMsg.Store(reason)
		c.evicted.Store(true)
		first = true
		close(c.closed)
		_ = c.conn.Close()
	})
	if first && c.logger != nil {
		c.logger.Warn("VSP stream evicted id=%s remote=%s reason=%s", c.ID, c.RemoteAddr, reason)
	}
}

// StartWriter launches the per-client writer goroutine: it drains the outbox
// and emits hub.heartbeat frames, both under StreamWriteTimeout. Any write
// failure evicts the client instead of blocking shared paths.
func (c *StreamClient) StartWriter() {
	if c == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(streamPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.closed:
				return
			case body := <-c.outbox:
				if !c.writeFrame(body) {
					return
				}
				c.sent.Add(1)
			case <-ticker.C:
				if !c.writeFrame(streamHeartbeatBody) {
					return
				}
			}
		}
	}()
}

func (c *StreamClient) writeFrame(body []byte) bool {
	_ = c.conn.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
	if err := c.conn.WriteMessage(websocket.TextMessage, body); err != nil {
		c.Evict("write_failed")
		return false
	}
	return true
}

// SnapshotInfo reports per-client delivery counters for diagnostics and the
// resilience injection leg.
func (c *StreamClient) SnapshotInfo() map[string]any {
	if c == nil {
		return nil
	}
	return map[string]any{
		"id":           c.ID,
		"remote":       c.RemoteAddr,
		"connected_at": c.ConnectedAt.Format(time.RFC3339Nano),
		"client_id":    c.ClientID,
		"role":         c.Role,
		"queue_depth":  len(c.outbox),
		"queue_cap":    streamQueueCapacity,
		"queued":       c.queued.Load(),
		"sent":         c.sent.Load(),
		"dropped":      c.dropped.Load(),
		"evicted":      c.evicted.Load(),
		"evict_reason": c.evictMsg.Load(),
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Hub) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("websocket upgrade failed: %v", err)
		}
		return
	}
	conn.SetReadLimit(MaxHTTPBodyBytes)
	stream := h.streams.Add(conn, r.RemoteAddr, h.logger)
	stream.StartWriter()
	defer func() {
		h.streams.Remove(stream.ID)
		stream.Evict("handler_exit")
		conn.Close()
	}()
	if h.logger != nil {
		h.logger.Info("VSP stream connected id=%s remote=%s", stream.ID, stream.RemoteAddr)
	}

	for {
		messageType, body, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		env, err := ParseEnvelopeBytes(body)
		if err != nil {
			resp := ErrorEnvelope(nil, "session", "session.close", "validation_error", err.Error())
			data, _ := json.Marshal(resp)
			stream.Write(append(data, '\n'))
			continue
		}
		ctx, cancel := context.WithTimeout(r.Context(), h.cfg.RequestTimeout)
		resp := h.Dispatch(ctx, env, TransportWebSocket)
		cancel()
		stream.BindFromEnvelopeReply(env, resp.Body)
		stream.Write(resp.Body)
		if env.Channel() == "event" && env.Type() == "event.subscribe" {
			h.replayCachedTelemetry(stream)
		}
	}
}
