package collaboration

import (
	"testing"
	"time"
)

func reservationFixture() WorktreeReservation {
	return WorktreeReservation{
		ProjectUUID: "project-1", WorktreeRef: "vocal-natural-a31f", WorktreePath: `D:\Worktrees\vocal.vit`,
		OwnerAgentID: "agent-a", OwnerGoalID: "goal-a", OwnerRunID: "run-a", OwnerConversationID: "conversation-a",
		ParentNodeID: "node-1", ParentCommitID: "commit-1", Purpose: "natural vocal direction",
	}
}

func TestRegistryReservesOneWriterPerWorktreeAndReleases(t *testing.T) {
	registry := NewRegistry()
	first, err := registry.Reserve(reservationFixture())
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.Status != ReservationReserved || first.SchemaVersion != SchemaVersion {
		t.Fatalf("reservation=%+v", first)
	}
	if _, err := registry.Reserve(WorktreeReservation{ProjectUUID: "project-1", WorktreeRef: "vocal-natural-a31f", WorktreePath: first.WorktreePath, OwnerAgentID: "agent-b", OwnerGoalID: "goal-b", OwnerRunID: "run-b", OwnerConversationID: "conversation-b"}); err == nil {
		t.Fatal("second writer was accepted")
	}
	if err := registry.AuthorizeWrite("project-1", first.WorktreeRef, "agent-b", "goal-b", "run-b"); err == nil {
		t.Fatal("non-owner write was authorized")
	}
	if err := registry.AuthorizeWrite("project-1", first.WorktreeRef, "agent-a", "goal-a", "run-a"); err != nil {
		t.Fatal(err)
	}
	released, err := registry.Release(first.ID, "agent-a", "goal-a", "run-a", time.Unix(10, 0))
	if err != nil || released.Status != ReservationReleased {
		t.Fatalf("released=%+v err=%v", released, err)
	}
	if _, err := registry.Reserve(WorktreeReservation{ProjectUUID: "project-1", WorktreeRef: first.WorktreeRef, WorktreePath: first.WorktreePath, OwnerAgentID: "agent-b", OwnerGoalID: "goal-b", OwnerRunID: "run-b", OwnerConversationID: "conversation-b"}); err != nil {
		t.Fatalf("worktree was not reusable after release: %v", err)
	}
}

func TestRegistryRenewAndRecoverStaleKeepsHistory(t *testing.T) {
	registry := NewRegistry()
	reservation, err := registry.Reserve(reservationFixture())
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Unix(20, 0).UTC()
	active, err := registry.Renew(reservation.ID, "agent-a", "goal-a", "run-a", expires, time.Unix(10, 0))
	if err != nil || active.Status != ReservationActive || !active.ExpiresAt.Equal(expires) {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	stale := registry.RecoverStale(time.Unix(21, 0))
	if len(stale) != 1 || stale[0].Status != ReservationStale {
		t.Fatalf("stale=%+v", stale)
	}
	stored, ok := registry.Get(reservation.ID)
	if !ok || stored.Status != ReservationStale {
		t.Fatalf("stale reservation was deleted: %+v ok=%v", stored, ok)
	}
}

func TestRegistrySnapshotRestoresReservationsAndChildTasks(t *testing.T) {
	registry := NewRegistry()
	reservation, err := registry.Reserve(reservationFixture())
	if err != nil {
		t.Fatal(err)
	}
	child, err := registry.RegisterChildTask(ChildTask{ParentAgentID: "agent-parent", AgentID: "agent-child", GoalID: "goal-child", RunID: "run-child", ConversationID: "conversation-child", WorktreeRef: reservation.WorktreeRef, ReservationID: reservation.ID, Status: ChildTaskRunning})
	if err != nil {
		t.Fatal(err)
	}
	restored := NewRegistry()
	if err := restored.Restore(registry.Snapshot()); err != nil {
		t.Fatal(err)
	}
	gotReservation, ok := restored.Get(reservation.ID)
	if !ok || gotReservation.WorktreeRef != reservation.WorktreeRef {
		t.Fatalf("reservation restore=%+v ok=%v", gotReservation, ok)
	}
	children := restored.ListChildTasks("agent-parent")
	if len(children) != 1 || children[0].ID != child.ID || children[0].ReservationID != reservation.ID {
		t.Fatalf("children restore=%+v", children)
	}
}

func TestRegistryExplicitTakeoverAndDisposition(t *testing.T) {
	registry := NewRegistry()
	reservation, err := registry.Reserve(reservationFixture())
	if err != nil {
		t.Fatal(err)
	}
	taken, err := registry.Takeover(reservation.ID, "agent-b", "goal-b", "run-b", "conversation-b", time.Unix(20, 0))
	if err != nil || taken.OwnerAgentID != "agent-b" || taken.Status != ReservationActive {
		t.Fatalf("taken=%+v err=%v", taken, err)
	}
	promoted, err := registry.SetDisposition(reservation.ID, DispositionPromote, time.Unix(30, 0))
	if err != nil || promoted.Status != ReservationReleased || promoted.Disposition != DispositionPromote {
		t.Fatalf("promoted=%+v err=%v", promoted, err)
	}
	if _, ok := registry.ActiveReservation(promoted.ProjectUUID, promoted.WorktreeRef); ok {
		t.Fatal("promoted worktree retained an active writer")
	}
	continued, err := registry.SetDisposition(reservation.ID, DispositionContinue, time.Unix(40, 0))
	if err != nil || continued.Status != ReservationActive || continued.Disposition != DispositionContinue {
		t.Fatalf("continued=%+v err=%v", continued, err)
	}
}
