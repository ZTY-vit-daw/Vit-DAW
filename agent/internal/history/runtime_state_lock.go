package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrAgentRuntimeStateLocked reports that another owner currently holds the
// project-scoped agent runtime state lease.
var ErrAgentRuntimeStateLocked = errors.New("agent runtime state locked by another owner")

const agentRuntimeStateLockFile = "agent_runtime_state.lock.json"

// AgentRuntimeStateLock is a project-scoped advisory lease that serializes
// durable runtime-state writes between the chat server and the continuation
// scheduler (and, after a restart, between processes). The lease expires so a
// crashed owner cannot deadlock the project forever.
type AgentRuntimeStateLock struct {
	path    string
	owner   string
	expires time.Time
}

type agentRuntimeStateLease struct {
	Owner      string    `json:"owner"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func agentRuntimeStateLockPath(projectPath, projectUUID string) (string, error) {
	projectPath = BindProjectIdentity(projectPath, projectUUID)
	repo, err := Open(projectPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(repo.StateDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(repo.StateDir, agentRuntimeStateLockFile), nil
}

// AcquireAgentRuntimeStateLock takes the runtime-state lease for owner. An
// expired lease is recovered transparently; a live lease held by another owner
// returns ErrAgentRuntimeStateLocked. Re-acquiring as the current owner
// refreshes the lease.
func AcquireAgentRuntimeStateLock(projectPath, projectUUID, owner string, lease time.Duration) (*AgentRuntimeStateLock, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, errors.New("agent runtime state lock requires an owner")
	}
	if lease <= 0 {
		lease = time.Minute
	}
	path, err := agentRuntimeStateLockPath(projectPath, projectUUID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if existing := readAgentRuntimeLease(path); existing != nil && existing.Owner != owner && now.Before(existing.ExpiresAt) {
		return nil, fmt.Errorf("%w: owner=%s expires_at=%s", ErrAgentRuntimeStateLocked, existing.Owner, existing.ExpiresAt.Format(time.RFC3339))
	}
	lock := &AgentRuntimeStateLock{path: path, owner: owner, expires: now.Add(lease)}
	if err := writeJSON(path, &agentRuntimeStateLease{Owner: owner, AcquiredAt: now, ExpiresAt: lock.expires}); err != nil {
		return nil, err
	}
	return lock, nil
}

// Release drops the lease. Releasing a lease held by a different owner is
// rejected with ErrAgentRuntimeStateLocked; releasing an unset path is a no-op
// so a crash-simulated holder stays silent.
func (l *AgentRuntimeStateLock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	existing := readAgentRuntimeLease(l.path)
	if existing != nil && existing.Owner != l.owner {
		return fmt.Errorf("%w: cannot release lease owned by %s", ErrAgentRuntimeStateLocked, existing.Owner)
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readAgentRuntimeLease(path string) *agentRuntimeStateLease {
	lease := agentRuntimeStateLease{}
	if err := readJSON(path, &lease); err != nil {
		return nil
	}
	return &lease
}
