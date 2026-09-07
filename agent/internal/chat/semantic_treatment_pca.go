package chat

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

// semanticTreatmentPCAInput is deliberately server-owned. The model can
// choose semantic axes, but it cannot supply subject identity, a fingerprint,
// or an attestation reference.
type semanticTreatmentPCAInput struct {
	Family           string
	RequiredCoverage []processorattestation.Coverage
}

// semanticPCAAdmissionReceipt is the durable result of selecting an exact
// PCA-admitted installed binary. Rack instance IDs are deliberately absent:
// they identify a project node, not an installed processor subject.
type semanticPCAAdmissionReceipt struct {
	ProcessorFamily   string
	Name              string
	Manufacturer      string
	Format            string
	Identifier        string
	PluginPath        string
	SubjectKey        string
	BinaryFingerprint string
	AttestationID     string
}

func semanticPCAAdmissionReceiptMap(receipt semanticPCAAdmissionReceipt) map[string]any {
	return map[string]any{
		"processor_family":   receipt.ProcessorFamily,
		"name":               receipt.Name,
		"manufacturer":       receipt.Manufacturer,
		"format":             receipt.Format,
		"identifier":         receipt.Identifier,
		"plugin_path":        receipt.PluginPath,
		"subject_key":        receipt.SubjectKey,
		"binary_fingerprint": receipt.BinaryFingerprint,
		"attestation_id":     receipt.AttestationID,
	}
}

func semanticPCAAdmissionReceiptFromPlan(plan PendingPlan) (semanticPCAAdmissionReceipt, bool, error) {
	return semanticPCAAdmissionReceiptFromValue(firstMapFromAny(plan.WorkflowData["pca_admission_receipt"]), firstMapFromAny(plan.WorkflowData["selected_candidate"]))
}

// semanticPCAAdmissionReceiptFromContext reads the same server-issued receipt
// after a post-load handoff has become a separate capability proposal.  The
// receipt remains opaque to the model and is never reconstructed from a rack
// instance ID or a display name.
func semanticPCAAdmissionReceiptFromContext(ctx map[string]any) (semanticPCAAdmissionReceipt, bool, error) {
	return semanticPCAAdmissionReceiptFromValue(firstMapFromAny(ctx["pca_admission_receipt"]), firstMapFromAny(ctx["semantic_plugin_recommendation_candidate"]))
}

func semanticPCAAdmissionReceiptFromValue(receiptValue, candidate map[string]any) (semanticPCAAdmissionReceipt, bool, error) {
	if len(receiptValue) == 0 {
		if firstStringFromMap(candidate, "processor_family") != "" {
			return semanticPCAAdmissionReceipt{}, false, fmt.Errorf("pca admission receipt is missing for the selected plugin")
		}
		return semanticPCAAdmissionReceipt{}, false, nil
	}
	receipt := semanticPCAAdmissionReceipt{
		ProcessorFamily:   strings.ToLower(strings.TrimSpace(firstStringFromMap(receiptValue, "processor_family"))),
		Name:              firstNonEmpty(firstStringFromMap(receiptValue, "name", "plugin_name"), firstStringFromMap(candidate, "name", "plugin_name")),
		Manufacturer:      firstNonEmpty(firstStringFromMap(receiptValue, "manufacturer", "vendor"), firstStringFromMap(candidate, "manufacturer", "vendor")),
		Format:            firstNonEmpty(firstStringFromMap(receiptValue, "format"), firstStringFromMap(candidate, "format")),
		Identifier:        firstNonEmpty(firstStringFromMap(receiptValue, "identifier", "plugin_identifier"), firstStringFromMap(candidate, "identifier", "plugin_identifier")),
		PluginPath:        firstNonEmpty(firstStringFromMap(receiptValue, "plugin_path", "path"), firstStringFromMap(candidate, "plugin_path", "path")),
		SubjectKey:        firstStringFromMap(receiptValue, "subject_key"),
		BinaryFingerprint: firstStringFromMap(receiptValue, "binary_fingerprint"),
		AttestationID:     firstStringFromMap(receiptValue, "attestation_id"),
	}
	if receipt.ProcessorFamily == "" || receipt.Name == "" || receipt.Format == "" || receipt.Identifier == "" || receipt.PluginPath == "" ||
		receipt.SubjectKey == "" || receipt.BinaryFingerprint == "" || receipt.AttestationID == "" {
		return semanticPCAAdmissionReceipt{}, true, fmt.Errorf("pca admission receipt is incomplete")
	}
	return receipt, true, nil
}

