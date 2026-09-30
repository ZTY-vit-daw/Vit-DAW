package executionruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/orchestration"
)

// acquisition records one observed lease hold window, guarded by the segment
// mutex so overlap violations are detected without relying on wall-clock
// comparison of concurrent writes.
type acquisition struct {
	enter time.Time
	exit  time.Time
}

func newShortTTLManager() *ProjectWriteLeases {
	return &ProjectWriteLeases{ttl: 60 * time.Millisecond, projects: map[string]*projectLeaseState{}}
}

func TestProjectWriteLeaseSerializesTwoConcurrentSegments(t *testing.T) {
	manager := NewProjectWriteLeases()
	firstAcquired := make(chan *ProjectWriteLease, 1)
	firstRelease := make(chan struct{})
	secondAcquired := make(chan *ProjectWriteLease, 1)
	secondRelease := make(chan struct{})

	go func() {
		lease, err := manager.Acquire(context.Background(), "p1", "session-a")
		if err != nil {
			t.Errorf("first acquire failed: %v", err)
			close(firstAcquired)
			return
		}
		firstAcquired <- lease
		<-firstRelease
		lease.Release()
	}()

	first := <-firstAcquired
	if first == nil {
		t.Fatal("first lease never acquired")
	}

	go func() {
		lease, err := manager.Acquire(context.Background(), "p1", "session-b")
		if err != nil {
			t.Errorf("second acquire failed: %v", err)
			close(secondAcquired)
			return
		}
		secondAcquired <- lease
		<-secondRelease
		lease.Release()
	}()

	// While the first holder keeps the lease, the second must stay queued.
	select {
	case lease := <-secondAcquired:
		if lease != nil {
			t.Fatal("second acquirer obtained the lease while the first still held it")
		}
		return
	case <-time.After(50 * time.Millisecond):
	}

	close(firstRelease)
	second := <-secondAcquired
	if second == nil {
		t.Fatal("second lease never acquired after the first released")
	}
	if second.Holder() != "session-b" || second.AcquiredAt().Before(first.AcquiredAt()) {
		t.Fatalf("second lease identity/acquire time unexpected: %+v", second)
	}
	close(secondRelease)

	// After both released, a third acquisition must be immediate.
	third, err := manager.Acquire(context.Background(), "p1", "session-c")
	if err != nil {
		t.Fatalf("third acquire failed: %v", err)
	}
	third.Release()
}

func TestProjectWriteLeaseTTLAutoReleasesQueuedWaiterAndLateComer(t *testing.T) {
	manager := newShortTTLManager()

	stuck, err := manager.Acquire(context.Background(), "p1", "session-stuck")
	if err != nil {
		t.Fatalf("stuck acquire failed: %v", err)
	}
	if !stuck.Deadline().After(time.Now().UTC()) {
		t.Fatalf("lease deadline not in the future: %v", stuck.Deadline())
	}

	waiterDone := make(chan *ProjectWriteLease, 1)
	go func() {
		lease, err := manager.Acquire(context.Background(), "p1", "session-waiter")
		if err != nil {
			t.Errorf("waiter acquire failed: %v", err)
			close(waiterDone)
			return
		}
		waiterDone <- lease
	}()

	// The stuck holder never calls Release; the TTL watchdog must hand the
	// lease to the queued waiter anyway.
	select {
	case lease := <-waiterDone:
		if lease == nil {
			t.Fatal("waiter lease missing")
		}
		if !stuck.Expired() {
			t.Fatal("stuck lease was not marked expired after TTL")
		}
		// The late Release of an expired lease must be a no-op that does not
		// disturb the new holder.
		stuck.Release()
		select {
		case <-waiterDone:
			t.Fatal("waiter lease channel reused")
		default:
		}
		lease.Release()
	case <-time.After(2 * time.Second):
		t.Fatal("TTL watchdog did not release the stuck lease to the queued waiter")
	}

	// A late comer after expiry must take the lease immediately.
	late, err := manager.Acquire(context.Background(), "p1", "session-late")
	if err != nil {
		t.Fatalf("late acquire failed: %v", err)
	}
	late.Release()
}

