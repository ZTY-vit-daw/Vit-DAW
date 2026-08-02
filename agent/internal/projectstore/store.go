package projectstore

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	AgentDirName          = ".vit_agent"
	HistoryDirName        = ".vit_history"
	DerivedDirName        = ".vit_derived"
	ManifestFile          = "manifest.v2.json"
	ManifestSchemaVersion = "vit_agent_store_manifest.v2"
)

type Roots struct {
	ProjectPath string
	ProjectUUID string
	ProjectDir  string
	History     string
	Agent       string
	Derived     string
}

type Budgets struct {
	ObservationMaxBytes    int64 `json:"observation_max_bytes"`
	ObservationMaxCount    int64 `json:"observation_max_count"`
	EvidenceMaxBytes       int64 `json:"evidence_max_bytes"`
	EvidenceBlobMaxBytes   int64 `json:"evidence_blob_max_bytes"`
	JournalShardMaxBytes   int64 `json:"journal_shard_max_bytes"`
	JournalShardMaxRecords int64 `json:"journal_shard_max_records"`
	JournalMaxShards       int64 `json:"journal_max_shards"`
	StoreTotalMaxBytes     int64 `json:"store_total_max_bytes"`
}

type Counters struct {
	JournalBytes      int64 `json:"journal_bytes"`
	JournalRecords    int64 `json:"journal_records"`
	JournalShards     int64 `json:"journal_shards"`
	ObservationsBytes int64 `json:"observations_bytes"`
	ObservationsCount int64 `json:"observations_count"`
	EvidenceBytes     int64 `json:"evidence_bytes"`
	EvidenceCount     int64 `json:"evidence_count"`
	MixboardBytes     int64 `json:"mixboard_bytes"`
	TotalBytes        int64 `json:"total_bytes"`
}

type DegradedState struct {
	EvidenceOff bool   `json:"evidence_off"`
	Reason      string `json:"reason"`
}

type GateAuditEntry struct {
	At         time.Time `json:"at"`
	Action     string    `json:"action"`
	Detail     string    `json:"detail,omitempty"`
	FreedBytes int64     `json:"freed_bytes,omitempty"`
}

