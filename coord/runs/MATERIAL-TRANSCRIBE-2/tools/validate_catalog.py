# -*- coding: utf-8 -*-
import sys, io, os, re
cat = sys.argv[1]; assets_root = sys.argv[2]
text = io.open(cat, encoding='utf-8').read()
lines = text.split('\n')
items = []
cur = None; rec = {}
for ln in lines:
    st = ln.strip()
    if st.startswith('- id: '):
        if cur: items.append(rec)
        cur = st[len('- id: '):].strip(); rec = dict(id=cur)
    elif cur:
        for key in ('status:', 'file:', 'excerpt:'):
            if st.startswith(key):
                rec[key[:-1]] = st[len(key):].strip()
if cur: items.append(rec)
print('total items:', len(items))
from collections import Counter
print('status counts:', dict(Counter(r.get('status') for r in items)))
missing = []
for r in items:
    f = r.get('file', 'null')
    if f and f != 'null':
        p = os.path.join(assets_root, f.replace('assets/', ''))
        if not os.path.exists(p): missing.append((r['id'], f))
print('missing files:', missing if missing else 'NONE')
planned = [r['id'] for r in items if r.get('status') == 'planned_transcription']
print('planned remaining ids:', planned)
# check midi count
midi_dir = os.path.join(assets_root, 'midi')
print('midi files:', len(os.listdir(midi_dir)))
