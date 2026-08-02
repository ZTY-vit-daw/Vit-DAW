package projectworkspace

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/acousticpackage"
)

const L3FeatureLogFile = "l3_feature_log.jsonl"

var (
	l3FeatureLogMu      sync.Mutex
	l3FeatureLogDigests = map[string]map[string]struct{}{}
)

// AppendL3Feature persists an already-produced kernel L3 telemetry row in the
// owning project's derived workspace. It does not observe or read audio.
func AppendL3Feature(projectPath, projectUUID string, row map[string]any) (string, bool, error) {
	projectUUID = safeName(projectUUID)
	if strings.TrimSpace(projectPath) == "" || projectUUID == "" {
		return "", false, errors.New("L3 feature persistence requires project path and UUID")
	}
	featureType := text(row["feature_type"])
	if !isL3FeatureType(featureType) {
		return "", false, fmt.Errorf("unsupported L3 feature type %q", featureType)
	}
	stamped := cloneMap(row)
	stampL3ProjectIdentity(stamped, projectPath, projectUUID)
	encoded, err := json.Marshal(stamped)
	if err != nil {
		return "", false, err
	}
	digest := sha256.Sum256(encoded)
	wanted := hex.EncodeToString(digest[:])
	wantedSemantic := l3FeatureSemanticKey(stamped)
	dir, err := DerivedDir(projectPath, projectUUID)
	if err != nil {
		return "", false, err
	}
	path := filepath.Join(dir, L3FeatureLogFile)

	l3FeatureLogMu.Lock()
	defer l3FeatureLogMu.Unlock()
	digests := l3FeatureLogDigests[path]
	if digests == nil {
		digests = map[string]struct{}{}
		existing, readErr := readL3FeatureRows(path)
		if readErr != nil && !os.IsNotExist(readErr) {
			return path, false, readErr
		}
		for _, candidate := range existing {
			data, marshalErr := json.Marshal(candidate)
			if marshalErr != nil {
				return path, false, marshalErr
			}
			sum := sha256.Sum256(data)
			digests["digest:"+hex.EncodeToString(sum[:])] = struct{}{}
			if semantic := l3FeatureSemanticKey(candidate); semantic != "" {
				digests["semantic:"+semantic] = struct{}{}
			}
		}
		l3FeatureLogDigests[path] = digests
	}
	if _, duplicate := digests["digest:"+wanted]; duplicate {
		return path, false, nil
	}
	if wantedSemantic != "" {
		if _, duplicate := digests["semantic:"+wantedSemantic]; duplicate {
			return path, false, nil
		}
	}
	if _, duplicate := digests[wanted]; duplicate {
		// Compatibility with an in-memory digest cache populated by an older
		// Agent build in the same process.
		return path, false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return path, false, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return path, false, err
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return path, false, writeErr
	}
	if closeErr != nil {
		return path, false, closeErr
	}
	digests["digest:"+wanted] = struct{}{}
	if wantedSemantic != "" {
		digests["semantic:"+wantedSemantic] = struct{}{}
	}
	return path, true, nil
}

func L3FeatureLogPath(projectPath, projectUUID string) (string, error) {
	dir, err := DerivedDir(projectPath, projectUUID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, L3FeatureLogFile), nil
}

func ReadL3Features(projectPath, projectUUID string) ([]map[string]any, string, error) {
	path, err := L3FeatureLogPath(projectPath, projectUUID)
	if err != nil {
		return nil, "", err
	}
	l3FeatureLogMu.Lock()
	rows, err := readL3FeatureRows(path)
	l3FeatureLogMu.Unlock()
	if os.IsNotExist(err) {
		return nil, path, nil
	}
	return rows, path, err
}

// BuildL3AcousticStatuses folds the append-only project telemetry log into one
// Acoustic Package status per source/track/clip identity.
func BuildL3AcousticStatuses(projectPath, projectUUID string) ([]acousticpackage.Status, string, error) {
	rows, path, err := ReadL3Features(projectPath, projectUUID)
	if err != nil || len(rows) == 0 {
		return nil, path, err
	}
	type group struct {
		identity acousticpackage.Identity
		features map[string]map[string]any
		updated  string
	}
	groups := map[string]*group{}
	for _, row := range rows {
		featureType := strings.ToLower(text(row["feature_type"]))
		if !isL3FeatureType(featureType) {
			continue
		}
		resolved := mapValue(row["target"])
		if len(resolved) == 0 {
			resolved = row
		}
		identity := acousticpackage.IdentityFromMaps(map[string]any{"project_id": projectUUID}, resolved, row)
		identity.ProjectID = safeName(projectUUID)
		if identity.TrackID == "" || identity.ClipID == "" || (identity.SourceRevision == "" && identity.SourcePath == "") {
			continue
		}
		key := acousticIdentityKey(identity)
		current := groups[key]
		if current == nil {
			current = &group{identity: identity, features: map[string]map[string]any{}}
			groups[key] = current
		}
		current.features[featureType] = row
		if updated := text(row["updated_at"]); updated != "" {
			current.updated = updated
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]acousticpackage.Status, 0, len(keys))
	for _, key := range keys {
		current := groups[key]
		snapshot := map[string]any{}
		for featureType, row := range current.features {
			snapshot[featureType] = row
			snapshot[featureTypePlural(featureType)] = []any{row}
		}
		updated := current.updated
		if updated == "" {
			updated = time.Now().UTC().Format(time.RFC3339Nano)
		}
		out = append(out, acousticpackage.BuildStatus(current.identity, snapshot, updated, "project_l3_feature_log"))
	}
	return out, path, nil
}

