package llm

import (
	"bytes"
	"context"
	"encoding/json"
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

type Client struct {
	HTTPClient *http.Client
}

func (c *Client) Complete(ctx context.Context, cfg config.EngineConfig, messages []Message) (string, error) {
	if !cfg.Complete() {
		return "", fmt.Errorf("LLM config incomplete")
	}
	endpoint := endpointFor(cfg.BaseURL)
	body := requestBody(endpoint.Kind, cfg.DefaultModel, messages)
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	req.Header.Set("Content-Type", "application/json")

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Text string `json:"text"`
				Type string `json:"type"`
			} `json:"content"`
		} `json:"output"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		text := strings.TrimSpace(string(data))
		if text != "" {
			return "", fmt.Errorf("LLM invalid JSON response: %s", truncate(text, 400))
		}
		return "", err
	}
	if resp.StatusCode >= 400 {
		if msg := errorMessage(out.Error); msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		return "", fmt.Errorf("LLM HTTP error %d", resp.StatusCode)
	}
	if len(out.Choices) == 0 {
		if endpoint.Kind == "responses" {
			if text := responsesText(out.OutputText, out.Output); text != "" {
				return text, nil
			}
		}
		if msg := errorMessage(out.Error); msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		return "", fmt.Errorf("LLM returned no choices")
	}
	return out.Choices[0].Message.Content, nil
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

func requestBody(kind, model string, messages []Message) map[string]any {
	body := map[string]any{
		"model":       strings.TrimSpace(model),
		"temperature": 0.2,
	}
	if kind == "responses" {
		input := make([]map[string]string, 0, len(messages))
		for _, msg := range messages {
			input = append(input, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		}
		body["input"] = input
		return body
	}
	body["messages"] = messages
	return body
}

func responsesText(outputText string, output []struct {
	Content []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"content"`
}) string {
	if strings.TrimSpace(outputText) != "" {
		return strings.TrimSpace(outputText)
	}
	var parts []string
	for _, item := range output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) != "" {
				parts = append(parts, strings.TrimSpace(content.Text))
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
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