// semanticValidatePCAAdmissionReceipt rechecks the receipt against the
// current installed binary. It is used before the load and after load, but
// never derives PCA identity from a numeric rack instance ID.
func semanticValidatePCAAdmissionReceipt(plan PendingPlan, family, loadIdentifier string) (semanticPCAAdmissionReceipt, bool, error) {
	receipt, found, err := semanticPCAAdmissionReceiptFromPlan(plan)
	if err != nil || !found {
		return receipt, found, err
	}
	if err := semanticValidatePCAAdmissionReceiptValue(receipt, family, loadIdentifier); err != nil {
		return receipt, true, err
	}
	return receipt, true, nil
}

func semanticValidatePCAAdmissionReceiptValue(receipt semanticPCAAdmissionReceipt, family, loadIdentifier string) error {
	family = strings.ToLower(strings.TrimSpace(family))
	if family != "" && receipt.ProcessorFamily != family {
		return fmt.Errorf("pca admission receipt family mismatch: %s != %s", receipt.ProcessorFamily, family)
	}
	if loadIdentifier = strings.TrimSpace(loadIdentifier); loadIdentifier != "" && !strings.EqualFold(loadIdentifier, receipt.Identifier) {
		return fmt.Errorf("loaded plugin identifier does not match PCA admission receipt")
	}
	currentFingerprint, err := processorattestation.FingerprintPath(receipt.PluginPath)
	if err != nil {
		return fmt.Errorf("pca admission receipt binary fingerprint unavailable: %w", err)
	}
	if !strings.EqualFold(currentFingerprint, receipt.BinaryFingerprint) {
		return fmt.Errorf("pca admission receipt binary fingerprint changed")
	}
	result, err := processorattestation.QueryInstalledAdmission(processorattestation.InstalledSubject{Subject: processorattestation.Subject{
		Name: receipt.Name, Manufacturer: receipt.Manufacturer, Format: receipt.Format,
		Identifier: receipt.Identifier, InstalledPath: receipt.PluginPath,
	}}, receipt.ProcessorFamily)
	if err != nil {
		return fmt.Errorf("authoritative PCA receipt query failed: %w", err)
	}
	if !result.Eligible {
		return fmt.Errorf("pca admission receipt rejected: %s", result.Reason)
	}
	if !strings.EqualFold(result.SubjectKey, receipt.SubjectKey) || !strings.EqualFold(result.BinaryFingerprint, receipt.BinaryFingerprint) || result.AttestationID != receipt.AttestationID {
		return fmt.Errorf("pca admission receipt is no longer current")
	}
	return nil
}

