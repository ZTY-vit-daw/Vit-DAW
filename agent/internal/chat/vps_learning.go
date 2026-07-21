package chat

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/vps"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const vpsLearningSchemaVersion = "vit.plugin_learning_vps_v3.v1"

var vpsDBReadbackPattern = regexp.MustCompile(`(?i)([-+]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+))\s*dB\b`)

// vpsLearningCommit is deliberately compact enough for Chat/UI responses.
// Full parameter snapshots and write/readback receipts live in the VPS raw
// evidence archive instead of becoming a second project-scoped source of
// truth.
type vpsLearningCommit struct {
	SchemaVersion    string                      `json:"schema_version"`
	LibraryPath      string                      `json:"library_path"`
	VPSID            string                      `json:"vps_id,omitempty"`
	VPSRevision      int                         `json:"vps_revision,omitempty"`
	VPSStatus        vps.VPSStatus               `json:"vps_status,omitempty"`
	CredentialID     string                      `json:"credential_id,omitempty"`
	CredentialStatus vps.CredentialStatus        `json:"credential_status,omitempty"`
	CatalogVisible   bool                        `json:"catalog_visible"`
	CatalogEntry     *vps.ProviderCatalogEntry   `json:"catalog_entry,omitempty"`
	Conformance      map[string]any              `json:"conformance,omitempty"`
	Warnings         []string                    `json:"warnings,omitempty"`
	StateChanges     []vps.CredentialStateChange `json:"credential_state_changes,omitempty"`
}

type vpsStaticEQRun struct {
	Result      vps.ConformanceResult
	FinalDigest pluginParameterDigest
	Receipts    []map[string]any
}

