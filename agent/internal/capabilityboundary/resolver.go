package capabilityboundary

import (
	"fmt"
	"sort"
	"strings"
)

// Unavailable reason identifiers. They disclose why a remediation class is
// absent instead of letting the payload imply an option exists.
const (
	ReasonFamilyUnresolved         = "family_unresolved"
	ReasonNoSameFamilyCandidate    = "no_pca_admitted_same_family_candidate"
	ReasonNoAdjacentCandidate      = "no_pca_admitted_adjacent_family_candidate"
	ReasonReadOnlyFamily           = "read_only_family_has_no_controller_substitution"
	ReasonNativeUnavailable        = "native_option_unavailable"
	ReasonNoManualInstance         = "no_loaded_instance_for_manual_guidance"
	ReasonCertificationUnsupported = "certification_entry_unavailable_for_family"
)

const (
	riskLow    = "low"
	riskMedium = "medium"
)

// Resolve renders the deterministic capability_boundary.v1 payload. It is a
// pure function of the input snapshot: no LLM, no I/O, no gate calls, and no
// action kind that writes (or approximately writes) any parameter. Ordering
// is fixed: same-family admitted > adjacent-family admitted > native/stock >
// manual guidance > certification entry.
func Resolve(input Input) Boundary {
	boundary := Boundary{
		SchemaVersion:     SchemaVersion,
		Code:              strings.TrimSpace(input.Code),
		Class:             strings.TrimSpace(input.Class),
		Family:            NormalizeFamily(input.Family),
		TrackID:           strings.TrimSpace(input.TrackID),
		Reason:            strings.TrimSpace(input.Reason),
		EvidenceRefs:      dedupeStrings(input.EvidenceRefs),
		BlockedBy:         dedupeStrings(input.BlockedBy),
		MutationPerformed: false,
		NextActions:       []NextAction{},
	}

	family := boundary.Family
	readOnly := IsReadOnlyFamily(family)

	alternates := []Candidate{}
	if family == "" {
		boundary.addUnavailable(ReasonFamilyUnresolved)
	} else if readOnly {
		boundary.addUnavailable(ReasonReadOnlyFamily)
	} else {
		alternates = rankedAdmittedCandidates(input.Candidates, family)
		if len(alternates) == 0 {
			boundary.addUnavailable(ReasonNoSameFamilyCandidate)
			boundary.addUnavailable(ReasonNoAdjacentCandidate)
		}
	}
	if len(alternates) > MaxAlternateActions {
		alternates = alternates[:MaxAlternateActions]
	}
	for _, candidate := range alternates {
		boundary.NextActions = append(boundary.NextActions, alternateProcessorAction(candidate))
	}

	if native := nativeAction(input.Native, family, readOnly); native != nil {
		boundary.NextActions = append(boundary.NextActions, *native)
	} else if family != "" && !readOnly {
		boundary.addUnavailable(ReasonNativeUnavailable)
	}

	instances := stableInstances(input.Instances)
	if len(instances) == 0 {
		boundary.addUnavailable(ReasonNoManualInstance)
	} else {
		boundary.NextActions = append(boundary.NextActions, manualGuidanceAction(instances, family))
		for _, instance := range instances {
			boundary.NextActions = append(boundary.NextActions, openPluginUIAction(instance))
		}
	}

	if input.Certification != nil && strings.TrimSpace(input.Certification.Endpoint) != "" {
		boundary.NextActions = append(boundary.NextActions, certificationEntryAction(*input.Certification))
	} else {
		boundary.addUnavailable(ReasonCertificationUnsupported)
	}

	for index := range boundary.NextActions {
		boundary.NextActions[index].Rank = index + 1
	}
	sort.Strings(boundary.UnavailableReasons)
	boundary.UnavailableReasons = dedupeStrings(boundary.UnavailableReasons)
	return boundary
}

func (b *Boundary) addUnavailable(reason string) {
	b.UnavailableReasons = append(b.UnavailableReasons, reason)
}

