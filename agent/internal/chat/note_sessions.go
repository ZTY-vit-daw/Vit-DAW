package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/promptruntime"
)

// VITNOTE-NOTESTREAM-2（2026-10-03）：每 note 独立会话流。
//
// 取证结论（卡内回填）：/agent/chat 把一切消息落工程单图（RecordConversationNodeForProject，
// 与 conversation_id 无关）且逐条走 beginChatGoal 治理链——note 问答因此既进主会话图又被
// webui 主流聚合水合（双重同向）。本文件落 note 会话模式：
//   - 会话键=note_id（载荷 note.session 带 run 戳防跨前端重启撞名；空回落 note_id）；
//   - 不走 beginChatGoal、不写工程单图、不建 durable continuation、不 emitTurnEvent——
//     主任务 goal 上下文与 webui 主流/轨迹零 note 往返（判据 1/3 的机制面）；
//   - 辖区上下文注入：载荷 note.faces（圈选解析摘要）进 note 专属上下文快照，主控台
//     组包（选中轨等）不再混入（三号场 contract_scope=1007 误取的根因面）；
//   - 持久化：.vit_derived/<uuid>/note_sessions.json（AGENTS §11 fail-open：缺文件/损坏
//     JSON/异形行一律按空处理，旧工程加载零破坏；原子写 tmp+rename）。
// 归档语义：note 删除=会话归档（archived=true，可查不入主流——/agent/note/archive）。

const noteSessionsFileName = "note_sessions.json"
const noteSessionsSchemaV1 = "vit_note_sessions.v1"
const noteConversationPrefix = "note_"
const noteSessionHistoryLimit = 12
const noteSessionProjectionMessageLimit = 50

// NoteChatPayload 是 /agent/chat 载荷的 note 对象（Godot vit_note_panel v3 载荷）。
// 出现即进入 note 会话模式（主管道早分叉，见 handleChat）。
type NoteChatPayload struct {
	// NoteID 面板显示名（"note_3" 形，跨重启不保证唯一，仅展示/诊断）。
	NoteID string `json:"note_id"`
	// Session 会话键（run 戳形如 "r1a2b3c4-3"；空→回落 NoteID）。会话 id=note_+Session。
	Session string `json:"session,omitempty"`
	// Title 默认命名（「便签·轨道时间线 72%」式；空→服务端 "便签 "+NoteID 兜底）。用户可
	// 在 webui 侧边栏改名（本地注册表优先，见 webui sessionFlow）。
	Title string `json:"title,omitempty"`
	// Faces 圈选辖区摘要（resolve_circle §5.2 faces[]：face_id/face_kind/label/
	// selection_share/face_coverage/domain）——辖区上下文注入的数据源。
	Faces []map[string]any `json:"faces,omitempty"`
}

type noteSessionMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type NoteSession struct {
	ConversationID string               `json:"conversation_id"`
	NoteID         string               `json:"note_id"`
	Title          string               `json:"title"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	Archived       bool                 `json:"archived"`
	Messages       []noteSessionMessage `json:"messages"`
	Faces          []map[string]any     `json:"faces,omitempty"`
}

type noteSessionsFile struct {
	SchemaVersion string                  `json:"schema_version"`
	ProjectUUID   string                  `json:"project_uuid"`
	Sessions      map[string]*NoteSession `json:"sessions"`
}

// noteConversationID：会话 id=note_+会话键（Session 优先，空回落 NoteID——IMPL-B/C/D 链的
// note_<id> 即天然键，run 戳是 NOTESTREAM-2 新增的防撞层）。
func noteConversationID(payload *NoteChatPayload) string {
	key := strings.TrimSpace(payload.Session)
	if key == "" {
		key = strings.TrimSpace(payload.NoteID)
	}
	return noteConversationPrefix + key
}

// noteSessionStorePath：.vit_derived/<uuid>/note_sessions.json（projectworkspace 侧车惯例）。
func noteSessionStorePath(projectPath, projectUUID string) (string, error) {
	dir, err := projectworkspace.DerivedDir(projectPath, projectUUID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, noteSessionsFileName), nil
}

// loadNoteSessions：fail-open 读取（缺文件/损坏/异形行→空表；AGENTS §11）。未知字段忽略。
func loadNoteSessions(projectPath, projectUUID string) map[string]*NoteSession {
	out := map[string]*NoteSession{}
	path, err := noteSessionStorePath(projectPath, projectUUID)
	if err != nil {
		return out
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var file noteSessionsFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return out
	}
	for id, session := range file.Sessions {
		if session == nil || strings.TrimSpace(session.ConversationID) == "" {
			continue
		}
		if id != session.ConversationID {
			continue
		}
		if session.Messages == nil {
			session.Messages = []noteSessionMessage{}
		}
		out[id] = session
	}
	return out
}

func saveNoteSessions(projectPath, projectUUID string, sessions map[string]*NoteSession) error {
	path, err := noteSessionStorePath(projectPath, projectUUID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(noteSessionsFile{
		SchemaVersion: noteSessionsSchemaV1,
		ProjectUUID:   strings.TrimSpace(projectUUID),
		Sessions:      sessions,
	}, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// noteDefaultTitle：默认命名兜底（载荷 title 空→「便签 note_3」式；正常路径 Godot 送
// 「便签·轨道时间线 72%」式——目标 6 用户裁定方向）。
func noteDefaultTitle(payload *NoteChatPayload) string {
	if title := strings.TrimSpace(payload.Title); title != "" {
		return title
	}
	return "便签 " + strings.TrimSpace(payload.NoteID)
}

// appendNoteSessionExchange：upsert 会话（首问建档+默认名）+追加问答对+落盘。失败返回
// 错误（调用侧答错给面板，不中断已答回合的返回——面板显示连接失败样式）。
func (s *Server) appendNoteSessionExchange(projectPath, projectUUID string, payload *NoteChatPayload, ask, reply string) error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	s.noteSessionMu.Lock()
	defer s.noteSessionMu.Unlock()
	sessions := loadNoteSessions(projectPath, projectUUID)
	conversationID := noteConversationID(payload)
	now := time.Now().UTC()
	session, ok := sessions[conversationID]
	if !ok {
		session = &NoteSession{
			ConversationID: conversationID,
			NoteID:         strings.TrimSpace(payload.NoteID),
			Title:          noteDefaultTitle(payload),
			CreatedAt:      now,
			Faces:          payload.Faces,
		}
		sessions[conversationID] = session
	}
	if session.Title == "" {
		session.Title = noteDefaultTitle(payload)
	}
	session.UpdatedAt = now
	session.Messages = append(session.Messages,
		noteSessionMessage{Role: "user", Content: ask, CreatedAt: now},
		noteSessionMessage{Role: "assistant", Content: reply, CreatedAt: now},
	)
	return saveNoteSessions(projectPath, projectUUID, sessions)
}

// archiveNoteSession：note 删除=归档（可查不入主流）。幂等：未建档/已归档均成功返回。
func (s *Server) archiveNoteSession(projectPath, projectUUID, conversationID string) error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	s.noteSessionMu.Lock()
	defer s.noteSessionMu.Unlock()
	sessions := loadNoteSessions(projectPath, projectUUID)
	session, ok := sessions[strings.TrimSpace(conversationID)]
	if !ok || session.Archived {
		return nil
	}
	session.Archived = true
	session.UpdatedAt = time.Now().UTC()
	return saveNoteSessions(projectPath, projectUUID, sessions)
}

// noteSessionRows：/agent/state 投影（webui 侧边栏会话流合并源）。按 UpdatedAt 降序、
// 同刻按会话 id 字典序（确定性）；messages 截断最近 noteSessionProjectionMessageLimit 条。
func (s *Server) noteSessionRows(projectPath, projectUUID string) []map[string]any {
	if s == nil {
		return nil
	}
	s.noteSessionMu.Lock()
	sessions := loadNoteSessions(projectPath, projectUUID)
	s.noteSessionMu.Unlock()
	ordered := make([]*NoteSession, 0, len(sessions))
	for _, session := range sessions {
		ordered = append(ordered, session)
	}
	// 插入排序按 UpdatedAt 降序+id 决胜（会话数小，无需 sort.Slice 依赖差异）。
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0; j-- {
			a, b := ordered[j-1], ordered[j]
			if a.UpdatedAt.After(b.UpdatedAt) {
				break
			}
			if a.UpdatedAt.Equal(b.UpdatedAt) && a.ConversationID < b.ConversationID {
				break
			}
			ordered[j-1], ordered[j] = ordered[j], ordered[j-1]
		}
	}
	rows := make([]map[string]any, 0, len(ordered))
	for _, session := range ordered {
		messages := session.Messages
		if len(messages) > noteSessionProjectionMessageLimit {
			messages = messages[len(messages)-noteSessionProjectionMessageLimit:]
		}
		rows = append(rows, map[string]any{
			"conversation_id": session.ConversationID,
			"note_id":         session.NoteID,
			"title":           session.Title,
			"archived":        session.Archived,
			"created_at":      session.CreatedAt,
			"updated_at":      session.UpdatedAt,
			"project_path":    strings.TrimSpace(projectPath),
			"project_uuid":    strings.TrimSpace(projectUUID),
			"messages":        messages,
		})
	}
	return rows
}

// handleNoteArchive：POST /agent/note/archive——note 删除生命周期的归档簿记（非 chat 主管道；
// 概念裁定 4 约束的是 note 问答输入走主管道，归档是会话面生命周期记账）。
func (s *Server) handleNoteArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var payload struct {
		ConversationID string `json:"conversation_id"`
		NoteID         string `json:"note_id"`
		Session        string `json:"session"`
		ProjectPath    string `json:"project_path"`
		ProjectUUID    string `json:"project_uuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	conversationID := strings.TrimSpace(payload.ConversationID)
	if conversationID == "" {
		key := strings.TrimSpace(payload.Session)
		if key == "" {
			key = strings.TrimSpace(payload.NoteID)
		}
		if key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "conversation_id or note identity is required"})
			return
		}
		conversationID = noteConversationPrefix + key
	}
	projectPath, projectUUID := s.noteProjectIdentity(r.Context(), payload.ProjectPath, payload.ProjectUUID)
	if err := s.archiveNoteSession(projectPath, projectUUID, conversationID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "conversation_id": conversationID, "archived": true})
}

