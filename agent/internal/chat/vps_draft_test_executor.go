package chat

// This production file intentionally implements a narrow, local-only test path for VPS
// drafts.  It is not a provider runtime: a passed test never creates a
// Credential, never changes the Provider Catalog, and never makes a mapping
// available to SPAL.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/vps"
)

const (
	vpsDraftTestSchemaVersion                       = "vit.vps.draft_test.v1"
	vpsDraftTestExecutorSource                      = "vps_draft_test_executor"
	vpsDraftTestConfirmationPhrase                  = "I_CONFIRM_DRAFT_VPS_TEST_AND_ROLLBACK"
	vpsDraftTestManualRollbackConfirmationPhrase    = "I_CONFIRM_DRAFT_VPS_GUI_WITNESS_COMPLETE_AND_ROLLBACK"
	vpsDraftTestSurfaceMismatchAcknowledgmentPhrase = "I_ACKNOWLEDGE_DRAFT_SURFACE_MISMATCH_TEST_ONLY"
	vpsDraftTestTicketTTL                           = 10 * time.Minute
	vpsDraftTestMaximumHold                         = 30 * time.Second
	vpsDraftTestMaximumSceneChanges                 = 8
	vpsDraftTestReportsDirectoryEnv                 = "VIT_VPS_DRAFT_TEST_REPORTS_DIR"
	vpsDraftTestObservationModeTimedRollback        = "timed_rollback"
	vpsDraftTestObservationModeManualWitness        = "manual_witness"
	vpsDraftTestObservationStatePreimagePersisted   = "preimage_persisted_before_write"
	vpsDraftTestObservationStateAwaiting            = "awaiting_user_observation"
	vpsDraftTestObservationStateRollbackInProgress  = "rollback_in_progress"
	vpsDraftTestObservationStateRollbackFailed      = "rollback_failed_recovery_required"
	vpsDraftTestActiveObservationsDirectory         = "active_observations"

	vpsDraftTestOnlyBindingStatus = "agent-inferred_test_only"
	vpsDraftTestOnlyScope         = "local_agent_integration_test_only"

	vpsDraftTestSurfaceCompatibilityNotEvaluated               = "not_evaluated"
	vpsDraftTestSurfaceCompatibilityStrictMatch                = "strict_match"
	vpsDraftTestSurfaceCompatibilityDraftFingerprintUnknown    = "draft_parameter_surface_unknown"
	vpsDraftTestSurfaceCompatibilityIdentityMismatch           = "identity_mismatch"
	vpsDraftTestSurfaceCompatibilityObservedSurfaceUnavailable = "observed_parameter_surface_unavailable"
	vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged     = "mismatch_unacknowledged_draft_test_only"
	vpsDraftTestSurfaceCompatibilityMismatchAcknowledged       = "mismatch_acknowledged_draft_test_only"
)

// vpsDraftTestTarget is deliberately instance-scoped.  A reusable VPS never
// stores these project IDs; they live only in a short-lived local test ticket.
type vpsDraftTestTarget struct {
	TrackID  string `json:"track_id"`
	PluginID string `json:"plugin_id"`
}

func (t vpsDraftTestTarget) valid() error {
	if strings.TrimSpace(t.TrackID) == "" || strings.TrimSpace(t.PluginID) == "" {
		return fmt.Errorf("draft VPS test requires track_id and plugin_id")
	}
	return nil
}

type vpsDraftTestPrepareRequest struct {
	VPSID                         string                        `json:"vps_id"`
	Target                        vpsDraftTestTarget            `json:"target"`
	TrackID                       string                        `json:"track_id,omitempty"`
	PluginID                      string                        `json:"plugin_id,omitempty"`
	MappingKey                    string                        `json:"mapping_key"`
	RequestedNormalized           *float64                      `json:"requested_normalized"`
	ScenarioID                    string                        `json:"scenario_id,omitempty"`
	Changes                       []vpsDraftTestRequestedChange `json:"changes,omitempty"`
	HoldMilliseconds              int                           `json:"hold_milliseconds,omitempty"`
	ObservationMode               string                        `json:"observation_mode,omitempty"`
	SurfaceMismatchAcknowledgment string                        `json:"surface_mismatch_acknowledgment,omitempty"`
}

type vpsDraftTestRequestedChange struct {
	MappingKey          string   `json:"mapping_key"`
	RequestedNormalized *float64 `json:"requested_normalized"`
}

// vpsDraftTestChange is the fully resolved, host-addressable portion of one
// requested scene change. It carries only Draft mapping and host facts.
type vpsDraftTestChange struct {
	MappingKey          string  `json:"mapping_key"`
	ParameterID         string  `json:"parameter_id"`
	RequestedNormalized float64 `json:"requested_normalized"`
}

type vpsDraftTestPreparedChange struct {
	Change           vpsDraftTestChange         `json:"change"`
	Mapping          vpsDraftTestMappingSummary `json:"mapping"`
	CurrentParameter map[string]any             `json:"current_parameter"`
}

func (r vpsDraftTestPrepareRequest) resolvedTarget() vpsDraftTestTarget {
	target := r.Target
	if strings.TrimSpace(target.TrackID) == "" {
		target.TrackID = strings.TrimSpace(r.TrackID)
	}
	if strings.TrimSpace(target.PluginID) == "" {
		target.PluginID = strings.TrimSpace(r.PluginID)
	}
	return target
}

func (r vpsDraftTestPrepareRequest) resolvedChanges() ([]vpsDraftTestRequestedChange, error) {
	legacyMappingKey := strings.TrimSpace(r.MappingKey)
	if len(r.Changes) == 0 {
		if legacyMappingKey == "" || r.RequestedNormalized == nil {
			return nil, fmt.Errorf("mapping_key and requested_normalized are required when changes is omitted")
		}
		if !vpsDraftTestNormalizedValueValid(*r.RequestedNormalized) {
			return nil, fmt.Errorf("requested_normalized must be a finite value in [0,1]")
		}
		return []vpsDraftTestRequestedChange{{MappingKey: legacyMappingKey, RequestedNormalized: r.RequestedNormalized}}, nil
	}
	if legacyMappingKey != "" || r.RequestedNormalized != nil {
		return nil, fmt.Errorf("use either mapping_key/requested_normalized or changes, not both")
	}
	if len(r.Changes) > vpsDraftTestMaximumSceneChanges {
		return nil, fmt.Errorf("a Draft GUI-witness scene may contain at most %d changes", vpsDraftTestMaximumSceneChanges)
	}
	seen := map[string]bool{}
	changes := make([]vpsDraftTestRequestedChange, 0, len(r.Changes))
	for _, input := range r.Changes {
		key := strings.TrimSpace(input.MappingKey)
		if key == "" || input.RequestedNormalized == nil || !vpsDraftTestNormalizedValueValid(*input.RequestedNormalized) {
			return nil, fmt.Errorf("every scene change requires a unique mapping_key and finite requested_normalized in [0,1]")
		}
		if seen[key] {
			return nil, fmt.Errorf("scene repeats mapping_key %q", key)
		}
		seen[key] = true
		changes = append(changes, vpsDraftTestRequestedChange{MappingKey: key, RequestedNormalized: input.RequestedNormalized})
	}
	return changes, nil
}

type vpsDraftTestExecuteRequest struct {
	TicketID           string `json:"ticket_id"`
	ConfirmationPhrase string `json:"confirmation_phrase"`
}

type vpsDraftTestManualRollbackRequest struct {
	ObservationSessionID string `json:"observation_session_id"`
	ConfirmationPhrase   string `json:"confirmation_phrase"`
}

// vpsDraftTestSurfaceCompatibility is intentionally scoped to the local Draft
// executor.  A mismatch can only reach the write-confirmation stage when the
// caller supplied the exact acknowledgement during prepare; it never changes
// the Draft's fingerprint or the Credential/Catalog/SPAL rules.
type vpsDraftTestSurfaceCompatibility struct {
	Status                   string `json:"status"`
	ExpectedParameterSurface string `json:"expected_parameter_surface,omitempty"`
	ObservedParameterSurface string `json:"observed_parameter_surface,omitempty"`
	MismatchAcknowledged     bool   `json:"mismatch_acknowledged"`
}

type vpsDraftTestTicket struct {
	ID                  string               `json:"id"`
	VPSID               string               `json:"vps_id"`
	VPSRevision         int                  `json:"vps_revision"`
	Target              vpsDraftTestTarget   `json:"target"`
	MappingKey          string               `json:"mapping_key"`
	ParameterID         string               `json:"parameter_id"`
	RequestedNormalized float64              `json:"requested_normalized"`
	Hold                time.Duration        `json:"hold"`
	ObservationMode     string               `json:"observation_mode"`
	ScenarioID          string               `json:"scenario_id,omitempty"`
	Changes             []vpsDraftTestChange `json:"changes,omitempty"`
	// LibraryPath is empty for the normal user VPS Library.  An explicit
	// staging bridge may instead pin a disposable library for the full
	// write/readback/rollback transaction without changing the Server's
	// canonical Library or Provider Catalog.
	LibraryPath          string                           `json:"library_path,omitempty"`
	ReportRoot           string                           `json:"report_root"`
	SurfaceCompatibility vpsDraftTestSurfaceCompatibility `json:"surface_compatibility"`
	CreatedAt            time.Time                        `json:"created_at"`
	ExpiresAt            time.Time                        `json:"expires_at"`
}

func (t vpsDraftTestTicket) effectiveChanges() []vpsDraftTestChange {
	if len(t.Changes) > 0 {
		return append([]vpsDraftTestChange(nil), t.Changes...)
	}
	if strings.TrimSpace(t.MappingKey) == "" || strings.TrimSpace(t.ParameterID) == "" {
		return nil
	}
	return []vpsDraftTestChange{{
		MappingKey:          t.MappingKey,
		ParameterID:         t.ParameterID,
		RequestedNormalized: t.RequestedNormalized,
	}}
}

type vpsDraftTestMappingSummary struct {
	Key            string            `json:"key"`
	ComponentID    string            `json:"component_id"`
	SemanticSlot   string            `json:"semantic_slot"`
	ParameterID    string            `json:"parameter_id"`
	Label          string            `json:"label,omitempty"`
	DisplayDomain  vps.DisplayDomain `json:"display_domain,omitempty"`
	BindingStatus  string            `json:"binding_status"`
	ExecutionScope string            `json:"execution_scope"`
	Confirmed      bool              `json:"confirmed"`
}

