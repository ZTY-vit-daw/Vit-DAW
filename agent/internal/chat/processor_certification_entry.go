package chat

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorauthority"
)

const processorCertificationConsent = "temporary_track_apply_readback_restore"

var processorCertificationFamilies = []string{
	processorattestation.FamilyStaticEQ,
	processorattestation.FamilyBroadbandCompressor,
	processorattestation.FamilyLimiter,
	processorattestation.FamilyGateExpander,
	processorattestation.FamilyDeEsser,
	processorattestation.FamilyTransient,
	processorattestation.FamilyMultiband,
	"spectral_dynamics",
	"clipper",
}

type processorCapabilityStatus struct {
	Family           string   `json:"family"`
	Version          string   `json:"version"`
	Status           string   `json:"status"`
	Reason           string   `json:"reason"`
	Certifiable      bool     `json:"certifiable"`
	FingerprintMatch bool     `json:"fingerprint_match"`
	CoverageAxes     []string `json:"coverage_axes,omitempty"`
	CoverageLevel    string   `json:"coverage_level,omitempty"`
	AttestationID    string   `json:"attestation_id,omitempty"`
}

type processorCertificationCandidate struct {
	ID               string                      `json:"id"`
	Name             string                      `json:"name"`
	Manufacturer     string                      `json:"manufacturer,omitempty"`
	Format           string                      `json:"format,omitempty"`
	Identifier       string                      `json:"identifier"`
	PluginPath       string                      `json:"plugin_path"`
	Family           string                      `json:"family"`
	Status           string                      `json:"status"`
	Reason           string                      `json:"reason"`
	Certifiable      bool                        `json:"certifiable"`
	FingerprintMatch bool                        `json:"fingerprint_match"`
	CoverageAxes     []string                    `json:"coverage_axes,omitempty"`
	CoverageLevel    string                      `json:"coverage_level,omitempty"`
	AttestationID    string                      `json:"attestation_id,omitempty"`
	Capabilities     []processorCapabilityStatus `json:"capabilities,omitempty"`
}

type processorCertificationStartRequest struct {
	Identifier string `json:"identifier"`
	Family     string `json:"family"`
	Confirmed  bool   `json:"confirmed"`
	Consent    string `json:"consent"`
}

type processorCertificationJob struct {
	JobID         string                                      `json:"job_id"`
	Identifier    string                                      `json:"identifier"`
	PluginName    string                                      `json:"plugin_name"`
	Family        string                                      `json:"family"`
	Status        string                                      `json:"status"`
	Stage         string                                      `json:"stage"`
	Progress      int                                         `json:"progress"`
	CreatedAt     time.Time                                   `json:"created_at"`
	StartedAt     *time.Time                                  `json:"started_at,omitempty"`
	CompletedAt   *time.Time                                  `json:"completed_at,omitempty"`
	OutputDir     string                                      `json:"output_dir,omitempty"`
	SummaryPath   string                                      `json:"summary_path,omitempty"`
	Error         string                                      `json:"error,omitempty"`
	Certification processorauthority.LocalCertificationReport `json:"certification,omitempty"`
	Import        processorauthority.ImportReportV2           `json:"import,omitempty"`
	ImportV1      processorauthority.ImportReport             `json:"import_v1,omitempty"`
}

type processorCertificationLoadAuthorization struct {
	JobID             string
	Identifier        string
	PluginPath        string
	ProcessorFamily   string
	SubjectKey        string
	BinaryFingerprint string
	ExpiresAt         time.Time
}

