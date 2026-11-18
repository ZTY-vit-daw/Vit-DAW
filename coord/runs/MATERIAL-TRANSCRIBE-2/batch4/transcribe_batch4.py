# -*- coding: utf-8 -*-
"""MATERIAL-TRANSCRIBE-2 batch4: 教材谱例转写（5 条）+ SMF 回读核对。

来源页：
- pdf-034_book-029.png → 编创实践 P29：即兴一~四（development examples）
- pdf-035_book-030.png → 三十里铺（陕西民歌）旋律前两句 8 小节

SMF format 0 / 单轨 / 480 PPQ。status=transcribed_unverified（听核未做）。
"""
import json, struct, hashlib, os, sys, math

PPQ = 480
HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..', '..', '..'))
ASSETS_MIDI = os.path.join(ROOT, 'coord', 'lesson-designs', 'music-creation-unit2', 'assets', 'midi')
Q, E, S, H, W = 480, 240, 120, 960, 1920
DE, DQ = 360, 720
DH = 1440  # dotted half

def ev(events, tick, pitch, dur):
    events.append((tick, pitch, dur))

# 1=C 的简谱级数→MIDI 音高（与 develop-prototype 既有 demo 一致：1=C4=60）
C = {1:60, 2:62, 3:64, 4:65, 5:67, 6:69, 7:71, 'i':72, '2h':74, 'i_':60-12+12}

# ---------- 1. develop-ornament 即兴一：加花（P29，4/4 两小节） ----------
def develop_ornament():
    e = []
    # b1: 1 2 3 1 2 5（前两对八分 + 两个四分）
    ev(e, 0, C[1], E); ev(e, E, C[2], E)
    ev(e, 2*E, C[3], E); ev(e, 3*E, C[1], E)
    ev(e, 2*Q, C[2], Q); ev(e, 3*Q, C[5], Q)
    # b2: 3 2 3 6 5 -
    t = W
    ev(e, t, C[3], E); ev(e, t+E, C[2], E)
    ev(e, t+2*E, C[3], E); ev(e, t+3*E, C[6], E)
    ev(e, t+2*Q, C[5], H)
    return dict(events=e, bars=2, ts=(4,4), bpm=80, channel=0, total_ticks=2*W, trailing_rest=0,
                desc='ornament-demo')

# ---------- 2. develop-rhythm 即兴二：改变节奏（P29，4/4 两小节） ----------
def develop_rhythm():
    e = []
    # b1: 1 3 1 2 5.（1八分, 3四分, 1八分, 2八分, 5附点四分）
    ev(e, 0, C[1], E); ev(e, E, C[3], Q)
    ev(e, E+Q, C[1], E); ev(e, E+Q+E, C[2], E)
    ev(e, E+Q+2*E, C[5], DQ)
    # b2: 3 6 3 5 -
    t = W
    ev(e, t, C[3], E); ev(e, t+E, C[6], Q)
    ev(e, t+E+Q, C[3], E)
    ev(e, t+E+Q+E, C[5], H)
    return dict(events=e, bars=2, ts=(4,4), bpm=80, channel=0, total_ticks=2*W, trailing_rest=0,
                desc='rhythm-change-demo')

# ---------- 3. develop-augment-meter 即兴三：节奏放大并改变节拍（P29，3/4 七小节） ----------
def develop_augment():
    e = []
    bar = 3*Q  # 1440
    # 1 - - | 3 - 1 | 2 - - | 5 - - | 3 - - | 6 - 3 | 5 - -
    ev(e, 0*bar, C[1], DH)
    ev(e, 1*bar, C[3], H); ev(e, 1*bar+2*Q, C[1], Q)
    ev(e, 2*bar, C[2], DH)
    ev(e, 3*bar, C[5], DH)
    ev(e, 4*bar, C[3], DH)
    ev(e, 5*bar, C[6], H); ev(e, 5*bar+2*Q, C[3], Q)
    ev(e, 6*bar, C[5], DH)
    return dict(events=e, bars=7, ts=(3,4), bpm=80, channel=0, total_ticks=7*bar, trailing_rest=0,
                desc='augment-3_4-demo')

