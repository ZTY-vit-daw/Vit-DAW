package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Messages   []Message
	Metadata   RequestMetadata
	NoTimeout  bool
	Timeout    time.Duration
	Tools      []map[string]any
	ToolChoice any
	PreferJSON bool
}

type ImageInput struct {
	MIME       string
	DataBase64 string
	URL        string
	Detail     string
}

type VisionRequest struct {
	Prompt   string
	Images   []ImageInput
	Metadata RequestMetadata
}

type RequestMetadata struct {
	Source            string
	ConversationID    string
	GoalID            string
	PromptFingerprint string
	PromptStats       map[string]any
}

type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens"`
}

type Timings struct {
	RequestBuildMs int64 `json:"request_build_ms"`
	HTTPMs         int64 `json:"http_ms"`
	TotalMs        int64 `json:"total_ms"`
}

type Response struct {
	Text    string
	Usage   Usage
	Timings Timings
	RawKind string
}

type llmChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type llmOutputContent struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type llmOutputItem struct {
	Content []llmOutputContent `json:"content"`
}

type llmResponseEnvelope struct {
	Choices    []llmChoice     `json:"choices"`
	OutputText string          `json:"output_text"`
	Output     []llmOutputItem `json:"output"`
	Usage      json.RawMessage `json:"usage"`
	Error      json.RawMessage `json:"error"`
}

type Client struct {
	HTTPClient *http.Client
}

var ErrImageUnderstandingRouteUnavailable = errors.New("image_understanding_route_unavailable")

type Completer interface {
	Complete(context.Context, config.EngineConfig, []Message) (string, error)
}

type RequestCompleter interface {
	CompleteRequest(context.Context, config.EngineConfig, Request) (Response, error)
}

func CompleteText(ctx context.Context, client Completer, cfg config.EngineConfig, req Request) (string, error) {
	if requestClient, ok := client.(RequestCompleter); ok {
		resp, err := requestClient.CompleteRequest(ctx, cfg, req)
		if err != nil {
			return "", err
		}
		return resp.Text, nil
	}
	return client.Complete(ctx, cfg, req.Messages)
}

