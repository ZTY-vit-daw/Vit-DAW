# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
bands=[(392,468),(549,625),(716,792),(881,957)]
for y0,y1 in bands:
    cols=[]
    for x in range(205,1040):
        dark=sum(1 for y in range(y0+2,y1-2) if sum(px[x,y])<400)
        if dark > int((y1-y0-4)*0.55): cols.append(x)
    # group contiguous cols
    gs=[]
    for x in cols:
        if gs and x-gs[-1][1]<=2: gs[-1][1]=x
        else: gs.append([x,x])
    print(y0,y1,'barlines:', [round((a+b)/2) for a,b in gs])
