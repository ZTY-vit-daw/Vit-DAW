package chat

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/llm"
)

// VITNOTE-NOTESTREAM-2 测试面：
//   - 会话库 fail-open（缺文件/损坏 JSON/异形行，AGENTS §11）+往返+归档；
//   - note 回合隔离（主对话记忆零写入=判据 1/3 机制面；辖区上下文注入=三号场错路由解法）；
//   - 回信封剥离、/agent/note/archive、投影行排序与截断；
//   - note 组装只吃 note 会话库历史，不回退主会话记忆。

func noteTestProjectDir(t *testing.T) (projectPath, projectUUID string) {
	t.Helper()
	root := t.TempDir()
	projectPath = filepath.Join(root, "draft_test.vit")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	return projectPath, "uuid-note-test"
}

func TestNoteConversationIDPrefersSessionKeyOverNoteID(t *testing.T) {
	withSession := noteConversationID(&NoteChatPayload{NoteID: "note_3", Session: "r1a2b3c4-3"})
	if withSession != "note_r1a2b3c4-3" {
		t.Fatalf("session key not preferred: %s", withSession)
	}
	fallback := noteConversationID(&NoteChatPayload{NoteID: "note_3"})
	if fallback != "note_note_3" {
		t.Fatalf("note_id fallback mismatch: %s", fallback)
	}
}

