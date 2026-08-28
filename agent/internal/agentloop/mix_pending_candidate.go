package agentloop

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var messageLoopDBAdjustmentPattern = regexp.MustCompile(`(?i)(降低|降|下调|调低|减少|减|提高|提升|上调|调高|增加|加|raise|boost|increase|up|higher|lower|reduce|decrease|down|cut)[^0-9+\-]{0,24}([+\-]?\d+(?:\.\d+)?)\s*(?:dB|db|分贝)`)

func messageLoopPendingMixTickCandidateFromReply(state *runState, reply string) *PendingMixTickCandidate {
	if state == nil || strings.TrimSpace(reply) == "" {
		return nil
	}
	if messageLoopMutationBarrierActive(state) {
		return nil
	}
	if messageLoopFreeStateRoundPendingSettlement(state) {
		// The applied experiment round already spent its single mutation
		// budget and is waiting for the structured settle report. Converting a
		// settle-turn reply into another pending mix tick parks the newest
		// continuation at waiting_interaction and the round never settles
		// (2026-08-28 121306→130901 smokes: every failed run wedged exactly
		// here). The reply must be answered by the final-gate settle feedback,
		// not by a second-mutation suggestion.
		return nil
	}
	delta, evidenceText, ok := messageLoopExtractSingleGainDelta(reply)
	if !ok || delta == 0 || math.Abs(delta) > 2 {
		return nil
	}
	trackID := messageLoopPendingMixCandidateTrackID(state, reply, evidenceText)
	if trackID == "" {
		return nil
	}
	observationID := ""
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		observationID = firstMapText(state.recentObservation.Summary, "observation_id")
	}
	if observationID == "" {
		observationID = messageLoopLastMixObservationField(state, "observation_id")
	}
	fingerprint := map[string]any{
		"conversation_id":   messageLoopConversationID(state),
		"goal_id":           state.goal.GoalID,
		"run_id":            state.goal.RunID,
		"target_track_id":   trackID,
		"observation_id":    observationID,
		"target_scope":      messageLoopLastMixObservationScope(state),
		"track_gain_db":     nil,
		"track_count":       messageLoopPendingMixCandidateTrackCount(state),
		"created_from_turn": state.turnsUsed,
		"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
	}
	if track := messageLoopPendingMixCandidateTrackRow(state, trackID); len(track) > 0 {
		fingerprint["before_track"] = track
		for _, key := range []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"} {
			if value, ok := track[key]; ok && value != nil {
				fingerprint[key] = value
			}
		}
	}
	if currentDB, ok := messageLoopPendingMixCandidateCurrentTrackDB(state, trackID); ok {
		fingerprint["track_gain_db"] = currentDB
	}
	return &PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   trackID,
		DeltaDB:                   delta,
		ObservationID:             observationID,
		Evidence:                  map[string]any{"matched_text": evidenceText, "source": "assistant_reply"},
		CreatedFromReply:          reply,
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               fingerprint,
	}
}

