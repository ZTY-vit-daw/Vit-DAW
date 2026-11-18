# -*- coding: utf-8 -*-
"""MATERIAL-TRANSCRIBE-2 batch3: 教材谱例转写（5 条）+ SMF 回读核对。

来源页（coord/lesson-designs/music-creation-unit2/source/pages/）：
- pdf-027_book-022.png → 乘胜追击（平直线，P22）
- pdf-030_book-025.png → 早晨的问候（平缓进行，P25）
- pdf-031_book-026.png → 海港之夜（跌宕起伏，P26，取第一行）
- pdf-032_book-027.png → 怀念战友（创作示例，P27，取首句 4 小节）
- pdf-033_book-028.png → 小草（分析例，P28，取第一行 8 小节）

SMF format 0 / 单轨 / 480 PPQ。status 一律 transcribed_unverified（听核未做）。
"""
import json, struct, hashlib, os, sys, math

PPQ = 480
HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..', '..', '..'))
ASSETS_MIDI = os.path.join(ROOT, 'coord', 'lesson-designs', 'music-creation-unit2', 'assets', 'midi')
Q, E, S, H, W = 480, 240, 120, 960, 1920
DE, DQ = 360, 720

def ev(events, tick, pitch, dur):
    events.append((tick, pitch, dur))

def jianpu_map(key_pc):
    base = 60 + key_pc
    m = {1: base, 2: base+2, 3: base+4, 4: base+5, 5: base+7, 6: base+9, 7: base+11, 'i': base+12}
    for n, off in [(1,-12),(2,-10),(3,-8),(4,-7),(5,-5),(6,-3),(7,-1)]:
        m[str(n)+'_'] = base+off
    m['#1'] = base+1; m['#2'] = base+3; m['#4'] = base+6; m['#5'] = base+8
    return m

# ---------- 1. contour-flat 乘胜追击（P22，1=♭B 2/4，图面 8 小节） ----------
def contour_flat():
    m = jianpu_map(10)
    e = []
    B = 2*Q  # 960 ticks per 2/4 bar
    def group(t, p):
        ev(e, t, p, DE); ev(e, t+DE, m[3], S); ev(e, t+2*Q, m[3], E)
    group(0, m[3]); group(B, m[3])
    for i in range(4): ev(e, 2*B + i*E, m[3], E)
    ev(e, 3*B, m['#2'], E); ev(e, 3*B+E, m[3], E)
    group(4*B, m[6]); group(5*B, m[6])
    for i in range(4): ev(e, 6*B + i*E, m[6], E)
    ev(e, 7*B, m['#5'], E); ev(e, 7*B+E, m[6], E)
    return dict(events=e, bars=8, ts=(2,4), bpm=120, channel=0, total_ticks=8*B, trailing_rest=Q,
                desc='chengsheng-first8_Bb')

# ---------- 2. contour-smooth 早晨的问候（P25，1=C 3/4；弱起2个十六分+4小节） ----------
def contour_smooth():
    m = jianpu_map(0)
    e = []
    ev(e, 0, m[2]+12, S); ev(e, S, m[2]+12, S)        # 弱起 2̇2̇ 两个十六分
    t = 2*S                                            # bar1 起于 240
    ev(e, t, m[2]+12, DQ); ev(e, t+DQ, m[2]+12, E)     # 2̇. 2̇
    ev(e, t+DQ+E, m[2]+12, E); ev(e, t+DQ+2*E, m[2]+12, E)  # 2̇ 2̇
    t += 3*Q
    ev(e, t, m['#1']+12, DE); ev(e, t+DE, m[3]+12, S)  # ♯1̇. 3̇
    ev(e, t+2*Q, m[3]+12, DQ)                          # 3̇.
    ev(e, t+2*Q+DQ, m[6], S); ev(e, t+2*Q+DQ+S, m[7], S)  # 6 7
    t += 3*Q
    ev(e, t, m[1]+12, DQ); ev(e, t+DQ, m[1]+12, E)     # i̇. i̇
    ev(e, t+DQ+E, m[1]+12, E); ev(e, t+DQ+2*E, m[1]+12, E)  # i̇ i̇
    t += 3*Q
    ev(e, t, m[7], DE); ev(e, t+DE, m[2]+12, S)        # 7. 2̇
    ev(e, t+Q, m[2]+12, Q)                             # 2̇（延长记号）
    # t+3*Q 起为休止（四分休止）→ 结束 6000
    return dict(events=e, bars=4, ts=(3,4), bpm=72, channel=0, total_ticks=2*S+4*3*Q, trailing_rest=Q,
                desc='morning-first4_C')

