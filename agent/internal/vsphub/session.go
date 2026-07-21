package vsphub

import (
	"strings"
	"sync"
	"time"
)

type ClientSession struct {
	SessionID       string         `json:"session_id"`
	ClientID        string         `json:"client_id"`
	Role            string         `json:"role"`
	ClientName      string         `json:"client_name,omitempty"`
	ClientVersion   string         `json:"client_version,omitempty"`
	Transport       string         `json:"transport"`
	Wants           []string       `json:"wants,omitempty"`
	Capabilities    []string       `json:"capabilities,omitempty"`
	FeatureFlags    map[string]any `json:"feature_flags,omitempty"`
	ConnectedAt     time.Time      `json:"connected_at"`
	LastSeenAt      time.Time      `json:"last_seen_at"`
	MessageCount    int64          `json:"message_count"`
	KernelSessionID string         `json:"kernel_session_id,omitempty"`
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*ClientSession
	clients  map[string]string
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: map[string]*ClientSession{},
		clients:  map[string]string{},
	}
}

func (m *SessionManager) RegisterHello(request *Envelope, reply map[string]any, transport string, caps []string) ClientSession {
	now := time.Now().UTC()
	payload := request.Payload()
	sessionID := strings.TrimSpace(stringFromAny(reply["session_id"]))
	if sessionID == "" {
		sessionID = strings.TrimSpace(request.SessionID())
	}
	if sessionID == "" || sessionID == "session_pending" {
		sessionID = NewID("sess_hub")
	}
	clientID := firstNonEmpty(request.ClientID(), stringFromAny(payload["client_id"]), "client_"+sessionID)
	role := normalizeRole(firstNonEmpty(request.Role(), stringFromAny(payload["role"])))

	session := &ClientSession{
		SessionID:       sessionID,
		ClientID:        clientID,
		Role:            role,
		ClientName:      firstNonEmpty(stringFromAny(payload["client_name"]), stringFromAny(payload["name"])),
		ClientVersion:   firstNonEmpty(stringFromAny(payload["client_version"]), stringFromAny(payload["version"])),
		Transport:       transport,
		Wants:           stringSliceFromAny(payload["wants"]),
		Capabilities:    append([]string(nil), caps...),
		FeatureFlags:    mapFromAny(reply["feature_flags"]),
		ConnectedAt:     now,
		LastSeenAt:      now,
		MessageCount:    1,
		KernelSessionID: sessionID,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = session
	m.clients[clientID] = sessionID
	return *session
}

func (m *SessionManager) RegisterLocal(request *Envelope, transport string, caps []string) ClientSession {
	return m.RegisterHello(request, map[string]any{"session_id": request.SessionID()}, transport, caps)
}

func (m *SessionManager) Unregister(sessionID, clientID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	clientID = strings.TrimSpace(clientID)

	m.mu.Lock()
	defer m.mu.Unlock()

	if sessionID == "" && clientID != "" {
		sessionID = m.clients[clientID]
	}
	session, ok := m.sessions[sessionID]
	if !ok || session == nil {
		return false
	}
	delete(m.sessions, sessionID)
	delete(m.clients, session.ClientID)
	if clientID != "" {
		delete(m.clients, clientID)
	}
	return true
}

func (m *SessionManager) Touch(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if session, ok := m.sessions[sessionID]; ok {
		session.LastSeenAt = time.Now().UTC()
		session.MessageCount++
	}
}

func (m *SessionManager) Get(sessionID string) (ClientSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[strings.TrimSpace(sessionID)]
	if !ok || session == nil {
		return ClientSession{}, false
	}
	return *session, true
}

func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) Snapshot() []ClientSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ClientSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		if session != nil {
			out = append(out, *session)
		}
	}
	return out
}
