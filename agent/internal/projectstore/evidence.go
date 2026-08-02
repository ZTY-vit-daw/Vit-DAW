package projectstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	EvidenceIndexSchema = "evidence_index.v1"
	EvidenceBlobSchema  = "evidence_blob.v1"
	EvidenceIndexFile   = "index.json"
)

var ErrEvidenceDisabled = errors.New("project evidence store is disabled")

type EvidenceEntry struct {
	Kind             string    `json:"kind"`
	Size             int64     `json:"size"`
	CreatedAt        time.Time `json:"created_at"`
	LastReferencedAt time.Time `json:"last_referenced_at"`
	Refs             []string  `json:"refs,omitempty"`
	Renewable        bool      `json:"renewable"`
}

type EvidenceIndex struct {
	SchemaVersion string                   `json:"schema_version"`
	ProjectUUID   string                   `json:"project_uuid"`
	Entries       map[string]EvidenceEntry `json:"entries"`
}

type EvidenceBlob struct {
	SchemaVersion string `json:"schema_version"`
	ProjectUUID   string `json:"project_uuid"`
	Kind          string `json:"kind"`
	SHA256        string `json:"sha256"`
	Content       any    `json:"content"`
}

var evidenceLocks sync.Map

func PutEvidence(roots Roots, kind string, content any, refs ...string) (string, int64, error) {
	if roots.Agent == "" || roots.ProjectUUID == "" {
		return "", 0, errors.New("evidence store requires project roots")
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "", 0, errors.New("evidence kind is required")
	}
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return "", 0, err
	}
	digest := sha256.Sum256(contentBytes)
	hash := hex.EncodeToString(digest[:])
	lock := evidenceLock(roots.Agent)
	lock.Lock()
	defer lock.Unlock()

	manifest, err := Load(roots)
	if err != nil {
		return "", 0, err
	}
	if manifest.Degraded.EvidenceOff {
		return "", 0, ErrEvidenceDisabled
	}
	blob := EvidenceBlob{
		SchemaVersion: EvidenceBlobSchema,
		ProjectUUID:   roots.ProjectUUID,
		Kind:          kind,
		SHA256:        hash,
		Content:       content,
	}
	blobBytes, err := json.Marshal(blob)
	if err != nil {
		return "", 0, err
	}
	if int64(len(blobBytes)) > manifest.Budgets.EvidenceBlobMaxBytes {
		manifest.GateAudit = append(manifest.GateAudit, GateAuditEntry{At: time.Now().UTC(), Action: "reject_evidence_blob", Detail: kind + ":blob_too_large"})
		_ = Write(roots, manifest)
		return "", int64(len(blobBytes)), errors.New("evidence blob exceeds project limit")
	}
	index, err := loadEvidenceIndex(roots)
	if err != nil {
		return "", 0, err
	}
	now := time.Now().UTC()
	if existing, ok := index.Entries[hash]; ok {
		existing.LastReferencedAt = now
		existing.Refs = uniqueStrings(append(existing.Refs, refs...))
		index.Entries[hash] = existing
		if err := writeEvidenceIndex(roots, index); err != nil {
			return "", 0, err
		}
		return "evidence://" + hash, existing.Size, nil
	}
	if err := makeEvidenceRoom(roots, &manifest, &index, int64(len(blobBytes))); err != nil {
		return "", int64(len(blobBytes)), err
	}
	path := evidenceObjectPath(roots, hash)
	if err := writeFileAtomic(path, blobBytes, 0o600); err != nil {
		return "", 0, err
	}
	index.Entries[hash] = EvidenceEntry{
		Kind: kind, Size: int64(len(blobBytes)), CreatedAt: now, LastReferencedAt: now,
		Refs: uniqueStrings(refs), Renewable: true,
	}
	if err := writeEvidenceIndex(roots, index); err != nil {
		_ = os.Remove(path)
		return "", 0, err
	}
	_, _ = Recalibrate(roots)
	return "evidence://" + hash, int64(len(blobBytes)), nil
}

func GetEvidence(roots Roots, ref string) (EvidenceBlob, error) {
	hash := strings.TrimPrefix(strings.TrimSpace(ref), "evidence://")
	if len(hash) != 64 {
		return EvidenceBlob{}, errors.New("invalid evidence reference")
	}
	data, err := os.ReadFile(evidenceObjectPath(roots, hash))
	if err != nil {
		return EvidenceBlob{}, err
	}
	blob := EvidenceBlob{}
	if err := json.Unmarshal(data, &blob); err != nil {
		return EvidenceBlob{}, err
	}
	if blob.SchemaVersion != EvidenceBlobSchema || blob.ProjectUUID != roots.ProjectUUID || blob.SHA256 != hash {
		return EvidenceBlob{}, errors.New("evidence identity mismatch")
	}
	return blob, nil
}