func messageLoopDeterministicVocalClarificationPendingTick(state *runState, reply string) *PendingMixTickCandidate {
	if state == nil || messageLoopMutationBarrierActive(state) || messageLoopHasPendingMixAction(state) {
		return nil
	}
	if messageLoopFreeStateRoundPendingSettlement(state) {
		// Same mid-round second-mutation refusal as the reply-synthesized tick.
		return nil
	}
	if !messageLoopUserExplicitlyIdentifiesVocalTrackResolved(state.input.UserText) {
		return nil
	}
	if !messageLoopFocusRelationshipIntent(state.input.UserText) && !messageLoopConversationHasFocusRelationshipIntent(state) {
		return nil
	}
	if !messageLoopHasUsableMixObservation(state) {
		return nil
	}
	userTrackIndex, ok := messageLoopUserTrackIndexFromText(state.input.UserText)
	if !ok || userTrackIndex <= 0 {
		return nil
	}
	result, relationship := messageLoopLatestFocusProjectRelationship(state)
	if len(relationship) == 0 {
		return nil
	}
	allRows := messageLoopPendingMixCandidateTrackRows(state)
	focusID, focusRow := messageLoopVocalClarificationFocusTrack(state, relationship, allRows, userTrackIndex)
	if focusID == "" {
		return nil
	}
	targetRow := messageLoopVocalClarificationTargetTrack(relationship, allRows, focusID)
	targetID := firstMapText(targetRow, "track_id", "id", "target_track_id")
	if targetID == "" || strings.EqualFold(targetID, focusID) {
		return nil
	}
	observationID := firstNonEmpty(firstMapText(relationship, "observation_id"), firstMapText(result, "observation_id"), messageLoopLastMixObservationField(state, "observation_id"))
	mixSessionID := firstNonEmpty(firstMapText(relationship, "mix_session_id"), firstMapText(result, "mix_session_id"), messageLoopLastMixObservationField(state, "mix_session_id"))
	fingerprint := map[string]any{
		"conversation_id":   messageLoopConversationID(state),
		"goal_id":           state.goal.GoalID,
		"run_id":            state.goal.RunID,
		"target_track_id":   targetID,
		"focus_track_id":    focusID,
		"observation_id":    observationID,
		"target_scope":      messageLoopLastMixObservationScope(state),
		"track_gain_db":     nil,
		"track_count":       messageLoopPendingMixCandidateTrackCount(state),
		"created_from_turn": state.turnsUsed,
		"mix_session_id":    mixSessionID,
	}
	if len(targetRow) > 0 {
		fingerprint["before_track"] = cloneMap(targetRow)
		for _, key := range []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"} {
			if value, ok := targetRow[key]; ok && value != nil {
				fingerprint[key] = value
			}
		}
	}
	if len(focusRow) > 0 {
		fingerprint["focus_track"] = cloneMap(focusRow)
	}
	if currentDB, ok := messageLoopPendingMixCandidateCurrentTrackDB(state, targetID); ok {
		fingerprint["track_gain_db"] = currentDB
	}
	return &PendingMixTickCandidate{
		Operation:     "track_gain_adjust",
		TrackID:       targetID,
		DeltaDB:       -1,
		ObservationID: observationID,
		Evidence: map[string]any{
			"source":           "deterministic_vocal_clarification",
			"relationship":     "focus_vs_project",
			"focus_track_id":   focusID,
			"target_track_id":  targetID,
			"user_track_index": userTrackIndex,
		},
		CreatedFromReply:          strings.TrimSpace(reply),
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               fingerprint,
	}
}

func messageLoopDeterministicVocalClarificationPendingReply(state *runState, candidate *PendingMixTickCandidate) string {
	if candidate == nil {
		return ""
	}
	targetLabel := messageLoopTrackReplyLabel(messageLoopPendingMixCandidateTrackRow(state, candidate.TrackID), candidate.TrackID)
	focusID := firstMapText(candidate.Evidence, "focus_track_id")
	focusLabel := messageLoopTrackReplyLabel(messageLoopPendingMixCandidateTrackRow(state, focusID), focusID)
	if focusLabel == "" {
		focusLabel = "Track"
	}
	if targetLabel == "" {
		targetLabel = "Track " + strings.TrimSpace(candidate.TrackID)
	}
	return fmt.Sprintf("\u5df2\u786e\u8ba4 %s \u662f\u4e3b\u5531\u3002\u57fa\u4e8e\u8fd9\u6b21\u5173\u7cfb\u89c2\u5bdf\uff0c\u6211\u5efa\u8bae\u5148\u628a %s \u5c0f\u5e45\u964d\u4f4e 1.0 dB\uff0c\u8ba9\u4e3b\u5531\u76f8\u5bf9\u66f4\u9760\u524d\uff0c\u540c\u65f6\u7ed9\u6574\u4f53\u5cf0\u503c\u7559\u51fa\u4e00\u70b9\u4f59\u91cf\u3002\u8981\u6211\u7ee7\u7eed\u6267\u884c\u8fd9\u4e2a\u5f85\u786e\u8ba4\u7684\u5c0f\u52a8\u4f5c\u5417\uff1f", focusLabel, targetLabel)
}

func messageLoopLatestFocusProjectRelationship(state *runState) (map[string]any, map[string]any) {
	if state == nil {
		return nil, nil
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
		if name != "" && name != "mix.derive" && name != "mix_derive" {
			continue
		}
		result := messageLoopMapValue(record["result"])
		if len(result) == 0 {
			continue
		}
		if relationship := messageLoopFocusProjectRelationshipFromResult(result); len(relationship) > 0 {
			return result, relationship
		}
	}
	for i := len(state.trace) - 1; i >= 0; i-- {
		event := state.trace[i]
		if event.ToolResult == nil || toolStatusFailed(event.ToolResult.Status) {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(event.ToolResult.Tool))
		if name != "" && name != "mix.derive" && name != "mix_derive" {
			continue
		}
		result := event.ToolResult.Result
		if len(result) == 0 {
			continue
		}
		if relationship := messageLoopFocusProjectRelationshipFromResult(result); len(relationship) > 0 {
			return result, relationship
		}
	}
	return nil, nil
}

