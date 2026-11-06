// Package edu 提供乐局（音乐教学场景）的符号分析原语：MIDI 音符序列的
// 变换族结构检测（repeat / sequence / inversion / retrograde）与学生片段
// 对原型的判定（错音定位）。
//
// 本包全部函数为确定性纯函数：同输入同输出，不执行 IO，不读取时钟或
// 随机源，不依赖本仓库其他包。beats 域浮点比较统一使用 1e-6 容差。
//
// DetectTransformations 检测约定：
//   - 原型段与匹配段是输入序列（按 start 排序后）中两个互不相交的连续
//     子序列，且匹配段位于原型段之后；重叠段对不在检测范围。
//   - repeat / sequence(chromatic)：两段音程序列（半音差）逐位相等；
//     repeat=首音差 0，sequence=首音差恒定非 0（严格移调，半音 shift 单值）。
//   - sequence(diatonic)：提供 Key 时，两段所有音落在同一大调音阶内，
//     且绝对音级差逐位恒定非 0（调内模进；半音差允许逐位变化）。
//   - inversion：匹配段音程序列 = 原型段音程序列逐位取反。
//   - retrograde：匹配段音高序列 = 原型段音高序列逆序，节奏同步镜像。
//   - 节奏比对：repeat/sequence/inversion 要求两段 IOI（inter-onset
//     interval）与逐音长度完全相等；retrograde 要求匹配段节奏 = 原型段
//     节奏逆序。
//   - 同一起点对上多种变换同时成立（退化情形，如同音反复）时仅报更高
//     优先级且更长的那种：identity（repeat/sequence） > inversion >
//     retrograde。
//   - 每种变换只报极大匹配段：被同变换更长段完全包含的子段被过滤。
package edu

import "sort"

// Note 是符号分析的最小音符单元，字段与内核 get_midi_clip_notes 返回的
// note 行对应。Start/Length 单位为 clip 内 beats。
type Note struct {
	ID     string  `json:"id"`
	Pitch  int     `json:"pitch"`
	Start  float64 `json:"start"`
	Length float64 `json:"length"`
}

// beatEpsilon 是 beats 域浮点比较容差。
const beatEpsilon = 1e-6

func floatEq(a, b float64) bool {
	d := a - b
	return d < beatEpsilon && d > -beatEpsilon
}

// sortedNotes 返回按 (Start, Pitch, ID) 稳定排序的副本；输入顺序不影响
// 检测与判定结果（确定性保证的一部分）。
func sortedNotes(notes []Note) []Note {
	out := append([]Note(nil), notes...)
	sort.SliceStable(out, func(a, b int) bool {
		if !floatEq(out[a].Start, out[b].Start) {
			return out[a].Start < out[b].Start
		}
		if out[a].Pitch != out[b].Pitch {
			return out[a].Pitch < out[b].Pitch
		}
		return out[a].ID < out[b].ID
	})
	return out
}

// extractIntervals 返回音高序列的半音差序列：ret[k] = pitches[k+1]-pitches[k]。
func extractIntervals(pitches []int) []int {
	if len(pitches) < 2 {
		return nil
	}
	ret := make([]int, len(pitches)-1)
	for k := 0; k+1 < len(pitches); k++ {
		ret[k] = pitches[k+1] - pitches[k]
	}
	return ret
}

// rhythmEqual 判断两个等长音符序列的节奏是否完全一致：
// 逐音长度相等且段内 IOI（inter-onset interval）逐位相等。
func rhythmEqual(a, b []Note) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	for k := range a {
		if !floatEq(a[k].Length, b[k].Length) {
			return false
		}
		if k > 0 && !floatEq(a[k].Start-a[k-1].Start, b[k].Start-b[k-1].Start) {
			return false
		}
	}
	return true
}

// Key 描述调性上下文，用于调内模进（diatonic sequence）检测。
// 首版仅支持大调：Mode 为空或 "major"。
type Key struct {
	Tonic int    `json:"tonic"` // 主音 pitch class：0=C … 11=B
	Mode  string `json:"mode,omitempty"`
}

// preparedKey 是 Key 的预计算形态：pitch class -> 大调音级（0-6），
// 调外 pitch class 为 -1。
type preparedKey struct {
	tonic   int
	degByPC [12]int
}

// majorScaleOffsets 是大调音阶各音级相对主音的半音偏移。
var majorScaleOffsets = [7]int{0, 2, 4, 5, 7, 9, 11}

func prepareKey(k Key) (preparedKey, error) {
	if k.Tonic < 0 || k.Tonic > 11 {
		return preparedKey{}, errInvalidKey
	}
	if k.Mode != "" && k.Mode != "major" {
		return preparedKey{}, errUnsupportedMode
	}
	var pk preparedKey
	pk.tonic = k.Tonic
	for d := range pk.degByPC {
		pk.degByPC[d] = -1
	}
	for d, off := range majorScaleOffsets {
		pk.degByPC[(k.Tonic+off)%12] = d
	}
	return pk, nil
}

// absDeg 返回音高 p 相对主音的绝对音级（跨八度连续计数，仅用于求差）。
// p 落在音阶外（调外音）时第二个返回值为 false。
func (pk preparedKey) absDeg(p int) (int, bool) {
	if p < 0 || p > 127 {
		return 0, false
	}
	semis := p - pk.tonic + 1200
	oct := semis / 12
	pc := semis % 12
	d := pk.degByPC[pc]
	if d < 0 {
		return 0, false
	}
	return 7*(oct-100) + d, true
}
