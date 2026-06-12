package webtools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultTimeout  = 15 * time.Second
	DefaultMaxBytes = 512 * 1024
)

type searchProvider struct {
	source string
	url    string
	parse  func(string) []map[string]any
}

func Fetch(ctx context.Context, args map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rawURL := stringValue(args["url"])
	if rawURL == "" {
		return nil, errors.New("url is required")
	}
	maxBytes := int64(intValue(args["max_bytes"], DefaultMaxBytes))
	u, err := validatePublicURL(rawURL)
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Timeout: timeoutFromArgs(args, DefaultTimeout),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			_, err := validatePublicURL(req.URL.String())
			return err
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	applyBrowserLikeHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	truncated := int64(len(body)) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}
	bodyText := string(body)
	out := map[string]any{
		"url":          u.String(),
		"final_url":    resp.Request.URL.String(),
		"status_code":  resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"bytes":        len(body),
		"truncated":    truncated,
		"text_digest":  TextDigest(bodyText, intValue(args["digest_chars"], 2400)),
		"body_excerpt": TextExcerpt(bodyText, intValue(args["excerpt_chars"], 4000)),
	}
	if boolValue(args["include_body"], false) || boolValue(args["raw_body"], false) {
		out["body"] = bodyText
	}
	return out, nil
}

func TextDigest(body string, max int) string {
	text := cleanText(body)
	if max > 0 && len([]rune(text)) > max {
		runes := []rune(text)
		text = strings.TrimSpace(string(runes[:max])) + "..."
	}
	return text
}

func TextExcerpt(body string, max int) string {
	text := cleanText(body)
	if max > 0 && len([]rune(text)) > max {
		runes := []rune(text)
		text = strings.TrimSpace(string(runes[:max])) + "..."
	}
	return text
}

func Search(ctx context.Context, args map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	query := stringValue(args["query"])
	if query == "" {
		return nil, errors.New("query is required")
	}
	if looksSensitive(query) && !boolValue(args["allow_sensitive_query"], false) {
		return nil, errors.New("query appears to contain local paths or user-specific data; set allow_sensitive_query:true after confirmation")
	}
	limit := intValue(args["max_results"], 8)
	maxBytes := intValue(args["max_bytes"], 256*1024)
	timeoutMS := intValue(args["timeout_ms"], 8000)
	providers := []searchProvider{
		{
			source: "bing_html",
			url:    "https://www.bing.com/search?" + url.Values{"q": []string{query}, "setlang": []string{"zh-CN"}}.Encode(),
			parse:  parseBing,
		},
		{
			source: "duckduckgo_html",
			url:    "https://duckduckgo.com/html/?" + url.Values{"q": []string{query}}.Encode(),
			parse:  parseDuckDuckGo,
		},
	}
	errs := make([]string, 0, len(providers))
	for _, provider := range providers {
		result, err := Fetch(ctx, map[string]any{"url": provider.url, "max_bytes": maxBytes, "timeout_ms": timeoutMS, "include_body": true})
		if err != nil {
			errs = append(errs, provider.source+": "+compactError(err))
			continue
		}
		items := provider.parse(stringValue(result["body"]))
		if len(items) > limit {
			items = items[:limit]
		}
		if len(items) == 0 {
			errs = append(errs, provider.source+": no parseable results")
			continue
		}
		return map[string]any{
			"query":   redactQuery(query),
			"results": items,
			"source":  provider.source,
		}, nil
	}
	return nil, fmt.Errorf("联网搜索失败：搜索服务响应超时或暂时不可用（%s）。可以稍后重试，或改用浏览器搜索工具", strings.Join(errs, "; "))
}

func compactError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if strings.Contains(msg, "Client.Timeout") || strings.Contains(msg, "context deadline exceeded") {
		return "timeout"
	}
	if strings.Contains(msg, "resolve host") {
		return "resolve_failed"
	}
	if len([]rune(msg)) > 140 {
		msg = string([]rune(msg)[:140]) + "..."
	}
	return msg
}

