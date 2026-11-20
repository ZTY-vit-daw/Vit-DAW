package tutorsim

// TUTOR-SIM-4（池序 438）：教学材料 .mid 的最小只读解析。
//
// 仓库既有 Go MIDI 解析面（legamedesign/compile_midi.go、genctx/validate.go）
// 要么不导出音符时值，要么不导出音符列表；本文件按同一既证语法面（SMF
// format 0/1、metrical division、delta VLQ + running status、note on/off
// 配对）做 tutorsim 包内最小解析，坐标域=clip 内 beats（四分音符=1 beat，
// beats=ticks/division）。解析是确定性纯语法映射，不做音乐推断；不支持
// SMPTE division / format 2 / 未闭合音符（fail-closed，不乱补）。

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// MidiNote is one parsed note in the beats domain (quarter note = 1 beat).
type MidiNote struct {
	Pitch    int
	Start    float64
	Length   float64
	Velocity int
}

// MidiSummary is the deterministic summary the UI material card shows.
type MidiSummary struct {
	NoteCount     int
	DurationBeats float64
	TempoBPM      float64
	Division      int
}

// MidiDocument is a parsed .mid: note list plus summary.
type MidiDocument struct {
	Summary MidiSummary
	Notes   []MidiNote
}

// SummarizeMidi parses data and returns only the summary face.
func SummarizeMidi(data []byte) (MidiSummary, error) {
	doc, err := ParseMidi(data)
	if err != nil {
		return MidiSummary{}, err
	}
	return doc.Summary, nil
}

// ParseMidi parses a minimal SMF (format 0/1, metrical division) into notes
// sorted by (start, pitch) plus the deterministic summary.
func ParseMidi(data []byte) (*MidiDocument, error) {
	if len(data) < 14 || string(data[0:4]) != "MThd" {
		return nil, fmt.Errorf("SMF: 缺 MThd 头")
	}
	headerLen := int(binary.BigEndian.Uint32(data[4:8]))
	if headerLen < 6 || 8+headerLen > len(data) {
		return nil, fmt.Errorf("SMF: header 长度非法（%d）", headerLen)
	}
	format := binary.BigEndian.Uint16(data[8:10])
	division := int(binary.BigEndian.Uint16(data[12:14]))
	if format > 1 {
		return nil, fmt.Errorf("SMF: 不支持 format %d（只认 0/1）", format)
	}
	if division == 0 {
		return nil, fmt.Errorf("SMF: division=0")
	}
	if division&0x8000 != 0 {
		return nil, fmt.Errorf("SMF: 不支持 SMPTE division（0x%04x）", division)
	}

	pos := 8 + headerLen
	var notes []MidiNote
	tempoUSPerQN := int64(0)
	tracks := 0
	for pos < len(data) {
		if pos+8 > len(data) {
			return nil, fmt.Errorf("SMF: chunk 头截断")
		}
		chunkType := string(data[pos : pos+4])
		chunkLen := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		pos += 8
		if chunkLen < 0 || pos+chunkLen > len(data) {
			return nil, fmt.Errorf("SMF: chunk %q 长度越界（%d）", chunkType, chunkLen)
		}
		body := data[pos : pos+chunkLen]
		pos += chunkLen
		if chunkType != "MTrk" {
			continue // 未知 chunk 跳过（SMF 规范允许）
		}
		tracks++
		trackNotes, trackTempo, err := parseMidiTrack(body, division)
		if err != nil {
			return nil, fmt.Errorf("SMF: 轨道 %d 解析失败: %w", tracks, err)
		}
		notes = append(notes, trackNotes...)
		if tempoUSPerQN == 0 && trackTempo != 0 {
			tempoUSPerQN = trackTempo
		}
	}
	if tracks == 0 {
		return nil, fmt.Errorf("SMF: 无 MTrk 轨道")
	}
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].Start != notes[j].Start {
			return notes[i].Start < notes[j].Start
		}
		return notes[i].Pitch < notes[j].Pitch
	})
	duration := 0.0
	for _, note := range notes {
		if end := note.Start + note.Length; end > duration {
			duration = end
		}
	}
	tempoBPM := 120.0 // SMF 缺省（500000 µs/qn）直到 tempo meta 声明
	if tempoUSPerQN > 0 {
		tempoBPM = float64(60_000_000) / float64(tempoUSPerQN)
	}
	return &MidiDocument{
		Summary: MidiSummary{
			NoteCount:     len(notes),
			DurationBeats: duration,
			TempoBPM:      tempoBPM,
			Division:      division,
		},
		Notes: notes,
	}, nil
}

