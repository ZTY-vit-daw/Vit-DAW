# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
x0,x1,y0,y1=map(int,sys.argv[2:6])
for y in range(y0,y1):
    segs=[]; s=None
    for x in range(x0,x1+1):
        d = (sum(px[x,y])<430) if x<x1 else False
        if d and s is None: s=x
        elif not d and s is not None:
            if x-s>=5: segs.append((s,x-1))
            s=None
    if segs: print('y=%d'%y, segs)
