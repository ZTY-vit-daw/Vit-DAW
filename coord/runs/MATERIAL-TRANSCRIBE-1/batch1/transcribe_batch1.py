# -*- coding: utf-8 -*-
"""MATERIAL-TRANSCRIBE-1 batch1: 教材谱例转写（5 条）+ SMF 回读核对。

谱面来源：coord/lesson-designs/music-creation-unit2/source/pages/（用户教材页缓存）。
转写范围 = 教材页面印刷的教学摘录小节（第 17-20 页五例），不超出图面所示内容。
SMF format 0 / 单轨 / 480 PPQ（随仓库 seed-midi-manifest 惯例）。
诚实边界：uncertain 项逐条记录于 UNCERTAIN（不猜不编）；status 一律 transcribed_unverified（听核未做）。
"""
import json, struct, hashlib, os, sys

PPQ = 480
ASSETS_MIDI = r"D:\Vit_DAW\coord\lesson-designs\music-creation-unit2\assets\midi"
RUN_DIR = r"D:\Vit_DAW\coord\runs\MATERIAL-TRANSCRIBE-1\batch1"

# jianpu(1=X) → MIDI 音高换算：1=X4，点下=降八度，i=升八度
def jianpu(key_pc):  # key_pc: 1=C→0, 1=F→5, 1=G→7, 1=D→2
    base = 60 + key_pc  # 1 = X4
    m = {1: base, 2: base+2, 3: base+4, 4: base+5, 5: base+7, 6: base+9, 7: base+11}
    m['i'] = base + 12
    m['1_'] = base-12; m['2_'] = base-10; m['3_'] = base-8; m['4_'] = base-7
    m['5_'] = base-5;  m['6_'] = base-3;  m['7_'] = base-1
    m['#4'] = base+6; m['#5'] = base+8
    return m

Q, E, H, W = 480, 240, 960, 1920          # quarter/eighth/half/whole
DE = 360, 120                              # dotted-eighth, sixteenth
T8 = 160                                   # triplet eighth (1/3 beat)

def ev(events, tick, pitch, dur):
    events.append((tick, pitch, dur))