// commitPluginLearningVPSV3 is called only after the existing, user-confirmed
// Plugin Skill upsert succeeds. It first persists a non-dispatchable VPS
// candidate, then runs the bounded conformance transaction when the learning
// evidence is sufficient. A failed conformance remains useful evidence but
// cannot leak into the derived Catalog.
func (s *Server) commitPluginLearningVPSV3(ctx context.Context, plan PendingPlan) (vpsLearningCommit, error) {
	result := vpsLearningCommit{SchemaVersion: vpsLearningSchemaVersion}
	if s == nil || s.harness == nil {
		return result, fmt.Errorf("VPS v3 learning requires the command harness")
	}
	skill, err := vpsSkillFromWorkflowData(plan.WorkflowData)
	if err != nil {
		return result, err
	}
	target := vpsLearningTarget(plan.WorkflowData)
	if firstNonEmptyText(target, "track_id") == "" || firstNonEmptyText(target, "plugin_id") == "" {
		return result, fmt.Errorf("VPS v3 learning requires a track and loaded plugin target")
	}

	readContext := vpsLearningHarnessContext(plan)
	fresh, err := s.readVPSV3ParameterSurface(ctx, target, readContext)
	if err != nil {
		return result, err
	}
	digest := buildPluginParameterDigest(fresh)
	installationFingerprint, installationErr := vpsInstallationFingerprint(digest, skill)
	evidenceRefs := []string{"plugin_learning:confirmed_skill"}
	input := vps.LearningInput{
		Skill:                   skill,
		Digest:                  digest,
		InstallationFingerprint: installationFingerprint,
		EvidenceRefs:            evidenceRefs,
		ObservedAt:              time.Now().UTC(),
	}

	library, err := s.userVPSLibrary()
	if err != nil {
		return result, err
	}
	result.LibraryPath = library.Path()
	firstBuild, err := vps.BuildLearningVPS(input, nil)
	if err != nil {
		return result, err
	}
	build := firstBuild
	if prior, found, findErr := library.FindByPluginIdentity(firstBuild.Document.PluginIdentity); findErr != nil {
		return result, findErr
	} else if found {
		build, err = vps.BuildLearningVPS(input, &prior)
		if err != nil {
			return result, err
		}
	}
	if installationErr != nil {
		build.Warnings = append(build.Warnings, "Installation fingerprint was not available: "+installationErr.Error())
	}
	stored, err := library.Upsert(build.Document)
	if err != nil {
		return result, err
	}
	result = vpsCommitFromDocument(result, stored, build.Warnings, build.StateChanges)

	// No candidate mapping or no complete current fingerprint is a normal
	// fail-closed outcome. The VPS stays mapped and the user never has to
	// manually register a Provider.
	if build.StaticEQBinding == nil {
		return finalizeVPSCommitCatalog(library, result)
	}
	if !stored.PluginIdentity.Fingerprint.Complete() {
		result.Warnings = append(result.Warnings, "Credential was not issued because the fresh plugin fingerprint is incomplete.")
		return finalizeVPSCommitCatalog(library, result)
	}

	run, runErr := s.runVPSStaticEQConformance(ctx, target, *build.StaticEQBinding, digest, readContext)
	if runErr != nil {
		stored.RecordConformanceAttempt(run.Result, "failed", runErr.Error(), time.Now().UTC())
		stored.Status = vps.VPSStatusMapped
		stored, err = library.Upsert(stored)
		if err != nil {
			return result, err
		}
		result = vpsCommitFromDocument(result, stored, append(result.Warnings, "Static EQ conformance did not pass: "+runErr.Error()), build.StateChanges)
		result.Conformance = vpsConformanceSummary(run.Result, "fail", runErr.Error())
		return finalizeVPSCommitCatalog(library, result)
	}
	postFingerprint, err := vps.BuildPluginFingerprintFromDigest(installationFingerprint, run.FinalDigest)
	if err != nil {
		stored.RecordConformanceAttempt(run.Result, "incomplete", err.Error(), time.Now().UTC())
		stored.Status = vps.VPSStatusMapped
		stored, upsertErr := library.Upsert(stored)
		if upsertErr != nil {
			return result, upsertErr
		}
		result = vpsCommitFromDocument(result, stored, append(result.Warnings, "Credential was not issued after conformance: "+err.Error()), build.StateChanges)
		result.Conformance = vpsConformanceSummary(run.Result, "fail", err.Error())
		return finalizeVPSCommitCatalog(library, result)
	}
	if !stored.PluginIdentity.Fingerprint.Equal(postFingerprint) {
		if _, reconcileErr := stored.ReconcileCredentialFingerprints(postFingerprint, time.Now().UTC()); reconcileErr != nil {
			return result, reconcileErr
		}
		stored.PluginIdentity.Fingerprint = postFingerprint
	}
	credential, err := stored.IssueCredential(run.Result, time.Now().UTC())
	if err != nil {
		stored.RecordConformanceAttempt(run.Result, "incomplete", err.Error(), time.Now().UTC())
		stored.Status = vps.VPSStatusMapped
		stored, upsertErr := library.Upsert(stored)
		if upsertErr != nil {
			return result, upsertErr
		}
		result = vpsCommitFromDocument(result, stored, append(result.Warnings, "Credential was not issued: "+err.Error()), build.StateChanges)
		result.Conformance = vpsConformanceSummary(run.Result, "fail", err.Error())
		return finalizeVPSCommitCatalog(library, result)
	}
	stored, err = library.Upsert(stored)
	if err != nil {
		return result, err
	}
	result = vpsCommitFromDocument(result, stored, build.Warnings, build.StateChanges)
	result.CredentialID = credential.ID
	result.CredentialStatus = credential.Status
	result.Conformance = vpsConformanceSummary(run.Result, "pass", "")
	return finalizeVPSCommitCatalog(library, result)
}

func vpsCommitFromDocument(base vpsLearningCommit, document vps.VPSDocument, warnings []string, changes []vps.CredentialStateChange) vpsLearningCommit {
	base.VPSID = document.ID
	base.VPSRevision = document.Revision
	base.VPSStatus = document.Status
	base.Warnings = uniqueVPSWarnings(append(base.Warnings, warnings...))
	base.StateChanges = append([]vps.CredentialStateChange(nil), changes...)
	return base
}

