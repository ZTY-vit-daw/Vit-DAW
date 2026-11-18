# -*- coding: utf-8 -*-
"""Apply batch4 catalog updates (5 items)."""
import sys, io
CAT, AFTER = sys.argv[1], sys.argv[2]
UPDATES = {
 'develop-ornament': dict(
   excerpt='P29 编创实践·即兴一 加花（1=C 4/4，图面 2 小节：1 2 3 1 2 5｜3 2 3 6 5 -；与原型 1 3 2 5｜3 6 5 - 配对）',
   file='assets/midi/develop-ornament_jiaxing1_C_80bpm.mid'),
 'develop-rhythm': dict(
   excerpt='P29 编创实践·即兴二 改变节奏（1=C 4/4，图面 2 小节：1 3 1 2 5.｜3 6 3 5 -；与原型配对）',
   file='assets/midi/develop-rhythm_change1_C_80bpm.mid'),
 'develop-augment-meter': dict(
   excerpt='P29 编创实践·即兴三 节奏放大并改变节拍（1=C 3/4，图面 7 小节：1 - -｜3 - 1｜2 - -｜5 - -｜3 - -｜6 - 3｜5 - -）',
   file='assets/midi/develop-augment-meter_3-4_C_80bpm.mid'),
 'develop-shift-rhythm': dict(
   excerpt='P29 编创实践·即兴四 旋律移位并作节奏变化（1=C 4/4，图面 4 小节：1 3 2 5｜3 6 5 -｜4 6 4 5 i̇｜6 2̇. i̇ -）',
   file='assets/midi/develop-shift-rhythm_yiwei1_C_80bpm.mid'),
 'reference-sanshilipu-melody': dict(
   excerpt='P30 三十里铺（陕西民歌）前两句 8 小节（1=C 2/4：1̇2̇2̇｜5̇1̇6｜5.6 5 2｜5 - 重复两次；歌词“提起个家来家有名，家住绥德三十里铺村”）',
   file='assets/midi/reference-sanshilipu-melody_first8_C_80bpm.mid'),
}
src = io.open(CAT, encoding='utf-8').read()
lines = src.split('\n')
cur = None; stats = {}; out = []
for ln in lines:
    st = ln.strip()
    if st.startswith('- id: '):
        cur = st[len('- id: '):].strip(); stats.setdefault(cur, 0)
    elif cur in UPDATES:
        u = UPDATES[cur]
        if st.startswith('excerpt: '): ln = '    excerpt: ' + u['excerpt']; stats[cur] += 1
        elif st.startswith('file: '): ln = '    file: ' + u['file']; stats[cur] += 1
        elif st.startswith('status: '):
            assert st == 'status: planned_transcription', (cur, st)
            ln = '    status: transcribed_unverified'; stats[cur] += 1
    out.append(ln)
for k in UPDATES: assert stats.get(k) == 3, (k, stats.get(k))
new = '\n'.join(out)
io.open(CAT, 'w', encoding='utf-8', newline='\n').write(new)
io.open(AFTER, 'w', encoding='utf-8', newline='\n').write(new)
print('updated:', list(UPDATES))
print('planned remaining:', new.count('status: planned_transcription'))
print('transcribed total:', new.count('status: transcribed_unverified'))
print('checked total:', new.count('status: generated_symbolic_checked'))