# ---------- 3. contour-wave 海港之夜（P26，1=♭B 4/4；弱起+第一行4小节） ----------
def contour_wave():
    m = jianpu_map(10)
    e = []
    # 弱起: 0 3（八分休止 + 八分音）
    ev(e, E, m[3], E)
    t = 2*E  # bar1 起于 480
    ev(e, t, m[3]+12, Q)                       # 3̇
    ev(e, t+Q, m[3], DE); ev(e, t+Q+DE, m[3]+12, S)  # 3. 3̇
    ev(e, t+2*Q, m[2], Q)                      # 2
    ev(e, t+3*Q, m[2]+12, E); ev(e, t+3*Q+E, m[3], S)  # 2̇ 3（含八分休止在中间？）
    t += 4*Q
    ev(e, t, m[2]+12, Q)                       # 2̇
    ev(e, t+Q, m[3], DE); ev(e, t+Q+DE, m[3]+12, S)  # 3. 3̇
    ev(e, t+2*Q, m[1]+12, Q)                   # i̇
    ev(e, t+3*Q, m[1]+12, E); ev(e, t+3*Q+E, m[1]+12, S)  # i̇ i̇（中间八分休止）
    t += 4*Q
    ev(e, t, m[7], Q)                          # 7
    ev(e, t+Q, m[2]+12, DE); ev(e, t+Q+DE, m[1]+12, S)  # 2̇. i̇
    ev(e, t+2*Q, m[7], E); ev(e, t+2*Q+E, m[6], E)  # 7 6
    ev(e, t+3*Q, m[5], E); ev(e, t+3*Q+E, m[6], E)  # 5 6
    t += 4*Q
    ev(e, t, m[3], 3*Q)                        # 3 - -
    ev(e, t+3*Q+E, m[3], E)                    # 0 3（八分休止 + 八分音）
    return dict(events=e, bars=4, ts=(4,4), bpm=60, channel=0, total_ticks=2*E+4*4*Q, trailing_rest=0,
                desc='seaport-first4_Bb')

# ---------- 4. analysis-huainian 怀念战友首句（P27，1=♭B 2/4，4 小节） ----------
def analysis_huainian():
    m = jianpu_map(10)
    e = []
    K = 2*Q  # 2/4 bar = 960
    # b1: 0 22 5 5（八分休止 + 两个十六分 + 两个八分）
    ev(e, E, m[2], S); ev(e, E+S, m[2], S)
    ev(e, 2*E, m[5], E); ev(e, 2*E+E, m[5], E)
    # b2: 5. 6 7 i̇ 7
    t = K
    ev(e, t, m[5], DE); ev(e, t+DE, m[6], S)
    ev(e, t+2*Q, m[7], E); ev(e, t+2*Q+E, m[1]+12, S); ev(e, t+2*Q+E+S, m[7], S)
    # b3: 6 6 i̇ 7 6
    t = 2*K
    ev(e, t, m[6], S); ev(e, t+S, m[6], E); ev(e, t+S+E, m[1]+12, S)
    ev(e, t+2*Q, m[7], E); ev(e, t+2*Q+E, m[6], E)
    # b4: 5 -
    ev(e, 3*K, m[5], 2*Q)
    return dict(events=e, bars=4, ts=(2,4), bpm=72, channel=0, total_ticks=4*K, trailing_rest=0,
                desc='huainian-first4_Bb')

