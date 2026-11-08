import re, sys
path = sys.argv[1]
start = int(sys.argv[2]) if len(sys.argv) > 2 else 0
lines = open(path, encoding='utf-8', errors='replace').read().splitlines()
marker = 'processing queued command on JUCE message thread: '
cmdpat = re.compile(r'"cmd"\s*:\s*"([a-z_0-9.]+)"')
chpat = re.compile(r'"channel"\s*:\s*"([a-z_.]+)"')
clid = re.compile(r'"client_id"\s*:\s*"([a-z_.0-9]+)"')
scope = re.compile(r'"scope"\s*:\s*"([a-z_.0-9*]+)"')
op = re.compile(r'"op"\s*:\s*"([a-z_.0-9]+)"')
req = re.compile(r'"request_id"\s*:\s*"([a-zA-Z_0-9]+)"')
for i, l in enumerate(lines[start:], start):
    idx = l.find(marker)
    if idx < 0:
        continue
    body = l[idx + len(marker):]
    c = cmdpat.search(body); ch = chpat.search(body); cl = clid.search(body)
    sc = scope.search(body); o = op.search(body); rq = req.search(body)
    head = c.group(1) if c else ((ch.group(1) if ch else '?') + '/' + (o.group(1) if o else (sc.group(1) if sc else '?')))
    print(f'{i}|{head}|{cl.group(1) if cl else ""}|{rq.group(1) if rq else ""}|{body[:200]}')
