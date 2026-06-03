package shelltools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var allowedPrefixes = [][]string{
	{"go", "test"},
	{"go", "build"},
	{"go", "doc"},
	{"git", "-C"},
	{"gofmt", "-w"},
	{"cmake", "--build"},
}

func Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	command := stringValue(args["command"])
	if command == "" {
		command = strings.Join(stringSlice(args["argv"]), " ")
	}
	if command == "" {
		return nil, errors.New("command is required")
	}
	argv, err := parseCommand(command)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, errors.New("command is empty")
	}
	if !isAllowed(argv) {
		return nil, fmt.Errorf("shell command is not allowlisted: %s", argv[0])
	}
	timeout := time.Duration(intValue(args["timeout_ms"], 120000)) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	if cwd := stringValue(args["cwd"]); cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	status := "ok"
	if err != nil {
		status = "error"
	}
	return map[string]any{
		"status":    status,
		"argv":      argv,
		"cwd":       cmd.Dir,
		"output":    string(out),
		"timed_out": runCtx.Err() == context.DeadlineExceeded,
	}, err
}

func isAllowed(argv []string) bool {
	for _, prefix := range allowedPrefixes {
		if len(argv) < len(prefix) {
			continue
		}
		ok := true
		for i := range prefix {
			if !strings.EqualFold(argv[i], prefix[i]) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func parseCommand(command string) ([]string, error) {
	for _, blocked := range []string{"|", "&&", "||", ";", ">", "<", "$(", "`"} {
		if strings.Contains(command, blocked) {
			return nil, fmt.Errorf("shell metacharacter %q is not allowed", blocked)
		}
	}
	fields := []string{}
	var b strings.Builder
	inQuote := rune(0)
	for _, r := range command {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inQuote = r
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			if b.Len() > 0 {
				fields = append(fields, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if inQuote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if b.Len() > 0 {
		fields = append(fields, b.String())
	}
	return fields, nil
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func stringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, stringValue(item))
		}
		return out
	default:
		return nil
	}
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
