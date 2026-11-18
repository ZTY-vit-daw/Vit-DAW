# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); w,h=im.size; px=im.load()
def is_beige(y):
    c=0
    for x in range(210,1040,4):
        p=px[x,y]
        if abs(p[0]-246)<=4 and abs(p[1]-237)<=4 and abs(p[2]-228)<=4: c+=1
    return c
rows=[y for y in range(h) if is_beige(y)>120]
groups=[]
for y in rows:
    if groups and y-groups[-1][1]<=2: groups[-1][1]=y
    else: groups.append([y,y])
print('bands:', groups)
# x extent per band
for g0,g1 in groups:
    y=(g0+g1)//2
    xs=[x for x in range(w) if abs(px[x,y][0]-246)<=4 and abs(px[x,y][1]-237)<=4 and abs(px[x,y][2]-228)<=4]
    print('band', g0, g1, 'x', min(xs), max(xs))
