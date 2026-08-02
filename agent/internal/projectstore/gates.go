package projectstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EnforceBudgets applies only renewable-data gates. Authoritative journal and
// decision records are never removed automatically. The operation is best
// effort and may enter evidence_off when no safe renewable data remains.
func EnforceBudgets(roots Roots) (Manifest, error) {
	manifest, err := Load(roots)
	if err != nil {
		return Manifest{}, err
	}
	if err := evictObservations(roots, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.Budgets.StoreTotalMaxBytes > 0 {
		if counters, scanErr := scanCounters(roots.Agent); scanErr == nil && counters.TotalBytes > manifest.Budgets.StoreTotalMaxBytes {
			if err := evictEvidenceForTotal(roots, &manifest, counters.TotalBytes-manifest.Budgets.StoreTotalMaxBytes); err != nil {
				return Manifest{}, err
			}
		}
	}
	if err := evictContextPacks(roots, &manifest); err != nil {
		return Manifest{}, err
	}
	counters, err := scanCounters(roots.Agent)
	if err != nil {
		return Manifest{}, err
	}
	manifest.Counters = counters
	if manifest.Budgets.StoreTotalMaxBytes > 0 && counters.TotalBytes > manifest.Budgets.StoreTotalMaxBytes {
		manifest.Degraded = DegradedState{EvidenceOff: true, Reason: "store_total_budget_exhausted"}
		manifest.GateAudit = append(manifest.GateAudit, GateAuditEntry{
			At: time.Now().UTC(), Action: "enable_evidence_off",
			Detail: "store total budget remains above limit after renewable eviction",
		})
	}
	if err := Write(roots, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func evictEvidenceForTotal(roots Roots, manifest *Manifest, requiredFree int64) error {
	if requiredFree <= 0 {
		return nil
	}
	lock := evidenceLock(roots.Agent)
	lock.Lock()
	defer lock.Unlock()
	index, err := loadEvidenceIndex(roots)
	if err != nil {
		return err
	}
	type candidate struct {
		hash  string
		entry EvidenceEntry
	}
	var candidates []candidate
	decisionObservations, _ := pinnedObservationIDs(roots.Agent)
	for hash, entry := range index.Entries {
		if entry.Renewable && !evidencePinned(entry.Refs) && !evidencePinnedByObservation(entry.Refs, decisionObservations) && !evidenceReferencedByDecision(roots.Agent, hash) {
			candidates = append(candidates, candidate{hash: hash, entry: entry})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].entry.LastReferencedAt.Before(candidates[j].entry.LastReferencedAt)
	})
	freed := int64(0)
	for _, candidate := range candidates {
		if freed >= requiredFree {
			break
		}
		if err := os.Remove(evidenceObjectPath(roots, candidate.hash)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		delete(index.Entries, candidate.hash)
		freed += candidate.entry.Size
	}
	if freed == 0 {
		return nil
	}
	if err := writeEvidenceIndex(roots, index); err != nil {
		return err
	}
	manifest.GateAudit = append(manifest.GateAudit, GateAuditEntry{
		At: time.Now().UTC(), Action: "evict_evidence",
		Detail: "store total budget", FreedBytes: freed,
	})
	return nil
}

type observationCandidate struct {
	path string
	id   string
	when time.Time
}

func evictObservations(roots Roots, manifest *Manifest) error {
	limit := manifest.Budgets.ObservationMaxCount
	if limit <= 0 {
		return nil
	}
	dir := filepath.Join(roots.Agent, "observations")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pinned, err := pinnedObservationIDs(roots.Agent)
	if err != nil {
		return err
	}
	candidates := make([]observationCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		var envelope struct {
			ObservationID string `json:"observation_id"`
		}
		if data, readErr := os.ReadFile(path); readErr == nil {
			_ = json.Unmarshal(data, &envelope)
			if strings.TrimSpace(envelope.ObservationID) != "" {
				id = envelope.ObservationID
			}
		}
		if pinned[id] {
			continue
		}
		candidates = append(candidates, observationCandidate{path: path, id: id, when: info.ModTime()})
	}
	if int64(len(candidates))+int64(len(pinned)) <= limit {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].when.Before(candidates[j].when) })
	removeCount := int64(len(candidates)) + int64(len(pinned)) - limit
	if removeCount < 0 {
		removeCount = 0
	}
	freed := int64(0)
	removed := int64(0)
	for _, candidate := range candidates {
		if removed >= removeCount {
			break
		}
		if info, statErr := os.Stat(candidate.path); statErr == nil {
			freed += info.Size()
		}
		if removeErr := os.Remove(candidate.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		removed++
	}
	if removed > 0 {
		manifest.GateAudit = append(manifest.GateAudit, GateAuditEntry{
			At: time.Now().UTC(), Action: "evict_observations",
			Detail: "oldest unpinned observations", FreedBytes: freed,
		})
	}
	return nil
}

func evictContextPacks(roots Roots, manifest *Manifest) error {
	limit := manifest.Budgets.StoreTotalMaxBytes
	if limit <= 0 {
		return nil
	}
	counters, err := scanCounters(roots.Agent)
	if err != nil {
		return err
	}
	if counters.TotalBytes <= limit {
		return nil
	}
	type candidate struct {
		path string
		size int64
		when time.Time
	}
	var candidates []candidate
	root := filepath.Join(roots.Agent, "mixboard", "sessions")
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.EqualFold(entry.Name(), "context_pack.json") {
			return walkErr
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			candidates = append(candidates, candidate{path: path, size: info.Size(), when: info.ModTime()})
		}
		return nil
	})
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].when.Before(candidates[j].when) })
	freed := int64(0)
	for _, candidate := range candidates {
		if counters.TotalBytes-freed <= limit {
			break
		}
		if err := os.Remove(candidate.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		freed += candidate.size
	}
	if freed > 0 {
		manifest.GateAudit = append(manifest.GateAudit, GateAuditEntry{
			At: time.Now().UTC(), Action: "evict_context_packs",
			Detail: "oldest renewable context packs", FreedBytes: freed,
		})
	}
	return nil
}

func pinnedObservationIDs(agentRoot string) (map[string]bool, error) {
	pinned := map[string]bool{}
	root := filepath.Join(agentRoot, "mixboard", "decisions")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			return walkErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var value any
		if json.Unmarshal(data, &value) != nil {
			return nil
		}
		collectObservationRefs(value, pinned)
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return pinned, nil
	}
	return pinned, err
}

func collectObservationRefs(value any, pinned map[string]bool) {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		for _, prefix := range []string{"mix.observe:", "observation_id:"} {
			if strings.HasPrefix(text, prefix) {
				id := strings.TrimSpace(strings.TrimPrefix(text, prefix))
				if id != "" {
					pinned[id] = true
				}
			}
		}
	case []any:
		for _, child := range typed {
			collectObservationRefs(child, pinned)
		}
	case map[string]any:
		for _, child := range typed {
			collectObservationRefs(child, pinned)
		}
	}
}
