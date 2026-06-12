package browsercapture

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/artifacts"
)

func CapturePage(store artifacts.Store, args map[string]any) (map[string]any, error) {
	url := stringValue(args["url"])
	title := firstNonEmpty(stringValue(args["title"]), url)
	text := compactRunes(firstNonEmpty(stringValue(args["selection_text"]), stringValue(args["text"])), 30000)
	if url == "" && text == "" {
		return nil, errors.New("url or text is required")
	}
	a := artifacts.New(time.Now())
	a.ID = "art_" + randomID()
	a.Kind = "web_page"
	a.Source = "browser"
	a.URL = url
	a.Title = title
	a.Text = text
	a.Summary = firstLine(text, 240)
	a.ConversationID = stringValue(args["conversation_id"])
	a.GoalID = stringValue(args["goal_id"])
	a.RunID = stringValue(args["run_id"])
	a.Metadata = map[string]any{
		"capture_reason": firstNonEmpty(stringValue(args["capture_reason"]), "user_requested"),
		"selection_only": strings.TrimSpace(stringValue(args["selection_text"])) != "",
		"user_approved":  true,
	}
	a = artifacts.ApplyScope(a, artifacts.ScopeFromMap(args))
	a, err := store.Upsert(a)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":   "ok",
		"artifact": a,
	}, nil
}

func CaptureFetchResult(store artifacts.Store, args map[string]any, result map[string]any) (map[string]any, error) {
	url := firstNonEmpty(stringValue(result["final_url"]), stringValue(result["url"]), stringValue(args["url"]))
	body := compactRunes(stringValue(result["body"]), 30000)
	a := artifacts.New(time.Now())
	a.ID = "art_" + randomID()
	a.Kind = "web_page"
	a.Source = "browser"
	a.URL = url
	a.Title = firstNonEmpty(stringValue(args["title"]), url)
	a.MIME = stringValue(result["content_type"])
	a.SizeBytes = int64Value(result["bytes"])
	a.Text = body
	a.Summary = firstLine(body, 240)
	a.Metadata = map[string]any{
		"status_code": result["status_code"],
		"truncated":   result["truncated"],
		"fetch_mode":  "public_reader",
	}
	a.ConversationID = stringValue(args["conversation_id"])
	a.GoalID = stringValue(args["goal_id"])
	a.RunID = stringValue(args["run_id"])
	a = artifacts.ApplyScope(a, artifacts.ScopeFromMap(args))
	a, err := store.Upsert(a)
	if err != nil {
		return nil, err
	}
	out := cloneMap(result)
	delete(out, "body")
	out["status"] = "ok"
	out["artifact"] = a
	return out, nil
}

func CaptureSearchResult(store artifacts.Store, args map[string]any, result map[string]any) (map[string]any, error) {
	query := stringValue(result["query"])
	if query == "" {
		query = stringValue(args["query"])
	}
	a := artifacts.New(time.Now())
	a.ID = "art_" + randomID()
	a.Kind = "analysis"
	a.Source = "browser"
	a.Title = "Search: " + query
	a.Summary = fmt.Sprintf("Search results for %q", query)
	a.Metadata = map[string]any{
		"query":   query,
		"source":  result["source"],
		"results": result["results"],
	}
	a.ConversationID = stringValue(args["conversation_id"])
	a.GoalID = stringValue(args["goal_id"])
	a.RunID = stringValue(args["run_id"])
	a = artifacts.ApplyScope(a, artifacts.ScopeFromMap(args))
	a, err := store.Upsert(a)
	if err != nil {
		return nil, err
	}
	out := cloneMap(result)
	out["status"] = "ok"
	out["artifact"] = a
	return out, nil
}

func firstLine(text string, max int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Captured browser page."
	}
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	return compactRunes(text, max)
}

func compactRunes(text string, max int) string {
	if max <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "\n[truncated]"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func int64Value(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func randomID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
