package vpsforge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/vps"
)

func TestRecordHumanWitnessArchivesUserConfirmedEvidenceWithoutConformance(t *testing.T) {
	root := filepath.Join(t.TempDir(), "staging", "preflight", "pro-q-3")
	if _, err := Init(InitRequest{
		Root:         root,
		Identity:     vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "3.2.3"},
		Capabilities: []string{"equalizer.v2"},
		Now:          time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(root, witnessRoundsFile), map[string]any{
		"schema_version": "vit.vpsforge.preflight.v1",
		"trust":          "unsupported/unknown",
		"rounds": []any{map[string]any{
			"id":       "witness_round_1",
			"badge_id": "equalizer.v2",
			"status":   "planned",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	screenshot := filepath.Join(t.TempDir(), "pro-q-3.png")
	imageBytes := []byte("test-image-bytes")
	if err := os.WriteFile(screenshot, imageBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	statement := "第一行：点击曲线创建点。\n第二行：拖拽该点。"
	result, err := RecordHumanWitness(HumanWitnessRecordRequest{
		Root:            root,
		RoundID:         "witness_round_1",
		UserStatement:   statement,
		ScreenshotPaths: []string{screenshot},
		Now:             time.Date(2026, 7, 17, 12, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RoundStatus != "in_progress" || result.Record.Trust != "user-confirmed" || result.Record.ConformanceStatus == "conformed" {
		t.Fatalf("result = %#v", result)
	}
	if result.Record.UserStatement != statement || result.Record.CredentialImpact != "none" {
		t.Fatalf("record = %#v", result.Record)
	}
	if len(result.Record.Attachments) != 1 {
		t.Fatalf("attachments = %#v", result.Record.Attachments)
	}
	archivedPath := filepath.Join(root, filepath.FromSlash(result.Record.Attachments[0].ArchivedPath))
	archived, err := os.ReadFile(archivedPath)
	if err != nil || string(archived) != string(imageBytes) {
		t.Fatalf("archived screenshot = %q err=%v", archived, err)
	}

	var evidence HumanWitnessEvidence
	if err := readJSON(filepath.Join(root, humanWitnessFile), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Trust != "user-confirmed" || len(evidence.Records) != 1 || evidence.Records[0].UserStatement != statement {
		t.Fatalf("human witness evidence = %#v", evidence)
	}
	var ledger EvidenceLedger
	if err := readJSON(filepath.Join(root, ledgerFile), &ledger); err != nil {
		t.Fatal(err)
	}
	foundLedgerEntry := false
	for _, entry := range ledger.Entries {
		if entry.Kind == "human_gui_witness" {
			foundLedgerEntry = entry.Trust == "user-confirmed" && entry.Source == "user_gui_witness" && entry.Artifact == humanWitnessFile
		}
	}
	if !foundLedgerEntry {
		t.Fatalf("human witness ledger entry missing: %#v", ledger.Entries)
	}
	var rounds map[string]any
	if err := readJSON(filepath.Join(root, witnessRoundsFile), &rounds); err != nil {
		t.Fatal(err)
	}
	round := rounds["rounds"].([]any)[0].(map[string]any)
	if round["status"] != "in_progress" {
		t.Fatalf("round status = %#v", round["status"])
	}
	var manifest Manifest
	if err := readJSON(filepath.Join(root, manifestFile), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Artifacts["human_witness_evidence"] != humanWitnessFile {
		t.Fatalf("manifest artifact = %#v", manifest.Artifacts)
	}
	var draft vps.VPSDocument
	if err := readJSON(filepath.Join(root, draftFile), &draft); err != nil {
		t.Fatal(err)
	}
	if len(draft.ProviderCredentials) != 0 {
		t.Fatalf("witness record must not issue credentials: %#v", draft.ProviderCredentials)
	}
}

func TestRecordHumanWitnessRejectsNonStagingWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := Init(InitRequest{Root: root, Identity: vps.PluginIdentity{Name: "Test EQ", Format: "VST3"}}); err != nil {
		t.Fatal(err)
	}
	_, err := RecordHumanWitness(HumanWitnessRecordRequest{Root: root, RoundID: "witness_round_1", UserStatement: "test", ScreenshotPaths: []string{"test.png"}})
	if err == nil || !strings.Contains(err.Error(), "staging") {
		t.Fatalf("expected staging refusal, got %v", err)
	}
}

func TestRecordHumanWitnessAcceptsTextOnlyConfirmationWithPromptContext(t *testing.T) {
	root := filepath.Join(t.TempDir(), "staging", "preflight", "pro-q-3")
	if _, err := Init(InitRequest{
		Root:     root,
		Identity: vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3"},
		Now:      time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(root, witnessRoundsFile), map[string]any{
		"schema_version": "vit.vpsforge.preflight.v1",
		"rounds":         []any{map[string]any{"id": "witness_round_1", "status": "planned"}},
	}); err != nil {
		t.Fatal(err)
	}
	prompt := "在指定频率区域创建、选择和编辑 EQ 频段。"
	result, err := RecordHumanWitness(HumanWitnessRecordRequest{
		Root:          root,
		RoundID:       "witness_round_1",
		UserStatement: "确认",
		PromptContext: prompt,
		Now:           time.Date(2026, 7, 17, 12, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Record.Attachments) != 0 || result.Record.UserStatement != "确认" || result.Record.PromptContext != prompt {
		t.Fatalf("text-only record = %#v", result.Record)
	}
	var evidence HumanWitnessEvidence
	if err := readJSON(filepath.Join(root, humanWitnessFile), &evidence); err != nil {
		t.Fatal(err)
	}
	if len(evidence.Records) != 1 || evidence.Records[0].ConformanceStatus != "not-conformed" || evidence.Records[0].PromptContext != prompt {
		t.Fatalf("evidence = %#v", evidence)
	}
}