func messageLoopFocusProjectRelationshipFromResult(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	relationship := messageLoopMapValue(result["relationship"])
	if len(relationship) == 0 {
		relationship = result
	}
	relationshipType := firstNonEmpty(firstMapText(relationship, "type", "relationship_type"), firstMapText(result, "type", "relationship_type"))
	if strings.EqualFold(relationshipType, "focus_vs_project") {
		return relationship
	}
	if strings.Contains(strings.ToLower(firstMapText(relationship, "relationship_id", "id")), "focus_vs_project") {
		return relationship
	}
	return nil
}

func messageLoopVocalClarificationFocusTrack(state *runState, relationship map[string]any, allRows []map[string]any, userTrackIndex int) (string, map[string]any) {
	if userTrackIndex <= 0 {
		return "", nil
	}
	if row := messageLoopTrackRowByUserIndex(allRows, userTrackIndex); len(row) > 0 {
		return firstMapText(row, "track_id", "id", "target_track_id"), row
	}
	for _, row := range messageLoopRelationshipTrackRows(relationship) {
		if messageLoopTrackRowMatchesUserIndex(row, userTrackIndex) {
			return firstMapText(row, "track_id", "id", "target_track_id"), row
		}
	}
	for _, row := range []map[string]any{
		messageLoopMapValue(relationship["focus_track"]),
		messageLoopMapValue(messageLoopMapValue(relationship["facts"])["focus_track"]),
	} {
		if id := firstMapText(row, "track_id", "id", "target_track_id"); id != "" && messageLoopTrackRowMatchesUserIndex(row, userTrackIndex) {
			return id, row
		}
	}
	if state != nil {
		for _, row := range messageLoopPendingMixCandidateTrackRows(state) {
			if messageLoopTrackRowMatchesUserIndex(row, userTrackIndex) {
				return firstMapText(row, "track_id", "id", "target_track_id"), row
			}
		}
	}
	return "", nil
}

func messageLoopVocalClarificationTargetTrack(relationship map[string]any, allRows []map[string]any, focusID string) map[string]any {
	focusID = strings.TrimSpace(focusID)
	if focusID == "" {
		return nil
	}
	facts := messageLoopMapValue(relationship["facts"])
	for _, row := range []map[string]any{
		messageLoopMapValue(relationship["peer_track"]),
		messageLoopMapValue(facts["peer_track"]),
	} {
		if id := firstMapText(row, "track_id", "id", "target_track_id"); id != "" && !strings.EqualFold(id, focusID) {
			return row
		}
	}
	rows := append(messageLoopRelationshipTrackRows(relationship), allRows...)
	var best map[string]any
	bestScore := -1
	for _, row := range rows {
		id := firstMapText(row, "track_id", "id", "target_track_id")
		if id == "" || strings.EqualFold(id, focusID) {
			continue
		}
		score := messageLoopVocalClarificationTargetScore(row)
		if best == nil || score > bestScore {
			best = row
			bestScore = score
		}
	}
	return best
}

func messageLoopRelationshipTrackRows(relationship map[string]any) []map[string]any {
	if len(relationship) == 0 {
		return nil
	}
	var rows []map[string]any
	addRows := func(values ...any) {
		for _, value := range values {
			if row := messageLoopMapValue(value); len(row) > 0 {
				rows = append(rows, row)
			}
			rows = append(rows, messageLoopMapRows(value)...)
		}
	}
	addProjectRows := func(projectTracks map[string]any) {
		addRows(projectTracks["peak_risk"], projectTracks["headroom_risk"], projectTracks["peak"], projectTracks["level"], projectTracks["loudness"], projectTracks["tracks"])
	}
	addRows(relationship["focus_track"], relationship["peer_track"], relationship["tracks"])
	addProjectRows(messageLoopMapValue(relationship["project_tracks"]))
	facts := messageLoopMapValue(relationship["facts"])
	addRows(facts["focus_track"], facts["peer_track"], facts["tracks"])
	addProjectRows(messageLoopMapValue(facts["project_tracks"]))
	return messageLoopUniqueTrackRows(rows)
}

