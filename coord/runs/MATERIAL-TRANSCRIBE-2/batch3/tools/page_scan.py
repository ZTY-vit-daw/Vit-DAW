# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load(); w,h=im.size
print('size',w,h)
prof=[sum(1 for x in range(120,w-40) if sum(px[x,y])<430) for y in range(h)]
gs=[]
for y,c in enumerate(prof):
    if c>35:
        if gs and y-gs[-1][1]<=3: gs[-1][1]=y
        else: gs.append([y,y])
print('dense y bands:', gs)