func cleanText(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	body = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<noscript[^>]*>.*?</noscript>`).ReplaceAllString(body, " ")
	body = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(body, " ")
	replacements := map[string]string{
		"&nbsp;": " ",
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": `"`,
		"&#39;":  "'",
	}
	for old, next := range replacements {
		body = strings.ReplaceAll(body, old, next)
	}
	return strings.Join(strings.Fields(body), " ")
}

func validatePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https URLs are allowed")
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New("url host is required")
	}
	if strings.EqualFold(host, "localhost") {
		return nil, errors.New("localhost URLs are blocked")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("blocked private or local IP for host %s", host)
		}
	}
	return u, nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsPrivate() {
		return false
	}
	return true
}

func parseDuckDuckGo(body string) []map[string]any {
	re := regexp.MustCompile(`(?s)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?(?:<a[^>]+class="result__snippet"[^>]*>(.*?)</a>|<div[^>]+class="result__snippet"[^>]*>(.*?)</div>)?`)
	return parseSearchAnchors(body, re)
}

func parseBing(body string) []map[string]any {
	re := regexp.MustCompile(`(?s)<li[^>]+class="[^"]*\bb_algo\b[^"]*"[^>]*>.*?<h2[^>]*>\s*<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?(?:<p[^>]*>(.*?)</p>)?`)
	return parseSearchAnchors(body, re)
}

func parseSearchAnchors(body string, re *regexp.Regexp) []map[string]any {
	tagRe := regexp.MustCompile(`<[^>]+>`)
	matches := re.FindAllStringSubmatch(body, -1)
	out := []map[string]any{}
	seen := map[string]bool{}
	for _, m := range matches {
		title := htmlUnescape(tagRe.ReplaceAllString(m[2], ""))
		link := normalizeResultURL(htmlUnescape(m[1]))
		if title == "" || link == "" || seen[link] {
			continue
		}
		seen[link] = true
		row := map[string]any{"title": title, "url": link}
		if len(m) > 3 {
			snippet := ""
			for _, raw := range m[3:] {
				if text := htmlUnescape(tagRe.ReplaceAllString(raw, "")); text != "" {
					snippet = text
					break
				}
			}
			if snippet != "" {
				row["snippet"] = snippet
			}
		}
		out = append(out, row)
	}
	return out
}

func normalizeResultURL(link string) string {
	link = strings.TrimSpace(link)
	if strings.HasPrefix(link, "//") {
		link = "https:" + link
	}
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	if strings.EqualFold(u.Hostname(), "duckduckgo.com") && strings.HasPrefix(u.Path, "/l/") {
		if uddg := strings.TrimSpace(u.Query().Get("uddg")); uddg != "" {
			return uddg
		}
	}
	return link
}

func looksSensitive(query string) bool {
	q := strings.ToLower(query)
	return strings.Contains(q, `:\`) || strings.Contains(q, "/users/") || strings.Contains(q, "\\users\\") || strings.Contains(q, "d:\\")
}

func redactQuery(query string) string {
	if looksSensitive(query) {
		return "[redacted-sensitive-query]"
	}
	return query
}

func timeoutFromArgs(args map[string]any, fallback time.Duration) time.Duration {
	ms := intValue(args["timeout_ms"], 0)
	if ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	seconds := intValue(args["timeout_seconds"], 0)
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func applyBrowserLikeHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36 VitAgent/0.9")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.7")
}

func htmlUnescape(s string) string {
	replacements := map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": `"`,
		"&#39;":  "'",
	}
	for old, repl := range replacements {
		s = strings.ReplaceAll(s, old, repl)
	}
	return strings.TrimSpace(s)
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func intValue(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		if t > 0 {
			return t
		}
	case int64:
		if t > 0 {
			return int(t)
		}
	case float64:
		if t > 0 {
			return int(t)
		}
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &n); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func boolValue(v any, fallback bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return fallback
}