func (c *Client) Complete(ctx context.Context, cfg config.EngineConfig, messages []Message) (string, error) {
	resp, err := c.CompleteRequest(ctx, cfg, Request{Messages: messages})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

func (c *Client) CompleteRequest(ctx context.Context, cfg config.EngineConfig, llmReq Request) (response Response, err error) {
	totalStart := time.Now()
	endpoint := endpoint{}
	defer func() {
		response.Timings.TotalMs = elapsedMs(totalStart)
		if response.RawKind == "" {
			response.RawKind = endpoint.Kind
		}
		recordTelemetry(cfg, llmReq, response, err)
	}()

	buildStart := time.Now()
	if !cfg.Complete() {
		response.Timings.RequestBuildMs = elapsedMs(buildStart)
		return response, fmt.Errorf("LLM config incomplete")
	}
	endpoint = endpointFor(cfg.BaseURL)
	response.RawKind = endpoint.Kind
	body := requestBody(endpoint.Kind, cfg.DefaultModel, llmReq)
	payload, err := json.Marshal(body)
	response.Timings.RequestBuildMs = elapsedMs(buildStart)
	if err != nil {
		return response, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(payload))
	if err != nil {
		return response, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")
	if endpoint.Kind == "responses" && llmReq.PreferJSON {
		req.Header.Set("Accept", "application/json")
	}

	client := c.HTTPClient
	if client == nil {
		timeout := llmReq.Timeout
		if timeout <= 0 && !llmReq.NoTimeout {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	httpStart := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		response.Timings.HTTPMs = elapsedMs(httpStart)
		return response, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	response.Timings.HTTPMs = elapsedMs(httpStart)
	if err != nil {
		return response, err
	}
	text, usage, err := parseLLMResponseData(data, resp.StatusCode, endpoint.Kind)
	response.Usage = usage
	if err != nil {
		return response, err
	}
	response.Text = text
	return response, nil
}

func (c *Client) CompleteImageUnderstanding(ctx context.Context, cfg config.EngineConfig, visionReq VisionRequest) (response Response, err error) {
	totalStart := time.Now()
	endpoint := endpoint{}
	telemetryReq := Request{
		Messages: []Message{{Role: "user", Content: visionReq.Prompt}},
		Metadata: visionReq.Metadata,
	}
	defer func() {
		response.Timings.TotalMs = elapsedMs(totalStart)
		if response.RawKind == "" {
			response.RawKind = endpoint.Kind
		}
		recordTelemetry(cfg, telemetryReq, response, err)
	}()

	buildStart := time.Now()
	routeCfg, routeErr := imageUnderstandingRouteConfig(cfg)
	if routeErr != nil {
		response.Timings.RequestBuildMs = elapsedMs(buildStart)
		return response, routeErr
	}
	if len(visionReq.Images) == 0 {
		response.Timings.RequestBuildMs = elapsedMs(buildStart)
		return response, fmt.Errorf("image understanding requires at least one image")
	}
	endpoint = imageUnderstandingEndpointFor(routeCfg.BaseURL)
	response.RawKind = endpoint.Kind
	body, err := visionRequestBody(endpoint.Kind, routeCfg.DefaultModel, visionReq)
	if err != nil {
		response.Timings.RequestBuildMs = elapsedMs(buildStart)
		return response, err
	}
	payload, err := json.Marshal(body)
	response.Timings.RequestBuildMs = elapsedMs(buildStart)
	if err != nil {
		return response, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(payload))
	if err != nil {
		return response, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(routeCfg.APIKey))
	req.Header.Set("Content-Type", "application/json")
	if endpoint.Kind == "responses" {
		req.Header.Set("Accept", "text/event-stream")
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	httpStart := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		response.Timings.HTTPMs = elapsedMs(httpStart)
		return response, err
	}
	defer resp.Body.Close()

	if endpoint.Kind == "responses" && isEventStreamContentType(resp.Header.Get("Content-Type")) {
		text, usage, err := parseResponsesSSEStream(resp.Body)
		response.Timings.HTTPMs = elapsedMs(httpStart)
		response.Usage = usage
		if err != nil {
			return response, err
		}
		response.Text = text
		return response, nil
	}

	data, err := io.ReadAll(resp.Body)
	response.Timings.HTTPMs = elapsedMs(httpStart)
	if err != nil {
		return response, err
	}
	text, usage, err := parseLLMResponseData(data, resp.StatusCode, endpoint.Kind)
	response.Usage = usage
	if err != nil {
		return response, err
	}
	response.Text = text
	return response, nil
}

func imageUnderstandingRouteConfig(cfg config.EngineConfig) (config.EngineConfig, error) {
	cfg.Normalize()
	route, ok := cfg.MultimodalRoutes["image_understanding"]
	if !ok || !route.Enabled {
		return config.EngineConfig{}, ErrImageUnderstandingRouteUnavailable
	}
	provider := strings.ToLower(strings.TrimSpace(route.Provider))
	if provider == "" || provider == "default" {
		out := cfg
		if strings.TrimSpace(route.BaseURL) != "" {
			out.BaseURL = strings.TrimSpace(route.BaseURL)
		}
		if strings.TrimSpace(route.APIKey) != "" {
			out.APIKey = strings.TrimSpace(route.APIKey)
		}
		if strings.TrimSpace(route.Model) != "" {
			out.DefaultModel = strings.TrimSpace(route.Model)
		}
		if !out.Complete() {
			return config.EngineConfig{}, ErrImageUnderstandingRouteUnavailable
		}
		return out, nil
	}
	out := config.EngineConfig{
		BaseURL:      strings.TrimSpace(route.BaseURL),
		APIKey:       strings.TrimSpace(route.APIKey),
		DefaultModel: strings.TrimSpace(route.Model),
	}
	if !out.Complete() {
		return config.EngineConfig{}, ErrImageUnderstandingRouteUnavailable
	}
	return out, nil
}

type endpoint struct {
	URL  string
	Kind string
}

func endpointFor(base string) endpoint {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(b, "/responses") {
		return endpoint{URL: b, Kind: "responses"}
	}
	if strings.HasSuffix(b, "/chat/completions") {
		return endpoint{URL: b, Kind: "chat"}
	}
	return endpoint{URL: b + "/chat/completions", Kind: "chat"}
}

func imageUnderstandingEndpointFor(base string) endpoint {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(b, "/responses") {
		return endpoint{URL: b, Kind: "responses"}
	}
	if strings.HasSuffix(b, "/chat/completions") {
		return endpoint{URL: b, Kind: "chat"}
	}
	return endpoint{URL: b + "/responses", Kind: "responses"}
}

func requestBody(kind, model string, req Request) map[string]any {
	body := map[string]any{
		"model":       strings.TrimSpace(model),
		"temperature": 0.2,
	}
	if kind == "responses" {
		if req.PreferJSON {
			body["stream"] = false
		}
		input := make([]map[string]string, 0, len(req.Messages))
		for _, msg := range req.Messages {
			input = append(input, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
		body["input"] = input
		if len(req.Tools) > 0 {
			body["tools"] = req.Tools
		}
		if req.ToolChoice != nil {
			body["tool_choice"] = req.ToolChoice
		}
		return body
	}
	body["messages"] = req.Messages
	if req.PreferJSON {
		// Prompts alone are not a reliable structured-output contract on
		// OpenAI-compatible chat endpoints. C1/B4 planners validate exact JSON
		// schemas, so request the provider's JSON mode when callers opt in.
		body["response_format"] = map[string]any{"type": "json_object"}
	}
	return body
}

func visionRequestBody(kind, model string, req VisionRequest) (map[string]any, error) {
	body := map[string]any{
		"model":       strings.TrimSpace(model),
		"temperature": 0.0,
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Describe the provided image."
	}
	if kind == "responses" {
		content := []map[string]any{{"type": "input_text", "text": prompt}}
		for _, image := range req.Images {
			url, detail, err := imageInputURL(image)
			if err != nil {
				return nil, err
			}
			row := map[string]any{"type": "input_image", "image_url": url}
			if detail != "" {
				row["detail"] = detail
			}
			content = append(content, row)
		}
		body["input"] = []map[string]any{{"role": "user", "content": content}}
		return body, nil
	}
	content := []map[string]any{{"type": "text", "text": prompt}}
	for _, image := range req.Images {
		url, detail, err := imageInputURL(image)
		if err != nil {
			return nil, err
		}
		imageURL := map[string]any{"url": url}
		if detail != "" {
			imageURL["detail"] = detail
		}
		content = append(content, map[string]any{"type": "image_url", "image_url": imageURL})
	}
	body["messages"] = []map[string]any{{"role": "user", "content": content}}
	return body, nil
}

func imageInputURL(image ImageInput) (string, string, error) {
	detail := strings.TrimSpace(image.Detail)
	if detail == "" {
		detail = "auto"
	}
	if url := strings.TrimSpace(image.URL); url != "" {
		return url, detail, nil
	}
	data := strings.TrimSpace(image.DataBase64)
	if data == "" {
		return "", "", fmt.Errorf("image input is missing data")
	}
	mime := strings.TrimSpace(image.MIME)
	if mime == "" {
		mime = "image/png"
	}
	return "data:" + mime + ";base64," + data, detail, nil
}

func parseLLMResponseData(data []byte, statusCode int, kind string) (string, Usage, error) {
	var out llmResponseEnvelope
	trimmed := bytes.TrimSpace(data)
	if looksLikeHTML(trimmed) {
		return "", Usage{}, htmlEndpointError(statusCode, string(trimmed))
	}
	if err := json.Unmarshal(trimmed, &out); err != nil {
		if text, usage, streamErr, ok := parseResponsesSSE(data); ok {
			return text, usage, streamErr
		}
		text := strings.TrimSpace(string(trimmed))
		if statusCode >= 400 {
			if text != "" {
				return "", Usage{}, fmt.Errorf("LLM HTTP error %d: %s", statusCode, truncate(text, 400))
			}
			return "", Usage{}, fmt.Errorf("LLM HTTP error %d", statusCode)
		}
		if text != "" {
			return "", Usage{}, fmt.Errorf("LLM invalid JSON response: %s", truncate(text, 400))
		}
		return "", Usage{}, err
	}

	usage := parseUsage(out.Usage)
	if statusCode >= 400 {
		if msg := errorMessage(out.Error); msg != "" {
			return "", usage, fmt.Errorf("%s", msg)
		}
		return "", usage, fmt.Errorf("LLM HTTP error %d", statusCode)
	}
	if len(out.Choices) == 0 {
		if kind == "responses" {
			if text := responsesText(out.OutputText, out.Output); text != "" {
				return text, usage, nil
			}
		}
		if msg := errorMessage(out.Error); msg != "" {
			return "", usage, fmt.Errorf("%s", msg)
		}
		return "", usage, fmt.Errorf("LLM returned no choices")
	}
	return out.Choices[0].Message.Content, usage, nil
}

func looksLikeHTML(data []byte) bool {
	text := strings.TrimSpace(strings.ToLower(string(data)))
	return strings.HasPrefix(text, "<!doctype html") ||
		strings.HasPrefix(text, "<html") ||
		strings.Contains(text, "<body")
}

func htmlEndpointError(statusCode int, text string) error {
	status := ""
	if statusCode > 0 {
		status = fmt.Sprintf(" HTTP %d", statusCode)
	}
	hint := "LLM endpoint returned HTML" + status + "; baseUrl may point to a web page instead of an API endpoint"
	if strings.Contains(strings.ToLower(text), "right.codes") || strings.Contains(strings.ToLower(text), "claude-aws") {
		hint += ". Try using https://www.right.codes/claude-aws/v1"
	}
	return fmt.Errorf("%s", hint)
}

type responsesSSEState struct {
	deltaParts    []string
	itemDoneText  string
	completedText string
	failedMessage string
	usage         Usage
	seen          bool
	done          bool
}

func (state *responsesSSEState) process(eventName string, dataLines []string) {
	if state == nil || eventName == "" && len(dataLines) == 0 {
		return
	}
	state.seen = true
	payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
	if payload == "" || payload == "[DONE]" {
		return
	}
	var row struct {
		Type  string `json:"type"`
		Delta string `json:"delta"`
		Text  string `json:"text"`
		Item  *struct {
			Content []llmOutputContent `json:"content"`
		} `json:"item"`
		Response *struct {
			Status     string          `json:"status"`
			OutputText string          `json:"output_text"`
			Output     []llmOutputItem `json:"output"`
			Usage      json.RawMessage `json:"usage"`
			Error      json.RawMessage `json:"error"`
		} `json:"response"`
		Error json.RawMessage `json:"error"`
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(payload), &row); err != nil {
		return
	}
	kind := strings.TrimSpace(row.Type)
	if kind == "" {
		kind = eventName
	}
	if len(bytes.TrimSpace(row.Usage)) > 0 {
		if parsed := parseUsage(row.Usage); parsed != (Usage{}) {
			state.usage = parsed
		}
	}
	switch kind {
	case "response.output_text.delta":
		state.deltaParts = append(state.deltaParts, row.Delta)
	case "response.output_text.done":
		if strings.TrimSpace(row.Text) != "" {
			state.itemDoneText = strings.TrimSpace(row.Text)
		}
	case "response.output_item.done":
		if row.Item != nil {
			if text := outputContentText(row.Item.Content); text != "" {
				state.itemDoneText = text
			}
		}
	case "response.completed":
		state.done = true
		if row.Response != nil {
			if parsed := parseUsage(row.Response.Usage); parsed != (Usage{}) {
				state.usage = parsed
			}
			if text := responsesText(row.Response.OutputText, row.Response.Output); text != "" {
				state.completedText = text
			}
		}
	case "response.failed", "response.incomplete":
		state.done = true
		if row.Response != nil {
			if msg := errorMessage(row.Response.Error); msg != "" {
				state.failedMessage = msg
			} else if row.Response.Status != "" {
				state.failedMessage = "LLM responses stream " + row.Response.Status
			}
		}
		if state.failedMessage == "" {
			if msg := errorMessage(row.Error); msg != "" {
				state.failedMessage = msg
			}
		}
	}
}

func (state *responsesSSEState) result() (string, Usage, error, bool) {
	if state == nil || !state.seen {
		return "", Usage{}, nil, false
	}
	if state.failedMessage != "" {
		return "", state.usage, fmt.Errorf("%s", state.failedMessage), true
	}
	if state.completedText != "" {
		return state.completedText, state.usage, nil, true
	}
	if len(state.deltaParts) > 0 {
		return strings.TrimSpace(strings.Join(state.deltaParts, "")), state.usage, nil, true
	}
	if state.itemDoneText != "" {
		return state.itemDoneText, state.usage, nil, true
	}
	return "", state.usage, fmt.Errorf("LLM responses stream returned no output"), true
}

func parseResponsesSSE(data []byte) (string, Usage, error, bool) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.Contains(text, "data:") && !strings.Contains(text, "event:") {
		return "", Usage{}, nil, false
	}
	state := &responsesSSEState{}
	eventName := ""
	dataLines := []string{}
	process := func() {
		state.process(eventName, dataLines)
		eventName = ""
		dataLines = nil
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			process()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
	}
	process()
	return state.result()
}

func parseResponsesSSEStream(reader io.Reader) (string, Usage, error) {
	state := &responsesSSEState{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	eventName := ""
	dataLines := []string{}
	process := func() bool {
		state.process(eventName, dataLines)
		eventName = ""
		dataLines = nil
		return state.done
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if process() {
				text, usage, err, _ := state.result()
				return text, usage, err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
	}
	if len(dataLines) > 0 || eventName != "" {
		process()
	}
	if err := scanner.Err(); err != nil && !state.done {
		return "", state.usage, err
	}
	text, usage, err, ok := state.result()
	if !ok {
		return "", Usage{}, fmt.Errorf("LLM responses stream returned no events")
	}
	return text, usage, err
}

func isEventStreamContentType(value string) bool {
	return strings.Contains(strings.ToLower(value), "event-stream")
}

func responsesText(outputText string, output []llmOutputItem) string {
	if strings.TrimSpace(outputText) != "" {
		return strings.TrimSpace(outputText)
	}
	return outputContentTextFromItems(output)
}

func outputContentTextFromItems(output []llmOutputItem) string {
	var parts []string
	for _, item := range output {
		if text := outputContentText(item.Content); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func outputContentText(content []llmOutputContent) string {
	var parts []string
	for _, item := range content {
		if strings.TrimSpace(item.Text) != "" {
			parts = append(parts, strings.TrimSpace(item.Text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func parseUsage(raw json.RawMessage) Usage {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return Usage{}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return Usage{}
	}
	return Usage{
		InputTokens:         firstIntField(obj, "prompt_tokens", "input_tokens"),
		OutputTokens:        firstIntField(obj, "completion_tokens", "output_tokens"),
		CacheReadTokens:     firstNestedIntField(obj, []string{"prompt_tokens_details", "input_tokens_details"}, "cached_tokens"),
		CacheCreationTokens: firstIntField(obj, "cache_creation_tokens", "cache_creation_input_tokens"),
	}
}

func firstIntField(obj map[string]any, keys ...string) int {
	for _, key := range keys {
		if n := intValue(obj[key]); n != 0 {
			return n
		}
	}
	return 0
}

func firstNestedIntField(obj map[string]any, parents []string, key string) int {
	for _, parent := range parents {
		nested, ok := obj[parent].(map[string]any)
		if !ok {
			continue
		}
		if n := intValue(nested[key]); n != 0 {
			return n
		}
	}
	return 0
}

func intValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func errorMessage(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		for _, key := range []string{"message", "error", "detail", "reason"} {
			if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
			if nested, ok := obj[key].(map[string]any); ok {
				if msg, ok := nested["message"].(string); ok && strings.TrimSpace(msg) != "" {
					return strings.TrimSpace(msg)
				}
			}
		}
	}
	return truncate(string(raw), 400)
}

func truncate(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func elapsedMs(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}
	ms := time.Since(start).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}
