package edu

// 判定器注册表（乐局 3a，EDU-LEVEL-RUNTIME-A 目标 2）：首版 3 个判定器的
// 闭集注册表 + 纯函数实现。输入=读取命令返回的 notes 数组（纯数据），
// 确定性零 LLM；推进契约=纯函数 (state, observation) → state'（见 jam_state.go）。
//
// 与 276 检测核的关系：note_sequence_match 复用 JudgeAgainstPrototype（judge.go），
// rhythm_match 复用 rhythmEqual（edu.go）——同核复用，语义零漂移；
// pitch_set_match 为本卡新增（276 核未覆盖集合比较），语义定案=开放问题⑤-1
// 的 280 侧提案：音高**多重集**相等（重复音计数保留，缺音/多音不对称即 fail），
// set_mode 口径二选一（pitch_class 默认=容八度错位 / absolute=严格绝对音高）。
//
// 注册表是闭集：LookupJudge 对未知名返回 false——乐局卡校验门（lesson_card.go）
// 的拒绝路径 1（未知判定器名直接拒绝）与运行时同型。
//
// 本文件零 controller 引用（§五-16 接口不变式：不进 orchestrationcontroller
// 枚举、不进路由白名单），仅依赖本包纯函数。

import (
	"fmt"
	"sort"
)

// 首版判定器名（闭集；与乐局卡 v0 done_when.judge / objectives.check.judge 对齐）。
const (
	JudgeNoteSequenceMatch = "note_sequence_match" // 音程序列逐位比对（276 JudgeAgainstPrototype 语义）
	JudgePitchSetMatch     = "pitch_set_match"     // 音高多重集相等（集合口径见 set_mode）
	JudgeRhythmMatch       = "rhythm_match"        // 节奏逐位相等（IOI + 逐音长度，276 rhythmEqual 语义）
)

// pitch_set_match 的集合口径（开放问题⑤-1 定案提案；卡面裁定落点）。
const (
	// PitchSetModePitchClass 按音高类（pitch class）比较多重集，容八度错位——
	// 初学者八度错位不判 fail 的宽容口径（默认）。
	PitchSetModePitchClass = "pitch_class"
	// PitchSetModeAbsolute 按绝对音高比较多重集，八度错位即 fail（严格口径）。
	PitchSetModeAbsolute = "absolute"
)

// PitchSetModes 是 set_mode 的合法值闭集。
var PitchSetModes = []string{PitchSetModePitchClass, PitchSetModeAbsolute}

// JudgeInput 是一次判定的纯数据输入。两侧 notes 由调用方按卡内 range 窗口
// 切好（windowNotes 同口径半开区间），判定器不再做窗口处理；顺序无关
// （判定器内部按 (Start, Pitch, ID) 稳定排序）。
type JudgeInput struct {
	// JudgeName 仅用于回填 Verdict.Judge，由评估方从注册表查找时带入。
	JudgeName string
	// TargetNotes 学生侧片段（运行时轨槽 notes 快照）。
	TargetNotes []Note
	// ReferenceNotes 参照侧片段（乐局卡内嵌 notes 表投影，id=asset:<id>#<k>）。
	ReferenceNotes []Note
	// AcceptTransposed 仅 note_sequence_match 有效：transposed_match 是否算过。
	// 默认 false（276 judge.go 语义说明预留的「由乐局卡裁决」落点，卡面字段）。
	AcceptTransposed bool
	// SetMode 仅 pitch_set_match 有效：空 = pitch_class（默认）。
	SetMode string
}

// JudgeVerdict 是判定结果：Passed 驱动状态机推进，Summary/Detail 供事件表
// 与回执留档。Detail 为 JSON 可序列化纯数据。
type JudgeVerdict struct {
	Judge   string         `json:"judge"`
	Passed  bool           `json:"passed"`
	Summary string         `json:"summary"`
	Detail  map[string]any `json:"detail,omitempty"`
}

// JudgeFunc 判定器签名：确定性纯函数，同输入同输出，零 IO 零时钟零 LLM。
type JudgeFunc func(in JudgeInput) JudgeVerdict

// judgeRegistry 首版 3 判定器闭集注册表。
var judgeRegistry = map[string]JudgeFunc{
	JudgeNoteSequenceMatch: judgeNoteSequenceMatch,
	JudgePitchSetMatch:     judgePitchSetMatch,
	JudgeRhythmMatch:       judgeRhythmMatch,
}

