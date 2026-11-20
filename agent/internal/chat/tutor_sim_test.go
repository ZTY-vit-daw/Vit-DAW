package chat

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-4（池序 438）：v1 schema（brief/materials）+ 材料装载面      */
/* ------------------------------------------------------------------ */

const tutorSimV1TestYAML = `schema_version: vit.tutor_sim.v1
id: tutor-v1-material
persona: |
  实验导师，简短、不透露答案。
steps:
  - id: step-listen
    ask: "听并回答：主题有几个音？只输入数字。"
    expect: "42"
    retry: "重试：再听一遍。"
    pass: "通过。"
    brief: |
      教学简报：先点材料卡装载主题，再从头听一遍。
    materials:
      - kind: midi_clip
        title: 主题材料
        path: assets/theme.mid
  - id: step-confirm
    ask: "确认完成：输入「完成」。"
    expect: "完成"
    retry: "重试确认。"
    pass: "通过确认。"
`

// writeTutorMidiSMF 构造 480 ticks/qn、tempo 100 BPM、两个音（60/67）的
// 最小 format 0 SMF（与 tutorsim 包测试同型的手工字节）。
func writeTutorMidiSMF(t *testing.T) []byte {
	t.Helper()
	out := []byte{'M', 'T', 'h', 'd'}
	out = append(out, 0, 0, 0, 6, 0, 0, 0, 1, 0x01, 0xE0) // format 0, 1 track, division 480
	out = append(out, 'M', 'T', 'r', 'k', 0, 0, 0, 0)
	body := []byte{
		0x00, 0xFF, 0x51, 0x03, 0x09, 0x27, 0xC0, // tempo 600000 → 100 BPM
		0x00, 0x90, 60, 90,  // tick 0 on
		0x7C, 0x80, 60, 64,  // +124 off
		0x04, 0x90, 67, 100, // +4 → tick 128 on
		0x7C, 0x80, 67, 64,  // +124 → end tick 252
		0x00, 0xFF, 0x2F, 0x00,
	}
	out = append(out, body...)
	binary.BigEndian.PutUint32(out[len(out)-len(body)-4:], uint32(len(body)))
	return out
}

func writeTutorV1Project(t *testing.T) (projectPath string) {
	t.Helper()
	projectPath, sidecarPath := writeTutorProject(t, tutorSimV1TestYAML)
	assetDir := filepath.Join(filepath.Dir(sidecarPath), "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "theme.mid"), writeTutorMidiSMF(t), 0o644); err != nil {
		t.Fatal(err)
	}
	return projectPath
}

type fakeTutorKernel struct {
	mu       sync.Mutex
	commands []map[string]any
	replies  []map[string]any // 按命令顺序弹出；空表=按命令名给默认 ok 回执
}

func (f *fakeTutorKernel) send(_ context.Context, cmd map[string]any) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := fmt.Sprint(cmd["cmd"])
	f.commands = append(f.commands, cmd)
	if len(f.replies) > 0 {
		reply := f.replies[0]
		f.replies = f.replies[1:]
		return reply, nil
	}
	switch name {
	case "add_track":
		return map[string]any{"status": "ok", "track_id": "trk-1"}, nil
	case "rename_track", "add_midi_notes":
		return map[string]any{"status": "ok"}, nil
	case "insert_midi_clip":
		return map[string]any{"status": "ok", "clip_id": "clip-1"}, nil
	case "rack_add_node":
		return map[string]any{"status": "ok", "plugin_id": "pl-1"}, nil
	default:
		return map[string]any{"status": "error", "error": "unexpected command " + name}, nil
	}
}

func (f *fakeTutorKernel) recorded() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.commands...)
}

func attachTutorKernel(server *Server, fake *fakeTutorKernel) {
	server.tutorSim.kernelSendOverride = fake.send
}

func tutorAnySlice(t *testing.T, body map[string]any, key string) []any {
	t.Helper()
	value, ok := body[key].([]any)
	if !ok {
		t.Fatalf("field %q = %#v, want array", key, body[key])
	}
	return value
}

