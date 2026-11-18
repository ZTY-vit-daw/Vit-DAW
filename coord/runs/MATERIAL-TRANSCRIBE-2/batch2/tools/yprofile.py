# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load(); w,h=im.size
prof=[sum(1 for x in range(150,1120) if sum(px[x,y])<420) for y in range(h)]
# print contiguous y ranges with count>40
gs=[]
for y,c in enumerate(prof):
    if c>40:
        if gs and y-gs[-1][1]<=3: gs[-1][1]=y
        else: gs.append([y,y])
print('dense y bands:', gs)