func finalizeVPSCommitCatalog(library *vps.Library, result vpsLearningCommit) (vpsLearningCommit, error) {
	catalog, err := library.Catalog()
	if err != nil {
		return result, err
	}
	for index := range catalog.Entries {
		entry := catalog.Entries[index]
		if entry.VPSID == result.VPSID && entry.CredentialID == result.CredentialID && result.CredentialID != "" {
			result.CatalogVisible = true
			copy := entry
			result.CatalogEntry = &copy
			break
		}
	}
	return result, nil
}

func vpsSkillFromWorkflowData(data map[string]any) (plugingrabber.PluginSkillDocument, error) {
	raw := firstPresentAny(data, "plugin_skill")
	if raw == nil {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("Plugin Learning did not retain the validated Plugin Skill required for VPS v3")
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("encode learned Plugin Skill: %w", err)
	}
	var skill plugingrabber.PluginSkillDocument
	if err := json.Unmarshal(payload, &skill); err != nil {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("decode learned Plugin Skill: %w", err)
	}
	if skill.SchemaVersion != plugingrabber.PluginSkillSchemaVersion {
		return plugingrabber.PluginSkillDocument{}, fmt.Errorf("Plugin Learning retained an unsupported Plugin Skill schema")
	}
	return skill, nil
}

func vpsLearningTarget(data map[string]any) map[string]any {
	target := copyStringAnyMap(mapValue(data["target"]))
	if target == nil {
		target = map[string]any{}
	}
	for _, key := range []string{"track_id", "plugin_id", "plugin_name"} {
		if strings.TrimSpace(fmt.Sprint(target[key])) == "" || fmt.Sprint(target[key]) == "<nil>" {
			target[key] = data[key]
		}
	}
	return target
}

func vpsLearningHarnessContext(plan PendingPlan) map[string]any {
	context := copyStringAnyMap(plan.Context)
	if context == nil {
		context = map[string]any{}
	}
	// This string is intentionally specific: the Harness's broad-mix guard
	// must not mistake a confirmed qualification transaction for an acoustic
	// treatment request.
	context["user_message"] = "VPS v3 Plugin Learning conformance"
	context["vps_v3_conformance"] = true
	return context
}

func (s *Server) userVPSLibrary() (*vps.Library, error) {
	if s == nil {
		return nil, fmt.Errorf("VPS v3 server is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vpsLibrary != nil {
		return s.vpsLibrary, nil
	}
	library, err := vps.OpenDefaultLibrary()
	if err != nil {
		return nil, err
	}
	s.vpsLibrary = library
	return library, nil
}

func (s *Server) readVPSV3ParameterSurface(ctx context.Context, target, requestContext map[string]any) (map[string]any, error) {
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "plugin.get_parameters",
		Args: map[string]any{
			"track_id":               firstNonEmptyText(target, "track_id"),
			"plugin_id":              firstNonEmptyText(target, "plugin_id"),
			"include_vps_v3_surface": true,
		},
		Context:   requestContext,
		Source:    "vps_v3_conformance",
		Confirmed: true,
	})
	if err != nil {
		return nil, err
	}
	if response.Status != "ok" || !kernelReplyOK(response.Result) {
		return nil, fmt.Errorf("fresh plugin parameter readback failed: %s", firstNonEmptyText(response.Result, "message", "error"))
	}
	if len(mapRowsValue(response.Result["parameters"])) == 0 {
		return nil, fmt.Errorf("fresh plugin parameter readback omitted the VPS v3 parameter surface")
	}
	return response.Result, nil
}

func vpsInstallationFingerprint(digest pluginParameterDigest, skill plugingrabber.PluginSkillDocument) (string, error) {
	identity := digest.PluginIdentity
	for _, key := range []string{"installation_fingerprint", "plugin_installation_fingerprint"} {
		if value := strings.TrimSpace(fmt.Sprint(identity[key])); value != "" && value != "<nil>" {
			if vpsSHA256Fingerprint(value) {
				return value, nil
			}
		}
	}
	path := firstNonEmptyText(identity, "plugin_path", "path")
	if path == "" {
		path = strings.TrimSpace(skill.Identity.Path)
	}
	return vps.BuildInstallationFingerprint(path)
}

