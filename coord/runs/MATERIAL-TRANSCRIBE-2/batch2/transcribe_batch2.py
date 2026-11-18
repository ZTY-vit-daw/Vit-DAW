# -*- coding: utf-8 -*-
"""MATERIAL-TRANSCRIBE-2 batch2: 教材谱例转写（5 条）+ SMF 回读核对。

来源：coord/lesson-designs/music-creation-unit2/source/pages/（用户教材页缓存）
- pdf-026_book-021.png → 节奏四例（X 谱，无音高）
- pdf-027_book-022.png → 二月里来（自由曲线）、乘胜追击（平直线）

SMF format 0 / 单轨 / 480 PPQ（随仓库 seed-midi-manifest 惯例）。
status 一律 transcribed_unverified（听核未做）。uncertain 逐条记录，不猜不编。
"""
import json, struct, hashlib, os, sys, math

PPQ = 480
HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..', '..', '..'))
ASSETS_MIDI = os.path.join(ROOT, 'coord', 'lesson-designs', 'music-creation-unit2', 'assets', 'midi')
RUN_DIR = HERE

Q, E, S, H, W = 480, 240, 120, 960, 1920   # quarter/eighth/16th/half/whole
DE, DQ = 360, 720                           # dotted eighth / dotted quarter

def ev(events, tick, pitch, dur):
    events.append((tick, pitch, dur))

def jianpu_map(key_pc):
    base = 60 + key_pc
    m = {1: base, 2: base+2, 3: base+4, 4: base+5, 5: base+7, 6: base+9, 7: base+11,
         'i': base+12}
    for n, off in [(1,-12),(2,-10),(3,-8),(4,-7),(5,-5),(6,-3),(7,-1)]:
        m[str(n)+'_'] = base+off
    m['#2'] = base+3; m['#4'] = base+6; m['#5'] = base+8
    return m

# ---------- 1. rhythm-even 均分节奏（P21，第1-4小节；第5小节三连音未选入） ----------
def rhythm_even():
    e = []
    ev(e, 0, 37, W)                       # b1 全音符
    ev(e, W, 37, H); ev(e, W+H, 37, H)    # b2 两二分
    for i in range(4): ev(e, 2*W + i*Q, 37, Q)   # b3 四四分
    for i in range(8): ev(e, 3*W + i*E, 37, E)   # b4 八八分
    return dict(events=e, bars=4, ts=(4,4), bpm=80, channel=9, desc='uniform-4bars')

# ---------- 2. rhythm-long-short 前长后短（P21 全 4 小节） ----------
def rhythm_long_short():
    e = []
    # b1 X - - XX : 四分+二分延长+两八分
    ev(e, 0, 37, 3*Q); ev(e, 3*Q, 37, E); ev(e, 3*Q+E, 37, E)
    # b2 X - XX XX : 二分+两对八分
    t = W
    ev(e, t, 37, 2*Q)
    for i in range(4): ev(e, t+2*Q+i*E, 37, E)
    # b3 X. X X. XX : 附点四分+八分+附点四分+两十六分
    t = 2*W
    ev(e, t, 37, DQ); ev(e, t+DQ, 37, E); ev(e, t+DQ+E, 37, DQ)
    ev(e, t+DQ+E+DQ, 37, S); ev(e, t+DQ+E+DQ+S, 37, S)
    # b4 [八分+两十六分][附点八分+十六分][八分+两十六分][四十六分]
    t = 3*W
    ev(e, t, 37, E); ev(e, t+E, 37, S); ev(e, t+E+S, 37, S)
    t += Q
    ev(e, t, 37, DE); ev(e, t+DE, 37, S)
    t += Q
    ev(e, t, 37, E); ev(e, t+E, 37, S); ev(e, t+E+S, 37, S)
    t += Q
    for i in range(4): ev(e, t+i*S, 37, S)
    return dict(events=e, bars=4, ts=(4,4), bpm=80, channel=9, desc='long-short-4bars')

