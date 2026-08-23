package history

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func runtimeLockFixture(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.vit")
	if err := os.WriteFile(path, []byte("identity only"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, "vitproj_runtime_lock_test"
}

func TestAgentRuntimeStateLockExcludesConcurrentOwners(t *testing.T) {
	projectPath, projectUUID := runtimeLockFixture(t)
	first, err := AcquireAgentRuntimeStateLock(projectPath, projectUUID, "owner-a", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := AcquireAgentRuntimeStateLock(projectPath, projectUUID, "owner-b", time.Minute); !errors.Is(err, ErrAgentRuntimeStateLocked) {
		t.Fatalf("second owner error = %v, want ErrAgentRuntimeStateLocked", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireAgentRuntimeStateLock(projectPath, projectUUID, "owner-b", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAgentRuntimeStateLockRejectsNonOwnerRelease(t *testing.T) {
	projectPath, projectUUID := runtimeLockFixture(t)
	owner, err := AcquireAgentRuntimeStateLock(projectPath, projectUUID, "owner-a", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Release()
	forged := &AgentRuntimeStateLock{path: owner.path, owner: "owner-b"}
	if err := forged.Release(); !errors.Is(err, ErrAgentRuntimeStateLocked) {
		t.Fatalf("forged release error = %v, want ErrAgentRuntimeStateLocked", err)
	}
	if _, err := AcquireAgentRuntimeStateLock(projectPath, projectUUID, "owner-c", time.Minute); !errors.Is(err, ErrAgentRuntimeStateLocked) {
		t.Fatalf("lock was released by non-owner: %v", err)
	}
}

func TestAgentRuntimeStateLockRecoversExpiredLease(t *testing.T) {
	projectPath, projectUUID := runtimeLockFixture(t)
	first, err := AcquireAgentRuntimeStateLock(projectPath, projectUUID, "crashed-owner", 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a process crash by leaving the directory in place and not
	// releasing the lease.
	first.path = ""
	time.Sleep(50 * time.Millisecond)
	second, err := AcquireAgentRuntimeStateLock(projectPath, projectUUID, "recovered-owner", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}