// semanticValidatePCAAdmissionReceiptForInput verifies the receipt's exact
// installed binary against the action coverage frozen by the semantic intent.
// Family admission alone is insufficient: a promoted multiband processor, for
// example, may be certified for timing but not frequency-focus control.
func semanticValidatePCAAdmissionReceiptForInput(receipt semanticPCAAdmissionReceipt, input semanticTreatmentPCAInput) error {
	if err := semanticValidatePCAAdmissionReceiptValue(receipt, input.Family, ""); err != nil {
		return err
	}
	if len(input.RequiredCoverage) == 0 {
		return nil
	}
	result, err := processorattestation.QueryInstalled(processorattestation.InstalledSubject{Subject: processorattestation.Subject{
		Name: receipt.Name, Manufacturer: receipt.Manufacturer, Format: receipt.Format,
		Identifier: receipt.Identifier, InstalledPath: receipt.PluginPath,
	}}, processorattestation.EligibilityRequirement{ProcessorFamily: input.Family, RequiredCoverage: input.RequiredCoverage})
	if err != nil {
		return fmt.Errorf("authoritative PCA coverage query failed: %w", err)
	}
	if !result.Eligible {
		return fmt.Errorf("pca admission receipt does not cover frozen controls: %s", result.Reason)
	}
	if !strings.EqualFold(result.SubjectKey, receipt.SubjectKey) || !strings.EqualFold(result.BinaryFingerprint, receipt.BinaryFingerprint) || result.AttestationID != receipt.AttestationID {
		return fmt.Errorf("pca admission receipt is no longer current")
	}
	return nil
}

func semanticTreatmentPCAInputFromIntent(value map[string]any) (semanticTreatmentPCAInput, error) {
	if len(value) == 0 {
		return semanticTreatmentPCAInput{}, fmt.Errorf("semantic processor intent is required before PCA qualification")
	}
	family := strings.ToLower(strings.TrimSpace(firstStringFromMap(value, "family")))
	if family == "" {
		return semanticTreatmentPCAInput{}, fmt.Errorf("semantic processor intent family is required before PCA qualification")
	}
	return semanticTreatmentPCAInput{Family: family}, nil
}

func semanticTreatmentPCAInputFromRequirement(value any) (semanticTreatmentPCAInput, error) {
	requirement, ok := pluginControlRequirementFromAny(value)
	if !ok {
		return semanticTreatmentPCAInput{}, fmt.Errorf("processor control requirement is unavailable")
	}
	return semanticTreatmentPCAInput{Family: requirement.ProcessorFamily}, nil
}

func qualifySemanticTreatmentSurfaces(surfaces []semanticProcessorSurface, digest plugingrabber.ParameterDigest, refIdentity processorattestation.Subject, input semanticTreatmentPCAInput) []semanticProcessorSurface {
	identity := refIdentity
	if identity.Name == "" {
		identity.Name = digest.PluginName
	}
	if identity.Identifier == "" {
		identity.Identifier = digest.PluginIdentifier
	}
	if identity.InstalledPath == "" {
		identity.InstalledPath = digest.PluginPath
	}
	if identity.Format == "" {
		identity.Format = digest.PluginFormat
	}
	if identity.Manufacturer == "" {
		identity.Manufacturer = digest.PluginManufacturer
	}

	for index := range surfaces {
		surface := &surfaces[index]
		surface.PCAReviewed = true
		surface.PCARequiredCoverage = nil
		result := processorattestation.EligibilityResult{EffectiveStatus: "missing", Reason: "pca_identity_unresolved"}
		if input.Family == "" {
			result.Reason = "pca_family_unresolved"
		} else if surface.Family != input.Family {
			result.EffectiveStatus = "not_requested"
			result.Reason = "pca_family_not_selected"
		} else if identity.Name == "" || identity.Format == "" || (identity.Identifier == "" && identity.InstalledPath == "") {
			result.Reason = "pca_exact_identity_unresolved"
		} else {
			queried, err := processorattestation.QueryInstalledAdmission(processorattestation.InstalledSubject{Subject: identity}, input.Family)
			if err != nil {
				result.Reason = "pca_query_failed: " + err.Error()
			} else {
				result = queried
			}
		}
		surface.PCAEligible = result.Eligible
		surface.PCAStatus = result.EffectiveStatus
		surface.PCAReason = result.Reason
		surface.PCASubjectKey = result.SubjectKey
		surface.PCABinaryFingerprint = result.BinaryFingerprint
		surface.PCAAttestationID = result.AttestationID
		if !result.Eligible {
			surface.InspectOnly = true
			surface.Limitation = appendSemanticTreatmentLimitation(surface.Limitation, "PCA admission: "+result.Reason)
		}
	}
	return surfaces
}

