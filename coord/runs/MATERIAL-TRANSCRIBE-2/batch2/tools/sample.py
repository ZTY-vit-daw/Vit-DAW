# -*- coding: utf-8 -*-
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); w,h=im.size; px=im.load()
for y in [300,350,400,420,440,460,480,520,560,600,640,680,720,760,800,840,880,920,960,1000]:
    print(y, [px[x,y] for x in (200,300,400,500,600,700,800,900)])
