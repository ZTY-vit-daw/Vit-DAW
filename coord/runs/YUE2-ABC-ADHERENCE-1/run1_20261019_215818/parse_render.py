import sys, re, json

rawf, wavf, repf = sys.argv[1], sys.argv[2], sys.argv[3]
body = open(rawf, 'rb').read()
m = re.search(rb'--([A-Za-z0-9._-]+)\r\n', body)
if not m:
    print('NO_BOUNDARY', rawf); sys.exit(1)
bnd = m.group(1)
for c in body.split(b'--' + bnd):
    if b'Content-Type' not in c:
        continue
    header, _, payload = c.partition(b'\r\n\r\n')
    ct = re.search(rb'Content-Type:\s*([^\r\n]+)', header).group(1).decode()
    payload = payload.rstrip(b'\r\n')
    if payload.endswith(b'--'):
        payload = payload[:-2].rstrip(b'\r\n')
    if 'json' in ct:
        open(repf, 'w', encoding='utf-8').write(
            json.dumps(json.loads(payload), ensure_ascii=False, indent=1))
    elif 'audio' in ct:
        open(wavf, 'wb').write(payload)
