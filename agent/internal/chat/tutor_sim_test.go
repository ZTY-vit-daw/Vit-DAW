package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
)

const tutorSimTestYAML = `schema_version: vit.tutor_sim.v0
id: tutor-sim-3step
persona: |
  你是实验导师，语气简短、鼓励式。
  你不替用户回答，也不透露答案。
steps:
  - id: step-1
    ask: "第 1 步：请计算 3 + 4，并只输入数字。"
    expect: "7"
    retry: "这个答案还不对。请重新计算 3 + 4，只输入一个数字。"
    pass: "第 1 步通过。"
  - id: step-2
    ask: "第 2 步：请计算 6 × 7，并只输入数字。"
    expect: "42"
    retry: "这个答案还不对。请重新计算 6 × 7，只输入一个数字。"
    pass: "第 2 步通过。"
  - id: step-3
    ask: "第 3 步：请计算 100 + 23，并只输入数字。"
    expect: "123"
    retry: "这个答案还不对。请重新计算 100 + 23，只输入一个数字。"
    pass: "第 3 步通过，实验完成。"
`

type fakeTutorCompleter struct {
	mu       sync.Mutex
	requests []llm.Request
	replies  []string
	errs     []error
}

func (f *fakeTutorCompleter) CompleteRequest(_ context.Context, _ config.EngineConfig, req llm.Request) (llm.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	index := len(f.requests)
	f.requests = append(f.requests, req)
	if index < len(f.errs) && f.errs[index] != nil {
		return llm.Response{}, f.errs[index]
	}
	if index < len(f.replies) {
		return llm.Response{Text: f.replies[index]}, nil
	}
	return llm.Response{Text: fmt.Sprintf("fake tutor reply %d", index+1)}, nil
}

func (f *fakeTutorCompleter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeTutorCompleter) request(t *testing.T, index int) llm.Request {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.requests) {
		t.Fatalf("request index %d out of range (calls=%d)", index, len(f.requests))
	}
	return f.requests[index]
}

func newTutorTestServer(t *testing.T, fake *fakeTutorCompleter) *Server {
	t.Helper()
	if fake == nil {
		fake = &fakeTutorCompleter{}
	}
	server := &Server{llm: &llm.Client{}}
	server.tutorSim = newTutorSimRuntime()
	server.tutorSim.completer = fake
	server.tutorSim.loadConfig = func() (config.EngineConfig, string, error) {
		return config.EngineConfig{
			BaseURL:      "http://fake.local/v1",
			APIKey:       "test-key",
			DefaultModel: "deepseek-flash",
		}, "test-config.json", nil
	}
	return server
}