func TestNoteSessionStoreFailOpenAndRoundTrip(t *testing.T) {
	projectPath, projectUUID := noteTestProjectDir(t)
	if sessions := loadNoteSessions(projectPath, projectUUID); len(sessions) != 0 {
		t.Fatalf("missing file should load empty, got %d", len(sessions))
	}
	storePath, err := noteSessionStorePath(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storePath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if sessions := loadNoteSessions(projectPath, projectUUID); len(sessions) != 0 {
		t.Fatalf("corrupt file should load empty, got %d", len(sessions))
	}
	// 异形行：null 行与 id 不一致行丢弃（fail-open 逐行）。
	if err := os.WriteFile(storePath, []byte(`{"schema_version":"vit_note_sessions.v1","sessions":{"note_bad":null,"note_x":{"conversation_id":"note_y"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if sessions := loadNoteSessions(projectPath, projectUUID); len(sessions) != 0 {
		t.Fatalf("malformed rows should drop, got %d", len(sessions))
	}

	s := &Server{}
	payload := &NoteChatPayload{NoteID: "note_3", Session: "r-3", Title: "便签·轨道时间线 72%", Faces: []map[string]any{{"label": "轨道时间线", "selection_share": 0.72}}}
	if err := s.appendNoteSessionExchange(projectPath, projectUUID, payload, "问一圈", "答一句"); err != nil {
		t.Fatal(err)
	}
	sessions := loadNoteSessions(projectPath, projectUUID)
	session := sessions["note_r-3"]
	if session == nil {
		t.Fatalf("session missing after append: %#v", sessions)
	}
	if session.Title != "便签·轨道时间线 72%" || len(session.Messages) != 2 || session.Archived {
		t.Fatalf("session mismatch: %#v", session)
	}
	if len(session.Faces) != 1 || session.Faces[0]["label"] != "轨道时间线" {
		t.Fatalf("faces not persisted: %#v", session.Faces)
	}
	// 第二轮延续同会话（判据 2：重开/续问不清史）。
	if err := s.appendNoteSessionExchange(projectPath, projectUUID, payload, "再问", "再答"); err != nil {
		t.Fatal(err)
	}
	sessions = loadNoteSessions(projectPath, projectUUID)
	if len(sessions["note_r-3"].Messages) != 4 {
		t.Fatalf("second exchange missing: %#v", sessions["note_r-3"].Messages)
	}
	// 归档幂等。
	if err := s.archiveNoteSession(projectPath, projectUUID, "note_r-3"); err != nil {
		t.Fatal(err)
	}
	if err := s.archiveNoteSession(projectPath, projectUUID, "note_r-3"); err != nil {
		t.Fatal(err)
	}
	if err := s.archiveNoteSession(projectPath, projectUUID, "note_unknown"); err != nil {
		t.Fatal(err)
	}
	if !loadNoteSessions(projectPath, projectUUID)["note_r-3"].Archived {
		t.Fatal("archive flag not persisted")
	}
}

func TestNoteDefaultTitleFallback(t *testing.T) {
	if got := noteDefaultTitle(&NoteChatPayload{NoteID: "note_7"}); got != "便签 note_7" {
		t.Fatalf("fallback title mismatch: %s", got)
	}
	if got := noteDefaultTitle(&NoteChatPayload{NoteID: "note_7", Title: " 便签·机架 40% "}); got != "便签·机架 40%" {
		t.Fatalf("payload title not trimmed/used: %s", got)
	}
}

func TestNoteReplyTextStripsEnvelope(t *testing.T) {
	if got := noteReplyText(`{"reply":"纯文本答复"}`); got != "纯文本答复" {
		t.Fatalf("envelope not stripped: %s", got)
	}
	if got := noteReplyText("直接纯文本"); got != "直接纯文本" {
		t.Fatalf("plain text mutated: %s", got)
	}
	if got := noteReplyText(`{"other":1}`); got != `{"other":1}` {
		t.Fatalf("non-reply json mutated: %s", got)
	}
}

// note 回合全链（handleChat 早分叉→直答 LLM→note 会话库落盘）+隔离断言。
func TestHandleChatNoteTurnRoutesToIndependentSession(t *testing.T) {
	projectPath, projectUUID := noteTestProjectDir(t)
	var llmRequestBody map[string]any
	llmStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &llmRequestBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"这是辖区内的答复"}}]}`))
	}))
	defer llmStub.Close()
	t.Setenv("VIT_AGENT_LLM_BASE_URL", llmStub.URL+"/v1")
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test-key")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test-model")

	server := &Server{
		llm:           &llm.Client{HTTPClient: llmStub.Client()},
		conversations: map[string][]llm.Message{},
	}
	chatStub := httptest.NewServer(http.HandlerFunc(server.handleChat))
	defer chatStub.Close()

	notePayload := map[string]any{
		"note_id": "note_3",
		"session": "r-abc-3",
		"title":   "便签·轨道时间线 72%",
		"faces":   []any{map[string]any{"face_kind": "timeline", "label": "轨道时间线", "selection_share": 0.72, "domain": map[string]any{"entries": []any{map[string]any{"track_id": 1007, "track_name": "Bass"}}}}},
	}
	first := postNoteChat(t, chatStub.URL, "你能告诉我这个范围是什么内容吗？", notePayload, projectPath, projectUUID)
	if first.ConversationID != "note_r-abc-3" {
		t.Fatalf("conversation id = %s", first.ConversationID)
	}
	if first.Reply != "这是辖区内的答复" {
		t.Fatalf("reply = %s", first.Reply)
	}
	if first.GoalID != "" || first.GoalStatus != "" {
		t.Fatalf("note turn must not create a goal: %+v", first)
	}

	// 隔离断言（判据 1/3 机制面）：主对话记忆零写入；note 会话库一轮问答+默认名在档。
	server.mu.Lock()
	mainMemoryLen := len(server.conversations)
	server.mu.Unlock()
	if mainMemoryLen != 0 {
		t.Fatalf("note turn leaked into main conversation memory: %d conversations", mainMemoryLen)
	}
	session := loadNoteSessions(projectPath, projectUUID)["note_r-abc-3"]
	if session == nil || len(session.Messages) != 2 || session.Title != "便签·轨道时间线 72%" {
		t.Fatalf("note session store mismatch: %#v", session)
	}

	// 辖区上下文注入断言：LLM 请求包含 faces digest 与 note 观察者系统段。
	raw, _ := json.Marshal(llmRequestBody)
	blob := string(raw)
	for _, needle := range []string{"note observer", "轨道时间线", "Bass", "note_jurisdiction"} {
		if !strings.Contains(blob, needle) {
			t.Fatalf("llm request missing %q", needle)
		}
	}

	// 第二轮：同 note 延续同会话（历史注入 LLM）。
	second := postNoteChat(t, chatStub.URL, "再补充一句？", notePayload, projectPath, projectUUID)
	if second.ConversationID != "note_r-abc-3" || second.Reply != "这是辖区内的答复" {
		t.Fatalf("second turn mismatch: %+v", second)
	}
	session = loadNoteSessions(projectPath, projectUUID)["note_r-abc-3"]
	if len(session.Messages) != 4 {
		t.Fatalf("second exchange not persisted: %d messages", len(session.Messages))
	}
	assembly := server.buildNoteAssembly(context.Background(), ChatRequest{
		Message: "第三问",
		Note:    &NoteChatPayload{NoteID: "note_3", Session: "r-abc-3"},
		Context: map[string]any{"project_path": projectPath, "project_uuid": projectUUID},
	}, "note_r-abc-3")
	joined := strings.Join(messageContents(assembly.Messages), "\n")
	if !strings.Contains(joined, "你能告诉我这个范围是什么内容吗？") || !strings.Contains(joined, "再补充一句？") {
		t.Fatalf("note assembly missing prior note history: %s", joined)
	}
}

