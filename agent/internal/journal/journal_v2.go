package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	JournalSchemaVersion   = "journal_action.v2"
	defaultShardMaxBytes   = int64(4 * 1024 * 1024)
	defaultShardMaxRecords = int64(1000)
)

func NewSharded(max int, root, projectUUID string) (*Journal, error) {
	j := New(max)
	if err := j.RebindSharded(root, projectUUID); err != nil {
		return j, err
	}
	return j, nil
}

func (j *Journal) RebindSharded(root, projectUUID string) error {
	if j == nil {
		return errors.New("journal is nil")
	}
	root = strings.TrimSpace(root)
	projectUUID = strings.TrimSpace(projectUUID)
	if root == "" || projectUUID == "" {
		return errors.New("sharded journal requires root and project UUID")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.path = ""
	j.root = filepath.Clean(root)
	j.projectUUID = projectUUID
	j.sharded = true
	j.shardMaxBytes = defaultShardMaxBytes
	j.shardMaxRecords = defaultShardMaxRecords
	j.shardMaxShards = 50
	j.actions = nil
	return j.loadV2Locked()
}

// SetShardLimits configures the active JSONL shard limits. maxShards limits
// active shards; older read-only polling shards are folded into archive.jsonl
// before a new active shard is published.
func (j *Journal) SetShardLimits(maxBytes, maxRecords int64, shardLimits ...int64) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if maxBytes > 0 {
		j.shardMaxBytes = maxBytes
	}
	if maxRecords > 0 {
		j.shardMaxRecords = maxRecords
	}
	if len(shardLimits) > 0 && shardLimits[0] > 0 {
		j.shardMaxShards = shardLimits[0]
	}
}

func (j *Journal) loadV2Locked() error {
	entries, err := os.ReadDir(j.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	paths := make([]string, 0)
	archivePaths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(j.root, entry.Name()))
	}
	archiveRoot := filepath.Join(j.root, "archive")
	if archiveEntries, archiveErr := os.ReadDir(archiveRoot); archiveErr == nil {
		for _, entry := range archiveEntries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
				continue
			}
			archivePaths = append(archivePaths, filepath.Join(archiveRoot, entry.Name()))
		}
	}
	sort.Strings(archivePaths)
	sort.Strings(paths)
	positions := map[string]int{}
	for _, path := range archivePaths {
		if err := j.readShardLocked(path, positions); err != nil {
			return err
		}
	}
	for _, path := range paths {
		if err := j.readShardLocked(path, positions); err != nil {
			return err
		}
	}
	if j.max > 0 && len(j.actions) > j.max {
		j.actions = append([]Action(nil), j.actions[len(j.actions)-j.max:]...)
	}
	return nil
}

func (j *Journal) readShardLocked(path string, positions map[string]int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		action := Action{}
		if err := json.Unmarshal(scanner.Bytes(), &action); err != nil {
			return fmt.Errorf("journal: parse %q: %w", path, err)
		}
		if action.SchemaVersion != JournalSchemaVersion || action.ProjectUUID != j.projectUUID || strings.TrimSpace(action.AgentActionID) == "" {
			return fmt.Errorf("journal: identity mismatch in %q", path)
		}
		if index, ok := positions[action.AgentActionID]; ok {
			j.actions[index] = action
			continue
		}
		positions[action.AgentActionID] = len(j.actions)
		j.actions = append(j.actions, action)
	}
	return scanner.Err()
}

func (j *Journal) appendV2Locked(action Action) error {
	if j.root == "" || j.projectUUID == "" {
		return nil
	}
	persisted := compactPersistentAction(action, j.projectUUID)
	encoded, err := json.Marshal(persisted)
	if err != nil {
		return err
	}
	if int64(len(encoded)) > defaultShardMaxBytes {
		return errors.New("journal: compact action exceeds shard maximum")
	}
	if err := os.MkdirAll(j.root, 0o755); err != nil {
		return err
	}
	path, records, size, err := j.currentShardLocked()
	if err != nil {
		return err
	}
	maxBytes := j.shardMaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultShardMaxBytes
	}
	maxRecords := j.shardMaxRecords
	if maxRecords <= 0 {
		maxRecords = defaultShardMaxRecords
	}
	if path == "" || records >= maxRecords || size+int64(len(encoded))+1 > maxBytes {
		path, err = j.nextShardPathLocked()
		if err != nil {
			return err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return j.compactShardsLocked()
}

func (j *Journal) currentShardLocked() (string, int64, int64, error) {
	entries, err := os.ReadDir(j.root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, 0, nil
		}
		return "", 0, 0, err
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return "", 0, 0, nil
	}
	sort.Strings(names)
	path := filepath.Join(j.root, names[len(names)-1])
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, 0, err
	}
	records, err := shardLineCount(path)
	return path, records, info.Size(), err
}

func (j *Journal) nextShardPathLocked() (string, error) {
	entries, err := os.ReadDir(j.root)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	next := 1
	for _, entry := range entries {
		var number int
		if _, scanErr := fmt.Sscanf(entry.Name(), "%06d.jsonl", &number); scanErr == nil && number >= next {
			next = number + 1
		}
	}
	return filepath.Join(j.root, fmt.Sprintf("%06d.jsonl", next)), nil
}

