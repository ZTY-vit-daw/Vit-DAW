package vsphub

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type StreamRegistry struct {
	mu      sync.RWMutex
	streams map[string]*StreamClient
}

type StreamClient struct {
	ID               string
	RemoteAddr       string
	ConnectedAt      time.Time
	SessionID        string
	ClientID         string
	Role             string
	conn             *websocket.Conn
	mu               sync.Mutex
	lastRealtimeSent map[string]time.Time
}

func NewStreamRegistry() *StreamRegistry {
	return &StreamRegistry{streams: map[string]*StreamClient{}}
}

func (r *StreamRegistry) Add(conn *websocket.Conn, remoteAddr string) *StreamClient {
	client := &StreamClient{
		ID:          NewID("stream"),
		RemoteAddr:  remoteAddr,
		ConnectedAt: time.Now().UTC(),
		conn:        conn,
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

func (c *StreamClient) Write(body []byte) {
	if c == nil || c.conn == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.WriteMessage(websocket.TextMessage, body)
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
	defer conn.Close()
	conn.SetReadLimit(MaxHTTPBodyBytes)
	stream := h.streams.Add(conn, r.RemoteAddr)
	defer h.streams.Remove(stream.ID)
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
