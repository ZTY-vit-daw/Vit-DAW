package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const DirName = ".vit_history"
const conversationGraphFile = "conversation_graph.json"
const agentRuntimeStateFile = "agent_runtime_state.json"
const draftProjectFileName = "Unsaved.vit"
const rootWorktreeName = "main"
const worktreesDirName = "Worktrees"

var draftProject = struct {
	sync.Mutex
	root string
	path string
}{}

var projectIdentityRegistry = struct {
	sync.RWMutex
	byPath map[string]projectWorkspaceBinding
}{byPath: map[string]projectWorkspaceBinding{}}

type projectWorkspaceBinding struct {
	ProjectUUID  string
	WorkspaceDir string
	SessionID    string
}

var identityMigrationMu sync.Mutex

type FileEntry struct {
	Path   string `json:"path"`
	Hash   string `json:"sha256"`
	Size   int64  `json:"size"`
	Kind   string `json:"kind"`
	Opaque bool   `json:"opaque,omitempty"`
}

type Commit struct {
	ID              string         `json:"id"`
	Parents         []string       `json:"parents,omitempty"`
	Message         string         `json:"message,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	ProjectPath     string         `json:"project_path"`
	ProjectUUID     string         `json:"project_uuid,omitempty"`
	Branch          string         `json:"branch,omitempty"`
	GoalID          string         `json:"goal_id,omitempty"`
	RunID           string         `json:"run_id,omitempty"`
	Source          string         `json:"source,omitempty"`
	CheckpointKind  string         `json:"checkpoint_kind,omitempty"`
	ProjectFile     FileEntry      `json:"project_file"`
	Files           []FileEntry    `json:"files"`
	Warnings        []string       `json:"warnings,omitempty"`
	SemanticSummary map[string]any `json:"semantic_summary,omitempty"`
}

type ConversationNode struct {
	ID               string           `json:"id"`
	Kind             string           `json:"kind"`
	CommitID         string           `json:"commit_id"`
	ParentNodeID     string           `json:"parent_node_id,omitempty"`
	Branch           string           `json:"branch,omitempty"`
	Text             string           `json:"text,omitempty"`
	TextPreview      string           `json:"text_preview,omitempty"`
	Artifacts        []map[string]any `json:"artifacts,omitempty"`
	ProjectCards     []map[string]any `json:"project_result_cards,omitempty"`
	MessageData      map[string]any   `json:"message_data,omitempty"`
	GoalID           string           `json:"goal_id,omitempty"`
	RunID            string           `json:"run_id,omitempty"`
	Lifecycle        string           `json:"lifecycle,omitempty"`
	Persistence      string           `json:"persistence,omitempty"`
	MessageKind      string           `json:"message_kind,omitempty"`
	TurnID           string           `json:"turn_id,omitempty"`
	LogicalMessageID string           `json:"logical_message_id,omitempty"`
	Supersedes       []string         `json:"supersedes,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
}

type ConversationMessage struct {
	Role             string           `json:"role"`
	Content          string           `json:"content"`
	NodeID           string           `json:"node_id,omitempty"`
	CommitID         string           `json:"commit_id,omitempty"`
	Branch           string           `json:"branch,omitempty"`
	Artifacts        []map[string]any `json:"artifacts,omitempty"`
	ProjectCards     []map[string]any `json:"project_result_cards,omitempty"`
	MessageData      map[string]any   `json:"message_data,omitempty"`
	Lifecycle        string           `json:"lifecycle,omitempty"`
	Persistence      string           `json:"persistence,omitempty"`
	MessageKind      string           `json:"message_kind,omitempty"`
	TurnID           string           `json:"turn_id,omitempty"`
	LogicalMessageID string           `json:"logical_message_id,omitempty"`
	Supersedes       []string         `json:"supersedes,omitempty"`
	CreatedAt        time.Time        `json:"created_at,omitempty"`
}

type ConversationGraph struct {
	ProjectPath    string             `json:"project_path"`
	ProjectUUID    string             `json:"project_uuid,omitempty"`
	ActiveNodeID   string             `json:"active_node_id,omitempty"`
	ActiveBranch   string             `json:"active_branch,omitempty"`
	ActiveWorktree string             `json:"active_worktree,omitempty"`
	Nodes          []ConversationNode `json:"nodes"`
}

type Repo struct {
	ProjectPath string
	ProjectUUID string
	ProjectDir  string
	MediaDir    string
	HistoryDir  string
	StateDir    string
	Draft       bool
}

func BindProjectIdentity(projectPath, projectUUID string) string {
	projectPath = strings.TrimSpace(projectPath)
	projectUUID = safeName(strings.TrimSpace(projectUUID))
	if projectPath == "" {
		if draftPath, err := defaultDraftProjectPath(); err == nil {
			projectPath = draftPath
		}
	}
	if projectPath == "" || projectUUID == "" {
		return projectPath
	}
	if abs, err := filepath.Abs(projectPath); err == nil {
		projectPath = filepath.Clean(abs)
	}
	projectIdentityRegistry.Lock()
	key := normalizeProjectPath(projectPath)
	binding := projectIdentityRegistry.byPath[key]
	if binding.ProjectUUID != projectUUID {
		binding = projectWorkspaceBinding{}
	}
	binding.ProjectUUID = projectUUID
	projectIdentityRegistry.byPath[key] = binding
	projectIdentityRegistry.Unlock()
	return projectPath
}

func IsDraftProjectPath(projectPath string) bool {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return true
	}
	return strings.EqualFold(filepath.Base(projectPath), draftProjectFileName) &&
		strings.Contains(strings.ToLower(filepath.ToSlash(projectPath)), "/projecthistory/drafts/")
}

func boundProjectIdentity(projectPath string) string {
	projectIdentityRegistry.RLock()
	defer projectIdentityRegistry.RUnlock()
	return projectIdentityRegistry.byPath[normalizeProjectPath(projectPath)].ProjectUUID
}

func boundProjectWorkspace(projectPath string) projectWorkspaceBinding {
	projectIdentityRegistry.RLock()
	defer projectIdentityRegistry.RUnlock()
	return projectIdentityRegistry.byPath[normalizeProjectPath(projectPath)]
}

func Open(projectPath string) (Repo, error) {
	projectPath = strings.TrimSpace(projectPath)
	draft := false
	if projectPath == "" {
		var err error
		projectPath, err = defaultDraftProjectPath()
		if err != nil {
			return Repo{}, err
		}
		draft = true
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return Repo{}, err
	}
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	projectDir := filepath.Dir(abs)
	clean := filepath.Clean(abs)
	historyRoot := filepath.Join(projectDir, DirName)
	historyDir := historyRoot
	stateDir := filepath.Join(historyRoot, "projects", projectKey(clean))
	binding := boundProjectWorkspace(clean)
	projectUUID := binding.ProjectUUID
	if projectUUID != "" {
		historyDir = filepath.Join(historyRoot, safeName(projectUUID))
		stateDir = filepath.Join(historyDir, "state")
	}
	if strings.TrimSpace(binding.WorkspaceDir) != "" {
		historyDir = filepath.Clean(binding.WorkspaceDir)
		stateDir = filepath.Join(historyDir, "state")
	}
	repo := Repo{
		ProjectPath: clean,
		ProjectUUID: projectUUID,
		ProjectDir:  projectDir,
		MediaDir:    filepath.Join(projectDir, base+"_Media"),
		HistoryDir:  historyDir,
		StateDir:    stateDir,
		Draft:       draft,
	}
	if projectUUID != "" && strings.TrimSpace(binding.WorkspaceDir) == "" {
		if err := migrateLegacyHistoryWorkspace(repo, historyRoot); err != nil {
			return Repo{}, err
		}
	}
	return repo, nil
}

func Status(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["project_path"] = repo.ProjectPath
	out["project_uuid"] = repo.ProjectUUID
	out["history_dir"] = repo.HistoryDir
	out["initialized"] = dirExists(repo.HistoryDir)
	return out, nil
}

func historyState(repo Repo, commitCount int) map[string]any {
	graph := readConversationGraphOrDefault(repo)
	return map[string]any{
		"project_path":          repo.ProjectPath,
		"project_uuid":          repo.ProjectUUID,
		"history_dir":           repo.HistoryDir,
		"root_project_path":     rootProjectPath(repo),
		"state_dir":             repo.StateDir,
		"initialized":           dirExists(repo.HistoryDir),
		"draft":                 repo.Draft,
		"unsaved":               repo.Draft,
		"project_label":         projectLabel(repo),
		"active_branch":         publicActiveBranch(repo),
		"active_node_id":        graph.ActiveNodeID,
		"active_worktree":       graph.ActiveWorktree,
		"detached":              isDetached(repo),
		"head":                  currentHead(repo),
		"branches":              listBranches(repo),
		"refs":                  listRefs(repo),
		"worktrees":             listWorktreeItems(repo),
		"conversation_graph":    graph,
		"conversation_messages": conversationMessagesForGraph(graph),
		"commit_count":          commitCount,
		"warnings":              historyWarnings(repo),
	}
}

func Checkpoint(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(repo.HistoryDir, 0o755); err != nil {
		return nil, err
	}
	head := currentHead(repo)
	detached := isDetached(repo)
	activeBranch := activeBranch(repo)
	commit := Commit{
		ID:          "c_" + time.Now().UTC().Format("20060102T150405") + "_" + shortID(),
		CreatedAt:   time.Now().UTC(),
		Message:     value(args, "message"),
		ProjectPath: repo.ProjectPath,
		ProjectUUID: repo.ProjectUUID,
		GoalID:      value(args, "goal_id"),
		RunID:       value(args, "run_id"),
		Source:      value(args, "source"),
		CheckpointKind: firstNonEmpty(
			value(args, "checkpoint_kind"),
			"manual",
		),
		Warnings: []string{},
		SemanticSummary: map[string]any{
			"source": "manifest_only",
		},
	}
	if !detached {
		commit.Branch = activeBranch
	}
	if head != "" {
		commit.Parents = []string{head}
	}
	if snapshot := strings.TrimSpace(fmt.Sprint(args["project_snapshot_xml"])); snapshot != "" && snapshot != "<nil>" {
		entry, err := storeBytes(repo, []byte(snapshot), filepath.Base(repo.ProjectPath), "project_snapshot")
		if err != nil {
			return nil, err
		}
		commit.ProjectFile = entry
	} else {
		entry, err := storeFile(repo, repo.ProjectPath, "project")
		if err != nil {
			return nil, err
		}
		commit.ProjectFile = entry
	}
	files, warnings := collectSidecars(repo)
	commit.Warnings = append(commit.Warnings, warnings...)
	for _, file := range files {
		entry, err := storeFile(repo, file.path, file.kind)
		if err != nil {
			commit.Warnings = append(commit.Warnings, err.Error())
			continue
		}
		commit.Files = append(commit.Files, entry)
	}
	sort.Slice(commit.Files, func(i, j int) bool { return commit.Files[i].Path < commit.Files[j].Path })
	if err := writeCommit(repo, commit); err != nil {
		return nil, err
	}
	if err := advanceCurrentHead(repo, commit.ID); err != nil {
		return nil, err
	}
	graph := readConversationGraphOrDefault(repo)
	if nodeID := graphNodeForCheckout(graph, commit.ID, ""); nodeID != "" {
		graph.ActiveNodeID = nodeID
		_ = writeConversationGraph(repo, graph)
	}
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["commit"] = commit
	out["history_dir"] = repo.HistoryDir
	out["commit_id"] = commit.ID
	return out, nil
}