func TestTutorV1StartCarriesBriefAndMaterials(t *testing.T) {
	projectPath := writeTutorV1Project(t)
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	body := startTutorConversation(t, server, projectPath, "conv-v1-start")
	if got := tutorString(t, body, "schema_version"); got != "vit.tutor_sim.v1" {
		t.Fatalf("schema_version = %q", got)
	}
	data := tutorMap(t, body, "workflow_data")
	if brief := tutorString(t, data, "brief"); !strings.Contains(brief, "教学简报") {
		t.Fatalf("start brief = %q", brief)
	}
	materials := tutorAnySlice(t, data, "materials")
	if len(materials) != 1 {
		t.Fatalf("start materials = %v", materials)
	}
	material := materials[0].(map[string]any)
	if material["kind"] != "midi_clip" || material["title"] != "主题材料" || material["path"] != "assets/theme.mid" {
		t.Fatalf("material payload = %v", material)
	}
	if int(material["note_count"].(float64)) != 2 {
		t.Fatalf("note_count = %v", material["note_count"])
	}
	if math.Abs(material["duration_beats"].(float64)-252.0/480.0) > 1e-9 {
		t.Fatalf("duration_beats = %v", material["duration_beats"])
	}
	if math.Abs(material["tempo_bpm"].(float64)-100) > 1e-6 {
		t.Fatalf("tempo_bpm = %v", material["tempo_bpm"])
	}
	if material["loaded"] != false || material["loading"] != false {
		t.Fatalf("load state flags = %v", material)
	}
}

func TestTutorV0WorkflowDataKeepsEmptyBriefAndMaterials(t *testing.T) {
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening", "advanced"}}
	server := newTutorTestServer(t, fake)
	body := startTutorConversation(t, server, projectPath, "conv-v0")
	data := tutorMap(t, body, "workflow_data")
	if tutorString(t, data, "brief") != "" {
		t.Fatalf("v0 start brief = %q", tutorString(t, data, "brief"))
	}
	if materials := tutorAnySlice(t, data, "materials"); len(materials) != 0 {
		t.Fatalf("v0 start materials = %v", materials)
	}
	chat := postTutorChat(t, server, "conv-v0", "7")
	chatData := tutorMap(t, chat, "workflow_data")
	if tutorString(t, chatData, "brief") != "" || len(tutorAnySlice(t, chatData, "materials")) != 0 {
		t.Fatalf("v0 chat brief/materials = %v / %v", chatData["brief"], chatData["materials"])
	}
}

func TestTutorV1ChatCarriesBriefAndMaterialsPerStep(t *testing.T) {
	projectPath := writeTutorV1Project(t)
	fake := &fakeTutorCompleter{replies: []string{"opening", "advanced"}}
	server := newTutorTestServer(t, fake)
	startTutorConversation(t, server, projectPath, "conv-v1-chat")
	chat := postTutorChat(t, server, "conv-v1-chat", "42")
	data := tutorMap(t, chat, "workflow_data")
	if tutorString(t, data, "step_id") != "step-confirm" {
		t.Fatalf("advanced step = %v", data["step_id"])
	}
	// 第 2 步无 brief/材料：字段保留但为空（UI 不渲染卡）。
	if tutorString(t, data, "brief") != "" || len(tutorAnySlice(t, data, "materials")) != 0 {
		t.Fatalf("step-2 brief/materials = %v / %v", data["brief"], data["materials"])
	}
	// brief 永不进 LLM 提示词（教学面只在 workflow_data）。
	for i := 0; i < fake.callCount(); i++ {
		for _, message := range fake.request(t, i).Messages {
			if strings.Contains(message.Content, "教学简报") {
				t.Fatalf("brief leaked into prompt %d: %q", i, message.Content)
			}
		}
	}
}

// TUTOR-INPUT-TYPE-1（池序 444）：expect 纯数字 → number，其余 → text；
// 空串（Parse 校验外防御面）→ number 兼容 v0。
func TestTutorSimAnswerKindClassification(t *testing.T) {
	cases := []struct {
		expect string
		want   string
	}{
		{"7", tutorSimAnswerKindNumber},
		{"42", tutorSimAnswerKindNumber},
		{" 123 ", tutorSimAnswerKindNumber},
		{"007", tutorSimAnswerKindNumber},
		{"完成", tutorSimAnswerKindText},
		{"模进", tutorSimAnswerKindText},
		{"已装载", tutorSimAnswerKindText},
		{"12a", tutorSimAnswerKindText},
		{"1 2", tutorSimAnswerKindText},
		{"-5", tutorSimAnswerKindText},
		{"", tutorSimAnswerKindNumber},
		{"   ", tutorSimAnswerKindNumber},
	}
	for _, tc := range cases {
		if got := tutorSimAnswerKind(tc.expect); got != tc.want {
			t.Fatalf("tutorSimAnswerKind(%q) = %q, want %q", tc.expect, got, tc.want)
		}
	}
}