func RebindL3FeatureLogFile(path, sourcePath, sourceUUID, targetPath, targetUUID string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	l3FeatureLogMu.Lock()
	defer l3FeatureLogMu.Unlock()
	rows, err := readL3FeatureRows(path)
	if err != nil {
		return err
	}
	targetUUID = safeName(targetUUID)
	for _, row := range rows {
		rebindExactValues(row, sourcePath, safeName(sourceUUID), targetPath, targetUUID)
		stampL3ProjectIdentity(row, targetPath, targetUUID)
	}
	rows = compactL3FeatureRows(rows)
	if err := writeL3FeatureRowsAtomic(path, rows); err != nil {
		return err
	}
	delete(l3FeatureLogDigests, path)
	return nil
}

func compactL3FeatureRows(rows []map[string]any) []map[string]any {
	type retained struct {
		row   map[string]any
		index int
	}
	byKey := map[string]retained{}
	for index, row := range rows {
		key := l3FeatureSemanticKey(row)
		if key == "" {
			data, _ := json.Marshal(row)
			sum := sha256.Sum256(data)
			key = "opaque:" + hex.EncodeToString(sum[:])
		}
		byKey[key] = retained{row: row, index: index}
	}
	kept := make([]retained, 0, len(byKey))
	for _, item := range byKey {
		kept = append(kept, item)
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].index < kept[j].index })
	out := make([]map[string]any, 0, len(kept))
	for _, item := range kept {
		out = append(out, item.row)
	}
	return out
}

func l3FeatureSemanticKey(row map[string]any) string {
	featureType := strings.ToLower(text(row["feature_type"]))
	if !isL3FeatureType(featureType) {
		return ""
	}
	resolved := mapValue(row["target"])
	if len(resolved) == 0 {
		resolved = row
	}
	identity := acousticpackage.IdentityFromMaps(
		map[string]any{"project_id": firstNonEmptyText(row["project_id"], row["project_uuid"])},
		resolved,
		row,
	)
	if identity.TrackID == "" || identity.ClipID == "" || (identity.SourceRevision == "" && identity.SourcePath == "") {
		return ""
	}
	return strings.Join([]string{
		acousticIdentityKey(identity),
		featureType,
		strings.ToLower(text(row["analyzer_revision"])),
	}, "\x1f")
}

func firstNonEmptyText(values ...any) string {
	for _, value := range values {
		if candidate := text(value); candidate != "" {
			return candidate
		}
	}
	return ""
}

func readL3FeatureRows(path string) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rows := make([]map[string]any, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		data := strings.TrimSpace(scanner.Text())
		if data == "" {
			continue
		}
		row := map[string]any{}
		if err := json.Unmarshal([]byte(data), &row); err != nil {
			return nil, fmt.Errorf("parse L3 feature log line %d: %w", line, err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func writeL3FeatureRowsAtomic(path string, rows []map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp"
	file, err := os.Create(temp)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	for _, row := range rows {
		data, marshalErr := json.Marshal(row)
		if marshalErr != nil {
			_ = file.Close()
			_ = os.Remove(temp)
			return marshalErr
		}
		if _, writeErr := writer.Write(append(data, '\n')); writeErr != nil {
			_ = file.Close()
			_ = os.Remove(temp)
			return writeErr
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		_ = os.Remove(temp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temp)
		return err
	}
	backup := path + ".previous"
	_ = os.Remove(backup)
	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backup); err != nil {
			_ = os.Remove(temp)
			return err
		}
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func stampL3ProjectIdentity(row map[string]any, projectPath, projectUUID string) {
	row["project_id"] = projectUUID
	row["project_uuid"] = projectUUID
	row["project_path"] = projectPath
	for _, key := range []string{"source_identity", "target"} {
		if nested := mapValue(row[key]); len(nested) > 0 {
			nested["project_id"] = projectUUID
			nested["project_uuid"] = projectUUID
		}
	}
}

func isL3FeatureType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "band_energy_summary", "stereo_relation_summary", "loudness_summary":
		return true
	default:
		return false
	}
}

func featureTypePlural(value string) string {
	switch value {
	case "band_energy_summary":
		return "band_energy_summaries"
	case "stereo_relation_summary":
		return "stereo_relation_summaries"
	case "loudness_summary":
		return "loudness_summaries"
	default:
		return value + "_rows"
	}
}

func acousticIdentityKey(identity acousticpackage.Identity) string {
	return strings.Join([]string{
		strings.ToLower(identity.ProjectID), identity.SessionID, identity.TrackID, identity.ClipID,
		strings.ToLower(filepath.Clean(identity.SourcePath)), identity.SourceHash,
		identity.SourceFingerprint, identity.SourceRevision, identity.ClipRevision, identity.RenderRevision,
	}, "\x1f")
}

func cloneMap(value map[string]any) map[string]any {
	data, _ := json.Marshal(value)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func rebindExactValues(value any, sourcePath, sourceUUID, targetPath, targetUUID string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = rebindExactValues(child, sourcePath, sourceUUID, targetPath, targetUUID)
		}
	case []any:
		for index, child := range typed {
			typed[index] = rebindExactValues(child, sourcePath, sourceUUID, targetPath, targetUUID)
		}
	case string:
		if typed == sourceUUID {
			return targetUUID
		}
		if strings.TrimSpace(sourcePath) != "" && strings.EqualFold(filepath.Clean(typed), filepath.Clean(sourcePath)) {
			return targetPath
		}
	}
	return value
}