func List(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	commits, err := listCommits(repo)
	if err != nil {
		return nil, err
	}
	out := historyState(repo, len(commits))
	out["project_path"] = repo.ProjectPath
	out["history_dir"] = repo.HistoryDir
	out["state_dir"] = repo.StateDir
	out["commits"] = commits
	out["head"] = currentHead(repo)
	out["refs"] = listRefs(repo)
	return out, nil
}

func Show(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	id := firstNonEmpty(value(args, "commit_id"), currentHead(repo))
	commit, err := readCommit(repo, id)
	if err != nil {
		return nil, err
	}
	if !commitBelongsToProject(repo, commit) {
		return nil, fmt.Errorf("checkpoint %s belongs to a different project", id)
	}
	return map[string]any{
		"project_path":  repo.ProjectPath,
		"history_dir":   repo.HistoryDir,
		"head":          currentHead(repo),
		"active_branch": publicActiveBranch(repo),
		"detached":      isDetached(repo),
		"commit":        commit,
	}, nil
}

func Diff(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	id := firstNonEmpty(value(args, "commit_id"), currentHead(repo))
	commit, err := readCommit(repo, id)
	if err != nil {
		return nil, err
	}
	if !commitBelongsToProject(repo, commit) {
		return nil, fmt.Errorf("checkpoint %s belongs to a different project", id)
	}
	changes := compareWorking(repo, commit, value(args, "project_snapshot_xml"))
	return map[string]any{
		"project_path": repo.ProjectPath,
		"history_dir":  repo.HistoryDir,
		"commit_id":    id,
		"changes":      changes,
	}, nil
}

func RestorePreview(args map[string]any) (map[string]any, error) {
	repo, commit, err := repoAndCommit(args)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"project_path": repo.ProjectPath,
		"history_dir":  repo.HistoryDir,
		"commit_id":    commit.ID,
		"changes":      compareWorking(repo, commit, value(args, "project_snapshot_xml")),
		"warnings":     commit.Warnings,
	}, nil
}

func Restore(args map[string]any) (map[string]any, error) {
	repo, commit, err := repoAndCommit(args)
	if err != nil {
		return nil, err
	}
	if err := materialize(repo, commit, repo.ProjectDir); err != nil {
		return nil, err
	}
	if err := advanceCurrentHead(repo, commit.ID); err != nil {
		return nil, err
	}
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["status"] = "ok"
	out["commit_id"] = commit.ID
	out["restored_to"] = repo.ProjectDir
	return out, nil
}

func BranchCreate(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	name := safeName(firstNonEmpty(value(args, "name"), value(args, "branch")))
	if name == "" {
		return nil, errors.New("branch name is required")
	}
	id := firstNonEmpty(value(args, "commit_id"), currentHead(repo))
	if id == "" {
		return nil, errors.New("no commit available for branch")
	}
	commit, err := readCommit(repo, id)
	if err != nil {
		return nil, err
	}
	if !commitBelongsToProject(repo, commit) {
		return nil, fmt.Errorf("checkpoint %s belongs to a different project", id)
	}
	if err := writeRef(repo, filepath.Join("refs", "heads", name), id); err != nil {
		return nil, err
	}
	if err := materialize(repo, commit, repo.ProjectDir); err != nil {
		return nil, err
	}
	if err := writeActiveBranch(repo, name, false); err != nil {
		return nil, err
	}
	if err := writeRef(repo, "HEAD", id); err != nil {
		return nil, err
	}
	if fromNodeID := value(args, "from_node_id"); fromNodeID != "" {
		_, _ = AppendConversationNode(map[string]any{
			"project_path":   repo.ProjectPath,
			"kind":           "branch_marker",
			"commit_id":      id,
			"parent_node_id": fromNodeID,
			"branch":         name,
			"text_preview":   "branch " + name,
			"goal_id":        value(args, "goal_id"),
			"run_id":         value(args, "run_id"),
		})
	}
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["branch"] = name
	out["commit_id"] = id
	out["status"] = "ok"
	out["checked_out_to"] = repo.ProjectDir
	return out, nil
}

func WorktreeCreate(args map[string]any) (map[string]any, error) {
	sourceRepo, commit, err := repoAndCommit(args)
	if err != nil {
		return nil, err
	}
	repo := rootRepoForWorktrees(sourceRepo)
	ensureRootWorktreeItem(repo)
	name := safeName(firstNonEmpty(value(args, "name"), commit.ID))
	target := filepath.Join(visibleWorktreeRoot(repo), name)
	if dirExists(target) {
		return nil, fmt.Errorf("worktree already exists: %s", name)
	}
	if err := materialize(sourceRepo, commit, target); err != nil {
		return nil, err
	}
	projectFilePath := filepath.Join(target, worktreeProjectFileName(repo, name))
	if err := normalizeMaterializedWorktreeProject(target, commit, projectFilePath); err != nil {
		return nil, err
	}
	meta := map[string]any{
		"name":                name,
		"commit_id":           commit.ID,
		"origin_commit_id":    commit.ID,
		"path":                target,
		"project_file_path":   projectFilePath,
		"project_path":        repo.ProjectPath,
		"root_project_path":   repo.ProjectPath,
		"source_project_path": sourceRepo.ProjectPath,
		"created_at":          time.Now().UTC(),
	}
	if err := writeJSON(filepath.Join(target, ".vit_worktree.json"), meta); err != nil {
		return nil, err
	}
	if fileExists(projectFilePath) {
		if seed, err := Checkpoint(map[string]any{
			"project_path":         projectFilePath,
			"message":              "worktree seed from " + commit.ID,
			"source":               "worktree_create",
			"checkpoint_kind":      "manual",
			"project_snapshot_xml": "",
		}); err == nil {
			if seedID := strings.TrimSpace(fmt.Sprint(seed["commit_id"])); seedID != "" && seedID != "<nil>" {
				meta["worktree_head_commit_id"] = seedID
				_, _ = AppendConversationNode(map[string]any{
					"project_path": projectFilePath,
					"kind":         "checkpoint",
					"commit_id":    seedID,
					"branch":       "main",
					"text_preview": "worktree seed",
				})
				_ = writeJSON(filepath.Join(target, ".vit_worktree.json"), meta)
			}
		} else {
			meta["warning"] = "worktree history seed failed: " + err.Error()
		}
	}
	return meta, nil
}

func WorktreeList(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	rootPath := rootProjectPath(repo)
	listRepo := repo
	if rootPath != "" {
		if rootRepo, rootErr := Open(rootPath); rootErr == nil {
			listRepo = rootRepo
		}
	}
	ensureRootWorktreeItem(listRepo)
	return map[string]any{
		"project_path":      repo.ProjectPath,
		"history_dir":       repo.HistoryDir,
		"root_project_path": firstNonEmpty(rootPath, listRepo.ProjectPath),
		"state_dir":         repo.StateDir,
		"draft":             repo.Draft,
		"unsaved":           repo.Draft,
		"project_label":     projectLabel(repo),
		"active_worktree":   activeWorktree(repo),
		"worktrees":         listWorktreeItems(listRepo),
	}, nil
}