func (j *Journal) compactShardsLocked() error {
	maxShards := j.shardMaxShards
	if maxShards <= 0 {
		maxShards = 50
	}
	entries, err := os.ReadDir(j.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
			paths = append(paths, filepath.Join(j.root, entry.Name()))
		}
	}
	sort.Strings(paths)
	if int64(len(paths)) <= maxShards {
		return nil
	}
	// Keep the newest active shards. Older records are retained only when they
	// represent a write, failure, confirmation or rollback; successful polling
	// records are reproducible and may be discarded.
	archiveRoot := filepath.Join(j.root, "archive")
	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return err
	}
	archivePath := filepath.Join(archiveRoot, "000001.jsonl")
	retained := map[string]Action{}
	if data, readErr := os.ReadFile(archivePath); readErr == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var action Action
			if json.Unmarshal(scanner.Bytes(), &action) == nil && action.AgentActionID != "" {
				retained[action.AgentActionID] = action
			}
		}
	}
	cut := len(paths) - int(maxShards)
	if cut < 1 {
		cut = 1
	}
	toRemove := append([]string(nil), paths[:cut]...)
	for _, path := range toRemove {
		if err := collectArchivableShard(path, retained); err != nil {
			return err
		}
	}
	ids := make([]string, 0, len(retained))
	for id := range retained {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	for _, id := range ids {
		encoded, err := json.Marshal(retained[id])
		if err != nil {
			return err
		}
		builder.Write(encoded)
		builder.WriteByte('\n')
	}
	if err := writeArchiveAtomic(archivePath, []byte(builder.String())); err != nil {
		return err
	}
	for _, path := range toRemove {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func collectArchivableShard(path string, retained map[string]Action) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var action Action
		if err := json.Unmarshal(scanner.Bytes(), &action); err != nil {
			return err
		}
		if action.AgentActionID == "" || archivablePollingAction(action) {
			continue
		}
		retained[action.AgentActionID] = action
	}
	return scanner.Err()
}

func archivablePollingAction(action Action) bool {
	if action.Status != StatusSucceeded {
		return false
	}
	if action.RequiresConfirmation || strings.TrimSpace(action.RollbackActionID) != "" || strings.TrimSpace(action.Error) != "" {
		return false
	}
	tool := strings.ToLower(strings.TrimSpace(action.Tool))
	command := strings.ToLower(strings.TrimSpace(action.CommandName))
	return strings.Contains(tool, "observe") || strings.Contains(command, "observe") ||
		strings.Contains(command, "project.state") || strings.Contains(command, "mix.observe")
}

func writeArchiveAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func shardLineCount(path string) (int64, error) {
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

func compactPersistentAction(action Action, projectUUID string) Action {
	out := cloneAction(action)
	out.SchemaVersion = JournalSchemaVersion
	out.ProjectUUID = projectUUID
	out.Summary = truncate(out.Summary, 512)
	out.Error = truncate(out.Error, 1024)
	out.ArgsSummary = compactJSON(out.Command, 512)
	out.ResultSummary = compactResult(out.Result)
	if len(out.ResultSummary) > 0 {
		out.Result = cloneMap(out.ResultSummary)
	} else {
		out.Result = nil
	}
	if strings.EqualFold(out.Domain, "workspace") && strings.EqualFold(out.CommandName, "workspace_apply_edit") {
		out.Command = compactWorkspaceCommand(out.Command)
		out.Result = compactWorkspaceRollback(out.Result, action.Result)
	} else {
		out.Command = nil
	}
	return out
}

func compactWorkspaceCommand(command map[string]any) map[string]any {
	return selectKeys(command, "project_path", "current_project_path", "workspace_roots", "root_id")
}

func compactWorkspaceRollback(summary, full map[string]any) map[string]any {
	if summary == nil {
		summary = map[string]any{}
	}
	for _, key := range []string{"path", "reverse_old_text", "reverse_new_text"} {
		if value, ok := full[key]; ok {
			summary[key] = value
		}
	}
	return summary
}

func compactResult(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range result {
		if compact, ok := compactScalarOrMap(value, 0); ok {
			out[key] = compact
		}
	}
	encoded, _ := json.Marshal(out)
	if len(encoded) <= 16*1024 {
		return out
	}
	return selectKeys(out, "status", "message", "error", "command", "project_uuid", "project_revision", "agent_action_id", "count", "counts")
}

func compactScalarOrMap(value any, depth int) (any, bool) {
	if depth > 2 {
		return nil, false
	}
	switch typed := value.(type) {
	case nil, bool, string, float64, float32, int, int32, int64, uint, uint32, uint64, json.Number:
		return typed, true
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed {
			if compact, ok := compactScalarOrMap(child, depth+1); ok {
				out[key] = compact
			}
		}
		return out, len(out) > 0
	default:
		return nil, false
	}
}

func selectKeys(source map[string]any, keys ...string) map[string]any {
	if len(source) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := source[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactJSON(value any, limit int) string {
	if value == nil || limit <= 0 {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return truncate(string(encoded), limit)
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
