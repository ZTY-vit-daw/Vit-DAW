package chat

import (
	"strings"
	"time"

	"vit-daw-agent/internal/experiment"
)

func syncD1Receipt(loop *freeStateReasoningLoop) {
	if loop == nil || loop.Experiment == nil || !loop.Experiment.Admission.IsD1S1() || len(loop.Experiment.Rounds) == 0 {
		return
	}
	round := loop.Experiment.Rounds[len(loop.Experiment.Rounds)-1]
	if len(round.Interventions) == 0 {
		return
	}
	intervention := round.Interventions[0]
	projectRevision := firstStringFromMap(intervention.Receipt, "after_revision", "applied_revision")
	parameterApplied := intervention.TechnicalApplication == experiment.TechnicalApplied
	readbackVerified, _ := intervention.Receipt["readback_verified"].(bool)
	if !readbackVerified {
		readbackVerified, _ = firstMapFromAny(intervention.Receipt["details"])["readback_verified"].(bool)
	}
	evaluationReady := false
	for _, observation := range round.Observations {
		if observation.PostAction && observation.Fresh && observation.ProjectRevision == projectRevision {
			evaluationReady = true
		}
	}

	materialityStatus := "none"
	classification := experiment.ClassificationAmbiguous
	if round.Materiality != nil {
		switch round.Materiality.State {
		case experiment.MaterialityMaterial:
			materialityStatus = "material"
		case experiment.MaterialitySubthreshold:
			materialityStatus = "subthreshold"
			classification = experiment.ClassificationSubthreshold
		}
	}
	targetStatus := "absent"
	if round.TargetResponse != nil {
		switch round.TargetResponse.Response {
		case experiment.TargetDirectional:
			targetStatus = "directional"
		case experiment.TargetSufficient:
			targetStatus = "sufficient"
		default:
			targetStatus = "ambiguous"
		}
		if materialityStatus == "material" {
			classification = experiment.ClassificationMaterial
		}
	}

	humanStatus := "not_requested"
	if round.UserJudgmentRequested {
		humanStatus = "pending"
	}
	var judgment experiment.UserJudgmentEvidence
	if len(round.UserJudgmentEvidence) > 0 {
		judgment = round.UserJudgmentEvidence[len(round.UserJudgmentEvidence)-1]
		humanStatus = "decided"
	}
	humanConfirmed := judgment.HeardDifference == experiment.HeardDifferenceYes && (judgment.Preference == experiment.PreferenceA || judgment.Preference == experiment.PreferenceB)
	ambiguous := len(round.UserJudgmentEvidence) > 0 && !humanConfirmed
	rolledBack := len(round.RollbackReceipt) > 0 || loop.Experiment.Outcome == experiment.OutcomeRolledBack
	settled := loop.Experiment.Status == experiment.StatusSettled
	disposition := experiment.DispositionRequestAudition
	netOutcome := "stable"
	if rolledBack {
		disposition = experiment.DispositionRollback
		netOutcome = "rolled_back"
	} else if humanConfirmed && auditionJudgmentSelectsPhysicalSide(judgment, round.CheckpointRef, auditionPhysicalAfter) && settled {
		disposition = experiment.DispositionRetain
		netOutcome = "improved"
	} else if classification == experiment.ClassificationSubthreshold {
		netOutcome = "plateau"
	}
	beforeRender := firstMapFromAny(loop.D1State["before_render"])
	afterRender := firstMapFromAny(loop.D1State["after_render"])
	humanAuditionReady := evaluationReady && firstStringFromMap(beforeRender, "status") == "ready" && firstStringFromMap(afterRender, "status") == "ready" &&
		(strings.EqualFold(firstStringFromMap(loop.AuditionSessionSnapshot, "status"), "ready") || humanStatus == "pending" || humanStatus == "decided")
	evidence := append([]string(nil), loop.Experiment.Admission.EvidenceRefs...)
	for _, observation := range round.Observations {
		evidence = append(evidence, observation.ID)
		evidence = append(evidence, observation.EvidenceRefs...)
	}
	evidence = append(evidence, intervention.ID)
	evidence = append(evidence, intervention.EvidenceRefs...)
	if round.Materiality != nil {
		evidence = append(evidence, round.Materiality.EvidenceRefs...)
	}
	if round.TargetResponse != nil {
		evidence = append(evidence, round.TargetResponse.EvidenceRefs...)
	}
	if judgment.ID != "" {
		evidence = append(evidence, judgment.ID)
	}
	for _, render := range []map[string]any{beforeRender, afterRender} {
		if revision := firstStringFromMap(render, "render_revision"); revision != "" {
			evidence = append(evidence, revision)
		}
	}
	receipt := experiment.ImprovementExecutionReceipt{
		SchemaVersion: experiment.ImprovementReceiptSchema, ReceiptID: "fsx_" + sanitizeCanaryID(loop.Experiment.ID), ExperimentContractRef: loop.Experiment.ID,
		ProjectRevision: projectRevision, Classification: classification, Disposition: disposition,
		Layers: experiment.ReceiptLayers{
			TechnicalReadback:   experiment.TechnicalReadbackLayer{Status: map[bool]string{true: "applied", false: "failed"}[parameterApplied]},
			AcousticMateriality: experiment.AcousticMaterialityLayer{Status: materialityStatus},
			TargetResponse:      experiment.TargetResponseLayer{Status: targetStatus}, NetOutcome: experiment.NetOutcomeLayer{Status: netOutcome}, HumanAB: experiment.HumanABLayer{Status: humanStatus},
		},
		EvidenceRefs: freeStateNormalizedViewIDs(evidence), ParameterApplied: parameterApplied, ReadbackVerified: readbackVerified,
		EvaluationReady: evaluationReady, HumanAuditionReady: humanAuditionReady, HumanConfirmed: humanConfirmed, Ambiguous: ambiguous,
		RolledBack: rolledBack, Settled: settled, RecordedAt: time.Now().UTC(),
	}
	projection := structMap(receipt)
	if err := receipt.Validate(); err != nil {
		projection["validation_error"] = err.Error()
	}
	// The receipt projection must state the admitted domain, not the legacy
	// track_gain constant: D2-1 static_eq rounds would otherwise surface a
	// wrong-domain D1 receipt (observed in the 2026-08-27 09:03 smoke run).
	projection["action_domain"] = firstNonEmpty(firstStringFromMap(loop.Experiment.Admission.TypedAction, "action_domain", "domain"), experiment.D1S1ActionDomain)
	projection["action_kind"] = firstNonEmpty(firstStringFromMap(loop.Experiment.Admission.TypedAction, "action_kind", "kind"), experiment.D1S1ActionKind)
	// The D2-2 tier counts forward mutations across the whole experiment (one
	// per round); the multi-round probe asserts the count equals the round
	// count. The single-round expression stays byte-for-byte.
	if loop.Experiment.Admission.IsD2MultiRound() {
		projection["forward_mutation_count"] = loop.Experiment.InterventionCount()
	} else {
		projection["forward_mutation_count"] = len(round.Interventions)
	}
	projection["rollback_compensation_count"] = map[bool]int{true: 1, false: 0}[rolledBack]
	projection["before_render"] = cloneContext(beforeRender)
	projection["after_render"] = cloneContext(afterRender)
	projection["rollback_receipt"] = cloneContext(round.RollbackReceipt)
	loop.D1Receipt = projection
}