func vpsSHA256Fingerprint(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != len("sha256:")+64 || !strings.EqualFold(value[:len("sha256:")], "sha256:") {
		return false
	}
	_, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil
}

func (s *Server) runVPSStaticEQConformance(ctx context.Context, target map[string]any, binding vps.StaticEQBinding, initial pluginParameterDigest, requestContext map[string]any) (run vpsStaticEQRun, err error) {
	evidenceID := "vps_v3_conformance:" + randomID()
	run.Result = vps.ConformanceResult{
		CapabilityID: vps.StaticEQCapabilityID,
		Operations:   []string{vps.OperationAllocateBand, vps.OperationPatchBand, vps.OperationReadBand, vps.OperationReleaseBand, vps.OperationBellCut},
		Parameters:   []string{"filter_type", "frequency_hz", "gain_db", "q", "enabled"},
		FilterTypes:  []string{"bell"},
		StaticEQBinding: func() *vps.StaticEQBinding {
			copy := binding
			return &copy
		}(),
		EvidenceRefs: []string{evidenceID},
	}
	defer func() {
		run.Result.CompletedAt = time.Now().UTC()
		detail, marshalErr := json.Marshal(map[string]any{
			"binding":  binding,
			"receipts": run.Receipts,
			"error":    errorText(err),
		})
		if marshalErr == nil {
			run.Result.Details = detail
		}
	}()

	parameters := vpsParameterIndex(initial)
	needed := []struct {
		semantic string
		id       string
	}{
		{"filter_type", binding.FilterTypeParameterID},
		{"frequency_hz", binding.FrequencyParameterID},
		{"gain_db", binding.GainParameterID},
		{"q", binding.QParameterID},
		{"enabled", binding.EnabledParameterID},
	}
	original := map[string]float64{}
	restoreValues := map[string]float64{}
	for _, item := range needed {
		parameter, ok := parameters[item.id]
		if !ok || !parameter.HostControllable {
			return run, fmt.Errorf("conformance mapping %s no longer exists in the fresh host parameter surface", item.semantic)
		}
		value, ok := vpsFiniteNumber(parameter.NormalizedValue)
		if !ok {
			return run, fmt.Errorf("conformance mapping %s has no finite normalized readback", item.semantic)
		}
		original[item.id] = value
		if item.semantic != "filter_type" {
			restoreValues[item.id] = value
		}
	}
	filterType := parameters[binding.FilterTypeParameterID]
	if !strings.Contains(strings.ToLower(strings.TrimSpace(filterType.ValueText)), "bell") {
		return run, fmt.Errorf("the confirmed filter-type mapping is not currently a Bell filter")
	}
	gain := parameters[binding.GainParameterID]
	if _, _, rangeOK := vpsBipolarDBDisplayRange(gain); !rangeOK {
		return run, fmt.Errorf("the confirmed gain mapping does not prove a bipolar dB range")
	}
	for _, id := range []string{binding.FrequencyParameterID, binding.GainParameterID, binding.QParameterID} {
		parameter := parameters[id]
		if parameter.IsDiscrete || parameter.IsBoolean {
			return run, fmt.Errorf("the confirmed static-EQ mapping %s is not continuously writable", id)
		}
	}
	run.Receipts = append(run.Receipts, map[string]any{
		"stage":        "allocate_band",
		"component_id": binding.ComponentID,
		"status":       "pass",
	})

	writesStarted := false
	defer func() {
		if !writesStarted {
			return
		}
		final, restoreErr := s.restoreVPSStaticEQParameters(ctx, target, restoreValues, original, requestContext, &run)
		if restoreErr != nil {
			run.Result.RollbackTestPassed = false
			if err == nil {
				err = fmt.Errorf("VPS v3 conformance rollback failed: %w", restoreErr)
			}
			return
		}
		run.FinalDigest = final
		run.Result.RollbackTestPassed = true
		run.Receipts = append(run.Receipts, map[string]any{"stage": "release_band", "status": "pass"})
	}()

	patchValues := []struct {
		name  string
		id    string
		value float64
	}{
		{"frequency_hz", binding.FrequencyParameterID, 0.37},
		{"gain_db", binding.GainParameterID, 0.25},
		{"q", binding.QParameterID, 0.62},
		{"enabled", binding.EnabledParameterID, 1.0},
	}
	for _, patch := range patchValues {
		writesStarted = true
		if err := s.writeAndReadVPSParameter(ctx, target, patch.id, patch.value, patch.name, requestContext, &run); err != nil {
			return run, err
		}
	}
	run.Result.WriteReadbackPassed = true
	run.Receipts = append(run.Receipts, map[string]any{"stage": "patch_band", "status": "pass"})
	patched, readErr := s.readVPSV3ParameterSurface(ctx, target, requestContext)
	if readErr != nil {
		return run, readErr
	}
	patchedDigest := buildPluginParameterDigest(patched)
	patchedGain, ok := vpsParameterIndex(patchedDigest)[binding.GainParameterID]
	if !ok {
		return run, fmt.Errorf("gain mapping disappeared after patch readback")
	}
	patchedGainDB, ok := vpsDBValueFromReadback(patchedGain)
	if !ok || patchedGainDB >= 0 {
		return run, fmt.Errorf("Bell-cut probe did not produce a negative dB gain readback")
	}
	run.Receipts = append(run.Receipts, map[string]any{"stage": "read_band", "actual_display_db": patchedGainDB, "status": "pass"})
	run.Receipts = append(run.Receipts, map[string]any{"stage": "bell_cut", "status": "pass"})

	boundaries := []struct {
		name string
		id   string
	}{
		{"frequency_hz", binding.FrequencyParameterID},
		{"gain_db", binding.GainParameterID},
		{"q", binding.QParameterID},
		{"enabled", binding.EnabledParameterID},
	}
	for _, boundary := range boundaries {
		for _, value := range []float64{0, 1} {
			if err := s.writeAndReadVPSParameter(ctx, target, boundary.id, value, boundary.name+"_boundary", requestContext, &run); err != nil {
				return run, err
			}
		}
	}
	run.Result.BoundaryTestsPassed = true
	run.Receipts = append(run.Receipts, map[string]any{"stage": "boundary", "status": "pass"})
	return run, nil
}