// RegisteredJudges 返回排序后的判定器名快照（闭集枚举，供校验与文档）。
func RegisteredJudges() []string {
	names := make([]string, 0, len(judgeRegistry))
	for name := range judgeRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LookupJudge 按名查找判定器；未知名返回 false（fail-closed，不静默替代）。
func LookupJudge(name string) (JudgeFunc, bool) {
	f, ok := judgeRegistry[name]
	return f, ok
}

// judgeNoteSequenceMatch：学生片段 vs 原型的音程序列逐位比对（276
// JudgeAgainstPrototype 同核）。Passed = pitch_status==match，或
// transposed_match 且卡面 accept_transposed=true；length_mismatch /
// pitch_mismatch 一律 fail（错音定位明细随 Detail 返回，供 n1/n3 教学叙事）。
func judgeNoteSequenceMatch(in JudgeInput) JudgeVerdict {
	j := JudgeAgainstPrototype(in.ReferenceNotes, in.TargetNotes)
	passed := j.PitchStatus == StatusMatch ||
		(in.AcceptTransposed && j.PitchStatus == StatusTransposedMatch)
	summary := fmt.Sprintf("pitch_status=%s transposition=%+d rhythm_matched=%v mismatches=%d",
		j.PitchStatus, j.Transposition, j.RhythmMatched, len(j.Mismatches))
	return JudgeVerdict{
		Judge:   in.JudgeName,
		Passed:  passed,
		Summary: summary,
		Detail:  map[string]any{"judgment": j},
	}
}

// judgePitchSetMatch：两段音高多重集相等（开放问题⑤-1 提案口径）。
// 重复音计数保留：参照里出现两次的音学生只弹一次 = missing 1 → fail。
// 两侧均非空才可能 Passed（空对空不构成证据，fail-closed）。
func judgePitchSetMatch(in JudgeInput) JudgeVerdict {
	mode := in.SetMode
	if mode == "" {
		mode = PitchSetModePitchClass
	}
	classify := func(p int) int {
		if mode == PitchSetModeAbsolute {
			return p
		}
		return ((p % 12) + 12) % 12
	}
	target := map[int]int{}
	for _, n := range in.TargetNotes {
		target[classify(n.Pitch)]++
	}
	ref := map[int]int{}
	for _, n := range in.ReferenceNotes {
		ref[classify(n.Pitch)]++
	}
	var missing, extra []int // missing=参照有而学生缺；extra=学生多出
	for c, cnt := range ref {
		if d := cnt - target[c]; d > 0 {
			missing = append(missing, c)
		}
	}
	for c, cnt := range target {
		if d := cnt - ref[c]; d > 0 {
			extra = append(extra, c)
		}
	}
	sort.Ints(missing)
	sort.Ints(extra)
	passed := len(in.TargetNotes) > 0 && len(in.ReferenceNotes) > 0 &&
		len(missing) == 0 && len(extra) == 0
	summary := fmt.Sprintf("set_mode=%s target=%d reference=%d missing=%v extra=%v",
		mode, len(in.TargetNotes), len(in.ReferenceNotes), missing, extra)
	return JudgeVerdict{
		Judge:   in.JudgeName,
		Passed:  passed,
		Summary: summary,
		Detail: map[string]any{
			"set_mode":        mode,
			"target_count":    len(in.TargetNotes),
			"reference_count": len(in.ReferenceNotes),
			"missing":         missing,
			"extra":           extra,
		},
	}
}

// judgeRhythmMatch：两段节奏逐位相等（IOI + 逐音长度，276 rhythmEqual 同核）。
// 音数不等即 fail；两侧均非空才可能 Passed（设计稿 §1.2 口径）。
func judgeRhythmMatch(in JudgeInput) JudgeVerdict {
	ts := sortedNotes(in.TargetNotes)
	rs := sortedNotes(in.ReferenceNotes)
	rEq := len(ts) == len(rs) && rhythmEqual(ts, rs)
	passed := len(ts) > 0 && rEq
	summary := fmt.Sprintf("target=%d reference=%d rhythm_matched=%v", len(ts), len(rs), rEq)
	return JudgeVerdict{
		Judge:   in.JudgeName,
		Passed:  passed,
		Summary: summary,
		Detail: map[string]any{
			"target_count":    len(ts),
			"reference_count": len(rs),
			"rhythm_matched":  rEq,
		},
	}
}