func messageLoopTrackRowByUserIndex(rows []map[string]any, userTrackIndex int) map[string]any {
	if userTrackIndex <= 0 {
		return nil
	}
	for _, row := range rows {
		if messageLoopTrackRowMatchesUserIndex(row, userTrackIndex) {
			return row
		}
	}
	return nil
}

func messageLoopTrackRowMatchesUserIndex(row map[string]any, userTrackIndex int) bool {
	if len(row) == 0 || userTrackIndex <= 0 {
		return false
	}
	if index, ok := firstNumericMapValue(row, "user_track_index", "track_index", "index"); ok && int(index) == userTrackIndex {
		return true
	}
	wantSpaced := "track " + strconv.Itoa(userTrackIndex)
	wantCompact := "track" + strconv.Itoa(userTrackIndex)
	for _, alias := range messageLoopTrackMentionAliases(row) {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == wantSpaced || alias == wantCompact {
			return true
		}
	}
	return false
}

func messageLoopVocalClarificationTargetScore(row map[string]any) int {
	if len(row) == 0 {
		return 0
	}
	score := 1
	switch strings.ToLower(firstMapText(row, "risk", "headroom_risk", "risk_level")) {
	case "high", "critical", "clip", "clipping":
		score += 100
	case "medium", "moderate":
		score += 50
	}
	if headroom, ok := firstNumericMapValue(row, "headroom_db"); ok {
		switch {
		case headroom <= 0:
			score += 90
		case headroom <= 1:
			score += 70
		case headroom <= 3:
			score += 30
		}
	}
	if peak, ok := firstNumericMapValue(row, "peak_dbfs", "value"); ok {
		switch {
		case peak >= -0.1:
			score += 70
		case peak >= -1:
			score += 40
		}
	}
	if rank, ok := firstNumericMapValue(row, "rank"); ok && int(rank) == 1 {
		score += 10
	}
	if strings.EqualFold(firstMapText(row, "metric"), "peak_dbfs") {
		score += 10
	}
	return score
}

func messageLoopTrackReplyLabel(row map[string]any, fallbackID string) string {
	if label := firstMapText(row, "user_label", "track_name", "name", "label"); label != "" {
		return label
	}
	if index, ok := firstNumericMapValue(row, "user_track_index", "track_index", "index"); ok && index > 0 {
		return "Track " + strconv.Itoa(int(index))
	}
	fallbackID = strings.TrimSpace(fallbackID)
	if fallbackID != "" {
		return "Track " + fallbackID
	}
	return ""
}

func messageLoopPendingMixCandidateTrackID(state *runState, reply string, evidenceText string) string {
	if state == nil {
		return ""
	}
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		if target := messageLoopMapValue(state.recentObservation.Summary["target_ref"]); len(target) > 0 {
			if strings.EqualFold(firstMapText(target, "kind"), "track") {
				if trackID := firstMapText(target, "id", "track_id"); trackID != "" {
					return trackID
				}
			}
		}
		if trackID := firstMapText(state.recentObservation.Summary, "track_id", "target_track_id"); trackID != "" {
			return trackID
		}
	}
	if trackID := messageLoopPendingMixCandidateTrackIDFromReply(state, reply, evidenceText); trackID != "" {
		return trackID
	}
	if trackID := messageLoopPendingMixCandidateLikelyAttentionTrackID(state); trackID != "" {
		return trackID
	}
	if trackID := messageLoopLastMixObservationTrackID(state); trackID != "" {
		return trackID
	}
	return state.executionMemory.ActiveWorkTargetTrackID
}

func messageLoopPendingMixCandidateTrackIDFromReply(state *runState, reply string, evidenceText string) string {
	rows := messageLoopPendingMixCandidateTrackRows(state)
	if len(rows) == 0 {
		return ""
	}
	if trackID := messageLoopTrackIDNearestToEvidence(rows, reply, evidenceText); trackID != "" {
		return trackID
	}
	for _, text := range []string{
		messageLoopReplyWindowAroundEvidence(reply, evidenceText, 96, 48),
		reply,
	} {
		if trackID := messageLoopTrackIDMentionedOnce(rows, text); trackID != "" {
			return trackID
		}
	}
	return ""
}