# ---------- 3. rhythm-short-long 前短后长（P21 全 4 小节） ----------
def rhythm_short_long():
    e = []
    # b1 XX X - - : 两八分+四分+二分延长（合计 3 拍长音）
    ev(e, 0, 37, E); ev(e, E, 37, E); ev(e, 2*E, 37, 3*Q)
    # b2 八分 + 附点四分 + 两十六分 + 附点四分
    t = W
    ev(e, t, 37, E); ev(e, t+E, 37, DQ); ev(e, t+E+DQ, 37, S); ev(e, t+E+DQ+S, 37, S)
    ev(e, t+E+DQ+2*S, 37, DQ)
    # b3 两十六分 + 附点四分 + 八分 + 附点四分
    t = 2*W
    ev(e, t, 37, S); ev(e, t+S, 37, S); ev(e, t+2*S, 37, DQ)
    ev(e, t+2*S+DQ, 37, E); ev(e, t+2*S+DQ+E, 37, DQ)
    # b4 四十六分 + 四分 + [十六分+附点八分] + [两十六分+八分]
    t = 3*W
    for i in range(4): ev(e, t+i*S, 37, S)
    ev(e, t+4*S, 37, Q)
    t += 4*S + Q
    ev(e, t, 37, S); ev(e, t+S, 37, DE)
    t += Q
    ev(e, t, 37, S); ev(e, t+S, 37, S); ev(e, t+2*S, 37, E)
    return dict(events=e, bars=4, ts=(4,4), bpm=80, channel=9, desc='short-long-4bars')

# ---------- 4. rhythm-syncopation 切分节奏（P21 全 4 小节） ----------
def rhythm_syncopation():
    e = []
    # b1 四分 + 第2拍起二分（切分）+ 四分
    ev(e, 0, 37, Q); ev(e, Q, 37, 2*Q); ev(e, 3*Q, 37, Q)
    # b2 八分起 + 三连续四分 + 八分
    t = W
    ev(e, t, 37, E)
    for i in range(3): ev(e, t+E+i*Q, 37, Q)
    ev(e, t+E+3*Q, 37, E)
    # b3 八分 四分 八分 十六 十六 八分 四十六分
    t = 2*W
    ev(e, t, 37, E); ev(e, t+E, 37, Q); ev(e, t+E+Q, 37, E)
    ev(e, t+E+Q+E, 37, S); ev(e, t+E+Q+E+S, 37, S); ev(e, t+E+Q+E+2*S, 37, E)
    for i in range(4): ev(e, t+2*Q+2*S+E+i*S, 37, S)
    # b4 [十六+八分+十六][十六+附点八分][十六+十六+八分][十六+十六+八分]
    t = 3*W
    ev(e, t, 37, S); ev(e, t+S, 37, E); ev(e, t+S+E, 37, S)
    t += Q
    ev(e, t, 37, S); ev(e, t+S, 37, DE)
    t += Q
    ev(e, t, 37, S); ev(e, t+S, 37, S); ev(e, t+2*S, 37, E)
    t += Q
    ev(e, t, 37, S); ev(e, t+S, 37, S); ev(e, t+2*S, 37, E)
    return dict(events=e, bars=4, ts=(4,4), bpm=80, channel=9, desc='syncopation-4bars')

# ---------- 5. contour-free 二月里来（P22，1=♭A 4/4，4 小节） ----------
def contour_free():
    m = jianpu_map(8)  # 1=A♭
    e = []
    # b1: 2 1 2 3 | [装饰 5 3] 2. 3
    ev(e, 0, m[2], E); ev(e, E, m[1], E); ev(e, 2*E, m[2], E); ev(e, 3*E, m[3], S)
    ev(e, 3*E+S, m[5], S); ev(e, 3*E+2*S, m[3], S)      # 两装饰音（32 分）
    ev(e, 2*Q, m[2], DQ); ev(e, 2*Q+DQ, m[3], E)
    # b2: 6̣ 1 2 3 | [装饰 1 6] 5̣ -
    T = W
    ev(e, T, m['6_'], E); ev(e, T+E, m[1], E); ev(e, T+2*E, m[2], E); ev(e, T+3*E, m[3], S)
    ev(e, T+3*E+S, m[1], S); ev(e, T+3*E+2*S, m['6_'], S)
    ev(e, T+2*Q, m['5_'], H)
    # b3: 2. 3 5 3 2 3 1 2 6̣
    T = 2*W
    ev(e, T, m[2], DE); ev(e, T+DE, m[3], S); ev(e, T+2*Q, m[5], E); ev(e, T+2*Q+E, m[3], E)
    ev(e, T+3*Q, m[2], E); ev(e, T+3*Q+E, m[3], E)
    ev(e, T+4*Q, m[1], S); ev(e, T+4*Q+S, m[2], S); ev(e, T+4*Q+2*S, m['6_'], E)
    # b4: 3̣ 5̣ 6̣ 1 | [装饰 6̣] 5̣ -
    T = 3*W
    ev(e, T, m['3_'], E); ev(e, T+E, m['5_'], E); ev(e, T+2*E, m['6_'], E); ev(e, T+3*E, m[1], 3*S)
    ev(e, T+3*E+3*S, m['6_'], S)
    ev(e, T+2*Q, m['5_'], H)
    return dict(events=e, bars=4, ts=(4,4), bpm=72, channel=0, desc='february-first4_Ab')