func WorktreeCheckout(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	targetName := safeName(value(args, "name"))
	targetPath := strings.TrimSpace(firstNonEmpty(value(args, "project_file_path"), value(args, "path")))
	var selected map[string]any
	for _, item := range listWorktreeItems(repo) {
		itemPath := strings.TrimSpace(fmt.Sprint(item["project_file_path"]))
		if targetName != "" && safeName(fmt.Sprint(item["name"])) == targetName {
			selected = item
			break
		}
		if targetPath != "" && sameProjectPath(itemPath, targetPath) {
			selected = item
			break
		}
	}
	if selected == nil && targetPath != "" {
		selected = map[string]any{"project_file_path": targetPath, "path": filepath.Dir(targetPath)}
	}
	if selected == nil {
		return nil, fmt.Errorf("worktree not found: %s", firstNonEmpty(targetName, targetPath))
	}
	projectFilePath := strings.TrimSpace(fmt.Sprint(selected["project_file_path"]))
	if projectFilePath == "" {
		projectFilePath = filepath.Join(strings.TrimSpace(fmt.Sprint(selected["path"])), filepath.Base(repo.ProjectPath))
		selected["project_file_path"] = projectFilePath
	}
	if !fileExists(projectFilePath) {
		return nil, fmt.Errorf("worktree project file does not exist: %s", projectFilePath)
	}
	targetRepo, err := Open(projectFilePath)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	if head, err := materializeCurrentHead(targetRepo); err != nil {
		warnings = append(warnings, "worktree head materialize failed: "+err.Error())
	} else if head != "" {
		selected["worktree_head_commit_id"] = head
		selected["head"] = head
		if !isTrueValue(selected["is_root"]) {
			_ = writeJSON(filepath.Join(targetRepo.ProjectDir, ".vit_worktree.json"), selected)
		}
	}
	out := map[string]any{
		"status":            "ok",
		"project_path":      repo.ProjectPath,
		"project_file_path": projectFilePath,
		"worktree":          selected,
	}
	if !isTrueValue(selected["is_root"]) {
		out["active_worktree"] = strings.TrimSpace(fmt.Sprint(selected["name"]))
	} else {
		out["active_worktree"] = ""
		out["root_project_path"] = projectFilePath
	}
	if status, err := Status(map[string]any{"project_path": projectFilePath}); err == nil {
		out["target_project_history"] = status
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	return out, nil
}

func ProjectSaved(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	if repo.Draft {
		return nil, errors.New("project_path is required for saved project adoption")
	}
	out := map[string]any{
		"status":          "ok",
		"project_path":    repo.ProjectPath,
		"adopted_draft":   false,
		"adoption_reason": "no_draft_history",
	}
	draftPath := currentDraftProjectPath()
	if draftPath == "" || sameProjectPath(draftPath, repo.ProjectPath) {
		commits, _ := listCommits(repo)
		for k, v := range historyState(repo, len(commits)) {
			out[k] = v
		}
		return out, nil
	}
	draftRepo, err := Open(draftPath)
	if err != nil {
		out["warnings"] = []string{"draft history unavailable: " + err.Error()}
		return out, nil
	}
	if !historyHasUserData(draftRepo) {
		commits, _ := listCommits(repo)
		for k, v := range historyState(repo, len(commits)) {
			out[k] = v
		}
		return out, nil
	}
	if historyHasUserData(repo) {
		out["adoption_reason"] = "target_history_exists"
		commits, _ := listCommits(repo)
		for k, v := range historyState(repo, len(commits)) {
			out[k] = v
		}
		return out, nil
	}
	if dirExists(repo.HistoryDir) {
		if err := os.RemoveAll(repo.HistoryDir); err != nil {
			return nil, err
		}
	}
	if err := copyDir(draftRepo.HistoryDir, repo.HistoryDir); err != nil {
		return nil, err
	}
	if err := adoptCopiedStateDir(repo, draftRepo); err != nil {
		return nil, err
	}
	if err := adoptVisibleWorktrees(repo, draftRepo); err != nil {
		return nil, err
	}
	if err := rewriteAdoptedDraftHistory(repo, draftRepo); err != nil {
		return nil, err
	}
	ensureRootWorktreeItem(repo)
	commits, _ := listCommits(repo)
	for k, v := range historyState(repo, len(commits)) {
		out[k] = v
	}
	out["adopted_draft"] = true
	out["adoption_reason"] = "draft_history_adopted"
	out["draft_project_path"] = draftRepo.ProjectPath
	return out, nil
}

func ProjectFork(args map[string]any) (map[string]any, error) {
	sourcePath := firstNonEmpty(value(args, "source_project_path"), value(args, "from_project_path"))
	targetPath := firstNonEmpty(value(args, "project_path"), value(args, "target_project_path"))
	sourceUUID := firstNonEmpty(value(args, "source_project_uuid"), value(args, "from_project_uuid"))
	targetUUID := firstNonEmpty(value(args, "project_uuid"), value(args, "target_project_uuid"))
	if sourcePath == "" || targetPath == "" || sourceUUID == "" || targetUUID == "" {
		return nil, errors.New("source/target project path and UUID are required for project fork")
	}
	if sourceUUID == targetUUID {
		return nil, errors.New("project fork requires a new target project UUID")
	}
	BindProjectIdentity(sourcePath, sourceUUID)
	BindProjectIdentity(targetPath, targetUUID)
	sourceRepo, err := Open(sourcePath)
	if err != nil {
		return nil, err
	}
	targetRepo, err := Open(targetPath)
	if err != nil {
		return nil, err
	}
	if historyHasUserData(targetRepo) {
		return nil, errors.New("target project workspace already contains history")
	}
	if !historyHasUserData(sourceRepo) {
		commits, _ := listCommits(targetRepo)
		out := historyState(targetRepo, len(commits))
		out["status"] = "ok"
		out["forked_history"] = false
		out["fork_reason"] = "source_history_empty"
		return out, nil
	}
	if dirExists(targetRepo.HistoryDir) {
		if err := os.RemoveAll(targetRepo.HistoryDir); err != nil {
			return nil, err
		}
	}
	if err := copyDir(sourceRepo.HistoryDir, targetRepo.HistoryDir); err != nil {
		return nil, err
	}
	if err := adoptVisibleWorktrees(targetRepo, sourceRepo); err != nil {
		return nil, err
	}
	if err := rewriteAdoptedDraftHistory(targetRepo, sourceRepo); err != nil {
		return nil, err
	}
	ensureRootWorktreeItem(targetRepo)
	commits, _ := listCommits(targetRepo)
	out := historyState(targetRepo, len(commits))
	out["status"] = "ok"
	out["forked_history"] = true
	out["source_project_path"] = sourceRepo.ProjectPath
	out["source_project_uuid"] = sourceRepo.ProjectUUID
	out["project_path"] = targetRepo.ProjectPath
	out["project_uuid"] = targetRepo.ProjectUUID
	return out, nil
}

func ProjectNew(args map[string]any) (map[string]any, error) {
	draftProject.Lock()
	draftProject.root = ""
	draftProject.path = ""
	draftProject.Unlock()

	repo, err := Open("")
	if err != nil {
		return nil, err
	}
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["status"] = "ok"
	out["created_draft"] = true
	out["draft_project_path"] = repo.ProjectPath
	return out, nil
}

func NodeCheckout(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	nodeID := value(args, "node_id")
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}
	graph := readConversationGraphOrDefault(repo)
	var node ConversationNode
	for _, candidate := range graph.Nodes {
		if candidate.ID == nodeID {
			node = candidate
			break
		}
	}
	if node.ID == "" {
		return nil, fmt.Errorf("conversation node not found: %s", nodeID)
	}
	if strings.TrimSpace(node.CommitID) == "" {
		return nil, fmt.Errorf("conversation node has no checkpoint: %s", nodeID)
	}
	out, err := Checkout(map[string]any{"project_path": repo.ProjectPath, "commit_id": node.CommitID})
	if err != nil {
		return nil, err
	}
	graph = readConversationGraphOrDefault(repo)
	graph.ActiveNodeID = nodeID
	graph.ActiveBranch = publicActiveBranch(repo)
	graph.ActiveWorktree = activeWorktree(repo)
	if err := writeConversationGraph(repo, graph); err != nil {
		return nil, err
	}
	out["active_node_id"] = nodeID
	out["conversation_graph"] = graph
	return out, nil
}

func AppendConversationNode(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	commitID := firstNonEmpty(value(args, "commit_id"), currentHead(repo))
	if commitID == "" {
		return nil, errors.New("commit_id or current HEAD is required for conversation node")
	}
	commit, err := readCommit(repo, commitID)
	if err != nil {
		return nil, err
	}
	if !commitBelongsToProject(repo, commit) {
		return nil, fmt.Errorf("checkpoint %s belongs to a different project", commitID)
	}
	if strings.EqualFold(value(args, "lifecycle"), "transient") || strings.EqualFold(value(args, "persistence"), "none") {
		return nil, errors.New("transient conversation activity cannot be written to Project History")
	}
	graph := readConversationGraphOrDefault(repo)
	branch := value(args, "branch")
	if branch == "" && !isDetached(repo) {
		branch = activeBranch(repo)
	}
	parentNodeID := value(args, "parent_node_id")
	if parentNodeID == "" {
		parentNodeID = graph.ActiveNodeID
	}
	nodeText := firstNonEmpty(value(args, "text"), value(args, "text_preview"), value(args, "message"))
	nodeID := firstNonEmpty(value(args, "node_id"), "n_"+time.Now().UTC().Format("20060102T150405")+"_"+shortID())
	nodeKind := firstNonEmpty(value(args, "kind"), value(args, "node_type"), "checkpoint")
	messageKind := value(args, "message_kind")
	if messageKind == "" {
		switch strings.ToLower(strings.TrimSpace(nodeKind)) {
		case "ask":
			messageKind = "user"
		case "vit":
			messageKind = "assistant"
		default:
			messageKind = "system"
		}
	}
	node := ConversationNode{
		ID:               nodeID,
		Kind:             nodeKind,
		CommitID:         commitID,
		ParentNodeID:     parentNodeID,
		Branch:           branch,
		Text:             nodeText,
		TextPreview:      compactPreview(nodeText),
		Artifacts:        conversationArtifactRows(args["artifacts"]),
		ProjectCards:     conversationProjectResultRows(args["project_result_cards"]),
		MessageData:      conversationMessageData(args["message_data"]),
		GoalID:           value(args, "goal_id"),
		RunID:            value(args, "run_id"),
		Lifecycle:        firstNonEmpty(value(args, "lifecycle"), "durable"),
		Persistence:      firstNonEmpty(value(args, "persistence"), "project_history"),
		MessageKind:      messageKind,
		TurnID:           firstNonEmpty(value(args, "turn_id"), value(args, "run_id"), value(args, "goal_id")),
		LogicalMessageID: firstNonEmpty(value(args, "logical_message_id"), nodeID),
		Supersedes:       conversationStringValues(args["supersedes"]),
		CreatedAt:        time.Now().UTC(),
	}
	graph.ProjectPath = repo.ProjectPath
	graph.ProjectUUID = repo.ProjectUUID
	graph.ActiveBranch = publicActiveBranch(repo)
	graph.ActiveWorktree = activeWorktree(repo)
	graph.Nodes = append(graph.Nodes, node)
	if !isFalseValue(args["set_active_node"]) {
		graph.ActiveNodeID = node.ID
	}
	if err := writeConversationGraph(repo, graph); err != nil {
		return nil, err
	}
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["node"] = node
	out["active_node_id"] = graph.ActiveNodeID
	out["conversation_graph"] = graph
	return out, nil
}

func ConversationMessages(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	graph := readConversationGraphOrDefault(repo)
	return map[string]any{
		"project_path":          repo.ProjectPath,
		"active_branch":         publicActiveBranch(repo),
		"active_node_id":        graph.ActiveNodeID,
		"active_worktree":       activeWorktree(repo),
		"conversation_messages": conversationMessagesForGraph(graph),
		"conversation_graph":    graph,
	}, nil
}

func WriteAgentRuntimeState(projectPath, projectUUID string, data []byte) error {
	projectPath = BindProjectIdentity(projectPath, projectUUID)
	repo, err := Open(projectPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(repo.StateDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(repo.StateDir, agentRuntimeStateFile)
	temp := target + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, target)
}

func ReadAgentRuntimeState(projectPath, projectUUID string) ([]byte, error) {
	projectPath = BindProjectIdentity(projectPath, projectUUID)
	repo, err := Open(projectPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(repo.StateDir, agentRuntimeStateFile))
}

func NodeDelete(args map[string]any) (map[string]any, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return nil, err
	}
	nodeID := value(args, "node_id")
	if nodeID == "" {
		return nil, errors.New("node_id is required")
	}
	graph := readConversationGraphOrDefault(repo)
	nodeByID := map[string]ConversationNode{}
	children := map[string][]string{}
	for _, node := range graph.Nodes {
		nodeByID[node.ID] = node
		if strings.TrimSpace(node.ParentNodeID) != "" {
			children[node.ParentNodeID] = append(children[node.ParentNodeID], node.ID)
		}
	}
	selected, ok := nodeByID[nodeID]
	if !ok {
		return nil, fmt.Errorf("conversation node not found: %s", nodeID)
	}
	deleteIDs := map[string]bool{}
	queue := []string{nodeID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if deleteIDs[current] {
			continue
		}
		deleteIDs[current] = true
		queue = append(queue, children[current]...)
	}
	remaining := make([]ConversationNode, 0, len(graph.Nodes)-len(deleteIDs))
	remainingByID := map[string]ConversationNode{}
	deletedBranches := map[string]bool{}
	deletedCommits := map[string]bool{}
	for _, node := range graph.Nodes {
		if deleteIDs[node.ID] {
			if strings.TrimSpace(node.Branch) != "" {
				deletedBranches[node.Branch] = true
			}
			if strings.TrimSpace(node.CommitID) != "" {
				deletedCommits[node.CommitID] = true
			}
			continue
		}
		remaining = append(remaining, node)
		remainingByID[node.ID] = node
	}
	deleteIDs, remaining = pruneEmptyDeletedBranches(deleteIDs, remaining, deletedBranches)
	remainingByID = map[string]ConversationNode{}
	deletedCommits = map[string]bool{}
	for _, node := range graph.Nodes {
		if deleteIDs[node.ID] && strings.TrimSpace(node.CommitID) != "" {
			deletedCommits[node.CommitID] = true
		}
	}
	for _, node := range remaining {
		remainingByID[node.ID] = node
	}
	if len(remaining) == 0 {
		return nil, errors.New("cannot delete the only history node")
	}
	activeWasDeleted := deleteIDs[graph.ActiveNodeID]
	nextActiveID := graph.ActiveNodeID
	if activeWasDeleted || nextActiveID == "" {
		if parent := strings.TrimSpace(selected.ParentNodeID); parent != "" && !deleteIDs[parent] {
			nextActiveID = parent
		} else {
			nextActiveID = remaining[len(remaining)-1].ID
		}
	}
	if activeWasDeleted {
		if nextActive, ok := remainingByID[nextActiveID]; ok && strings.TrimSpace(nextActive.CommitID) != "" {
			if _, err := Checkout(map[string]any{"project_path": repo.ProjectPath, "commit_id": nextActive.CommitID}); err != nil {
				return nil, err
			}
			graph = readConversationGraphOrDefault(repo)
		}
	}
	graph.Nodes = remaining
	graph.ActiveNodeID = nextActiveID
	graph.ActiveBranch = publicActiveBranch(repo)
	graph.ActiveWorktree = activeWorktree(repo)
	if err := writeConversationGraph(repo, graph); err != nil {
		return nil, err
	}
	remainingBranches := map[string]bool{}
	for _, node := range remaining {
		if strings.TrimSpace(node.Branch) != "" {
			remainingBranches[node.Branch] = true
		}
	}
	deletedBranchNames := []string{}
	for branch := range deletedBranches {
		if branch == "" || branch == "main" || remainingBranches[branch] {
			continue
		}
		_ = os.Remove(refPath(repo.StateDir, filepath.Join("refs", "heads", branch)))
		deletedBranchNames = append(deletedBranchNames, branch)
	}
	deletedCommitIDs, deletedObjectHashes := deleteUnreferencedCommits(repo, deletedCommits)
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["status"] = "ok"
	out["deleted_node_id"] = nodeID
	out["deleted_node_count"] = len(deleteIDs)
	out["deleted_branches"] = deletedBranchNames
	out["deleted_commit_ids"] = deletedCommitIDs
	out["deleted_object_count"] = len(deletedObjectHashes)
	out["active_node_id"] = graph.ActiveNodeID
	out["conversation_graph"] = graph
	return out, nil
}
func Checkout(args map[string]any) (map[string]any, error) {
	repo, commit, err := repoAndCommit(args)
	if err != nil {
		return nil, err
	}
	if err := materialize(repo, commit, repo.ProjectDir); err != nil {
		return nil, err
	}
	branch := safeName(value(args, "branch"))
	if branch != "" {
		if err := writeActiveBranch(repo, branch, false); err != nil {
			return nil, err
		}
		if err := writeRef(repo, "HEAD", commit.ID); err != nil {
			return nil, err
		}
	} else {
		if err := writeActiveBranch(repo, "", true); err != nil {
			return nil, err
		}
		if err := writeRef(repo, "HEAD", commit.ID); err != nil {
			return nil, err
		}
	}
	graph := readConversationGraphOrDefault(repo)
	if nodeID := graphNodeForCheckout(graph, commit.ID, branch); nodeID != "" {
		graph.ActiveNodeID = nodeID
		_ = writeConversationGraph(repo, graph)
	}
	commits, _ := listCommits(repo)
	out := historyState(repo, len(commits))
	out["status"] = "ok"
	out["commit_id"] = commit.ID
	out["checked_out_to"] = repo.ProjectDir
	if branch != "" {
		out["branch"] = branch
	}
	return out, nil
}

