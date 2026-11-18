# -*- coding: utf-8 -*-
"""Analyze a jianpu melody line: components classified into digits/grace/underlines/dots/dashes/arcs."""
import sys, json
from PIL import Image
im = Image.open(sys.argv[1]).convert('RGB'); px=im.load()
def dark(x,y): return sum(px[x,y])<450

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

def analyze(label, xstart, barlines, ytop, ybot, task_digits_y, outfile):
    lines = dict(label=label, bars=[])
    for i in range(len(barlines)):
        bx0 = xstart if i==0 else barlines[i-1]+2
        bx1 = barlines[i]
        cs = comps(bx0,bx1,ytop,ybot)
        rec=dict(bar=i+1, x=(bx0,bx1), items=[])
        for c in cs:
            h=c['y1']-c['y0']+1; w=c['x1']-c['x0']+1
            cx=round((c['x0']+c['x1'])/2)
            if h>=13 and w<=16:
                kind='digit'
            elif h<=3 and w>=7 and c['y0']>=task_digits_y+18:
                kind='underline'
            elif h<=4 and w>=7:
                kind='dash'
            elif h<=6 and w<=6 and c['y0']>=task_digits_y+16:
                kind='dot_below'
            elif h<=6 and w<=6 and c['y0']<task_digits_y-2:
                kind='dot_above'
            elif h<=6 and w<=6:
                kind='dot_mid'
            elif h<=10 and w>=18:
                kind='arc'
            else:
                kind='other'
            rec['items'].append(dict(kind=kind,x0=c['x0'],x1=c['x1'],y0=c['y0'],y1=c['y1'],cx=cx,w=w,h=h))
        lines['bars'].append(rec)
    json.dump(lines, open(outfile,'w',encoding='utf-8'), ensure_ascii=False, indent=1)
    for b in lines['bars']:
        print('== %s bar%d x=%s' % (label,b['bar'],b['x']))
        for it in sorted(b['items'], key=lambda t:(t['y0'],t['x0'])):
            if it['kind']=='other' or it['kind']=='dot_mid':
                print('   %-9s x=%d..%d y=%d..%d w=%d' % (it['kind'],it['x0'],it['x1'],it['y0'],it['y1'],it['w']))
            else:
                print('   %-9s cx=%d y=%d..%d w=%d h=%d' % (it['kind'],it['cx'],it['y0'],it['y1'],it['w'],it['h']))

# feb: digits y 678..695; underlines 700..703; grace above ~660..677
analyze('feb', 200, [384,560,782,970], 655, 712, 678, sys.argv[2])
# cheng: digits 1348..1365; underlines ~1370..1373; grace above
analyze('cheng', 240, [312,414,514,606,696,794,896,982], 1325, 1382, 1348, sys.argv[2])
