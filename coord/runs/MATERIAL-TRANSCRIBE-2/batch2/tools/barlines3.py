# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
def dark(x,y): return sum(px[x,y])<450
for label,y0,y1 in [('feb',677,701),('cheng',1391,1415)]:
    cols=[]
    for x in range(150,1120):
        c=sum(1 for y in range(y0,y1) if dark(x,y))
        if c>=15: cols.append(x)
    gs=[]
    for x in cols:
        if gs and x-gs[-1][1]<=2: gs[-1][1]=x
        else: gs.append([x,x])
    print(label,[(round((a+b)/2),b-a+1) for a,b in gs])