// noteProjectIdentity：归档/投影用工程身份——载荷显式值优先，回落当前活动工程（handleChat
// 分叉前已由 contextWithCurrentProjectWorkspace 注入 context 的同源口径）。
func (s *Server) noteProjectIdentity(ctx context.Context, payloadPath, payloadUUID string) (string, string) {
	projectPath := strings.TrimSpace(payloadPath)
	projectUUID := strings.TrimSpace(payloadUUID)
	if projectPath != "" && projectUUID != "" {
		return projectPath, projectUUID
	}
	if s != nil && s.harness != nil {
		activePath, activeUUID := s.harness.CurrentProjectIdentity(ctx)
		if projectPath == "" {
			projectPath = activePath
		}
		if projectUUID == "" {
			projectUUID = activeUUID
		}
	}
	return projectPath, projectUUID
}

// handleNoteChatTurn：note 会话模式回合（handleChat 早分叉入口）。直答 LLM（观察语义），
// 会话记忆=note 会话库（不回退工程单图——与主流双向隔离），辖区上下文注入快照。
func (s *Server) handleNoteChatTurn(w http.ResponseWriter, r *http.Request, req ChatRequest, agentMode string) {
	payload := req.Note
	conversationID := noteConversationID(payload)
	projectPath := projectPathFromChatContext(req.Context)
	projectUUID := firstStringFromMap(req.Context, "project_uuid", "project_id")

	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			AgentMode:      agentMode,
			Reply:          "便签问答不可用：AI 配置缺失或无效。",
			Error:          "llm_config_incomplete",
		})
		return
	}

	assembly := s.buildNoteAssembly(r.Context(), req, conversationID)
	resp, err := s.llm.CompleteRequest(r.Context(), cfg, llm.Request{
		Messages: assembly.Messages,
		Metadata: llm.RequestMetadata{
			Source:            "vitnote_chat",
			ConversationID:    conversationID,
			PromptFingerprint: assembly.Fingerprint,
			PromptStats:       assembly.Stats.Map(),
		},
	})
	if err != nil {
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			AgentMode:      agentMode,
			Reply:          "便签问答失败：" + err.Error(),
			Error:          err.Error(),
		})
		return
	}
	reply := noteReplyText(resp.Text)
	if reply == "" {
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			AgentMode:      agentMode,
			Reply:          "（无回复内容）",
			Error:          "empty_note_reply",
		})
		return
	}
	if err := s.appendNoteSessionExchange(projectPath, projectUUID, payload, req.Message, reply); err != nil && s.logger != nil {
		s.logger.Warn("[vitnote] session persist failed conversation=%s error=%v", conversationID, err)
	}
	writeJSON(w, http.StatusOK, ChatResponse{
		ConversationID: conversationID,
		AgentMode:      agentMode,
		Reply:          reply,
	})
}