func TestTutorV0CarriesNumericAnswerKind(t *testing.T) {
	// v0 fixture 全部纯数字 expect：start 与 chat 的 workflow_data.answer_kind
	// 恒为 number（数字框=现行为，v0 兼容面）。
	projectPath, _ := writeTutorProject(t, tutorSimTestYAML)
	fake := &fakeTutorCompleter{replies: []string{"opening", "advanced"}}
	server := newTutorTestServer(t, fake)
	body := startTutorConversation(t, server, projectPath, "conv-v0-kind")
	data := tutorMap(t, body, "workflow_data")
	if got := tutorString(t, data, "answer_kind"); got != tutorSimAnswerKindNumber {
		t.Fatalf("v0 start answer_kind = %q, want %q", got, tutorSimAnswerKindNumber)
	}
	// 错输入不推进：answer_kind 仍是当前数字步。
	wrong := postTutorChat(t, server, "conv-v0-kind", "6")
	wrongData := tutorMap(t, wrong, "workflow_data")
	if tutorInt(t, wrongData, "step_index") != 1 {
		t.Fatalf("wrong input advanced: %v", wrongData["step_index"])
	}
	if got := tutorString(t, wrongData, "answer_kind"); got != tutorSimAnswerKindNumber {
		t.Fatalf("v0 retry answer_kind = %q, want %q", got, tutorSimAnswerKindNumber)
	}
	chat := postTutorChat(t, server, "conv-v0-kind", "7")
	chatData := tutorMap(t, chat, "workflow_data")
	if tutorInt(t, chatData, "step_index") != 2 {
		t.Fatalf("advance step_index = %v", chatData["step_index"])
	}
	if got := tutorString(t, chatData, "answer_kind"); got != tutorSimAnswerKindNumber {
		t.Fatalf("v0 chat answer_kind = %q, want %q", got, tutorSimAnswerKindNumber)
	}
}

func TestTutorV1AnswerKindTextStep(t *testing.T) {
	// v1 fixture：第 1 步 expect=42（number）→ 提交 42 推进到 step-confirm
	// （expect=完成）→ chat 回执 answer_kind 翻为 text（词答案=文本框）。
	projectPath := writeTutorV1Project(t)
	fake := &fakeTutorCompleter{replies: []string{"opening", "advanced"}}
	server := newTutorTestServer(t, fake)
	body := startTutorConversation(t, server, projectPath, "conv-v1-kind")
	startData := tutorMap(t, body, "workflow_data")
	if got := tutorString(t, startData, "answer_kind"); got != tutorSimAnswerKindNumber {
		t.Fatalf("v1 start answer_kind = %q, want %q", got, tutorSimAnswerKindNumber)
	}
	chat := postTutorChat(t, server, "conv-v1-kind", "42")
	data := tutorMap(t, chat, "workflow_data")
	if tutorString(t, data, "step_id") != "step-confirm" {
		t.Fatalf("advanced step = %v", data["step_id"])
	}
	if got := tutorString(t, data, "answer_kind"); got != tutorSimAnswerKindText {
		t.Fatalf("step-confirm answer_kind = %q, want %q", got, tutorSimAnswerKindText)
	}
	// 完成态重复输入（AlreadyCompleted）：answer_kind 保持末步（text）。
	repeat := postTutorChat(t, server, "conv-v1-kind", "42")
	repeatData := tutorMap(t, repeat, "workflow_data")
	if got := tutorString(t, repeatData, "answer_kind"); got != tutorSimAnswerKindText {
		t.Fatalf("already-completed answer_kind = %q, want %q", got, tutorSimAnswerKindText)
	}
}

func TestTutorLoadMaterialGuards(t *testing.T) {
	projectPath := writeTutorV1Project(t)
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	attachTutorKernel(server, &fakeTutorKernel{})

	status, _ := doTutorJSON(t, server, http.MethodGet, "/agent/tutor/load_material", nil)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("GET load_material = %d", status)
	}
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{"conversation_id": "c"})
	if status != http.StatusBadRequest || tutorString(t, body, "error") == "" {
		t.Fatalf("missing title status=%d body=%v", status, body)
	}
	status, body = doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{"conversation_id": "missing", "material_title": "主题材料"})
	if status != http.StatusNotFound || tutorString(t, body, "error") != "unknown_tutor_session" {
		t.Fatalf("unknown session status=%d body=%v", status, body)
	}
	startTutorConversation(t, server, projectPath, "conv-guards")
	status, body = doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{"conversation_id": "conv-guards", "material_title": "不存在的材料"})
	if status != http.StatusNotFound || tutorString(t, body, "error") != "unknown_material" {
		t.Fatalf("unknown material status=%d body=%v", status, body)
	}
}