func (s *Server) consumeProcessorCertificationLoadAuthorization(req harness.InvokeRequest) (harness.InvokeRequest, error) {
	token := strings.TrimSpace(req.AuthorizationToken)
	s.processorCertificationMu.Lock()
	auth, ok := s.processorCertificationLoadTokens[token]
	if ok {
		delete(s.processorCertificationLoadTokens, token)
	}
	s.processorCertificationMu.Unlock()
	if !ok || token == "" {
		return req, fmt.Errorf("pca_load_gate: certification authorization token is invalid or already consumed")
	}
	if time.Now().UTC().After(auth.ExpiresAt) {
		return req, fmt.Errorf("pca_load_gate: certification authorization token expired")
	}
	wantSource := "pcactl.certify_processor"
	if auth.ProcessorFamily == processorattestation.FamilyBroadbandCompressor {
		wantSource = "pcactl.certify_compressor"
	}
	if strings.TrimSpace(req.Source) != wantSource || !req.Confirmed || !strings.EqualFold(strings.TrimSpace(req.Tool), "plugin.load_to_rack") {
		return req, fmt.Errorf("pca_load_gate: certification authorization scope mismatch")
	}
	trackID := firstStringFromMap(req.Args, "track_id")
	path := firstStringFromMap(req.Args, "plugin_path", "path")
	identifier := firstStringFromMap(req.Args, "plugin_identifier", "identifier")
	if trackID == "" || !strings.EqualFold(path, auth.PluginPath) || !strings.EqualFold(identifier, auth.Identifier) {
		return req, fmt.Errorf("pca_load_gate: certification exact load identity mismatch")
	}
	currentFingerprint, err := processorattestation.FingerprintPath(path)
	if err != nil || !strings.EqualFold(currentFingerprint, auth.BinaryFingerprint) {
		return req, fmt.Errorf("pca_load_gate: certification binary changed before load")
	}
	req.Context = harness.AuthorizeProcessorCertificationLoad(req.Context, trackID, auth.PluginPath, auth.Identifier, auth.ProcessorFamily, auth.SubjectKey, auth.BinaryFingerprint)
	req.AuthorizationToken = ""
	return req, nil
}

func (s *Server) handleProcessorCertificationCandidates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	family := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("family")))
	if family == "" {
		family = "all"
	}
	if family != "all" && !processorCertificationFamilyKnown(family) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "unsupported PCA v2 family"})
		return
	}
	indexPath, err := pluginsemantics.DefaultPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	index, err := pluginsemantics.Load(indexPath)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "semantic library unavailable: " + err.Error()})
		return
	}
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	library, readReport, err := store.Read()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	v1Store, _ := processorattestation.NewStore("")
	v1Library := processorattestation.Library{}
	if v1Store != nil {
		v1Library, _, _ = v1Store.Read()
	}
	var candidates []processorCertificationCandidate
	switch {
	case family == "all":
		candidates = buildAllProcessorCertificationCandidates(index, v1Library, library)
	case processorattestation.IsV2Family(family):
		candidates = buildProcessorCertificationCandidates(index, library, family)
	default:
		candidates = buildCapabilityFilteredCandidates(index, v1Library, library, family)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "schema_version": "processor_certification_entry.v2", "family": family,
		"families": processorCertificationFamilies, "candidates": candidates, "candidate_count": len(candidates),
		"semantic_index_path": indexPath, "pca_store_path": store.Path, "store_read": readReport,
		"consent_token": processorCertificationConsent,
	})
}