func writeTutorProject(t *testing.T, yaml string) (projectPath, sidecarPath string) {
	t.Helper()
	dir := t.TempDir()
	projectPath = filepath.Join(dir, "tutor-blank.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	sidecarPath = filepath.Join(dir, ".vit_agent", "tutor_sim.yaml")
	if yaml != "" {
		if err := os.MkdirAll(filepath.Dir(sidecarPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecarPath, []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return projectPath, sidecarPath
}

func doTutorJSON(t *testing.T, server *Server, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = strings.NewReader(string(encoded))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	var decoded map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("%s %s returned non-JSON body %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code, decoded
}

func tutorString(t *testing.T, body map[string]any, key string) string {
	t.Helper()
	value, _ := body[key].(string)
	return value
}

func tutorInt(t *testing.T, body map[string]any, key string) int {
	t.Helper()
	value, ok := body[key].(float64)
	if !ok {
		t.Fatalf("field %q = %#v, want number (body=%v)", key, body[key], body)
	}
	return int(value)
}

func tutorBool(t *testing.T, body map[string]any, key string) bool {
	t.Helper()
	value, ok := body[key].(bool)
	if !ok {
		t.Fatalf("field %q = %#v, want bool (body=%v)", key, body[key], body)
	}
	return value
}

func tutorMap(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := body[key].(map[string]any)
	if !ok {
		t.Fatalf("field %q = %#v, want object (body=%v)", key, body[key], body)
	}
	return value
}

func startTutorConversation(t *testing.T, server *Server, projectPath, conversationID string) map[string]any {
	t.Helper()
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/start", map[string]any{
		"conversation_id": conversationID,
		"project_path":    projectPath,
	})
	if status != http.StatusOK {
		t.Fatalf("start status = %d body=%v", status, body)
	}
	if got := tutorString(t, body, "status"); got != "ok" {
		t.Fatalf("start status field = %q body=%v", got, body)
	}
	return body
}

func getTutorStatus(t *testing.T, server *Server, conversationID string) map[string]any {
	t.Helper()
	status, body := doTutorJSON(t, server, http.MethodGet, "/agent/tutor/status?conversation_id="+conversationID, nil)
	if status != http.StatusOK {
		t.Fatalf("status endpoint = %d body=%v", status, body)
	}
	return body
}

func postTutorChat(t *testing.T, server *Server, conversationID, message string) map[string]any {
	t.Helper()
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/chat", map[string]any{
		"conversation_id": conversationID,
		"message":         message,
	})
	if status != http.StatusOK {
		t.Fatalf("chat status = %d body=%v", status, body)
	}
	return body
}

func TestTutorStartUnavailableWithoutSidecar(t *testing.T) {
	projectPath, sidecarPath := writeTutorProject(t, "")
	fake := &fakeTutorCompleter{}
	server := newTutorTestServer(t, fake)
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/start", map[string]any{"project_path": projectPath})
	if status != http.StatusOK {
		t.Fatalf("status = %d body=%v", status, body)
	}
	if tutorString(t, body, "status") != "unavailable" || tutorString(t, body, "reason") != "sidecar_missing" {
		t.Fatalf("body = %v", body)
	}
	if tutorString(t, body, "sidecar_path") != sidecarPath {
		t.Fatalf("sidecar_path = %q, want %q", tutorString(t, body, "sidecar_path"), sidecarPath)
	}
	if fake.callCount() != 0 {
		t.Fatalf("LLM called %d times without a sidecar", fake.callCount())
	}
	if _, ok := server.tutorSim.store.Status("tutor_anything"); ok {
		t.Fatal("unavailable start created a session")
	}
}

func TestTutorStartWithoutAnyProject(t *testing.T) {
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/start", map[string]any{})
	if status != http.StatusOK || tutorString(t, body, "status") != "unavailable" || tutorString(t, body, "reason") != "no_project" {
		t.Fatalf("status=%d body=%v", status, body)
	}
}

func TestTutorStartRejectsInvalidSidecar(t *testing.T) {
	projectPath, _ := writeTutorProject(t, "schema_version: vit.tutor_sim.v0\nid: broken\n")
	fake := &fakeTutorCompleter{}
	server := newTutorTestServer(t, fake)
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/start", map[string]any{"project_path": projectPath})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%v", status, body)
	}
	if tutorString(t, body, "status") != "error" || !strings.Contains(tutorString(t, body, "error"), "persona") {
		t.Fatalf("body = %v", body)
	}
	if fake.callCount() != 0 {
		t.Fatal("invalid sidecar reached the LLM")
	}
}

func TestTutorStartCreatesSessionAndOpensWithLLM(t *testing.T) {
	projectPath, sidecarPath := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"  fake opening  "}}
	server := newTutorTestServer(t, fake)
	body := startTutorConversation(t, server, projectPath, "conv-tutor-1")
	if got := tutorString(t, body, "conversation_id"); got != "conv-tutor-1" {
		t.Fatalf("conversation_id = %q", got)
	}
	if got := tutorString(t, body, "experiment_id"); got != "tutor-sim-3step" {
		t.Fatalf("experiment_id = %q", got)
	}
	if got := tutorString(t, body, "schema_version"); got != "vit.tutor_sim.v0" {
		t.Fatalf("schema_version = %q", got)
	}
	if got := tutorString(t, body, "reply"); got != "fake opening" {
		t.Fatalf("reply = %q", got)
	}
	if got := tutorString(t, body, "sidecar_path"); got != sidecarPath {
		t.Fatalf("sidecar_path = %q, want %q", got, sidecarPath)
	}
	if tutorInt(t, body, "step_index") != 1 || tutorInt(t, body, "step_total") != 3 || tutorString(t, body, "step_id") != "step-1" {
		t.Fatalf("step fields = %v", body)
	}

	opener := fake.request(t, 0)
	if len(opener.Messages) != 2 || opener.Messages[0].Role != "system" || opener.Messages[1].Role != "user" {
		t.Fatalf("opener messages = %+v", opener.Messages)
	}
	prompt := opener.Messages[0].Content + "\n" + opener.Messages[1].Content
	if !strings.Contains(prompt, "实验导师") || !strings.Contains(prompt, "3 + 4") {
		t.Fatalf("opener prompt missing persona/ask: %q", prompt)
	}
	for _, leaked := range []string{"\"7\"", "\"42\"", "\"123\""} {
		if strings.Contains(prompt, leaked) {
			t.Fatalf("opener prompt leaked expect value %s: %q", leaked, prompt)
		}
	}
	if opener.Metadata.Source != "tutor_sim" || opener.Metadata.ConversationID != "conv-tutor-1" {
		t.Fatalf("opener metadata = %+v", opener.Metadata)
	}

	statusBody := getTutorStatus(t, server, "conv-tutor-1")
	if tutorInt(t, statusBody, "step_index") != 1 || tutorInt(t, statusBody, "step_total") != 3 ||
		tutorString(t, statusBody, "session_status") != "active" || tutorBool(t, statusBody, "completed") {
		t.Fatalf("status body = %v", statusBody)
	}
}