func loadEvidenceIndex(roots Roots) (EvidenceIndex, error) {
	path := filepath.Join(roots.Agent, "evidence", EvidenceIndexFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return EvidenceIndex{SchemaVersion: EvidenceIndexSchema, ProjectUUID: roots.ProjectUUID, Entries: map[string]EvidenceEntry{}}, nil
		}
		return EvidenceIndex{}, err
	}
	index := EvidenceIndex{}
	if err := json.Unmarshal(data, &index); err != nil {
		return EvidenceIndex{}, err
	}
	if index.SchemaVersion != EvidenceIndexSchema || index.ProjectUUID != roots.ProjectUUID {
		return EvidenceIndex{}, errors.New("evidence index identity mismatch")
	}
	if index.Entries == nil {
		index.Entries = map[string]EvidenceEntry{}
	}
	return index, nil
}

func writeEvidenceIndex(roots Roots, index EvidenceIndex) error {
	index.SchemaVersion = EvidenceIndexSchema
	index.ProjectUUID = roots.ProjectUUID
	if index.Entries == nil {
		index.Entries = map[string]EvidenceEntry{}
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(roots.Agent, "evidence", EvidenceIndexFile), data, 0o600)
}

func makeEvidenceRoom(roots Roots, manifest *Manifest, index *EvidenceIndex, required int64) error {
	current := int64(0)
	for _, entry := range index.Entries {
		current += entry.Size
	}
	limit := manifest.Budgets.EvidenceMaxBytes
	if limit <= 0 || current+required <= limit {
		return nil
	}
	type candidate struct {
		hash  string
		entry EvidenceEntry
	}
	candidates := make([]candidate, 0, len(index.Entries))
	decisionObservations, _ := pinnedObservationIDs(roots.Agent)
	for hash, entry := range index.Entries {
		if !entry.Renewable || evidencePinned(entry.Refs) || evidencePinnedByObservation(entry.Refs, decisionObservations) || evidenceReferencedByDecision(roots.Agent, hash) {
			continue
		}
		candidates = append(candidates, candidate{hash: hash, entry: entry})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].entry.LastReferencedAt.Before(candidates[j].entry.LastReferencedAt)
	})
	freed := int64(0)
	for _, candidate := range candidates {
		if current-freed+required <= limit {
			break
		}
		if err := os.Remove(evidenceObjectPath(roots, candidate.hash)); err != nil && !os.IsNotExist(err) {
			return err
		}
		delete(index.Entries, candidate.hash)
		freed += candidate.entry.Size
	}
	if current-freed+required > limit {
		manifest.Degraded = DegradedState{EvidenceOff: true, Reason: "evidence_budget_exhausted"}
		manifest.GateAudit = append(manifest.GateAudit, GateAuditEntry{At: time.Now().UTC(), Action: "enable_evidence_off", Detail: "no renewable evidence can satisfy budget", FreedBytes: freed})
		_ = writeEvidenceIndex(roots, *index)
		_ = Write(roots, *manifest)
		return ErrEvidenceDisabled
	}
	manifest.GateAudit = append(manifest.GateAudit, GateAuditEntry{At: time.Now().UTC(), Action: "evict_evidence", Detail: "evidence budget", FreedBytes: freed})
	if err := writeEvidenceIndex(roots, *index); err != nil {
		return err
	}
	return Write(roots, *manifest)
}

func evidencePinnedByObservation(refs []string, pinned map[string]bool) bool {
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if strings.HasPrefix(ref, "obs:") && pinned[strings.TrimSpace(strings.TrimPrefix(ref, "obs:"))] {
			return true
		}
	}
	return false
}

func evidenceReferencedByDecision(agentRoot, hash string) bool {
	root := filepath.Join(agentRoot, "mixboard", "decisions")
	found := false
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if found || err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil && strings.Contains(string(data), hash) {
			found = true
		}
		return nil
	})
	return found
}

func evidenceObjectPath(roots Roots, hash string) string {
	return filepath.Join(roots.Agent, "evidence", "objects", hash[:2], hash+".json")
}

func evidenceLock(root string) *sync.Mutex {
	value, _ := evidenceLocks.LoadOrStore(filepath.Clean(root), &sync.Mutex{})
	return value.(*sync.Mutex)
}

func evidencePinned(refs []string) bool {
	for _, ref := range refs {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(ref)), "decision:") {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