// parseMidiTrack 解析单个 MTrk body；返回 notes（beats 域）与首个 tempo
// meta（µs/qn，无则 0）。
func parseMidiTrack(body []byte, division int) ([]MidiNote, int64, error) {
	pos := 0
	tick := int64(0)
	var runningStatus byte
	tempoUSPerQN := int64(0)
	type openNote struct {
		startTick int64
		velocity  int
	}
	open := map[[2]int]openNote{} // (channel, pitch) -> open note
	notes := make([]MidiNote, 0, 32)

	readVLQ := func() (int64, error) {
		var value int64
		for i := 0; i < 4; i++ {
			if pos >= len(body) {
				return 0, fmt.Errorf("VLQ 截断")
			}
			b := body[pos]
			pos++
			value = (value << 7) | int64(b&0x7F)
			if b&0x80 == 0 {
				return value, nil
			}
		}
		return 0, fmt.Errorf("VLQ 超过 4 字节")
	}

	closeNote := func(channel, pitch int, endTick int64, velocity int) {
		key := [2]int{channel, pitch}
		if opened, ok := open[key]; ok {
			lengthTicks := endTick - opened.startTick
			if lengthTicks < 0 {
				lengthTicks = 0
			}
			notes = append(notes, MidiNote{
				Pitch:    pitch,
				Start:    float64(opened.startTick) / float64(division),
				Length:   float64(lengthTicks) / float64(division),
				Velocity: opened.velocity,
			})
			delete(open, key)
		}
	}

	for pos < len(body) {
		delta, err := readVLQ()
		if err != nil {
			return nil, 0, err
		}
		tick += delta
		if pos >= len(body) {
			return nil, 0, fmt.Errorf("事件状态字节截断")
		}
		statusByte := body[pos]
		if statusByte&0x80 != 0 {
			runningStatus = statusByte
			pos++
		} else if runningStatus == 0 {
			return nil, 0, fmt.Errorf("running status 前缺状态字节")
		}
		status := runningStatus
		switch {
		case status == 0xFF: // meta：type byte + VLQ length + data
			if pos >= len(body) {
				return nil, 0, fmt.Errorf("meta 事件截断")
			}
			metaType := body[pos]
			pos++
			metaLen, err := readVLQ()
			if err != nil {
				return nil, 0, err
			}
			if metaLen < 0 || pos+int(metaLen) > len(body) {
				return nil, 0, fmt.Errorf("meta 数据越界（type 0x%02x）", metaType)
			}
			if metaType == 0x51 && metaLen == 3 {
				tempoUSPerQN = int64(body[pos])<<16 | int64(body[pos+1])<<8 | int64(body[pos+2])
			}
			pos += int(metaLen)
		case status == 0xF0 || status == 0xF7: // SysEx：VLQ length + data，跳过
			length, err := readVLQ()
			if err != nil {
				return nil, 0, err
			}
			if length < 0 || pos+int(length) > len(body) {
				return nil, 0, fmt.Errorf("SysEx 数据越界")
			}
			pos += int(length)
		case status&0xF0 == 0x90: // note on
			if pos+2 > len(body) {
				return nil, 0, fmt.Errorf("note on 数据截断")
			}
			pitch := int(body[pos])
			velocity := int(body[pos+1])
			pos += 2
			channel := int(status & 0x0F)
			if velocity == 0 {
				closeNote(channel, pitch, tick, velocity)
			} else if _, dup := open[[2]int{channel, pitch}]; dup {
				return nil, 0, fmt.Errorf("同一音符重复开启（channel %d pitch %d）", channel, pitch)
			} else {
				open[[2]int{channel, pitch}] = openNote{startTick: tick, velocity: velocity}
			}
		case status&0xF0 == 0x80: // note off
			if pos+2 > len(body) {
				return nil, 0, fmt.Errorf("note off 数据截断")
			}
			pitch := int(body[pos])
			pos += 2
			closeNote(int(status&0x0F), pitch, tick, 0)
		case status&0xF0 == 0xA0 || status&0xF0 == 0xB0 || status&0xF0 == 0xE0: // 2 数据字节
			if pos+2 > len(body) {
				return nil, 0, fmt.Errorf("通道事件数据截断（status 0x%02x）", status)
			}
			pos += 2
		case status&0xF0 == 0xC0 || status&0xF0 == 0xD0: // 1 数据字节
			if pos+1 > len(body) {
				return nil, 0, fmt.Errorf("通道事件数据截断（status 0x%02x）", status)
			}
			pos++
		default:
			return nil, 0, fmt.Errorf("不支持的状态字节 0x%02x", status)
		}
	}
	if len(open) > 0 {
		return nil, 0, fmt.Errorf("%d 个音符未闭合（fail-closed）", len(open))
	}
	return notes, tempoUSPerQN, nil
}
