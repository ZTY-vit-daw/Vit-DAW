package carriers

// L2 用户偏好载体（CONTEXT_LAYERING_V1_DESIGN §2.2）：跨工程用户级偏好档
// <workspace>/user_profile.v1.json。聚合键=人（v1 本地单用户，user_id
// 占位 "local"；跨设备同步器归 D14 memory 收尾卡，条目字段已足支撑合并
// 语义）。前缀只渲染活跃集（Class=core|conditional 且 Status=active）；
// 活跃集变化=profile_updated 断裂，由 PrefixService 对前后轮 diff 归因。
//
// 内容盲审查（§8.1 L2 行）：条目必须带 EvidenceRefs+Source——无确权来源
// 不入档（写入器拒绝，装载器对缺失来源的条目拒载+WARN）。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProfileFilename 是 L2 载体文件名（工作区根下，工程包外）。
const ProfileFilename = "user_profile.v1.json"

// UserIDLocal 是 v1 本地单用户的 user_id 占位（§2.2 要点 3）。
const UserIDLocal = "local"

// Class/Status 封闭枚举（§2.2 设计态）。
const (
	ProfileClassCore        = "core"
	ProfileClassConditional = "conditional"

	ProfileStatusActive     = "active"
	ProfileStatusSuperseded = "superseded"
)

// ErrProfileUserMismatch 表示档内 user_id 与期望聚合键失配——非静默沿用
// （T-C2：显式断裂，不上别的用户的档）。
var ErrProfileUserMismatch = errors.New("user_profile user_id mismatch")

// ProfileEntry 是确权偏好条目（八字段+Source，§2.2 设计态签名）。
type ProfileEntry struct {
	PrefID       string    `json:"pref_id"`
	Statement    string    `json:"statement"`
	Class        string    `json:"class"` // core | conditional
	Condition    string    `json:"condition,omitempty"`
	EvidenceRefs []string  `json:"evidence_refs,omitempty"`
	Status       string    `json:"status"` // active | superseded
	SupersededBy string    `json:"superseded_by,omitempty"`
	ConfirmedAt  time.Time `json:"confirmed_at"`
	Source       string    `json:"source"` // 确权票据 ID（D13/L2-3 协议产出；槽位）
}

// ProfileDoc 是 user_profile.v1.json 的整档格式。
type ProfileDoc struct {
	SchemaVersion string         `json:"schema_version"` // user_profile.v1
	UserID        string         `json:"user_id"`        // v1: local
	Entries       []ProfileEntry `json:"entries"`
}

// profileState 是一次装载的结果形态。
type profileState struct {
	doc      *ProfileDoc
	state    string // rendered | absent | corrupt
	warnings []string
	err      error
}

// LoadProfile 装载并校验偏好档：缺文件=空层 absent（旧工程包常态路径，
// §8.3）；JSON/schema 损坏=corrupt；条目级封闭枚举越界=该条拒载+WARN
// （fail-closed，不静默丢弃）；user_id 失配=ErrProfileUserMismatch。
func LoadProfile(workspaceDir string, expectedUserID string) profileState {
	path := filepath.Join(workspaceDir, ProfileFilename)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return profileState{state: "absent"}
		}
		return profileState{state: "corrupt", err: fmt.Errorf("user profile unreadable: %w", err)}
	}
	var doc ProfileDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return profileState{state: "corrupt", err: fmt.Errorf("user profile unparseable: %w", err)}
	}
	if doc.SchemaVersion != "user_profile.v1" {
		return profileState{state: "corrupt", err: fmt.Errorf("user profile schema_version %q != user_profile.v1", doc.SchemaVersion)}
	}
	if strings.TrimSpace(doc.UserID) == "" {
		return profileState{state: "corrupt", err: fmt.Errorf("user profile missing user_id")}
	}
	if expectedUserID != "" && doc.UserID != expectedUserID {
		return profileState{
			state:    "corrupt",
			err:      fmt.Errorf("%w: file=%s expected=%s", ErrProfileUserMismatch, doc.UserID, expectedUserID),
			warnings: []string{fmt.Sprintf("profile user_id mismatch (file=%s, expected=%s): profile layer not rendered, not silently reused", doc.UserID, expectedUserID)},
		}
	}
	state := profileState{doc: &doc, state: "rendered"}
	kept := make([]ProfileEntry, 0, len(doc.Entries))
	for index, entry := range doc.Entries {
		if warning := validateProfileEntry(entry); warning != "" {
			state.warnings = append(state.warnings,
				fmt.Sprintf("profile entry #%d (%s) refused: %s", index+1, entry.PrefID, warning))
			continue
		}
		kept = append(kept, entry)
	}
	state.doc.Entries = kept
	return state
}