// noteReplyText：note 观察答复是纯文本；若模型仍吐 {"reply":...} 信封则剥壳（防御，不依赖）。
func noteReplyText(raw string) string {
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "{") {
		var envelope struct {
			Reply string `json:"reply"`
		}
		if err := json.Unmarshal([]byte(text), &envelope); err == nil && strings.TrimSpace(envelope.Reply) != "" {
			return strings.TrimSpace(envelope.Reply)
		}
	}
	return text
}

// buildNoteAssembly：note 专属组包——系统段（观察者角色+只读约束）+上下文快照段（辖区
// digest+工程只读摘要）+会话历史（note 会话库，非工程单图）+当前问句。段落命名带
// vitnote_ 前缀，遥测 section_stats 可与主任务组包对照（判据 3 证据面）。
func (s *Server) buildNoteAssembly(ctx context.Context, req ChatRequest, conversationID string) promptruntime.Assembly {
	payload := req.Note
	projectPath := projectPathFromChatContext(req.Context)
	projectUUID := firstStringFromMap(req.Context, "project_uuid", "project_id")
	history := noteSessionHistoryLLMMessages(loadNoteSessions(projectPath, projectUUID)[conversationID])

	stateSummary := map[string]any{}
	if s != nil && s.harness != nil {
		stateSummary = s.harness.UserStateSummary(ctx)
	}
	snapshot := map[string]any{
		"note_jurisdiction": map[string]any{
			"note_id": strings.TrimSpace(payload.NoteID),
			"title":   noteDefaultTitle(payload),
			"faces":   payload.Faces,
		},
		"read_only_project_state": stateSummary,
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		snapshotJSON = []byte(`{"note_jurisdiction":{}}`)
	}
	system := `You are the Vit-DAW note observer. You answer questions asked from a small sticky note that the user placed on a circled region (辖区) of the DAW interface.
The note jurisdiction below describes what was circled: which UI faces were hit (timeline / rack / library / control surfaces), each face's share of the selection, and resolved domain entries (tracks, clips, time windows, plugins, library rows).
Answer using the note jurisdiction digest and the read-only project state summary.
This is a read-only observation conversation: do not propose project mutations, do not emit commands or JSON envelopes, do not start workflows. If the user asks for a change, describe what you observe and tell them to ask in the main chat console for actual edits.
When the question refers to "这个范围" / "这一块" / "this range" / "this region", it means the note jurisdiction, not the DAW track selection.
Reply with plain text only. Answer in the user's language. Be concise and concrete.`

	assembly := promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "vitnote_system", "", system, true),
			promptruntime.TextSection(promptruntime.SectionRuntime, "vitnote_context_snapshot", "Note jurisdiction + read-only project state JSON", string(snapshotJSON), false),
		},
		History:     history,
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionCurrentUser, "vitnote_current_user", "", req.Message, false),
		},
	})
	return assembly
}

// noteSessionHistoryLLMMessages：note 会话库消息→LLM 形（最近 noteSessionHistoryLimit 条）。
func noteSessionHistoryLLMMessages(session *NoteSession) []llm.Message {
	if session == nil {
		return nil
	}
	messages := session.Messages
	if len(messages) > noteSessionHistoryLimit {
		messages = messages[len(messages)-noteSessionHistoryLimit:]
	}
	out := make([]llm.Message, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case "user", "assistant":
			out = append(out, llm.Message{Role: message.Role, Content: message.Content})
		}
	}
	return out
}
