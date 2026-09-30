package executionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultProjectWriteLeaseTTL bounds how long one capability execution segment
// may hold the per-project write lease before it is force-released, so a hung
// holder cannot deadlock every other session waiting on the same project
// (VITNOTE_V1_DESIGN §7.3).
const DefaultProjectWriteLeaseTTL = 120 * time.Second

// writeLeaseEventsPathEnv, when set before agent start, makes every lease
// transition append a JSONL line to that file. Real-stack smokes use it as the
// durable evidence that concurrent execution segments were serialized.
const writeLeaseEventsPathEnv = "VIT_WRITE_LEASE_EVENTS_PATH"

// ProjectWriteLease is a single acquisition of one project's write lease. The
// holder owns the lease from Acquire until Release, TTL expiry, or the Acquire
// context being cancelled while still queued. Release is idempotent.
type ProjectWriteLease struct {
	manager    *ProjectWriteLeases
	projectID  string
	holder     string
	acquiredAt time.Time
	deadline   time.Time
	timer      *time.Timer
	released   bool
	expired    bool
}

// ProjectID returns the project this lease was acquired for.
func (l *ProjectWriteLease) ProjectID() string {
	if l == nil {
		return ""
	}
	return l.projectID
}

// Holder returns the caller-supplied label (the planning session id) recorded
// with this lease.
func (l *ProjectWriteLease) Holder() string {
	if l == nil {
		return ""
	}
	return l.holder
}

// AcquiredAt returns the UTC time the lease was granted to its holder.
func (l *ProjectWriteLease) AcquiredAt() time.Time {
	if l == nil {
		return time.Time{}
	}
	return l.acquiredAt
}

// Deadline returns the UTC time the lease would be force-released if its
// holder never calls Release.
func (l *ProjectWriteLease) Deadline() time.Time {
	if l == nil {
		return time.Time{}
	}
	return l.deadline
}

// Expired reports whether the lease was force-released by the TTL watchdog.
func (l *ProjectWriteLease) Expired() bool {
	if l == nil {
		return false
	}
	return l.expired
}

// Release returns the lease to its manager and hands it to the next queued
// waiter. Releasing an already released (or TTL-expired) lease is a no-op.
func (l *ProjectWriteLease) Release() {
	if l == nil || l.manager == nil {
		return
	}
	m := l.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if l.released {
		return
	}
	m.releaseLocked(m.projects[l.projectID], l, "released")
}

// ProjectWriteLeases is the per-project write lease manager. All capability
// execution segments in one agent process share a single manager instance, so
// concurrent sessions writing the same project serialize while different
// projects proceed in parallel. v1 is in-process memory only; cross-process
// mutual exclusion stays with the kernel command CAS and the store file lock.
type ProjectWriteLeases struct {
	mu       sync.Mutex
	ttl      time.Duration
	projects map[string]*projectLeaseState

	eventsMu   sync.Mutex
	eventsPath string
}

type projectLeaseState struct {
	holder  *ProjectWriteLease
	waiters []*projectLeaseWaiter
}

type projectLeaseWaiter struct {
	ready     chan struct{}
	holder    string
	lease     *ProjectWriteLease
	granted   bool
	cancelled bool
}

// NewProjectWriteLeases returns a manager using DefaultProjectWriteLeaseTTL and
// the JSONL events file named by VIT_WRITE_LEASE_EVENTS_PATH, if that variable
// is set in the process environment.
func NewProjectWriteLeases() *ProjectWriteLeases {
	return &ProjectWriteLeases{
		ttl:        DefaultProjectWriteLeaseTTL,
		projects:   map[string]*projectLeaseState{},
		eventsPath: strings.TrimSpace(os.Getenv(writeLeaseEventsPathEnv)),
	}
}

