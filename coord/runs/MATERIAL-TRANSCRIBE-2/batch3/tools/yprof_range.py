# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
x0,x1,y0,y1=map(int,sys.argv[2:6])
for y in range(y0,y1):
    c=sum(1 for x in range(x0,x1) if sum(px[x,y])<430)
    if c>0: print(y, c)