func TestTutorChatWrongAnswerDoesNotAdvance(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening", "retry reply"}}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-wrong")

	body := postTutorChat(t, server, "conv-wrong", " 6 ")
	if got := tutorString(t, body, "workflow"); got != tutorSimWorkflow {
		t.Fatalf("workflow = %q", got)
	}
	data := tutorMap(t, body, "workflow_data")
	if tutorBool(t, data, "passed") || tutorBool(t, data, "completed") {
		t.Fatalf("workflow_data = %v", data)
	}
	if tutorInt(t, data, "step_index") != 1 || tutorInt(t, data, "step_total") != 3 || tutorString(t, data, "step_id") != "step-1" {
		t.Fatalf("workflow_data step = %v", data)
	}
	if got := tutorString(t, body, "stop_reason"); got != tutorSimStopRetry {
		t.Fatalf("stop_reason = %q", got)
	}
	if got := tutorString(t, body, "reply"); got != "retry reply" {
		t.Fatalf("reply = %q", got)
	}
	statusBody := getTutorStatus(t, server, "conv-wrong")
	if tutorInt(t, statusBody, "step_index") != 1 || tutorBool(t, statusBody, "completed") ||
		tutorInt(t, statusBody, "attempts") != 1 || tutorInt(t, statusBody, "failures") != 1 {
		t.Fatalf("status after wrong answer = %v", statusBody)
	}
	retryPrompt := fake.request(t, 1).Messages[1].Content
	if !strings.Contains(retryPrompt, "用户输入：6") || !strings.Contains(retryPrompt, "未通过") || !strings.Contains(retryPrompt, "3 + 4") {
		t.Fatalf("retry prompt = %q", retryPrompt)
	}
	if strings.Contains(retryPrompt, "42") || strings.Contains(retryPrompt, "123") {
		t.Fatalf("retry prompt leaked a later expect: %q", retryPrompt)
	}
}

func TestTutorChatAdvancesThreeStepsAndCompletes(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening", "passed 1", "passed 2", "completed"}}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-flow")

	step2 := postTutorChat(t, server, "conv-flow", " 7 ")
	data := tutorMap(t, step2, "workflow_data")
	if !tutorBool(t, data, "passed") || tutorBool(t, data, "completed") ||
		tutorInt(t, data, "step_index") != 2 || tutorString(t, data, "step_id") != "step-2" {
		t.Fatalf("step 2 data = %v", data)
	}
	if tutorString(t, step2, "stop_reason") != tutorSimStopAdvanced {
		t.Fatalf("step 2 stop = %q", tutorString(t, step2, "stop_reason"))
	}
	if prompt := fake.request(t, 1).Messages[1].Content; !strings.Contains(prompt, "6 × 7") || strings.Contains(prompt, "42") {
		t.Fatalf("step 2 prompt = %q", prompt)
	}

	step3 := postTutorChat(t, server, "conv-flow", "42")
	data = tutorMap(t, step3, "workflow_data")
	if !tutorBool(t, data, "passed") || tutorBool(t, data, "completed") ||
		tutorInt(t, data, "step_index") != 3 || tutorString(t, data, "step_id") != "step-3" {
		t.Fatalf("step 3 data = %v", data)
	}
	if prompt := fake.request(t, 2).Messages[1].Content; !strings.Contains(prompt, "100 + 23") || strings.Contains(prompt, "123") {
		t.Fatalf("step 3 prompt = %q", prompt)
	}

	completed := postTutorChat(t, server, "conv-flow", "123")
	data = tutorMap(t, completed, "workflow_data")
	if !tutorBool(t, data, "passed") || !tutorBool(t, data, "completed") ||
		tutorInt(t, data, "step_index") != 3 || tutorString(t, data, "step_id") != "step-3" {
		t.Fatalf("completion data = %v", data)
	}
	if tutorString(t, completed, "stop_reason") != tutorSimStopCompleted ||
		tutorString(t, completed, "goal_status") != "completed" {
		t.Fatalf("completion response = %v", completed)
	}
	if fake.callCount() != 4 {
		t.Fatalf("LLM calls = %d, want 4", fake.callCount())
	}

	statusBody := getTutorStatus(t, server, "conv-flow")
	if !tutorBool(t, statusBody, "completed") || tutorString(t, statusBody, "session_status") != "completed" ||
		tutorInt(t, statusBody, "step_index") != 3 {
		t.Fatalf("completed status = %v", statusBody)
	}

	after := postTutorChat(t, server, "conv-flow", "7")
	if tutorString(t, after, "stop_reason") != tutorSimStopAlreadyCompleted ||
		!tutorBool(t, tutorMap(t, after, "workflow_data"), "completed") {
		t.Fatalf("post-completion response = %v", after)
	}
	if fake.callCount() != 4 {
		t.Fatalf("post-completion input called the LLM (%d calls)", fake.callCount())
	}
}

