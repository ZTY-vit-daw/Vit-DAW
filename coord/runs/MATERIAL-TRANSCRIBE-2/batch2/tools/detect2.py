# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load(); w,h=im.size
print('size',w,h)
rows=[]
for y in range(h):
    c=0
    for x in range(0,w,4):
        p=px[x,y]
        if abs(p[0]-246)<=4 and abs(p[1]-237)<=4 and abs(p[2]-228)<=4: c+=1
    if c>100: rows.append(y)
gs=[]
for y in rows:
    if gs and y-gs[-1][1]<=2: gs[-1][1]=y
    else: gs.append([y,y])
print('bands',gs)