# ---------- 1. pitch-repeat 我向党来唱支歌 1=F 2/4（教材 P17 同音反复标注句，4 小节） ----------
def pitch_repeat():
    m = jianpu(5)  # 1=F
    evs = []
    # b1: 5 5 5 5（春 风 阵 阵，同音反复标注）
    for _ in range(4): ev(evs, 0, m[5], E)
    # b2: 5 5 6 5（吹 心 窝 哩）
    ev(evs, W, m[5], E); ev(evs, W+Q//2, m[5], E); ev(evs, W+Q, m[6], E); ev(evs, W+Q+Q//2, m[5], E)
    # b3: 2 2 2. 3（赛 罗 赛[附点+六分转音]）
    ev(evs, 2*W, m[2], E); ev(evs, 2*W+E, m[2], E); ev(evs, 2*W+Q, m[2], DE[0]); ev(evs, 2*W+Q+DE[0], m[3], DE[1])
    # b4: 1 1 5̣（赛 罗 赛，）
    ev(evs, 3*W, m[1], E); ev(evs, 3*W+E, m[1], E); ev(evs, 3*W+Q, m['5_'], Q)
    return dict(notes=evs, bars=4, ts=(2,4), bpm=96, key='F')

# ---------- 2. pitch-step-up 哆来咪 1=C 2/4（教材 P18 上行级进标注句，12 小节） ----------
def pitch_step_up():
    m = jianpu(0)  # 1=C
    evs = []
    def dq(t, p):  # 附点四分+八分 对（X. Y）
        ev(evs, t, p, Q+Q//2); ev(evs, t+Q+Q//2, None, 0)  # placeholder removed below
    # 直接逐小节写
    t = 0
    # b1: 5̣. 1
    ev(evs, t, m['5_'], Q+Q//2); ev(evs, t+Q+Q//2, m[1], Q//2); t += W
    # b2: 2 3 4 5
    for i, p in enumerate([2, 3, 4, 5]): ev(evs, t+i*Q//2, m[p], Q//2)
    t += W
    # b3: 6 - ；b4: 6 -（连）
    ev(evs, t, m[6], H); t += W
    ev(evs, t, m[6], H); t += W
    # b5: 6̣. 2
    ev(evs, t, m['6_'], Q+Q//2); ev(evs, t+Q+Q//2, m[2], Q//2); t += W
    # b6: 3 #4 5 6
    for i, p in enumerate([3, '#4', 5, 6]): ev(evs, t+i*Q//2, m[p], Q//2)
    t += W
    # b7: 7 - ；b8: 7 -
    ev(evs, t, m[7], H); t += W
    ev(evs, t, m[7], H); t += W
    # b9: 7̣. 3
    ev(evs, t, m['7_'], Q+Q//2); ev(evs, t+Q+Q//2, m[3], Q//2); t += W
    # b10: #4 #5 6 7
    for i, p in enumerate(['#4', '#5', 6, 7]): ev(evs, t+i*Q//2, m[p], Q//2)
    t += W
    # b11: i - ；b12: i -
    ev(evs, t, m['i'], H); t += W
    ev(evs, t, m['i'], H); t += W
    return dict(notes=[e for e in evs if e[1] is not None], bars=12, ts=(2,4), bpm=96, key='C')

# ---------- 3. pitch-step-down 小白菜 1=G 5/4（教材 P19 下行级进例，4 小节） ----------
def pitch_step_down():
    m = jianpu(7)  # 1=G
    evs = []
    # b1: 5 3 3 2 -（小白菜呀）
    ev(evs, 0, m[5], Q); ev(evs, Q, m[3], Q); ev(evs, 2*Q, m[3], Q); ev(evs, 3*Q, m[2], 2*Q)
    # b2: 5 5 3 3 2 1 -（地里黄呀；里/黄 两字各挂二音连音；末音 1 占 2 拍）
    t = 5*Q
    ev(evs, t, m[5], Q); ev(evs, t+Q, m[5], Q//2); ev(evs, t+Q+Q//2, m[3], Q//2)
    ev(evs, t+2*Q, m[3], Q//2); ev(evs, t+2*Q+Q//2, m[2], Q//2); ev(evs, t+3*Q, m[1], Q+Q)
    # b3: 1 3 2 6̣ -（三两岁上）
    t = 10*Q
    ev(evs, t, m[1], Q); ev(evs, t+Q, m[3], Q); ev(evs, t+2*Q, m[2], Q); ev(evs, t+3*Q, m['6_'], 2*Q)
    # b4: 2 1 6̣ 5̣ -（没了娘啊；娘 挂 6̣→5̣ 连音）
    t = 15*Q
    ev(evs, t, m[2], Q); ev(evs, t+Q, m[1], Q); ev(evs, t+2*Q, m['6_'], Q//2); ev(evs, t+2*Q+Q//2, m['5_'], Q*2+Q//2)
    return dict(notes=evs, bars=4, ts=(5,4), bpm=60, key='G')

# ---------- 4. pitch-small-leap 共产儿童团歌 1=F 4/4（教材 P19 小跳标注句，4 小节） ----------
def pitch_small_leap():
    m = jianpu(5)  # 1=F
    evs = []
    def bar12(t):  # 1 3̣. 1 2 5̣（准备好了么 / 时刻准备着）
        ev(evs, t, m[1], Q); ev(evs, t+Q, m[3], DE[0]); ev(evs, t+Q+DE[0], m[1], DE[1])
        ev(evs, t+2*Q, m[2], Q); ev(evs, t+3*Q, m['5_'], Q)
    def bar3(t):   # 2 3̣. 5 6 5（我 们 都 是[+尾音]）
        ev(evs, t, m[2], Q); ev(evs, t+Q, m[3], DE[0]); ev(evs, t+Q+DE[0], m[5], DE[1])
        ev(evs, t+2*Q, m[6], Q); ev(evs, t+3*Q, m[5], Q)
    bar12(0); bar12(4*Q); bar3(8*Q)
    # b4: 3̣. 5 2 3 5 -（共 产 儿 童 团；简谱「5 -」=二分音符）
    t = 12*Q
    ev(evs, t, m[3], DE[0]); ev(evs, t+DE[0], m[5], DE[1]); ev(evs, t+Q, m[2], Q//2); ev(evs, t+Q+Q//2, m[3], Q//2); ev(evs, t+2*Q, m[5], H)
    return dict(notes=evs, bars=4, ts=(4,4), bpm=100, key='F')

# ---------- 5. pitch-large-leap 东方之珠 1=D 4/4（教材 P20 大跳标注摘引，弱起+4 小节） ----------
def pitch_large_leap():
    m = jianpu(2)  # 1=D
    evs = []
    # 弱起: 0 1（让）
    ev(evs, Q//2, m[1], Q//2)
    # b2: 4̣. 6̣ i ⌒(7̣ 6̣ 5̣)（海 风 吹 拂[了]）——弱起小节仅 1 拍，b2 起于 tick Q
    t = Q
    ev(evs, t, m['4_'], Q+Q//2); ev(evs, t+Q+Q//2, m['6_'], Q//2); ev(evs, t+2*Q, m['i'], Q)
    for i, p in enumerate(['7_', '6_', '5_']): ev(evs, t+3*Q+i*T8, m[p], T8)
    # b3: 5̣. i 3 ⌒(1 3 6)（五 千 年，每）
    t = Q+W
    ev(evs, t, m['5_'], Q+Q//2); ev(evs, t+Q+Q//2, m['i'], Q//2); ev(evs, t+2*Q, m[3], Q)
    for i, p in enumerate([1, 3, 6]): ev(evs, t+3*Q+i*T8, m[p], T8)
    # b4: 5 5 ⌒(7̣ 2̣ 6) 5 5 ⌒(4̣ 2̣ 6)（一 滴 泪 珠 仿 佛）
    t = Q+2*W
    ev(evs, t, m[5], Q//2); ev(evs, t+Q//2, m[5], Q//2)
    for i, p in enumerate(['7_', '2_', 6]): ev(evs, t+Q+i*T8, m[p], T8)
    ev(evs, t+2*Q, m[5], Q//2); ev(evs, t+2*Q+Q//2, m[5], Q//2)
    for i, p in enumerate(['4_', '2_', 6]): ev(evs, t+3*Q+i*T8, m[p], T8)
    # b5: 5 - - -（严）
    ev(evs, Q+3*W, m[5], W)
    return dict(notes=evs, bars=5, ts=(4,4), bpm=72, key='D')

# ---------- SMF 写出（format 0, 1 track, 480ppq） ----------
def vlq(n):
    out = bytearray([n & 0x7F]); n >>= 7
    while n: out.insert(0, 0x80 | (n & 0x7F)); n >>= 7
    return bytes(out)

def write_smf(path, notes, ts, bpm):
    body = bytearray()
    import math
    body += vlq(0) + bytes([0xFF, 0x58, 0x04, ts[0], int(round(math.log2(ts[1]))), 24, 8])  # ts meta: dd=2^exp
    body += vlq(0) + bytes([0xFF, 0x51, 0x03]) + struct.pack('>I', round(60_000_000/bpm))[1:]
    flat = []
    for tick, pitch, dur in sorted(notes, key=lambda x: (x[0], x[1])):
        flat.append((tick, 'on', pitch)); flat.append((tick+dur, 'off', pitch))
    flat.sort(key=lambda x: (x[0], 0 if x[1] == 'off' else 1))
    cur = 0; chan_note = {}
    for tick, kind, pitch in flat:
        d = tick - cur; cur = tick
        if kind == 'on':
            body += vlq(d) + bytes([0x90, pitch, 80])
        else:
            body += vlq(d) + bytes([0x80, pitch, 0])
    body += vlq(0) + bytes([0xFF, 0x2F, 0x00])
    with open(path, 'wb') as f:
        f.write(b'MThd' + struct.pack('>IHHH', 6, 0, 1, PPQ))
        f.write(b'MTrk' + struct.pack('>I', len(body)) + bytes(body))
    return len(body) + 8 + 14

# ---------- SMF 回读核对 ----------
def read_smf(path):
    data = open(path, 'rb').read()
    assert data[:4] == b'MThd', 'header'
    fmt, ntrk, div = struct.unpack('>HHH', data[8:14])
    pos = 14
    assert data[pos:pos+4] == b'MTrk'
    tlen = struct.unpack('>I', data[pos+4:pos+8])[0]
    body = data[pos+8:pos+8+tlen]
    i = 0; cur = 0; notes = []; tempo = None; ts = None; open_notes = {}
    while i < len(body):
        d = 0
        while True:
            b = body[i]; i += 1; d = (d << 7) | (b & 0x7F)
            if not (b & 0x80): break
        cur += d
        st = body[i]
        if st == 0xFF:
            mt = body[i+1]; ln = body[i+2]; payload = body[i+3:i+3+ln]; i += 3+ln
            if mt == 0x51: tempo = struct.unpack('>I', b'\x00'+payload)[0]
            if mt == 0x58: ts = (payload[0], 2**payload[1])
        elif st in (0x90, 0x80):
            pitch, vel = body[i+1], body[i+2]; i += 3
            if st == 0x90 and vel > 0:
                open_notes.setdefault(pitch, []).append(cur)
            else:
                if open_notes.get(pitch):
                    notes.append((open_notes[pitch].pop(0), pitch, cur))
        else:
            i += 2
    return dict(format=fmt, ppq=div, tempo_us=tempo, ts=ts, notes=sorted(notes))

# ---------- 五条定义 ----------
ITEMS = [
    dict(id='pitch-repeat', fname='pitch-repeat_annotated-phrase_F_96bpm.mid', fn=pitch_repeat,
         page='P17(pdf-022)', title='我向党来唱支歌'),
    dict(id='pitch-step-up', fname='pitch-step-up_textbook-excerpt_C_96bpm.mid', fn=pitch_step_up,
         page='P18(pdf-023)', title='哆来咪'),
    dict(id='pitch-step-down', fname='pitch-step-down_first4_G_60bpm.mid', fn=pitch_step_down,
         page='P19(pdf-024)', title='小白菜'),
    dict(id='pitch-small-leap', fname='pitch-small-leap_first4_F_100bpm.mid', fn=pitch_small_leap,
         page='P19(pdf-024)', title='共产儿童团歌'),
    dict(id='pitch-large-leap', fname='pitch-large-leap_excerpt_D_72bpm.mid', fn=pitch_large_leap,
         page='P20(pdf-025)', title='东方之珠'),
]

UNCERTAIN = {
 'pitch-repeat': [
   'b3「2. 3」按附点八分+十六分转音读谱（教学标注弧线覆盖 2.3）；若实为连线二音均分则时值分布不同——候听核/对谱复核',
   'b4 末音 5̣ 低八度点清晰（C4）',
 ],
 'pitch-step-up': [
   'b5 首音 6̣ / b9 首音 7̣ 的低八度点在扫描分辨率下不完全可辨，按三段句平行结构（5̣→6̣→7̣ 递进低起）推断取低八度——候对谱复核',
   'b7/b8 的 7 按中八度（B4）读——若实为 7̣（B3）则 b7-b8 与 b9 关系不同',
   'b6 ♯4 与 b10 ♯4 ♯5 变音记号清晰（F#4/G#4）',
   'b3-4 / b7-8 / b11-12 的两小节 6/7/i 按连音（每小节二分音符各一）处理',
 ],
 'pitch-step-down': [
   'b2/b4 小音符群上的弧线在扫描分辨率下与低八度点难区分；按下行级进教学标注取连音读法（b2=5 5 3 3 2 1 同八度下行；b4 娘=6̣→5̣ 连音）——若实为低八度点则音高整体不同，候对谱复核',
   'b3 末音 6̣ 低八度点清晰（E4）',
   'b4「娘」装饰群按 [6̣,5̣] 两音读（三音 [1̣,6̣,5̣] 读法与下行级进逻辑不符，已排除）',
 ],
 'pitch-small-leap': [
   'b1/b2 末音 5̣ 低八度点（C4）较清晰保留；b3/b4 的 3/5/6 低八度点不可完全排除，按小跳（三度）与「号角之音」（大三度）教学注释取同八度读法——候对谱复核',
   'b3 第五音（5̣）无对应单字（「是」字尾腔或连接音），按谱面音符如实转写',
   'b1/b2 附点节奏 3̣.1（附点八分+十六分）按谱面附点符号读',
 ],
 'pitch-large-leap': [
   '本条扫描分辨率下小节线/八度点/三连音标记整体可信度最低，全部音高按图面八度点直读；i=D5（高八度）依据图面字形——候对谱复核（建议以更高分辨率图像重核本条）',
   'b2/b3 三连音组（7̣6̣5̣ / 136）按图面「3」标记读；b4 两组三连音（7̣2̣6 / 4̣2̣6）音位含 2̣（E3），与前后音衔接是否如实候复核',
   '弱起小节 0 1（让）按八分休止+八分读',
   '歌词对位未逐字核（转写 MIDI 为教学概念演示用途，歌词不入 MIDI）',
 ],
}

def main():
    report = dict(schema='vit.material_transcribe_batch.v0', batch=1, ppq=PPQ,
                  scope='教材印刷教学摘录（P17-P20 五例），不超出图面所示小节',
                  status_rule='全部 transcribed_unverified（人耳/对谱听核未做，绝不写 checked）',
                  items=[])
    os.makedirs(ASSETS_MIDI, exist_ok=True)
    for it in ITEMS:
        spec = it['fn']()
        path = os.path.join(ASSETS_MIDI, it['fname'])
        size = write_smf(path, spec['notes'], spec['ts'], spec['bpm'])
        rb = read_smf(path)
        # 核对：音符数/音高序列/时值序列
        exp_pitches = [p for _, p, _ in sorted(spec['notes'], key=lambda x: (x[0], x[1]))]
        got_pitches = [p for _, p, _ in rb['notes']]
        exp_durs = [d for _, _, d in sorted(spec['notes'], key=lambda x: (x[0], x[1]))]
        got_durs = [c - s for s, p, c in rb['notes']]
        ok_count = len(rb['notes']) == len(spec['notes'])
        ok_pitch = exp_pitches == got_pitches
        ok_dur = exp_durs == got_durs
        bpm_rb = round(60_000_000 / rb['tempo_us'], 1) if rb['tempo_us'] else None
        sha = hashlib.sha256(open(path, 'rb').read()).hexdigest()
        ok = ok_count and ok_pitch and ok_dur and rb['ppq'] == PPQ and bpm_rb == spec['bpm'] and rb['ts'] == spec['ts']
        rec = dict(id=it['id'], title=it['title'], page=it['page'], file='assets/midi/'+it['fname'],
                   bytes=size, sha256=sha, format=rb['format'], ppq=rb['ppq'], ts=f"{rb['ts'][0]}/{rb['ts'][1]}",
                   demo_bpm=spec['bpm'], bpm_readback=bpm_rb,
                   bars=spec['bars'], note_count_expected=len(spec['notes']), note_count_readback=len(rb['notes']),
                   pitches=got_pitches, durations_ticks=got_durs,
                   check_count=ok_count, check_pitch=ok_pitch, check_duration=ok_dur,
                   check_meta=(rb['ppq'] == PPQ and bpm_rb == spec['bpm'] and rb['ts'] == spec['ts']),
                   result='PASS' if ok else 'FAIL', uncertain=UNCERTAIN[it['id']])
        report['items'].append(rec)
        print(f"[{it['id']}] {rec['result']} notes={len(rb['notes'])}/{len(spec['notes'])} bpm={bpm_rb} ts={rec['ts']} sha={sha[:12]}")
    with open(os.path.join(RUN_DIR, 'verification_batch1.json'), 'w', encoding='utf-8') as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    all_ok = all(r['result'] == 'PASS' for r in report['items'])
    print('BATCH1:', 'ALL PASS' if all_ok else 'HAS FAIL')
    return 0 if all_ok else 1

if __name__ == '__main__':
    sys.exit(main())