func TestTutorChatLLMFailureKeepsServerVerdict(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{
		replies: []string{"opening"},
		errs:    []error{nil, errors.New("context deadline exceeded")},
	}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-fail")
	body := postTutorChat(t, server, "conv-fail", "7")
	if tutorString(t, body, "error") == "" {
		t.Fatalf("failure response has no error: %v", body)
	}
	data := tutorMap(t, body, "workflow_data")
	if !tutorBool(t, data, "passed") || tutorBool(t, data, "completed") ||
		tutorInt(t, data, "step_index") != 2 || tutorString(t, data, "failure_class") != tutorSimFailureTransient {
		t.Fatalf("failure workflow_data = %v", data)
	}
	if tutorString(t, body, "stop_reason") != tutorSimStopLLMTransient {
		t.Fatalf("stop_reason = %q", tutorString(t, body, "stop_reason"))
	}
	if reply := tutorString(t, body, "reply"); !strings.Contains(reply, "导师回复生成失败") {
		t.Fatalf("reply = %q", reply)
	}
	statusBody := getTutorStatus(t, server, "conv-fail")
	if tutorInt(t, statusBody, "step_index") != 2 || tutorBool(t, statusBody, "completed") {
		t.Fatalf("server verdict was not preserved: %v", statusBody)
	}
}

func TestTutorChatEmptyLLMReplyIsFailureNotFabrication(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening", "   "}}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-empty")
	body := postTutorChat(t, server, "conv-empty", "7")
	data := tutorMap(t, body, "workflow_data")
	if tutorString(t, data, "failure_class") != tutorSimFailureEmpty {
		t.Fatalf("failure_class = %q", tutorString(t, data, "failure_class"))
	}
	if tutorString(t, body, "stop_reason") != tutorSimStopLLMEmptyReply {
		t.Fatalf("stop_reason = %q", tutorString(t, body, "stop_reason"))
	}
	if reply := tutorString(t, body, "reply"); strings.TrimSpace(reply) == "" || !strings.Contains(reply, "导师回复生成失败") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestTutorStartLLMFailureCreatesNoSession(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{errs: []error{errors.New("boom")}}
	server := newTutorTestServer(t, fake)
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/start", map[string]any{
		"conversation_id": "conv-start-fail",
		"project_path":    projectPath,
	})
	if status != http.StatusBadGateway || tutorString(t, body, "status") != "error" ||
		tutorString(t, body, "failure_class") != tutorSimFailurePermanent {
		t.Fatalf("status=%d body=%v", status, body)
	}
	notFound, _ := doTutorJSON(t, server, http.MethodGet, "/agent/tutor/status?conversation_id=conv-start-fail", nil)
	if notFound != http.StatusNotFound {
		t.Fatalf("failed start created a session: status endpoint = %d", notFound)
	}
}

func TestTutorChatMissDoesNotHandle(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening"}}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-hit")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/agent/chat", strings.NewReader(`{"conversation_id":"conv-miss","message":"hello"}`))
	if server.handleTutorSimChatTurn(rec, req, "conv-miss", "hello") {
		t.Fatal("tutor loop handled a conversation it does not own")
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("miss wrote a response: %q", rec.Body.String())
	}
	if fake.callCount() != 1 {
		t.Fatalf("miss called the LLM (%d calls)", fake.callCount())
	}
}

