# -*- coding: utf-8 -*-
"""Generic jianpu line analyzer (components -> digits/underline/dot/dash/grace/arc).
usage: gen_melody.py <page.png> <out.json> <label> <xstart> <xend> <ytop> <ybot> <barlines comma>
"""
import sys, json
from PIL import Image
page, outfile, label = sys.argv[1], sys.argv[2], sys.argv[3]
xstart, xend = int(sys.argv[4]), int(sys.argv[5])
ytop, ybot = int(sys.argv[6]), int(sys.argv[7])
barlines = [int(v) for v in sys.argv[8].split(',') if v.strip()]
im = Image.open(page).convert('RGB'); px=im.load()
def dark(x,y): return sum(px[x,y])<430
def comps(x0,x1,y0,y1):
    seen=set(); out=[]
    for yy in range(y0,y1):
        for xx in range(x0,x1):
            if (xx,yy) in seen or not dark(xx,yy): continue
            st=[(xx,yy)]; seen.add((xx,yy)); cells=[]
            while st:
                cx,cy=st.pop(); cells.append((cx,cy))
                for dx in (-1,0,1):
                    for dy in (-1,0,1):
                        nx,ny=cx+dx,cy+dy
                        if x0<=nx<x1 and y0<=ny<y1 and (nx,ny) not in seen and dark(nx,ny):
                            seen.add((nx,ny)); st.append((nx,ny))
            xs=[c[0] for c in cells]; ys=[c[1] for c in cells]
            out.append(dict(x0=min(xs),x1=max(xs),y0=min(ys),y1=max(ys),n=len(cells)))
    return out
rows=[]; 
bounds=[xstart]+barlines
for i in range(len(bounds)-1):
    bx0,bx1 = bounds[i]+1, bounds[i+1]-1
    items=[]
    for c in comps(bx0,bx1,ytop,ybot):
        h=c['y1']-c['y0']+1; w=c['x1']-c['x0']+1
        items.append(dict(x0=c['x0'],x1=c['x1'],y0=c['y0'],y1=c['y1'],n=c['n'],w=w,h=h,cx=(c['x0']+c['x1'])//2))
    rows.append(dict(idx=i+1, x0=bx0, x1=bx1, items=items))
json.dump(dict(page=page,label=label,rows=rows), open(outfile,'w',encoding='utf-8'), ensure_ascii=False, indent=1)
for r in rows:
    print('== %s seg%d x=%d..%d' % (label, r['idx'], r['x0'], r['x1']))
    for it in sorted(r['items'], key=lambda t:(t['y0'],t['x0'])):
        h,w=it['h'],it['w']; y0=it['y0']
        tag='MISC'
        if h<=3 and w>=6:
            tag='UNDER' if y0>=640 else 'DASHLINE'
        elif h<=6 and w<=6:
            tag='DOT'
        elif h>=11 and w<=14:
            tag='DIGIT'
        elif h<=9 and w>=14:
            tag='ARC'
        print('   %-8s x=%d..%d y=%d..%d w=%d h=%d' % (tag,it['x0'],it['x1'],it['y0'],it['y1'],w,h))