func appendSemanticTreatmentLimitation(existing, addition string) string {
	existing = strings.TrimSpace(existing)
	addition = strings.TrimSpace(addition)
	if addition == "" {
		return existing
	}
	if existing == "" {
		return addition
	}
	if strings.Contains(existing, addition) {
		return existing
	}
	return existing + "; " + addition
}

func semanticTreatmentSurfacePCAEligible(surface semanticProcessorSurface) bool {
	if surface.QualificationStatus == "ownership_conflict" || surface.InspectOnly {
		return false
	}
	if surface.PCAReviewed {
		return surface.PCAEligible
	}
	return true
}

// semanticLoadedInstancePCAAdmission is the shared loaded-instance boundary
// used by processor-specific planners. It deliberately re-reads the live
// parameter surface and exact project identity so a recognizer result cannot
// be reused after a fingerprint, topology, or coverage change.
func (s *Server) semanticLoadedInstancePCAAdmission(ctx context.Context, trackID, pluginID string, input semanticTreatmentPCAInput) (semanticProcessorSurface, error) {
	return s.semanticLoadedInstancePCAAdmissionWithReceipt(ctx, trackID, pluginID, input, nil)
}

// semanticLoadedInstancePCAAdmissionForContext keeps an exact post-load PCA
// receipt with a capability-owned leaf through its separate confirmation. The
// receipt authenticates the installed binary; the normal admission path still
// reads live topology and controller ownership at the moment of use.
func (s *Server) semanticLoadedInstancePCAAdmissionForContext(ctx context.Context, requestContext map[string]any, trackID, pluginID string, input semanticTreatmentPCAInput) (semanticProcessorSurface, error) {
	receipt, found, err := semanticPCAAdmissionReceiptFromContext(requestContext)
	if err != nil {
		return semanticProcessorSurface{}, fmt.Errorf("pca_rejected:%w", err)
	}
	if found {
		return s.semanticLoadedInstancePCAAdmissionWithReceipt(ctx, trackID, pluginID, input, &receipt)
	}
	return s.semanticLoadedInstancePCAAdmission(ctx, trackID, pluginID, input)
}

