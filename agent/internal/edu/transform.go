package edu

import (
	"errors"
	"sort"
)

// Kind 标识变换族中的一种结构关系。
type Kind string

const (
	KindRepeat     Kind = "repeat"     // 原样重复（shift=0 的恒等关系）
	KindSequence   Kind = "sequence"   // 模进：chromatic（严格移调）或 diatonic（调内模进）
	KindInversion  Kind = "inversion"  // 倒影：匹配段音程序列 = 原型段逐位取反
	KindRetrograde Kind = "retrograde" // 逆行：匹配段音高序列 = 原型段逆序
)

// SequenceQuality 区分 sequence 的两级语义。
type SequenceQuality string

const (
	SeqChromatic SequenceQuality = "chromatic" // 严格移调：半音差恒定，shift_semitones 单值
	SeqDiatonic  SequenceQuality = "diatonic"  // 调内模进：音级差恒定，半音差允许变化
)

var errInvalidKey = errors.New("edu: key tonic must be a pitch class 0-11")
var errUnsupportedMode = errors.New("edu: only major mode is supported (leave Mode empty or set \"major\")")
var errUnknownKind = errors.New("edu: unknown transformation kind")

// DetectOptions 控制 DetectTransformations 的行为。零值 = 全族变换 + 最小
// 匹配音数 3 + 不启用调内模进检测。
type DetectOptions struct {
	// Transformations 是待检测的变换白名单；nil/空 = 全族四种。
	Transformations []Kind
	// MinMatchNotes 是每段（原型段与匹配段各自）的最小音符数；<=0 取 3。
	MinMatchNotes int
	// Key 提供时启用调内模进（sequence/diatonic）检测；nil 则只检测
	// 半音域关系（chromatic repeat/sequence、inversion、retrograde）。
	Key *Key
}

// Segment 描述参与一个 Finding 的音符段（输入排序后序列的连续子序列）。
type Segment struct {
	StartIndex int      `json:"start_index"` // 排序后序列中的起始索引
	NoteCount  int      `json:"note_count"`
	StartBeats float64  `json:"start_beats"` // 段内首音 start
	EndBeats   float64  `json:"end_beats"`   // 段内末音 start+length
	NoteIDs    []string `json:"note_ids"`
	Pitches    []int    `json:"pitches"`
}

// Finding 是一条变换族结构判定。段 2 命令面在此基础上补 bar 锚定与
// LLMContext 字段族（256 报告点①草案）。
type Finding struct {
	Kind            Kind            `json:"kind"`
	SequenceQuality SequenceQuality `json:"sequence_quality,omitempty"` // 仅 KindSequence 有效
	// IntervalPattern 是原型段音程序列（半音）。
	IntervalPattern []int `json:"interval_pattern"`
	// ShiftSemitones：sequence=首音半音差（chromatic 时逐位恒定）；
	// repeat=0；inversion/retrograde 无意义（0）。
	ShiftSemitones int `json:"shift_semitones"`
	// DiatonicSteps 是 diatonic sequence 的音级步差（如 +1=移高一级）；
	// 其他变换无意义（0）。
	DiatonicSteps int `json:"diatonic_steps,omitempty"`
	// InversionAxisSemitones 是 inversion 的镜像轴（= 原型首音 + 匹配首音）；
	// 匹配段第 t 音 = 轴 - 原型第 t 音。其他变换无意义（0）。
	InversionAxisSemitones int     `json:"inversion_axis_semitones,omitempty"`
	Prototype              Segment `json:"prototype"`
	Match                  Segment `json:"match"`
}