# ---------- 5. analysis-xiaocao 小草首行（P28，1=G 2/4，8 小节） ----------
def analysis_xiaocao():
    m = jianpu_map(7)
    e = []
    K = 2*Q
    def rest(t): pass
    # b1: 6̣ 6̣ 1 7̣（均八分）
    for i, p in enumerate([m['6_'], m['6_'], m[1], m['7_']]): ev(e, i*E, p, E)
    # b2: 6̣. 0
    ev(e, K, m['6_'], DQ)
    # b3: 6̣ 6̣ 3 2
    for i, p in enumerate([m['6_'], m['6_'], m[3], m[2]]): ev(e, 2*K + i*E, p, E)
    # b4: 3. 0
    ev(e, 3*K, m[3], DQ)
    # b5: 3 3 5 3
    for i, p in enumerate([m[3], m[3], m[5], m[3]]): ev(e, 4*K + i*E, p, E)
    # b6: 2 2 1 7̣ 7̣（后两音十六分）
    t = 5*K
    ev(e, t, m[2], E); ev(e, t+E, m[2], E); ev(e, t+2*E, m[1], E)
    ev(e, t+3*E, m['7_'], S); ev(e, t+3*E+S, m['7_'], S)
    # b7: 6̣ 0 5
    t = 6*K
    ev(e, t, m['6_'], Q); ev(e, t+Q+E, m[5], E)
    # b8: 3̣. 0
    ev(e, 7*K, m['3_'], DQ)
    return dict(events=e, bars=8, ts=(2,4), bpm=80, channel=0, total_ticks=8*K, trailing_rest=E,
                desc='xiaocao-first8_G')

# ---------- SMF 写出/回读（与批1/批2同源） ----------
def vlq(n):
    out = bytearray([n & 0x7F]); n >>= 7
    while n:
        out.insert(0, 0x80 | (n & 0x7F)); n >>= 7
    return bytes(out)

def write_smf(path, notes, ts, bpm, channel, name):
    body = bytearray()
    body += vlq(0) + b'\xff\x03' + vlq(len(name.encode('utf-8'))) + name.encode('utf-8')
    body += vlq(0) + bytes([0xFF, 0x58, 0x04, ts[0], int(round(math.log2(ts[1]))), 24, 8])
    body += vlq(0) + bytes([0xFF, 0x51, 0x03]) + struct.pack('>I', round(60_000_000 / bpm))[1:]
    flat = []
    for tick, pitch, dur in sorted(notes, key=lambda x: (x[0], x[1])):
        flat.append((tick, 'on', pitch)); flat.append((tick + dur, 'off', pitch))
    flat.sort(key=lambda x: (x[0], 0 if x[1] == 'off' else 1))
    cur = 0
    on = 0x90 | channel; off = 0x80 | channel
    for tick, kind, pitch in flat:
        body += vlq(tick - cur) + bytes([on if kind == 'on' else off, pitch, 80 if kind == 'on' else 0])
        cur = tick
    body += vlq(0) + bytes([0xFF, 0x2F, 0x00])
    with open(path, 'wb') as f:
        f.write(b'MThd' + struct.pack('>IHHH', 6, 0, 1, PPQ))
        f.write(b'MTrk' + struct.pack('>I', len(body)) + bytes(body))
    return os.path.getsize(path)

def read_smf(path):
    data = open(path, 'rb').read()
    assert data[:4] == b'MThd', 'header'
    fmt, ntrk, div = struct.unpack('>HHH', data[8:14])
    pos = 14
    assert data[pos:pos+4] == b'MTrk'
    tlen = struct.unpack('>I', data[pos+4:pos+8])[0]
    body = data[pos+8:pos+8+tlen]
    i = 0; cur = 0; notes = []; tempo = None; ts = None; opened = []; name = ''
    while i < len(body):
        d = 0
        while True:
            b = body[i]; i += 1; d = (d << 7) | (b & 0x7F)
            if not (b & 0x80): break
        cur += d
        st = body[i]
        if st == 0xFF:
            mt = body[i+1]; ln = body[i+2]; payload = body[i+3:i+3+ln]; i += 3 + ln
            if mt == 0x51: tempo = struct.unpack('>I', b'\x00' + payload)[0]
            if mt == 0x58: ts = (payload[0], 2 ** payload[1])
            if mt == 0x03: name = payload.decode('utf-8', 'replace')
        elif st in (0x90, 0x80, 0x99, 0x89):
            pitch, vel = body[i+1], body[i+2]; i += 3
            if st in (0x90, 0x99) and vel > 0:
                opened.append([cur, pitch])
            else:
                for k in range(len(opened)):
                    if opened[k][1] == pitch:
                        s, p = opened.pop(k)
                        notes.append((s, p, cur)); break
        else:
            i += 2
    return dict(format=fmt, ppq=div, tempo_us=tempo, ts=ts, notes=sorted(notes), name=name)

