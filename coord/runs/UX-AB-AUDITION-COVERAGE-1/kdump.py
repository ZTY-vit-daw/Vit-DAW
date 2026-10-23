import sys
path = sys.argv[1]
lines = open(path, encoding='utf-8', errors='replace').read().splitlines()
marker = 'processing queued command on JUCE message thread: '
want = {'292393','297383','380348','400454','400839','401046','401239','401265','401406','401582','401945','402087','260452'}
for i, l in enumerate(lines):
    if str(i) not in want:
        continue
    idx = l.find(marker)
    body = l[idx + len(marker):] if idx >= 0 else l
    print(f'--- line {i} ---')
    print(body[:600])