func TestTutorLoadMaterialFullChainAndIdempotency(t *testing.T) {
	projectPath := writeTutorV1Project(t)
	fake := &fakeTutorCompleter{replies: []string{"opening"}}
	server := newTutorTestServer(t, fake)
	kernel := &fakeTutorKernel{}
	attachTutorKernel(server, kernel)
	startTutorConversation(t, server, projectPath, "conv-load")

	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-load", "material_title": "主题材料",
	})
	if status != http.StatusOK || tutorString(t, body, "status") != "ok" {
		t.Fatalf("load status=%d body=%v", status, body)
	}
	load := tutorMap(t, body, "load")
	if load["loaded"] != true || load["track_id"] != "trk-1" || load["clip_id"] != "clip-1" || load["plugin_id"] != "pl-1" {
		t.Fatalf("load payload = %v", load)
	}
	if load["instrument_attached"] != true {
		t.Fatalf("instrument flag = %v", load["instrument_attached"])
	}
	material := tutorMap(t, body, "material")
	if material["note_count"].(float64) != 2 || material["path"] != "assets/theme.mid" {
		t.Fatalf("material payload = %v", material)
	}

	// 命令链=W1 已证命令面（add_track → rename_track → insert_midi_clip →
	// add_midi_notes → rack_add_node）。
	commands := kernel.recorded()
	wantOrder := []string{"add_track", "rename_track", "insert_midi_clip", "add_midi_notes", "rack_add_node"}
	if len(commands) != len(wantOrder) {
		t.Fatalf("kernel commands = %d, want %d: %v", len(commands), len(wantOrder), commands)
	}
	for i, name := range wantOrder {
		if got := commands[i]["cmd"]; got != name {
			t.Fatalf("command %d = %v, want %s", i, got, name)
		}
	}
	if commands[1]["track_id"] != "trk-1" || commands[1]["name"] != "主题材料" {
		t.Fatalf("rename_track args = %v", commands[1])
	}
	if commands[2]["time_unit"] != "beats" || commands[2]["start"] != 0.0 {
		t.Fatalf("insert_midi_clip args = %v", commands[2])
	}
	if math.Abs(commands[2]["length"].(float64)-252.0/480.0) > 1e-9 {
		t.Fatalf("clip length = %v", commands[2]["length"])
	}
	if commands[3]["clip_id"] != "clip-1" || commands[3]["time_unit"] != "beats" {
		t.Fatalf("add_midi_notes args = %v", commands[3])
	}
	notes := commands[3]["notes"].([]map[string]any)
	if len(notes) != 2 || notes[0]["pitch"] != 60 || notes[1]["pitch"] != 67 {
		t.Fatalf("note rows = %v", notes)
	}
	if notes[0]["velocity"] != 90 || notes[1]["velocity"] != 100 {
		t.Fatalf("velocities = %v", notes)
	}
	if commands[4]["plugin_kind"] != "Instrument" || commands[4]["plugin_identifier"] != tutorSimDefaultInstrumentIdentifier || commands[4]["auto_connect"] != true {
		t.Fatalf("rack_add_node args = %v", commands[4])
	}

	// 重复装载：already_loaded 且不再发内核命令。
	status, body = doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-load", "material_title": "主题材料",
	})
	if status != http.StatusOK || tutorString(t, body, "status") != "ok" || tutorBool(t, body, "already_loaded") != true {
		t.Fatalf("reload status=%d body=%v", status, body)
	}
	if len(kernel.recorded()) != len(wantOrder) {
		t.Fatalf("reload issued extra kernel commands: %d", len(kernel.recorded()))
	}
}

