"""Round 5: thinner open outline and finer AB knockout; test non-scaling hairlines."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SPRITE = (ROOT / 'round2-sprite.svg').read_text(encoding='utf-8')
NI = 'class="ni" viewBox="0 0 24 24"'

def svg(inner, size):
    return f'<svg {NI} style="width:{size}px;height:{size}px" aria-hidden="true">{inner}</svg>'

def old(name, size=24):
    return (f'<svg class="icon" style="width:{size}px;height:{size}px;--icon-bg:#FFFFFF" '
            f'aria-hidden="true"><use href="#i-{name}"/></svg>')

# Open: same approved shape/content; structural 1.5px, front 1.2px, tape 1px, all non-scaling.
OPEN_FINAL = '''
<path d="M3 6H9L11 8H21V19H7L3 14Z" fill="none" stroke="currentColor" stroke-width="1.5" vector-effect="non-scaling-stroke"/>
<path d="M3 10.5H21M3 10.5L7 19" fill="none" stroke="currentColor" stroke-width="1.2" vector-effect="non-scaling-stroke"/>
<rect x="9" y="12.5" width="6" height="3" fill="none" stroke="currentColor" stroke-width="1" vector-effect="non-scaling-stroke"/>
'''

# AB: true knockout; tile border 1.5px, letter cut 1.2px, non-scaling.
AB_FINAL = '''
<defs>
 <mask id="abA5" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24">
  <rect x="0" y="0" width="24" height="24" fill="#FFFFFF"/>
  <path d="M4.1 18L7.5 5.6L10.9 18M5.5 13.7H9.5" fill="none" stroke="#000000" stroke-width="1.2" vector-effect="non-scaling-stroke"/>
 </mask>
 <mask id="abB5" maskUnits="userSpaceOnUse" x="0" y="0" width="24" height="24">
  <rect x="0" y="0" width="24" height="24" fill="#FFFFFF"/>
  <path d="M15.2 5.6V18.4M15.2 5.6H17.9A3.2 3.2 0 0 1 17.9 12.1H15.2M15.2 12.1H18.2A3.2 3.2 0 0 1 18.2 18.4H15.2" fill="none" stroke="#000000" stroke-width="1.2" vector-effect="non-scaling-stroke"/>
 </mask>
</defs>
<rect x="2" y="5" width="10" height="14" fill="currentColor"/>
<rect x="2" y="5" width="10" height="14" fill="#FFFFFF" mask="url(#abA5)"/>
<rect x="2" y="5" width="10" height="14" fill="none" stroke="currentColor" stroke-width="1.5" vector-effect="non-scaling-stroke"/>
<rect x="12" y="5" width="10" height="14" fill="#FFFFFF"/>
<rect x="12" y="5" width="10" height="14" fill="currentColor" mask="url(#abB5)"/>
'''

BUS_FINAL = '''
<path d="M3 7H8L12 11M3 17H8L12 13" fill="none" stroke="currentColor" stroke-width="1.5" vector-effect="non-scaling-stroke"/>
<path d="M12 12H21" fill="none" stroke="currentColor" stroke-width="2.5" vector-effect="non-scaling-stroke"/>
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
.icon,.ni{display:inline-block;fill:none;stroke:currentColor;stroke-linecap:butt;stroke-linejoin:miter;overflow:visible;vertical-align:middle}
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

open_card = (f'<article class="fix"><header><h3>打开文件夹 open</h3><span class="old">{old("open")}<b>旧 i-open</b></span></header>'
             f'<p class="note">形不变；把外轮廓从 2px 收到 1.5px，前盖 1.2px，磁带边缘 1px。'
             f'这三条线用 non-scaling 描边，48px 不会跟着变粗，16px 也不会消失。</p>'
             f'<div class="sizes">' + ''.join(f'<span class="sz">{svg(OPEN_FINAL,s)}<b>{s}</b></span>' for s in (48,32,24,16)) +
             f'</div><div class="detailbox"><h4>96px 细节</h4>{svg(OPEN_FINAL,96)}</div></article>')

ab_card = (f'<article class="fix"><header><h3>对比 A/B</h3><span class="old">{old("ab")}<b>旧 i-ab</b></span></header>'
           f'<p class="note">结构不变；块边框 1.5px，字母切口 1.2px，同样 non-scaling。'
           f'48px 看的是细切口，16px 看的是两块黑白；如果 16px 仍糊，下一步只简化字母笔画，不再加粗。</p>'
           f'<div class="sizes">' + ''.join(f'<span class="sz">{svg(AB_FINAL,s)}<b>{s}</b></span>' for s in (48,32,24,16)) +
           f'</div><div class="detailbox"><h4>144px 细节（切口）</h4>{svg(AB_FINAL,144)}</div></article>')

bus_card = (f'<article class="fix" style="grid-column:1 / -1"><header><h3>总线 bus（不变）</h3><span class="old">{old("bus")}<b>旧 i-bus</b></span></header>'
            f'<p class="note">你未否，保持两路斜汇 + 2.5px 粗输出；non-scaling，16px 只剩“入 → 出”。</p>'
            f'<div class="sizes">' + ''.join(f'<span class="sz">{svg(BUS_FINAL,s)}<b>{s}</b></span>' for s in (48,32,24,16)) +
            f'</div></article>')

html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' \
       '<title>Vit v5.1 图标 · 第五轮（细线）</title><style>' + CSS + '</style></head><body>' + SPRITE + \
       '<main class="page"><header class="top"><div><h1>Vit v5.1 · 图标（第五轮 · 细线）</h1>' \
       '<p>只收细 open 与 AB：两条线改用不随尺寸缩放的描边（1.5 / 1.2 / 1px），48px 不粗、16px 不消失；' \
       '形和内容物不变。bus 保持两路精简版。</p></div>' \
       '<div class="right">修正对照 / 2026-11-14<br>只差厚度这一刀</div></header>' \
       '<h2 class="section">细线修正</h2><div class="fixes">' + open_card + ab_card + bus_card + '</div>' \
       '<div class="notes"><div class="note2"><h3>为什么用 non-scaling</h3>' \
       '这两枚是“细节型”图标：外轮廓、前盖、字母切口。如果按 24 网格缩放，48px 时 1.5px 会变成 3px，' \
       '又回到“太粗”；non-scaling 让线在 16–48px 都保持同样的物理细度，所以大尺寸更精致、小尺寸不糊。</div>' \
       '<div class="note2"><h3>还觉得粗怎么办</h3>' \
       'open 可再收到外 1.2 / 前盖 1 / 磁带 0.9；AB 可把字母切口收到 1.0，或只保留 A/B 各两笔。' \
       '但 16px 会开始丢细节，所以我先停在 1.5 / 1.2。你看 96/144px 细节后定。</div></div>' \
       '<footer><span>细线修正稿 · 不进入产品屏</span><span>确认后 → 最终 symbol → S3/S5/P3 + 审计</span></footer>' \
       '</main></body></html>'

(ROOT / 'icon-concepts-v5-round5.html').write_text(html, encoding='utf-8')
print('wrote', ROOT / 'icon-concepts-v5-round5.html', len(html), 'bytes')