// semanticLoadedInstancePCAAdmissionWithReceipt keeps PCA subject identity
// separate from a loaded rack node.  When a post-load receipt is present it
// is revalidated against the installed binary, while the live rack digest is
// used only to prove the current topology and controller surface.
func (s *Server) semanticLoadedInstancePCAAdmissionWithReceipt(ctx context.Context, trackID, pluginID string, input semanticTreatmentPCAInput, receipt *semanticPCAAdmissionReceipt) (semanticProcessorSurface, error) {
	if s == nil || strings.TrimSpace(trackID) == "" || strings.TrimSpace(pluginID) == "" {
		return semanticProcessorSurface{}, fmt.Errorf("pca_exact_target_unresolved")
	}
	if receipt != nil {
		if err := semanticValidatePCAAdmissionReceiptForInput(*receipt, input); err != nil {
			return semanticProcessorSurface{}, fmt.Errorf("pca_rejected:%w", err)
		}
	}
	instances, _ := s.semanticTreatmentInstances(ctx, trackID, input)
	instance, ok := semanticTreatmentFindPlugin(instances, pluginID)
	if !ok {
		return semanticProcessorSurface{}, fmt.Errorf("pca_exact_target_unresolved")
	}
	surface, ok := semanticTreatmentInstanceSurface(instance, input.Family)
	if !ok {
		return semanticProcessorSurface{}, fmt.Errorf("pca_family_not_proven:%s", input.Family)
	}
	if receipt != nil {
		if surface.QualificationStatus == "ownership_conflict" {
			resolved, conflictErr := semanticPCAReceiptResolvesOwnershipConflict(instance.QualifiedSurfaces, input.Family, *receipt)
			if conflictErr != nil {
				return surface, fmt.Errorf("pca_rejected:ownership admission check: %w", conflictErr)
			}
			if !resolved {
				return surface, fmt.Errorf("pca_rejected:%s", firstNonEmpty(surface.Limitation, "live topology is not executable"))
			}
			surface.QualificationStatus = "pca_receipt_bound_topology_qualified"
			surface.Limitation = ""
		}
		// semanticTreatmentInstances had no exact live identity, so its normal
		// qualification marked this otherwise-proven surface inspect-only. The
		// revalidated receipt supplies only that missing identity proof; topology
		// and ownership are still the freshly read live facts above.
		surface.InspectOnly = false
		surface.Limitation = strings.Trim(strings.ReplaceAll(surface.Limitation, "PCA admission: pca_exact_identity_unresolved", ""), " ;")
		surface.PCAReviewed = true
		surface.PCAEligible = true
		surface.PCAStatus = "promoted"
		surface.PCAReason = "admission_receipt_current"
		surface.PCASubjectKey = receipt.SubjectKey
		surface.PCABinaryFingerprint = receipt.BinaryFingerprint
		surface.PCAAttestationID = receipt.AttestationID
		return surface, nil
	}
	if !surface.PCAReviewed || !semanticTreatmentSurfacePCAEligible(surface) {
		return surface, fmt.Errorf("pca_rejected:%s", firstNonEmpty(surface.PCAReason, surface.Limitation, surface.PCAStatus))
	}
	return surface, nil
}

// semanticRelayPCAAdmissionReceiptToContext relays a load plan's validated PCA
// admission receipt into the post-load request context, so the downstream
// semantic admission re-check sees the same accompanied receipt that was bound
// to the recommended candidate before the load. The load-result boundary has
// already revalidated the plan receipt against the actually loaded identifier;
// this only carries it across. Plans without a receipt relay nothing and the
// downstream no-receipt behavior is unchanged; a malformed plan receipt fails
// closed instead of entering the semantic chain.
func semanticRelayPCAAdmissionReceiptToContext(plan PendingPlan, requestContext map[string]any) error {
	receipt, found, err := semanticPCAAdmissionReceiptFromPlan(plan)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	requestContext["pca_admission_receipt"] = semanticPCAAdmissionReceiptMap(receipt)
	return nil
}

// pcaCertifiedAxisNarrowingSchema versions the server-owned disclosure of one
// deterministic narrowing of planner-selected semantic axes to the certified
// coverage of an accompanied PCA admission receipt.
const pcaCertifiedAxisNarrowingSchema = "pca_certified_axis_narrowing.v1"

type pcaCertifiedAxisNarrowing struct {
	SchemaVersion string   `json:"schema_version"`
	AttestationID string   `json:"attestation_id"`
	CertifiedAxes []string `json:"certified_axes"`
	KeptAxes      []string `json:"kept_axes"`
	ExcludedAxes  []string `json:"excluded_axes"`
}