func buildCapabilityFilteredCandidates(index pluginsemantics.Index, v1 processorattestation.Library, v2 processorattestation.LibraryV2, family string) []processorCertificationCandidate {
	all := buildAllProcessorCertificationCandidates(index, v1, v2)
	result := make([]processorCertificationCandidate, 0, len(all))
	for _, candidate := range all {
		for _, capability := range candidate.Capabilities {
			if capability.Family != family {
				continue
			}
			candidate.Family = family
			candidate.Status = capability.Status
			candidate.Reason = capability.Reason
			candidate.Certifiable = capability.Certifiable
			candidate.FingerprintMatch = capability.FingerprintMatch
			candidate.CoverageAxes = capability.CoverageAxes
			candidate.CoverageLevel = capability.CoverageLevel
			candidate.AttestationID = capability.AttestationID
			candidate.Capabilities = nil
			result = append(result, candidate)
			break
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		ri, rj := certificationStatusRank(result[i].Status), certificationStatusRank(result[j].Status)
		if ri != rj {
			return ri < rj
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func processorCertificationFamilyKnown(family string) bool {
	for _, candidate := range processorCertificationFamilies {
		if candidate == family {
			return true
		}
	}
	return false
}

func buildAllProcessorCertificationCandidates(index pluginsemantics.Index, v1 processorattestation.Library, v2 processorattestation.LibraryV2) []processorCertificationCandidate {
	fingerprintCache := map[string]string{}
	result := make([]processorCertificationCandidate, 0, len(index.Entries))
	for _, entry := range index.Entries {
		if entry.IsInstrument || strings.TrimSpace(entry.Identifier) == "" || strings.TrimSpace(entry.PluginPath) == "" {
			continue
		}
		candidate := processorCertificationCandidate{ID: entry.ID, Name: firstNonEmpty(entry.Name, entry.DescriptiveName, filepath.Base(entry.PluginPath)), Manufacturer: entry.Manufacturer, Format: entry.Format, Identifier: entry.Identifier, PluginPath: entry.PluginPath, Family: "all", Status: "unattested", Reason: "no_observed_capability", Certifiable: pathExists(entry.PluginPath)}
		subject := processorattestation.Subject{Name: entry.Name, Manufacturer: entry.Manufacturer, Format: entry.Format, Identifier: entry.Identifier, InstalledPath: entry.PluginPath}
		subjectKey, err := processorattestation.BuildSubjectKey(subject)
		if err != nil {
			candidate.Status, candidate.Reason, candidate.Certifiable = "unresolved", "subject_identity_invalid", false
			result = append(result, candidate)
			continue
		}
		fingerprint := ""
		if candidate.Certifiable {
			fingerprint = fingerprintCache[entry.PluginPath]
			if fingerprint == "" {
				fingerprint, _ = processorattestation.FingerprintPath(entry.PluginPath)
				fingerprintCache[entry.PluginPath] = fingerprint
			}
		}
		for _, family := range processorCertificationFamilies {
			capability := processorCapabilityForEntry(v1, v2, subjectKey, fingerprint, family, candidate.Certifiable)
			if family == "clipper" {
				capability.Reason = "independent_clipper_controller_not_in_this_entry"
			}
			candidate.Capabilities = append(candidate.Capabilities, capability)
			if capability.Status == processorattestation.StatusPromoted || capability.Status == processorattestation.StatusIssued {
				candidate.Status = "observed"
				candidate.Reason = "observed_capability"
			}
		}
		candidate.Certifiable = false
		for _, capability := range candidate.Capabilities {
			candidate.Certifiable = candidate.Certifiable || capability.Certifiable
		}
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result
}

func processorCapabilityForEntry(v1 processorattestation.Library, v2 processorattestation.LibraryV2, subjectKey, fingerprint, family string, pathAvailable bool) processorCapabilityStatus {
	capability := processorCapabilityStatus{Family: family, Version: "v2", Status: "unattested", Reason: "no_attestation", Certifiable: pathAvailable}
	if family == processorattestation.FamilyStaticEQ || family == processorattestation.FamilyBroadbandCompressor {
		capability.Version = "v1"
		if family == processorattestation.FamilyStaticEQ {
			capability.Certifiable = false
			capability.Reason = "generic_static_eq_certification_runner_unavailable"
		}
		attestation, found := bestProcessorAttestationV1(v1, subjectKey, family)
		if found {
			capability.AttestationID = attestation.AttestationID
			capability.CoverageLevel = "partial"
			capability.CoverageAxes = coverageAxes(attestation.Coverage)
			capability.FingerprintMatch = fingerprint != "" && fingerprint == attestation.BinaryFingerprint
			if !pathAvailable {
				capability.Status, capability.Reason = "unavailable", "plugin_binary_missing"
			} else if !capability.FingerprintMatch {
				capability.Status, capability.Reason = "stale", "binary_fingerprint_changed"
			} else {
				capability.Status, capability.Reason = attestation.Status, firstNonEmpty(attestation.StatusReason, "attestation_"+attestation.Status)
			}
		}
		return capability
	}
	if family == "spectral_dynamics" {
		return processorCapabilityStatus{Family: family, Version: "inspect-only", Status: "inspect_only", Reason: "no_pca_controller_vocabulary", Certifiable: false}
	}
	if family == "clipper" {
		return processorCapabilityStatus{Family: family, Version: "boundary", Status: "separate_boundary", Reason: "independent_clip­per_controller_not_in_this_entry", Certifiable: false}
	}
	attestation, found := bestProcessorAttestation(v2, subjectKey, family)
	if !found {
		return capability
	}
	capability.AttestationID = attestation.AttestationID
	capability.CoverageAxes = coverageAxes(attestation.Coverage)
	capability.CoverageLevel = "partial"
	if len(capability.CoverageAxes) == len(processorattestation.V2CoverageAxes(family)) {
		capability.CoverageLevel = "full"
	}
	capability.FingerprintMatch = fingerprint != "" && fingerprint == attestation.BinaryFingerprint
	if !pathAvailable {
		capability.Status, capability.Reason = "unavailable", "plugin_binary_missing"
	} else if !capability.FingerprintMatch {
		capability.Status, capability.Reason = "stale", "binary_fingerprint_changed"
	} else {
		capability.Status, capability.Reason = attestation.Status, firstNonEmpty(attestation.StatusReason, "attestation_"+attestation.Status)
	}
	return capability
}

func bestProcessorAttestationV1(library processorattestation.Library, subjectKey, family string) (processorattestation.Attestation, bool) {
	var best processorattestation.Attestation
	found := false
	for _, item := range library.Attestations {
		if item.Subject.SubjectKey != subjectKey || item.ProcessorFamily != family {
			continue
		}
		if !found || item.IssuedAt.After(best.IssuedAt) {
			best, found = item, true
		}
	}
	return best, found
}

func buildProcessorCertificationCandidates(index pluginsemantics.Index, library processorattestation.LibraryV2, family string) []processorCertificationCandidate {
	fingerprintCache := map[string]string{}
	result := make([]processorCertificationCandidate, 0, len(index.Entries))
	for _, entry := range index.Entries {
		if entry.IsInstrument || strings.TrimSpace(entry.Identifier) == "" || strings.TrimSpace(entry.PluginPath) == "" {
			continue
		}
		candidate := processorCertificationCandidate{
			ID: entry.ID, Name: firstNonEmpty(entry.Name, entry.DescriptiveName, filepath.Base(entry.PluginPath)),
			Manufacturer: entry.Manufacturer, Format: entry.Format, Identifier: entry.Identifier,
			PluginPath: entry.PluginPath, Family: family, Status: "unattested", Reason: "no_attestation",
			Certifiable: pathExists(entry.PluginPath), CoverageLevel: "none",
		}
		subject := processorattestation.Subject{Name: entry.Name, Manufacturer: entry.Manufacturer, Format: entry.Format, Identifier: entry.Identifier, InstalledPath: entry.PluginPath}
		subjectKey, keyErr := processorattestation.BuildSubjectKey(subject)
		if keyErr != nil {
			candidate.Status, candidate.Reason, candidate.Certifiable = "unresolved", "subject_identity_invalid", false
			result = append(result, candidate)
			continue
		}
		attestation, found := bestProcessorAttestation(library, subjectKey, family)
		if found {
			candidate.AttestationID = attestation.AttestationID
			candidate.CoverageAxes = coverageAxes(attestation.Coverage)
			candidate.CoverageLevel = "partial"
			if len(candidate.CoverageAxes) == len(processorattestation.V2CoverageAxes(family)) {
				candidate.CoverageLevel = "full"
			}
			fingerprint := fingerprintCache[entry.PluginPath]
			if fingerprint == "" && candidate.Certifiable {
				fingerprint, _ = processorattestation.FingerprintPath(entry.PluginPath)
				fingerprintCache[entry.PluginPath] = fingerprint
			}
			candidate.FingerprintMatch = fingerprint != "" && fingerprint == attestation.BinaryFingerprint
			if !candidate.Certifiable {
				candidate.Status, candidate.Reason = "unavailable", "plugin_binary_missing"
			} else if !candidate.FingerprintMatch {
				candidate.Status, candidate.Reason = "stale", "binary_fingerprint_changed"
			} else {
				candidate.Status = attestation.Status
				candidate.Reason = firstNonEmpty(attestation.StatusReason, "attestation_"+attestation.Status)
			}
		}
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		ri, rj := certificationStatusRank(result[i].Status), certificationStatusRank(result[j].Status)
		if ri != rj {
			return ri < rj
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func (s *Server) handleProcessorCertificationStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var request processorCertificationStartRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	request.Identifier = strings.TrimSpace(request.Identifier)
	request.Family = strings.ToLower(strings.TrimSpace(request.Family))
	if request.Identifier == "" || request.Family == "all" || !processorCertificationFamilyKnown(request.Family) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "exact identifier and supported processor capability are required"})
		return
	}
	if !request.Confirmed || request.Consent != processorCertificationConsent {
		writeJSON(w, http.StatusConflict, map[string]any{
			"status": "needs_confirmation", "error": "explicit certification consent is required",
			"consent_token": processorCertificationConsent,
			"notice":        "Certification creates a temporary track, performs typed apply/readback/restore, verifies the full parameter snapshot, and deletes the temporary track. It does not save the current project.",
		})
		return
	}
	if request.Family == processorattestation.FamilyStaticEQ || request.Family == "spectral_dynamics" || request.Family == "clipper" {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "inspect_only", "error": "selected capability does not have a product certification runner", "family": request.Family})
		return
	}
	index, err := pluginsemantics.Load("")
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "semantic library unavailable: " + err.Error()})
		return
	}
	entry, err := exactSemanticEntry(index, request.Identifier)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	if entry.IsInstrument || !pathExists(entry.PluginPath) {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "selected plugin is not a loadable effect binary"})
		return
	}
	base, err := localAgentBaseURL(r.Host)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	if running := s.runningProcessorCertificationJob(); running != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "busy", "error": "another processor certification is running", "job": running})
		return
	}
	jobID := "pca_job_" + randomID()
	outputDir, err := processorCertificationOutputDir(jobID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	job := processorCertificationJob{JobID: jobID, Identifier: entry.Identifier, PluginName: firstNonEmpty(entry.Name, entry.DescriptiveName), Family: request.Family, Status: "queued", Stage: "queued", Progress: 0, CreatedAt: time.Now().UTC(), OutputDir: outputDir}
	fingerprint, err := processorattestation.FingerprintPath(entry.PluginPath)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "selected plugin fingerprint unavailable: " + err.Error()})
		return
	}
	subjectKey, err := processorattestation.BuildSubjectKey(processorattestation.Subject{Name: entry.Name, Manufacturer: entry.Manufacturer, Format: entry.Format, Identifier: entry.Identifier, InstalledPath: entry.PluginPath})
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error": "selected plugin stable identity unavailable: " + err.Error()})
		return
	}
	loadToken := "pca_load_" + randomID()
	s.processorCertificationMu.Lock()
	if s.processorCertificationJobs == nil {
		s.processorCertificationJobs = map[string]processorCertificationJob{}
	}
	s.processorCertificationJobs[jobID] = job
	if s.processorCertificationLoadTokens == nil {
		s.processorCertificationLoadTokens = map[string]processorCertificationLoadAuthorization{}
	}
	s.processorCertificationLoadTokens[loadToken] = processorCertificationLoadAuthorization{JobID: jobID, Identifier: entry.Identifier, PluginPath: entry.PluginPath, ProcessorFamily: request.Family, SubjectKey: subjectKey, BinaryFingerprint: fingerprint, ExpiresAt: time.Now().UTC().Add(10 * time.Minute)}
	s.processorCertificationMu.Unlock()
	go s.runProcessorCertificationJob(jobID, base, index, entry, loadToken)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "job": job, "poll_url": "/agent/processor-certification/status?job_id=" + jobID})
}

