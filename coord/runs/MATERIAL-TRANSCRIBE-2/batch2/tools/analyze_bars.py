# -*- coding: utf-8 -*-
"""Pixel-level rhythm-symbol analysis: glyph x-centers, underline segments, dot blobs.
For jianpu rhythm X-notation: X glyphs = notes; horizontal lines beneath = beam count (1=eighth,2=16th);
'-' glyphs = beat extension; '.' dot = dotted note; ties/slurs = arcs (not detected here).
"""
import sys
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()

def dark(x,y): return sum(px[x,y])<420

def analyze(x0,x1,y0,y1,label):
    # vertical profile to locate glyph band and underline band
    prof=[sum(1 for x in range(x0,x1) if dark(x,y)) for y in range(y0,y1)]
    print(label,'yprof:',[(y0+i,c) for i,c in enumerate(prof) if c>0])
    # horizontal line segments (runs>=12) for each y
    for y in range(y0,y1):
        segs=[]; s=None
        for x in range(x0,x1+1):
            d = dark(x,y) if x<x1 else False
            if d and s is None: s=x
            elif not d and s is not None:
                if x-s>=12: segs.append((s,x-1))
                s=None
        if segs: print('  line y=%d'%y, segs)

for label,box in [('row2_bar4',(764,1000,543,631)),('row3_bar2',(408,579,710,798)),('row3_bar3',(579,752,710,798)),('row4_bar2',(383,535,875,963))]:
    analyze(box[0],box[1],box[2],box[3],label)