func messageLoopTrackIDNearestToEvidence(rows []map[string]any, reply string, evidenceText string) string {
	reply = strings.ToLower(strings.TrimSpace(reply))
	evidenceText = strings.ToLower(strings.TrimSpace(evidenceText))
	if len(rows) == 0 || reply == "" || evidenceText == "" {
		return ""
	}
	evidenceStart := strings.Index(reply, evidenceText)
	if evidenceStart < 0 {
		return ""
	}
	evidenceEnd := evidenceStart + len(evidenceText)
	windowStart := evidenceStart - 160
	if windowStart < 0 {
		windowStart = 0
	}
	windowEnd := evidenceEnd + 80
	if windowEnd > len(reply) {
		windowEnd = len(reply)
	}
	bestTrackID := ""
	bestDistance := 0
	ambiguous := false
	for _, row := range rows {
		trackID := firstMapText(row, "track_id", "id", "target_track_id")
		if trackID == "" {
			continue
		}
		trackBest := -1
		for _, alias := range messageLoopTrackMentionAliases(row) {
			for _, span := range messageLoopAliasSpans(reply, alias) {
				start, end := span[0], span[1]
				if end < windowStart || start > windowEnd {
					continue
				}
				distance := messageLoopSpanDistance(start, end, evidenceStart, evidenceEnd)
				if trackBest < 0 || distance < trackBest {
					trackBest = distance
				}
			}
		}
		if trackBest < 0 {
			continue
		}
		if bestTrackID == "" || trackBest < bestDistance {
			bestTrackID = trackID
			bestDistance = trackBest
			ambiguous = false
			continue
		}
		if trackBest == bestDistance && trackID != bestTrackID {
			ambiguous = true
		}
	}
	if ambiguous {
		return ""
	}
	return bestTrackID
}

func messageLoopSpanDistance(start int, end int, evidenceStart int, evidenceEnd int) int {
	if end <= evidenceStart {
		return evidenceStart - end
	}
	if start >= evidenceEnd {
		return start - evidenceEnd
	}
	return 0
}

func messageLoopReplyWindowAroundEvidence(reply string, evidenceText string, before int, after int) string {
	reply = strings.TrimSpace(reply)
	evidenceText = strings.TrimSpace(evidenceText)
	if reply == "" || evidenceText == "" {
		return ""
	}
	idx := strings.Index(reply, evidenceText)
	if idx < 0 {
		return ""
	}
	replyRunes := []rune(reply)
	startRune := len([]rune(reply[:idx]))
	evidenceRunes := len([]rune(evidenceText))
	start := startRune - before
	if start < 0 {
		start = 0
	}
	end := startRune + evidenceRunes + after
	if end > len(replyRunes) {
		end = len(replyRunes)
	}
	return strings.TrimSpace(string(replyRunes[start:end]))
}

func messageLoopTrackIDMentionedOnce(rows []map[string]any, text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	matches := map[string]bool{}
	for _, row := range rows {
		trackID := firstMapText(row, "track_id", "id", "target_track_id")
		if trackID == "" {
			continue
		}
		for _, alias := range messageLoopTrackMentionAliases(row) {
			if messageLoopTextContainsAlias(text, alias) {
				matches[trackID] = true
				break
			}
		}
	}
	if len(matches) != 1 {
		return ""
	}
	for trackID := range matches {
		return trackID
	}
	return ""
}

func messageLoopTrackMentionAliases(row map[string]any) []string {
	if len(row) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if messageLoopTrackAliasTooGeneric(value) || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, key := range []string{"track_id", "id", "target_track_id", "name", "track_name", "user_label", "label"} {
		add(firstMapText(row, key))
	}
	if index, ok := firstNumericMapValue(row, "user_track_index", "track_index", "index", "rank"); ok && index > 0 {
		raw := strconv.Itoa(int(index))
		add("track " + raw)
		add("track" + raw)
		add("轨道 " + raw)
		add("轨道" + raw)
		add("第" + raw + "轨")
	}
	return out
}

func messageLoopTrackAliasTooGeneric(alias string) bool {
	alias = strings.TrimSpace(alias)
	if alias == "" || len([]rune(alias)) < 2 {
		return true
	}
	digitsOnly := true
	for _, ch := range alias {
		if ch < '0' || ch > '9' {
			digitsOnly = false
			break
		}
	}
	return digitsOnly && len(alias) < 3
}

func messageLoopTextContainsAlias(text string, alias string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	alias = strings.ToLower(strings.TrimSpace(alias))
	return len(messageLoopAliasSpans(text, alias)) > 0
}