# ---------- SMF 写出 / 回读（与批1同源，扩展 channel 与 track name） ----------
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
    dict(id='rhythm-even', fname='rhythm-even_uniform-4bars_GM37_80bpm.mid', fn=rhythm_even,
         page='P21(pdf-026)', title='原页均分节奏例', ts=(4,4), bpm=80),
    dict(id='rhythm-long-short', fname='rhythm-long-short_4bars_GM37_80bpm.mid', fn=rhythm_long_short,
         page='P21(pdf-026)', title='原页前长后短节奏例', ts=(4,4), bpm=80),
    dict(id='rhythm-short-long', fname='rhythm-short-long_4bars_GM37_80bpm.mid', fn=rhythm_short_long,
         page='P21(pdf-026)', title='原页前短后长节奏例', ts=(4,4), bpm=80),
    dict(id='rhythm-syncopation', fname='rhythm-syncopation_4bars_GM37_80bpm.mid', fn=rhythm_syncopation,
         page='P21(pdf-026)', title='原页切分节奏例', ts=(4,4), bpm=80),
    dict(id='contour-free', fname='contour-free_february-first4_Ab_72bpm.mid', fn=contour_free,
         page='P22(pdf-027)', title='二月里来', ts=(4,4), bpm=72),
]

UNCERTAIN = {
 'rhythm-even': [
   '卡面/目录原注要求「不含三连音」：图面第 5 小节为三连音例，未选入首包；本条取第 1-4 小节（全音符/二分/四分/八分递进）',
   'X 谱无音高与调号：MIDI 用 GM 37 边击（channel 10）单音高 80BPM 作节奏演示设定，不代表原谱音高（与既有 sanshilipu rhythm-only 惯例一致）',
   '小节线位置按像素检测核对（rhythm_symbols_p26.json bar1-bar4），全音符小节按 1 音 4 拍转写',
 ],
 'rhythm-long-short': [
   'X 谱无音高：GM37 单音高 80BPM 为节奏演示设定',
   '第 2 小节 X - 上方弧线按圆滑线记（不改时值、不合并）；若为延音线则时值分布不同——候对谱复核',
   '第 3/4 小节附点与十六分按像素级「点符+减时线层数」判定（rhythm_symbols_p26.json row2 bar3-b4）；第 4 小节为密集十六分组合，候听核/对谱复核',
 ],
 'rhythm-short-long': [
   'X 谱无音高：GM37 单音高 80BPM 为节奏演示设定',
   '第 2/3/4 小节「X.」无减时线处按附点四分（1.5 拍）读；若实为附点八分则时值分布不同——候对谱复核',
   '第 3 小节首组两音有双层减时线，按十六分读；第 4 小节附点八分+十六分组合按像素层数判定',
   '第 1 小节「X - -」按 3 拍延长音转写（简谱延长线惯例）',
 ],
 'rhythm-syncopation': [
   'X 谱无音高：GM37 单音高 80BPM 为节奏演示设定',
   '第 2 小节末八分与第 3 小节首八分之间的跨小节弧线按圆滑线记、未合并时值；若为延音线（切分连线）则应为 1 拍单音——候对谱复核',
   '第 3 小节 X 之间的内部弧线同样按圆滑线记、未合并；第 3/4 小节十六分密集组合按像素层数判定',
   '第 2 小节为首八分+三连续四分+末八分的后半拍切分：首音若为四分则重音布局不同',
 ],
 'contour-free': [
   '装饰音（b1 「5 3」、b2 「1 6」、b4 「6」）原谱未标时值，按 2 个/1 个 32 分音符置于主音前、占用上一音时值（演示实现；候听核调整）',
   '低八度点按图面点符位置读取：b2 的 6、5；b3 的 6；b4 的 3、5、6、5（该字体点符按批1惯例作低八度点，非顿音；若为顿音则音区不同）',
   'b3 「2. 3」按附点八分+十六分（减时线一层+局部二层）读；若 2. 为附点四分则节奏不同——候对谱复核',
   '弧线按圆滑线记、不改时值；歌词不入 MIDI；1=♭A 与 4/4 清晰',
 ],
}