func (s *Server) writeAndReadVPSParameter(ctx context.Context, target map[string]any, parameterID string, wanted float64, stage string, requestContext map[string]any, run *vpsStaticEQRun) error {
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "plugin.set_parameter",
		Args: map[string]any{
			"track_id":         firstNonEmptyText(target, "track_id"),
			"plugin_id":        firstNonEmptyText(target, "plugin_id"),
			"param_id":         parameterID,
			"normalized_value": wanted,
		},
		Context:   requestContext,
		Source:    "vps_v3_conformance",
		Confirmed: true,
	})
	if err != nil {
		return fmt.Errorf("%s write %s: %w", stage, parameterID, err)
	}
	if response.Status != "ok" || !kernelReplyOK(response.Result) {
		return fmt.Errorf("%s write %s failed: %s", stage, parameterID, firstNonEmptyText(response.Result, "message", "error"))
	}
	fresh, err := s.readVPSV3ParameterSurface(ctx, target, requestContext)
	if err != nil {
		return fmt.Errorf("%s readback %s: %w", stage, parameterID, err)
	}
	digest := buildPluginParameterDigest(fresh)
	parameter, ok := vpsParameterIndex(digest)[parameterID]
	if !ok {
		return fmt.Errorf("%s readback omitted %s", stage, parameterID)
	}
	actual, ok := vpsFiniteNumber(parameter.NormalizedValue)
	if !ok || !vpsNormalizedValuesMatch(actual, wanted) {
		return fmt.Errorf("%s readback for %s is %.6f, want %.6f", stage, parameterID, actual, wanted)
	}
	if run != nil {
		run.Receipts = append(run.Receipts, map[string]any{
			"stage":                stage,
			"parameter_id":         parameterID,
			"requested_normalized": wanted,
			"readback_normalized":  actual,
			"status":               "pass",
		})
	}
	return nil
}