# ---------- 4. develop-shift-rhythm 即兴四：旋律移位并作节奏变化（P29，4/4 四小节） ----------
def develop_shift():
    e = []
    # b1/b2 = 原型 1 3 2 5 | 3 6 5 -
    for i, p in enumerate([C[1], C[3], C[2], C[5]]): ev(e, i*Q, p, Q)
    t = W
    ev(e, t, C[3], Q); ev(e, t+Q, C[6], Q); ev(e, t+2*Q, C[5], H)
    # b3: 4 6 4 5 i̇
    t = 2*W
    ev(e, t, C[4], E); ev(e, t+E, C[6], Q)
    ev(e, t+E+Q, C[4], E); ev(e, t+E+Q+E, C[5], Q); ev(e, t+E+Q+Q, C['i'], Q)
    # b4: 6 2̇. i̇ -
    t = 3*W
    ev(e, t, C[6], E); ev(e, t+E, C['2h'], DQ); ev(e, t+E+DQ, C['i'], H)
    return dict(events=e, bars=4, ts=(4,4), bpm=80, channel=0, total_ticks=4*W, trailing_rest=0,
                desc='shift-rhythm-demo')

# ---------- 5. reference-sanshilipu-melody 三十里铺前两句 8 小节（P30，1=C 2/4） ----------
def sanshilipu_melody():
    m = {1:60, 2:62, 3:64, 4:65, 5:67, 6:69, 7:71, 'i':72, '2h':74, '5h':79, '1h':72, '2hl':62}
    # 1=C4；高八度点：1̇=72, 2̇=74, 5̇=79
    e = []
    def phrase(t0):
        # b: 1̇ 2̇ 2̇（八分-四分-八分）
        ev(e, t0, m['1h'], E); ev(e, t0+E, m['2h'], Q); ev(e, t0+E+Q, m['2h'], E)
        t = t0 + 2*Q
        # b: 5̇ 1̇ 6（八分-四分-八分）
        ev(e, t, m['5h'], E); ev(e, t+E, m['1h'], Q); ev(e, t+E+Q, m[6], E)
        t += 2*Q
        # b: 5. 6 5 2（附点八分-十六分-八分-八分）
        ev(e, t, m[5], DE); ev(e, t+DE, m[6], S)
        ev(e, t+2*Q, m[5], E); ev(e, t+2*Q+E, m[2], E)
        t += 2*Q
        # b: 5 -（二分）
        ev(e, t, m[5], H)
    phrase(0)           # 第一句（4 小节）
    phrase(4*2*Q)       # 第二句（4 小节，同一曲调）
    return dict(events=e, bars=8, ts=(2,4), bpm=80, channel=0, total_ticks=8*2*Q, trailing_rest=0,
                desc='sanshilipu-first8-melody-demo')

# ---------- SMF 写出/回读（同批1-3） ----------
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
    dict(id='develop-ornament', fname='develop-ornament_jiaxing1_C_80bpm.mid', fn=develop_ornament,
         page='P29(pdf-034)', title='即兴一：加花', ts=(4,4), bpm=80),
    dict(id='develop-rhythm', fname='develop-rhythm_change1_C_80bpm.mid', fn=develop_rhythm,
         page='P29(pdf-034)', title='即兴二：改变节奏', ts=(4,4), bpm=80),
    dict(id='develop-augment-meter', fname='develop-augment-meter_3-4_C_80bpm.mid', fn=develop_augment,
         page='P29(pdf-034)', title='即兴三：节奏放大并改变节拍', ts=(3,4), bpm=80),
    dict(id='develop-shift-rhythm', fname='develop-shift-rhythm_yiwei1_C_80bpm.mid', fn=develop_shift,
         page='P29(pdf-034)', title='即兴四：旋律移位并作节奏变化', ts=(4,4), bpm=80),
    dict(id='reference-sanshilipu-melody', fname='reference-sanshilipu-melody_first8_C_80bpm.mid', fn=sanshilipu_melody,
         page='P30(pdf-035)', title='三十里铺前两句旋律', ts=(2,4), bpm=80),
]

