package agentloop

import "strings"

func messageLoopConservativeHeadroomTreatmentPending(state *runState, reply string) *MixTreatmentPending {
	if state == nil || strings.TrimSpace(reply) == "" {
		return nil
	}
	if messageLoopMutationBarrierActive(state) || state.executionMemory.PendingMixTreatment != nil || state.executionMemory.PendingMixTickCandidate != nil {
		return nil
	}
	if messageLoopFreeStateRoundPendingSettlement(state) {
		return nil
	}
	if !messageLoopNaturalMixRequest(state.input.UserText) || !messageLoopHasUsableMixObservation(state) {
		return nil
	}
	if messageLoopLastMixObservationDeepPackageIncomplete(state) || messageLoopReplyMentionsIncompleteDeepPackage(reply) {
		return nil
	}
	trackID, risk := messageLoopHeadroomRiskTrack(state)
	trackID = messageLoopCanonicalMixTrackID(state, trackID)
	if trackID == "" {
		return nil
	}
	delta := messageLoopConservativeHeadroomDelta(risk)
	if delta == 0 {
		return nil
	}
	observationID := messageLoopLastMixObservationField(state, "observation_id")
	treatment := &MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            messageLoopConversationID(state),
		ObservationID:             observationID,
		Intent:                    strings.TrimSpace(state.input.UserText),
		TargetRef:                 "track:" + trackID,
		ActionKind:                "gain_balance",
		ProcessorType:             "utility",
		DeltaDB:                   delta,
		Target:                    map[string]any{"delta_db": delta},
		ReasoningSummary:          "conservative gain trim from materialized project headroom risk",
		Confidence:                "medium",
		ExpiresAfterContextChange: true,
		CreatedFromReply:          reply,
		Fingerprint: map[string]any{
			"conversation_id":   messageLoopConversationID(state),
			"goal_id":           state.goal.GoalID,
			"run_id":            state.goal.RunID,
			"observation_id":    observationID,
			"target_scope":      messageLoopLastMixObservationScope(state),
			"target_track_id":   trackID,
			"track_count":       messageLoopPendingMixCandidateTrackCount(state),
			"created_from_turn": state.turnsUsed,
			"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
			"source":            "materialized_headroom_risk_after_observation",
			"risk":              cloneMap(risk),
		},
	}
	if observationID != "" {
		treatment.EvidenceRefs = []string{observationID, "project.risks.headroom", "project.attention.first"}
	}
	if row := messageLoopPendingMixCandidateTrackRow(state, trackID); len(row) > 0 {
		treatment.Fingerprint["before_track"] = cloneMap(row)
		for _, key := range []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"} {
			if value, ok := row[key]; ok && value != nil {
				treatment.Fingerprint[key] = value
			}
		}
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	return treatment
}

func messageLoopHeadroomRiskTrack(state *runState) (string, map[string]any) {
	var candidates []map[string]any
	addRisk := func(row map[string]any) {
		if len(row) == 0 {
			return
		}
		if nested := messageLoopMapValue(row["track"]); len(nested) > 0 {
			merged := cloneMap(nested)
			for key, value := range row {
				if key == "track" || messageLoopEmptyValue(value) {
					continue
				}
				if _, exists := merged[key]; !exists {
					merged[key] = value
				}
			}
			row = merged
		}
		if firstMapText(row, "track_id", "id", "target_track_id") == "" {
			return
		}
		if !messageLoopHeadroomRiskHigh(row) {
			return
		}
		candidates = append(candidates, cloneMap(row))
	}
	addRisks := func(value any) {
		if row := messageLoopMapValue(value); len(row) > 0 {
			addRisk(row)
		}
		for _, row := range messageLoopMapRows(value) {
			addRisk(row)
		}
	}
	for _, row := range messageLoopPendingMixCandidateLikelyAttentionRows(state) {
		addRisk(row)
	}
	for _, row := range messageLoopPendingMixCandidateTrackRows(state) {
		addRisk(row)
	}
	if state != nil && state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		summary := state.recentObservation.Summary
		addRisks(messageLoopMapValue(summary["digest"])["project_headroom_risk_excerpt"])
		addRisks(messageLoopMapValue(summary["acoustic_digest"])["project_headroom_risk_excerpt"])
	}
	if state != nil {
		for i := len(state.executed) - 1; i >= 0; i-- {
			result := messageLoopMapValue(state.executed[i]["result"])
			if len(result) == 0 {
				continue
			}
			addRisks(messageLoopMapValue(result["digest"])["project_headroom_risk_excerpt"])
			addRisks(messageLoopMapValue(result["acoustic_digest"])["project_headroom_risk_excerpt"])
			observation := messageLoopObservationFromResult(result)
			projectPackage := messageLoopMapValue(observation["project_package"])
			addRisks(projectPackage["headroom_risk"])
			addRisks(projectPackage["likely_first_attention_target"])
			for _, entry := range messageLoopMapRows(result["entries"]) {
				value := messageLoopMapValue(entry["value"])
				addRisks(value["headroom_risk"])
				addRisks(value["likely_first_attention_target"])
				if strings.EqualFold(firstMapText(entry, "key"), "project.risks.headroom") {
					addRisks(value["rows"])
				}
				if strings.EqualFold(firstMapText(entry, "key"), "project.attention.first") {
					addRisks(entry["value"])
				}
			}
			for key, item := range messageLoopMapValue(result["items"]) {
				value := messageLoopMapValue(item)
				addRisks(value["headroom_risk"])
				addRisks(value["likely_first_attention_target"])
				if strings.EqualFold(strings.TrimSpace(key), "project.risks.headroom") {
					addRisks(value["rows"])
				}
				if strings.EqualFold(strings.TrimSpace(key), "project.attention.first") {
					addRisks(item)
				}
			}
		}
	}
	var best map[string]any
	bestHeadroom := 1.01
	for _, row := range candidates {
		headroom, ok := messageLoopHeadroomFromRiskRow(row)
		if !ok {
			headroom = 1.0
		}
		if best == nil || headroom < bestHeadroom {
			best = row
			bestHeadroom = headroom
		}
	}
	if len(best) == 0 {
		return "", nil
	}
	return firstMapText(best, "track_id", "id", "target_track_id"), best
}

func messageLoopHeadroomRiskHigh(row map[string]any) bool {
	if len(row) == 0 {
		return false
	}
	risk := strings.ToLower(strings.TrimSpace(firstMapText(row, "risk", "risk_level", "headroom_risk")))
	if risk == "high" || risk == "critical" || risk == "clipping" {
		return true
	}
	if headroom, ok := messageLoopHeadroomFromRiskRow(row); ok && headroom <= 1.0 {
		return true
	}
	if peak, ok := firstNumericMapValue(row, "peak_dbfs", "peak"); ok && peak >= -1.0 {
		return true
	}
	return false
}

func messageLoopHeadroomFromRiskRow(row map[string]any) (float64, bool) {
	if value, ok := firstNumericMapValue(row, "headroom_db", "headroom", "value"); ok {
		return value, true
	}
	if track := messageLoopMapValue(row["track"]); len(track) > 0 {
		if value, ok := firstNumericMapValue(track, "headroom_db", "headroom", "value"); ok {
			return value, true
		}
	}
	return 0, false
}

func messageLoopConservativeHeadroomDelta(risk map[string]any) float64 {
	headroom, hasHeadroom := messageLoopHeadroomFromRiskRow(risk)
	if !hasHeadroom {
		return -1.0
	}
	switch {
	case headroom <= 0.25:
		return -1.0
	case headroom <= 1.0:
		return -0.75
	default:
		return 0
	}
}
