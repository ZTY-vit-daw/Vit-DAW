package processorauthority

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	SchemaWavesEQLive            = "waves.static_eq.phase3_live_smoke.v1"
	SchemaPluginAllianceEQLive   = "plugin_alliance.eq_two_level_live_smoke.v1"
	SchemaCompressorCompat       = "plugin_grabber.compressor_compat_matrix_report.v1"
	SchemaCompressorRegression11 = "plugin_grabber.compressor_plugin_alliance_regression.v1.1"
	SchemaCompressorFix12        = "plugin_grabber.compressor_plugin_alliance_fix.v1.2"
	SchemaCompressorBlind3       = "plugin_grabber.compressor_plugin_alliance_blind_report.v3"
)

type Candidate struct {
	Spec       processorattestation.IssueSpec
	Receipt    string
	CaseID     string
	Strong     bool
	GateReason string
}

type ReceiptReport struct {
	Path       string      `json:"path"`
	Schema     string      `json:"schema"`
	Candidates []Candidate `json:"-"`
	Skipped    []Skip      `json:"skipped,omitempty"`
}

type Skip struct {
	CaseID string `json:"case_id,omitempty"`
	Reason string `json:"reason"`
}

type rawReceipt struct {
	SchemaVersion string             `json:"schema_version"`
	Status        string             `json:"status"`
	Verdict       string             `json:"verdict"`
	CompletedAt   string             `json:"completed_at"`
	Audit         rawReceiptAudit    `json:"audit"`
	Results       []rawReceiptResult `json:"results"`
}

type rawReceiptAudit struct {
	NaturalLanguageChatCount int `json:"natural_language_chat_count"`
}

type rawReceiptResult struct {
	ID                       string             `json:"id"`
	CaseID                   string             `json:"case_id"`
	PluginName               string             `json:"plugin_name"`
	Identifier               string             `json:"identifier"`
	Resolution               rawIdentity        `json:"resolution"`
	Mode                     string             `json:"mode"`
	Expectation              string             `json:"expectation"`
	ProcessorFamily          string             `json:"processor_family"`
	Status                   string             `json:"status"`
	Restored                 bool               `json:"restored"`
	TemporaryTrackDeleted    bool               `json:"temporary_track_deleted"`
	TopologyGenerationStable bool               `json:"topology_generation_stable"`
	ApplyStatus              string             `json:"apply_status"`
	RestoreStatus            string             `json:"restore_status"`
	WriteCount               int                `json:"write_count"`
	FullSnapshotRestored     bool               `json:"full_snapshot_restored"`
	SelectedRoles            []string           `json:"selected_roles"`
	RequestedEdits           []rawEQEdit        `json:"requested_edits"`
	Undo                     rawUndo            `json:"undo"`
	ExtraActions             []rawEQExtraAction `json:"extra_actions"`
	Stages                   []rawEQStage       `json:"stages"`
	FinalDrift               []json.RawMessage  `json:"final_drift"`
}

type rawIdentity struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Format       string `json:"format"`
	Identifier   string `json:"identifier"`
	PluginPath   string `json:"plugin_path"`
}

type rawEQEdit struct {
	Action string `json:"action"`
	Shape  string `json:"shape"`
}

type rawUndo struct {
	Rollback struct {
		Verified bool `json:"verified"`
	} `json:"rollback"`
}

type rawEQExtraAction struct {
	Action string `json:"action"`
	Result struct {
		Status string `json:"status"`
		Edits  []struct {
			Action string `json:"action"`
			Shape  string `json:"shape"`
			Status string `json:"status"`
		} `json:"edits"`
		Rollback struct {
			Verified bool `json:"verified"`
		} `json:"rollback"`
	} `json:"result"`
}

type rawEQStage struct {
	RequestedEdits []rawEQEdit `json:"requested_edits"`
	Status         string      `json:"status"`
	Restored       bool        `json:"restored"`
	Undo           rawUndo     `json:"undo"`
}

