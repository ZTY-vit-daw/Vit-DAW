package resourceintake

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/artifacts"
)

func TestScanDownloadsHonorsMinModifiedAt(t *testing.T) {
	tmp := t.TempDir()
	downloadDir := filepath.Join(tmp, "Downloads")
	if err := os.Mkdir(downloadDir, 0755); err != nil {
		t.Fatal(err)
	}
	store := artifacts.NewStore(filepath.Join(tmp, "Artifacts"))
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	oldPath := filepath.Join(downloadDir, "old.wav")
	newPath := filepath.Join(downloadDir, "new.wav")
	if err := os.WriteFile(oldPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldPath, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, now.Add(-5*time.Minute), now.Add(-5*time.Minute)); err != nil {
		t.Fatal(err)
	}

	_, err := ScanDownloads(
		store,
		map[string]any{"since_minutes": 24 * 60, "project_id": "test-project"},
		ScanOptions{
			DownloadDirs:  []string{downloadDir},
			MinModifiedAt: now.Add(-30 * time.Minute),
			Now:           now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 scanned artifact, got %d", len(rows))
	}
	if rows[0].Title != "new.wav" {
		t.Fatalf("expected new.wav, got %q", rows[0].Title)
	}
}

func TestScanDownloadsSkipsWhenChangeTokenUnchanged(t *testing.T) {
	tmp := t.TempDir()
	downloadDir := filepath.Join(tmp, "Downloads")
	if err := os.Mkdir(downloadDir, 0755); err != nil {
		t.Fatal(err)
	}
	store := artifacts.NewStore(filepath.Join(tmp, "Artifacts"))
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(downloadDir, "loop.wav")
	if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	first, err := ScanDownloads(
		store,
		map[string]any{"since_minutes": 30},
		ScanOptions{DownloadDirs: []string{downloadDir}, Now: now},
	)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := first["change_token"].(string)
	if token == "" {
		t.Fatal("expected change token")
	}
	second, err := ScanDownloads(
		store,
		map[string]any{"since_minutes": 30, "only_if_changed": true, "change_token": token},
		ScanOptions{DownloadDirs: []string{downloadDir}, Now: now},
	)
	if err != nil {
		t.Fatal(err)
	}
	if changed, _ := second["changed"].(bool); changed {
		t.Fatal("expected unchanged scan result")
	}
	rows, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected existing artifact only, got %d", len(rows))
	}
}

func TestWatchDownloadsReturnsWhenDirectoryChanges(t *testing.T) {
	tmp := t.TempDir()
	downloadDir := filepath.Join(tmp, "Downloads")
	if err := os.Mkdir(downloadDir, 0755); err != nil {
		t.Fatal(err)
	}
	initial := downloadsSnapshot([]string{downloadDir})
	if initial.Token == "" {
		t.Fatal("expected initial token")
	}
	if err := os.WriteFile(filepath.Join(downloadDir, "new.wav"), []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := WatchDownloads(
		context.Background(),
		map[string]any{"change_token": initial.Token, "timeout_ms": 1200, "poll_ms": 100},
		ScanOptions{DownloadDirs: []string{downloadDir}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if changed, _ := result["changed"].(bool); !changed {
		t.Fatalf("expected changed watch result, got %#v", result)
	}
}

func TestRegisterAssetsRegistersPreviewableFiles(t *testing.T) {
	tmp := t.TempDir()
	store := artifacts.NewStore(filepath.Join(tmp, "Artifacts"))
	audioPath := filepath.Join(tmp, "loop.wav")
	skipPath := filepath.Join(tmp, "skip.exe")
	if err := os.WriteFile(audioPath, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skipPath, []byte("binary"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := RegisterAssets(store, map[string]any{
		"file_paths": []any{audioPath, skipPath},
		"project_id": "test-project",
	})
	if err != nil {
		t.Fatalf("RegisterAssets: %v", err)
	}
	rows, ok := result["artifacts"].([]artifacts.Summary)
	if !ok {
		t.Fatalf("artifacts type = %T", result["artifacts"])
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 registered artifact, got %d (%#v)", len(rows), result)
	}
	if rows[0].Kind != "audio" || rows[0].Source != "authorized_media" {
		t.Fatalf("artifact summary = %+v", rows[0])
	}
	stored, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Path != audioPath {
		t.Fatalf("stored artifacts = %+v", stored)
	}
}

func TestIndexAuthorizedFolderFiltersKindsAndRecursion(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "assets")
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "beat.wav"),
		filepath.Join(root, "clip.mp4"),
		filepath.Join(root, "skip.exe"),
		filepath.Join(nested, "nested.wav"),
	} {
		if err := os.WriteFile(path, []byte("asset"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	store := artifacts.NewStore(filepath.Join(tmp, "Artifacts"))
	first, err := IndexAuthorizedFolder(store, map[string]any{
		"asset_location": root,
		"media_kinds":    []any{"audio"},
	})
	if err != nil {
		t.Fatalf("IndexAuthorizedFolder: %v", err)
	}
	rows, ok := first["artifacts"].([]artifacts.Summary)
	if !ok {
		t.Fatalf("artifacts type = %T", first["artifacts"])
	}
	if len(rows) != 1 || rows[0].Title != "beat.wav" {
		t.Fatalf("non-recursive audio rows = %+v", rows)
	}
	second, err := IndexAuthorizedFolder(store, map[string]any{
		"asset_location": root,
		"media_kinds":    []any{"audio"},
		"recursive":      true,
	})
	if err != nil {
		t.Fatalf("IndexAuthorizedFolder recursive: %v", err)
	}
	rows, ok = second["artifacts"].([]artifacts.Summary)
	if !ok {
		t.Fatalf("artifacts type = %T", second["artifacts"])
	}
	if len(rows) != 2 {
		t.Fatalf("recursive audio rows = %+v", rows)
	}
}