func messageLoopAliasSpans(text string, alias string) [][2]int {
	text = strings.ToLower(strings.TrimSpace(text))
	alias = strings.ToLower(strings.TrimSpace(alias))
	if text == "" || alias == "" {
		return nil
	}
	var spans [][2]int
	for offset := 0; offset <= len(text); {
		idx := strings.Index(text[offset:], alias)
		if idx < 0 {
			return spans
		}
		start := offset + idx
		end := start + len(alias)
		if messageLoopAliasBoundary(text, start-1) && messageLoopAliasBoundary(text, end) {
			spans = append(spans, [2]int{start, end})
		}
		offset = end
	}
	return spans
}

func messageLoopAliasBoundary(text string, idx int) bool {
	if idx < 0 || idx >= len(text) {
		return true
	}
	ch := text[idx]
	return !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_')
}

func messageLoopPendingMixCandidateTrackRows(state *runState) []map[string]any {
	if state == nil {
		return nil
	}
	var rows []map[string]any
	addRow := func(row map[string]any) {
		if firstMapText(row, "track_id", "id", "target_track_id") == "" {
			return
		}
		rows = append(rows, row)
	}
	addRows := func(values ...any) {
		for _, value := range values {
			if row := messageLoopMapValue(value); len(row) > 0 {
				addRow(row)
			}
			for _, row := range messageLoopMapRows(value) {
				addRow(row)
			}
		}
	}
	addRows(state.input.State["tracks"], state.contextSnapshot["tracks"])
	addRowsFromResult := func(result map[string]any) {
		if len(result) == 0 {
			return
		}
		addRows(result["tracks"])
		project := messageLoopMapValue(result["project"])
		addRows(project["tracks"])
		mixboard := messageLoopMapValue(result["mixboard"])
		addRows(mixboard["tracks"])
		observation := messageLoopObservationFromResult(result)
		addRows(observation["tracks"])
		projectPackage := messageLoopMapValue(observation["project_package"])
		addRows(projectPackage["tracks"], projectPackage["loudness_ranking"], projectPackage["level_ranking"], projectPackage["peak_ranking"], projectPackage["headroom_risk"])
		relationship := messageLoopMapValue(result["relationship"])
		addRows(relationship["focus_track"], relationship["peer_track"], relationship["tracks"])
		projectTracks := messageLoopMapValue(relationship["project_tracks"])
		addRows(projectTracks["tracks"], projectTracks["loudness"], projectTracks["level"], projectTracks["peak"], projectTracks["peak_risk"])
		facts := messageLoopMapValue(relationship["facts"])
		addRows(facts["focus_track"], facts["peer_track"], facts["tracks"])
		factProjectTracks := messageLoopMapValue(facts["project_tracks"])
		addRows(factProjectTracks["tracks"], factProjectTracks["loudness"], factProjectTracks["level"], factProjectTracks["peak"], factProjectTracks["peak_risk"])
		digest := messageLoopMapValue(result["digest"])
		addRows(digest["project_loudness_ranking_excerpt"], digest["project_level_ranking_excerpt"], digest["project_peak_ranking_excerpt"], digest["project_headroom_risk_excerpt"])
		acoustic := messageLoopMapValue(result["acoustic_digest"])
		addRows(acoustic["project_loudness_ranking_excerpt"], acoustic["project_level_ranking_excerpt"], acoustic["project_peak_ranking_excerpt"], acoustic["project_headroom_risk_excerpt"])
		for _, entry := range messageLoopMapRows(result["entries"]) {
			value := messageLoopMapValue(entry["value"])
			addRows(value["tracks"], value["rows"], value["rankings"])
		}
		for _, item := range messageLoopMapValue(result["items"]) {
			value := messageLoopMapValue(item)
			addRows(value["tracks"], value["rows"], value["rankings"], item)
		}
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		result := messageLoopMapValue(state.executed[i]["result"])
		if len(result) == 0 {
			continue
		}
		addRowsFromResult(result)
	}
	for i := len(state.trace) - 1; i >= 0; i-- {
		event := state.trace[i]
		if event.ToolResult == nil || toolStatusFailed(event.ToolResult.Status) {
			continue
		}
		addRowsFromResult(event.ToolResult.Result)
	}
	return messageLoopUniqueTrackRows(rows)
}

func messageLoopUniqueTrackRows(rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := []map[string]any{}
	index := map[string]int{}
	for _, row := range rows {
		trackID := firstMapText(row, "track_id", "id", "target_track_id")
		if trackID == "" {
			continue
		}
		if pos, ok := index[trackID]; ok {
			out[pos] = messageLoopMergeTrackRows(out[pos], row)
			continue
		}
		index[trackID] = len(out)
		out = append(out, cloneMap(row))
	}
	return out
}