type rawCaseEvidence struct {
	Case             rawEvidenceCase  `json:"case"`
	FrozenResolution rawIdentity      `json:"frozen_resolution"`
	CompletedAt      string           `json:"completed_at"`
	SelectedRoles    []string         `json:"selected_roles"`
	Result           rawReceiptResult `json:"result"`
}

type rawEvidenceCase struct {
	ID               string `json:"id"`
	PluginName       string `json:"plugin_name"`
	PluginPath       string `json:"plugin_path"`
	PluginIdentifier string `json:"plugin_identifier"`
}

func ReadReceipt(path string, index pluginsemantics.Index) (ReceiptReport, error) {
	path = filepath.Clean(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return ReceiptReport{}, fmt.Errorf("processor authority: read receipt: %w", err)
	}
	var receipt rawReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return ReceiptReport{}, fmt.Errorf("processor authority: decode receipt: %w", err)
	}
	report := ReceiptReport{Path: path, Schema: receipt.SchemaVersion}
	ref := evidenceRef(path, receipt.SchemaVersion, raw, receipt.CompletedAt)
	switch receipt.SchemaVersion {
	case SchemaWavesEQLive, SchemaPluginAllianceEQLive:
		if receipt.Status != "passed" {
			return report, fmt.Errorf("processor authority: EQ receipt status is not passed")
		}
		for _, result := range receipt.Results {
			if receipt.SchemaVersion == SchemaWavesEQLive && receipt.Audit.NaturalLanguageChatCount > 0 &&
				strings.EqualFold(firstText(result.CaseID, result.ID), "q10_stereo") {
				report.Skipped = append(report.Skipped, Skip{CaseID: firstText(result.CaseID, result.ID), Reason: "historical_llm_activity_in_case_evidence"})
				continue
			}
			candidate, reason := eqCandidate(result, ref, index)
			if reason != "" {
				report.Skipped = append(report.Skipped, Skip{CaseID: firstText(result.CaseID, result.ID), Reason: reason})
				continue
			}
			candidate.Receipt = path
			report.Candidates = append(report.Candidates, candidate)
		}
	case SchemaCompressorRegression11, SchemaCompressorFix12, SchemaCompressorBlind3, SchemaLocalCompressorCertification:
		if receipt.Verdict != "passed" {
			return report, fmt.Errorf("processor authority: compressor receipt verdict is not passed")
		}
		for _, result := range receipt.Results {
			candidate, reason := compressorCandidate(path, result, ref, index)
			if reason != "" {
				report.Skipped = append(report.Skipped, Skip{CaseID: firstText(result.ID, result.CaseID), Reason: reason})
				continue
			}
			candidate.Receipt = path
			report.Candidates = append(report.Candidates, candidate)
		}
	case SchemaCompressorCompat:
		for _, result := range receipt.Results {
			report.Skipped = append(report.Skipped, Skip{CaseID: firstText(result.ID, result.CaseID), Reason: "evidence_insufficient_control_write_restore_missing"})
		}
	default:
		return report, fmt.Errorf("processor authority: unsupported receipt schema %q", receipt.SchemaVersion)
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].Spec.Subject.SubjectKey < report.Candidates[j].Spec.Subject.SubjectKey
	})
	return report, nil
}

