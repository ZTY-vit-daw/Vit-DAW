# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
def dark(x,y): return sum(px[x,y])<420
for label,y0,y1 in [('feb',676,700),('cheng',1390,1414)]:
    cols=[]
    for x in range(150,1120):
        c=sum(1 for y in range(y0,y1) if dark(x,y))
        if c>=16: cols.append((x,c))
    gs=[]
    for x,c in cols:
        if gs and x-gs[-1][1]<=2: gs[-1][1]=x
        else: gs.append([x,x])
    print(label,'barlines:',[round((a+b)/2) for a,b in gs])