func messageLoopMergeTrackRows(base map[string]any, next map[string]any) map[string]any {
	if len(base) == 0 {
		return cloneMap(next)
	}
	out := cloneMap(base)
	for key, value := range next {
		if messageLoopEmptyValue(out[key]) && !messageLoopEmptyValue(value) {
			out[key] = value
		}
	}
	return out
}

func messageLoopPendingMixCandidateLikelyAttentionTrackID(state *runState) string {
	for _, row := range messageLoopPendingMixCandidateLikelyAttentionRows(state) {
		if trackID := messageLoopTrackIDFromAttentionRow(row); trackID != "" {
			return trackID
		}
	}
	return ""
}

func messageLoopPendingMixCandidateLikelyAttentionRows(state *runState) []map[string]any {
	if state == nil {
		return nil
	}
	var out []map[string]any
	add := func(value any) {
		if row := messageLoopMapValue(value); len(row) > 0 {
			out = append(out, row)
		}
	}
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		summary := state.recentObservation.Summary
		add(messageLoopMapValue(summary["acoustic_digest"])["likely_first_attention_target"])
		add(messageLoopMapValue(summary["digest"])["likely_first_attention_target"])
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		result := messageLoopMapValue(state.executed[i]["result"])
		if len(result) == 0 {
			continue
		}
		add(messageLoopMapValue(result["digest"])["likely_first_attention_target"])
		add(messageLoopMapValue(result["acoustic_digest"])["likely_first_attention_target"])
		observation := messageLoopObservationFromResult(result)
		add(messageLoopMapValue(observation["digest"])["likely_first_attention_target"])
		add(messageLoopMapValue(observation["project_package"])["likely_first_attention_target"])
		for _, entry := range messageLoopMapRows(result["entries"]) {
			add(messageLoopMapValue(entry["value"])["likely_first_attention_target"])
			if strings.EqualFold(firstMapText(entry, "key"), "project.attention.first") {
				add(entry["value"])
			}
		}
		for key, item := range messageLoopMapValue(result["items"]) {
			add(messageLoopMapValue(item)["likely_first_attention_target"])
			if strings.EqualFold(strings.TrimSpace(key), "project.attention.first") {
				add(item)
			}
		}
	}
	return out
}

func messageLoopTrackIDFromAttentionRow(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	if trackID := firstMapText(row, "track_id", "id", "target_track_id"); trackID != "" {
		return trackID
	}
	return firstMapText(messageLoopMapValue(row["track"]), "track_id", "id", "target_track_id")
}

func messageLoopPendingMixCandidateTrackCount(state *runState) int {
	if rows := messageLoopPendingMixCandidateTrackRows(state); len(rows) > 0 {
		return len(rows)
	}
	return len(messageLoopMapRows(state.input.State["tracks"]))
}

func messageLoopPendingMixCandidateCurrentTrackDB(state *runState, trackID string) (float64, bool) {
	if currentDB, ok := messageLoopCurrentTrackDB(state, trackID); ok {
		return currentDB, true
	}
	if row := messageLoopPendingMixCandidateTrackRow(state, trackID); len(row) > 0 {
		return firstNumericMapValue(row, "volume_db", "gain_db", "fader_db", "db")
	}
	return 0, false
}

func messageLoopPendingMixCandidateTrackRow(state *runState, trackID string) map[string]any {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return nil
	}
	for _, row := range messageLoopPendingMixCandidateTrackRows(state) {
		if firstMapText(row, "track_id", "id", "target_track_id") == trackID {
			return row
		}
	}
	return nil
}

func messageLoopExtractSingleGainDelta(reply string) (float64, string, bool) {
	if delta, evidence, ok := messageLoopExtractSingleGainDeltaInText(reply); ok {
		return delta, evidence, true
	}
	return messageLoopExtractSingleGainDeltaFromActionableWindow(reply)
}

func messageLoopExtractSingleGainDeltaInText(reply string) (float64, string, bool) {
	matches := messageLoopDBAdjustmentPattern.FindAllStringSubmatch(reply, -1)
	if len(matches) != 1 || len(matches[0]) < 3 {
		return messageLoopExtractSingleGainDeltaFromDBAmount(reply)
	}
	verb := strings.ToLower(strings.TrimSpace(matches[0][1]))
	raw := strings.TrimSpace(matches[0][2])
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value == 0 {
		return 0, "", false
	}
	sign := 1.0
	if messageLoopTextHasAny(verb, "降低", "降", "下调", "调低", "减少", "减", "lower", "reduce", "decrease", "down", "cut") {
		sign = -1
	}
	if strings.HasPrefix(raw, "-") {
		sign = -1
	} else if strings.HasPrefix(raw, "+") {
		sign = 1
	}
	return sign * math.Abs(value), strings.TrimSpace(matches[0][0]), true
}