// validateProfileEntry 返回拒载理由（空=通过）。封闭枚举 fail-closed；
// 内容盲面（§8.1）：无确权来源（EvidenceRefs/Source 缺失）不入档。
func validateProfileEntry(entry ProfileEntry) string {
	switch entry.Class {
	case ProfileClassCore, ProfileClassConditional:
	default:
		return fmt.Sprintf("class %q outside closed enum {core, conditional}", entry.Class)
	}
	switch entry.Status {
	case ProfileStatusActive, ProfileStatusSuperseded:
	default:
		return fmt.Sprintf("status %q outside closed enum {active, superseded}", entry.Status)
	}
	if strings.TrimSpace(entry.PrefID) == "" || strings.TrimSpace(entry.Statement) == "" {
		return "empty pref_id or statement"
	}
	if entry.Class == ProfileClassConditional && strings.TrimSpace(entry.Condition) == "" {
		return "conditional entry missing condition"
	}
	if len(entry.EvidenceRefs) == 0 || strings.TrimSpace(entry.Source) == "" {
		return "missing evidence refs or confirmation source (no attestation, no profile entry)"
	}
	if entry.Status == ProfileStatusSuperseded && strings.TrimSpace(entry.SupersededBy) == "" {
		return "superseded entry missing superseded_by"
	}
	return ""
}

// ActiveEntries 返回活跃集（Class=core|conditional 且 Status=active），
// 按 PrefID 排序（渲染确定性）。
func (d *ProfileDoc) ActiveEntries() []ProfileEntry {
	active := make([]ProfileEntry, 0, len(d.Entries))
	for _, entry := range d.Entries {
		if entry.Status == ProfileStatusActive {
			active = append(active, entry)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].PrefID < active[j].PrefID })
	return active
}

// RenderProfile 渲染 L2 前缀段内容（活跃集；确定性：排序+固定行格式）。
func RenderProfile(doc *ProfileDoc) string {
	active := doc.ActiveEntries()
	if len(active) == 0 {
		return ""
	}
	rows := make([]string, 0, len(active))
	for _, entry := range active {
		row := fmt.Sprintf("%s [%s]", entry.PrefID, entry.Class)
		if entry.Class == ProfileClassConditional {
			row = fmt.Sprintf("%s [%s: %s]", entry.PrefID, entry.Class, entry.Condition)
		}
		rows = append(rows, row+" "+entry.Statement)
	}
	return "Confirmed user preferences:\n" + strings.Join(rows, "\n")
}

// ProfileCacheKey 是 L2 渲染输入身份（活跃集摘要，§3.2 归因轨）。
func ProfileCacheKey(doc *ProfileDoc) string {
	active := doc.ActiveEntries()
	parts := make([]string, 0, len(active))
	for _, entry := range active {
		parts = append(parts, strings.Join([]string{entry.PrefID, entry.Class, entry.Condition, entry.Statement}, "\x00"))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1e")))
	return "profile:" + hex.EncodeToString(sum[:])
}

// SaveProfile 原子写档（临时文件+rename）。
func SaveProfile(workspaceDir string, doc *ProfileDoc) error {
	if doc.SchemaVersion == "" {
		doc.SchemaVersion = "user_profile.v1"
	}
	if doc.UserID == "" {
		doc.UserID = UserIDLocal
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(workspaceDir, ProfileFilename)
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// AppendProfileEntry 追加一条已确权条目（PrefID 必须全新；无确权来源
// 拒绝写入——"无确权来源不入档"）。文件缺失时建新档（user_id=local）。
func AppendProfileEntry(workspaceDir string, entry ProfileEntry) error {
	state := LoadProfile(workspaceDir, "")
	if state.state == "corrupt" && !errors.Is(state.err, ErrProfileUserMismatch) {
		return state.err
	}
	doc := state.doc
	if doc == nil {
		doc = &ProfileDoc{SchemaVersion: "user_profile.v1", UserID: UserIDLocal}
	}
	for _, existing := range doc.Entries {
		if existing.PrefID == entry.PrefID {
			return fmt.Errorf("profile entry %s already exists (supersede explicitly via SupersedeProfileEntry)", entry.PrefID)
		}
	}
	if entry.Status == "" {
		entry.Status = ProfileStatusActive
	}
	entry.SupersededBy = ""
	if warning := validateProfileEntry(entry); warning != "" {
		return fmt.Errorf("profile entry refused: %s", warning)
	}
	doc.Entries = append(doc.Entries, entry)
	return SaveProfile(workspaceDir, doc)
}

// SupersedeProfileEntry 推翻旧条目并追加新条目：旧条 Status=superseded、
// SupersededBy=新 PrefID（superseded 链保留历史，存储 append 倾向）。
func SupersedeProfileEntry(workspaceDir string, oldPrefID string, newEntry ProfileEntry) error {
	state := LoadProfile(workspaceDir, "")
	if state.state != "rendered" {
		return fmt.Errorf("profile load failed: %v", state.err)
	}
	if newEntry.PrefID == oldPrefID {
		return fmt.Errorf("superseding entry must use a new pref_id (got %s twice)", oldPrefID)
	}
	if newEntry.Status == "" {
		newEntry.Status = ProfileStatusActive
	}
	newEntry.SupersededBy = ""
	if warning := validateProfileEntry(newEntry); warning != "" {
		return fmt.Errorf("profile entry refused: %s", warning)
	}
	replaced := false
	for index := range state.doc.Entries {
		if state.doc.Entries[index].PrefID == oldPrefID {
			if state.doc.Entries[index].Status != ProfileStatusActive {
				return fmt.Errorf("profile entry %s is not active", oldPrefID)
			}
			state.doc.Entries[index].Status = ProfileStatusSuperseded
			state.doc.Entries[index].SupersededBy = newEntry.PrefID
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("profile entry %s not found", oldPrefID)
	}
	state.doc.Entries = append(state.doc.Entries, newEntry)
	return SaveProfile(workspaceDir, state.doc)
}
