# -*- coding: utf-8 -*-
"""Apply batch2 catalog updates (5 items): excerpt/file/status."""
import sys, io
CAT = sys.argv[1]      # catalog path
AFTER = sys.argv[2]    # catalog_after snapshot path
UPDATES = {
 'rhythm-even': dict(
   excerpt='P21 均分节奏例（4/4，第 1-4 小节：全音符 X - - -｜两二分 X - X -｜四四分 X X X X｜八八分 X X X X X X X X；第 5 小节三连音例未选入首包；X 谱无音高）',
   file='assets/midi/rhythm-even_uniform-4bars_GM37_80bpm.mid'),
 'rhythm-long-short': dict(
   excerpt='P21 前长后短节奏例（4/4，图面全 4 小节：四分+二分+两八分｜二分+四八分｜附点四分+八分+附点四分+两十六分｜八分+附点八分与十六分组合；X 谱无音高）',
   file='assets/midi/rhythm-long-short_4bars_GM37_80bpm.mid'),
 'rhythm-short-long': dict(
   excerpt='P21 前短后长节奏例（4/4，图面全 4 小节：两八分+四分+二分延长｜八分+附点四分+两十六分+附点四分｜两十六分+附点四分+八分+附点四分｜四十六分+四分+十六分附点组合；X 谱无音高）',
   file='assets/midi/rhythm-short-long_4bars_GM37_80bpm.mid'),
 'rhythm-syncopation': dict(
   excerpt='P21 切分节奏例（4/4，图面全 4 小节：四分+第2拍起二分+四分｜八分起+三连续四分+八分｜八分与十六分切分组合｜十六分密集切分；跨小节弧线未合并时值；X 谱无音高）',
   file='assets/midi/rhythm-syncopation_4bars_GM37_80bpm.mid'),
 'contour-free': dict(
   excerpt='P22 自由曲线例（1=♭A 4/4，图面 4 小节：2 1 2 3 2. 3｜6̣ 1 2 3 5̣ -｜2. 3 5 3 2 3 1 2 6̣｜3̣ 5̣ 6̣ 1 5̣ -；歌词“二月里来呀好春光，家家户户种田忙”；装饰音见 verification）',
   file='assets/midi/contour-free_february-first4_Ab_72bpm.mid'),
}
src = io.open(CAT, encoding='utf-8').read()
lines = src.split('\n')
cur = None
stats = {}
out = []
for ln in lines:
    st = ln.strip()
    if st.startswith('- id: '):
        cur = st[len('- id: '):].strip()
        stats.setdefault(cur, 0)
    elif cur in UPDATES:
        u = UPDATES[cur]
        if st.startswith('excerpt: '):
            ln = '    excerpt: ' + u['excerpt']; stats[cur] += 1
        elif st.startswith('file: '):
            ln = '    file: ' + u['file']; stats[cur] += 1
        elif st.startswith('status: '):
            assert st == 'status: planned_transcription', (cur, st)
            ln = '    status: transcribed_unverified'; stats[cur] += 1
    out.append(ln)
for k, v in UPDATES.items():
    assert stats.get(k) == 3, (k, stats.get(k))
new = '\n'.join(out)
io.open(CAT, 'w', encoding='utf-8', newline='\n').write(new)
io.open(AFTER, 'w', encoding='utf-8', newline='\n').write(new)
print('updated items:', list(UPDATES))
print('planned remaining:', new.count('status: planned_transcription'))
print('transcribed total:', new.count('status: transcribed_unverified'))
print('checked total:', new.count('status: generated_symbolic_checked'))