func (s *Server) handleProcessorCertificationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))
	s.processorCertificationMu.Lock()
	job, ok := s.processorCertificationJobs[jobID]
	s.processorCertificationMu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "certification job not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "job": job})
}

func (s *Server) runProcessorCertificationJob(jobID, agentBase string, index pluginsemantics.Index, entry pluginsemantics.Entry, loadToken string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.failProcessorCertificationJob(jobID, "panic: "+fmt.Sprint(recovered))
		}
	}()
	started := time.Now().UTC()
	s.updateProcessorCertificationJob(jobID, func(job *processorCertificationJob) {
		job.Status, job.Stage, job.Progress, job.StartedAt = "running", "preflight", 3, &started
	})
	job := s.processorCertificationJob(jobID)
	if job.Family == processorattestation.FamilyBroadbandCompressor {
		s.runCompressorCertificationJob(jobID, agentBase, index, entry, loadToken)
		return
	}
	inspectTool, applyTool := processorCertificationToolPair(job.Family)
	report, err := processorauthority.CertifyProcessor(processorauthority.LocalProcessorCertificationOptions{
		AgentHTTP: agentBase, Entry: entry, OutputDir: job.OutputDir, ProcessorFamily: job.Family,
		InspectTool: inspectTool, ApplyTool: applyTool, Timeout: 180 * time.Second,
		LoadAuthorizationToken: loadToken,
		Progress:               func(stage string) { s.setProcessorCertificationStage(jobID, stage) },
	})
	s.updateProcessorCertificationJob(jobID, func(current *processorCertificationJob) {
		current.Certification, current.SummaryPath = report, report.SummaryPath
	})
	if err != nil {
		s.failProcessorCertificationJob(jobID, err.Error())
		return
	}
	s.updateProcessorCertificationJob(jobID, func(current *processorCertificationJob) {
		current.Stage, current.Progress = "importing_attestation", 95
	})
	store, err := processorattestation.NewStoreV2("")
	if err != nil {
		s.failProcessorCertificationJob(jobID, err.Error())
		return
	}
	importReport, err := processorauthority.ImportReceiptsV2(store, index, []string{report.SummaryPath})
	if err != nil {
		s.failProcessorCertificationJob(jobID, err.Error())
		return
	}
	completed := time.Now().UTC()
	s.updateProcessorCertificationJob(jobID, func(current *processorCertificationJob) {
		current.Import, current.Status, current.Stage, current.Progress, current.CompletedAt = importReport, "completed", "completed", 100, &completed
	})
}