// narrowCompressorIntentAxesToLoadedReceipt bounds a planner-owned semantic
// axis selection to the certified coverage of the accompanied admission
// receipt — the same promoted attestation the planning admission revalidates.
// A well-formed receipt is the only trigger: without one (or with a malformed
// one) the path is unchanged and the loaded-instance admission keeps its exact
// fail-closed behavior. An empty intersection is a named failure, never a
// silent empty plan.
func narrowCompressorIntentAxesToLoadedReceipt(requestContext map[string]any, selected []string) (*pcaCertifiedAxisNarrowing, bool, error) {
	receipt, found, err := semanticPCAAdmissionReceiptFromContext(requestContext)
	if err != nil || !found {
		return nil, false, nil
	}
	certified, err := processorattestation.PromotedAttestationCoverageAxes(receipt.AttestationID)
	if err != nil {
		return nil, false, fmt.Errorf("pca_certified_axes_empty: certified coverage lookup failed: %w", err)
	}
	certifiedSet := map[string]bool{}
	for _, axis := range certified {
		certifiedSet[axis] = true
	}
	narrowing := &pcaCertifiedAxisNarrowing{SchemaVersion: pcaCertifiedAxisNarrowingSchema,
		AttestationID: receipt.AttestationID, CertifiedAxes: certified}
	for _, axis := range selected {
		axis = strings.ToLower(strings.TrimSpace(axis))
		if axis == "" {
			continue
		}
		if certifiedSet[axis] {
			narrowing.KeptAxes = append(narrowing.KeptAxes, axis)
		} else {
			narrowing.ExcludedAxes = append(narrowing.ExcludedAxes, axis)
		}
	}
	if len(narrowing.KeptAxes) == 0 {
		return narrowing, true, fmt.Errorf("pca_certified_axes_empty: loaded instance attestation %s certifies [%s], none of the planned axes [%s]",
			receipt.AttestationID, pcaAxisListLabel(certified), pcaAxisListLabel(selected))
	}
	return narrowing, true, nil
}

func pcaAxisListLabel(axes []string) string {
	if len(axes) == 0 {
		return "none"
	}
	return strings.Join(axes, ",")
}

func semanticTreatmentInstancePCAEligible(instance semanticTreatmentInstance, family string) bool {
	if len(instance.QualifiedSurfaces) == 0 {
		return !instance.PCAReviewed
	}
	for _, surface := range instance.QualifiedSurfaces {
		if surface.Family == family {
			return semanticTreatmentSurfacePCAEligible(surface)
		}
	}
	return false
}

func semanticTreatmentPCARequirementForFamily(registry *processorregistry.Registry, family string, axes []string) ([]processorattestation.Coverage, error) {
	if registry == nil {
		return nil, fmt.Errorf("processor registry unavailable")
	}
	if strings.EqualFold(strings.TrimSpace(family), processorintent.FamilySpectralDynamics) || strings.EqualFold(strings.TrimSpace(family), processorintent.FamilyClipper) {
		return nil, fmt.Errorf("inspect-only family cannot enter PCA executable admission")
	}
	return registry.PCARequiredCoverage(family, axes)
}

func semanticPostLoadAdapterForProcessorType(processorType string) (family, planner string, ok bool) {
	family = processorAttestationFamily(processorType)
	if family == "" {
		return "", "", false
	}
	registry, err := processorregistry.Default()
	if err != nil {
		return "", "", false
	}
	definition, exists := registry.Resolve(family)
	if !exists || definition.InspectOnly || definition.Planner == "" || definition.Planner == "inspect_only" {
		return "", "", false
	}
	return family, definition.Planner, true
}

