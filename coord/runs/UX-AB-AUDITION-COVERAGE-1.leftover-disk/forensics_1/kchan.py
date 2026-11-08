import re, sys
path = sys.argv[1]
lines = open(path, encoding='utf-8', errors='replace').read().splitlines()
marker = 'processing queued command on JUCE message thread: '
for i, l in enumerate(lines):
    idx = l.find(marker)
    if idx < 0:
        continue
    body = l[idx + len(marker):]
    cat = re.search(r'"cmd"\s*:\s*"([a-zA-Z_0-9.]+)"', body)
    ch = re.search(r'"channel"\s*:\s*"([a-zA-Z_.0-9]+)"', body)
    cl = re.search(r'"client_id"\s*:\s*"([a-zA-Z_.0-9]+)"', body)
    at = re.search(r'"created_at"\s*:\s*"([^"]+)"', body)
    op = re.search(r'"op"\s*:\s*"([a-zA-Z_.0-9]+)"', body)
    if cat or (ch and at):
        t = at.group(1) if at else ''
        who = cl.group(1) if cl else ''
        if '2026-10-22T16:2' in t or '2026-10-22T16:1' in t or not t:
            name = cat.group(1) if cat else (ch.group(1) + ':' + (op.group(1) if op else ''))
            print(f'{i}|{t}|{name}|{who}|{body[:170]}')