func (s *Server) runCompressorCertificationJob(jobID, agentBase string, index pluginsemantics.Index, entry pluginsemantics.Entry, loadToken string) {
	s.updateProcessorCertificationJob(jobID, func(job *processorCertificationJob) {
		job.Stage, job.Progress = "certifying_broadband_compressor", 20
	})
	job := s.processorCertificationJob(jobID)
	report, err := processorauthority.CertifyCompressor(processorauthority.LocalCertificationOptions{AgentHTTP: agentBase, Entry: entry, OutputDir: job.OutputDir, Timeout: 180 * time.Second, LoadAuthorizationToken: loadToken})
	s.updateProcessorCertificationJob(jobID, func(current *processorCertificationJob) {
		current.Certification, current.SummaryPath = report, report.SummaryPath
	})
	if err != nil {
		s.failProcessorCertificationJob(jobID, err.Error())
		return
	}
	s.updateProcessorCertificationJob(jobID, func(current *processorCertificationJob) {
		current.Stage, current.Progress = "importing_attestation", 95
	})
	store, err := processorattestation.NewStore("")
	if err != nil {
		s.failProcessorCertificationJob(jobID, err.Error())
		return
	}
	importReport, err := processorauthority.ImportReceipts(store, index, []string{report.SummaryPath})
	if err != nil {
		s.failProcessorCertificationJob(jobID, err.Error())
		return
	}
	completed := time.Now().UTC()
	s.updateProcessorCertificationJob(jobID, func(current *processorCertificationJob) {
		current.ImportV1, current.Status, current.Stage, current.Progress, current.CompletedAt = importReport, "completed", "completed", 100, &completed
	})
}