ITEMS = [
    dict(id='contour-flat', fname='contour-flat_chengsheng-first8_Bb_120bpm.mid', fn=contour_flat,
         page='P22(pdf-027)', title='乘胜追击', ts=(2,4), bpm=120),
    dict(id='contour-smooth', fname='contour-smooth_morning-first4_C_72bpm.mid', fn=contour_smooth,
         page='P25(pdf-030)', title='早晨的问候', ts=(3,4), bpm=72),
    dict(id='contour-wave', fname='contour-wave_seaport-first4_Bb_60bpm.mid', fn=contour_wave,
         page='P26(pdf-031)', title='海港之夜', ts=(4,4), bpm=60),
    dict(id='analysis-huainian', fname='analysis-huainian_first4_Bb_72bpm.mid', fn=analysis_huainian,
         page='P27(pdf-032)', title='怀念战友（首句）', ts=(2,4), bpm=72),
    dict(id='analysis-xiaocao', fname='analysis-xiaocao_first8_G_80bpm.mid', fn=analysis_xiaocao,
         page='P28(pdf-033)', title='小草（首行）', ts=(2,4), bpm=80),
]

UNCERTAIN = {
 'contour-flat': [
   '「0」为休止符：b1/b2 与前一八分同组（八分休止），b4/b8 独立（四分休止）；休止不入 MIDI 音符表',
   '♯2 / ♯5 变音记号清晰；按仅本音升号转写，相邻小节未示还原记号',
   '「3. 3」「6. 6」按附点八分+十六分读（点符+局部双层减时线）；若为附点四分+八分则节奏不同——候对谱复核',
   '2/4 与 1=♭B 清晰；120BPM 为演示设定（图面未标速度）；歌词不入 MIDI',
 ],
 'contour-smooth': [
   '弱起 2̇2̇ 双层减时线按两个十六分读（3/4 拍的半拍弱起）——若实为八分则弱起时值不同，候对谱复核',
   'b2 ♯1̇ 为临时升号（高八度 i̇ 加升号）；b2「6 7」按两个十六分读，6/7 无八度点',
   'b4 末音 2̇ 上方有延长记号（fermata），按四分音符记；延长时值未写入',
   '高八度点按图面点符位置读取（2̇/i̇/3̇/♯1̇）；弧线按圆滑线记、不改时值',
   '1=C、3/4 清晰；72BPM 为演示设定（图面未标速度）；歌词不入 MIDI',
 ],
 'contour-wave': [
   '摘取图面第一行（弱起 0 3 + 4 小节），未转写全曲；图面右侧「九度」标注对应到下一行首音',
   '弱起「0 3」按八分休止+八分音读；b1 末组「2̇ 0 3」中的「0」按十六分休止读（双层减时线）——若为八分休止则节奏不同',
   '高八度点按图面点符读取（3̇/2̇/i̇ 等）；3̇（高八度 3）与 3 的八度关系是「上下大跳」标注所指，候对谱复核',
   'b4 「3 - -」按附点二分（3 拍）读，末组「0 3」八分休止+八分音',
   '1=♭B、4/4、「慢速 抒情地」清晰；60BPM 为演示设定（图面未标速度）；歌词不入 MIDI',
 ],
 'analysis-huainian': [
   '仅取首句 4 小节（歌词“天山脚下是我可爱的家乡”），未转写整曲（卡面要求不转写整曲作为首包前提）',
   '图面小节上方的小数字/分数标记（如 ⁷/₆、⁴/₄）疑为和弦或分析标记，未用于旋律转写，候复核',
   'b1「0 22」中两个 2 为双层减时线（十六分），b1 首「0」为八分休止；b2/b3 附点与十六分按像素层数判定',
   '低/高八度点按图面点符读取（b2 i̇、b3 i̇）；弧线按圆滑线记、不改时值',
   '1=♭B、2/4、♩=72（图面标注）清晰；摘录各小节均为 2/4（该曲另有 3/4 变化段，未入本摘录）；歌词不入 MIDI',
 ],
 'analysis-xiaocao': [
   '仅取图面第一行 8 小节（“没有花香，没有树高，我是一棵无人知道的小草”），未转写整曲',
   '低八度点按图面点符读取：b1 6̣6̣7̣、b2 6̣、b3 6̣6̣、b7 6̣、b8 3̣；若为顿音记号则音区不同——批1 同字体惯例按低八度点处理',
   'b6 末两音 7̣7̣ 为双层减时线（十六分）；b7「6̣ 0 5」中 6̣ 为四分、0 为八分休止、5 为八分',
   'b8 3̣ 为附点四分+八分休止；3̣ 低八度点较淡（扫描 904px），候对谱复核',
   '1=G、2/4、「中速 纯朴地」清晰；80BPM 为演示设定（图面未标速度）；歌词不入 MIDI',
 ],
}