// DetectTransformations 在音符序列中检测变换族结构关系，返回确定性排序的
// findings（同输入同输出）。音符会先按 (Start, Pitch, ID) 稳定排序，调用方
// 无需预排序。opts 无效（未知 Kind、Key 非法）时返回 error。
//
// 复杂度 O(N^2 * L)（N=音符数，L=平均匹配长度）；retrograde 为枚举验证，
// O(N^2 * L^2)。长序列应由调用方先按 range_beats 分窗（段 2 命令面职责）。
func DetectTransformations(notes []Note, opts DetectOptions) ([]Finding, error) {
	wantRepeat, wantSequence, wantInversion, wantRetro := true, true, true, true
	if len(opts.Transformations) > 0 {
		wantRepeat, wantSequence, wantInversion, wantRetro = false, false, false, false
		for _, k := range opts.Transformations {
			switch k {
			case KindRepeat:
				wantRepeat = true
			case KindSequence:
				wantSequence = true
			case KindInversion:
				wantInversion = true
			case KindRetrograde:
				wantRetro = true
			default:
				return nil, errUnknownKind
			}
		}
	}
	minNotes := opts.MinMatchNotes
	if minNotes <= 0 {
		minNotes = 3
	}
	var pk preparedKey
	hasKey := false
	if opts.Key != nil {
		p, err := prepareKey(*opts.Key)
		if err != nil {
			return nil, err
		}
		pk = p
		hasKey = true
	}

	ns := sortedNotes(notes)
	n := len(ns)
	findings := []Finding{}
	if n < 2*minNotes {
		return findings, nil
	}

	type candidate struct {
		kind    Kind
		quality SequenceQuality
		i, j, m int // 原型起点、匹配起点、段内音符数
		shift   int // repeat/sequence：首音半音差
		degDiff int // diatonic：音级差
		axis    int // inversion：轴
	}
	cands := []candidate{}

	for i := 0; i+minNotes <= n-minNotes; i++ {
		for j := i + minNotes; j+minNotes <= n; j++ {
			nID := extendIdentity(ns, i, j)
			// 链优先级判定（退化覆盖）与 want 白名单无关：即使某变换不在
			// 白名单内，其成立的更长段仍用于压制更低优先级的重复报告。
			nInv := extendInversion(ns, i, j)
			if nID >= minNotes {
				shift := ns[j].Pitch - ns[i].Pitch
				if shift == 0 && wantRepeat {
					cands = append(cands, candidate{kind: KindRepeat, i: i, j: j, m: nID})
				} else if shift != 0 && wantSequence {
					cands = append(cands, candidate{kind: KindSequence, quality: SeqChromatic, i: i, j: j, m: nID, shift: shift})
				}
			}
			if hasKey && wantSequence {
				if mDia, deg := extendDiatonic(ns, i, j, pk); mDia >= minNotes && deg != 0 && mDia > nID {
					cands = append(cands, candidate{kind: KindSequence, quality: SeqDiatonic, i: i, j: j, m: mDia, shift: ns[j].Pitch - ns[i].Pitch, degDiff: deg})
				}
			}
			if wantInversion {
				if nInv >= minNotes && nInv > nID {
					cands = append(cands, candidate{kind: KindInversion, i: i, j: j, m: nInv, axis: ns[i].Pitch + ns[j].Pitch})
				}
			}
			if wantRetro {
				if nRet := extendRetrograde(ns, i, j, minNotes); nRet >= minNotes && nRet > nID && nRet > nInv {
					cands = append(cands, candidate{kind: KindRetrograde, i: i, j: j, m: nRet})
				}
			}
		}
	}

	// 极大性过滤：同 kind+quality 下被更长段完全包含的候选淘汰。
	kept := make([]candidate, 0, len(cands))
	for ci, c := range cands {
		covered := false
		for di, d := range cands {
			if di == ci || d.kind != c.kind || d.quality != c.quality || d.m <= c.m {
				continue
			}
			if d.i <= c.i && c.i+c.m <= d.i+d.m && d.j <= c.j && c.j+c.m <= d.j+d.m {
				covered = true
				break
			}
		}
		if !covered {
			kept = append(kept, c)
		}
	}

	for _, c := range kept {
		proto := buildSegment(ns, c.i, c.m)
		match := buildSegment(ns, c.j, c.m)
		findings = append(findings, Finding{
			Kind:                   c.kind,
			SequenceQuality:        c.quality,
			IntervalPattern:        extractIntervals(proto.Pitches),
			ShiftSemitones:         c.shift,
			DiatonicSteps:          c.degDiff,
			InversionAxisSemitones: c.axis,
			Prototype:              proto,
			Match:                  match,
		})
	}

	sortFindings(findings)
	return findings, nil
}

func buildSegment(ns []Note, start, count int) Segment {
	seg := Segment{
		StartIndex: start,
		NoteCount:  count,
		StartBeats: ns[start].Start,
		EndBeats:   ns[start+count-1].Start + ns[start+count-1].Length,
		NoteIDs:    make([]string, count),
		Pitches:    make([]int, count),
	}
	for k := 0; k < count; k++ {
		seg.NoteIDs[k] = ns[start+k].ID
		seg.Pitches[k] = ns[start+k].Pitch
	}
	return seg
}

func sortFindings(fs []Finding) {
	rank := map[Kind]int{KindRepeat: 0, KindSequence: 1, KindInversion: 2, KindRetrograde: 3}
	sort.SliceStable(fs, func(a, b int) bool {
		x, y := fs[a], fs[b]
		if !floatEq(x.Match.StartBeats, y.Match.StartBeats) {
			return x.Match.StartBeats < y.Match.StartBeats
		}
		if x.Prototype.StartIndex != y.Prototype.StartIndex {
			return x.Prototype.StartIndex < y.Prototype.StartIndex
		}
		if x.Match.StartIndex != y.Match.StartIndex {
			return x.Match.StartIndex < y.Match.StartIndex
		}
		if rank[x.Kind] != rank[y.Kind] {
			return rank[x.Kind] < rank[y.Kind]
		}
		if x.NoteCount() != y.NoteCount() {
			return x.NoteCount() > y.NoteCount()
		}
		return x.ShiftSemitones < y.ShiftSemitones
	})
}

