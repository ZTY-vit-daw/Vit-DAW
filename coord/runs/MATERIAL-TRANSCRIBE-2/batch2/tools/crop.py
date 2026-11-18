# -*- coding: utf-8 -*-
"""Crop/upscale source page regions for careful jianpu reading (MATERIAL-TRANSCRIBE-2).
Usage: python crop.py <page.png> <outdir> <spec.json>
spec: [{"name": "...", "box": [x0,y0,x1,y1], "scale": 3}]
"""
import json, sys, os
from PIL import Image

def main():
    page, outdir, specpath = sys.argv[1], sys.argv[2], sys.argv[3]
    os.makedirs(outdir, exist_ok=True)
    spec = json.load(open(specpath, encoding='utf-8'))
    im = Image.open(page).convert('RGB')
    print('page', page, im.size)
    for s in spec:
        x0, y0, x1, y1 = s['box']
        c = im.crop((x0, y0, x1, y1))
        sc = s.get('scale', 3)
        c = c.resize((int(c.width*sc), int(c.height*sc)), Image.LANCZOS)
        out = os.path.join(outdir, s['name'] + '.png')
        c.save(out)
        print(out, c.size)

if __name__ == '__main__':
    main()