// Acquire returns the lease for projectID, queueing behind the current holder
// when necessary. Waiters are served in arrival order and are never rejected;
// the only failure is the caller's context being cancelled while queued, which
// returns a wrapped context error. holder is an observability label (planning
// session id) recorded in the lease event log.
func (m *ProjectWriteLeases) Acquire(ctx context.Context, projectID, holder string) (*ProjectWriteLease, error) {
	if m == nil {
		return nil, errors.New("project write lease manager is nil")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("project write lease requires a non-empty project id")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("acquire project write lease for %s: %w", projectID, err)
	}

	m.mu.Lock()
	if m.projects == nil {
		m.projects = map[string]*projectLeaseState{}
	}
	state := m.projects[projectID]
	if state == nil {
		state = &projectLeaseState{}
		m.projects[projectID] = state
	}
	// The TTL timer is the primary watchdog; this lazy check keeps correctness
	// from depending on timer promptness when a waiter arrives around the
	// deadline of an already expired holder.
	if state.holder != nil && !time.Now().UTC().Before(state.holder.deadline) {
		m.releaseLocked(state, state.holder, "ttl_expired")
	}
	if state.holder == nil {
		lease := m.grantLocked(projectID, holder, state)
		m.mu.Unlock()
		return lease, nil
	}
	waiter := &projectLeaseWaiter{ready: make(chan struct{}), holder: holder}
	state.waiters = append(state.waiters, waiter)
	queued := len(state.waiters)
	m.mu.Unlock()
	m.recordEvent(writeLeaseEvent{Time: time.Now().UTC(), Event: "wait_started", ProjectID: projectID, Holder: holder, Waiters: queued})

	select {
	case <-waiter.ready:
		return waiter.lease, nil
	case <-ctx.Done():
		m.mu.Lock()
		if waiter.granted {
			// The lease was handed to us concurrently with cancellation; return
			// it instead of leaking a held lease nobody will release.
			m.releaseLocked(m.projects[projectID], waiter.lease, "released")
		} else {
			waiter.cancelled = true
			m.removeWaiterLocked(m.projects[projectID], waiter)
		}
		m.mu.Unlock()
		m.recordEvent(writeLeaseEvent{Time: time.Now().UTC(), Event: "wait_cancelled", ProjectID: projectID, Holder: holder})
		return nil, fmt.Errorf("acquire project write lease for %s: %w", projectID, ctx.Err())
	}
}

func (m *ProjectWriteLeases) grantLocked(projectID, holder string, state *projectLeaseState) *ProjectWriteLease {
	ttl := m.ttl
	if ttl <= 0 {
		ttl = DefaultProjectWriteLeaseTTL
	}
	now := time.Now().UTC()
	lease := &ProjectWriteLease{
		manager:    m,
		projectID:  projectID,
		holder:     holder,
		acquiredAt: now,
		deadline:   now.Add(ttl),
	}
	lease.timer = time.AfterFunc(ttl, func() { m.expire(lease) })
	state.holder = lease
	m.recordEvent(writeLeaseEvent{Time: now, Event: "acquired", ProjectID: projectID, Holder: holder, Waiters: len(state.waiters)})
	return lease
}

func (m *ProjectWriteLeases) expire(lease *ProjectWriteLease) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lease.released {
		return
	}
	state := m.projects[lease.projectID]
	if state == nil || state.holder != lease {
		return
	}
	m.releaseLocked(state, lease, "ttl_expired")
}

// releaseLocked marks the lease released, disarms its timer and hands the
// project to the next live waiter. reason is "released" for a holder-initiated
// release and "ttl_expired" for the watchdog.
func (m *ProjectWriteLeases) releaseLocked(state *projectLeaseState, lease *ProjectWriteLease, reason string) {
	if lease == nil || lease.released {
		return
	}
	lease.released = true
	if reason == "ttl_expired" {
		lease.expired = true
	}
	if lease.timer != nil {
		lease.timer.Stop()
	}
	if state == nil || state.holder != lease {
		return
	}
	m.recordEvent(writeLeaseEvent{Time: time.Now().UTC(), Event: reason, ProjectID: lease.projectID, Holder: lease.holder, Waiters: len(state.waiters)})
	for len(state.waiters) > 0 {
		waiter := state.waiters[0]
		state.waiters = state.waiters[1:]
		if waiter.cancelled {
			continue
		}
		granted := m.grantLocked(lease.projectID, waiter.holder, state)
		waiter.lease = granted
		waiter.granted = true
		close(waiter.ready)
		return
	}
	state.holder = nil
}

func (m *ProjectWriteLeases) removeWaiterLocked(state *projectLeaseState, waiter *projectLeaseWaiter) {
	if state == nil {
		return
	}
	for index, candidate := range state.waiters {
		if candidate == waiter {
			state.waiters = append(state.waiters[:index], state.waiters[index+1:]...)
			return
		}
	}
}

type writeLeaseEvent struct {
	Time      time.Time `json:"time"`
	Event     string    `json:"event"`
	ProjectID string    `json:"project_id"`
	Holder    string    `json:"holder"`
	Waiters   int       `json:"waiters"`
}

// recordEvent appends one JSONL line to the configured events file. It is best
// effort: observability must never fail a lease transition. It runs under the
// manager mutex, which is acceptable because lease transitions are rare
// (once per capability execution segment).
func (m *ProjectWriteLeases) recordEvent(event writeLeaseEvent) {
	if m == nil || m.eventsPath == "" {
		return
	}
	line, err := json.Marshal(event)
	if err != nil {
		return
	}
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(m.eventsPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(m.eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(line, '\n'))
}
