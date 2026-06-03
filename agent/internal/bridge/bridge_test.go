package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestControlReplyPayloadKeepsSmallReplyInline(t *testing.T) {
	b := New(Config{FileReplyDir: t.TempDir()}, nil, nil, nil)
	reply := `{"status":"ok"}`

	out, spilled, err := b.controlReplyPayload(map[string]any{"cmd": "ping", "request_id": "req_1"}, reply)
	if err != nil {
		t.Fatalf("controlReplyPayload returned error: %v", err)
	}
	if spilled {
		t.Fatalf("small reply unexpectedly spilled to file")
	}
	if string(out) != reply {
		t.Fatalf("inline reply mismatch: got %q want %q", string(out), reply)
	}
}

func TestControlReplyPayloadSpillsLargeReplyToFile(t *testing.T) {
	dir := t.TempDir()
	b := New(Config{FileReplyDir: dir}, nil, nil, nil)
	reply := fmt.Sprintf(`{"status":"ok","parameters":["%s"]}`, strings.Repeat("x", maxDirectUDPReplyBytes))

	out, spilled, err := b.controlReplyPayload(map[string]any{
		"cmd":        "get_plugin_parameters",
		"request_id": "req/with unsafe chars",
	}, reply)
	if err != nil {
		t.Fatalf("controlReplyPayload returned error: %v", err)
	}
	if !spilled {
		t.Fatalf("large reply was not spilled")
	}

	var envelope map[string]any
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("file reply envelope is not JSON: %v", err)
	}
	if got := fmt.Sprint(envelope["transport"]); got != "file_reply" {
		t.Fatalf("transport = %q, want file_reply", got)
	}
	replyFile := fmt.Sprint(envelope["reply_file"])
	if replyFile == "" {
		t.Fatalf("reply_file missing in envelope: %v", envelope)
	}
	body, err := os.ReadFile(filepath.FromSlash(replyFile))
	if err != nil {
		t.Fatalf("reading spilled reply failed: %v", err)
	}
	if string(body) != reply {
		t.Fatalf("spilled reply body mismatch")
	}
	base := filepath.Base(replyFile)
	if strings.Contains(base, "/") || strings.Contains(base, "\\") || !strings.Contains(base, "get_plugin_parameters") {
		t.Fatalf("unexpected spilled filename: %q", base)
	}
}
