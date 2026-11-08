import sys
path = sys.argv[1]
lines = open(path, encoding='utf-8', errors='replace').read().splitlines()
marker = 'processing queued command on JUCE message thread: '
want = set(sys.argv[2].split(','))
for i, l in enumerate(lines):
    if str(i) not in want:
        continue
    idx = l.find(marker)
    body = l[idx + len(marker):] if idx >= 0 else l
    print(f'--- line {i} ---')
    print(body[:450])