func TestTutorChatDoesNotEnterOrdinaryLoop(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening", "advanced"}}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-loop")

	body := postTutorChat(t, server, "conv-loop", "7")
	if tutorString(t, body, "workflow") != tutorSimWorkflow {
		t.Fatalf("workflow = %q", tutorString(t, body, "workflow"))
	}
	if got := tutorString(t, body, "goal_id"); got != "" {
		t.Fatalf("tutor turn created a goal id %q", got)
	}
	server.mu.Lock()
	_, hasGoal := server.conversationGoals["conv-loop"]
	ordinaryMessages := len(server.conversations["conv-loop"])
	server.mu.Unlock()
	if hasGoal || ordinaryMessages != 0 {
		t.Fatalf("tutor turn entered ordinary loop: goal=%t messages=%d", hasGoal, ordinaryMessages)
	}
	if len(server.events["conv-loop"]) != 0 {
		t.Fatalf("tutor turn emitted ordinary turn events: %d", len(server.events["conv-loop"]))
	}
}

func TestTutorStatusValidation(t *testing.T) {
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	status, _ := doTutorJSON(t, server, http.MethodGet, "/agent/tutor/status", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("missing conversation_id status = %d", status)
	}
	status, body := doTutorJSON(t, server, http.MethodGet, "/agent/tutor/status?conversation_id=missing", nil)
	if status != http.StatusNotFound || tutorString(t, body, "error") != "unknown_tutor_session" {
		t.Fatalf("unknown session status=%d body=%v", status, body)
	}
	status, _ = doTutorJSON(t, server, http.MethodPost, "/agent/tutor/status?conversation_id=x", map[string]any{})
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", status)
	}
	status, _ = doTutorJSON(t, server, http.MethodGet, "/agent/tutor/start", nil)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("GET start = %d", status)
	}
}

func TestTutorStartConfigUnavailable(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{}
	server := newTutorTestServer(t, fake)
	server.tutorSim.loadConfig = func() (config.EngineConfig, string, error) {
		return config.EngineConfig{}, "test-config.json", errors.New("no config")
	}
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/start", map[string]any{"project_path": projectPath})
	if status != http.StatusServiceUnavailable || tutorString(t, body, "failure_class") != tutorSimFailureConfig {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if fake.callCount() != 0 {
		t.Fatal("incomplete config reached the LLM")
	}
}

// TestTutorPromptsNeverCarryExpect is the prompt-hygiene guard: the expect
// field is server-only. A prompt may contain the user's own input and the
// current ask/retry/pass prose, but never the expect label or a later step's
// expect value.
func TestTutorPromptsNeverCarryExpect(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening", "retry", "advanced", "advanced2", "done"}}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-prompt")
	promptText := func(index int) string {
		text := ""
		for _, message := range fake.request(t, index).Messages {
			text += message.Content + " "
		}
		return text
	}
	// Opener: no expect value may appear at all.
	opener := promptText(0)
	for _, value := range []string{"7", "42", "123"} {
		if strings.Contains(opener, value) {
			t.Fatalf("opener prompt leaked expect %q: %q", value, opener)
		}
	}
	// Wrong answer: current step only.
	postTutorChat(t, server, "conv-prompt", "wrong")
	for _, value := range []string{"42", "123"} {
		if strings.Contains(promptText(1), value) {
			t.Fatalf("wrong-answer prompt leaked later expect %q: %q", value, promptText(1))
		}
	}
	// After step 1: user input 7 is legitimate; later expects are not.
	postTutorChat(t, server, "conv-prompt", "7")
	for _, value := range []string{"42", "123"} {
		if strings.Contains(promptText(2), value) {
			t.Fatalf("step-2 prompt leaked later expect %q: %q", value, promptText(2))
		}
	}
	// After step 2: user input 42 is legitimate; 123 is not.
	postTutorChat(t, server, "conv-prompt", "42")
	if strings.Contains(promptText(3), "123") {
		t.Fatalf("step-3 prompt leaked expect 123: %q", promptText(3))
	}
	postTutorChat(t, server, "conv-prompt", "123")
	for i := 0; i < fake.callCount(); i++ {
		if strings.Contains(promptText(i), "expect") {
			t.Fatalf("prompt %d mentions expect: %q", i, promptText(i))
		}
	}
}