func processorCertificationToolPair(family string) (string, string) {
	pairs := map[string][2]string{
		processorattestation.FamilyLimiter:      {"plugin_grabber.inspect_limiter", "plugin_grabber.apply_limiter_controls"},
		processorattestation.FamilyGateExpander: {"plugin_grabber.inspect_gate_expander", "plugin_grabber.apply_gate_expander_controls"},
		processorattestation.FamilyDeEsser:      {"plugin_grabber.inspect_de_esser", "plugin_grabber.apply_de_esser_controls"},
		processorattestation.FamilyTransient:    {"plugin_grabber.inspect_transient_shaper", "plugin_grabber.apply_transient_shaper_controls"},
		processorattestation.FamilyMultiband:    {"plugin_grabber.inspect_multiband", "plugin_grabber.apply_multiband_controls"},
	}
	return pairs[family][0], pairs[family][1]
}

func processorCertificationOutputDir(jobID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".vit", "pca_certifications", jobID)
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Server) setProcessorCertificationStage(jobID, stage string) {
	progress := map[string]int{"health_check": 5, "create_temporary_track": 10, "load_plugin": 18, "snapshot_parameters": 28, "inspect_topology": 40, "typed_apply": 58, "typed_readback": 68, "typed_restore": 76, "verify_full_snapshot": 86, "cleanup_temporary_track": 92}[stage]
	s.updateProcessorCertificationJob(jobID, func(job *processorCertificationJob) {
		job.Stage = stage
		if progress > job.Progress {
			job.Progress = progress
		}
	})
}

