# -*- coding: utf-8 -*-
"""Deterministic symbol analysis of the rhythm X-notation rows on book p.21.
Outputs, per bar: glyph components (X note / dash / dot), underline levels with x ranges.
"""
import sys, json
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load(); W,H=im.size
def dark(x,y): return sum(px[x,y])<420

def components(x0,x1,y0,y1):
    # connected components (8-neighborhood) in box
    seen=set(); comps=[]
    for yy in range(y0,y1):
        for xx in range(x0,x1):
            if (xx,yy) in seen or not dark(xx,yy): continue
            stack=[(xx,yy)]; seen.add((xx,yy)); cells=[]
            while stack:
                cx,cy=stack.pop(); cells.append((cx,cy))
                for dx in (-1,0,1):
                    for dy in (-1,0,1):
                        nx,ny=cx+dx,cy+dy
                        if x0<=nx<x1 and y0<=ny<y1 and (nx,ny) not in seen and dark(nx,ny):
                            seen.add((nx,ny)); stack.append((nx,ny))
            xs=[c[0] for c in cells]; ys=[c[1] for c in cells]
            comps.append(dict(x0=min(xs),x1=max(xs),y0=min(ys),y1=max(ys),n=len(cells),cx=sum(xs)/len(cells),cy=sum(ys)/len(ys)))
    return comps

rows = {
 'row1': dict(bars=[209,372,492,617,791,1000], band=(390,470)),
 'row2': dict(bars=[209,401,582,764,1000], band=(543,631)),
 'row3': dict(bars=[209,408,579,752,1000], band=(710,798)),
 'row4': dict(bars=[209,383,535,752,1000], band=(875,963)),
}
out={}
for rn,spec in rows.items():
    bs=spec['bars']; y0,y1=spec['band']
    # global vertical profile to find glyph band (rows with many dark px)
    prof=[(y,sum(1 for x in range(bs[0],bs[-1]) if dark(x,y))) for y in range(y0,y1)]
    # glyph rows: count>5 but underlines are also long rows; use height: lines are 2px tall
    # find underline rows: runs of consecutive rows with >60 dark; glyph band: 14-40 dark
    tall=[y for y,c in prof if c>60]
    mid=[y for y,c in prof if 5<c<=60]
    g0=min(mid); g1=max(mid)
    out[rn]=dict(glyph_band=(g0,g1), underline_rows=tall, bars=[])
    for i in range(len(bs)-1):
        bx0,bx1=bs[i]+2,bs[i+1]-1
        comps=components(bx0,bx1,g0-2,max(tall)+4 if tall else g1+8)
        out[rn]['bars'].append(dict(idx=i+1, x=(bx0,bx1), comps=comps))
json.dump(out, open(sys.argv[2],'w',encoding='utf-8'), ensure_ascii=False, indent=1)
# compact print
for rn in out:
    print('==',rn,'glyph band',out[rn]['glyph_band'],'underline rows',out[rn]['underline_rows'])
    for b in out[rn]['bars']:
        glyphs=[c for c in b['comps'] if c['y1']-c['y0']>=8]
        others=[c for c in b['comps'] if c['y1']-c['y0']<8]
        print(' bar%d x=%s'%(b['idx'],b['x']))
        print('   glyphs:', [(round(c['cx']), c['x0'],c['x1'], 'h=%d'%(c['y1']-c['y0']+1)) for c in glyphs])
        print('   small :', [(round(c['cx']), c['x0'],c['x1'],c['y0'],c['y1']) for c in others])