func eqCandidate(result rawReceiptResult, ref processorattestation.EvidenceRef, index pluginsemantics.Index) (Candidate, string) {
	if result.Mode == "read_only_rejection" {
		return Candidate{}, "negative_or_read_only_case"
	}
	identity := result.Resolution
	identity.Identifier = firstText(identity.Identifier, result.Identifier)
	identity.Name = firstText(identity.Name, result.PluginName)
	coverage := []processorattestation.Coverage{}
	if successful(result.Status) && result.Restored && result.Undo.Rollback.Verified {
		for _, edit := range result.RequestedEdits {
			coverage = appendEQCoverage(coverage, edit.Action, edit.Shape)
			coverage = appendEQCoverage(coverage, "undo", edit.Shape)
		}
	}
	lastShapes := []string{}
	for _, action := range result.ExtraActions {
		if action.Action == "modify" || action.Action == "disable" {
			lastShapes = lastShapes[:0]
			for _, edit := range action.Result.Edits {
				if successful(action.Result.Status) && successful(edit.Status) {
					coverage = appendEQCoverage(coverage, edit.Action, edit.Shape)
					lastShapes = append(lastShapes, edit.Shape)
				}
			}
		} else if strings.HasPrefix(action.Action, "undo_") && action.Result.Rollback.Verified {
			for _, shape := range lastShapes {
				coverage = appendEQCoverage(coverage, "undo", shape)
			}
		}
	}
	for _, stage := range result.Stages {
		if !successful(stage.Status) || !stage.Restored || !stage.Undo.Rollback.Verified {
			continue
		}
		for _, edit := range stage.RequestedEdits {
			coverage = appendEQCoverage(coverage, edit.Action, edit.Shape)
			coverage = appendEQCoverage(coverage, "undo", edit.Shape)
		}
	}
	if len(coverage) == 0 || len(result.FinalDrift) > 0 {
		return Candidate{}, "evidence_insufficient_successful_restored_action_missing"
	}
	subject, reason := resolveSubject(identity, index)
	if reason != "" {
		return Candidate{}, reason
	}
	fingerprint, err := processorattestation.FingerprintPath(subject.InstalledPath)
	if err != nil {
		return Candidate{}, "binary_fingerprint_failed: " + err.Error()
	}
	return Candidate{CaseID: firstText(result.CaseID, result.ID), Strong: true, GateReason: "successful_write_readback_restore",
		Spec: processorattestation.IssueSpec{Subject: subject, BinaryFingerprint: fingerprint,
			ProcessorFamily: processorattestation.FamilyStaticEQ, Coverage: coverage, Evidence: []processorattestation.EvidenceRef{ref}}}, ""
}

func compressorCandidate(summaryPath string, result rawReceiptResult, summaryRef processorattestation.EvidenceRef, index pluginsemantics.Index) (Candidate, string) {
	if result.Expectation != "compressor" || result.Status != "passed" {
		return Candidate{}, "negative_or_failed_case"
	}
	if !result.TemporaryTrackDeleted || !result.TopologyGenerationStable || !successful(result.ApplyStatus) ||
		!successful(result.RestoreStatus) || result.WriteCount <= 0 || !result.FullSnapshotRestored {
		return Candidate{}, "evidence_insufficient_control_gate_failed"
	}
	evidence, caseRef, err := findCaseEvidence(summaryPath, firstText(result.ID, result.CaseID))
	if err != nil {
		return Candidate{}, "case_evidence_missing: " + err.Error()
	}
	roles := result.SelectedRoles
	if len(roles) == 0 {
		roles = firstNonEmptySlice(evidence.SelectedRoles, evidence.Result.SelectedRoles)
	}
	axes := plugingrabber.CompressorAxesForRoles(roles)
	if len(axes) == 0 {
		return Candidate{}, "evidence_insufficient_verified_semantic_axis_missing"
	}
	identity := evidence.FrozenResolution
	identity.Name = firstText(identity.Name, evidence.Case.PluginName, result.PluginName)
	identity.Identifier = firstText(identity.Identifier, evidence.Case.PluginIdentifier)
	identity.PluginPath = firstText(identity.PluginPath, evidence.Case.PluginPath)
	subject, reason := resolveSubject(identity, index)
	if reason != "" {
		return Candidate{}, reason
	}
	fingerprint, err := processorattestation.FingerprintPath(subject.InstalledPath)
	if err != nil {
		return Candidate{}, "binary_fingerprint_failed: " + err.Error()
	}
	coverage := make([]processorattestation.Coverage, 0, len(axes))
	for _, axis := range axes {
		coverage = append(coverage, processorattestation.Coverage{Action: "adjust", Axis: axis})
	}
	return Candidate{CaseID: firstText(result.ID, result.CaseID), Strong: true, GateReason: "successful_write_readback_restore",
		Spec: processorattestation.IssueSpec{Subject: subject, BinaryFingerprint: fingerprint,
			ProcessorFamily: processorattestation.FamilyBroadbandCompressor, Coverage: coverage,
			Evidence: []processorattestation.EvidenceRef{summaryRef, caseRef}}}, ""
}

