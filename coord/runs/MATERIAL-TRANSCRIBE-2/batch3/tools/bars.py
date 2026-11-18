# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load(); w,h=im.size
def dark(x,y): return sum(px[x,y])<430
# args: y0 y1 minh xa xb
y0,y1,minh = int(sys.argv[2]), int(sys.argv[3]), int(sys.argv[4])
xa,xb = int(sys.argv[5]), int(sys.argv[6])
cols=[]
for x in range(xa,xb):
    c=sum(1 for y in range(y0,y1) if dark(x,y))
    if c>=minh: cols.append(x)
gs=[]
for x in cols:
    if gs and x-gs[-1][1]<=2: gs[-1][1]=x
    else: gs.append([x,x])
print('barlines:', [round((a+b)/2) for a,b in gs])
# also print row profile for fine band check
prof=[(y,sum(1 for x in range(xa,xb) if dark(x,y))) for y in range(y0,y1)]
print('yprof:', [(y,c) for y,c in prof if c>0])
