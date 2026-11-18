# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
def dark(x,y): return sum(px[x,y])<450
for label,y0,y1,minc in [('feb',670,708,26),('cheng',1340,1378,26)]:
    cols=[]
    for x in range(150,1120):
        c=sum(1 for y in range(y0,y1) if dark(x,y))
        if c>=minc: cols.append(x)
    gs=[]
    for x in cols:
        if gs and x-gs[-1][1]<=2: gs[-1][1]=x
        else: gs.append([x,x])
    print(label,[round((a+b)/2) for a,b in gs])