func (s *Server) failProcessorCertificationJob(jobID, message string) {
	completed := time.Now().UTC()
	s.updateProcessorCertificationJob(jobID, func(job *processorCertificationJob) {
		job.Status, job.Stage, job.Error, job.CompletedAt = "failed", "failed", message, &completed
		if job.Progress < 100 {
			job.Progress = 100
		}
	})
}

func (s *Server) updateProcessorCertificationJob(jobID string, update func(*processorCertificationJob)) {
	s.processorCertificationMu.Lock()
	defer s.processorCertificationMu.Unlock()
	job, ok := s.processorCertificationJobs[jobID]
	if !ok {
		return
	}
	update(&job)
	s.processorCertificationJobs[jobID] = job
}

func (s *Server) processorCertificationJob(jobID string) processorCertificationJob {
	s.processorCertificationMu.Lock()
	defer s.processorCertificationMu.Unlock()
	return s.processorCertificationJobs[jobID]
}

func (s *Server) runningProcessorCertificationJob() *processorCertificationJob {
	s.processorCertificationMu.Lock()
	defer s.processorCertificationMu.Unlock()
	for _, job := range s.processorCertificationJobs {
		if job.Status == "queued" || job.Status == "running" {
			copy := job
			return &copy
		}
	}
	return nil
}

func exactSemanticEntry(index pluginsemantics.Index, identifier string) (pluginsemantics.Entry, error) {
	for _, entry := range index.Entries {
		if strings.EqualFold(strings.TrimSpace(entry.Identifier), strings.TrimSpace(identifier)) {
			return entry, nil
		}
	}
	return pluginsemantics.Entry{}, fmt.Errorf("exact plugin identifier %q not found", identifier)
}

func bestProcessorAttestation(library processorattestation.LibraryV2, subjectKey, family string) (processorattestation.AttestationV2, bool) {
	var best processorattestation.AttestationV2
	found := false
	for _, item := range library.Attestations {
		if item.Subject.SubjectKey != subjectKey || item.ProcessorFamily != family {
			continue
		}
		if !found || certificationStatusRank(item.Status) < certificationStatusRank(best.Status) || item.IssuedAt.After(best.IssuedAt) {
			best, found = item, true
		}
	}
	return best, found
}

func coverageAxes(coverage []processorattestation.Coverage) []string {
	seen := map[string]bool{}
	var axes []string
	for _, item := range coverage {
		axis := strings.TrimSpace(item.Axis)
		if axis != "" && !seen[axis] {
			seen[axis] = true
			axes = append(axes, axis)
		}
	}
	sort.Strings(axes)
	return axes
}

func certificationStatusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case processorattestation.StatusPromoted:
		return 0
	case processorattestation.StatusIssued:
		return 1
	case processorattestation.StatusStale:
		return 2
	case processorattestation.StatusRevoked:
		return 3
	case "unattested":
		return 4
	case "unresolved":
		return 5
	default:
		return 6
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func localAgentBaseURL(hostPort string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(hostPort))
	if err != nil {
		return "", fmt.Errorf("invalid local Agent address")
	}
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("processor certification is local-only")
	}
	return "http://127.0.0.1:" + port, nil
}
