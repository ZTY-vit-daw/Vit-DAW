"""Round 3 icon concepts: 3/4 dashed idle, sparse track, knockout AB, new bus ideas, open contents."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SPRITE = (ROOT / 'round2-sprite.svg').read_text(encoding='utf-8')
NI = 'class="ni" viewBox="0 0 24 24"'

def svg(inner, size, cls=''):
    return f'<svg {NI} class="{cls}" style="width:{size}px;height:{size}px" aria-hidden="true">{inner}</svg>'

def old(name, size=24):
    return (f'<svg class="icon" style="width:{size}px;height:{size}px;--icon-bg:#FFFFFF" '
            f'aria-hidden="true"><use href="#i-{name}"/></svg>')

V_A = '''
<path d="M4 3L12 21L20 3H16L12 12.5L8 3Z" fill="currentColor"/>
<path d="M8.4 4.6L12 12.2L15.6 4.6" fill="none" stroke="var(--bg,#fff)" stroke-width="1"/>
'''
FX_A = '''
<rect x="3" y="4" width="18" height="16" fill="none" stroke="currentColor" stroke-width="2"/>
<circle cx="8.5" cy="10" r="2" fill="currentColor"/>
<circle cx="15.5" cy="10" r="2" fill="currentColor"/>
<path d="M7 15H17" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M5 4V2.5M19 4V2.5" fill="none" stroke="currentColor" stroke-width="1"/>
'''
NEW_TAPE = '''
<use href="#i-vit"/>
<rect x="17" y="2" width="6" height="6" fill="currentColor"/>
<path d="M20 3.3V6.7M18.3 5H21.7" fill="none" stroke="var(--bg,#fff)" stroke-width="1.3"/>
'''
ATTACH_REF = '''
<path d="M8 4H3V20H8" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M16 4H21V20H16" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M7 12H17" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M12 7L17 12L12 17" fill="none" stroke="currentColor" stroke-width="2"/>
'''
TRACK = '''
<path d="M3 7H21M3 17H21" fill="none" stroke="currentColor" stroke-width="1.2"/>
<path d="M4 12L7 10L10 13.4L14 10.6L17 13L20 12" fill="none" stroke="currentColor" stroke-width="1.3"/>
'''

AB = '''
<rect x="2" y="5" width="10" height="14" fill="#FFFFFF" stroke="currentColor" stroke-width="2"/>
<rect x="12" y="5" width="10" height="14" fill="currentColor"/>
<text x="7" y="15.4" text-anchor="middle" font-size="9" font-weight="700" font-family="Segoe UI,sans-serif" fill="currentColor">A</text>
<text x="17" y="15.4" text-anchor="middle" font-size="9" font-weight="700" font-family="Segoe UI,sans-serif" fill="#FFFFFF">B</text>
'''

BUS_MERGE = '''
<path d="M3 6H8L12 11M3 18H8L12 13M3 12H12" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M12 12H21" fill="none" stroke="currentColor" stroke-width="2.5"/>
<rect x="10.5" y="10.5" width="3" height="3" fill="currentColor"/>
'''
BUS_BRACKET = '''
<path d="M3 6H9M3 12H9M3 18H9" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M9 3V21" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M9 12H21" fill="none" stroke="currentColor" stroke-width="2.5"/>
<rect x="7.5" y="10.5" width="3" height="3" fill="currentColor"/>
'''
BUS_FUNNEL = '''
<path d="M3 6H9M3 12H9M3 18H9" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M9 4L15 9V15L9 20Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M15 12H21" fill="none" stroke="currentColor" stroke-width="2.5"/>
'''

OPEN_WITH_CONTENT = '''
<path d="M3 6H9L11 8H21V19H8L3 14Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M7.5 11.5H16V15.5H7.5Z" fill="none" stroke="currentColor" stroke-width="1.2"/>
<circle cx="9.2" cy="13.5" r="1" fill="currentColor"/>
<circle cx="14.3" cy="13.5" r="1" fill="currentColor"/>
'''

def state_icon(kind, size):
    arc = '<path d="M12 3A9 9 0 1 1 3 12" fill="none" stroke="currentColor" stroke-width="2"/>'
    if kind == 'idle':
        inner = '<path d="M12 3A9 9 0 1 1 3 12" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="3.2 3.2"/>'
        cls = 'spin-slow'
    elif kind == 'busy':
        inner = arc
        cls = 'spin-fast'
    elif kind == 'done':
        inner = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/><path d="M7.5 12.5 10.5 15.5 16.5 8.5" fill="none" stroke="currentColor" stroke-width="2"/>'
        cls = ''
    else:
        inner = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/><path d="M12 7.5V13.5M12 16.5V17.5" fill="none" stroke="currentColor" stroke-width="2"/>'
        cls = ''
    return svg(inner, size, cls=cls)

CSS = r'''
:root{--ink:#141414;--muted:#555555;--weak:#666666;--line:#D3D3D3;--rule:#E2E2E2;--yellow:#FACC1A;--blue:#065BC7;--red:#DB1214;--sans:Inter,"Segoe UI","Microsoft YaHei","PingFang SC",sans-serif;--mono:Consolas,monospace}
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
.adopted{display:grid;grid-template-columns:repeat(5,1fr);gap:12px}
.adopt{border:1px solid var(--line);padding:14px;background:#fff}
.adopt h3{font-size:14px;font-weight:600;display:flex;align-items:center;gap:6px}
.adopt p{font-size:11px;color:var(--muted);line-height:1.5;margin-top:6px}
.tag{font:9px/1 var(--mono);background:var(--ink);color:#fff;padding:2px 4px;font-weight:400}
.sizes{display:flex;align-items:end;gap:12px;margin-top:8px}
.sz{display:flex;flex-direction:column;align-items:center;gap:3px}
.sz b{font:9px/1.2 var(--mono);font-weight:400;color:var(--weak)}
.choices{display:grid;grid-template-columns:1fr 1fr;gap:18px}
.group{border:1px solid var(--line);padding:16px;background:#fff}
.group header{display:flex;justify-content:space-between;align-items:center;gap:12px;padding-bottom:8px;border-bottom:1px solid var(--rule)}
.group h3{font-size:16px;font-weight:600}
.old{display:flex;align-items:center;gap:8px;font:10px/1.3 var(--mono);color:var(--weak)}
.group .note{font-size:12px;color:var(--muted);line-height:1.55;margin:8px 0 10px}
.opts{display:grid;grid-template-columns:1fr 1fr;gap:10px}
.opt{border:1px solid var(--rule);padding:10px;background:#FAFAFA}
.opt h4{font-size:13px;font-weight:600}
.opt p{font-size:11px;color:var(--muted);line-height:1.5;margin-top:6px;min-height:32px}
.states{display:grid;grid-template-columns:repeat(4,1fr);gap:14px}
.statecard{border:1px solid var(--line);padding:14px;background:#fff}
.statecard h4{font-size:15px;font-weight:600}
.statecard p{font-size:11px;color:var(--muted);line-height:1.5;margin-top:6px}
.motion{display:flex;align-items:center;gap:6px;font-size:11px;color:var(--muted);margin-top:8px}
.dot{width:8px;height:8px;border-radius:50%;border:1px solid rgba(0,0,0,.35);display:inline-block;flex:none}
.dot.idle{background:#888888}.dot.busy{background:var(--yellow)}.dot.done{background:var(--blue)}.dot.error{background:var(--red)}
@keyframes spin{to{transform:rotate(360deg)}}
.spin-slow{animation:spin 9s linear infinite;transform-origin:center}
.spin-fast{animation:spin 1.6s linear infinite;transform-origin:center}
@media(prefers-reduced-motion:reduce){.spin-slow,.spin-fast{animation:none}}
.notes{display:grid;grid-template-columns:1fr 1fr;gap:18px;margin-top:22px}
.note2{border:1px solid var(--line);padding:14px 16px;background:#FAFAFA;font-size:12px;line-height:1.6;color:#333}
.note2 h3{font-size:14px;margin-bottom:5px;color:var(--ink)}
footer{margin-top:26px;border-top:2px solid var(--ink);padding-top:12px;display:flex;justify-content:space-between;gap:24px;font-size:12px;color:var(--weak)}
'''.strip()

def size_row(inner, sizes=(32,24,16), cls=''):
    return ''.join(f'<span class="sz">{svg(inner,s,cls=cls)}<b>{s}</b></span>' for s in sizes)

adopted = ''
project_sizes = ''.join(f'<span class="sz">{old("vit",s)}<b>{s}</b></span>' for s in (32,24,16))
adopted += f'<article class="adopt"><h3>project <span class="tag">沿用</span></h3><div class="sizes">{project_sizes}</div><p>旧磁带，改名 i-project。</p></article>'
adopted += f'<article class="adopt"><h3>new <span class="tag">定</span></h3><div class="sizes">{size_row(NEW_TAPE)}</div><p>旧磁带 + 右上角加号。</p></article>'
adopted += f'<article class="adopt"><h3>V <span class="tag">定</span></h3><div class="sizes">{size_row(V_A)}</div><p>只用于 OS 表层。</p></article>'
adopted += f'<article class="adopt"><h3>fx <span class="tag">定</span></h3><div class="sizes">{size_row(FX_A)}</div><p>方形机架机器人头。</p></article>'
adopted += f'<article class="adopt"><h3>attach <span class="tag">定</span></h3><div class="sizes">{size_row(ATTACH_REF)}</div><p>旧结构精修：括号细、箭头粗。</p></article>'

choices = ''
choices += (f'<article class="group"><header><h3>轨道 track</h3><span class="old">{old("track")}<b>旧 i-track</b></span></header>'
            f'<p class="note">三条 lane，中间细波形；按你说的再稀疏一档（约 5 段）。</p><div class="opts">'
            f'<div class="opt"><h4>稀疏波形（采用）</h4><div class="sizes">{size_row(TRACK)}</div><p>1.3px，不粗；16px 仍能读出波形。</p></div></div></article>')
choices += (f'<article class="group"><header><h3>对比 A/B</h3><span class="old">{old("ab")}<b>旧 i-ab</b></span></header>'
            f'<p class="note">单块分割保留；左块白底黑 A，右块黑底白 B；字母最终做成镂空负空间。</p><div class="opts">'
            f'<div class="opt"><h4>白底黑 A / 黑底白 B（采用）</h4><div class="sizes">{size_row(AB)}</div><p>左白右黑，字母为镂空；16px 仍以两块对比为主。</p></div></div></article>')
choices += (f'<article class="group"><header><h3>打开文件夹 open</h3><span class="old">{old("open")}<b>旧 i-open</b></span></header>'
            f'<p class="note">单轮廓斜左保留；把磁带内容物补回文件夹里，不再是一张空壳。</p><div class="opts">'
            f'<div class="opt"><h4>斜左轮廓 + 磁带（采用）</h4><div class="sizes">{size_row(OPEN_WITH_CONTENT)}</div><p>外侧 2px，磁带 1.2px；左下是斜的，没有直角。</p></div></div></article>')

bus = (f'<article class="group"><header><h3>总线 bus · 重新设计</h3><span class="old">{old("bus")}<b>旧 i-bus</b></span></header>'
       f'<p class="note">之前两版都作废。这次不再用“方框/推子”，只回答“多路并成一路”。三个方向：</p><div class="opts" style="grid-template-columns:1fr 1fr 1fr">'
       f'<div class="opt"><h4>A 汇流（推荐）</h4><div class="sizes">{size_row(BUS_MERGE)}</div><p>三条 lane 汇成一条 2.5px 粗线；汇合点一个方节点。</p></div>'
       f'<div class="opt"><h4>B 集束括号</h4><div class="sizes">{size_row(BUS_BRACKET)}</div><p>三条 lane 被一根括号收成一束，再接粗输出。</p></div>'
       f'<div class="opt"><h4>C 汇流漏斗</h4><div class="sizes">{size_row(BUS_FUNNEL)}</div><p>三路进漏斗，一路出；更强“求和”，但更像滤波器。</p></div>'
       f'</div></article>')
choices += bus

states = ''
for kind, name, desc, motion in [
    ('idle', '待机', '3/4 虚线弧；已连接，没有回合在跑。', '9s 慢转'),
    ('busy', '进行中', '3/4 实线弧；回合/工具执行中。', '1.6s 快转'),
    ('done', '完成', '满环 + 勾；收成回执。', '不转'),
    ('error', '出错', '满环 + 感叹；原因与重试。', '不转'),
]:
    states += (f'<div class="statecard"><h4>{name}</h4><div class="sizes">'
               + ''.join(f'<span class="sz">{state_icon(kind,s)}<b>{s}</b></span>' for s in (16,24,32))
               + f'</div><div class="motion"><span class="dot {kind}"></span>8px · {motion}</div><p>{desc}</p></div>')

html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' \
       '<title>Vit v5.1 图标概念 · 第三轮</title><style>' + CSS + '</style></head><body>' + SPRITE + \
       '<main class="page"><header class="top"><div><h1>Vit v5.1 · 图标概念（第三轮）</h1>' \
       '<p>状态改成“待机 3/4 虚线弧、进行中 3/4 实线弧”，两者都转；轨道再稀疏；AB 改成左白右黑镂空；' \
       '打开文件夹补回磁带内容物；bus 三个新方向。</p></div>' \
       '<div class="right">概念对照 / 2026-11-14<br>只保留还没定的 bus</div></header>' \
       '<h2 class="section">已定</h2><div class="adopted">' + adopted + '</div>' \
       '<h2 class="section">状态修正</h2>' \
       '<p class="lead">待机和进行中现在是同一个 3/4 弧：待机用虚线、进行中用实线；两者都旋转（待机 9s、进行中 1.6s）。' \
       '完成/出错仍是满环 + 中心标记。8px 点只保留颜色。</p><div class="states">' + states + '</div>' \
       '<h2 class="section">还需要看</h2><div class="choices">' + choices + '</div>' \
       '<div class="notes"><div class="note2"><h3>这轮修正</h3>' \
       '状态：虚线/实线都收成 3/4 弧，保留旋转。轨道：5 段稀疏波形。AB：左白底黑 A、右黑底白 B，最终做镂空。' \
       '打开：单轮廓斜左 + 磁带内容物。bus：汇流 / 集束括号 / 漏斗三选一。</div>' \
       '<div class="note2"><h3>下一轮</h3>' \
       'bus 选定后即画最终 symbol；更新 `round2-sprite.svg`、S3 / S5 / P3，重跑审计。A/B 字母、V 内槽、fx 旋钮会在最终稿里用几何笔画重画。</div></div>' \
       '<footer><span>概念第三轮 · 不进入产品屏</span><span>下一步：bus 选 A/B/C → 最终 symbol → 审计</span></footer>' \
       '</main></body></html>'

(ROOT / 'icon-concepts-v5-round3.html').write_text(html, encoding='utf-8')
print('wrote', ROOT / 'icon-concepts-v5-round3.html', len(html), 'bytes')