UNCERTAIN = {
 'develop-ornament': [
   '与既有 checked 素材 develop-prototype 同页配对；原型 1 3 2 5｜3 6 5 - 的 C 调/80BPM 演示设定沿用（原页无调号/BPM）',
   'b1「1 2」「3 1」为八分音对，其后 2、5 为四分；b2「3 2」「3 6」八分音对，5 - 为二分——减时线/时值按图面读取',
   '无八度点/变音记号；歌词（即兴内容）不入 MIDI',
 ],
 'develop-rhythm': [
   '与既有 checked 素材 develop-prototype 同页配对；C 调/80BPM 为演示设定',
   'b1「1」八分、「3」四分、「1 2」八分对、「5.」附点四分；b2「3」八分、「6」四分、「3」八分、「5 -」二分——按图面减时线/点符读取',
   '无八度点/变音记号；歌词不入 MIDI',
 ],
 'develop-augment-meter': [
   '3/4 与完整时值关系按图面「1 - -」「3 - 1」「6 - 3」读取（附点二分/二分+四分）；原页标 3/4 清晰',
   '「节奏放大并改变节拍」与原型（4/4 四分音符）的关系为教材设计，本 MIDI 只记即兴三本身',
   'C 调/80BPM 为演示设定（与原型 demo 一致）；无八度点/变音记号',
 ],
 'develop-shift-rhythm': [
   'b1-b2 = 原型 1 3 2 5｜3 6 5 -（与 checked develop-prototype 音高一致：60 64 62 67｜64 69 67）',
   'b3「4 6 4 5 i̇」：第 1、3 音为八分（减时线），6、5、i̇ 为四分；i̇ 为高八度 1（72）——高八度点按图面读取',
   'b4「6 2̇. i̇ -」：6 八分、2̇. 附点四分、i̇- 二分（延音线）；2̇/i̇ 高八度点清晰',
   'C 调/80BPM 为演示设定；移位与原型的对应关系按教材说明保留，不额外改写',
 ],
 'reference-sanshilipu-melody': [
   '取原曲前两句 8 小节（每句 4 小节，两句曲调相同；与既有 checked「sanshilipu-first8_rhythm-only」的 22 音节奏模板逐音对应）',
   '高八度点按图面读取：第一句 1̇ 2̇ 2̇、5̇ 1̇ 6；第二句同；若点符实为其他记号则音区不同——候对谱复核',
   'b3「5. 6 5 2」为附点八分+十六分+两个八分（减时线一层+局部二层）；b4「5 -」为二分',
   '1=C、2/4、「慢」清晰；80BPM 为演示设定（与既有 rhythm-only 参照的 80BPM 一致）；歌词不入 MIDI',
 ],
}

def main():
    report = dict(schema='vit.material_transcribe_batch.v0', batch=4, ppq=PPQ,
                  scope='教材印刷教学摘录（P29 即兴一~四；P30 三十里铺前两句 8 小节），不超出图面所示小节',
                  status_rule='全部 transcribed_unverified（人耳/对谱听核未做，绝不写 checked）',
                  reading_crops='coord/runs/MATERIAL-TRANSCRIBE-2/batch4/crops_p34, crops_p35',
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
    with open(os.path.join(HERE, 'verification_batch4.json'), 'w', encoding='utf-8') as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    all_ok = all(r['result'] == 'PASS' for r in report['items'])
    print('BATCH4:', 'ALL PASS' if all_ok else 'HAS FAIL')
    return 0 if all_ok else 1

if __name__ == '__main__':
    sys.exit(main())