// rankedAdmittedCandidates keeps only admitted candidates whose family is the
// boundary family or an adjacent family, then stable-sorts by family rank and
// exact identifier. The wiring already ran the PCA admission/coverage query;
// this only ranks what that query admitted.
func rankedAdmittedCandidates(candidates []Candidate, boundaryFamily string) []Candidate {
	adjacency := map[string]int{}
	for rank, family := range AdjacentFamilies(boundaryFamily) {
		adjacency[family] = rank + 1
	}
	out := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		family := NormalizeFamily(candidate.Family)
		if family != boundaryFamily {
			if _, adjacent := adjacency[family]; !adjacent {
				continue
			}
		}
		if strings.TrimSpace(candidate.Identifier) == "" || strings.TrimSpace(candidate.PluginPath) == "" {
			continue
		}
		candidate.Family = family
		if family == boundaryFamily {
			candidate.FamilyRank = 0
		} else {
			candidate.FamilyRank = adjacency[family]
		}
		out = append(out, candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FamilyRank != out[j].FamilyRank {
			return out[i].FamilyRank < out[j].FamilyRank
		}
		left := strings.ToLower(strings.TrimSpace(out[i].Identifier)) + "\x00" + strings.ToLower(strings.TrimSpace(out[i].PluginPath))
		right := strings.ToLower(strings.TrimSpace(out[j].Identifier)) + "\x00" + strings.ToLower(strings.TrimSpace(out[j].PluginPath))
		return left < right
	})
	// Deduplicate exact binaries that reached the list twice (same identifier
	// and path admitted under one family keeps its first, better-ranked copy).
	deduped := out[:0]
	seen := map[string]bool{}
	for _, candidate := range out {
		key := strings.ToLower(strings.TrimSpace(candidate.Identifier)) + "\x00" + strings.ToLower(strings.TrimSpace(candidate.PluginPath))
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, candidate)
	}
	return deduped
}

func stableInstances(instances []InstanceRef) []InstanceRef {
	out := append([]InstanceRef(nil), instances...)
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(strings.TrimSpace(out[i].PluginID)) < strings.ToLower(strings.TrimSpace(out[j].PluginID))
	})
	return out
}

func alternateProcessorAction(candidate Candidate) NextAction {
	relation := "同族"
	if candidate.FamilyRank > 0 {
		relation = "相邻族"
	}
	return NextAction{
		Kind:                 KindAlternateProcessor,
		Key:                  "alternate_" + strings.ToLower(strings.TrimSpace(candidate.Identifier)),
		Title:                fmt.Sprintf("改用%s已准入处理器 %s", relation, firstNonEmpty(candidate.PluginName, candidate.Identifier)),
		RequiresConfirmation: true,
		RiskLevel:            riskMedium,
		ExpectedEffect:       fmt.Sprintf("以已通过 PCA 准入的 %s 处理器继续同一目标；加载与后续控制仍走既有确认与准入流程。", candidate.Family),
		Tradeoff:             "替换处理器会改变音色与控制面；加载后仍需逐参数经 typed 准入，不会自动写入任何参数。",
		Evidence: map[string]any{
			"attestation_id":     candidate.AttestationID,
			"subject_key":        candidate.SubjectKey,
			"binary_fingerprint": candidate.BinaryFingerprint,
			"family":             candidate.Family,
			"family_rank":        candidate.FamilyRank,
			"coverage_axes":      candidate.CoverageAxes,
			"coverage_level":     candidate.CoverageLevel,
		},
		Target: map[string]any{
			"identifier":  candidate.Identifier,
			"plugin_path": candidate.PluginPath,
			"plugin_name": candidate.PluginName,
			"family":      candidate.Family,
		},
	}
}

