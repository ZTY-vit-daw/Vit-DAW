package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpillFileStampHasRealNanosecondSegment(t *testing.T) {
	ts := time.Date(2026, 10, 29, 15, 4, 5, 123456789, time.Local)
	got := spillFileStamp(ts)
	if got != "20261029_150405_123456789" {
		t.Fatalf("spillFileStamp = %q, want %q", got, "20261029_150405_123456789")
	}
}

func TestSpillControlReplySameSecondRequestsGetDistinctFiles(t *testing.T) {
	dir := t.TempDir()
	b := New(Config{FileReplyDir: dir}, nil, nil, nil)
	request := map[string]any{"cmd": "get_plugin_parameters", "request_id": "req_same_second"}
	first := []byte(fmt.Sprintf(`{"status":"ok","payload":"%s"}`, strings.Repeat("a", 4096)))
	second := []byte(fmt.Sprintf(`{"status":"ok","payload":"%s"}`, strings.Repeat("b", 4096)))

	out1, spilled1, err := b.spillControlReply(request, first)
	if err != nil {
		t.Fatalf("first spillControlReply returned error: %v", err)
	}
	if !spilled1 {
		t.Fatalf("first reply was not spilled")
	}
	out2, spilled2, err := b.spillControlReply(request, second)
	if err != nil {
		t.Fatalf("second spillControlReply returned error: %v", err)
	}
	if !spilled2 {
		t.Fatalf("second reply was not spilled")
	}

	path1, path2 := spillFilePath(t, out1), spillFilePath(t, out2)
	if path1 == path2 {
		t.Fatalf("same-second spills collided on one file: %s", path1)
	}
	body1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("reading first spilled reply failed: %v", err)
	}
	if string(body1) != string(first) {
		t.Fatalf("first spilled reply body mismatch")
	}
	body2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("reading second spilled reply failed: %v", err)
	}
	if string(body2) != string(second) {
		t.Fatalf("second spilled reply body mismatch")
	}
}

func TestSpillTelemetryPacketSameSecondLabelsGetDistinctFiles(t *testing.T) {
	dir := t.TempDir()
	b := New(Config{FileReplyDir: dir}, nil, nil, nil)
	first := []byte(fmt.Sprintf(`{"command":"get_project_state","payload":"%s"}`, strings.Repeat("a", 4096)))
	second := []byte(fmt.Sprintf(`{"command":"get_project_state","payload":"%s"}`, strings.Repeat("b", 4096)))

	out1, spilled1, err := b.spillTelemetryPacket(first)
	if err != nil {
		t.Fatalf("first spillTelemetryPacket returned error: %v", err)
	}
	if !spilled1 {
		t.Fatalf("first telemetry packet was not spilled")
	}
	out2, spilled2, err := b.spillTelemetryPacket(second)
	if err != nil {
		t.Fatalf("second spillTelemetryPacket returned error: %v", err)
	}
	if !spilled2 {
		t.Fatalf("second telemetry packet was not spilled")
	}

	path1, path2 := spillFilePath(t, out1), spillFilePath(t, out2)
	if path1 == path2 {
		t.Fatalf("same-second telemetry spills collided on one file: %s", path1)
	}
	body1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("reading first spilled telemetry failed: %v", err)
	}
	if string(body1) != string(first) {
		t.Fatalf("first spilled telemetry body mismatch")
	}
	body2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("reading second spilled telemetry failed: %v", err)
	}
	if string(body2) != string(second) {
		t.Fatalf("second spilled telemetry body mismatch")
	}
}

func TestWriteSpillFileExclusiveSuffixesOnCollision(t *testing.T) {
	dir := t.TempDir()
	name := "20261029_150405_000000000_cmd_req.json"

	firstPath, err := writeSpillFileExclusive(dir, name, []byte("first"))
	if err != nil {
		t.Fatalf("first writeSpillFileExclusive returned error: %v", err)
	}
	secondPath, err := writeSpillFileExclusive(dir, name, []byte("second"))
	if err != nil {
		t.Fatalf("second writeSpillFileExclusive returned error: %v", err)
	}
	if want := "20261029_150405_000000000_cmd_req_1.json"; filepath.Base(secondPath) != want {
		t.Fatalf("collision suffix = %q, want %q", filepath.Base(secondPath), want)
	}
	body1, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("reading first spill file failed: %v", err)
	}
	if string(body1) != "first" {
		t.Fatalf("first spill file was overwritten: %q", string(body1))
	}
	body2, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("reading second spill file failed: %v", err)
	}
	if string(body2) != "second" {
		t.Fatalf("second spill file body mismatch: %q", string(body2))
	}
}

func TestWriteSpillFileExclusiveGivesUpAfterBoundedCollisions(t *testing.T) {
	dir := t.TempDir()
	name := "stamped_cmd_req.json"
	base := strings.TrimSuffix(name, ".json")
	for attempt := 0; attempt < maxSpillFilenameAttempts; attempt++ {
		variant := name
		if attempt > 0 {
			variant = fmt.Sprintf("%s_%d.json", base, attempt)
		}
		if err := os.WriteFile(filepath.Join(dir, variant), []byte("occupied"), 0o644); err != nil {
			t.Fatalf("occupying %q failed: %v", variant, err)
		}
	}
	path, err := writeSpillFileExclusive(dir, name, []byte("data"))
	if err == nil {
		t.Fatalf("expected error after %d collisions, got path %q", maxSpillFilenameAttempts, path)
	}
	if !strings.Contains(err.Error(), "no free name") {
		t.Fatalf("unexpected give-up error: %v", err)
	}
}

func spillFilePath(t *testing.T, envelope []byte) string {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(envelope, &decoded); err != nil {
		t.Fatalf("spill envelope is not JSON: %v", err)
	}
	replyFile := fmt.Sprint(decoded["reply_file"])
	if replyFile == "" {
		t.Fatalf("reply_file missing in envelope: %v", decoded)
	}
	if _, err := os.Stat(filepath.FromSlash(replyFile)); err != nil {
		t.Fatalf("spilled file missing: %v", err)
	}
	return filepath.FromSlash(replyFile)
}
