package vpsforge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// HumanWitnessSchema is deliberately separate from the automatic preflight
	// schema: its contents are raw user testimony plus archived attachments.
	// It cannot establish semantic conformance or issue a Credential.
	HumanWitnessSchema = "vit.vpsforge.human_witness.v1"

	humanWitnessEvidenceDir        = "witness_evidence"
	maxHumanWitnessStatementBytes  = 32 * 1024
	maxHumanWitnessAttachments     = 12
	maxHumanWitnessAttachmentBytes = 32 * 1024 * 1024
)

// HumanWitnessRecordRequest records one user-operated GUI witness round in an
// existing, isolated preflight workspace. UserStatement is retained verbatim
// after validation; no parameter labels or screenshot pixels are promoted to
// semantic authority by this operation.
type HumanWitnessRecordRequest struct {
	Root            string
	RoundID         string
	UserStatement   string
	PromptContext   string
	ScreenshotPaths []string
	Now             time.Time
}

type HumanWitnessRecordResult struct {
	Workspace   string             `json:"workspace"`
	RoundID     string             `json:"round_id"`
	RoundStatus string             `json:"round_status"`
	Record      HumanWitnessRecord `json:"record"`
}

type HumanWitnessEvidence struct {
	SchemaVersion string               `json:"schema_version"`
	WorkspaceID   string               `json:"workspace_id"`
	Trust         string               `json:"trust"`
	Records       []HumanWitnessRecord `json:"records"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type HumanWitnessRecord struct {
	ID                string                   `json:"id"`
	RoundID           string                   `json:"round_id"`
	Trust             string                   `json:"trust"`
	Source            string                   `json:"source"`
	UserStatement     string                   `json:"user_statement"`
	PromptContext     string                   `json:"prompt_context,omitempty"`
	Attachments       []HumanWitnessAttachment `json:"attachments"`
	ConformanceStatus string                   `json:"conformance_status"`
	CredentialImpact  string                   `json:"credential_impact"`
	CapturedAt        time.Time                `json:"captured_at"`
}

type HumanWitnessAttachment struct {
	OriginalFilename string `json:"original_filename"`
	ArchivedPath     string `json:"archived_path"`
	MediaType        string `json:"media_type"`
	SHA256           string `json:"sha256"`
	Bytes            int64  `json:"bytes"`
}

// RecordHumanWitness archives user-provided GUI evidence. It is intentionally
// constrained to a staging/preflight workspace and changes a witness round
// only from planned to in_progress. It never writes a VPS Library, Credential,
// Catalog, or SPAL routing state.
func RecordHumanWitness(request HumanWitnessRecordRequest) (HumanWitnessRecordResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return HumanWitnessRecordResult{}, err
	}
	roundID := strings.TrimSpace(request.RoundID)
	if roundID == "" {
		return HumanWitnessRecordResult{}, fmt.Errorf("witness round id is required")
	}
	if strings.TrimSpace(request.UserStatement) == "" {
		return HumanWitnessRecordResult{}, fmt.Errorf("a non-empty raw user statement is required")
	}
	if len([]byte(request.UserStatement)) > maxHumanWitnessStatementBytes {
		return HumanWitnessRecordResult{}, fmt.Errorf("user statement exceeds %d bytes", maxHumanWitnessStatementBytes)
	}
	if len([]byte(request.PromptContext)) > maxHumanWitnessStatementBytes {
		return HumanWitnessRecordResult{}, fmt.Errorf("prompt context exceeds %d bytes", maxHumanWitnessStatementBytes)
	}
	if len(request.ScreenshotPaths) > maxHumanWitnessAttachments {
		return HumanWitnessRecordResult{}, fmt.Errorf("at most %d screenshots may be attached to one witness record", maxHumanWitnessAttachments)
	}

	status, err := Inspect(root)
	if err != nil {
		return HumanWitnessRecordResult{}, fmt.Errorf("inspect witness workspace: %w", err)
	}
	roundsDocument, err := witnessRoundInProgressDocument(root, roundID)
	if err != nil {
		return HumanWitnessRecordResult{}, err
	}

	var evidence HumanWitnessEvidence
	evidencePath := filepath.Join(root, humanWitnessFile)
	if err := readJSON(evidencePath, &evidence); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return HumanWitnessRecordResult{}, err
		}
		evidence = HumanWitnessEvidence{
			SchemaVersion: HumanWitnessSchema,
			WorkspaceID:   status.Manifest.WorkspaceID,
			Trust:         "user-confirmed",
		}
	} else {
		if evidence.SchemaVersion != HumanWitnessSchema {
			return HumanWitnessRecordResult{}, fmt.Errorf("human witness evidence schema must be %s", HumanWitnessSchema)
		}
		if evidence.WorkspaceID != status.Manifest.WorkspaceID {
			return HumanWitnessRecordResult{}, fmt.Errorf("human witness evidence workspace does not match authoring workspace")
		}
		if evidence.Trust != "user-confirmed" {
			return HumanWitnessRecordResult{}, fmt.Errorf("human witness evidence trust must remain user-confirmed")
		}
	}

	now := nowOrCurrent(request.Now)
	recordID := stableID("human_witness", status.Manifest.WorkspaceID, roundID, request.UserStatement, request.PromptContext, now.Format(time.RFC3339Nano))
	attachments, err := archiveHumanWitnessAttachments(root, recordID, request.ScreenshotPaths)
	if err != nil {
		return HumanWitnessRecordResult{}, err
	}
	record := HumanWitnessRecord{
		ID:                recordID,
		RoundID:           roundID,
		Trust:             "user-confirmed",
		Source:            "user_gui_witness",
		UserStatement:     request.UserStatement,
		PromptContext:     request.PromptContext,
		Attachments:       attachments,
		ConformanceStatus: "not-conformed",
		CredentialImpact:  "none",
		CapturedAt:        now,
	}
	evidence.Records = append(evidence.Records, record)
	evidence.UpdatedAt = now
	if err := writeJSON(evidencePath, evidence); err != nil {
		return HumanWitnessRecordResult{}, err
	}

	ledgerData, err := json.Marshal(map[string]any{
		"record_id":              record.ID,
		"round_id":               record.RoundID,
		"attachments":            record.Attachments,
		"prompt_context_present": strings.TrimSpace(record.PromptContext) != "",
		"conformance_status":     record.ConformanceStatus,
		"credential_impact":      record.CredentialImpact,
	})
	if err != nil {
		return HumanWitnessRecordResult{}, err
	}
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "human_gui_witness",
		Trust:      "user-confirmed",
		Source:     "user_gui_witness",
		Summary:    "Archived user GUI interaction testimony; it is not semantic conformance and does not issue a Credential or modify Catalog or SPAL routing.",
		Artifact:   humanWitnessFile,
		CapturedAt: now,
		Data:       ledgerData,
	}); err != nil {
		return HumanWitnessRecordResult{}, err
	}
	if err := updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["human_witness_evidence"] = humanWitnessFile
		manifest.UpdatedAt = now
	}); err != nil {
		return HumanWitnessRecordResult{}, err
	}
	// Write the planned-to-in-progress change last, so a failure elsewhere can
	// never make a round appear active without an archived record and ledger.
	if err := writeJSON(filepath.Join(root, witnessRoundsFile), roundsDocument); err != nil {
		return HumanWitnessRecordResult{}, err
	}
	return HumanWitnessRecordResult{Workspace: root, RoundID: roundID, RoundStatus: "in_progress", Record: record}, nil
}

func preflightStagingWorkspaceRoot(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("isolated preflight workspace root is required")
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve witness workspace: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve witness workspace: %w", err)
	}
	preflightDirectory := filepath.Dir(resolved)
	stagingDirectory := filepath.Dir(preflightDirectory)
	if !strings.EqualFold(filepath.Base(preflightDirectory), "preflight") || !strings.EqualFold(filepath.Base(stagingDirectory), "staging") {
		return "", fmt.Errorf("human GUI evidence may be recorded only below an isolated staging%spreflight workspace", string(filepath.Separator))
	}
	return resolved, nil
}

func witnessRoundInProgressDocument(root, roundID string) (map[string]any, error) {
	var document map[string]any
	if err := readJSON(filepath.Join(root, witnessRoundsFile), &document); err != nil {
		return nil, err
	}
	rounds, ok := document["rounds"].([]any)
	if !ok || len(rounds) == 0 {
		return nil, fmt.Errorf("witness_rounds.json has no usable rounds")
	}
	for _, item := range rounds {
		round, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("witness_rounds.json contains an invalid round")
		}
		id, ok := round["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("witness_rounds.json contains a round without an id")
		}
		if id != roundID {
			continue
		}
		current, ok := round["status"].(string)
		if !ok {
			return nil, fmt.Errorf("witness round %q has no status", roundID)
		}
		switch strings.TrimSpace(current) {
		case "planned":
			round["status"] = "in_progress"
		case "in_progress":
			// A later attachment may be added while the same round is active.
		default:
			return nil, fmt.Errorf("witness round %q must be planned or in_progress, got %q", roundID, current)
		}
		return document, nil
	}
	return nil, fmt.Errorf("witness round %q does not exist", roundID)
}

func archiveHumanWitnessAttachments(root, recordID string, sourcePaths []string) ([]HumanWitnessAttachment, error) {
	directory := filepath.Join(root, humanWitnessEvidenceDir)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	attachments := make([]HumanWitnessAttachment, 0, len(sourcePaths))
	created := make([]string, 0, len(sourcePaths))
	for index, sourcePath := range sourcePaths {
		attachment, destination, err := archiveHumanWitnessAttachment(directory, recordID, index, strings.TrimSpace(sourcePath))
		if err != nil {
			for _, path := range created {
				_ = os.Remove(path)
			}
			return nil, err
		}
		attachments = append(attachments, attachment)
		created = append(created, destination)
	}
	return attachments, nil
}

func archiveHumanWitnessAttachment(directory, recordID string, index int, sourcePath string) (HumanWitnessAttachment, string, error) {
	if sourcePath == "" {
		return HumanWitnessAttachment{}, "", fmt.Errorf("screenshot %d path is required", index+1)
	}
	extension := strings.ToLower(filepath.Ext(sourcePath))
	mediaType, ok := humanWitnessImageMediaType(extension)
	if !ok {
		return HumanWitnessAttachment{}, "", fmt.Errorf("screenshot %d must be a PNG, JPG, JPEG or WEBP image", index+1)
	}
	input, err := os.Open(sourcePath)
	if err != nil {
		return HumanWitnessAttachment{}, "", fmt.Errorf("open screenshot %d: %w", index+1, err)
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return HumanWitnessAttachment{}, "", fmt.Errorf("inspect screenshot %d: %w", index+1, err)
	}
	if !info.Mode().IsRegular() {
		return HumanWitnessAttachment{}, "", fmt.Errorf("screenshot %d must be a regular file", index+1)
	}
	if info.Size() <= 0 || info.Size() > maxHumanWitnessAttachmentBytes {
		return HumanWitnessAttachment{}, "", fmt.Errorf("screenshot %d must be between 1 and %d bytes", index+1, maxHumanWitnessAttachmentBytes)
	}

	temporary, err := os.CreateTemp(directory, ".human-witness-*")
	if err != nil {
		return HumanWitnessAttachment{}, "", err
	}
	temporaryPath := temporary.Name()
	cleanupTemporary := func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}
	if err := temporary.Chmod(0o600); err != nil {
		cleanupTemporary()
		return HumanWitnessAttachment{}, "", err
	}
	hash := sha256.New()
	bytesWritten, copyErr := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(input, maxHumanWitnessAttachmentBytes+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		_ = os.Remove(temporaryPath)
		return HumanWitnessAttachment{}, "", fmt.Errorf("copy screenshot %d: %w", index+1, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(temporaryPath)
		return HumanWitnessAttachment{}, "", closeErr
	}
	if bytesWritten <= 0 || bytesWritten > maxHumanWitnessAttachmentBytes {
		_ = os.Remove(temporaryPath)
		return HumanWitnessAttachment{}, "", fmt.Errorf("screenshot %d exceeds %d bytes while copying", index+1, maxHumanWitnessAttachmentBytes)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	filename := fmt.Sprintf("witness-%s-%02d%s", recordID, index+1, extension)
	destination := filepath.Join(directory, filename)
	if err := os.Rename(temporaryPath, destination); err != nil {
		_ = os.Remove(temporaryPath)
		return HumanWitnessAttachment{}, "", fmt.Errorf("archive screenshot %d: %w", index+1, err)
	}
	return HumanWitnessAttachment{
		OriginalFilename: filepath.Base(sourcePath),
		ArchivedPath:     filepath.ToSlash(filepath.Join(humanWitnessEvidenceDir, filename)),
		MediaType:        mediaType,
		SHA256:           "sha256:" + digest,
		Bytes:            bytesWritten,
	}, destination, nil
}

func humanWitnessImageMediaType(extension string) (string, bool) {
	switch extension {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".webp":
		return "image/webp", true
	default:
		return "", false
	}
}