def main():
    report = dict(schema='vit.material_transcribe_batch.v0', batch=3, ppq=PPQ,
                  scope='教材印刷教学摘录（P22 平直线 / P25 平缓进行 / P26 第一行 / P27 首句 / P28 第一行），不超出图面所示小节',
                  status_rule='全部 transcribed_unverified（人耳/对谱听核未做，绝不写 checked）',
                  reading_crops='coord/runs/MATERIAL-TRANSCRIBE-2/batch3/crops_p25, crops_p26, crops_p27, crops_p28, crops_p32',
                  items=[])
    os.makedirs(ASSETS_MIDI, exist_ok=True)
    for it in ITEMS:
        spec = it['fn']()
        path = os.path.join(ASSETS_MIDI, it['fname'])
        size = write_smf(path, spec['events'], spec['ts'], spec['bpm'], spec['channel'],
                         'MATERIAL-TRANSCRIBE-2 %s (%s BPM)' % (it['id'], spec['bpm']))
        rb = read_smf(path)
        exp = sorted(spec['events'], key=lambda x: (x[0], x[1]))
        exp_pitches = [p for _, p, _ in exp]
        exp_durs = [d for _, _, d in exp]
        got_pitches = [p for _, p, _ in rb['notes']]
        got_durs = [c - s for s, p, c in rb['notes']]
        ok_count = len(rb['notes']) == len(spec['events'])
        ok_pitch = exp_pitches == got_pitches
        ok_dur = exp_durs == got_durs
        bpm_rb = round(60_000_000 / rb['tempo_us'], 1) if rb['tempo_us'] else None
        sha = hashlib.sha256(open(path, 'rb').read()).hexdigest()
        end_tick = max(t + d for t, p, d in spec['events'])
        total_ticks = spec['total_ticks']
        trailing = total_ticks - end_tick
        ok = ok_count and ok_pitch and ok_dur and rb['ppq'] == PPQ and bpm_rb == spec['bpm'] and rb['ts'] == spec['ts'] and trailing == spec['trailing_rest']
        rec = dict(id=it['id'], title=it['title'], page=it['page'], file='assets/midi/' + it['fname'],
                   bytes=size, sha256=sha, format=rb['format'], ppq=rb['ppq'], ts=f"{rb['ts'][0]}/{rb['ts'][1]}",
                   demo_bpm=spec['bpm'], bpm_readback=bpm_rb, channel='1',
                   bars=spec['bars'], end_tick=end_tick, total_ticks=total_ticks, trailing_rest_ticks=trailing,
                   end_quarter_beats=end_tick/PPQ, total_quarter_beats=total_ticks/PPQ,
                   note_count_expected=len(spec['events']), note_count_readback=len(rb['notes']),
                   pitches=got_pitches, durations_ticks=got_durs,
                   check_count=ok_count, check_pitch=ok_pitch, check_duration=ok_dur,
                   check_meta=(rb['ppq'] == PPQ and bpm_rb == spec['bpm'] and rb['ts'] == spec['ts']),
                   check_bars=(trailing == spec['trailing_rest']),
                   result='PASS' if ok else 'FAIL', uncertain=UNCERTAIN[it['id']])
        report['items'].append(rec)
        print(f"[{it['id']}] {rec['result']} notes={len(rb['notes'])}/{len(spec['events'])} ts={rec['ts']} bpm={bpm_rb} end_q={rec['end_quarter_beats']} sha={sha[:12]}")
    with open(os.path.join(HERE, 'verification_batch3.json'), 'w', encoding='utf-8') as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    all_ok = all(r['result'] == 'PASS' for r in report['items'])
    print('BATCH3:', 'ALL PASS' if all_ok else 'HAS FAIL')
    return 0 if all_ok else 1

if __name__ == '__main__':
    sys.exit(main())