type Manifest struct {
	SchemaVersion     string              `json:"schema_version"`
	ProjectUUID       string              `json:"project_uuid"`
	OriginProjectUUID string              `json:"origin_project_uuid,omitempty"`
	ProjectPath       string              `json:"project_path"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	Budgets           Budgets             `json:"budgets"`
	Counters          Counters            `json:"counters"`
	Classes           map[string][]string `json:"classes"`
	Degraded          DegradedState       `json:"degraded"`
	GateAudit         []GateAuditEntry    `json:"gate_audit,omitempty"`
}

var active struct {
	sync.RWMutex
	roots Roots
	set   bool
}

func DefaultBudgets() Budgets {
	return Budgets{
		ObservationMaxBytes:    2 * 1024 * 1024,
		ObservationMaxCount:    500,
		EvidenceMaxBytes:       400 * 1024 * 1024,
		EvidenceBlobMaxBytes:   64 * 1024 * 1024,
		JournalShardMaxBytes:   4 * 1024 * 1024,
		JournalShardMaxRecords: 1000,
		JournalMaxShards:       50,
		StoreTotalMaxBytes:     512 * 1024 * 1024,
	}
}

func Resolve(projectPath, projectUUID string) (Roots, error) {
	projectPath = strings.TrimSpace(projectPath)
	projectUUID = SafeName(projectUUID)
	if projectPath == "" || projectUUID == "" {
		return Roots{}, errors.New("project store requires project path and UUID")
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return Roots{}, err
	}
	abs = filepath.Clean(abs)
	dir := filepath.Dir(abs)
	return Roots{
		ProjectPath: abs,
		ProjectUUID: projectUUID,
		ProjectDir:  dir,
		History:     filepath.Join(dir, HistoryDirName, projectUUID),
		Agent:       filepath.Join(dir, AgentDirName, projectUUID),
		Derived:     filepath.Join(dir, DerivedDirName, projectUUID),
	}, nil
}

func Ensure(projectPath, projectUUID string) (Roots, Manifest, error) {
	roots, err := Resolve(projectPath, projectUUID)
	if err != nil {
		return Roots{}, Manifest{}, err
	}
	if err := os.MkdirAll(roots.Agent, 0o755); err != nil {
		return Roots{}, Manifest{}, err
	}
	manifest, err := Load(roots)
	if err != nil && !os.IsNotExist(err) {
		if rebound, ok, rebindErr := rebindRelocatedStore(roots); ok {
			if rebindErr != nil {
				return Roots{}, Manifest{}, rebindErr
			}
			manifest = rebound
			err = nil
		}
	}
	if os.IsNotExist(err) {
		now := time.Now().UTC()
		manifest = Manifest{
			SchemaVersion: ManifestSchemaVersion,
			ProjectUUID:   roots.ProjectUUID,
			ProjectPath:   roots.ProjectPath,
			CreatedAt:     now,
			UpdatedAt:     now,
			Budgets:       DefaultBudgets(),
			Classes: map[string][]string{
				"authoritative": {"journal", "mixboard_decisions"},
				"renewable":     {"evidence", "context_packs"},
			},
		}
		if err := Write(roots, manifest); err != nil {
			return Roots{}, Manifest{}, err
		}
		return roots, manifest, nil
	}
	if err != nil {
		return Roots{}, Manifest{}, err
	}
	return roots, manifest, nil
}

// rebindRelocatedStore accepts a project folder that was copied or moved as a
// unit. The store is already scoped by the same project UUID and lives beside
// the opened .vit file; only its persisted absolute project path is stale.
// Rebinding the copied tree keeps the original folder untouched while making
// the local store usable at its new location.
func rebindRelocatedStore(roots Roots) (Manifest, bool, error) {
	data, err := os.ReadFile(filepath.Join(roots.Agent, ManifestFile))
	if err != nil {
		return Manifest{}, false, nil
	}
	manifest := Manifest{}
	if err := json.Unmarshal(data, &manifest); err != nil ||
		manifest.SchemaVersion != ManifestSchemaVersion ||
		SafeName(manifest.ProjectUUID) != roots.ProjectUUID {
		return Manifest{}, false, nil
	}
	oldPath := strings.TrimSpace(manifest.ProjectPath)
	if oldPath == "" || samePath(oldPath, roots.ProjectPath) {
		return Manifest{}, false, nil
	}
	if err := RebindProjectTree(roots.Agent, oldPath, roots.ProjectUUID, roots.ProjectPath, roots.ProjectUUID); err != nil {
		return Manifest{}, true, err
	}
	rebound, err := Load(roots)
	return rebound, true, err
}

func Activate(projectPath, projectUUID string) (Roots, Manifest, error) {
	roots, manifest, err := Ensure(projectPath, projectUUID)
	if err != nil {
		return Roots{}, Manifest{}, err
	}
	active.Lock()
	active.roots = roots
	active.set = true
	active.Unlock()
	// Startup calibration and renewable-data gates are deliberately best
	// effort: a corrupt/overfull Agent store must never prevent the .vit
	// project itself from opening or being saved.
	if calibrated, calibrateErr := Recalibrate(roots); calibrateErr == nil {
		manifest = calibrated
	}
	if gated, gateErr := EnforceBudgets(roots); gateErr == nil {
		manifest = gated
	}
	return roots, manifest, nil
}

func Current() (Roots, bool) {
	active.RLock()
	defer active.RUnlock()
	return active.roots, active.set
}

func Deactivate() {
	active.Lock()
	active.roots = Roots{}
	active.set = false
	active.Unlock()
}

func Load(roots Roots) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(roots.Agent, ManifestFile))
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != ManifestSchemaVersion || SafeName(manifest.ProjectUUID) != roots.ProjectUUID {
		return Manifest{}, errors.New("agent store manifest identity mismatch")
	}
	manifestPath, err := filepath.Abs(strings.TrimSpace(manifest.ProjectPath))
	if err != nil || !samePath(manifestPath, roots.ProjectPath) {
		return Manifest{}, errors.New("agent store manifest project path mismatch")
	}
	return manifest, nil
}

func Write(roots Roots, manifest Manifest) error {
	if roots.ProjectUUID == "" || roots.Agent == "" {
		return errors.New("agent store roots are incomplete")
	}
	manifest.SchemaVersion = ManifestSchemaVersion
	manifest.ProjectUUID = roots.ProjectUUID
	manifest.ProjectPath = roots.ProjectPath
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = time.Now().UTC()
	}
	manifest.UpdatedAt = time.Now().UTC()
	if manifest.Budgets.ObservationMaxBytes <= 0 {
		manifest.Budgets = DefaultBudgets()
	}
	if len(manifest.GateAudit) > 100 {
		manifest.GateAudit = append([]GateAuditEntry(nil), manifest.GateAudit[len(manifest.GateAudit)-100:]...)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(roots.Agent, 0o755); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(roots.Agent, ManifestFile), data, 0o600)
}

func Recalibrate(roots Roots) (Manifest, error) {
	manifest, err := Load(roots)
	if err != nil {
		return Manifest{}, err
	}
	counters, err := scanCounters(roots.Agent)
	if err != nil {
		return Manifest{}, err
	}
	manifest.Counters = counters
	if err := Write(roots, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func RecordGateAudit(roots Roots, entry GateAuditEntry) error {
	manifest, err := Load(roots)
	if err != nil {
		return err
	}
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}
	manifest.GateAudit = append(manifest.GateAudit, entry)
	return Write(roots, manifest)
}

func ForkAgentStore(sourcePath, sourceUUID, targetPath, targetUUID string) (string, error) {
	source, err := Resolve(sourcePath, sourceUUID)
	if err != nil {
		return "", err
	}
	target, err := Resolve(targetPath, targetUUID)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(source.Agent); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		_, _, ensureErr := Ensure(targetPath, targetUUID)
		return target.Agent, ensureErr
	}
	if manifest, loadErr := Load(target); loadErr == nil {
		if manifest.ProjectUUID == target.ProjectUUID && samePath(manifest.ProjectPath, target.ProjectPath) {
			return target.Agent, nil
		}
		return "", errors.New("target agent store already exists with a different identity")
	} else if !os.IsNotExist(loadErr) {
		return "", fmt.Errorf("target agent store is not reusable: %w", loadErr)
	}
	if err := os.MkdirAll(filepath.Dir(target.Agent), 0o755); err != nil {
		return "", err
	}
	staging := filepath.Join(filepath.Dir(target.Agent), "."+filepath.Base(target.Agent)+".staging_"+fmt.Sprint(time.Now().UnixNano()))
	if err := copyDir(source.Agent, staging); err != nil {
		return "", err
	}
	cleanup := func() { _ = os.RemoveAll(staging) }
	if err := rebindTree(staging, source, target); err != nil {
		cleanup()
		return "", err
	}
	stagedRoots := target
	stagedRoots.Agent = staging
	manifest, err := LoadWithExpected(stagedRoots, target.ProjectUUID, target.ProjectPath)
	if err != nil {
		cleanup()
		return "", err
	}
	manifest.OriginProjectUUID = firstNonEmpty(manifest.OriginProjectUUID, source.ProjectUUID)
	manifest.CreatedAt = time.Now().UTC()
	manifest.Counters = Counters{}
	if err := Write(stagedRoots, manifest); err != nil {
		cleanup()
		return "", err
	}
	if _, err := Recalibrate(stagedRoots); err != nil {
		cleanup()
		return "", err
	}
	if err := os.Rename(staging, target.Agent); err != nil {
		cleanup()
		return "", err
	}
	return target.Agent, nil
}

// RebindProjectTree rewrites exact project path/UUID identities in JSON and
// JSONL files under root. It is used while a folder export is still staged:
// files live under the staging directory but must already name the final
// logical project path before the directory is atomically published.
func RebindProjectTree(root, sourcePath, sourceUUID, targetPath, targetUUID string) error {
	source, err := Resolve(sourcePath, sourceUUID)
	if err != nil {
		return err
	}
	target, err := Resolve(targetPath, targetUUID)
	if err != nil {
		return err
	}
	return rebindTree(filepath.Clean(root), source, target)
}

func LoadWithExpected(roots Roots, projectUUID, projectPath string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(roots.Agent, ManifestFile))
	if err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != ManifestSchemaVersion || SafeName(manifest.ProjectUUID) != SafeName(projectUUID) || !samePath(manifest.ProjectPath, projectPath) {
		return Manifest{}, errors.New("agent store manifest identity mismatch")
	}
	return manifest, nil
}

func SafeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "..", "_")
	return strings.Trim(value, " .")
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func scanCounters(root string) (Counters, error) {
	counters := Counters{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Base(path) == ManifestFile {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		size := info.Size()
		counters.TotalBytes += size
		rel, _ := filepath.Rel(root, path)
		top := strings.ToLower(strings.Split(filepath.ToSlash(rel), "/")[0])
		switch top {
		case "journal":
			counters.JournalBytes += size
			if strings.EqualFold(filepath.Ext(path), ".jsonl") {
				counters.JournalShards++
				count, countErr := countLines(path)
				if countErr != nil {
					return countErr
				}
				counters.JournalRecords += count
			}
		case "observations":
			counters.ObservationsBytes += size
			counters.ObservationsCount++
		case "evidence":
			counters.EvidenceBytes += size
			if strings.Contains(filepath.ToSlash(rel), "/objects/") {
				counters.EvidenceCount++
			}
		case "mixboard":
			counters.MixboardBytes += size
		}
		return nil
	})
	return counters, err
}

func countLines(path string) (int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	var count int64
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

func copyDir(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, info.Mode().Perm())
	})
}

func rebindTree(root string, source, target Roots) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".json":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var value any
			if err := json.Unmarshal(data, &value); err != nil {
				return err
			}
			value = rebindValue(value, source, target)
			encoded, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return err
			}
			return writeFileAtomic(path, encoded, 0o600)
		case ".jsonl":
			return rebindJSONL(path, source, target)
		default:
			return nil
		}
	})
}

func rebindJSONL(path string, source, target Roots) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	lines := make([][]byte, 0)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 16*1024*1024)
	for scanner.Scan() {
		var value any
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil {
			return err
		}
		encoded, err := json.Marshal(rebindValue(value, source, target))
		if err != nil {
			return err
		}
		lines = append(lines, encoded)
	}
	if err := scanner.Err(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	var builder strings.Builder
	for _, line := range lines {
		builder.Write(line)
		builder.WriteByte('\n')
	}
	return writeFileAtomic(path, []byte(builder.String()), 0o600)
}

func rebindValue(value any, source, target Roots) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch strings.ToLower(key) {
			case "project_uuid", "project_id":
				if text, ok := child.(string); ok && SafeName(text) == source.ProjectUUID {
					typed[key] = target.ProjectUUID
					continue
				}
			case "project_path", "current_project_path":
				if text, ok := child.(string); ok && samePath(text, source.ProjectPath) {
					typed[key] = target.ProjectPath
					continue
				}
			}
			typed[key] = rebindValue(child, source, target)
		}
		return typed
	case []any:
		for index := range typed {
			typed[index] = rebindValue(typed[index], source, target)
		}
		return typed
	case string:
		if SafeName(typed) == source.ProjectUUID {
			return target.ProjectUUID
		}
		if samePath(typed, source.ProjectPath) {
			return target.ProjectPath
		}
	}
	return value
}

func samePath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
