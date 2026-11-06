package edu

// 判定器：学生片段 vs 原型的比较。与检测核共用音程序列提取与节奏比对
// （同核复用），错音定位 = 逐音程序列 diff。
//
// 语义说明（教学事实层，非教学裁决层）：
//   - 音高序列完全一致（首音差 0）→ match；
//   - 首音差 s≠0 且音程序列逐位一致 → transposed_match（整体移调 s 半音，
//     对「模进练习」是正确变体，对「原调重复练习」是否算对由乐局卡裁决）；
//   - 音程序列存在不一致 → pitch_mismatch，逐个定位到学生片段中的错音；
//   - 音数不等 → length_mismatch（仍按公共区间做逐音 diff，供部分定位）；
//   - 错音期望值（ExpectedPitch）以「首音差为移位假设」计算：期望学生
//     第 k 音 = 原型第 k 音 + 首音差；PrototypePitch 同步给出原型原值，
//     由调用方按练习语境选择展示口径。

// 判定状态常量。
const (
	StatusMatch           = "match"            // 原样一致
	StatusTransposedMatch = "transposed_match" // 整体移调后一致
	StatusPitchMismatch   = "pitch_mismatch"   // 存在错音
	StatusLengthMismatch  = "length_mismatch"  // 音数不等
)

// PitchMismatch 定位学生片段中的一个错音：学生片段第 AttemptIndex 个音的
// 音高偏离了原型（经移位假设换算后）的对应音。
type PitchMismatch struct {
	AttemptIndex   int    `json:"attempt_index"` // 学生片段（排序后）中的索引
	NoteID         string `json:"note_id"`
	ActualPitch    int    `json:"actual_pitch"`
	ExpectedPitch  int    `json:"expected_pitch"`  // 原型对应音 + Transposition（移位假设）
	PrototypePitch int    `json:"prototype_pitch"` // 原型对应音原值
}

// Judgment 是判定结果。Mismatches 为空 slice（非 nil）便于 JSON 序列化。
type Judgment struct {
	PitchStatus        string          `json:"pitch_status"`
	Transposition      int             `json:"transposition"` // 首音半音差；match 时为 0
	PrototypeNoteCount int             `json:"prototype_note_count"`
	AttemptNoteCount   int             `json:"attempt_note_count"`
	Mismatches         []PitchMismatch `json:"mismatches"`
	RhythmMatched      bool            `json:"rhythm_matched"` // 音数相等时逐位比对节奏
}

// JudgeAgainstPrototype 比较学生片段与原型片段（确定性纯函数）。两侧输入
// 均按 (Start, Pitch, ID) 稳定排序后对齐，调用方无需预排序。
func JudgeAgainstPrototype(prototype []Note, attempt []Note) Judgment {
	ps := sortedNotes(prototype)
	as := sortedNotes(attempt)
	j := Judgment{
		PitchStatus:        StatusLengthMismatch,
		PrototypeNoteCount: len(ps),
		AttemptNoteCount:   len(as),
		Mismatches:         []PitchMismatch{},
	}
	nP, nA := len(ps), len(as)
	if nP == 0 || nA == 0 {
		return j
	}
	shift := as[0].Pitch - ps[0].Pitch
	j.Transposition = shift

	// 公共可比较区间：intervals 数 = min(nP, nA) - 1。
	cmpIntervals := nP - 1
	if nA-1 < cmpIntervals {
		cmpIntervals = nA - 1
	}
	intervalsAllEqual := true
	for k := 0; k < cmpIntervals; k++ {
		ivP := ps[k+1].Pitch - ps[k].Pitch
		ivA := as[k+1].Pitch - as[k].Pitch
		if ivA == ivP {
			continue
		}
		intervalsAllEqual = false
		// 发散可能源自第 k+1 音本身偏离移位假设（真错音），也可能只是
		// 前一错音的恢复点（该音已回到 原型+shift 轨道）。只报前者，
		// 避免把被牵连的音标成「期望=实际」的空 mismatch。
		if as[k+1].Pitch != ps[k+1].Pitch+shift {
			j.Mismatches = append(j.Mismatches, PitchMismatch{
				AttemptIndex:   k + 1,
				NoteID:         as[k+1].ID,
				ActualPitch:    as[k+1].Pitch,
				ExpectedPitch:  ps[k+1].Pitch + shift,
				PrototypePitch: ps[k+1].Pitch,
			})
		}
	}

	switch {
	case nP != nA:
		j.PitchStatus = StatusLengthMismatch
	case intervalsAllEqual && shift == 0:
		j.PitchStatus = StatusMatch
		j.Transposition = 0
	case intervalsAllEqual:
		j.PitchStatus = StatusTransposedMatch
	default:
		j.PitchStatus = StatusPitchMismatch
	}
	j.RhythmMatched = rhythmEqual(ps, as)
	return j
}
