package legamedesign

// T-13 判定参照输入：最小标准 MIDI（SMF format 0/1）解析为卡 notes 表（beats 域）。
//
// 坐标约定=卡 v0：start/length 单位=clip 内 beats（四分音符=1 beat），
// 由 SMF division（ticks per quarter note）换算 beats=ticks/division。
// 解析是确定性的纯语法映射，不做音乐推断；不支持 SMPTE division/format 2 与
// 未闭合音符（fail-closed，不乱补）。

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// parseSMFNotes 解析 SMF 字节流为按 (start,pitch) 排序的 notes 表。
func parseSMFNotes(data []byte) ([]cardNote, error) {
	if len(data) < 14 || string(data[0:4]) != "MThd" {
		return nil, fmt.Errorf("SMF: 缺 MThd 头")
	}
	headerLen := int(binary.BigEndian.Uint32(data[4:8]))
	if headerLen < 6 || 8+headerLen > len(data) {
		return nil, fmt.Errorf("SMF: header 长度非法（%d）", headerLen)
	}
	format := binary.BigEndian.Uint16(data[8:10])
	division := binary.BigEndian.Uint16(data[12:14])
	if format > 1 {
		return nil, fmt.Errorf("SMF: 不支持 format %d（v0 只认 0/1）", format)
	}
	if division == 0 {
		return nil, fmt.Errorf("SMF: division=0")
	}
	if division&0x8000 != 0 {
		return nil, fmt.Errorf("SMF: 不支持 SMPTE division（0x%04x）", division)
	}

	pos := 8 + headerLen
	notes := make([]cardNote, 0, 64)
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
		trackNotes, err := parseSMFTrack(body, division)
		if err != nil {
			return nil, fmt.Errorf("SMF: 轨道 %d 解析失败: %w", tracks, err)
		}
		notes = append(notes, trackNotes...)
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
	return notes, nil
}

// parseSMFTrack 解析单个 MTrk body（delta VLQ + 事件；支持 running status）。
func parseSMFTrack(body []byte, division uint16) ([]cardNote, error) {
	pos := 0
	tick := int64(0)
	var runningStatus byte
	type openNote struct{ startTick int64 }
	open := map[[2]int]openNote{}
	notes := make([]cardNote, 0, 32)

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

	for pos < len(body) {
		delta, err := readVLQ()
		if err != nil {
			return nil, err
		}
		tick += delta
		if pos >= len(body) {
			return nil, fmt.Errorf("事件状态字节截断")
		}
		statusByte := body[pos]
		if statusByte&0x80 != 0 {
			runningStatus = statusByte
			pos++
		} else if runningStatus == 0 {
			return nil, fmt.Errorf("running status 前缺状态字节")
		}
		status := runningStatus
		switch {
		case status == 0xFF: // meta：type byte + VLQ length + data
			if pos >= len(body) {
				return nil, fmt.Errorf("meta 事件截断")
			}
			pos++ // meta type（不消费语义）
			length, err := readVLQ()
			if err != nil {
				return nil, err
			}
			if pos+int(length) > len(body) {
				return nil, fmt.Errorf("meta 数据越界")
			}
			pos += int(length)
		case status == 0xF0 || status == 0xF7: // sysex：VLQ length + data
			length, err := readVLQ()
			if err != nil {
				return nil, err
			}
			if pos+int(length) > len(body) {
				return nil, fmt.Errorf("sysex 数据越界")
			}
			pos += int(length)
		default:
			kind := status & 0xF0
			channel := int(status & 0x0F)
			dataLen := 0
			switch kind {
			case 0x80, 0x90, 0xA0, 0xB0, 0xE0:
				dataLen = 2
			case 0xC0, 0xD0:
				dataLen = 1
			default:
				return nil, fmt.Errorf("未知状态字节 0x%02x", status)
			}
			if pos+dataLen > len(body) {
				return nil, fmt.Errorf("事件数据截断（状态 0x%02x）", status)
			}
			data1 := int(body[pos])
			data2 := 0
			if dataLen == 2 {
				data2 = int(body[pos+1])
			}
			pos += dataLen
			key := [2]int{channel, data1}
			switch {
			case kind == 0x90 && data2 > 0:
				if _, dup := open[key]; dup {
					return nil, fmt.Errorf("同音重复 note_on 未闭合（channel=%d note=%d）", channel, data1)
				}
				open[key] = openNote{startTick: tick}
			case kind == 0x80 || (kind == 0x90 && data2 == 0):
				start, ok := open[key]
				if !ok {
					return nil, fmt.Errorf("note_off 无对应 note_on（channel=%d note=%d）", channel, data1)
				}
				delete(open, key)
				lengthTicks := tick - start.startTick
				if lengthTicks <= 0 {
					return nil, fmt.Errorf("音符时长为 0 ticks（channel=%d note=%d）", channel, data1)
				}
				notes = append(notes, cardNote{
					Pitch:  data1,
					Start:  float64(start.startTick) / float64(division),
					Length: float64(lengthTicks) / float64(division),
				})
			}
		}
	}
	if len(open) > 0 {
		return nil, fmt.Errorf("轨道结束时仍有 %d 个未闭合音符", len(open))
	}
	return notes, nil
}
