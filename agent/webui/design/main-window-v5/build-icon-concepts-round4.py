"""Round 4: final corrections for open (first-sheet A content, slanted corner), AB (fine knockout), bus A (simplified)."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SPRITE = (ROOT / 'round2-sprite.svg').read_text(encoding='utf-8')
NI = 'class="ni" viewBox="0 0 24 24"'

def svg(inner, size):
    return f'<svg {NI} style="width:{size}px;height:{size}px" aria-hidden="true">{inner}</svg>'

def old(name, size=24):
    return (f'<svg class="icon" style="width:{size}px;height:{size}px;--icon-bg:#FFFFFF" '
            f'aria-hidden="true"><use href="#i-{name}"/></svg>')

# Open: first-sheet A content (front flap + tape edge), only the bottom-left corner becomes a slant.
OPEN_FINAL = '''
<path d="M3 6H9L11 8H21V19H7L3 14Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M3 10.5H21M3 10.5L7 19" fill="none" stroke="currentColor" stroke-width="1.5"/>
<rect x="9" y="12.5" width="6" height="3" fill="none" stroke="currentColor" stroke-width="1"/>
'''

# AB: true knockout, thin letters, left white with black A, right black with white B.
AB_FINAL = '''
<defs>
 <mask id="abA" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24">
  <rect x="0" y="0" width="24" height="24" fill="#FFFFFF"/>
  <path d="M4.1 17.8L7.5 5.8L10.9 17.8M5.4 13.6H9.6" fill="none" stroke="#000000" stroke-width="1.5"/>
 </mask>
 <mask id="abB" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24">
  <rect x="0" y="0" width="24" height="24" fill="#FFFFFF"/>
  <path d="M15.2 5.8V18.2M15.2 5.8H17.8A3.1 3.1 0 0 1 17.8 12H15.2M15.2 12H18.1A3.1 3.1 0 0 1 18.1 18.2H15.2" fill="none" stroke="#000000" stroke-width="1.5"/>
 </mask>
</defs>
<rect x="2" y="5" width="10" height="14" fill="currentColor"/>
<rect x="2" y="5" width="10" height="14" fill="#FFFFFF" mask="url(#abA)"/>
<rect x="2" y="5" width="10" height="14" fill="none" stroke="currentColor" stroke-width="2"/>
<rect x="12" y="5" width="10" height="14" fill="#FFFFFF"/>
<rect x="12" y="5" width="10" height="14" fill="currentColor" mask="url(#abB)"/>
'''

# Bus A simplified: two lanes merge into one thick output, no node, no middle input.
BUS_FINAL = '''
<path d="M3 7H8L12 11M3 17H8L12 13" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M12 12H21" fill="none" stroke="currentColor" stroke-width="2.5"/>
'''

CSS = r'''
:root{--ink:#141414;--muted:#555555;--weak:#666666;--line:#D3D3D3;--rule:#E2E2E2;--sans:Inter,"Segoe UI","Microsoft YaHei","PingFang SC",sans-serif;--mono:Consolas,monospace}
*{box-sizing:border-box}html,body{margin:0;width:1440px;background:#E4E4E4;color:var(--ink);font:16px/1.65 var(--sans)}
h1,h2,h3,h4,p{margin:0}
.page{width:1440px;padding:44px 56px 56px;background:#FFFFFF}
header.top{display:flex;justify-content:space-between;align-items:end;gap:32px;padding-bottom:24px;border-bottom:2px solid var(--ink)}
header.top h1{font-size:28px;font-weight:700;line-height:1.3}
header.top p{font-size:14px;color:var(--muted);line-height:1.55;margin-top:10px;max-width:66em}
header.top .right{font-size:13px;color:var(--weak);text-align:right;white-space:nowrap;line-height:1.6}
h2.section{font-size:20px;margin:30px 0 14px}
.lead{font-size:14px;color:var(--muted);line-height:1.6;max-width:74em;margin-bottom:14px}
.icon,.ni{display:inline-block;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:butt;stroke-linejoin:miter;overflow:visible;vertical-align:middle}
.solid{fill:currentColor;stroke:none}.selected-fill{fill:var(--icon-fill,none)}.knockout{fill:var(--icon-bg,#fff);stroke:none}.detail{stroke:var(--icon-mark,currentColor)}.alert-mark{fill:var(--icon-mark,currentColor);stroke:none}
.sprite{position:absolute;width:0;height:0;overflow:hidden}
.fixes{display:grid;grid-template-columns:1fr 1fr;gap:20px}
.fix{border:1px solid var(--line);padding:18px;background:#fff}
.fix header{display:flex;justify-content:space-between;align-items:center;gap:14px;padding-bottom:10px;border-bottom:1px solid var(--rule)}
.fix h3{font-size:17px;font-weight:600}
.old{display:flex;align-items:center;gap:8px;font:11px/1.3 var(--mono);color:var(--weak)}
.fix .note{font-size:13px;color:var(--muted);line-height:1.55;margin:10px 0 12px}
.sizes{display:flex;align-items:end;gap:18px}
.sz{display:flex;flex-direction:column;align-items:center;gap:4px}
.sz b{font:10px/1.2 var(--mono);font-weight:400;color:var(--weak)}
.detailbox{background:#FAFAFA;border:1px solid var(--rule);padding:14px;margin-top:10px}
.detailbox h4{font-size:13px;font-weight:600;margin-bottom:8px}
.notes{display:grid;grid-template-columns:1fr 1fr;gap:20px;margin-top:24px}
.note2{border:1px solid var(--line);padding:16px 18px;background:#FAFAFA;font-size:13px;line-height:1.6;color:#333}
.note2 h3{font-size:15px;margin-bottom:6px;color:var(--ink)}
footer{margin-top:28px;border-top:2px solid var(--ink);padding-top:14px;display:flex;justify-content:space-between;gap:24px;font-size:13px;color:var(--weak)}
'''.strip()

def size_row(inner, sizes=(48,32,24,16)):
    return ''.join(f'<span class="sz">{svg(inner,s)}<b>{s}</b></span>' for s in sizes)

open_card = (f'<article class="fix"><header><h3>打开文件夹 open</h3><span class="old">{old("open")}<b>旧 i-open</b></span></header>'
             f'<p class="note">按你说的：取第一版 A 的内容物（前盖 + 磁带边缘），只把左下角改成斜角，不再留直角。'
             f'外侧 2px、前盖 1.5px、磁带边缘 1px。</p>'
             f'<div class="sizes">{size_row(OPEN_FINAL)}</div>'
             f'<div class="detailbox"><h4>48px 细节</h4>{svg(OPEN_FINAL,96)}</div></article>')

ab_card = (f'<article class="fix"><header><h3>对比 A/B</h3><span class="old">{old("ab")}<b>旧 i-ab</b></span></header>'
           f'<p class="note">单块分割保留；左白底黑 A、右黑底白 B。这次用真镂空：字母是块上的细线切口，'
           f'不是压在块上的粗字；1.5px 字线、13px 字高，48px 看细节，16px 看两块对比。</p>'
           f'<div class="sizes">{size_row(AB_FINAL)}</div>'
           f'<div class="detailbox"><h4>48px 细节（镂空切口）</h4>{svg(AB_FINAL,144)}</div></article>')

bus_card = (f'<article class="fix" style="grid-column:1 / -1"><header><h3>总线 bus · A 精简</h3><span class="old">{old("bus")}<b>旧 i-bus</b></span></header>'
            f'<p class="note">A 的方向保留：多路汇成一路。精简掉中间输入和汇合方节点——两路 lane 斜汇，接一条 2.5px 粗输出；'
            f'只剩“两路进、一路出”一个动作。</p>'
            f'<div class="sizes">{size_row(BUS_FINAL)}</div>'
            f'<div class="detailbox"><h4>48px 细节</h4>{svg(BUS_FINAL,96)}</div></article>')

html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' \
       '<title>Vit v5.1 图标 · 第四轮修正</title><style>' + CSS + '</style></head><body>' + SPRITE + \
       '<main class="page"><header class="top"><div><h1>Vit v5.1 · 图标（第四轮修正）</h1>' \
       '<p>只改三处：open 取第一版 A 的内容物、去掉左下直角；AB 做真镂空细字；bus A 精简。' \
       '其余状态 / 轨道 / attach / project / new / V / fx 已定，不再展示。</p></div>' \
       '<div class="right">修正对照 / 2026-11-14<br>这页只回答三处</div></header>' \
       '<h2 class="section">三处修正</h2><div class="fixes">' + open_card + ab_card + bus_card + '</div>' \
       '<div class="notes"><div class="note2"><h3>AB 这次不是“黑块白字”</h3>' \
       '左块是白底，A 是从白块上切掉的细线，露出下面的黑；右块是黑底，B 是从黑块上切掉的细线，露出下面的白。' \
       '48px 能看到切口和字腔；16px 如果仍糊，我会把字线加到 1.8px，或只保留更简的两笔字母。</div>' \
       '<div class="note2"><h3>bus 的精简点</h3>' \
       '去掉中间那条输入和汇合方节点；两路斜汇后直接接粗输出。这样 16px 只剩“入 → 出”一条动线。' \
       '如果你觉得两路不够“总线”，我再加回第三路但保持无节点。</div></div>' \
       '<footer><span>修正稿 · 不进入产品屏</span><span>三处确认后 → 最终 symbol → S3/S5/P3 + 审计</span></footer>' \
       '</main></body></html>'

(ROOT / 'icon-concepts-v5-round4.html').write_text(html, encoding='utf-8')
print('wrote', ROOT / 'icon-concepts-v5-round4.html', len(html), 'bytes')
