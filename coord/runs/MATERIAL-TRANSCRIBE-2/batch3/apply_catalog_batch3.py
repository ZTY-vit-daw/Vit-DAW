# -*- coding: utf-8 -*-
"""Apply batch3 catalog updates (5 items)."""
import sys, io
CAT, AFTER = sys.argv[1], sys.argv[2]
UPDATES = {
 'contour-flat': dict(
   excerpt='P22 平直线例（1=♭B 2/4，图面 8 小节：3. 3 3 0｜3. 3 3 0｜3 3 3 3｜♯2 3 0｜6. 6 6｜6. 6 6｜6 6 6 6｜♯5 6 0；0 为休止）',
   file='assets/midi/contour-flat_chengsheng-first8_Bb_120bpm.mid'),
 'contour-smooth': dict(
   excerpt='P25 平缓进行例（1=C 3/4，弱起 2 个十六分 + 图面 4 小节：2̇2̇｜2̇. 2̇2̇2̇｜♯1̇. 3̇ 3̇. 6 7｜i̇. i̇i̇i̇｜7. 2̇ 2̇(延长) 0）',
   file='assets/midi/contour-smooth_morning-first4_C_72bpm.mid'),
 'contour-wave': dict(
   excerpt='P26 跌宕起伏例（1=♭B 4/4，弱起 0 3 + 图面第一行 4 小节：3̇ 3. 3̇ 2 2̇ 0 3｜2̇ 3. 3̇ i̇ i̇ 0 i̇｜7 2̇. i̇ 7 6 5 6｜3 - - 0 3；含八度/七度/六度大跳标注）',
   file='assets/midi/contour-wave_seaport-first4_Bb_60bpm.mid'),
 'analysis-huainian': dict(
   excerpt='P27 创作示例《怀念战友》首句（1=♭B 2/4，图面 4 小节：0 22 5 5｜5. 6 7 i̇ 7｜6 6 i̇ 7 6｜5 -；歌词“天山脚下是我可爱的家乡”；上方小数字/分数标记未入 MIDI）',
   file='assets/midi/analysis-huainian_first4_Bb_72bpm.mid'),
 'analysis-xiaocao': dict(
   excerpt='P28 《小草》首句（1=G 2/4，图面第一行 8 小节：6̣6̣ 1 7̣｜6̣. 0｜6̣6̣ 3 2｜3. 0｜3 3 5 3｜2 2 1 7̣7̣｜6̣ 0 5｜3̣. 0；歌词“没有花香，没有树高，我是一棵无人知道的小草”）',
   file='assets/midi/analysis-xiaocao_first8_G_80bpm.mid'),
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