func (s *Server) restoreVPSStaticEQParameters(ctx context.Context, target map[string]any, restoreValues, expectedValues map[string]float64, requestContext map[string]any, run *vpsStaticEQRun) (pluginParameterDigest, error) {
	ids := make([]string, 0, len(restoreValues))
	for id := range restoreValues {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	for _, id := range ids {
		if err := s.writeAndReadVPSParameter(ctx, target, id, restoreValues[id], "rollback", requestContext, run); err != nil {
			return pluginParameterDigest{}, err
		}
	}
	fresh, err := s.readVPSV3ParameterSurface(ctx, target, requestContext)
	if err != nil {
		return pluginParameterDigest{}, err
	}
	digest := buildPluginParameterDigest(fresh)
	parameters := vpsParameterIndex(digest)
	for id, wanted := range expectedValues {
		parameter, ok := parameters[id]
		if !ok {
			return pluginParameterDigest{}, fmt.Errorf("rollback readback omitted %s", id)
		}
		actual, ok := vpsFiniteNumber(parameter.NormalizedValue)
		if !ok || !vpsNormalizedValuesMatch(actual, wanted) {
			return pluginParameterDigest{}, fmt.Errorf("rollback readback for %s is %.6f, want %.6f", id, actual, wanted)
		}
	}
	return digest, nil
}

func vpsParameterIndex(digest pluginParameterDigest) map[string]pluginParameterInfo {
	index := make(map[string]pluginParameterInfo, len(digest.Parameters))
	for _, parameter := range digest.Parameters {
		if id := strings.TrimSpace(parameter.ID); id != "" {
			index[id] = parameter
		}
	}
	return index
}

func vpsFiniteNumber(value any) (float64, bool) {
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case float32:
		number = float64(typed)
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case int32:
		number = float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		number = parsed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		number = parsed
	default:
		return 0, false
	}
	return number, !math.IsNaN(number) && !math.IsInf(number, 0)
}

// vpsBipolarDBDisplayRange deliberately reads the physical display domain,
// rather than ParameterInfo.Min/Max: hosts expose those latter values in the
// normalized 0..1 transport domain even when the plugin's audible gain is
// bipolar dB.
func vpsBipolarDBDisplayRange(parameter pluginParameterInfo) (float64, float64, bool) {
	domain := parameter.DisplayDomainCandidate
	if domain == nil || !strings.EqualFold(strings.TrimSpace(domain.Unit), "db") || domain.Min == nil || domain.Max == nil {
		return 0, 0, false
	}
	minimum, maximum := *domain.Min, *domain.Max
	if math.IsNaN(minimum) || math.IsInf(minimum, 0) || math.IsNaN(maximum) || math.IsInf(maximum, 0) || minimum >= 0 || maximum <= 0 {
		return 0, 0, false
	}
	return minimum, maximum, true
}

