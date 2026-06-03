package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRejectsRootEscape(t *testing.T) {
	root := t.TempDir()
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	if _, _, err := ctx.Resolve("tmp", "..\\outside.txt", false); err == nil {
		t.Fatal("expected root escape error")
	}
}

func TestTextReadRejectsBinary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "blob.bin")
	if err := os.WriteFile(path, []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	if _, err := ReadFile(ctx, map[string]any{"root_id": "tmp", "path": "blob.bin"}); err == nil {
		t.Fatal("expected binary read rejection")
	}
}

func TestApplyEditRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("hello vit"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	if _, err := EditPreview(ctx, map[string]any{"root_id": "tmp", "path": "note.txt", "old_text": "vit", "new_text": "history"}); err != nil {
		t.Fatalf("preview: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "hello vit" {
		t.Fatalf("preview mutated content = %q", string(b))
	}
	if _, err := ApplyEdit(ctx, map[string]any{"root_id": "tmp", "path": "note.txt", "old_text": "vit", "new_text": "history"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "hello history" {
		t.Fatalf("content = %q", string(b))
	}
}

func TestWriteRefusesVitProjectFile(t *testing.T) {
	root := t.TempDir()
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	if _, err := WriteFile(ctx, map[string]any{"root_id": "tmp", "path": "song.vit", "content": "x"}); err == nil {
		t.Fatal("expected live project write rejection")
	}
}

func TestWriteFileOverwriteProtection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	if _, err := WriteFile(ctx, map[string]any{"root_id": "tmp", "path": "note.txt", "content": "new"}); err == nil {
		t.Fatal("expected overwrite protection")
	}
	if b, _ := os.ReadFile(path); string(b) != "old" {
		t.Fatalf("content changed despite overwrite protection: %q", string(b))
	}
	if _, err := WriteFile(ctx, map[string]any{"root_id": "tmp", "path": "note.txt", "content": "new", "overwrite": true}); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "new" {
		t.Fatalf("content = %q", string(b))
	}
}

func TestAbsolutePathOutsideRootsRejected(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	if _, err := ReadFile(ctx, map[string]any{"path": outside}); err == nil {
		t.Fatal("expected absolute path outside roots rejection")
	}
}

func TestBlobInfoAndCopy(t *testing.T) {
	root := t.TempDir()
	blob := []byte{0, 1, 2, 3, 4, 5}
	if err := os.WriteFile(filepath.Join(root, "src.bin"), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	info, err := BlobInfo(ctx, map[string]any{"root_id": "tmp", "path": "src.bin"})
	if err != nil {
		t.Fatalf("blob info: %v", err)
	}
	sum := sha256.Sum256(blob)
	if info["sha256"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha = %+v", info)
	}
	copyResult, err := BlobCopy(ctx, map[string]any{
		"source_root_id": "tmp",
		"source_path":    "src.bin",
		"target_root_id": "tmp",
		"target_path":    "nested/dst.bin",
	})
	if err != nil {
		t.Fatalf("blob copy: %v", err)
	}
	if copyResult["bytes"] != int64(len(blob)) {
		t.Fatalf("copy result = %+v", copyResult)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "nested", "dst.bin")); string(got) != string(blob) {
		t.Fatalf("copied bytes = %#v", got)
	}
	if _, err := BlobCopy(ctx, map[string]any{
		"source_root_id": "tmp",
		"source_path":    "src.bin",
		"target_root_id": "tmp",
		"target_path":    "nested/dst.bin",
	}); err == nil {
		t.Fatal("expected blob overwrite protection")
	}
}

func TestBlobCopyRefusesLiveProjectFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "src.bin"), []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := Context{Roots: []Root{{ID: "tmp", Path: root}}}
	_, err := BlobCopy(ctx, map[string]any{
		"source_root_id": "tmp",
		"source_path":    "src.bin",
		"target_root_id": "tmp",
		"target_path":    "song.vit",
	})
	if err == nil {
		t.Fatal("expected live project blob copy rejection")
	}
}
