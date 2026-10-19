import sys, re, json, struct
run, rawpath, wavpath, tag = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
body = open(rawpath,'rb').read()
m = re.search(rb'boundary=([^\r\n;]+)', open(run + '/' + tag + '_headers.txt','rb').read())
bnd = m.group(1)
delim = b'--' + bnd
for c in body.split(delim):
    if b'Content-Type' not in c: continue
    header, _, payload = c.partition(b'\r\n\r\n')
    ct = re.search(rb'Content-Type:\s*([^\r\n]+)', header).group(1).decode()
    payload = payload.rstrip(b'\r\n')
    if 'json' in ct:
        j = json.loads(payload)
        open(run + '/' + tag + '_replay.json','w',encoding='utf-8').write(json.dumps(j, ensure_ascii=False, indent=2))
        print(tag, 'replay: seed=', j.get('seed'), 'duration=', j.get('duration'))
    elif 'audio' in ct:
        open(wavpath,'wb').write(payload)
        print(tag, 'audio bytes:', len(payload))
