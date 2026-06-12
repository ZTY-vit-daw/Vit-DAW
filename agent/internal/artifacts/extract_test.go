package artifacts

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractDOCXText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.docx")
	if err := writeTestDOCX(path, "Hello Vit document reader."); err != nil {
		t.Fatalf("write docx: %v", err)
	}
	a := ArtifactFromFile(path, "art_docx", "conv", "", "")
	a = Extract(a, DefaultTextLimit)
	if a.Kind != "document" {
		t.Fatalf("kind = %q, want document", a.Kind)
	}
	if !strings.Contains(a.Text, "Hello Vit document reader.") {
		t.Fatalf("extracted text missing: %q", a.Text)
	}
	if a.Metadata["document_format"] != "docx" {
		t.Fatalf("metadata document_format = %#v", a.Metadata["document_format"])
	}
}

func TestExtractPDFLiteralTextHeuristic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.pdf")
	pdf := `%PDF-1.4
1 0 obj
<< /Length 44 >>
stream
BT /F1 12 Tf 72 720 Td (Hello Vit PDF reader.) Tj ET
endstream
endobj
%%EOF`
	if err := os.WriteFile(path, []byte(pdf), 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	a := ArtifactFromFile(path, "art_pdf", "conv", "", "")
	a = Extract(a, DefaultTextLimit)
	if a.Kind != "document" {
		t.Fatalf("kind = %q, want document", a.Kind)
	}
	if !strings.Contains(a.Text, "Hello Vit PDF reader.") {
		t.Fatalf("extracted pdf text missing: %q summary=%q metadata=%v", a.Text, a.Summary, a.Metadata)
	}
}

func TestExtractCommandCreatesDocumentFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "command.docx")
	if err := writeTestDOCX(path, "Command path extraction works."); err != nil {
		t.Fatalf("write docx: %v", err)
	}
	store := NewStore(filepath.Join(dir, "artifacts"))
	result, err := ExtractCommand(store, map[string]any{
		"artifact_id": "art_command_doc",
		"path":        path,
		"kind":        "document",
	})
	if err != nil {
		t.Fatalf("ExtractCommand: %v", err)
	}
	artifact, ok := result["artifact"].(Artifact)
	if !ok {
		t.Fatalf("artifact result type = %T", result["artifact"])
	}
	if !strings.Contains(artifact.Text, "Command path extraction works.") {
		t.Fatalf("artifact text missing: %q", artifact.Text)
	}
	if _, err := store.Get("art_command_doc"); err != nil {
		t.Fatalf("stored artifact missing: %v", err)
	}
}

func TestReadCommandAcceptsIDFallback(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.Upsert(Artifact{ID: "art_read_id", Text: "read command fallback"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	result, err := ReadCommand(store, map[string]any{"id": "art_read_id"})
	if err != nil {
		t.Fatalf("ReadCommand: %v", err)
	}
	artifact, ok := result["artifact"].(Artifact)
	if !ok {
		t.Fatalf("artifact result type = %T", result["artifact"])
	}
	if artifact.ID != "art_read_id" {
		t.Fatalf("artifact ID = %q", artifact.ID)
	}
}

func TestExtractCommandAcceptsIDFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fallback.txt")
	if err := os.WriteFile(path, []byte("extract command id fallback"), 0o644); err != nil {
		t.Fatalf("write text: %v", err)
	}
	store := NewStore(filepath.Join(dir, "artifacts"))
	result, err := ExtractCommand(store, map[string]any{
		"id":   "art_extract_id",
		"path": path,
	})
	if err != nil {
		t.Fatalf("ExtractCommand: %v", err)
	}
	artifact, ok := result["artifact"].(Artifact)
	if !ok {
		t.Fatalf("artifact result type = %T", result["artifact"])
	}
	if artifact.ID != "art_extract_id" || !strings.Contains(artifact.Text, "extract command id fallback") {
		t.Fatalf("artifact = %+v", artifact)
	}
}

func writeTestDOCX(path string, text string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	w, err := zw.Create("word/document.xml")
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body>
</w:document>`))
	return err
}