type vpsDraftTestSnapshot struct {
	ParameterCount     int                `json:"parameter_count"`
	NormalizedValues   map[string]float64 `json:"normalized_values"`
	SurfaceValueSHA256 string             `json:"surface_value_sha256"`
}

type vpsDraftTestReceipt struct {
	Stage               string   `json:"stage"`
	MappingKey          string   `json:"mapping_key,omitempty"`
	ParameterID         string   `json:"parameter_id"`
	RequestedNormalized *float64 `json:"requested_normalized,omitempty"`
	ReadbackNormalized  *float64 `json:"readback_normalized,omitempty"`
	ReadbackValueText   string   `json:"readback_value_text,omitempty"`
	Status              string   `json:"status"`
	Error               string   `json:"error,omitempty"`
}

type vpsDraftTestWriteReadback struct {
	Passed   bool                  `json:"passed"`
	Receipts []vpsDraftTestReceipt `json:"receipts,omitempty"`
}

type vpsDraftTestRollback struct {
	Status              string                `json:"status"`
	Passed              bool                  `json:"passed"`
	Rounds              int                   `json:"rounds"`
	ChangedParameterIDs []string              `json:"changed_parameter_ids,omitempty"`
	FinalSnapshot       *vpsDraftTestSnapshot `json:"final_snapshot,omitempty"`
	Receipts            []vpsDraftTestReceipt `json:"receipts,omitempty"`
	Error               string                `json:"error,omitempty"`
}

type vpsDraftTestReport struct {
	SchemaVersion string    `json:"schema_version"`
	RunID         string    `json:"run_id"`
	EvidenceID    string    `json:"evidence_id"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`

	// A transport pass is deliberately narrower than semantic conformance.
	Trust                       string               `json:"trust"`
	TestScope                   string               `json:"test_scope"`
	DoesNotGrant                []string             `json:"does_not_grant"`
	ExplicitWriteConfirmed      bool                 `json:"explicit_write_confirmed"`
	SurfaceCompatibility        string               `json:"surface_compatibility"`
	ExpectedParameterSurface    string               `json:"expected_parameter_surface,omitempty"`
	ObservedParameterSurface    string               `json:"observed_parameter_surface,omitempty"`
	SurfaceMismatchAcknowledged bool                 `json:"surface_mismatch_acknowledged"`
	ObservationMode             string               `json:"observation_mode"`
	ObservationSessionID        string               `json:"observation_session_id,omitempty"`
	ObservationState            string               `json:"observation_state,omitempty"`
	ScenarioID                  string               `json:"scenario_id,omitempty"`
	RequestedChanges            []vpsDraftTestChange `json:"requested_changes,omitempty"`

	VPSID       string                       `json:"vps_id"`
	VPSRevision int                          `json:"vps_revision"`
	VPSStatus   vps.VPSStatus                `json:"vps_status"`
	Target      vpsDraftTestTarget           `json:"target"`
	Mapping     vpsDraftTestMappingSummary   `json:"mapping"`
	Mappings    []vpsDraftTestMappingSummary `json:"mappings,omitempty"`

	RequestedNormalized float64                   `json:"requested_normalized"`
	HoldMilliseconds    int                       `json:"hold_milliseconds"`
	Preimage            *vpsDraftTestSnapshot     `json:"preimage,omitempty"`
	WriteReadback       vpsDraftTestWriteReadback `json:"write_readback"`
	Rollback            vpsDraftTestRollback      `json:"rollback"`

	Status             string `json:"status"`
	FailureCode        string `json:"failure_code,omitempty"`
	MappingDisposition string `json:"mapping_disposition"`
	Error              string `json:"error,omitempty"`
}

type vpsDraftTestDispositionRecord struct {
	SchemaVersion string    `json:"schema_version"`
	RecordedAt    time.Time `json:"recorded_at"`
	VPSID         string    `json:"vps_id"`
	VPSRevision   int       `json:"vps_revision"`
	MappingKey    string    `json:"mapping_key"`
	Disposition   string    `json:"disposition"`
	FailureCode   string    `json:"failure_code,omitempty"`
	EvidenceID    string    `json:"evidence_id"`
	ReportFile    string    `json:"report_file"`
}