func findCaseEvidence(summaryPath, caseID string) (rawCaseEvidence, processorattestation.EvidenceRef, error) {
	pattern := filepath.Join(filepath.Dir(summaryPath), "cases", "*"+caseID, "evidence.json")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) != 1 {
		return rawCaseEvidence{}, processorattestation.EvidenceRef{}, fmt.Errorf("expected one case evidence file, found %d", len(matches))
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return rawCaseEvidence{}, processorattestation.EvidenceRef{}, err
	}
	var evidence rawCaseEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return rawCaseEvidence{}, processorattestation.EvidenceRef{}, err
	}
	return evidence, evidenceRef(matches[0], "processor_control_case_evidence.v1", raw, evidence.CompletedAt), nil
}

func resolveSubject(identity rawIdentity, index pluginsemantics.Index) (processorattestation.Subject, string) {
	matches := []pluginsemantics.Entry{}
	for _, entry := range index.Entries {
		if identity.Identifier != "" && strings.EqualFold(entry.Identifier, identity.Identifier) {
			matches = append(matches, entry)
			continue
		}
		if identity.Identifier == "" && strings.EqualFold(entry.Name, identity.Name) &&
			(identity.PluginPath == "" || strings.EqualFold(filepath.Clean(entry.PluginPath), filepath.Clean(identity.PluginPath))) {
			matches = append(matches, entry)
		}
	}
	if len(matches) != 1 {
		return processorattestation.Subject{}, fmt.Sprintf("semantic_identity_resolution_count_%d", len(matches))
	}
	entry := matches[0]
	subject := processorattestation.Subject{Name: entry.Name, Manufacturer: entry.Manufacturer, Format: entry.Format,
		Identifier: entry.Identifier, InstalledPath: entry.PluginPath}
	key, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		return processorattestation.Subject{}, "semantic_identity_invalid: " + err.Error()
	}
	subject.SubjectKey = key
	return subject, ""
}

func appendEQCoverage(values []processorattestation.Coverage, action, shape string) []processorattestation.Coverage {
	action = strings.ToLower(strings.TrimSpace(action))
	shape = strings.ToLower(strings.TrimSpace(shape))
	if action == "" || shape == "" {
		return values
	}
	return append(values, processorattestation.Coverage{Action: action, Shape: shape})
}

func evidenceRef(path, kind string, raw []byte, completedAt string) processorattestation.EvidenceRef {
	digest := sha256.Sum256(raw)
	digestText := hex.EncodeToString(digest[:])
	return processorattestation.EvidenceRef{ReceiptID: "pcr1_" + digestText[:24], Kind: strings.ToLower(kind),
		SHA256: "sha256:" + digestText, ObservedAt: observedAt(path, completedAt), CorpusRecord: filepath.Clean(path)}
}

func observedAt(path, value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed.UTC()
	}
	pattern := regexp.MustCompile(`(?:^|[\\/])(20\d{6})_(\d{6})(?:[\\/]|$)`)
	matches := pattern.FindAllStringSubmatch(filepath.ToSlash(path), -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		if parsed, err := time.ParseInLocation("20060102 150405", last[1]+" "+last[2], time.FixedZone("UTC+8", 8*60*60)); err == nil {
			return parsed.UTC()
		}
	}
	if info, err := os.Stat(path); err == nil && !info.ModTime().IsZero() {
		return info.ModTime().UTC()
	}
	return time.Unix(1, 0).UTC()
}

func successful(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "exact", "quantized", "applied", "ok", "passed":
		return true
	default:
		return false
	}
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmptySlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}