func nativeAction(native *NativeOption, family string, readOnly bool) *NextAction {
	if native == nil || readOnly || family == "" {
		return nil
	}
	return &NextAction{
		Kind:                 KindNativeProcessor,
		Key:                  "native_" + strings.TrimSpace(native.Family),
		Title:                firstNonEmpty(native.Label, "走 DAW 原生（native）路径"),
		RequiresConfirmation: false,
		RiskLevel:            riskLow,
		ExpectedEffect:       firstNonEmpty(native.Description, "由你在 DAW 中经原生路径完成加载与调整；该路径不经 agent 的 PCA 装载门。"),
		Tradeoff:             "原生路径完全由用户操作，agent 不参与装载或写参；不产生可回滚的执行收据。",
		Evidence:             map[string]any{"family": family, "source": "docs/PCA_PLUGIN_LOAD_GATE_V1.md"},
		Target:               map[string]any{"native_family": strings.TrimSpace(native.Family)},
	}
}

func manualGuidanceAction(instances []InstanceRef, boundaryFamily string) NextAction {
	params := []ParamRef{}
	for _, instance := range instances {
		for _, param := range instance.Params {
			copied := param
			copied.Role = firstNonEmpty(copied.Role, copied.ParamID)
			params = append(params, copied)
		}
	}
	instanceNames := make([]string, 0, len(instances))
	for _, instance := range instances {
		instanceNames = append(instanceNames, fmt.Sprintf("%s(%s)", firstNonEmpty(instance.PluginName, instance.PluginID), instance.PluginID))
	}
	return NextAction{
		Kind:                 KindManualGuidance,
		Key:                  "manual_guidance_loaded_instances",
		Title:                "按参数指引手动处理",
		RequiresConfirmation: false,
		RiskLevel:            riskLow,
		ExpectedEffect:       "依据 live context pack 的参数面（角色/参数/范围/当前值/预期方向）由你手动调整；agent 不写任何参数。",
		Tradeoff:             "手动调整没有执行收据与回读验证；效果由你现场判断。",
		Evidence: map[string]any{
			"boundary_family": boundaryFamily,
			"instances":       instanceNames,
		},
		Target:       map[string]any{"track_id": firstNonEmpty(instances[0].TrackID, ""), "source": "live_context_pack"},
		ManualParams: params,
	}
}

func openPluginUIAction(instance InstanceRef) NextAction {
	return NextAction{
		Kind:                 KindOpenPluginUI,
		Key:                  "open_ui_" + strings.TrimSpace(instance.PluginID),
		Title:                fmt.Sprintf("打开 %s 的编辑器界面", firstNonEmpty(instance.PluginName, instance.PluginID)),
		RequiresConfirmation: false,
		RiskLevel:            riskLow,
		ExpectedEffect:       "打开该已加载插件的 UI 供你查看与手动操作；仅打开界面，不写入参数。",
		Tradeoff:             "只提供界面入口；参数变化不在 agent 收据内。",
		Evidence: map[string]any{
			"family":   instance.Family,
			"track_id": instance.TrackID,
		},
		Target: map[string]any{
			"track_id":  instance.TrackID,
			"plugin_id": instance.PluginID,
			"tool":      "open_plugin_ui",
		},
	}
}

func certificationEntryAction(entry CertificationEntry) NextAction {
	return NextAction{
		Kind:                 KindCertificationEntry,
		Key:                  "certification_" + strings.TrimSpace(entry.Family),
		Title:                fmt.Sprintf("发起 %s 的 PCA 认证（入口）", strings.TrimSpace(entry.Family)),
		RequiresConfirmation: true,
		RiskLevel:            riskMedium,
		ExpectedEffect:       "进入产品内认证流程：临时轨 apply/readback/restore 后删除临时轨，不保存当前工程；完成后该处理器进入已准入清单。",
		Tradeoff:             "认证需要显式同意令牌并占用一次临时轨流程；本响应只给入口，不启动任何认证步骤。",
		Evidence: map[string]any{
			"family":        strings.TrimSpace(entry.Family),
			"consent_token": entry.ConsentToken,
		},
		Target: map[string]any{
			"endpoint":      strings.TrimSpace(entry.Endpoint),
			"family":        strings.TrimSpace(entry.Family),
			"consent_token": entry.ConsentToken,
			"note":          entry.Note,
		},
	}
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