func TestProjectWriteLeaseWaiterContextCancelReturnsError(t *testing.T) {
	manager := NewProjectWriteLeases()
	holder, err := manager.Acquire(context.Background(), "p1", "session-holder")
	if err != nil {
		t.Fatalf("holder acquire failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelledDone := make(chan error, 1)
	go func() {
		_, err := manager.Acquire(ctx, "p1", "session-cancelled")
		cancelledDone <- err
	}()

	// Let the waiter queue up, then cancel it.
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-cancelledDone:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled waiter must return a wrapped context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled waiter did not return")
	}

	// The lease must still be held by the original holder and transferable.
	holder.Release()
	next, err := manager.Acquire(context.Background(), "p1", "session-next")
	if err != nil {
		t.Fatalf("next acquire failed after cancel + release: %v", err)
	}
	next.Release()
}

func TestProjectWriteLeaseCrossProjectDoesNotMutuallyExclude(t *testing.T) {
	manager := NewProjectWriteLeases()
	first, err := manager.Acquire(context.Background(), "p1", "session-a")
	if err != nil {
		t.Fatalf("first project acquire failed: %v", err)
	}
	defer first.Release()

	// While p1 is held, a different project must acquire without waiting.
	secondDone := make(chan *ProjectWriteLease, 1)
	go func() {
		lease, err := manager.Acquire(context.Background(), "p2", "session-b")
		if err != nil {
			t.Errorf("second project acquire failed: %v", err)
			close(secondDone)
			return
		}
		secondDone <- lease
	}()
	select {
	case lease := <-secondDone:
		if lease == nil {
			t.Fatal("cross-project lease missing")
		}
		if lease.ProjectID() != "p2" {
			t.Fatalf("cross-project lease bound to %q", lease.ProjectID())
		}
		lease.Release()
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cross-project acquire blocked behind an unrelated project lease")
	}
}

func TestProjectWriteLeaseReleaseIsIdempotent(t *testing.T) {
	manager := NewProjectWriteLeases()
	lease, err := manager.Acquire(context.Background(), "p1", "session-a")
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	lease.Release()
	lease.Release()
	lease.Release()

	next, err := manager.Acquire(context.Background(), "p1", "session-b")
	if err != nil {
		t.Fatalf("acquire after idempotent release failed: %v", err)
	}
	next.Release()
}

func TestProjectWriteLeaseRequiresProjectAndRejectsCancelledContext(t *testing.T) {
	manager := NewProjectWriteLeases()
	if _, err := manager.Acquire(context.Background(), "  ", "session-a"); err == nil {
		t.Fatal("empty project id must be rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Acquire(ctx, "p1", "session-a"); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled context must fail acquire, got %v", err)
	}
	if _, err := (*ProjectWriteLeases)(nil).Acquire(context.Background(), "p1", "session-a"); err == nil {
		t.Fatal("nil manager must fail acquire")
	}
}

func TestProjectWriteLeaseEventsFileRecordsTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lease_events.jsonl")
	manager := &ProjectWriteLeases{ttl: time.Hour, projects: map[string]*projectLeaseState{}, eventsPath: path}

	lease, err := manager.Acquire(context.Background(), "p-events", "session-events")
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}
	lease.Release()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("events file was not written: %v", err)
	}
	content := string(data)
	for _, want := range []string{`"event":"acquired"`, `"event":"released"`, `"project_id":"p-events"`, `"holder":"session-events"`} {
		if !strings.Contains(content, want) {
			t.Fatalf("events file missing %s: %s", want, content)
		}
	}
}

// coordinatorLeasePort counts Apply entries and records entry/exit times under
// a mutex so the test can prove that two execution segments for the same
// project never had overlapping mutation windows.
type coordinatorLeasePort struct {
	mu      sync.Mutex
	entered int
	window  []acquisition
	block   chan struct{}
}

func (p *coordinatorLeasePort) Preflight(context.Context, orchestration.ActionSet, orchestration.ProjectCut) error {
	return nil
}

func (p *coordinatorLeasePort) Apply(_ context.Context, _ orchestration.Action, _ string) (orchestration.ActionReceipt, error) {
	p.mu.Lock()
	p.entered++
	enter := time.Now().UTC()
	p.mu.Unlock()
	if p.block != nil {
		<-p.block
	}
	p.mu.Lock()
	p.window = append(p.window, acquisition{enter: enter, exit: time.Now().UTC()})
	p.mu.Unlock()
	return orchestration.ActionReceipt{ActionID: "a1", Status: "applied", EffectivelyOnce: true}, nil
}

