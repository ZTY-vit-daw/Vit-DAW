package agentloop

import (
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
	if text == "" || alias == "" {
		return false
	}
	for offset := 0; offset <= len(text); {
		idx := strings.Index(text[offset:], alias)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(alias)
		if messageLoopAliasBoundary(text, start-1) && messageLoopAliasBoundary(text, end) {
			return true
		}
		offset = end
	}
	return false
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
	addRows := func(values ...any) {
		for _, value := range values {
			for _, row := range messageLoopMapRows(value) {
				if firstMapText(row, "track_id", "id", "target_track_id") == "" {
					continue
				}
				rows = append(rows, row)
			}
		}
	}
	addRows(state.input.State["tracks"], state.contextSnapshot["tracks"])
	for i := len(state.executed) - 1; i >= 0; i-- {
		result := messageLoopMapValue(state.executed[i]["result"])
		if len(result) == 0 {
			continue
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
	for _, row := range messageLoopPendingMixCandidateTrackRows(state) {
		if firstMapText(row, "track_id", "id", "target_track_id") != trackID {
			continue
		}
		return firstNumericMapValue(row, "volume_db", "gain_db", "fader_db", "db")
	}
	return 0, false
}

func messageLoopExtractSingleGainDelta(reply string) (float64, string, bool) {
	matches := messageLoopDBAdjustmentPattern.FindAllStringSubmatch(reply, -1)
	if len(matches) != 1 || len(matches[0]) < 3 {
		return 0, "", false
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
