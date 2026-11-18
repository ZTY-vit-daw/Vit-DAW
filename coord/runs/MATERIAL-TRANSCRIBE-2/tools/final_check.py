# -*- coding: utf-8 -*-
import sys, json, os, hashlib
root = sys.argv[1]; run = sys.argv[2]
total = 0; bad = []
for b in (2,3,4):
    v = json.load(open(os.path.join(run, 'batch%d' % b, 'verification_batch%d.json' % b), encoding='utf-8'))
    for it in v['items']:
        total += 1
        p = os.path.join(root, it['file'].replace('assets/', ''))
        sha = hashlib.sha256(open(p,'rb').read()).hexdigest()
        if sha != it['sha256']: bad.append((b, it['id'], 'sha'))
        if it['result'] != 'PASS': bad.append((b, it['id'], 'result'))
print('items checked:', total, 'problems:', bad if bad else 'NONE')