func repoAndCommit(args map[string]any) (Repo, Commit, error) {
	repo, err := Open(projectPath(args))
	if err != nil {
		return Repo{}, Commit{}, err
	}
	id := firstNonEmpty(value(args, "commit_id"), currentHead(repo))
	if branch := safeName(value(args, "branch")); branch != "" {
		id = readRef(repo, filepath.Join("refs", "heads", branch))
	}
	if id == "" {
		return Repo{}, Commit{}, errors.New("commit_id or branch is required")
	}
	commit, err := readCommit(repo, id)
	if err != nil {
		return repo, commit, err
	}
	if !commitBelongsToProject(repo, commit) {
		return repo, commit, fmt.Errorf("checkpoint %s belongs to a different project", id)
	}
	return repo, commit, nil
}

type sidecar struct {
	path string
	kind string
}

func collectSidecars(repo Repo) ([]sidecar, []string) {
	out := []sidecar{}
	warnings := []string{}
	for _, name := range []string{"plugin_grabber_profiles.json", "rack_control_macros.json"} {
		path := filepath.Join(repo.MediaDir, name)
		if fileExists(path) {
			out = append(out, sidecar{path: path, kind: strings.TrimSuffix(name, ".json")})
		}
	}
	generated := filepath.Join(repo.MediaDir, "Generated")
	if dirExists(generated) {
		_ = filepath.WalkDir(generated, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			out = append(out, sidecar{path: path, kind: "generated_asset"})
			return nil
		})
	}
	if dirExists(filepath.Join(repo.MediaDir, "AIGC_Cache")) {
		warnings = append(warnings, "AIGC_Cache is intentionally excluded from long-term checkpoints")
	}
	return out, warnings
}

func storeFile(repo Repo, path, kind string) (FileEntry, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return FileEntry{}, err
	}
	rel, err := filepath.Rel(repo.ProjectDir, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	return storeBytes(repo, b, filepath.ToSlash(rel), kind)
}

func storeBytes(repo Repo, b []byte, relPath, kind string) (FileEntry, error) {
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	objectPath := objectPath(repo, hash)
	if !fileExists(objectPath) {
		if err := os.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
			return FileEntry{}, err
		}
		if err := os.WriteFile(objectPath, b, 0o644); err != nil {
			return FileEntry{}, err
		}
	}
	return FileEntry{Path: filepath.ToSlash(relPath), Hash: hash, Size: int64(len(b)), Kind: kind}, nil
}