func semanticPostLoadPCAQualification(plan PendingPlan, digest plugingrabber.ParameterDigest, family string, trackID, pluginID, pluginName string) (semanticProcessorSurface, error) {
	family = strings.ToLower(strings.TrimSpace(family))
	if family == "" {
		return semanticProcessorSurface{}, fmt.Errorf("post-load PCA family is missing")
	}
	requestContext := cloneContext(plan.Context)
	if nested := firstMapFromAny(plan.WorkflowData["semantic_post_load_request_context"]); len(nested) > 0 {
		requestContext = mergeContext(requestContext, nested)
	}
	if nested := firstMapFromAny(plan.WorkflowData["semantic_eq_post_load_request_context"]); len(nested) > 0 {
		requestContext = mergeContext(requestContext, nested)
	}
	if nested := firstMapFromAny(plan.WorkflowData["semantic_compressor_post_load_request_context"]); len(nested) > 0 {
		requestContext = mergeContext(requestContext, nested)
	}
	var input semanticTreatmentPCAInput
	var err error
	if intent := firstMapFromAny(plan.WorkflowData["semantic_post_load_semantic_processor_intent"]); len(intent) > 0 {
		input, err = semanticTreatmentPCAInputFromIntent(intent)
	} else if intent := firstMapFromAny(requestContext["free_state_semantic_processor_intent"]); len(intent) > 0 {
		input, err = semanticTreatmentPCAInputFromIntent(intent)
	} else if requirement := firstMapFromAny(requestContext["processor_control_requirement"]); len(requirement) > 0 {
		input, err = semanticTreatmentPCAInputFromRequirement(requirement)
	} else if requirement := firstMapFromAny(firstMapFromAny(requestContext["plugin_recommendation"])["processor_control_requirement"]); len(requirement) > 0 {
		input, err = semanticTreatmentPCAInputFromRequirement(requirement)
	} else if admittedFamily := firstStringFromMap(requestContext, "semantic_post_load_family", "post_load_family"); admittedFamily != "" {
		input = semanticTreatmentPCAInput{Family: admittedFamily}
	} else {
		err = fmt.Errorf("post-load PCA family is unavailable")
	}
	if err != nil {
		return semanticProcessorSurface{}, err
	}
	if input.Family != family {
		return semanticProcessorSurface{}, fmt.Errorf("post-load PCA family mismatch: %s != %s", input.Family, family)
	}
	receipt, hasReceipt, receiptErr := semanticValidatePCAAdmissionReceipt(plan, family, "")
	if receiptErr != nil {
		return semanticProcessorSurface{}, receiptErr
	}
	candidate := firstMapFromAny(requestContext["semantic_plugin_recommendation_candidate"])
	if len(candidate) == 0 {
		candidate = firstMapFromAny(plan.WorkflowData["selected_candidate"])
	}
	identity := processorattestation.Subject{
		Name:          firstNonEmpty(firstStringFromMap(candidate, "name", "plugin_name"), digest.PluginName, pluginName),
		Manufacturer:  firstNonEmpty(firstStringFromMap(candidate, "manufacturer", "vendor"), digest.PluginManufacturer),
		Format:        firstNonEmpty(firstStringFromMap(candidate, "format"), digest.PluginFormat),
		Identifier:    firstNonEmpty(firstStringFromMap(candidate, "identifier", "plugin_identifier"), digest.PluginIdentifier),
		InstalledPath: firstNonEmpty(firstStringFromMap(candidate, "plugin_path", "path"), digest.PluginPath),
	}
	if hasReceipt {
		identity = processorattestation.Subject{
			Name: receipt.Name, Manufacturer: receipt.Manufacturer, Format: receipt.Format,
			Identifier: receipt.Identifier, InstalledPath: receipt.PluginPath,
		}
	}
	if identity.Name == "" || identity.Format == "" || (identity.Identifier == "" && identity.InstalledPath == "") {
		return semanticProcessorSurface{}, fmt.Errorf("post-load exact plugin identity is unavailable")
	}
	if digest.TrackID == "" {
		digest.TrackID = trackID
	}
	if digest.PluginID == "" {
		digest.PluginID = pluginID
	}
	if digest.PluginName == "" {
		digest.PluginName = identity.Name
	}
	surfaces, boundary := semanticTreatmentBuildSurfaces(trackID, pluginID, digest)
	if len(surfaces) == 0 {
		return semanticProcessorSurface{}, fmt.Errorf("post-load topology did not expose a governed processor surface: %s", firstNonEmpty(boundary, "no surface"))
	}
	if hasReceipt {
		// Admission was revalidated above against the exact selected binary.
		// Qualify only the freshly observed parameter surface below; a loaded
		// numeric plugin ID must not be used as a PCA subject identity.
		for index := range surfaces {
			surface := &surfaces[index]
			surface.PCAReviewed = true
			surface.PCAEligible = surface.Family == family
			surface.PCAStatus = "promoted"
			surface.PCAReason = "admission_receipt_current"
			surface.PCASubjectKey = receipt.SubjectKey
			surface.PCABinaryFingerprint = receipt.BinaryFingerprint
			surface.PCAAttestationID = receipt.AttestationID
		}
	} else {
		surfaces = qualifySemanticTreatmentSurfaces(surfaces, digest, identity, input)
	}
	for _, surface := range surfaces {
		if surface.Family != family {
			continue
		}
		if !surface.PCAReviewed || !surface.PCAEligible {
			return semanticProcessorSurface{}, fmt.Errorf("post-load PCA rejected %s: %s", family, firstNonEmpty(surface.PCAReason, surface.PCAStatus))
		}
		if surface.QualificationStatus == "ownership_conflict" {
			if !hasReceipt {
				return semanticProcessorSurface{}, fmt.Errorf("post-load topology ownership conflict for %s: %s", family, surface.Limitation)
			}
			resolved, conflictErr := semanticPCAReceiptResolvesOwnershipConflict(surfaces, family, receipt)
			if conflictErr != nil {
				return semanticProcessorSurface{}, fmt.Errorf("post-load ownership admission check for %s: %w", family, conflictErr)
			}
			if !resolved {
				return semanticProcessorSurface{}, fmt.Errorf("post-load topology ownership conflict for %s: %s", family, surface.Limitation)
			}
			surface.QualificationStatus = "pca_receipt_bound_topology_qualified"
			surface.Limitation = ""
		}
		if surface.InspectOnly {
			return semanticProcessorSurface{}, fmt.Errorf("post-load topology is inspect-only for %s: %s", family, surface.Limitation)
		}
		return surface, nil
	}
	return semanticProcessorSurface{}, fmt.Errorf("post-load topology family %s was not proven", family)
}