func TestTutorLoadMaterialInstrumentRejectionIsNotFatal(t *testing.T) {
	projectPath := writeTutorV1Project(t)
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	kernel := &fakeTutorKernel{replies: []map[string]any{
		{"status": "ok", "track_id": "trk-9"},
		{"status": "ok"},
		{"status": "ok", "clip_id": "clip-9"},
		{"status": "ok"},
		{"status": "error", "error": "plugin not found"},
	}}
	attachTutorKernel(server, kernel)
	startTutorConversation(t, server, projectPath, "conv-inst")
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-inst", "material_title": "主题材料",
	})
	if status != http.StatusOK || tutorString(t, body, "status") != "ok" {
		t.Fatalf("load status=%d body=%v", status, body)
	}
	load := tutorMap(t, body, "load")
	if load["loaded"] != true || load["instrument_attached"] != false {
		t.Fatalf("load payload = %v", load)
	}
	if got := load["instrument_error"]; got != "plugin not found" {
		t.Fatalf("instrument_error = %v", got)
	}
	if load["track_id"] != "trk-9" || load["clip_id"] != "clip-9" {
		t.Fatalf("evidence ids = %v", load)
	}
}

func TestTutorLoadMaterialKernelFailureRecordsEvidenceAndRetries(t *testing.T) {
	projectPath := writeTutorV1Project(t)
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	kernel := &fakeTutorKernel{replies: []map[string]any{
		{"status": "ok", "track_id": "trk-x"},
		{"status": "error", "error": "rename refused"},
	}}
	attachTutorKernel(server, kernel)
	startTutorConversation(t, server, projectPath, "conv-fail")
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-fail", "material_title": "主题材料",
	})
	if status != http.StatusBadGateway || tutorString(t, body, "failure_class") != "kernel_error" {
		t.Fatalf("failure status=%d body=%v", status, body)
	}
	load := tutorMap(t, body, "load")
	if load["loaded"] != false || load["track_id"] != "trk-x" {
		t.Fatalf("evidence payload = %v", load)
	}
	if got := tutorString(t, body, "error"); !strings.Contains(got, "rename refused") {
		t.Fatalf("error = %q", got)
	}
	// 失败后可重试：重置回执为默认 ok，第二次成功。
	kernel.replies = nil
	status, body = doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-fail", "material_title": "主题材料",
	})
	if status != http.StatusOK || tutorString(t, body, "status") != "ok" {
		t.Fatalf("retry status=%d body=%v", status, body)
	}
	if tutorMap(t, body, "load")["loaded"] != true {
		t.Fatalf("retry payload = %v", body["load"])
	}
}

func TestTutorLoadMaterialUnavailableMaterials(t *testing.T) {
	// midi 材料文件缺失：实验加载记录 summary_error，装载拒绝 422。
	projectPath, _ := writeTutorProject(t, tutorSimV1TestYAML)
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	attachTutorKernel(server, &fakeTutorKernel{})
	startTutorConversation(t, server, projectPath, "conv-missing")
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-missing", "material_title": "主题材料",
	})
	if status != http.StatusUnprocessableEntity || tutorString(t, body, "failure_class") != "material_unavailable" {
		t.Fatalf("missing file status=%d body=%v", status, body)
	}
	if tutorMap(t, body, "material")["summary_error"] == "" {
		t.Fatalf("summary_error not surfaced: %v", body["material"])
	}

	// audio 材料：schema 位，本轮不可装载。
	audioYAML := `schema_version: vit.tutor_sim.v1
id: tutor-audio-slot
persona: |
  实验导师。
steps:
  - id: step-audio
    ask: "听音频"
    expect: "ok"
    retry: "重试"
    pass: "通过"
    materials:
      - kind: audio
        title: 音频材料位
`
	audioPath, _ := writeTutorProject(t, audioYAML)
	startTutorConversation(t, server, audioPath, "conv-audio")
	status, body = doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-audio", "material_title": "音频材料位",
	})
	if status != http.StatusUnprocessableEntity || tutorString(t, body, "failure_class") != "audio_not_loadable" {
		t.Fatalf("audio slot status=%d body=%v", status, body)
	}
}

func TestTutorLoadMaterialKernelUnavailable(t *testing.T) {
	projectPath := writeTutorV1Project(t)
	server := newTutorTestServer(t, &fakeTutorCompleter{})
	// 不注入 kernelSendOverride 且 server.kernel 为 nil → kernel_unavailable。
	startTutorConversation(t, server, projectPath, "conv-nokernel")
	status, body := doTutorJSON(t, server, http.MethodPost, "/agent/tutor/load_material", map[string]any{
		"conversation_id": "conv-nokernel", "material_title": "主题材料",
	})
	if status != http.StatusServiceUnavailable || tutorString(t, body, "failure_class") != "kernel_unavailable" {
		t.Fatalf("kernel unavailable status=%d body=%v", status, body)
	}
}