func materialize(repo Repo, commit Commit, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	entries := append([]FileEntry{commit.ProjectFile}, commit.Files...)
	for _, entry := range entries {
		if entry.Path == "" || entry.Hash == "" {
			continue
		}
		src := objectPath(repo, entry.Hash)
		dst := filepath.Join(targetDir, filepath.FromSlash(entry.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func materializeCurrentHead(repo Repo) (string, error) {
	id := currentHead(repo)
	if id == "" {
		return "", nil
	}
	commit, err := readCommit(repo, id)
	if err != nil {
		return "", err
	}
	if !commitBelongsToProject(repo, commit) {
		return "", fmt.Errorf("checkpoint %s belongs to a different project", id)
	}
	if err := materialize(repo, commit, repo.ProjectDir); err != nil {
		return "", err
	}
	return id, nil
}

func compareWorking(repo Repo, commit Commit, projectSnapshotXML string) []map[string]any {
	changes := []map[string]any{}
	for _, entry := range append([]FileEntry{commit.ProjectFile}, commit.Files...) {
		if strings.TrimSpace(projectSnapshotXML) != "" && entry.Path == commit.ProjectFile.Path {
			hash := hashBytes([]byte(projectSnapshotXML))
			if hash != entry.Hash {
				changes = append(changes, map[string]any{"path": entry.Path, "status": "modified", "from": hash, "to": entry.Hash, "source": "memory_snapshot"})
			}
			continue
		}
		path := filepath.Join(repo.ProjectDir, filepath.FromSlash(entry.Path))
		if !fileExists(path) {
			changes = append(changes, map[string]any{"path": entry.Path, "status": "missing"})
			continue
		}
		hash, err := hashFile(path)
		if err != nil {
			changes = append(changes, map[string]any{"path": entry.Path, "status": "error", "error": err.Error()})
			continue
		}
		if hash != entry.Hash {
			changes = append(changes, map[string]any{"path": entry.Path, "status": "modified", "from": hash, "to": entry.Hash})
		}
	}
	return changes
}

func writeCommit(repo Repo, commit Commit) error {
	return writeJSON(filepath.Join(repo.HistoryDir, "commits", commit.ID+".json"), commit)
}

func readCommit(repo Repo, id string) (Commit, error) {
	var commit Commit
	if strings.TrimSpace(id) == "" {
		return commit, errors.New("commit id is required")
	}
	err := readJSON(filepath.Join(repo.HistoryDir, "commits", id+".json"), &commit)
	return commit, err
}

func listCommits(repo Repo) ([]Commit, error) {
	dir := filepath.Join(repo.HistoryDir, "commits")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	commits := []Commit{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var commit Commit
		if err := readJSON(filepath.Join(dir, entry.Name()), &commit); err == nil && commitBelongsToProject(repo, commit) {
			commits = append(commits, commit)
		}
	}
	sort.Slice(commits, func(i, j int) bool { return commits[i].CreatedAt.After(commits[j].CreatedAt) })
	return commits, nil
}

func pruneEmptyDeletedBranches(deleteIDs map[string]bool, remaining []ConversationNode, deletedBranches map[string]bool) (map[string]bool, []ConversationNode) {
	if len(deletedBranches) == 0 || len(remaining) == 0 {
		return deleteIDs, remaining
	}
	hasContent := map[string]bool{}
	for _, node := range remaining {
		branch := strings.TrimSpace(node.Branch)
		if branch == "" || !deletedBranches[branch] {
			continue
		}
		if strings.TrimSpace(node.Kind) != "branch_marker" {
			hasContent[branch] = true
		}
	}
	out := make([]ConversationNode, 0, len(remaining))
	for _, node := range remaining {
		branch := strings.TrimSpace(node.Branch)
		if branch != "" && deletedBranches[branch] && !hasContent[branch] && strings.TrimSpace(node.Kind) == "branch_marker" {
			deleteIDs[node.ID] = true
			continue
		}
		out = append(out, node)
	}
	return deleteIDs, out
}
func deleteUnreferencedCommits(repo Repo, candidates map[string]bool) ([]string, []string) {
	if len(candidates) == 0 {
		return nil, nil
	}
	referencedCommits, referencedObjects := referencedHistoryObjects(repo)
	deletedCommits := []string{}
	deletedObjects := []string{}
	for id := range candidates {
		id = strings.TrimSpace(id)
		if id == "" || referencedCommits[id] {
			continue
		}
		commit, err := readCommit(repo, id)
		if err != nil {
			continue
		}
		_ = os.Remove(filepath.Join(repo.HistoryDir, "commits", id+".json"))
		deletedCommits = append(deletedCommits, id)
		for _, entry := range append([]FileEntry{commit.ProjectFile}, commit.Files...) {
			if entry.Hash == "" || referencedObjects[entry.Hash] {
				continue
			}
			_ = os.Remove(objectPath(repo, entry.Hash))
			deletedObjects = append(deletedObjects, entry.Hash)
		}
	}
	sort.Strings(deletedCommits)
	sort.Strings(deletedObjects)
	return deletedCommits, deletedObjects
}

func referencedHistoryObjects(repo Repo) (map[string]bool, map[string]bool) {
	commits := map[string]bool{}
	objects := map[string]bool{}
	graph := readConversationGraphOrDefault(repo)
	for _, node := range graph.Nodes {
		if node.CommitID != "" {
			commits[node.CommitID] = true
		}
	}
	for _, id := range listRefs(repo) {
		if id != "" {
			commits[id] = true
		}
	}
	for _, item := range listWorktreeItems(repo) {
		for _, key := range []string{"commit_id", "origin_commit_id", "worktree_head_commit_id"} {
			id := strings.TrimSpace(fmt.Sprint(item[key]))
			if id != "" && id != "<nil>" {
				commits[id] = true
			}
		}
	}
	for id := range commits {
		commit, err := readCommit(repo, id)
		if err != nil {
			continue
		}
		for _, entry := range append([]FileEntry{commit.ProjectFile}, commit.Files...) {
			if entry.Hash != "" {
				objects[entry.Hash] = true
			}
		}
	}
	return commits, objects
}
func objectPath(repo Repo, hash string) string {
	prefix := hash
	if len(prefix) > 2 {
		prefix = hash[:2]
	}
	return filepath.Join(repo.HistoryDir, "objects", "sha256", prefix, hash)
}

func refPath(baseDir, name string) string {
	return filepath.Join(baseDir, filepath.FromSlash(filepath.ToSlash(name)))
}

func readProjectRefValue(repo Repo, name string) string {
	return readRefValue(refPath(repo.StateDir, name))
}

func readLegacyRefValue(repo Repo, name string) string {
	return readRefValue(refPath(repo.HistoryDir, name))
}

func readRefValue(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeRef(repo Repo, name, value string) error {
	path := refPath(repo.StateDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(value)), 0o644)
}

func readRef(repo Repo, name string) string {
	if value := readProjectRefValue(repo, name); value != "" {
		return value
	}
	value := readLegacyRefValue(repo, name)
	if !refValueBelongsToProject(repo, value) {
		return ""
	}
	return value
}

func listRefs(repo Repo) map[string]string {
	refs := map[string]string{}
	collectRefs(repo, repo.HistoryDir, refs)
	collectRefs(repo, repo.StateDir, refs)
	return refs
}

func collectRefs(repo Repo, baseDir string, refs map[string]string) {
	root := filepath.Join(baseDir, "refs")
	if !dirExists(root) {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}
		value := strings.TrimSpace(string(b))
		if refValueBelongsToProject(repo, value) {
			refs[filepath.ToSlash(rel)] = value
		}
		return nil
	})
}

func listBranches(repo Repo) map[string]string {
	branches := map[string]string{}
	for name, id := range listRefs(repo) {
		if !strings.HasPrefix(name, "refs/heads/") {
			continue
		}
		branch := strings.TrimPrefix(name, "refs/heads/")
		if branch != "" {
			branches[branch] = id
		}
	}
	return branches
}

func readConversationGraphOrDefault(repo Repo) ConversationGraph {
	graph := ConversationGraph{}
	_ = readJSON(filepath.Join(repo.StateDir, conversationGraphFile), &graph)
	if graph.ProjectUUID != "" && repo.ProjectUUID != "" && graph.ProjectUUID != repo.ProjectUUID {
		graph = ConversationGraph{}
	} else if graph.ProjectUUID == "" && !sameProjectPath(graph.ProjectPath, repo.ProjectPath) {
		graph = ConversationGraph{}
	}
	graph.ProjectPath = repo.ProjectPath
	graph.ProjectUUID = repo.ProjectUUID
	graph.ActiveBranch = publicActiveBranch(repo)
	graph.ActiveWorktree = activeWorktree(repo)
	if graph.Nodes == nil {
		graph.Nodes = []ConversationNode{}
	}
	validNodes := make([]ConversationNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		node.Kind = strings.TrimSpace(node.Kind)
		node.CommitID = strings.TrimSpace(node.CommitID)
		if node.ID == "" || node.CommitID == "" {
			continue
		}
		if _, err := readCommit(repo, node.CommitID); err != nil {
			continue
		}
		if node.Kind == "" {
			node.Kind = "checkpoint"
		}
		if strings.TrimSpace(node.Text) == "" {
			node.Text = strings.TrimSpace(node.TextPreview)
		}
		if strings.TrimSpace(node.TextPreview) == "" {
			node.TextPreview = compactPreview(node.Text)
		}
		validNodes = append(validNodes, node)
	}
	graph.Nodes = validNodes
	graph = repairConversationGraphResultCommits(repo, graph)
	if graph.ActiveNodeID != "" {
		found := false
		for _, node := range graph.Nodes {
			if node.ID == graph.ActiveNodeID {
				found = true
				break
			}
		}
		if !found {
			graph.ActiveNodeID = ""
		}
	}
	return graph
}

func conversationMessagesForGraph(graph ConversationGraph) []ConversationMessage {
	nodes := activeConversationPath(graph)
	out := make([]ConversationMessage, 0, len(nodes))
	for _, node := range nodes {
		role := ""
		switch strings.TrimSpace(strings.ToLower(node.Kind)) {
		case "ask":
			role = "user"
		case "vit":
			role = "assistant"
		default:
			continue
		}
		content := firstNonEmpty(node.Text, node.TextPreview)
		if content == "" {
			continue
		}
		out = append(out, ConversationMessage{
			Role:             role,
			Content:          content,
			NodeID:           node.ID,
			CommitID:         node.CommitID,
			Branch:           node.Branch,
			Artifacts:        node.Artifacts,
			ProjectCards:     node.ProjectCards,
			MessageData:      conversationMessageData(node.MessageData),
			Lifecycle:        firstNonEmpty(node.Lifecycle, "durable"),
			Persistence:      firstNonEmpty(node.Persistence, "project_history"),
			MessageKind:      firstNonEmpty(node.MessageKind, role),
			TurnID:           firstNonEmpty(node.TurnID, node.RunID, node.GoalID),
			LogicalMessageID: firstNonEmpty(node.LogicalMessageID, node.ID),
			Supersedes:       append([]string(nil), node.Supersedes...),
			CreatedAt:        node.CreatedAt,
		})
	}
	return out
}

func conversationStringValues(value any) []string {
	var raw []any
	switch rows := value.(type) {
	case []any:
		raw = rows
	case []string:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if clean := strings.TrimSpace(row); clean != "" {
				out = append(out, clean)
			}
		}
		return out
	case string:
		if clean := strings.TrimSpace(rows); clean != "" {
			return []string{clean}
		}
		return nil
	default:
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, row := range raw {
		if clean := strings.TrimSpace(fmt.Sprint(row)); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func conversationMessageData(value any) map[string]any {
	row, ok := value.(map[string]any)
	if !ok || len(row) == 0 {
		return nil
	}
	out := make(map[string]any, len(row))
	for key, item := range row {
		if strings.TrimSpace(key) == "" || item == nil {
			continue
		}
		out[key] = item
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func conversationProjectResultRows(value any) []map[string]any {
	switch rows := value.(type) {
	case nil:
		return nil
	case []map[string]any:
		return compactConversationProjectRows(rows)
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if record, ok := row.(map[string]any); ok {
				out = append(out, record)
			}
		}
		return compactConversationProjectRows(out)
	default:
		return nil
	}
}

func compactConversationProjectRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		compact := map[string]any{}
		for _, key := range []string{"id", "kind", "type", "title", "body", "badge", "target", "details", "readiness", "can_audition", "executions"} {
			if value, ok := row[key]; ok && !isEmptyArtifactValue(value) {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func conversationArtifactRows(value any) []map[string]any {
	switch rows := value.(type) {
	case nil:
		return nil
	case []map[string]any:
		return compactConversationArtifactRows(rows)
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if record, ok := row.(map[string]any); ok {
				out = append(out, compactConversationArtifactRow(record))
			}
		}
		return compactConversationArtifactRows(out)
	default:
		return nil
	}
}

func compactConversationArtifactRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		compact := compactConversationArtifactRow(row)
		id, _ := compact["id"].(string)
		if strings.TrimSpace(id) == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, compact)
	}
	return out
}

func compactConversationArtifactRow(row map[string]any) map[string]any {
	keys := []string{
		"id", "kind", "title", "source", "path", "url", "mime", "size_bytes", "status", "summary",
		"metadata", "created_at", "conversation_id", "goal_id", "run_id",
		"project_path", "root_project_path", "active_worktree", "active_branch", "active_node_id",
		"history_scope_key", "media_scope_key",
	}
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		if value, ok := row[key]; ok && !isEmptyArtifactValue(value) {
			out[key] = value
		}
	}
	return out
}

func isEmptyArtifactValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func activeConversationPath(graph ConversationGraph) []ConversationNode {
	if len(graph.Nodes) == 0 {
		return nil
	}
	byID := map[string]ConversationNode{}
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	activeID := strings.TrimSpace(graph.ActiveNodeID)
	if activeID == "" {
		for i := len(graph.Nodes) - 1; i >= 0; i-- {
			kind := strings.TrimSpace(strings.ToLower(graph.Nodes[i].Kind))
			if kind == "ask" || kind == "vit" {
				activeID = graph.Nodes[i].ID
				break
			}
		}
	}
	if activeID == "" {
		return graph.Nodes
	}
	path := []ConversationNode{}
	seen := map[string]bool{}
	for activeID != "" && !seen[activeID] {
		seen[activeID] = true
		node, ok := byID[activeID]
		if !ok {
			break
		}
		path = append(path, node)
		activeID = strings.TrimSpace(node.ParentNodeID)
	}
	if len(path) == 0 {
		return graph.Nodes
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func repairConversationGraphResultCommits(repo Repo, graph ConversationGraph) ConversationGraph {
	commits, err := listCommits(repo)
	if err != nil || len(commits) == 0 || len(graph.Nodes) == 0 {
		return graph
	}
	sort.SliceStable(commits, func(i, j int) bool { return commits[i].CreatedAt.Before(commits[j].CreatedAt) })
	used := map[string]bool{}
	for i, node := range graph.Nodes {
		if strings.TrimSpace(node.Kind) != "vit" {
			continue
		}
		current, err := readCommit(repo, node.CommitID)
		if err == nil && conversationNodeCommitLooksFinal(current) {
			continue
		}
		nextVitAt := time.Time{}
		for j := i + 1; j < len(graph.Nodes); j++ {
			next := graph.Nodes[j]
			if strings.TrimSpace(next.Kind) == "vit" && strings.TrimSpace(next.Branch) == strings.TrimSpace(node.Branch) {
				nextVitAt = next.CreatedAt
				break
			}
		}
		for _, commit := range commits {
			if used[commit.ID] || commit.Source != "branch_rail_result" {
				continue
			}
			if strings.TrimSpace(commit.Branch) != strings.TrimSpace(node.Branch) {
				continue
			}
			if !commit.CreatedAt.After(node.CreatedAt) {
				continue
			}
			if !nextVitAt.IsZero() && !commit.CreatedAt.Before(nextVitAt) {
				continue
			}
			if commit.CreatedAt.Sub(node.CreatedAt) > 30*time.Minute {
				continue
			}
			graph.Nodes[i].CommitID = commit.ID
			used[commit.ID] = true
			break
		}
	}
	return graph
}

func conversationNodeCommitLooksFinal(commit Commit) bool {
	if commit.Source == "branch_rail_result" {
		return true
	}
	if commit.Source == "conversation_graph" && strings.HasPrefix(strings.TrimSpace(commit.Message), "Vit:") {
		return true
	}
	return false
}

func writeConversationGraph(repo Repo, graph ConversationGraph) error {
	graph.ProjectPath = repo.ProjectPath
	graph.ProjectUUID = repo.ProjectUUID
	graph.ActiveBranch = firstNonEmpty(graph.ActiveBranch, publicActiveBranch(repo))
	graph.ActiveWorktree = firstNonEmpty(graph.ActiveWorktree, activeWorktree(repo))
	if graph.Nodes == nil {
		graph.Nodes = []ConversationNode{}
	}
	return writeJSON(filepath.Join(repo.StateDir, conversationGraphFile), graph)
}

func graphNodeForCheckout(graph ConversationGraph, commitID, branch string) string {
	commitID = strings.TrimSpace(commitID)
	branch = strings.TrimSpace(branch)
	if commitID == "" && branch == "" {
		return ""
	}
	if branch != "" {
		for i := len(graph.Nodes) - 1; i >= 0; i-- {
			node := graph.Nodes[i]
			if node.Branch == branch && node.CommitID == commitID {
				return node.ID
			}
		}
		for i := len(graph.Nodes) - 1; i >= 0; i-- {
			node := graph.Nodes[i]
			if node.Branch == branch {
				return node.ID
			}
		}
	}
	for i := len(graph.Nodes) - 1; i >= 0; i-- {
		node := graph.Nodes[i]
		if node.CommitID == commitID {
			return node.ID
		}
	}
	return ""
}

func activeWorktree(repo Repo) string {
	meta := map[string]any{}
	if err := readJSON(filepath.Join(repo.ProjectDir, ".vit_worktree.json"), &meta); err != nil {
		return ""
	}
	name := safeName(fmt.Sprint(meta["name"]))
	if name == "<nil>" {
		return ""
	}
	return name
}

func rootProjectPath(repo Repo) string {
	meta := map[string]any{}
	if err := readJSON(filepath.Join(repo.ProjectDir, ".vit_worktree.json"), &meta); err != nil {
		return ""
	}
	root := strings.TrimSpace(fmt.Sprint(meta["project_path"]))
	if root == "" || root == "<nil>" || sameProjectPath(root, repo.ProjectPath) {
		return ""
	}
	return root
}

func listWorktreeItems(repo Repo) []map[string]any {
	if root := rootProjectPath(repo); root != "" {
		if rootRepo, err := Open(root); err == nil {
			return listWorktreeItems(rootRepo)
		}
	}
	ensureRootWorktreeItem(repo)
	items := []map[string]any{}
	seen := map[string]bool{}
	rootItem := rootWorktreeItem(repo)
	items = append(items, rootItem)
	seen[worktreeItemKey(rootItem)] = true
	appendWorktreeItems(repo, visibleWorktreeRoot(repo), &items, seen)
	appendWorktreeItems(repo, filepath.Join(repo.HistoryDir, "worktrees"), &items, seen)
	appendWorktreeItems(repo, filepath.Join(repo.StateDir, "worktrees"), &items, seen)
	sort.SliceStable(items, func(i, j int) bool {
		if isTrueValue(items[i]["is_root"]) != isTrueValue(items[j]["is_root"]) {
			return isTrueValue(items[i]["is_root"])
		}
		return fmt.Sprint(items[i]["name"]) < fmt.Sprint(items[j]["name"])
	})
	return items
}

func ensureRootWorktreeItem(repo Repo) {
	if rootProjectPath(repo) != "" {
		return
	}
	target := filepath.Join(repo.StateDir, "worktrees", rootWorktreeName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return
	}
	meta := rootWorktreeItem(repo)
	existing := map[string]any{}
	_ = readJSON(filepath.Join(target, ".vit_worktree.json"), &existing)
	if createdAt := strings.TrimSpace(fmt.Sprint(existing["created_at"])); createdAt != "" && createdAt != "<nil>" {
		meta["created_at"] = existing["created_at"]
	}
	_ = writeJSON(filepath.Join(target, ".vit_worktree.json"), meta)
}

func rootWorktreeItem(repo Repo) map[string]any {
	head := currentHead(repo)
	meta := map[string]any{
		"name":              rootWorktreeName,
		"label":             "主工作树",
		"is_root":           true,
		"path":              repo.ProjectDir,
		"project_file_path": repo.ProjectPath,
		"project_path":      repo.ProjectPath,
		"root_project_path": repo.ProjectPath,
		"created_at":        time.Now().UTC(),
	}
	if head != "" {
		meta["commit_id"] = head
		meta["origin_commit_id"] = head
	}
	return meta
}

func rootRepoForWorktrees(repo Repo) Repo {
	if root := rootProjectPath(repo); root != "" {
		if rootRepo, err := Open(root); err == nil {
			return rootRepo
		}
	}
	return repo
}

func visibleWorktreeRoot(repo Repo) string {
	root := filepath.Join(repo.ProjectDir, worktreesDirName)
	if strings.TrimSpace(repo.ProjectUUID) == "" {
		return root
	}
	base := safeFileName(strings.TrimSuffix(filepath.Base(repo.ProjectPath), filepath.Ext(repo.ProjectPath)))
	if base == "" {
		base = "Project"
	}
	return filepath.Join(root, base+"_"+safeFileName(repo.ProjectUUID))
}

func worktreeProjectFileName(repo Repo, worktreeName string) string {
	ext := filepath.Ext(repo.ProjectPath)
	if ext == "" {
		ext = ".vit"
	}
	base := strings.TrimSuffix(filepath.Base(repo.ProjectPath), filepath.Ext(repo.ProjectPath))
	base = safeFileName(base)
	if base == "" {
		base = "Project"
	}
	suffix := safeFileName(worktreeName)
	if suffix == "" {
		suffix = shortID()
	}
	return base + "_" + suffix + ext
}

func safeFileName(name string) string {
	name = safeName(name)
	replacer := strings.NewReplacer(":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return strings.Trim(replacer.Replace(name), " .")
}

func worktreeItemKey(meta map[string]any) string {
	key := strings.TrimSpace(fmt.Sprint(meta["project_file_path"]))
	if key == "" || key == "<nil>" {
		key = strings.TrimSpace(fmt.Sprint(meta["path"]))
	}
	if key == "" || key == "<nil>" {
		key = strings.TrimSpace(fmt.Sprint(meta["name"]))
	}
	return strings.ToLower(filepath.Clean(key))
}

func appendWorktreeItems(repo Repo, root string, items *[]map[string]any, seen map[string]bool) {
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		actualPath := filepath.Join(root, entry.Name())
		meta := map[string]any{}
		_ = readJSON(filepath.Join(actualPath, ".vit_worktree.json"), &meta)
		if len(meta) == 0 {
			meta["name"] = entry.Name()
			meta["path"] = actualPath
		}
		path := strings.TrimSpace(fmt.Sprint(meta["path"]))
		if path == "" || path == "<nil>" {
			path = actualPath
			meta["path"] = path
		}
		if name := strings.TrimSpace(fmt.Sprint(meta["name"])); name == "" || name == "<nil>" {
			meta["name"] = entry.Name()
		}
		projectFilePath := strings.TrimSpace(fmt.Sprint(meta["project_file_path"]))
		if projectFilePath == "" || projectFilePath == "<nil>" {
			projectFilePath = filepath.Join(path, filepath.Base(repo.ProjectPath))
			meta["project_file_path"] = projectFilePath
		}
		if !fileExists(projectFilePath) {
			if repaired, ok := repairWorktreeProjectFilePath(repo, actualPath, projectFilePath); ok {
				path = actualPath
				projectFilePath = repaired
				meta["path"] = actualPath
				meta["project_file_path"] = repaired
				_ = writeJSON(filepath.Join(actualPath, ".vit_worktree.json"), meta)
			}
		}
		if originID := strings.TrimSpace(fmt.Sprint(meta["origin_commit_id"])); originID == "" || originID == "<nil>" {
			if commitID := strings.TrimSpace(fmt.Sprint(meta["commit_id"])); commitID != "" && commitID != "<nil>" {
				meta["origin_commit_id"] = commitID
			}
		}
		if !worktreeBelongsToProject(repo, meta) {
			continue
		}
		key := worktreeItemKey(meta)
		if key == "" || key == "." {
			key = entry.Name()
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		*items = append(*items, meta)
	}
}

func historyWarnings(repo Repo) []string {
	warnings := []string{}
	if repo.Draft {
		warnings = append(warnings, "未保存工程正在使用草稿历史")
	}
	head := currentHead(repo)
	if head != "" {
		if _, err := readCommit(repo, head); err != nil {
			warnings = append(warnings, "HEAD points to a missing checkpoint: "+head)
		}
	}
	if !isDetached(repo) {
		branch := activeBranch(repo)
		branchHead := readRef(repo, filepath.Join("refs", "heads", branch))
		if currentHead(repo) != "" && branchHead != "" && branchHead != currentHead(repo) {
			warnings = append(warnings, "active branch head differs from HEAD")
		}
	}
	return warnings
}

func repairWorktreeProjectFilePath(repo Repo, actualPath, oldProjectFilePath string) (string, bool) {
	candidateNames := []string{}
	if base := strings.TrimSpace(filepath.Base(oldProjectFilePath)); base != "" && base != "." {
		candidateNames = append(candidateNames, base)
	}
	if base := strings.TrimSpace(filepath.Base(repo.ProjectPath)); base != "" && base != "." {
		candidateNames = append(candidateNames, base)
	}
	candidateNames = append(candidateNames, draftProjectFileName)
	seen := map[string]bool{}
	for _, name := range candidateNames {
		name = strings.TrimSpace(name)
		lower := strings.ToLower(name)
		if name == "" || seen[lower] {
			continue
		}
		seen[lower] = true
		candidate := filepath.Join(actualPath, name)
		if fileExists(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func normalizeMaterializedWorktreeProject(targetDir string, commit Commit, desiredProjectFilePath string) error {
	materializedProjectPath := filepath.Join(targetDir, filepath.FromSlash(firstNonEmpty(commit.ProjectFile.Path, filepath.Base(desiredProjectFilePath))))
	if !sameProjectPath(materializedProjectPath, desiredProjectFilePath) && fileExists(materializedProjectPath) {
		if err := moveFile(materializedProjectPath, desiredProjectFilePath); err != nil {
			return err
		}
	}
	oldMediaDir := filepath.Join(targetDir, strings.TrimSuffix(filepath.Base(materializedProjectPath), filepath.Ext(materializedProjectPath))+"_Media")
	newMediaDir := filepath.Join(targetDir, strings.TrimSuffix(filepath.Base(desiredProjectFilePath), filepath.Ext(desiredProjectFilePath))+"_Media")
	if !sameProjectPath(oldMediaDir, newMediaDir) && dirExists(oldMediaDir) {
		if dirExists(newMediaDir) {
			if err := copyDir(oldMediaDir, newMediaDir); err != nil {
				return err
			}
			return os.RemoveAll(oldMediaDir)
		}
		if err := os.MkdirAll(filepath.Dir(newMediaDir), 0o755); err != nil {
			return err
		}
		return os.Rename(oldMediaDir, newMediaDir)
	}
	return nil
}

func activeBranch(repo Repo) string {
	if isDetached(repo) {
		return ""
	}
	branch := safeName(readProjectRefValue(repo, "ACTIVE_BRANCH"))
	if branch != "" {
		return branch
	}
	branch = safeName(readLegacyRefValue(repo, "ACTIVE_BRANCH"))
	if branch != "" && refValueBelongsToProject(repo, readLegacyRefValue(repo, filepath.Join("refs", "heads", branch))) {
		return branch
	}
	return "main"
}

func publicActiveBranch(repo Repo) string {
	if isDetached(repo) {
		return "detached"
	}
	return activeBranch(repo)
}

func isDetached(repo Repo) bool {
	if strings.EqualFold(readProjectRefValue(repo, "DETACHED"), "true") {
		return true
	}
	return strings.EqualFold(readLegacyRefValue(repo, "DETACHED"), "true") && refValueBelongsToProject(repo, readLegacyRefValue(repo, "HEAD"))
}

func currentHead(repo Repo) string {
	head := readRef(repo, "HEAD")
	if head != "" || isDetached(repo) {
		return head
	}
	return readRef(repo, filepath.Join("refs", "heads", activeBranch(repo)))
}

func advanceCurrentHead(repo Repo, commitID string) error {
	if err := writeRef(repo, "HEAD", commitID); err != nil {
		return err
	}
	if isDetached(repo) {
		return nil
	}
	branch := activeBranch(repo)
	if err := writeRef(repo, filepath.Join("refs", "heads", branch), commitID); err != nil {
		return err
	}
	return writeActiveBranch(repo, branch, false)
}

func writeActiveBranch(repo Repo, branch string, detached bool) error {
	if detached {
		if err := writeRef(repo, "ACTIVE_BRANCH", ""); err != nil {
			return err
		}
		return writeRef(repo, "DETACHED", "true")
	}
	branch = safeName(firstNonEmpty(branch, "main"))
	if err := writeRef(repo, "ACTIVE_BRANCH", branch); err != nil {
		return err
	}
	_ = os.Remove(refPath(repo.StateDir, "DETACHED"))
	return nil
}

func projectKey(projectPath string) string {
	base := safeName(strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath)))
	if base == "" {
		base = "project"
	}
	sum := sha256.Sum256([]byte(normalizeProjectPath(projectPath)))
	return base + "_" + hex.EncodeToString(sum[:])[:12]
}

func defaultDraftProjectPath() (string, error) {
	root := strings.TrimSpace(os.Getenv("VIT_HISTORY_DRAFT_ROOT"))
	if root == "" {
		if cacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(cacheDir) != "" {
			root = filepath.Join(cacheDir, "Vit", "ProjectHistory", "drafts")
		}
	}
	if root == "" {
		root = filepath.Join(os.TempDir(), "Vit", "ProjectHistory", "drafts")
	}
	draftProject.Lock()
	defer draftProject.Unlock()
	if draftProject.path != "" && draftProject.root == root {
		return draftProject.path, nil
	}
	sessionDir := "draft_" + time.Now().UTC().Format("20060102T150405") + "_" + shortID()
	path := filepath.Join(root, sessionDir, draftProjectFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if !fileExists(path) {
		if err := os.WriteFile(path, []byte("<EDIT draft=\"true\" projectID=\"0/0\" appVersion=\"Unknown\" creationTime=\"0\"/>\n"), 0o644); err != nil {
			return "", err
		}
	}
	draftProject.root = root
	draftProject.path = path
	return path, nil
}

func projectLabel(repo Repo) string {
	if repo.Draft {
		return "未保存工程"
	}
	name := strings.TrimSuffix(filepath.Base(repo.ProjectPath), filepath.Ext(repo.ProjectPath))
	if name == "" || name == "." {
		return filepath.Base(repo.ProjectPath)
	}
	return name
}

func normalizeProjectPath(projectPath string) string {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return ""
	}
	if abs, err := filepath.Abs(projectPath); err == nil {
		projectPath = abs
	}
	return strings.ToLower(filepath.ToSlash(filepath.Clean(projectPath)))
}

func sameProjectPath(a, b string) bool {
	return normalizeProjectPath(a) != "" && normalizeProjectPath(a) == normalizeProjectPath(b)
}

func commitBelongsToProject(repo Repo, commit Commit) bool {
	if repo.ProjectUUID != "" && commit.ProjectUUID != "" {
		return repo.ProjectUUID == commit.ProjectUUID
	}
	return sameProjectPath(commit.ProjectPath, repo.ProjectPath)
}

func refValueBelongsToProject(repo Repo, id string) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	commit, err := readCommit(repo, id)
	return err == nil && commitBelongsToProject(repo, commit)
}

func worktreeBelongsToProject(repo Repo, meta map[string]any) bool {
	projectPath := strings.TrimSpace(fmt.Sprint(meta["project_path"]))
	if projectPath != "" && projectPath != "<nil>" {
		return sameProjectPath(projectPath, repo.ProjectPath)
	}
	commitID := strings.TrimSpace(fmt.Sprint(meta["commit_id"]))
	if commitID == "" || commitID == "<nil>" {
		return false
	}
	return refValueBelongsToProject(repo, commitID)
}

func projectPath(args map[string]any) string {
	return firstNonEmpty(value(args, "project_path"), value(args, "path"))
}

func value(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v := strings.TrimSpace(fmt.Sprint(args[key]))
	if v == "<nil>" {
		return ""
	}
	return v
}

func isFalseValue(v any) bool {
	switch typed := v.(type) {
	case bool:
		return !typed
	case string:
		s := strings.TrimSpace(strings.ToLower(typed))
		return s == "false" || s == "0" || s == "no" || s == "off"
	default:
		s := strings.TrimSpace(strings.ToLower(fmt.Sprint(v)))
		return s == "false" || s == "0" || s == "no" || s == "off"
	}
}

func isTrueValue(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		s := strings.TrimSpace(strings.ToLower(typed))
		return s == "true" || s == "1" || s == "yes" || s == "on"
	default:
		s := strings.TrimSpace(strings.ToLower(fmt.Sprint(v)))
		return s == "true" || s == "1" || s == "yes" || s == "on"
	}
}

func compactPreview(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	const limit = 160
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func currentDraftProjectPath() string {
	draftProject.Lock()
	defer draftProject.Unlock()
	return draftProject.path
}

func historyHasUserData(repo Repo) bool {
	commits, err := listCommits(repo)
	if err == nil && len(commits) > 0 {
		return true
	}
	if fileExists(filepath.Join(repo.StateDir, agentRuntimeStateFile)) || currentHead(repo) != "" || len(listRefs(repo)) > 0 {
		return true
	}
	graph := readConversationGraphOrDefault(repo)
	return len(graph.Nodes) > 0
}

func rewriteAdoptedDraftHistory(repo Repo, draftRepo Repo) error {
	if err := rewriteAdoptedCommitFiles(repo, draftRepo); err != nil {
		return err
	}
	if err := rewriteAdoptedConversationGraph(repo, draftRepo); err != nil {
		return err
	}
	if err := rewriteAgentRuntimeIdentity(repo); err != nil {
		return err
	}
	if err := rewriteAdoptedWorktreeMetadata(repo, draftRepo); err != nil {
		return err
	}
	return writeWorkspaceIdentity(repo.HistoryDir, repo.ProjectPath, repo.ProjectUUID)
}

func rewriteAgentRuntimeIdentity(repo Repo) error {
	path := filepath.Join(repo.StateDir, agentRuntimeStateFile)
	state := map[string]any{}
	if err := readJSON(path, &state); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	state["project_path"] = repo.ProjectPath
	state["project_uuid"] = repo.ProjectUUID
	return writeJSON(path, state)
}

func adoptCopiedStateDir(repo Repo, draftRepo Repo) error {
	copiedDraftStateDir := filepath.Join(repo.HistoryDir, "projects", projectKey(draftRepo.ProjectPath))
	if sameProjectPath(copiedDraftStateDir, repo.StateDir) || !dirExists(copiedDraftStateDir) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(repo.StateDir), 0o755); err != nil {
		return err
	}
	if dirExists(repo.StateDir) {
		if err := os.RemoveAll(repo.StateDir); err != nil {
			return err
		}
	}
	return os.Rename(copiedDraftStateDir, repo.StateDir)
}

func adoptVisibleWorktrees(repo Repo, draftRepo Repo) error {
	srcRoot := visibleWorktreeRoot(draftRepo)
	if !dirExists(srcRoot) {
		return nil
	}
	dstRoot := visibleWorktreeRoot(repo)
	entries, err := os.ReadDir(srcRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if err := copyDir(filepath.Join(srcRoot, entry.Name()), filepath.Join(dstRoot, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func rewriteAdoptedCommitFiles(repo Repo, draftRepo Repo) error {
	dir := filepath.Join(repo.HistoryDir, "commits")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	oldProjectBase := filepath.Base(draftRepo.ProjectPath)
	newProjectBase := filepath.Base(repo.ProjectPath)
	oldMediaBase := strings.TrimSuffix(oldProjectBase, filepath.Ext(oldProjectBase)) + "_Media"
	newMediaBase := strings.TrimSuffix(newProjectBase, filepath.Ext(newProjectBase)) + "_Media"
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var commit Commit
		if err := readJSON(path, &commit); err != nil {
			return err
		}
		commit.ProjectPath = repo.ProjectPath
		commit.ProjectUUID = repo.ProjectUUID
		if sameSlashPath(commit.ProjectFile.Path, oldProjectBase) || commit.ProjectFile.Kind == "project" || commit.ProjectFile.Kind == "project_snapshot" {
			commit.ProjectFile.Path = filepath.ToSlash(newProjectBase)
		}
		for i := range commit.Files {
			commit.Files[i].Path = rewriteAdoptedRelativePath(commit.Files[i].Path, oldMediaBase, newMediaBase)
		}
		if err := writeJSON(path, commit); err != nil {
			return err
		}
	}
	return nil
}

func rewriteAdoptedConversationGraph(repo Repo, _ Repo) error {
	path := filepath.Join(repo.StateDir, conversationGraphFile)
	var graph ConversationGraph
	if err := readJSON(path, &graph); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	graph.ProjectPath = repo.ProjectPath
	graph.ProjectUUID = repo.ProjectUUID
	graph.ActiveWorktree = ""
	if graph.ActiveBranch == "" {
		graph.ActiveBranch = publicActiveBranch(repo)
	}
	return writeConversationGraph(repo, graph)
}

func rewriteAdoptedWorktreeMetadata(repo Repo, draftRepo Repo) error {
	roots := []string{
		visibleWorktreeRoot(repo),
		filepath.Join(repo.HistoryDir, "worktrees"),
		filepath.Join(repo.StateDir, "worktrees"),
	}
	for _, root := range roots {
		if !dirExists(root) {
			continue
		}
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Base(path) != ".vit_worktree.json" {
				return err
			}
			meta := map[string]any{}
			if err := readJSON(path, &meta); err != nil {
				return err
			}
			if err := rewriteAdoptedWorktreeMeta(repo, draftRepo, meta); err != nil {
				return err
			}
			return writeJSON(path, meta)
		}); err != nil {
			return err
		}
	}
	return nil
}

func rewriteAdoptedWorktreeMeta(repo Repo, draftRepo Repo, meta map[string]any) error {
	oldProjectFilePath := strings.TrimSpace(fmt.Sprint(meta["project_file_path"]))
	meta["root_project_path"] = repo.ProjectPath
	if sameProjectPath(fmt.Sprint(meta["project_path"]), draftRepo.ProjectPath) || strings.TrimSpace(fmt.Sprint(meta["project_path"])) == "" {
		meta["project_path"] = repo.ProjectPath
	}
	if sameProjectPath(fmt.Sprint(meta["project_file_path"]), draftRepo.ProjectPath) {
		meta["project_file_path"] = repo.ProjectPath
	}
	for _, key := range []string{"path", "project_file_path"} {
		value := strings.TrimSpace(fmt.Sprint(meta[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		value = replacePathPrefix(value, visibleWorktreeRoot(draftRepo), visibleWorktreeRoot(repo))
		value = replacePathPrefix(value, draftRepo.ProjectDir, repo.ProjectDir)
		value = strings.ReplaceAll(value, filepath.Clean(draftRepo.StateDir), filepath.Clean(repo.StateDir))
		value = strings.ReplaceAll(value, filepath.ToSlash(filepath.Clean(draftRepo.StateDir)), filepath.ToSlash(filepath.Clean(repo.StateDir)))
		value = strings.ReplaceAll(value, filepath.Clean(draftRepo.HistoryDir), filepath.Clean(repo.HistoryDir))
		value = strings.ReplaceAll(value, filepath.ToSlash(filepath.Clean(draftRepo.HistoryDir)), filepath.ToSlash(filepath.Clean(repo.HistoryDir)))
		meta[key] = value
	}
	if isTrueValue(meta["is_root"]) {
		meta["path"] = repo.ProjectDir
		meta["project_file_path"] = repo.ProjectPath
		meta["project_path"] = repo.ProjectPath
		return nil
	}
	name := safeName(fmt.Sprint(meta["name"]))
	worktreePath := strings.TrimSpace(fmt.Sprint(meta["path"]))
	projectFilePath := strings.TrimSpace(fmt.Sprint(meta["project_file_path"]))
	if name != "" && worktreePath != "" && worktreePath != "<nil>" && pathWithin(worktreePath, visibleWorktreeRoot(repo)) {
		oldCopiedProjectFilePath := replacePathPrefix(oldProjectFilePath, visibleWorktreeRoot(draftRepo), visibleWorktreeRoot(repo))
		oldCopiedProjectFilePath = replacePathPrefix(oldCopiedProjectFilePath, draftRepo.ProjectDir, repo.ProjectDir)
		if oldCopiedProjectFilePath == "" || oldCopiedProjectFilePath == "<nil>" {
			oldCopiedProjectFilePath = projectFilePath
		}
		desiredProjectFilePath := filepath.Join(worktreePath, worktreeProjectFileName(repo, name))
		if fileExists(oldCopiedProjectFilePath) && !sameProjectPath(oldCopiedProjectFilePath, desiredProjectFilePath) {
			if err := moveWorktreeProjectBundle(oldCopiedProjectFilePath, desiredProjectFilePath); err != nil {
				return err
			}
		}
		meta["project_file_path"] = desiredProjectFilePath
		if err := rewriteWorktreeProjectHistory(desiredProjectFilePath, oldProjectFilePath, oldCopiedProjectFilePath); err != nil {
			return err
		}
	}
	return nil
}

func rewriteAdoptedRelativePath(path, oldMediaBase, newMediaBase string) string {
	slash := filepath.ToSlash(path)
	prefix := filepath.ToSlash(oldMediaBase) + "/"
	if strings.HasPrefix(slash, prefix) {
		return filepath.ToSlash(newMediaBase) + "/" + strings.TrimPrefix(slash, prefix)
	}
	return slash
}

func rewriteWorktreeProjectHistory(projectFilePath string, oldProjectPaths ...string) error {
	repo, err := Open(projectFilePath)
	if err != nil {
		return err
	}
	for _, oldProjectPath := range oldProjectPaths {
		oldProjectPath = strings.TrimSpace(oldProjectPath)
		if oldProjectPath == "" || oldProjectPath == "<nil>" || sameProjectPath(oldProjectPath, repo.ProjectPath) {
			continue
		}
		oldStateDir := filepath.Join(repo.HistoryDir, "projects", projectKey(oldProjectPath))
		if dirExists(oldStateDir) && !sameProjectPath(oldStateDir, repo.StateDir) {
			if !dirExists(repo.StateDir) {
				if err := os.MkdirAll(filepath.Dir(repo.StateDir), 0o755); err != nil {
					return err
				}
				if err := os.Rename(oldStateDir, repo.StateDir); err != nil {
					return err
				}
			} else if err := copyDir(oldStateDir, repo.StateDir); err != nil {
				return err
			}
		}
	}
	if err := rewriteHistoryCommitsForProject(repo, oldProjectPaths...); err != nil {
		return err
	}
	graphPath := filepath.Join(repo.StateDir, conversationGraphFile)
	var graph ConversationGraph
	if err := readJSON(graphPath, &graph); err == nil {
		graph.ProjectPath = repo.ProjectPath
		if graph.ActiveBranch == "" {
			graph.ActiveBranch = publicActiveBranch(repo)
		}
		graph.ActiveWorktree = activeWorktree(repo)
		if err := writeJSON(graphPath, graph); err != nil {
			return err
		}
	}
	return nil
}

func rewriteHistoryCommitsForProject(repo Repo, oldProjectPaths ...string) error {
	dir := filepath.Join(repo.HistoryDir, "commits")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	newProjectBase := filepath.Base(repo.ProjectPath)
	newMediaBase := strings.TrimSuffix(newProjectBase, filepath.Ext(newProjectBase)) + "_Media"
	oldBases := map[string]string{}
	for _, oldProjectPath := range oldProjectPaths {
		oldProjectPath = strings.TrimSpace(oldProjectPath)
		if oldProjectPath == "" || oldProjectPath == "<nil>" {
			continue
		}
		oldProjectBase := filepath.Base(oldProjectPath)
		oldMediaBase := strings.TrimSuffix(oldProjectBase, filepath.Ext(oldProjectBase)) + "_Media"
		oldBases[oldProjectBase] = oldMediaBase
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var commit Commit
		if err := readJSON(path, &commit); err != nil {
			return err
		}
		shouldRewrite := false
		for _, oldProjectPath := range oldProjectPaths {
			if sameProjectPath(commit.ProjectPath, oldProjectPath) {
				shouldRewrite = true
				break
			}
		}
		if !shouldRewrite {
			continue
		}
		commit.ProjectPath = repo.ProjectPath
		if commit.ProjectFile.Kind == "project" || commit.ProjectFile.Kind == "project_snapshot" {
			commit.ProjectFile.Path = filepath.ToSlash(newProjectBase)
		}
		for i := range commit.Files {
			for _, oldMediaBase := range oldBases {
				commit.Files[i].Path = rewriteAdoptedRelativePath(commit.Files[i].Path, oldMediaBase, newMediaBase)
			}
		}
		if err := writeJSON(path, commit); err != nil {
			return err
		}
	}
	return nil
}

func moveWorktreeProjectBundle(oldProjectFilePath, newProjectFilePath string) error {
	if err := moveFile(oldProjectFilePath, newProjectFilePath); err != nil {
		return err
	}
	oldMediaDir := strings.TrimSuffix(oldProjectFilePath, filepath.Ext(oldProjectFilePath)) + "_Media"
	newMediaDir := strings.TrimSuffix(newProjectFilePath, filepath.Ext(newProjectFilePath)) + "_Media"
	if !sameProjectPath(oldMediaDir, newMediaDir) && dirExists(oldMediaDir) {
		if dirExists(newMediaDir) {
			if err := copyDir(oldMediaDir, newMediaDir); err != nil {
				return err
			}
			return os.RemoveAll(oldMediaDir)
		}
		if err := os.MkdirAll(filepath.Dir(newMediaDir), 0o755); err != nil {
			return err
		}
		return os.Rename(oldMediaDir, newMediaDir)
	}
	return nil
}

func replacePathPrefix(path, oldRoot, newRoot string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "<nil>" {
		return path
	}
	oldClean := filepath.Clean(oldRoot)
	newClean := filepath.Clean(newRoot)
	pathClean := filepath.Clean(path)
	if strings.EqualFold(pathClean, oldClean) {
		return newClean
	}
	prefix := oldClean + string(os.PathSeparator)
	if strings.HasPrefix(strings.ToLower(pathClean), strings.ToLower(prefix)) {
		return filepath.Join(newClean, strings.TrimPrefix(pathClean[len(oldClean):], string(os.PathSeparator)))
	}
	oldSlash := filepath.ToSlash(oldClean)
	pathSlash := filepath.ToSlash(pathClean)
	if strings.EqualFold(pathSlash, oldSlash) {
		return filepath.ToSlash(newClean)
	}
	prefixSlash := oldSlash + "/"
	if strings.HasPrefix(strings.ToLower(pathSlash), strings.ToLower(prefixSlash)) {
		return filepath.Join(newClean, filepath.FromSlash(strings.TrimPrefix(pathSlash, prefixSlash)))
	}
	return path
}

func pathWithin(path, root string) bool {
	path = strings.TrimSpace(path)
	root = strings.TrimSpace(root)
	if path == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func sameSlashPath(a, b string) bool {
	return strings.EqualFold(filepath.ToSlash(filepath.Clean(strings.TrimSpace(a))), filepath.ToSlash(filepath.Clean(strings.TrimSpace(b))))
}

func migrateLegacyHistoryWorkspace(repo Repo, historyRoot string) error {
	if repo.ProjectUUID == "" || sameProjectPath(repo.HistoryDir, historyRoot) {
		return nil
	}
	marker := filepath.Join(repo.HistoryDir, "workspace.json")
	if fileExists(marker) || dirExists(repo.StateDir) {
		return nil
	}

	identityMigrationMu.Lock()
	defer identityMigrationMu.Unlock()
	if fileExists(marker) || dirExists(repo.StateDir) {
		return nil
	}

	legacyStateDir := filepath.Join(historyRoot, "projects", projectKey(repo.ProjectPath))
	hasLegacyState := dirExists(legacyStateDir)
	hasLegacyCommits := dirExists(filepath.Join(historyRoot, "commits"))
	if !hasLegacyState && !hasLegacyCommits {
		return nil
	}
	if err := os.MkdirAll(repo.HistoryDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{"commits", "objects", "refs", "worktrees"} {
		src := filepath.Join(historyRoot, name)
		if dirExists(src) {
			if err := copyDir(src, filepath.Join(repo.HistoryDir, name)); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"HEAD", "ACTIVE_BRANCH", "DETACHED"} {
		src := filepath.Join(historyRoot, name)
		if fileExists(src) {
			if err := copyFile(src, filepath.Join(repo.HistoryDir, name)); err != nil {
				return err
			}
		}
	}
	if hasLegacyState {
		if err := copyDir(legacyStateDir, repo.StateDir); err != nil {
			return err
		}
	}
	return writeJSON(marker, map[string]any{
		"schema_version":         "vit_project_workspace.v1",
		"project_uuid":           repo.ProjectUUID,
		"project_path":           repo.ProjectPath,
		"migrated_from_legacy":   true,
		"legacy_history_root":    historyRoot,
		"legacy_project_state":   legacyStateDir,
		"migration_completed_at": time.Now().UTC(),
	})
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func moveFile(src, dst string) error {
	if sameProjectPath(src, dst) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func readJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func safeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "..", "_")
	return strings.Trim(name, " .")
}

func shortID() string {
	sum := sha256.Sum256([]byte(time.Now().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])[:8]
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && v != "<nil>" {
			return v
		}
	}
	return ""
}
