# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
def dark(x,y,th): return sum(px[x,y])<th
x0,x1,y0,y1,th=map(int,sys.argv[2:7])
for y in range(y0,y1):
    segs=[]; s=None
    for x in range(x0,x1+1):
        d = dark(x,y,th) if x<x1 else False
        if d and s is None: s=x
        elif not d and s is not None:
            if x-s>=4: segs.append((s,x-1))
            s=None
    if segs: print('y=%d'%y, segs)