// semanticPCAReceiptResolvesOwnershipConflict keeps recognizer conflicts
// fail-closed when two PCA-admitted capability families claim the same live
// controller. A non-admitted recognizer surface cannot veto the exact family
// carried by the load receipt: PCA is the authority for executable control,
// while the live surface still proves that the selected family exists.
func semanticPCAReceiptResolvesOwnershipConflict(surfaces []semanticProcessorSurface, family string, receipt semanticPCAAdmissionReceipt) (bool, error) {
	family = strings.ToLower(strings.TrimSpace(family))
	var target *semanticProcessorSurface
	for index := range surfaces {
		if surfaces[index].Family == family {
			target = &surfaces[index]
			break
		}
	}
	if target == nil || target.QualificationStatus != "ownership_conflict" || len(target.OwnedParameterIDs) == 0 {
		return false, nil
	}
	targetIDs := map[string]bool{}
	for _, parameterID := range target.OwnedParameterIDs {
		if parameterID = strings.TrimSpace(parameterID); parameterID != "" {
			targetIDs[parameterID] = true
		}
	}
	if len(targetIDs) == 0 {
		return false, nil
	}
	subject := processorattestation.InstalledSubject{Subject: processorattestation.Subject{
		Name: receipt.Name, Manufacturer: receipt.Manufacturer, Format: receipt.Format,
		Identifier: receipt.Identifier, InstalledPath: receipt.PluginPath,
	}}
	for _, candidate := range surfaces {
		if candidate.Family == family || !semanticSurfaceSharesOwnedParameter(targetIDs, candidate) {
			continue
		}
		admission, err := processorattestation.QueryInstalledAdmission(subject, candidate.Family)
		if err != nil {
			return false, err
		}
		if admission.Eligible {
			return false, nil
		}
	}
	return true, nil
}

func semanticSurfaceSharesOwnedParameter(targetIDs map[string]bool, surface semanticProcessorSurface) bool {
	for _, parameterID := range surface.OwnedParameterIDs {
		if targetIDs[strings.TrimSpace(parameterID)] {
			return true
		}
	}
	return false
}