// vpsDBValueFromReadback requires the host's human-facing dB text as actual
// Bell-cut evidence. Interpolating a normalized value through a learned range
// would only confirm our expectation, not the plugin's readback.
func vpsDBValueFromReadback(parameter pluginParameterInfo) (float64, bool) {
	text := strings.TrimSpace(parameter.ValueText)
	if text == "" && parameter.DisplayProbe != nil {
		text = strings.TrimSpace(parameter.DisplayProbe.CurrentText)
	}
	text = strings.ReplaceAll(text, "−", "-")
	match := vpsDBReadbackPattern.FindStringSubmatch(text)
	if len(match) != 2 {
		// TDR Nova exposes the numeric readout and its dB unit in separate
		// fields (for example value_text="-9.0", display_probe.label="dB").
		// That is still a human-facing host readback, but never accept a bare
		// number unless the same fresh surface explicitly identifies its unit.
		if !vpsReadbackReportsDB(parameter) {
			return 0, false
		}
		value, ok := vpsFiniteNumber(text)
		if !ok {
			return 0, false
		}
		return value, true
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func vpsReadbackReportsDB(parameter pluginParameterInfo) bool {
	for _, unit := range []string{parameter.Unit} {
		if strings.EqualFold(strings.TrimSpace(unit), "db") {
			return true
		}
	}
	if parameter.DisplayProbe != nil && strings.EqualFold(strings.TrimSpace(parameter.DisplayProbe.Label), "db") {
		return true
	}
	return parameter.DisplayDomainCandidate != nil && strings.EqualFold(strings.TrimSpace(parameter.DisplayDomainCandidate.Unit), "db")
}

func vpsNormalizedValuesMatch(actual, wanted float64) bool {
	// The host exposes normalized float values, so restoration is expected to
	// be materially exact. This admits only normal floating-point transport
	// noise; it is not a musical tolerance.
	return math.Abs(actual-wanted) <= 0.001
}

func vpsConformanceSummary(result vps.ConformanceResult, status, failure string) map[string]any {
	out := map[string]any{
		"status":         status,
		"capability_id":  result.CapabilityID,
		"write_readback": result.WriteReadbackPassed,
		"boundary_tests": result.BoundaryTestsPassed,
		"rollback":       result.RollbackTestPassed,
		"completed_at":   result.CompletedAt,
	}
	if strings.TrimSpace(failure) != "" {
		out["failure"] = strings.TrimSpace(failure)
	}
	return out
}

func uniqueVPSWarnings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// finalizePluginLearningVPSV3 deliberately does not turn an already-saved
// Plugin Skill into an HTTP failure when qualification is incomplete. The
// saved Skill and mapped VPS remain valuable; the response states clearly that
// no Credential or Catalog entry was granted.
func (s *Server) finalizePluginLearningVPSV3(ctx context.Context, plan *PendingPlan, message string) string {
	if plan == nil || !isPluginGrabberLearningPlan(*plan) {
		return message
	}
	if plan.WorkflowData == nil {
		plan.WorkflowData = map[string]any{}
	}
	commit, err := s.commitPluginLearningVPSV3(ctx, *plan)
	if err != nil {
		plan.WorkflowData["vps_v3"] = map[string]any{
			"schema_version":  vpsLearningSchemaVersion,
			"status":          "unavailable",
			"catalog_visible": false,
			"warning":         err.Error(),
		}
		return strings.TrimSpace(message + "\n\nVPS v3 was not updated: " + err.Error())
	}
	plan.WorkflowData["vps_v3"] = vpsLearningCommitData(commit)
	if commit.CredentialStatus == vps.CredentialVerified && commit.CatalogVisible {
		return strings.TrimSpace(message + "\n\nVPS v3 conformance passed. A Provider Credential was issued and is now visible in the derived Provider Catalog.")
	}
	reason := "The VPS was saved as a non-dispatchable mapping; no Provider Credential was issued."
	if len(commit.Warnings) > 0 {
		reason += " " + commit.Warnings[0]
	}
	return strings.TrimSpace(message + "\n\n" + reason)
}

func vpsLearningCommitData(commit vpsLearningCommit) map[string]any {
	payload, err := json.Marshal(commit)
	if err != nil {
		return map[string]any{
			"schema_version": vpsLearningSchemaVersion,
			"status":         "unavailable",
			"warning":        "Could not encode VPS v3 learning result.",
		}
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return map[string]any{"schema_version": vpsLearningSchemaVersion, "status": "unavailable"}
	}
	return out
}

// handleVPSCatalog exposes the read-only, derived user-level Catalog to the
// app. It never creates a library or a Credential; an empty response is the
// correct result until a learning/conformance run has actually verified one.
func (s *Server) handleVPSCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	library, err := s.userVPSLibrary()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	catalog, err := library.Catalog()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"library_path": library.Path(),
		"catalog":      catalog,
	})
}
