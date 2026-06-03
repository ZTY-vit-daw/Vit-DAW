package webtools

import "testing"

func TestValidatePublicURLRejectsLocalhost(t *testing.T) {
	if _, err := validatePublicURL("http://localhost:7878/health"); err == nil {
		t.Fatal("expected localhost rejection")
	}
}

func TestSearchRejectsSensitiveQueryWithoutOptIn(t *testing.T) {
	if _, err := Search(nil, map[string]any{"query": `D:\Vit_DAW\secret plugin path`}); err == nil {
		t.Fatal("expected sensitive query rejection")
	}
}

func TestParseBingResults(t *testing.T) {
	body := `<html><body><li class="b_algo"><h2><a href="https://example.com/page">Example <strong>Title</strong></a></h2></li></body></html>`
	rows := parseBing(body)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0]["title"] != "Example Title" || rows[0]["url"] != "https://example.com/page" {
		t.Fatalf("row = %+v", rows[0])
	}
}