func messageLoopExtractSingleGainDeltaFromActionableWindow(reply string) (float64, string, bool) {
	windows := messageLoopActionableGainWindows(reply)
	for _, window := range windows {
		delta, evidence, ok := messageLoopExtractSingleGainDeltaInText(window)
		if ok && delta != 0 && math.Abs(delta) <= 2 {
			return delta, evidence, true
		}
	}
	return 0, "", false
}

func messageLoopActionableGainWindows(reply string) []string {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return nil
	}
	replacer := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		". ", "\n",
		"\u3002", "\n",
		"\uff1f", "\n",
		"?", "\n",
		"\uff01", "\n",
		"!", "\n",
		"\uff1b", "\n",
		";", "\n",
	)
	rawSegments := strings.Split(replacer.Replace(reply), "\n")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		segment = strings.TrimSpace(segment)
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return nil
	}
	windows := make([]string, 0, len(segments))
	seen := map[string]bool{}
	addWindow := func(index int) {
		start := index - 1
		if start < 0 {
			start = index
		}
		end := index + 1
		if end >= len(segments) {
			end = index
		}
		window := strings.TrimSpace(strings.Join(segments[start:end+1], " "))
		if window == "" || seen[window] {
			return
		}
		seen[window] = true
		windows = append(windows, window)
	}
	for _, pass := range []string{"execute", "suggest"} {
		for i, segment := range segments {
			text := strings.ToLower(segment)
			switch pass {
			case "execute":
				if messageLoopTextHasAny(text, "\u8981\u6211", "\u6267\u884c", "\u7ee7\u7eed", "should i", "shall i", "want me", "execute", "continue") {
					addWindow(i)
				}
			case "suggest":
				if messageLoopTextHasAny(text, "\u5efa\u8bae", "\u4e0b\u4e00\u6b65", "\u5148", "\u5c0f\u52a8\u4f5c", "suggest", "recommend", "next step", "first step", "safe move") {
					addWindow(i)
				}
			}
		}
	}
	return windows
}

func messageLoopLastMixObservationTrackID(state *runState) string {
	if state == nil {
		return ""
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) || !messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			continue
		}
		result := messageLoopMapValue(record["result"])
		if trackID := firstMapText(result, "track_id", "target_track_id"); trackID != "" {
			return trackID
		}
		if target := messageLoopObservationTargetRefFromResult(result); len(target) > 0 && strings.EqualFold(firstMapText(target, "kind"), "track") {
			if trackID := firstMapText(target, "id", "track_id"); trackID != "" {
				return trackID
			}
		}
	}
	return ""
}

func messageLoopLastMixObservationField(state *runState, key string) string {
	if state == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		if value := firstMapText(state.recentObservation.Summary, key); value != "" {
			return value
		}
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) || !messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			continue
		}
		if value := firstMapText(messageLoopMapValue(record["result"]), key); value != "" {
			return value
		}
	}
	return ""
}

func messageLoopLastMixObservationScope(state *runState) string {
	if state == nil {
		return ""
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) || !messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			continue
		}
		if scope := firstMapText(messageLoopMapValue(record["result"]), "scope", "observation_scope"); scope != "" {
			return scope
		}
		result := messageLoopMapValue(record["result"])
		if scope := firstMapText(messageLoopMapValue(result["digest"]), "scope", "observation_scope"); scope != "" {
			return scope
		}
		if scope := firstMapText(messageLoopMapValue(result["acoustic_digest"]), "scope", "observation_scope"); scope != "" {
			return scope
		}
	}
	return ""
}

func messageLoopObservationTargetRefFromResult(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	if target := messageLoopMapValue(result["target_ref"]); len(target) > 0 {
		return target
	}
	observation := messageLoopObservationFromResult(result)
	return messageLoopMapValue(observation["target_ref"])
}

func messageLoopObservationFromResult(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	observation := messageLoopMapValue(result["observation"])
	if len(observation) == 0 {
		if pack := messageLoopMapValue(result["context_pack"]); len(pack) > 0 {
			observation = messageLoopMapValue(pack["latest_observation"])
		}
	}
	return observation
}