// note 回合缺 note_id/session → 400（防御载荷）。
func TestHandleChatNotePayloadRequiresIdentity(t *testing.T) {
	server := &Server{conversations: map[string][]llm.Message{}}
	chatStub := httptest.NewServer(http.HandlerFunc(server.handleChat))
	defer chatStub.Close()
	body := `{"message":"问句","note":{"title":"无名"}}`
	resp, err := http.Post(chatStub.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// VITNOTE-REGION-TIME-1：m:ss.d 格式化（十分位四舍五入；负值/非有限回落 0）。
func TestNoteFormatTimelineSeconds(t *testing.T) {
	cases := []struct {
		seconds  float64
		expected string
	}{
		{0, "0:00.0"},
		{3.2, "0:03.2"},
		{8.5, "0:08.5"},
		{62.45, "1:02.5"},
		{600.04, "10:00.0"},
		{-1, "0:00.0"},
		{math.NaN(), "0:00.0"},
		{math.Inf(1), "0:00.0"},
	}
	for _, c := range cases {
		if got := noteFormatTimelineSeconds(c.seconds); got != c.expected {
			t.Fatalf("noteFormatTimelineSeconds(%v) = %s, want %s", c.seconds, got, c.expected)
		}
	}
}

// 辖区时间维度摘要三例（全含/部分交/零交缺省）+全长段+轨/clip 身份。
func TestNoteJurisdictionDigestTimeIntersectionCases(t *testing.T) {
	faces := []map[string]any{{
		"face_kind": "timeline", "label": "轨道时间线", "selection_share": 0.72,
		"domain": map[string]any{
			"range_time_span": map[string]any{"start_s": 3.2, "end_s": 8.5},
			"entries": []any{
				// 全含：相交段=clip 全长界。
				map[string]any{"clip_id": "c_full", "track_id": "t1", "start_seconds": 3.2, "end_seconds": 7.8, "length_seconds": 4.6, "range_clip_start": 3.2, "range_clip_end": 7.8},
				// 部分交：相交段=裁剪界。
				map[string]any{"clip_id": "c_part", "track_id": "t1", "start_seconds": 5.0, "end_seconds": 12.0, "range_clip_start": 5.0, "range_clip_end": 8.5},
				// 零交：supplier 省键 → 只列身份+全长，无相交段。
				map[string]any{"clip_id": "c_out", "track_id": "t2", "start_seconds": 20.0, "end_seconds": 30.0},
			},
		},
	}}
	digest := noteJurisdictionDigest(faces)
	for _, needle := range []string{
		"时间线 0:03.2–0:08.5：命中 3 个 clip：",
		"clip c_full（轨 t1）全长 0:03.2–0:07.8（与范围相交段 0:03.2–0:07.8）",
		"clip c_part（轨 t1）全长 0:05.0–0:12.0（与范围相交段 0:05.0–0:08.5）",
		"clip c_out（轨 t2）全长 0:20.0–0:30.0",
	} {
		if !strings.Contains(digest, needle) {
			t.Fatalf("digest missing %q:\n%s", needle, digest)
		}
	}
	if strings.Contains(digest, "c_out（轨 t2）全长 0:20.0–0:30.0（") {
		t.Fatalf("zero-overlap clip must omit intersection segment:\n%s", digest)
	}
}

// 旧载荷 fail-open：无时间字段=身份行仍在、无时间段文本；非 timeline 面/空面=无时间行。
func TestNoteJurisdictionDigestFailOpen(t *testing.T) {
	// v3 旧载荷：无 range_time_span、条目无 range_clip_start/end。
	legacy := []map[string]any{{
		"face_kind": "timeline", "label": "轨道时间线",
		"domain": map[string]any{
			"entries": []any{
				map[string]any{"clip_id": "c1", "track_id": "t1", "start_seconds": 1.0, "end_seconds": 4.0},
				map[string]any{"clip_id": 1007, "track_id": 42},
			},
		},
	}}
	digest := noteJurisdictionDigest(legacy)
	for _, needle := range []string{"时间线：命中 2 个 clip：", "clip c1（轨 t1）全长 0:01.0–0:04.0", "clip 1007（轨 42）"} {
		if !strings.Contains(digest, needle) {
			t.Fatalf("legacy digest missing %q:\n%s", needle, digest)
		}
	}
	if strings.Contains(digest, "与范围相交段") || strings.Contains(digest, "时间线 0:") {
		t.Fatalf("legacy digest must not fabricate time spans:\n%s", digest)
	}
	// 异形值：负秒/退化区间/越界相交段一律省略（不伪造）。
	malformed := []map[string]any{{
		"face_kind": "timeline",
		"domain": map[string]any{
			"range_time_span": map[string]any{"start_s": 5.0, "end_s": 2.0},
			"entries": []any{
				map[string]any{"clip_id": "c_bad", "track_id": "t1", "start_seconds": -3.0, "range_clip_start": 1.0, "range_clip_end": 99.0},
			},
		},
	}}
	if got := noteJurisdictionDigest(malformed); got != "时间线：命中 1 个 clip：\nclip c_bad（轨 t1）" {
		t.Fatalf("malformed digest mismatch:\n%s", got)
	}
	// 非 timeline 面=无时间行（如实缺省）；空/nil 面=空摘要。
	if got := noteJurisdictionDigest([]map[string]any{{"face_kind": "rack", "domain": map[string]any{"entries": []any{map[string]any{"plugin_name": "EQ"}}}}}); got != "" {
		t.Fatalf("non-timeline face must not produce time lines: %s", got)
	}
	if noteJurisdictionDigest(nil) != "" {
		t.Fatal("nil faces must produce empty digest")
	}
	if noteJurisdictionDigest([]map[string]any{{"face_kind": "timeline"}}) != "" {
		t.Fatal("timeline face without domain must produce empty digest")
	}
}

// 组装注入：v3.1 载荷快照段带 time_digest+系统段时间指令；旧载荷快照形态不变（键缺席）。
func TestBuildNoteAssemblyTimeDimension(t *testing.T) {
	server := &Server{conversations: map[string][]llm.Message{}}
	projectPath, projectUUID := noteTestProjectDir(t)
	ctx := map[string]any{"project_path": projectPath, "project_uuid": projectUUID}
	richFaces := []map[string]any{{
		"face_kind": "timeline", "label": "轨道时间线",
		"domain": map[string]any{
			"range_time_span": map[string]any{"start_s": 3.2, "end_s": 8.5},
			"entries": []any{
				map[string]any{"clip_id": "c1", "track_id": "t1", "start_seconds": 3.2, "end_seconds": 7.8, "range_clip_start": 3.2, "range_clip_end": 7.8},
			},
		},
	}}
	rich := server.buildNoteAssembly(context.Background(), ChatRequest{
		Message: "这个范围是什么内容？",
		Note:    &NoteChatPayload{NoteID: "note_3", Session: "r-1", Faces: richFaces},
		Context: ctx,
	}, "note_r-1")
	richJoined := strings.Join(messageContents(rich.Messages), "\n")
	for _, needle := range []string{
		"time_digest",
		"时间线 0:03.2–0:08.5：命中 1 个 clip：",
		"与范围相交段 0:03.2–0:07.8",
		"always include the time dimension",
	} {
		if !strings.Contains(richJoined, needle) {
			t.Fatalf("rich assembly missing %q:\n%s", needle, richJoined)
		}
	}
	legacy := server.buildNoteAssembly(context.Background(), ChatRequest{
		Message: "问句",
		Note:    &NoteChatPayload{NoteID: "note_3", Session: "r-1"},
		Context: ctx,
	}, "note_r-1")
	legacyJoined := strings.Join(messageContents(legacy.Messages), "\n")
	// 键形断言（带 JSON 引号）：系统段指令行含裸词 time_digest，不能作缺省判据——
	// 快照 JSON 无 "time_digest" 键=旧载荷形态不变（fail-open）。
	if strings.Contains(legacyJoined, `"time_digest"`) {
		t.Fatalf("legacy assembly must omit time_digest key:\n%s", legacyJoined)
	}
	if !strings.Contains(legacyJoined, "always include the time dimension") {
		t.Fatal("time instruction must stay present for legacy payloads (fail-open wording)")
	}
}

func TestHandleNoteArchiveEndpoint(t *testing.T) {
	projectPath, projectUUID := noteTestProjectDir(t)
	server := &Server{}
	payload := &NoteChatPayload{NoteID: "note_3", Session: "r-xyz-3"}
	if err := server.appendNoteSessionExchange(projectPath, projectUUID, payload, "q", "a"); err != nil {
		t.Fatal(err)
	}
	stub := httptest.NewServer(http.HandlerFunc(server.handleNoteArchive))
	defer stub.Close()
	// 由 note 身份推导会话 id（Godot 删除路径同款载荷）。
	archive := func(body string) int {
		resp, err := http.Post(stub.URL, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if code := archive(`{"note_id":"note_3","session":"r-xyz-3","project_path":` + jsonQuote(projectPath) + `,"project_uuid":` + jsonQuote(projectUUID) + `}`); code != http.StatusOK {
		t.Fatalf("archive status = %d", code)
	}
	if !loadNoteSessions(projectPath, projectUUID)["note_r-xyz-3"].Archived {
		t.Fatal("archive not persisted via endpoint")
	}
	if code := archive(`{}`); code != http.StatusBadRequest {
		t.Fatalf("empty identity status = %d", code)
	}
}

func TestNoteSessionRowsOrderingAndCap(t *testing.T) {
	projectPath, projectUUID := noteTestProjectDir(t)
	server := &Server{}
	older := &NoteChatPayload{NoteID: "note_1", Session: "r-1", Title: "便签一"}
	newer := &NoteChatPayload{NoteID: "note_2", Session: "r-2", Title: "便签二"}
	if err := server.appendNoteSessionExchange(projectPath, projectUUID, older, "q", "a"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := server.appendNoteSessionExchange(projectPath, projectUUID, newer, "q", "a"); err != nil {
		t.Fatal(err)
	}
	rows := server.noteSessionRows(projectPath, projectUUID)
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0]["conversation_id"] != "note_r-2" || rows[0]["title"] != "便签二" {
		t.Fatalf("newest first violated: %#v", rows[0])
	}
	if rows[0]["archived"] != false || rows[0]["project_uuid"] != projectUUID {
		t.Fatalf("row fields mismatch: %#v", rows[0])
	}
	messages, _ := rows[0]["messages"].([]noteSessionMessage)
	if len(messages) != 2 {
		t.Fatalf("row messages = %#v", rows[0]["messages"])
	}
}

// 投影截断：超限消息只保留最近 noteSessionProjectionMessageLimit 条。
func TestNoteSessionRowsMessageCap(t *testing.T) {
	projectPath, projectUUID := noteTestProjectDir(t)
	server := &Server{}
	payload := &NoteChatPayload{NoteID: "note_9", Session: "r-9"}
	for i := 0; i < noteSessionProjectionMessageLimit; i++ {
		if err := server.appendNoteSessionExchange(projectPath, projectUUID, payload, "q", "a"); err != nil {
			t.Fatal(err)
		}
	}
	rows := server.noteSessionRows(projectPath, projectUUID)
	messages, _ := rows[0]["messages"].([]noteSessionMessage)
	if len(messages) != noteSessionProjectionMessageLimit {
		t.Fatalf("cap violated: %d", len(messages))
	}
}

func postNoteChat(t *testing.T, url, message string, note map[string]any, projectPath, projectUUID string) ChatResponse {
	t.Helper()
	body := map[string]any{
		"conversation_id": "",
		"message":         message,
		"note":            note,
		"context": map[string]any{
			"project_path": projectPath,
			"project_uuid": projectUUID,
		},
	}
	raw, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		rawBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("chat status = %d body=%s", resp.StatusCode, string(rawBody))
	}
	var out ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func jsonQuote(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func messageContents(messages []llm.Message) []string {
	out := make([]string, 0, len(messages))
	for _, message := range messages {
		out = append(out, message.Content)
	}
	return out
}