// vpsDraftTestObservationSession is a persisted recovery record for a single
// manual GUI witness.  It is saved before the test write, so an Agent restart
// cannot silently discard the complete preimage required for rollback.
type vpsDraftTestObservationSession struct {
	SchemaVersion string             `json:"schema_version"`
	SessionID     string             `json:"session_id"`
	State         string             `json:"state"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	Ticket        vpsDraftTestTicket `json:"ticket"`
	Report        vpsDraftTestReport `json:"report"`
}

func (s vpsDraftTestObservationSession) summary() map[string]any {
	preimageCount := 0
	if s.Report.Preimage != nil {
		preimageCount = s.Report.Preimage.ParameterCount
	}
	return map[string]any{
		"observation_session_id":                s.SessionID,
		"state":                                 s.State,
		"created_at":                            s.CreatedAt,
		"updated_at":                            s.UpdatedAt,
		"vps_id":                                s.Ticket.VPSID,
		"vps_revision":                          s.Ticket.VPSRevision,
		"target":                                s.Ticket.Target,
		"mapping_key":                           s.Ticket.MappingKey,
		"parameter_id":                          s.Ticket.ParameterID,
		"requested_normalized":                  s.Ticket.RequestedNormalized,
		"preimage_parameter_count":              preimageCount,
		"rollback_confirmation_phrase_required": vpsDraftTestManualRollbackConfirmationPhrase,
	}
}

func (s *Server) handleVPSDraftTests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	library, err := s.userVPSLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	documents, err := library.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	reportRoot := vpsDraftTestReportRoot(library.Path())
	activeObservations, activeErr := vpsDraftTestListObservationSessions(reportRoot)
	if activeErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": "read active Draft observation sessions: " + activeErr.Error()})
		return
	}
	activeSummaries := make([]map[string]any, 0, len(activeObservations))
	for _, observation := range activeObservations {
		activeSummaries = append(activeSummaries, observation.summary())
	}
	drafts := make([]map[string]any, 0)
	for _, document := range documents {
		if document.Status != vps.VPSStatusDraft {
			continue
		}
		mappings := make([]map[string]any, 0)
		for _, mapping := range document.ControlSurface.Mappings {
			key := vpsDraftTestMappingKey(mapping)
			if key == "" || !vpsDraftTestOnlyMapping(mapping) {
				continue
			}
			entry := map[string]any{"mapping": vpsDraftTestMappingSummaryFor(mapping), "eligible": true}
			if reason := vpsDraftTestUnsupportedMappingReason(key); reason != "" {
				entry["eligible"] = false
				entry["reason"] = reason
			} else if rejected, rejectedErr := vpsDraftTestMappingRejected(reportRoot, document.ID, document.Revision, key); rejectedErr != nil {
				entry["eligible"] = false
				entry["reason"] = "draft test disposition ledger could not be read: " + rejectedErr.Error()
			} else if rejected {
				entry["eligible"] = false
				entry["reason"] = "rejected for this VPS revision after a hard local mapping failure; revise the Draft before retesting"
			}
			mappings = append(mappings, entry)
		}
		if len(mappings) == 0 {
			continue
		}
		drafts = append(drafts, map[string]any{
			"vps_id":   document.ID,
			"revision": document.Revision,
			"status":   document.Status,
			"plugin": map[string]any{
				"manufacturer": document.PluginIdentity.Manufacturer,
				"name":         document.PluginIdentity.Name,
				"format":       document.PluginIdentity.Format,
				"version":      document.PluginIdentity.Version,
			},
			"mappings": mappings,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                         "ok",
		"schema_version":                 vpsDraftTestSchemaVersion,
		"library_path":                   library.Path(),
		"report_root":                    reportRoot,
		"catalog_visible":                false,
		"credential_issuance":            false,
		"spal_dispatch":                  false,
		"requires_explicit_confirmation": true,
		"confirmation_phrase":            vpsDraftTestConfirmationPhrase,
		"manual_witness": map[string]any{
			"observation_mode":                              vpsDraftTestObservationModeManualWitness,
			"rollback_confirmation_phrase":                  vpsDraftTestManualRollbackConfirmationPhrase,
			"preimage_persisted_before_write":               true,
			"automatic_timed_rollback":                      false,
			"blocks_other_draft_test_writes_until_rollback": true,
			"scene_batch_supported":                         true,
			"maximum_scene_changes":                         vpsDraftTestMaximumSceneChanges,
			"scene_write_order":                             "the explicit request order; every write has fresh readback",
		},
		"active_observations":                    activeSummaries,
		"surface_mismatch_acknowledgment_phrase": vpsDraftTestSurfaceMismatchAcknowledgmentPhrase,
		"surface_mismatch_acknowledgment_scope":  "Draft-only local transport testing; it cannot update a fingerprint or authorize Credential, Catalog, or SPAL use",
		"drafts":                                 drafts,
	})
}

func (s *Server) handleVPSDraftTestPrepare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var request vpsDraftTestPrepareRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	requestedChanges, changesErr := request.resolvedChanges()
	if changesErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": changesErr.Error()})
		return
	}
	if request.HoldMilliseconds < 0 || time.Duration(request.HoldMilliseconds)*time.Millisecond > vpsDraftTestMaximumHold {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "hold_milliseconds must be between 0 and 30000"})
		return
	}
	observationMode, modeErr := vpsDraftTestNormalizeObservationMode(request.ObservationMode, request.HoldMilliseconds)
	if modeErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": modeErr.Error()})
		return
	}
	if len(requestedChanges) > 1 && observationMode != vpsDraftTestObservationModeManualWitness {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "a multi-parameter Draft scene requires observation_mode manual_witness so the user can inspect the final combined state before explicit rollback"})
		return
	}
	if sceneErr := vpsDraftTestValidateSceneComposition(requestedChanges); sceneErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": sceneErr.Error()})
		return
	}
	target := request.resolvedTarget()
	if err := target.valid(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	mappingKeys := make([]string, 0, len(requestedChanges))
	for _, change := range requestedChanges {
		mappingKeys = append(mappingKeys, change.MappingKey)
	}
	library, document, mappings, err := s.resolveVPSDraftTestMappingKeys(request.VPSID, mappingKeys)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	primaryMapping := mappings[0]
	reportRoot := vpsDraftTestReportRoot(library.Path())
	for _, mapping := range mappings {
		mappingKey := vpsDraftTestMappingKey(mapping)
		if reason := vpsDraftTestUnsupportedMappingReason(mappingKey); reason != "" {
			writeJSON(w, http.StatusConflict, map[string]any{"status": "unsupported", "error": reason, "mapping_key": mappingKey})
			return
		}
		if rejected, rejectedErr := vpsDraftTestMappingRejected(reportRoot, document.ID, document.Revision, mappingKey); rejectedErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": "read draft-test disposition ledger: " + rejectedErr.Error()})
			return
		} else if rejected {
			writeJSON(w, http.StatusConflict, map[string]any{"status": "rejected", "error": "this mapping was rejected for the current Draft revision; revise the Draft before retesting", "mapping_key": mappingKey})
			return
		}
	}
	if err := vpsDraftTestEnsureReportRoot(reportRoot); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": "draft-test evidence storage unavailable: " + err.Error()})
		return
	}
	if activeObservations, activeErr := vpsDraftTestListObservationSessions(reportRoot); activeErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": "read active Draft observation sessions: " + activeErr.Error()})
		return
	} else if len(activeObservations) > 0 {
		activeIDs := make([]string, 0, len(activeObservations))
		for _, observation := range activeObservations {
			activeIDs = append(activeIDs, observation.SessionID)
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"status":                         "active_observation_requires_rollback",
			"error":                          "a Draft GUI witness is still holding a live plugin state; finish its explicit rollback before preparing another Draft test",
			"active_observation_session_ids": activeIDs,
			"writes_started":                 false,
		})
		return
	}
	contextData := vpsDraftTestRequestContext(document.ID, vpsDraftTestMappingKey(primaryMapping), "prepare")
	fresh, err := s.readVPSDraftTestParameterSurface(r.Context(), target, contextData)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	digest := buildPluginParameterDigest(fresh)
	surface, surfaceErr := vpsDraftTestValidateLiveSurface(document, digest, request.SurfaceMismatchAcknowledgment)
	if surfaceErr != nil {
		response := map[string]any{
			"status":                     "error",
			"error":                      surfaceErr.Error(),
			"surface_compatibility":      surface.Status,
			"expected_parameter_surface": surface.ExpectedParameterSurface,
			"observed_parameter_surface": surface.ObservedParameterSurface,
			"writes_started":             false,
		}
		if surface.Status == vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged {
			response["surface_mismatch_acknowledgment_required"] = vpsDraftTestSurfaceMismatchAcknowledgmentPhrase
			response["surface_mismatch_acknowledgment_scope"] = "Draft-only local transport testing; it cannot update a fingerprint or authorize Credential, Catalog, or SPAL use"
		}
		writeJSON(w, http.StatusConflict, response)
		return
	}
	preimage, err := vpsDraftTestSnapshotFromDigest(digest)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "complete pre-write parameter snapshot is unavailable: " + err.Error()})
		return
	}
	parameterIndex := vpsParameterIndex(digest)
	ticketChanges := make([]vpsDraftTestChange, 0, len(mappings))
	plannedChanges := make([]vpsDraftTestPreparedChange, 0, len(mappings))
	mappingSummaries := make([]vpsDraftTestMappingSummary, 0, len(mappings))
	for index, mapping := range mappings {
		parameter, found := parameterIndex[mapping.ParameterID]
		mappingKey := vpsDraftTestMappingKey(mapping)
		if !found || !parameter.HostControllable {
			writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "test-only mapped parameter is absent or not host-controllable in the fresh surface", "mapping_key": mappingKey})
			return
		}
		current, finite := vpsFiniteNumber(parameter.NormalizedValue)
		if !finite {
			writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "test-only mapped parameter has no finite normalized readback", "mapping_key": mappingKey})
			return
		}
		requested := *requestedChanges[index].RequestedNormalized
		if vpsNormalizedValuesMatch(current, requested) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "requested_normalized already equals the current value; choose a distinct probe value", "mapping_key": mappingKey})
			return
		}
		change := vpsDraftTestChange{MappingKey: mappingKey, ParameterID: mapping.ParameterID, RequestedNormalized: requested}
		ticketChanges = append(ticketChanges, change)
		mappingSummary := vpsDraftTestMappingSummaryFor(mapping)
		mappingSummaries = append(mappingSummaries, mappingSummary)
		plannedChanges = append(plannedChanges, vpsDraftTestPreparedChange{Change: change, Mapping: mappingSummary, CurrentParameter: vpsDraftTestParameterSummary(parameter)})
	}
	now := time.Now().UTC()
	ticket := vpsDraftTestTicket{
		ID:                   "vps_draft_ticket_" + randomID(),
		VPSID:                document.ID,
		VPSRevision:          document.Revision,
		Target:               target,
		MappingKey:           ticketChanges[0].MappingKey,
		ParameterID:          ticketChanges[0].ParameterID,
		RequestedNormalized:  ticketChanges[0].RequestedNormalized,
		Hold:                 time.Duration(request.HoldMilliseconds) * time.Millisecond,
		ObservationMode:      observationMode,
		ScenarioID:           strings.TrimSpace(request.ScenarioID),
		Changes:              ticketChanges,
		ReportRoot:           reportRoot,
		SurfaceCompatibility: surface,
		CreatedAt:            now,
		ExpiresAt:            now.Add(vpsDraftTestTicketTTL),
	}
	s.putVPSDraftTestTicket(ticket)
	testScope := fmt.Sprintf("%d Draft/test-only parameter mapping(s); fresh write/readback for every change; full normalized parameter snapshot rollback", len(ticketChanges))
	warning := "This is a local transport test only. It will not assert the semantic meaning of the control or issue a Credential."
	if observationMode == vpsDraftTestObservationModeManualWitness {
		testScope = fmt.Sprintf("%d Draft/test-only parameter mapping(s); preimage persisted before the first write; every change fresh-readback verified; final combined state held for GUI witness until an explicit full snapshot rollback", len(ticketChanges))
		warning = "This Draft GUI-witness test will leave the requested combined scene active until a separate explicit rollback confirmation. Do not make unrelated plugin edits while it is held, because rollback restores the complete pre-write parameter snapshot."
	}
	if surface.Status == vpsDraftTestSurfaceCompatibilityMismatchAcknowledged {
		warning += " The Draft parameter-surface mismatch was explicitly acknowledged for this local test only; it does not update the Draft fingerprint or authorize Credential, Catalog, or SPAL use."
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":                        "needs_explicit_confirmation",
		"ticket_id":                     ticket.ID,
		"expires_at":                    ticket.ExpiresAt,
		"confirmation_phrase_required":  vpsDraftTestConfirmationPhrase,
		"catalog_visible":               false,
		"credential_issuance":           false,
		"spal_dispatch":                 false,
		"test_scope":                    testScope,
		"mapping":                       mappingSummaries[0],
		"mappings":                      mappingSummaries,
		"scenario_id":                   ticket.ScenarioID,
		"planned_changes":               plannedChanges,
		"target":                        target,
		"current_parameter":             plannedChanges[0].CurrentParameter,
		"requested_normalized":          ticketChanges[0].RequestedNormalized,
		"requested_changes":             ticketChanges,
		"hold_milliseconds":             request.HoldMilliseconds,
		"observation_mode":              observationMode,
		"preimage_parameter_count":      preimage.ParameterCount,
		"preimage_surface_value_sha256": preimage.SurfaceValueSHA256,
		"surface_compatibility":         surface.Status,
		"expected_parameter_surface":    surface.ExpectedParameterSurface,
		"observed_parameter_surface":    surface.ObservedParameterSurface,
		"surface_mismatch_acknowledged": surface.MismatchAcknowledged,
		"rollback_confirmation_phrase_required": func() string {
			if observationMode == vpsDraftTestObservationModeManualWitness {
				return vpsDraftTestManualRollbackConfirmationPhrase
			}
			return ""
		}(),
		"warning": warning,
	})
}

func (s *Server) handleVPSDraftTestExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var request vpsDraftTestExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	ticket, err := s.takeVPSDraftTestTicket(request.TicketID, request.ConfirmationPhrase)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"status": "error", "error": err.Error(), "writes_started": false})
		return
	}
	// A draft test mutates a live plug-in briefly.  Serializing these runs
	// avoids a second test racing the first test's preimage/rollback cycle.
	s.vpsDraftTestRunMu.Lock()
	defer s.vpsDraftTestRunMu.Unlock()
	report, runErr := s.runVPSDraftTest(r.Context(), ticket)
	if runErr == nil && ticket.ObservationMode == vpsDraftTestObservationModeManualWitness && report.ObservationState == vpsDraftTestObservationStateAwaiting {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":                                vpsDraftTestObservationStateAwaiting,
			"observation_session_id":                report.ObservationSessionID,
			"rollback_confirmation_phrase_required": vpsDraftTestManualRollbackConfirmationPhrase,
			"mapping":                               report.Mapping,
			"mappings":                              report.Mappings,
			"scenario_id":                           report.ScenarioID,
			"target":                                report.Target,
			"requested_normalized":                  report.RequestedNormalized,
			"requested_changes":                     report.RequestedChanges,
			"write_readback":                        report.WriteReadback,
			"preimage_parameter_count":              report.Preimage.ParameterCount,
			"preimage_surface_value_sha256":         report.Preimage.SurfaceValueSHA256,
			"surface_compatibility":                 report.SurfaceCompatibility,
			"catalog_visible":                       false,
			"credential_issuance":                   false,
			"spal_dispatch":                         false,
			"warning":                               "The requested scene is deliberately held for GUI witness. Do not make unrelated plugin edits; the later rollback restores the complete pre-write parameter snapshot.",
		})
		return
	}
	reportPath, persistErr := vpsDraftTestPersistReport(ticket.ReportRoot, report)
	if persistErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":          "error",
			"error":           "draft-test result could not be written to the evidence ledger: " + persistErr.Error(),
			"report":          report,
			"catalog_visible": false,
			"spal_dispatch":   false,
		})
		return
	}
	if runErr != nil {
		writeJSON(w, http.StatusConflict, map[string]any{
			"status":          "failed",
			"error":           runErr.Error(),
			"report_path":     reportPath,
			"report":          report,
			"catalog_visible": false,
			"spal_dispatch":   false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          report.Status,
		"report_path":     reportPath,
		"report":          report,
		"catalog_visible": false,
		"spal_dispatch":   false,
		"warning":         "Passed means host transport + rollback only. The mapping remains test-only and non-dispatchable until a separate semantic/behavior conformance decision.",
	})
}

func (s *Server) handleVPSDraftTestManualRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var request vpsDraftTestManualRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	if request.ConfirmationPhrase != vpsDraftTestManualRollbackConfirmationPhrase {
		writeJSON(w, http.StatusForbidden, map[string]any{"status": "error", "error": "explicit GUI-witness rollback confirmation phrase does not match; no rollback write was started", "writes_started": false})
		return
	}
	library, err := s.userVPSLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error(), "writes_started": false})
		return
	}
	reportRoot := vpsDraftTestReportRoot(library.Path())
	s.vpsDraftTestRunMu.Lock()
	defer s.vpsDraftTestRunMu.Unlock()
	session, err := vpsDraftTestLoadObservationSession(reportRoot, request.ObservationSessionID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": err.Error(), "writes_started": false})
		return
	}
	if session.State != vpsDraftTestObservationStateAwaiting && session.State != vpsDraftTestObservationStateRollbackFailed && session.State != vpsDraftTestObservationStatePreimagePersisted && session.State != vpsDraftTestObservationStateRollbackInProgress {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "Draft GUI witness is not awaiting explicit rollback", "observation": session.summary(), "writes_started": false})
		return
	}
	report, rollbackErr := s.completeVPSDraftTestManualObservation(r.Context(), session)
	if rollbackErr != nil {
		report.ObservationState = vpsDraftTestObservationStateRollbackFailed
		report.Status = vpsDraftTestObservationStateRollbackFailed
		report.Error = rollbackErr.Error()
		session.Report = report
		session.State = vpsDraftTestObservationStateRollbackFailed
		session.UpdatedAt = time.Now().UTC()
		persistErr := vpsDraftTestPersistObservationSession(reportRoot, session)
		if persistErr != nil {
			rollbackErr = fmt.Errorf("%w; also failed to persist active-observation recovery state: %v", rollbackErr, persistErr)
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"status":          vpsDraftTestObservationStateRollbackFailed,
			"error":           rollbackErr.Error(),
			"report":          report,
			"observation":     session.summary(),
			"catalog_visible": false,
			"spal_dispatch":   false,
		})
		return
	}
	reportPath, persistErr := vpsDraftTestPersistReport(reportRoot, report)
	if persistErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":          "error",
			"error":           "Draft GUI-witness rollback passed but its final report could not be written: " + persistErr.Error(),
			"report":          report,
			"catalog_visible": false,
			"spal_dispatch":   false,
		})
		return
	}
	if err := vpsDraftTestRemoveObservationSession(reportRoot, session.SessionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":          "error",
			"error":           "Draft GUI-witness rollback passed and was reported, but its recovery record could not be removed: " + err.Error(),
			"report_path":     reportPath,
			"report":          report,
			"catalog_visible": false,
			"spal_dispatch":   false,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          report.Status,
		"report_path":     reportPath,
		"report":          report,
		"catalog_visible": false,
		"spal_dispatch":   false,
		"warning":         "Passed means host transport + explicit full-snapshot rollback only. The mapping remains test-only and non-dispatchable until separate semantic/behavior conformance.",
	})
}

func (s *Server) resolveVPSDraftTestMapping(vpsID, mappingKey string) (*vps.Library, vps.VPSDocument, vps.ControlSurfaceMapping, error) {
	library, document, mappings, err := s.resolveVPSDraftTestMappingKeys(vpsID, []string{mappingKey})
	if err != nil {
		return nil, vps.VPSDocument{}, vps.ControlSurfaceMapping{}, err
	}
	return library, document, mappings[0], nil
}

func (s *Server) resolveVPSDraftTestMappingKeys(vpsID string, mappingKeys []string) (*vps.Library, vps.VPSDocument, []vps.ControlSurfaceMapping, error) {
	vpsID = strings.TrimSpace(vpsID)
	if vpsID == "" || len(mappingKeys) == 0 {
		return nil, vps.VPSDocument{}, nil, fmt.Errorf("vps_id and at least one mapping_key are required")
	}
	library, err := s.userVPSLibrary()
	if err != nil {
		return nil, vps.VPSDocument{}, nil, err
	}
	return vpsDraftTestResolveMappingKeysFromLibrary(library, vpsID, mappingKeys)
}

func (s *Server) resolveVPSDraftTestTicketMappingKeys(ticket vpsDraftTestTicket, mappingKeys []string) (*vps.Library, vps.VPSDocument, []vps.ControlSurfaceMapping, error) {
	libraryPath := strings.TrimSpace(ticket.LibraryPath)
	if libraryPath == "" {
		return s.resolveVPSDraftTestMappingKeys(ticket.VPSID, mappingKeys)
	}
	library, err := vps.NewLibrary(libraryPath)
	if err != nil {
		return nil, vps.VPSDocument{}, nil, fmt.Errorf("open ticket-scoped staging VPS library: %w", err)
	}
	return vpsDraftTestResolveMappingKeysFromLibrary(library, ticket.VPSID, mappingKeys)
}

func vpsDraftTestResolveMappingKeysFromLibrary(library *vps.Library, vpsID string, mappingKeys []string) (*vps.Library, vps.VPSDocument, []vps.ControlSurfaceMapping, error) {
	vpsID = strings.TrimSpace(vpsID)
	if library == nil || vpsID == "" || len(mappingKeys) == 0 {
		return nil, vps.VPSDocument{}, nil, fmt.Errorf("VPS library, vps_id and at least one mapping_key are required")
	}
	document, found, err := library.Get(vpsID)
	if err != nil {
		return nil, vps.VPSDocument{}, nil, err
	}
	if !found {
		return nil, vps.VPSDocument{}, nil, fmt.Errorf("draft VPS %q was not found", vpsID)
	}
	mappings := make([]vps.ControlSurfaceMapping, 0, len(mappingKeys))
	seenKeys := map[string]bool{}
	seenParameters := map[string]bool{}
	for _, rawKey := range mappingKeys {
		mappingKey := strings.TrimSpace(rawKey)
		if mappingKey == "" || seenKeys[mappingKey] {
			return nil, vps.VPSDocument{}, nil, fmt.Errorf("Draft scene contains a missing or duplicate mapping_key")
		}
		seenKeys[mappingKey] = true
		mapping, findErr := vpsDraftTestFindMapping(document, mappingKey)
		if findErr != nil {
			return nil, vps.VPSDocument{}, nil, findErr
		}
		if seenParameters[mapping.ParameterID] {
			return nil, vps.VPSDocument{}, nil, fmt.Errorf("Draft scene mapping %q repeats host parameter %s", mappingKey, mapping.ParameterID)
		}
		seenParameters[mapping.ParameterID] = true
		mappings = append(mappings, mapping)
	}
	return library, document, mappings, nil
}

func vpsDraftTestFindMapping(document vps.VPSDocument, mappingKey string) (vps.ControlSurfaceMapping, error) {
	if document.Status != vps.VPSStatusDraft {
		return vps.ControlSurfaceMapping{}, fmt.Errorf("VPS %s is %s, not a draft", document.ID, document.Status)
	}
	for _, credential := range document.ProviderCredentials {
		if credential.Dispatchable() {
			return vps.ControlSurfaceMapping{}, fmt.Errorf("draft-test executor refuses a document containing a dispatchable Credential")
		}
	}
	var found *vps.ControlSurfaceMapping
	for _, mapping := range document.ControlSurface.Mappings {
		if vpsDraftTestMappingKey(mapping) != mappingKey {
			continue
		}
		if found != nil {
			return vps.ControlSurfaceMapping{}, fmt.Errorf("draft VPS has duplicate test mapping key %q", mappingKey)
		}
		copy := mapping
		found = &copy
	}
	if found == nil {
		return vps.ControlSurfaceMapping{}, fmt.Errorf("test-only mapping %q was not found", mappingKey)
	}
	if !vpsDraftTestOnlyMapping(*found) {
		return vps.ControlSurfaceMapping{}, fmt.Errorf("mapping %q is not explicitly marked agent-inferred_test_only/local_agent_integration_test_only", mappingKey)
	}
	if strings.TrimSpace(found.ParameterID) == "" {
		return vps.ControlSurfaceMapping{}, fmt.Errorf("test-only mapping %q has no parameter_id", mappingKey)
	}
	return *found, nil
}

func vpsDraftTestOnlyMapping(mapping vps.ControlSurfaceMapping) bool {
	if mapping.Confirmed {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(mapping.BindingStatus))
	scope := strings.ToLower(strings.TrimSpace(mapping.ExecutionScope))
	return (status == vpsDraftTestOnlyBindingStatus && scope == vpsDraftTestOnlyScope) ||
		(status == "user-confirmed_staging_executable" && scope == "forge_staging_actual_test")
}

func vpsDraftTestMappingKey(mapping vps.ControlSurfaceMapping) string {
	component := strings.TrimSpace(mapping.ComponentID)
	slot := strings.TrimSpace(mapping.SemanticSlot)
	if component == "" || slot == "" {
		return ""
	}
	return component + "." + slot
}

func vpsDraftTestUnsupportedMappingReason(mappingKey string) string {
	// A program/preset change can rewrite a whole plug-in state.  The generic
	// one-parameter executor cannot honestly prove its complete rollback, so
	// it remains unsupported until a dedicated preset protocol exists.
	if mappingKey == "factory_presets.program" {
		return "factory preset/program tests are unsupported here because one selection can alter many parameters; no generic preset rollback protocol has been conformed"
	}
	return ""
}

func vpsDraftTestMappingSummaryFor(mapping vps.ControlSurfaceMapping) vpsDraftTestMappingSummary {
	return vpsDraftTestMappingSummary{
		Key:            vpsDraftTestMappingKey(mapping),
		ComponentID:    mapping.ComponentID,
		SemanticSlot:   mapping.SemanticSlot,
		ParameterID:    mapping.ParameterID,
		Label:          mapping.Label,
		DisplayDomain:  mapping.DisplayDomain,
		BindingStatus:  mapping.BindingStatus,
		ExecutionScope: mapping.ExecutionScope,
		Confirmed:      mapping.Confirmed,
	}
}

func vpsDraftTestParameterSummary(parameter pluginParameterInfo) map[string]any {
	result := map[string]any{
		"parameter_id":      parameter.ID,
		"host_controllable": parameter.HostControllable,
		"normalized_value":  parameter.NormalizedValue,
		"value_text":        parameter.ValueText,
		"unit":              parameter.Unit,
		"is_boolean":        parameter.IsBoolean,
		"is_discrete":       parameter.IsDiscrete,
		"num_steps":         parameter.NumSteps,
	}
	if parameter.DisplayDomainCandidate != nil {
		result["display_domain_candidate"] = parameter.DisplayDomainCandidate
	}
	return result
}

func (s *Server) putVPSDraftTestTicket(ticket vpsDraftTestTicket) {
	if s == nil {
		return
	}
	s.vpsDraftTestMu.Lock()
	defer s.vpsDraftTestMu.Unlock()
	if s.vpsDraftTestTickets == nil {
		s.vpsDraftTestTickets = map[string]vpsDraftTestTicket{}
	}
	now := time.Now().UTC()
	for id, old := range s.vpsDraftTestTickets {
		if !old.ExpiresAt.After(now) {
			delete(s.vpsDraftTestTickets, id)
		}
	}
	s.vpsDraftTestTickets[ticket.ID] = ticket
}

func (s *Server) takeVPSDraftTestTicket(ticketID, phrase string) (vpsDraftTestTicket, error) {
	if s == nil {
		return vpsDraftTestTicket{}, fmt.Errorf("draft VPS test server is unavailable")
	}
	if strings.TrimSpace(phrase) != vpsDraftTestConfirmationPhrase {
		return vpsDraftTestTicket{}, fmt.Errorf("explicit confirmation phrase does not match; no write was started")
	}
	s.vpsDraftTestMu.Lock()
	defer s.vpsDraftTestMu.Unlock()
	ticket, found := s.vpsDraftTestTickets[strings.TrimSpace(ticketID)]
	if !found {
		return vpsDraftTestTicket{}, fmt.Errorf("draft VPS test ticket was not found or was already used; no write was started")
	}
	if !ticket.ExpiresAt.After(time.Now().UTC()) {
		delete(s.vpsDraftTestTickets, ticket.ID)
		return vpsDraftTestTicket{}, fmt.Errorf("draft VPS test ticket expired; no write was started")
	}
	// One-time consumption prevents a captured confirmation from being replayed.
	delete(s.vpsDraftTestTickets, ticket.ID)
	return ticket, nil
}

func (s *Server) runVPSDraftTest(ctx context.Context, ticket vpsDraftTestTicket) (report vpsDraftTestReport, err error) {
	if ticket.ObservationMode == "" {
		ticket.ObservationMode = vpsDraftTestObservationModeTimedRollback
	}
	changes := ticket.effectiveChanges()
	report = vpsDraftTestReport{
		SchemaVersion: vpsDraftTestSchemaVersion,
		RunID:         "vps_draft_run_" + randomID(),
		StartedAt:     time.Now().UTC(),
		Trust:         "observed local transport evidence over an agent-inferred or user-confirmed staging mapping; semantic behavior remains separate",
		TestScope:     "local host parameter write/readback plus full normalized-parameter rollback",
		DoesNotGrant: []string{
			"semantic mapping confirmation",
			"audio or response-shape behavior conformance",
			"acceptance of a parameter-surface mismatch",
			"Draft fingerprint update",
			"Provider Credential",
			"Catalog visibility",
			"SPAL dispatch",
		},
		ExplicitWriteConfirmed: true,
		SurfaceCompatibility:   vpsDraftTestSurfaceCompatibilityNotEvaluated,
		ObservationMode:        ticket.ObservationMode,
		ObservationState:       "running",
		ScenarioID:             ticket.ScenarioID,
		RequestedChanges:       changes,
		VPSID:                  ticket.VPSID,
		VPSRevision:            ticket.VPSRevision,
		Target:                 ticket.Target,
		RequestedNormalized:    ticket.RequestedNormalized,
		HoldMilliseconds:       int(ticket.Hold / time.Millisecond),
		WriteReadback:          vpsDraftTestWriteReadback{Receipts: []vpsDraftTestReceipt{}},
		Rollback:               vpsDraftTestRollback{Status: "not_run", Receipts: []vpsDraftTestReceipt{}},
		MappingDisposition:     "needs_review",
	}
	if ticket.ObservationMode == vpsDraftTestObservationModeManualWitness {
		report.TestScope = fmt.Sprintf("%d local host parameter write/readback operation(s) held for explicit GUI witness, followed by an explicit full normalized-parameter rollback", len(changes))
	}
	report.EvidenceID = "vps_draft_test:" + report.RunID
	var preimage vpsDraftTestSnapshot
	var observationSession vpsDraftTestObservationSession
	preimageReady := false
	autoRollback := true
	defer func() {
		if preimageReady && autoRollback {
			rollback, rollbackErr := s.restoreVPSDraftTestSnapshot(ctx, ticket, preimage)
			report.Rollback = rollback
			if rollbackErr != nil && err == nil {
				report.FailureCode = firstNonEmpty(report.FailureCode, "rollback_failed")
				err = fmt.Errorf("draft VPS test rollback failed: %w", rollbackErr)
			}
		}
		if !autoRollback && err == nil {
			return
		}
		vpsDraftTestFinalizeReport(&report, err)
		if ticket.ObservationMode == vpsDraftTestObservationModeManualWitness && observationSession.SessionID != "" {
			if report.Rollback.Passed {
				if removeErr := vpsDraftTestRemoveObservationSession(ticket.ReportRoot, observationSession.SessionID); removeErr != nil && err == nil {
					report.FailureCode = firstNonEmpty(report.FailureCode, "observation_recovery_record_remove_failed")
					err = fmt.Errorf("Draft test rollback passed but its active-observation recovery record could not be removed: %w", removeErr)
					vpsDraftTestFinalizeReport(&report, err)
				}
			} else {
				observationSession.State = vpsDraftTestObservationStateRollbackFailed
				observationSession.UpdatedAt = time.Now().UTC()
				observationSession.Report = report
				_ = vpsDraftTestPersistObservationSession(ticket.ReportRoot, observationSession)
			}
		}
	}()
	if len(changes) == 0 {
		report.FailureCode = "ticket_has_no_changes"
		return report, fmt.Errorf("Draft test ticket has no resolved changes")
	}
	if activeObservations, activeErr := vpsDraftTestListObservationSessions(ticket.ReportRoot); activeErr != nil {
		report.FailureCode = "active_observation_state_unavailable"
		return report, fmt.Errorf("read active Draft observation sessions: %w", activeErr)
	} else if len(activeObservations) > 0 {
		report.FailureCode = "active_observation_requires_rollback"
		return report, fmt.Errorf("a Draft GUI witness is still holding a live plugin state; no new Draft test write will be attempted until it is explicitly rolled back")
	}

	mappingKeys := make([]string, 0, len(changes))
	for _, change := range changes {
		mappingKeys = append(mappingKeys, change.MappingKey)
	}
	_, document, mappings, resolveErr := s.resolveVPSDraftTestTicketMappingKeys(ticket, mappingKeys)
	if resolveErr != nil {
		report.FailureCode = "draft_mapping_unavailable"
		return report, resolveErr
	}
	if document.Revision != ticket.VPSRevision || len(mappings) != len(changes) {
		report.FailureCode = "draft_changed_after_prepare"
		return report, fmt.Errorf("draft VPS or mapping changed after preparation; a new read-only preparation is required")
	}
	report.VPSStatus = document.Status
	report.Mappings = make([]vpsDraftTestMappingSummary, 0, len(mappings))
	for index, mapping := range mappings {
		change := changes[index]
		if mapping.ParameterID != change.ParameterID || vpsDraftTestMappingKey(mapping) != change.MappingKey {
			report.FailureCode = "draft_changed_after_prepare"
			return report, fmt.Errorf("Draft mapping %q changed after preparation; a new read-only preparation is required", change.MappingKey)
		}
		if reason := vpsDraftTestUnsupportedMappingReason(change.MappingKey); reason != "" {
			report.FailureCode = "unsupported_mapping"
			return report, fmt.Errorf("%s", reason)
		}
		report.Mappings = append(report.Mappings, vpsDraftTestMappingSummaryFor(mapping))
	}
	report.Mapping = report.Mappings[0]

	requestContext := vpsDraftTestRequestContext(ticket.VPSID, ticket.MappingKey, "execute")
	fresh, readErr := s.readVPSDraftTestParameterSurface(ctx, ticket.Target, requestContext)
	if readErr != nil {
		report.FailureCode = "fresh_read_failed"
		return report, readErr
	}
	digest := buildPluginParameterDigest(fresh)
	surfaceAcknowledgment := ""
	if ticket.SurfaceCompatibility.MismatchAcknowledged {
		surfaceAcknowledgment = vpsDraftTestSurfaceMismatchAcknowledgmentPhrase
	}
	surface, surfaceErr := vpsDraftTestValidateLiveSurface(document, digest, surfaceAcknowledgment)
	report.setSurfaceCompatibility(surface)
	if surfaceErr != nil && surface.Status != vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged {
		report.FailureCode = "target_surface_mismatch"
		return report, surfaceErr
	}
	if !strings.EqualFold(strings.TrimSpace(ticket.SurfaceCompatibility.ExpectedParameterSurface), strings.TrimSpace(surface.ExpectedParameterSurface)) {
		report.FailureCode = "draft_parameter_surface_changed_after_prepare"
		return report, fmt.Errorf("Draft parameter-surface fingerprint changed after preparation (prepared %q, current %q); no write will be attempted", ticket.SurfaceCompatibility.ExpectedParameterSurface, surface.ExpectedParameterSurface)
	}
	if !strings.EqualFold(strings.TrimSpace(ticket.SurfaceCompatibility.ObservedParameterSurface), strings.TrimSpace(surface.ObservedParameterSurface)) {
		report.FailureCode = "target_parameter_surface_changed_after_prepare"
		return report, fmt.Errorf("fresh host parameter-surface fingerprint changed after preparation (prepared %q, current %q); no write will be attempted", ticket.SurfaceCompatibility.ObservedParameterSurface, surface.ObservedParameterSurface)
	}
	if ticket.SurfaceCompatibility.Status != surface.Status || ticket.SurfaceCompatibility.MismatchAcknowledged != surface.MismatchAcknowledged {
		report.FailureCode = "target_surface_compatibility_changed_after_prepare"
		return report, fmt.Errorf("target surface compatibility changed after preparation (prepared %q, current %q); no write will be attempted", ticket.SurfaceCompatibility.Status, surface.Status)
	}
	if surfaceErr != nil {
		report.FailureCode = "target_surface_mismatch"
		return report, surfaceErr
	}
	preimage, err = vpsDraftTestSnapshotFromDigest(digest)
	if err != nil {
		report.FailureCode = "preimage_incomplete"
		return report, fmt.Errorf("complete pre-write parameter snapshot is unavailable: %w", err)
	}
	preimageReady = true
	report.Preimage = &preimage
	parameterIndex := vpsParameterIndex(digest)
	for _, change := range changes {
		parameter, found := parameterIndex[change.ParameterID]
		if !found {
			report.FailureCode = "mapped_parameter_missing"
			report.MappingDisposition = "rejected_for_this_vps_revision"
			return report, fmt.Errorf("mapped parameter %s for %s disappeared from the fresh host surface", change.ParameterID, change.MappingKey)
		}
		if !parameter.HostControllable {
			report.FailureCode = "mapped_parameter_not_host_controllable"
			report.MappingDisposition = "rejected_for_this_vps_revision"
			return report, fmt.Errorf("mapped parameter %s for %s is not host-controllable", change.ParameterID, change.MappingKey)
		}
		current, currentOK := vpsFiniteNumber(parameter.NormalizedValue)
		if !currentOK {
			report.FailureCode = "mapped_parameter_unreadable"
			return report, fmt.Errorf("mapped parameter %s for %s has no finite normalized readback", change.ParameterID, change.MappingKey)
		}
		if vpsNormalizedValuesMatch(current, change.RequestedNormalized) {
			report.FailureCode = "probe_value_noop"
			return report, fmt.Errorf("live parameter %s for %s changed after preparation and now equals its requested probe; prepare a new distinct scene", change.ParameterID, change.MappingKey)
		}
	}
	if ticket.ObservationMode == vpsDraftTestObservationModeManualWitness {
		now := time.Now().UTC()
		report.ObservationSessionID = "vps_draft_observation_" + randomID()
		report.ObservationState = vpsDraftTestObservationStatePreimagePersisted
		observationSession = vpsDraftTestObservationSession{
			SchemaVersion: vpsDraftTestSchemaVersion,
			SessionID:     report.ObservationSessionID,
			State:         report.ObservationState,
			CreatedAt:     now,
			UpdatedAt:     now,
			Ticket:        ticket,
			Report:        report,
		}
		if persistErr := vpsDraftTestPersistObservationSession(ticket.ReportRoot, observationSession); persistErr != nil {
			report.FailureCode = "observation_preimage_persist_failed"
			return report, fmt.Errorf("persist complete Draft GUI-witness preimage before write: %w", persistErr)
		}
	}
	for _, change := range changes {
		receipt, _, writeErr := s.writeAndReadVPSDraftTestParameter(ctx, ticket.Target, change.ParameterID, change.RequestedNormalized, "probe", requestContext)
		receipt.MappingKey = change.MappingKey
		report.WriteReadback.Receipts = append(report.WriteReadback.Receipts, receipt)
		if writeErr != nil {
			report.FailureCode = "write_readback_failed"
			return report, fmt.Errorf("scene change %s: %w", change.MappingKey, writeErr)
		}
	}
	report.WriteReadback.Passed = true
	if ticket.ObservationMode == vpsDraftTestObservationModeManualWitness {
		report.Status = vpsDraftTestObservationStateAwaiting
		report.MappingDisposition = "awaiting_gui_witness"
		report.Trust = "observed transport: host write/readback passed; live state is intentionally held pending explicit GUI-witness rollback"
		report.Rollback.Status = "awaiting_explicit_user_rollback"
		report.ObservationState = vpsDraftTestObservationStateAwaiting
		observationSession.State = vpsDraftTestObservationStateAwaiting
		observationSession.UpdatedAt = time.Now().UTC()
		observationSession.Report = report
		if persistErr := vpsDraftTestPersistObservationSession(ticket.ReportRoot, observationSession); persistErr != nil {
			report.FailureCode = "observation_state_persist_failed"
			return report, fmt.Errorf("persist Draft GUI-witness state after write/readback: %w", persistErr)
		}
		autoRollback = false
		return report, nil
	}
	if ticket.Hold > 0 {
		timer := time.NewTimer(ticket.Hold)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			report.FailureCode = "observation_hold_cancelled"
			return report, ctx.Err()
		case <-timer.C:
		}
	}
	return report, nil
}

func vpsDraftTestFinalizeReport(report *vpsDraftTestReport, runErr error) {
	if report == nil {
		return
	}
	report.CompletedAt = time.Now().UTC()
	if runErr != nil {
		report.Status = "failed"
		report.Error = runErr.Error()
		if report.MappingDisposition == "" {
			report.MappingDisposition = "needs_review"
		}
		if report.ObservationState == "" || report.ObservationState == "running" || report.ObservationState == vpsDraftTestObservationStateRollbackInProgress {
			report.ObservationState = "failed"
		}
		return
	}
	report.Status = "passed_transport_only"
	report.MappingDisposition = "retained_test_only"
	report.ObservationState = "completed"
	if report.SurfaceCompatibility == vpsDraftTestSurfaceCompatibilityMismatchAcknowledged {
		report.Trust = "observed local transport under an explicitly acknowledged Draft-only parameter-surface mismatch; semantic and behavior status remain unknown"
		return
	}
	report.Trust = "observed local transport: host write/readback and full normalized-parameter rollback; semantic and behavior status remain unknown"
}

func (s *Server) completeVPSDraftTestManualObservation(ctx context.Context, session vpsDraftTestObservationSession) (report vpsDraftTestReport, err error) {
	report = session.Report
	ticket := session.Ticket
	if ticket.ObservationMode != vpsDraftTestObservationModeManualWitness {
		return report, fmt.Errorf("active Draft observation session does not use manual_witness mode")
	}
	if report.Preimage == nil {
		return report, fmt.Errorf("active Draft observation session has no complete rollback preimage")
	}
	report.ObservationState = vpsDraftTestObservationStateRollbackInProgress
	session.State = vpsDraftTestObservationStateRollbackInProgress
	session.UpdatedAt = time.Now().UTC()
	session.Report = report
	if persistErr := vpsDraftTestPersistObservationSession(ticket.ReportRoot, session); persistErr != nil {
		report.FailureCode = "observation_rollback_state_persist_failed"
		return report, fmt.Errorf("persist Draft GUI-witness rollback intent before writing: %w", persistErr)
	}
	changes := ticket.effectiveChanges()
	if len(changes) == 0 {
		report.FailureCode = "ticket_has_no_changes"
		return report, fmt.Errorf("active Draft GUI-witness session has no resolved changes")
	}
	mappingKeys := make([]string, 0, len(changes))
	for _, change := range changes {
		mappingKeys = append(mappingKeys, change.MappingKey)
	}
	_, document, mappings, resolveErr := s.resolveVPSDraftTestTicketMappingKeys(ticket, mappingKeys)
	if resolveErr != nil {
		report.FailureCode = "draft_mapping_unavailable"
		return report, resolveErr
	}
	if document.Revision != ticket.VPSRevision || len(mappings) != len(changes) {
		report.FailureCode = "draft_changed_after_observation"
		return report, fmt.Errorf("Draft VPS or mapping changed while the GUI witness was active; no rollback write will be attempted")
	}
	report.VPSStatus = document.Status
	report.Mappings = make([]vpsDraftTestMappingSummary, 0, len(mappings))
	for index, mapping := range mappings {
		change := changes[index]
		if mapping.ParameterID != change.ParameterID || vpsDraftTestMappingKey(mapping) != change.MappingKey {
			report.FailureCode = "draft_changed_after_observation"
			return report, fmt.Errorf("Draft mapping %q changed while the GUI witness was active; no rollback write will be attempted", change.MappingKey)
		}
		report.Mappings = append(report.Mappings, vpsDraftTestMappingSummaryFor(mapping))
	}
	report.Mapping = report.Mappings[0]
	requestContext := vpsDraftTestRequestContext(ticket.VPSID, ticket.MappingKey, "manual_rollback_validate")
	fresh, readErr := s.readVPSDraftTestParameterSurface(ctx, ticket.Target, requestContext)
	if readErr != nil {
		report.FailureCode = "rollback_fresh_read_failed"
		return report, readErr
	}
	digest := buildPluginParameterDigest(fresh)
	surfaceAcknowledgment := ""
	if ticket.SurfaceCompatibility.MismatchAcknowledged {
		surfaceAcknowledgment = vpsDraftTestSurfaceMismatchAcknowledgmentPhrase
	}
	surface, surfaceErr := vpsDraftTestValidateLiveSurface(document, digest, surfaceAcknowledgment)
	report.setSurfaceCompatibility(surface)
	if surfaceErr != nil && surface.Status != vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged {
		report.FailureCode = "rollback_target_surface_mismatch"
		return report, surfaceErr
	}
	if !strings.EqualFold(strings.TrimSpace(ticket.SurfaceCompatibility.ExpectedParameterSurface), strings.TrimSpace(surface.ExpectedParameterSurface)) {
		report.FailureCode = "draft_parameter_surface_changed_after_observation"
		return report, fmt.Errorf("Draft parameter-surface fingerprint changed while the GUI witness was active (prepared %q, current %q); no rollback write will be attempted", ticket.SurfaceCompatibility.ExpectedParameterSurface, surface.ExpectedParameterSurface)
	}
	if !strings.EqualFold(strings.TrimSpace(ticket.SurfaceCompatibility.ObservedParameterSurface), strings.TrimSpace(surface.ObservedParameterSurface)) {
		report.FailureCode = "target_parameter_surface_changed_after_observation"
		return report, fmt.Errorf("fresh host parameter-surface fingerprint changed while the GUI witness was active (prepared %q, current %q); no rollback write will be attempted", ticket.SurfaceCompatibility.ObservedParameterSurface, surface.ObservedParameterSurface)
	}
	if ticket.SurfaceCompatibility.Status != surface.Status || ticket.SurfaceCompatibility.MismatchAcknowledged != surface.MismatchAcknowledged {
		report.FailureCode = "target_surface_compatibility_changed_after_observation"
		return report, fmt.Errorf("target surface compatibility changed while the GUI witness was active (prepared %q, current %q); no rollback write will be attempted", ticket.SurfaceCompatibility.Status, surface.Status)
	}
	if surfaceErr != nil {
		report.FailureCode = "rollback_target_surface_mismatch"
		return report, surfaceErr
	}
	rollback, rollbackErr := s.restoreVPSDraftTestSnapshot(ctx, ticket, *report.Preimage)
	report.Rollback = rollback
	if rollbackErr != nil {
		report.FailureCode = "rollback_failed"
		report.ObservationState = vpsDraftTestObservationStateRollbackFailed
		report.Status = vpsDraftTestObservationStateRollbackFailed
		report.Error = rollbackErr.Error()
		return report, fmt.Errorf("Draft GUI-witness rollback failed: %w", rollbackErr)
	}
	vpsDraftTestFinalizeReport(&report, nil)
	return report, nil
}

func (s *Server) readVPSDraftTestParameterSurface(ctx context.Context, target vpsDraftTestTarget, requestContext map[string]any) (map[string]any, error) {
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "plugin.get_parameters",
		Args: map[string]any{
			"track_id":               target.TrackID,
			"plugin_id":              target.PluginID,
			"include_vps_v3_surface": true,
		},
		Context: requestContext,
		Source:  vpsDraftTestExecutorSource,
	})
	if err != nil {
		return nil, fmt.Errorf("fresh plugin parameter readback failed: %w", err)
	}
	if response.Status != "ok" || !kernelReplyOK(response.Result) {
		return nil, fmt.Errorf("fresh plugin parameter readback failed: %s", firstNonEmptyText(response.Result, "message", "error"))
	}
	if len(mapRowsValue(response.Result["parameters"])) == 0 {
		return nil, fmt.Errorf("fresh plugin parameter readback omitted the complete VPS parameter surface")
	}
	return response.Result, nil
}

func (s *Server) writeAndReadVPSDraftTestParameter(ctx context.Context, target vpsDraftTestTarget, parameterID string, wanted float64, stage string, requestContext map[string]any) (vpsDraftTestReceipt, pluginParameterDigest, error) {
	receipt := vpsDraftTestReceipt{Stage: stage, ParameterID: parameterID, RequestedNormalized: vpsDraftTestFloatPointer(wanted), Status: "failed"}
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "plugin.set_parameter",
		Args: map[string]any{
			"track_id":         target.TrackID,
			"plugin_id":        target.PluginID,
			"param_id":         parameterID,
			"normalized_value": wanted,
		},
		Context:   requestContext,
		Source:    vpsDraftTestExecutorSource,
		Confirmed: true, // guarded by a one-time explicit HTTP confirmation ticket
	})
	if err != nil {
		receipt.Error = err.Error()
		return receipt, pluginParameterDigest{}, fmt.Errorf("%s write %s: %w", stage, parameterID, err)
	}
	if response.Status != "ok" || !kernelReplyOK(response.Result) {
		receipt.Error = firstNonEmptyText(response.Result, "message", "error")
		return receipt, pluginParameterDigest{}, fmt.Errorf("%s write %s failed: %s", stage, parameterID, receipt.Error)
	}
	fresh, err := s.readVPSDraftTestParameterSurface(ctx, target, requestContext)
	if err != nil {
		receipt.Error = err.Error()
		return receipt, pluginParameterDigest{}, fmt.Errorf("%s readback %s: %w", stage, parameterID, err)
	}
	digest := buildPluginParameterDigest(fresh)
	parameter, found := vpsParameterIndex(digest)[parameterID]
	if !found {
		receipt.Error = "parameter omitted from fresh readback"
		return receipt, digest, fmt.Errorf("%s readback omitted %s", stage, parameterID)
	}
	actual, finite := vpsFiniteNumber(parameter.NormalizedValue)
	if finite {
		receipt.ReadbackNormalized = vpsDraftTestFloatPointer(actual)
	}
	receipt.ReadbackValueText = parameter.ValueText
	if !finite || !vpsNormalizedValuesMatch(actual, wanted) {
		receipt.Error = fmt.Sprintf("normalized readback %.6f did not match requested %.6f", actual, wanted)
		return receipt, digest, fmt.Errorf("%s readback for %s: %s", stage, parameterID, receipt.Error)
	}
	receipt.Status = "passed"
	return receipt, digest, nil
}

func (s *Server) restoreVPSDraftTestSnapshot(ctx context.Context, ticket vpsDraftTestTicket, preimage vpsDraftTestSnapshot) (result vpsDraftTestRollback, err error) {
	result = vpsDraftTestRollback{Status: "running", Receipts: []vpsDraftTestReceipt{}}
	requestContext := vpsDraftTestRequestContext(ticket.VPSID, ticket.MappingKey, "rollback")
	changedSet := map[string]bool{}
	for round := 1; round <= 3; round++ {
		result.Rounds = round
		fresh, readErr := s.readVPSDraftTestParameterSurface(ctx, ticket.Target, requestContext)
		if readErr != nil {
			result.Status, result.Error = "failed", readErr.Error()
			return result, readErr
		}
		digest := buildPluginParameterDigest(fresh)
		current, snapshotErr := vpsDraftTestSnapshotFromDigest(digest)
		if snapshotErr != nil {
			result.Status, result.Error = "failed", snapshotErr.Error()
			return result, snapshotErr
		}
		changes, diffErr := vpsDraftTestSnapshotChanges(preimage, current)
		if diffErr != nil {
			result.Status, result.Error = "failed", diffErr.Error()
			return result, diffErr
		}
		if len(changes) == 0 {
			result.Status, result.Passed, result.FinalSnapshot = "passed", true, &current
			return result, nil
		}
		ids := make([]string, 0, len(changes))
		for id := range changes {
			changedSet[id] = true
			ids = append(ids, id)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(ids)))
		index := vpsParameterIndex(digest)
		for _, id := range ids {
			parameter, found := index[id]
			if !found || !parameter.HostControllable {
				err = fmt.Errorf("rollback cannot restore changed parameter %s because it is absent or not host-controllable", id)
				result.Status, result.Error = "failed", err.Error()
				result.ChangedParameterIDs = vpsDraftTestSortedKeys(changedSet)
				return result, err
			}
			receipt, _, writeErr := s.writeAndReadVPSDraftTestParameter(ctx, ticket.Target, id, changes[id], "rollback", requestContext)
			result.Receipts = append(result.Receipts, receipt)
			if writeErr != nil {
				result.Status, result.Error = "failed", writeErr.Error()
				result.ChangedParameterIDs = vpsDraftTestSortedKeys(changedSet)
				return result, writeErr
			}
		}
	}
	result.Status = "failed"
	result.ChangedParameterIDs = vpsDraftTestSortedKeys(changedSet)
	err = fmt.Errorf("rollback did not restore the complete parameter preimage after three rounds")
	result.Error = err.Error()
	return result, err
}

func vpsDraftTestSnapshotFromDigest(digest pluginParameterDigest) (vpsDraftTestSnapshot, error) {
	if len(digest.Parameters) == 0 {
		return vpsDraftTestSnapshot{}, fmt.Errorf("fresh surface has no parameters")
	}
	values := make(map[string]float64, len(digest.Parameters))
	for _, parameter := range digest.Parameters {
		id := strings.TrimSpace(parameter.ID)
		if id == "" {
			return vpsDraftTestSnapshot{}, fmt.Errorf("parameter surface contains an empty parameter id")
		}
		if _, duplicate := values[id]; duplicate {
			return vpsDraftTestSnapshot{}, fmt.Errorf("parameter surface contains duplicate parameter id %s", id)
		}
		value, finite := vpsFiniteNumber(parameter.NormalizedValue)
		if !finite {
			return vpsDraftTestSnapshot{}, fmt.Errorf("parameter %s has no finite normalized preimage", id)
		}
		values[id] = value
	}
	return vpsDraftTestSnapshot{ParameterCount: len(values), NormalizedValues: values, SurfaceValueSHA256: vpsDraftTestSnapshotHash(values)}, nil
}

func vpsDraftTestSnapshotHash(values map[string]float64) string {
	type row struct {
		ID    string  `json:"id"`
		Value float64 `json:"normalized_value"`
	}
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows := make([]row, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, row{ID: id, Value: values[id]})
	}
	payload, _ := json.Marshal(rows)
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func vpsDraftTestSnapshotChanges(expected, actual vpsDraftTestSnapshot) (map[string]float64, error) {
	if expected.ParameterCount != len(expected.NormalizedValues) || actual.ParameterCount != len(actual.NormalizedValues) {
		return nil, fmt.Errorf("invalid parameter snapshot count")
	}
	changes := map[string]float64{}
	for id, wanted := range expected.NormalizedValues {
		value, found := actual.NormalizedValues[id]
		if !found {
			return nil, fmt.Errorf("rollback surface no longer contains preimage parameter %s", id)
		}
		if !vpsNormalizedValuesMatch(value, wanted) {
			changes[id] = wanted
		}
	}
	for id := range actual.NormalizedValues {
		if _, known := expected.NormalizedValues[id]; !known {
			return nil, fmt.Errorf("rollback surface gained parameter %s after the preimage", id)
		}
	}
	return changes, nil
}

func (report *vpsDraftTestReport) setSurfaceCompatibility(surface vpsDraftTestSurfaceCompatibility) {
	if report == nil {
		return
	}
	report.SurfaceCompatibility = surface.Status
	report.ExpectedParameterSurface = surface.ExpectedParameterSurface
	report.ObservedParameterSurface = surface.ObservedParameterSurface
	report.SurfaceMismatchAcknowledged = surface.MismatchAcknowledged
}

func vpsDraftTestValidateLiveSurface(document vps.VPSDocument, digest pluginParameterDigest, surfaceMismatchAcknowledgment string) (vpsDraftTestSurfaceCompatibility, error) {
	compatibility := vpsDraftTestSurfaceCompatibility{Status: vpsDraftTestSurfaceCompatibilityNotEvaluated}
	identity := digest.PluginIdentity
	checks := []struct {
		field string
		want  string
		got   string
	}{
		{"manufacturer", document.PluginIdentity.Manufacturer, firstNonEmptyText(identity, "manufacturer", "maker", "vendor")},
		{"name", document.PluginIdentity.Name, firstNonEmpty(digest.PluginName, firstNonEmptyText(identity, "plugin_name", "name"))},
		{"format", document.PluginIdentity.Format, firstNonEmptyText(identity, "plugin_format", "format")},
		{"version", document.PluginIdentity.Version, firstNonEmptyText(identity, "version", "plugin_version")},
		{"profile_key", document.PluginIdentity.ProfileKey, firstNonEmptyText(identity, "profile_key")},
	}
	for _, check := range checks {
		if strings.TrimSpace(check.want) == "" {
			continue
		}
		if strings.TrimSpace(check.got) == "" || !strings.EqualFold(strings.TrimSpace(check.want), strings.TrimSpace(check.got)) {
			compatibility.Status = vpsDraftTestSurfaceCompatibilityIdentityMismatch
			return compatibility, fmt.Errorf("fresh plugin %s does not match Draft VPS identity (want %q, got %q); no write will be attempted", check.field, check.want, check.got)
		}
	}
	// A Draft may carry a host-specific observed projection guard.  The
	// independent Adapter surface remains the plug-in identity archive, while
	// the Vit projection is the only complete surface this local executor can
	// fresh-read.  Never acknowledge a mismatch merely because the two hosts
	// enumerate different numbers of controls.
	expected := strings.TrimSpace(document.PluginIdentity.Fingerprint.ParameterSurface)
	if binding, found := document.HostProjectionBindingFor(vps.VitHostProjectionID); found {
		expected = strings.TrimSpace(binding.Fingerprint.ParameterSurface)
	}
	compatibility.ExpectedParameterSurface = expected
	if expected == "" {
		compatibility.Status = vpsDraftTestSurfaceCompatibilityDraftFingerprintUnknown
		return compatibility, nil
	}
	fingerprint, err := vps.BuildPluginFingerprintFromDigest("draft-test", digest)
	if err != nil {
		compatibility.Status = vpsDraftTestSurfaceCompatibilityObservedSurfaceUnavailable
		return compatibility, fmt.Errorf("current parameter surface cannot be fingerprinted for the Draft guard: %w", err)
	}
	compatibility.ObservedParameterSurface = strings.TrimSpace(fingerprint.ParameterSurface)
	if strings.EqualFold(expected, compatibility.ObservedParameterSurface) {
		compatibility.Status = vpsDraftTestSurfaceCompatibilityStrictMatch
		return compatibility, nil
	}
	if surfaceMismatchAcknowledgment != vpsDraftTestSurfaceMismatchAcknowledgmentPhrase {
		compatibility.Status = vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged
		return compatibility, fmt.Errorf("current parameter-surface fingerprint differs from the Draft observation (expected %q, observed %q); no write will be attempted without the exact Draft-only mismatch acknowledgment", expected, compatibility.ObservedParameterSurface)
	}
	compatibility.Status = vpsDraftTestSurfaceCompatibilityMismatchAcknowledged
	compatibility.MismatchAcknowledged = true
	return compatibility, nil
}

func vpsDraftTestRequestContext(vpsID, mappingKey, stage string) map[string]any {
	return map[string]any{
		"user_message":            "Explicit user-authorized Draft VPS local mapping test with mandatory rollback",
		"vps_draft_test_executor": true,
		"vps_id":                  vpsID,
		"mapping_key":             mappingKey,
		"stage":                   stage,
	}
}

func vpsDraftTestNormalizedValueValid(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func vpsDraftTestNormalizeObservationMode(value string, holdMilliseconds int) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(value))
	switch mode {
	case "", vpsDraftTestObservationModeTimedRollback:
		return vpsDraftTestObservationModeTimedRollback, nil
	case vpsDraftTestObservationModeManualWitness:
		if holdMilliseconds != 0 {
			return "", fmt.Errorf("manual_witness requires hold_milliseconds to be 0 because rollback waits for a separate explicit GUI-witness confirmation")
		}
		return vpsDraftTestObservationModeManualWitness, nil
	default:
		return "", fmt.Errorf("observation_mode must be %q or %q", vpsDraftTestObservationModeTimedRollback, vpsDraftTestObservationModeManualWitness)
	}
}

func vpsDraftTestValidateSceneComposition(changes []vpsDraftTestRequestedChange) error {
	if len(changes) < 2 {
		return nil
	}
	for _, change := range changes {
		if strings.TrimSpace(change.MappingKey) == "io.bypass" {
			return fmt.Errorf("io.bypass must be tested in a separate scene because it masks the audible and visual effect of the other requested changes")
		}
	}
	return nil
}

func vpsDraftTestFloatPointer(value float64) *float64 {
	copy := value
	return &copy
}

func vpsDraftTestSortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func vpsDraftTestReportRoot(libraryPath string) string {
	if configured := strings.TrimSpace(os.Getenv(vpsDraftTestReportsDirectoryEnv)); configured != "" {
		return configured
	}
	return filepath.Join(filepath.Dir(libraryPath), "vps_draft_test_reports")
}

func vpsDraftTestObservationSessionsRoot(reportRoot string) string {
	return filepath.Join(reportRoot, vpsDraftTestActiveObservationsDirectory)
}

func vpsDraftTestObservationSessionPath(reportRoot, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if !strings.HasPrefix(sessionID, "vps_draft_observation_") || strings.ContainsAny(sessionID, `\\/:`) {
		return "", fmt.Errorf("invalid Draft GUI-witness observation session id")
	}
	for _, character := range sessionID {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return "", fmt.Errorf("invalid Draft GUI-witness observation session id")
	}
	return filepath.Join(vpsDraftTestObservationSessionsRoot(reportRoot), sessionID+".json"), nil
}

func vpsDraftTestPersistObservationSession(reportRoot string, session vpsDraftTestObservationSession) error {
	if err := vpsDraftTestEnsureReportRoot(reportRoot); err != nil {
		return err
	}
	if strings.TrimSpace(session.SchemaVersion) == "" {
		session.SchemaVersion = vpsDraftTestSchemaVersion
	}
	if session.SchemaVersion != vpsDraftTestSchemaVersion {
		return fmt.Errorf("unsupported Draft GUI-witness observation schema %q", session.SchemaVersion)
	}
	if strings.TrimSpace(session.State) == "" || session.Report.Preimage == nil {
		return fmt.Errorf("Draft GUI-witness observation requires state and complete preimage")
	}
	if err := session.Ticket.Target.valid(); err != nil {
		return err
	}
	path, err := vpsDraftTestObservationSessionPath(reportRoot, session.SessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func vpsDraftTestLoadObservationSession(reportRoot, sessionID string) (vpsDraftTestObservationSession, error) {
	path, err := vpsDraftTestObservationSessionPath(reportRoot, sessionID)
	if err != nil {
		return vpsDraftTestObservationSession{}, err
	}
	payload, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return vpsDraftTestObservationSession{}, fmt.Errorf("Draft GUI-witness observation session %q was not found", strings.TrimSpace(sessionID))
	}
	if err != nil {
		return vpsDraftTestObservationSession{}, err
	}
	var session vpsDraftTestObservationSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return vpsDraftTestObservationSession{}, fmt.Errorf("decode Draft GUI-witness observation session: %w", err)
	}
	if session.SchemaVersion != vpsDraftTestSchemaVersion || session.SessionID != strings.TrimSpace(sessionID) || strings.TrimSpace(session.State) == "" || session.Report.Preimage == nil {
		return vpsDraftTestObservationSession{}, fmt.Errorf("Draft GUI-witness observation session is incomplete or incompatible")
	}
	if err := session.Ticket.Target.valid(); err != nil {
		return vpsDraftTestObservationSession{}, err
	}
	// The persisted copy is recovery evidence, not authority to redirect a
	// rollback to a different directory after a process restart.
	session.Ticket.ReportRoot = reportRoot
	return session, nil
}

func vpsDraftTestListObservationSessions(reportRoot string) ([]vpsDraftTestObservationSession, error) {
	directory := vpsDraftTestObservationSessionsRoot(reportRoot)
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []vpsDraftTestObservationSession{}, nil
	}
	if err != nil {
		return nil, err
	}
	sessions := make([]vpsDraftTestObservationSession, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		sessionID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		session, loadErr := vpsDraftTestLoadObservationSession(reportRoot, sessionID)
		if loadErr != nil {
			return nil, loadErr
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func vpsDraftTestRemoveObservationSession(reportRoot, sessionID string) error {
	path, err := vpsDraftTestObservationSessionPath(reportRoot, sessionID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func vpsDraftTestEnsureReportRoot(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("draft-test report root is empty")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	probe, err := os.CreateTemp(root, ".draft_test_write_probe_*")
	if err != nil {
		return err
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func vpsDraftTestPersistReport(root string, report vpsDraftTestReport) (string, error) {
	if err := vpsDraftTestEnsureReportRoot(root); err != nil {
		return "", err
	}
	fileName := report.RunID + ".json"
	path := filepath.Join(root, fileName)
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return "", err
	}
	trust := "observed"
	evidence := map[string]any{
		"schema_version":                vpsDraftTestSchemaVersion,
		"evidence_id":                   report.EvidenceID,
		"recorded_at":                   report.CompletedAt,
		"trust":                         trust,
		"scope":                         "host parameter transport write/readback and rollback only",
		"does_not_grant":                report.DoesNotGrant,
		"vps_id":                        report.VPSID,
		"vps_revision":                  report.VPSRevision,
		"mapping_key":                   report.Mapping.Key,
		"parameter_id":                  report.Mapping.ParameterID,
		"status":                        report.Status,
		"failure_code":                  report.FailureCode,
		"rollback_passed":               report.Rollback.Passed,
		"surface_compatibility":         report.SurfaceCompatibility,
		"expected_parameter_surface":    report.ExpectedParameterSurface,
		"observed_parameter_surface":    report.ObservedParameterSurface,
		"surface_mismatch_acknowledged": report.SurfaceMismatchAcknowledged,
		"observation_mode":              report.ObservationMode,
		"observation_session_id":        report.ObservationSessionID,
		"observation_state":             report.ObservationState,
		"scenario_id":                   report.ScenarioID,
		"requested_changes":             report.RequestedChanges,
		"mapping_keys": func() []string {
			keys := make([]string, 0, len(report.Mappings))
			for _, mapping := range report.Mappings {
				keys = append(keys, mapping.Key)
			}
			return keys
		}(),
		"report_file": fileName,
	}
	if err := vpsDraftTestAppendJSONL(filepath.Join(root, "evidence-ledger.jsonl"), evidence); err != nil {
		return path, err
	}
	if report.MappingDisposition == "rejected_for_this_vps_revision" {
		disposition := vpsDraftTestDispositionRecord{
			SchemaVersion: vpsDraftTestSchemaVersion,
			RecordedAt:    report.CompletedAt,
			VPSID:         report.VPSID,
			VPSRevision:   report.VPSRevision,
			MappingKey:    report.Mapping.Key,
			Disposition:   report.MappingDisposition,
			FailureCode:   report.FailureCode,
			EvidenceID:    report.EvidenceID,
			ReportFile:    fileName,
		}
		if err := vpsDraftTestAppendJSONL(filepath.Join(root, "mapping-dispositions.jsonl"), disposition); err != nil {
			return path, err
		}
	}
	return path, nil
}

func vpsDraftTestAppendJSONL(path string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(payload, '\n')); err != nil {
		return err
	}
	return nil
}

func vpsDraftTestMappingRejected(root, vpsID string, revision int, mappingKey string) (bool, error) {
	payload, err := os.ReadFile(filepath.Join(root, "mapping-dispositions.jsonl"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(payload), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record vpsDraftTestDispositionRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return false, fmt.Errorf("invalid disposition ledger row: %w", err)
		}
		if record.VPSID == vpsID && record.VPSRevision == revision && record.MappingKey == mappingKey && record.Disposition == "rejected_for_this_vps_revision" {
			return true, nil
		}
	}
	return false, nil
}
