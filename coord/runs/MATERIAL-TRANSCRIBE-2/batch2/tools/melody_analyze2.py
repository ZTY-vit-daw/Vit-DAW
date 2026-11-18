# -*- coding: utf-8 -*-
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
def analyze(label, xstart, barlines, ytop, ybot, outfile, append):
    lines = json.load(open(outfile)) if append else dict(lines=[])
    line=dict(label=label, bars=[])
    for i in range(len(barlines)):
        bx0 = xstart if i==0 else barlines[i-1]+2
        bx1 = barlines[i]
        rec=dict(bar=i+1, x0=bx0, x1=bx1, items=[])
        for c in comps(bx0,bx1,ytop,ybot):
            h=c['y1']-c['y0']+1; w=c['x1']-c['x0']+1
            rec['items'].append(dict(x0=c['x0'],x1=c['x1'],y0=c['y0'],y1=c['y1'],n=c['n'],w=w,h=h))
        line['bars'].append(rec)
    lines.setdefault('lines',[]).append(line)
    json.dump(lines, open(outfile,'w',encoding='utf-8'), ensure_ascii=False, indent=1)
    for b in line['bars']:
        print('== %s bar%d x=%d..%d' % (label,b['bar'],b['x0'],b['x1']))
        # annotate digits: which underlines below and dots
        items=b['items']
        for it in sorted(items, key=lambda t:(t['y0'],t['x0'])):
            cx=(it['x0']+it['x1'])//2
            tag=''
            if it['h']<=3 and it['w']>=7:
                tag='LINE'
                if it['y0']>=ytop+18: tag='UNDER%d'%(it['y0']-ytop)
                else: tag='DASHUND'
            elif it['h']<=6 and it['w']<=6:
                tag='DOT'
            elif it['h']>=13 and it['w']<=15:
                tag='DIGIT'
            elif it['h']<=10 and it['w']>=17:
                tag='ARC'
            else:
                tag='MISC'
            print('   %-8s x=%d..%d y=%d..%d w=%d h=%d' % (tag,it['x0'],it['x1'],it['y0'],it['y1'],it['w'],it['h']))
analyze('feb', 200, [384,560,782,970], 655, 712, sys.argv[2], False)
analyze('cheng', 200, [312,414,514,606,696,794,896,982], 1325, 1382, sys.argv[2], True)