// NoteCount 返回参与该 finding 的每段音符数。
func (f Finding) NoteCount() int { return f.Prototype.NoteCount }

// extendIdentity 返回从起点对 (i, j) 出发满足「音程序列逐位相等 + 节奏
// 逐位相等」的最大段长（音符数）。原型段不得越过匹配段起点（i+m < j）。
func extendIdentity(ns []Note, i, j int) int {
	if !floatEq(ns[i].Length, ns[j].Length) {
		return 0
	}
	m := 1
	for i+m < j && j+m < len(ns) {
		if ns[i+m].Pitch-ns[i+m-1].Pitch != ns[j+m].Pitch-ns[j+m-1].Pitch {
			break
		}
		if !rhythmStepEq(ns, i, j, m) {
			break
		}
		m++
	}
	return m
}

// extendInversion 返回从起点对 (i, j) 出发满足「匹配段音程序列 = 原型段
// 逐位取反 + 节奏逐位相等」的最大段长。
func extendInversion(ns []Note, i, j int) int {
	if !floatEq(ns[i].Length, ns[j].Length) {
		return 0
	}
	m := 1
	for i+m < j && j+m < len(ns) {
		if ns[j+m].Pitch-ns[j+m-1].Pitch != -(ns[i+m].Pitch - ns[i+m-1].Pitch) {
			break
		}
		if !rhythmStepEq(ns, i, j, m) {
			break
		}
		m++
	}
	return m
}

// extendDiatonic 返回从起点对 (i, j) 出发满足「两段所有音在调内且绝对
// 音级差恒定 + 节奏逐位相等」的最大段长及音级差。首音对调外或音级差为 0
// （恒等域，归 repeat）时返回 (0, 0)。
func extendDiatonic(ns []Note, i, j int, pk preparedKey) (int, int) {
	da, okA := pk.absDeg(ns[i].Pitch)
	db, okB := pk.absDeg(ns[j].Pitch)
	if !okA || !okB {
		return 0, 0
	}
	deg := db - da
	if deg == 0 {
		return 0, 0
	}
	if !floatEq(ns[i].Length, ns[j].Length) {
		return 0, 0
	}
	m := 1
	for i+m < j && j+m < len(ns) {
		da, okA := pk.absDeg(ns[i+m].Pitch)
		db, okB := pk.absDeg(ns[j+m].Pitch)
		if !okA || !okB {
			break
		}
		if db-da != deg {
			break
		}
		if !rhythmStepEq(ns, i, j, m) {
			break
		}
		m++
	}
	return m, deg
}

// extendRetrograde 用枚举验证返回起点对 (i, j) 上满足「匹配段音高序列 =
// 原型段逆序 + 节奏镜像」的最大段长；不小于 minNotes 的最大值，无则 -1。
func extendRetrograde(ns []Note, i, j, minNotes int) int {
	maxN := len(ns) - j
	if j-i < maxN {
		maxN = j - i
	}
	for m := maxN; m >= minNotes; m-- {
		if retrogradeMatch(ns, i, j, m) {
			return m
		}
	}
	return -1
}

func retrogradeMatch(ns []Note, i, j, m int) bool {
	for t := 0; t < m; t++ {
		a := ns[i+m-1-t] // 原型段逆序位置 t
		b := ns[j+t]     // 匹配段位置 t
		if a.Pitch != b.Pitch {
			return false
		}
		if !floatEq(a.Length, b.Length) {
			return false
		}
		// IOI 镜像：匹配段 IOI[t] = 原型段 IOI[m-2-t]
		if t+1 < m {
			ioiA := a.Start - ns[i+m-2-t].Start
			ioiB := ns[j+t+1].Start - b.Start
			if !floatEq(ioiA, ioiB) {
				return false
			}
		}
	}
	return true
}

// rhythmStepEq 检查把第 m 步（索引 i+m / j+m 的音）纳入两段后节奏仍逐位
// 相等：IOI[m-1] 相等 + 第 m 音长度相等。
func rhythmStepEq(ns []Note, i, j, m int) bool {
	if !floatEq(ns[i+m].Start-ns[i+m-1].Start, ns[j+m].Start-ns[j+m-1].Start) {
		return false
	}
	return floatEq(ns[i+m].Length, ns[j+m].Length)
}