func (p *coordinatorLeasePort) enteredCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.entered
}

func TestCoordinatorWriteLeaseSerializesSameProjectExecutions(t *testing.T) {
	store, sessionA, actionSet, cut := authorizedFixture(t, "strong")
	// A second authorized session for the SAME project with its own frozen plan.
	sessionB, err := orchestration.NewSession(sessionA.ID+"-b", sessionA.ProjectUUID, "execute B2 second", orchestration.EngineV1, orchestration.CapabilityInvocation{CapabilityID: actionSet.CapabilityID})
	if err != nil {
		t.Fatal(err)
	}
	sessionB, err = sessionB.SetFrozenPlan(orchestration.FrozenPlan{
		Proposal:  orchestration.Proposal{ID: "prop-b", Revision: 1, CapabilityID: actionSet.CapabilityID, ProjectCutHash: cut.Hash, ActionSetHash: actionSet.Hash},
		ActionSet: actionSet, ProjectCut: cut, PreviousObservationID: "obs-before",
	})
	if err != nil {
		t.Fatal(err)
	}
	decisionB := orchestration.ApprovalDecision{SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove, ProposalID: "prop-b", ProposalRevision: 1, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn-b"}
	sessionB, err = sessionB.Authorize(orchestration.Authorization{ProposalID: "prop-b", ProposalRevision: 1, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn-b", Sequence: 1, Decision: &decisionB})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(sessionB); err != nil {
		t.Fatal(err)
	}

	// Each execution's Apply blocks until its controller releases it, so an
	// unserialized execution would show both windows open at once.
	unblockA := make(chan struct{})
	unblockB := make(chan struct{})
	portA := &coordinatorLeasePort{block: unblockA}
	portB := &coordinatorLeasePort{block: unblockB}

	coordinator := New(store)
	doneA := make(chan error, 1)
	doneB := make(chan error, 1)
	go func() {
		_, err := coordinator.ExecuteWithPersistence(context.Background(), sessionA.ID, actionSet, cut, portA, fakeVerifier{"pass"}, fakePersistence{})
		doneA <- err
	}()
	// Give execution A time to enter (and block inside) its Apply segment.
	time.Sleep(100 * time.Millisecond)
	go func() {
		_, err := coordinator.ExecuteWithPersistence(context.Background(), sessionB.ID, actionSet, cut, portB, fakeVerifier{"pass"}, fakePersistence{})
		doneB <- err
	}()
	time.Sleep(100 * time.Millisecond)

	// While A is blocked mid-mutation, B must not have entered its mutation.
	if portB.enteredCount() != 0 {
		t.Fatal("execution B entered mutation while execution A still held the project lease")
	}
	close(unblockA)
	if err := <-doneA; err != nil {
		t.Fatalf("execution A failed: %v", err)
	}
	// Only after A completed can B enter its mutation window.
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for portB.enteredCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("execution B never reached its mutation window after A released")
		case <-tick.C:
		}
	}
	close(unblockB)
	if err := <-doneB; err != nil {
		t.Fatalf("execution B failed: %v", err)
	}

	portA.mu.Lock()
	windows := append([]acquisition(nil), portA.window...)
	portA.mu.Unlock()
	portB.mu.Lock()
	windows = append(windows, portB.window...)
	portB.mu.Unlock()
	for index := 1; index < len(windows); index++ {
		if windows[index].enter.Before(windows[index-1].exit) {
			t.Fatalf("mutation windows overlapped across executions: %+v", windows)
		}
	}
}

func TestCoordinatorWithoutWriteLeasesStillExecutes(t *testing.T) {
	// Struct-literal construction without WriteLeases keeps the pre-lease
	// behavior (backward compatibility for embedders).
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	coordinator := &Coordinator{Store: store, Now: func() time.Time { return time.Now().UTC() }}
	port := &fakePort{}
	if _, err := coordinator.ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, port, fakeVerifier{"pass"}, fakePersistence{}); err != nil {
		t.Fatalf("lease-free execution failed: %v", err)
	}
	if len(port.Applied) != 2 {
		t.Fatalf("lease-free execution did not apply all actions: %#v", port.Applied)
	}
}