def main():
    report = dict(schema='vit.material_transcribe_batch.v0', batch=2, ppq=PPQ,
                  scope='教材印刷教学摘录（P21 节奏四例 + P22 自由曲线例），不超出图面所示小节',
                  status_rule='全部 transcribed_unverified（人耳/对谱听核未做，绝不写 checked）',
                  reading_crops='coord/runs/MATERIAL-TRANSCRIBE-2/batch2/crops_p26, crops_p27',
                  items=[])
    os.makedirs(ASSETS_MIDI, exist_ok=True)
    for it in ITEMS:
        spec = it['fn']()
        path = os.path.join(ASSETS_MIDI, it['fname'])
        size = write_smf(path, spec['events'], spec['ts'], spec['bpm'], spec['channel'],
                         'MATERIAL-TRANSCRIBE-2 %s (%s %s BPM)' % (it['id'], 'GM37 ch10' if spec['channel']==9 else 'ch1', spec['bpm']))
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
        total_ticks = spec['bars'] * spec['ts'][0] * (4*Q // spec['ts'][1])
        trailing = total_ticks - end_tick
        ok = ok_count and ok_pitch and ok_dur and rb['ppq'] == PPQ and bpm_rb == spec['bpm'] and rb['ts'] == spec['ts'] and trailing == spec.get('trailing_rest', 0)
        rec = dict(id=it['id'], title=it['title'], page=it['page'], file='assets/midi/' + it['fname'],
                   bytes=size, sha256=sha, format=rb['format'], ppq=rb['ppq'], ts=f"{rb['ts'][0]}/{rb['ts'][1]}",
                   demo_bpm=spec['bpm'], bpm_readback=bpm_rb, channel=('10' if spec['channel']==9 else '1'),
                   bars=spec['bars'], end_tick=end_tick, total_ticks=total_ticks, trailing_rest_ticks=trailing,
                   end_quarter_beats=end_tick/PPQ, total_quarter_beats=total_ticks/PPQ,
                   note_count_expected=len(spec['events']), note_count_readback=len(rb['notes']),
                   pitches=got_pitches, durations_ticks=got_durs,
                   check_count=ok_count, check_pitch=ok_pitch, check_duration=ok_dur,
                   check_meta=(rb['ppq'] == PPQ and bpm_rb == spec['bpm'] and rb['ts'] == spec['ts']),
                   check_bars=(trailing == spec.get('trailing_rest', 0)),
                   result='PASS' if ok else 'FAIL', uncertain=UNCERTAIN[it['id']])
        report['items'].append(rec)
        print(f"[{it['id']}] {rec['result']} notes={len(rb['notes'])}/{len(spec['events'])} ts={rec['ts']} bpm={bpm_rb} ch={rec['channel']} end_q={rec['end_quarter_beats']} sha={sha[:12]}")
    with open(os.path.join(RUN_DIR, 'verification_batch2.json'), 'w', encoding='utf-8') as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    all_ok = all(r['result'] == 'PASS' for r in report['items'])
    print('BATCH2:', 'ALL PASS' if all_ok else 'HAS FAIL')
    return 0 if all_ok else 1

if __name__ == '__main__':
    sys.exit(main())