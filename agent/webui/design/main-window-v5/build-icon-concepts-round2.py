"""Round 2 icon concepts: adopt the confirmed pieces, show only the open choices."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SPRITE = (ROOT / 'round2-sprite.svg').read_text(encoding='utf-8')
NI = 'class="ni" viewBox="0 0 24 24"'

def svg(inner, size, extra='', cls=''):
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

TRACK_A = '''
<path d="M3 7H21M3 17H21" fill="none" stroke="currentColor" stroke-width="1.2"/>
<path d="M4 12L5.2 9.8L6.5 13.4L7.8 10.6L9.2 12.8L10.5 9.4L11.8 13.2L13.2 11L14.6 13.6L16 9.9L17.4 12.6L18.8 10.4L20 12" fill="none" stroke="currentColor" stroke-width="1.4"/>
'''
TRACK_B = '''
<path d="M3 7H21M3 17H21" fill="none" stroke="currentColor" stroke-width="1.2"/>
<path d="M4 12.2L4.8 10.1L5.7 13.8L6.6 9.6L7.5 13.1L8.4 10.9L9.3 14L10.2 9.3L11.1 12.7L12 11.2L12.9 14.1L13.8 9.8L14.7 12.4L15.6 10.4L16.5 13.5L17.4 9.9L18.3 12.6L19.2 10.7L20 12.2" fill="none" stroke="currentColor" stroke-width="1.2"/>
'''

AB_1 = '''
<rect x="2" y="5" width="20" height="14" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M2 5H12V19H2Z" fill="currentColor"/>
<path d="M12 5V19" fill="none" stroke="currentColor" stroke-width="1"/>
<text x="7" y="15.4" text-anchor="middle" font-size="9" font-weight="700" font-family="Segoe UI,sans-serif" fill="var(--bg,#fff)">A</text>
<text x="17" y="15.4" text-anchor="middle" font-size="9" font-weight="700" font-family="Segoe UI,sans-serif" fill="currentColor">B</text>
'''
AB_2 = '''
<path d="M3 19L7 5L11 19H8.9L8.1 16.2H5.9L5.1 19Z" fill="currentColor"/>
<path d="M12.4 20L14.2 4" fill="none" stroke="currentColor" stroke-width="1.2"/>
<path d="M16.5 5H19.2A3.2 3.2 0 0 1 19.2 11.2H16.5ZM16.5 11.2H19.7A3.2 3.2 0 0 1 19.7 17.4H16.5Z" fill="none" stroke="currentColor" stroke-width="2"/>
'''

BUS_OLD = '''
<rect x="4" y="4" width="6" height="6" fill="none" stroke="currentColor" stroke-width="2"/>
<rect x="14" y="4" width="6" height="6" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M7 10V13H17V10M12 13V18" fill="none" stroke="currentColor" stroke-width="1.5"/>
<rect x="10" y="11" width="4" height="4" fill="currentColor"/>
<path d="M8 21H16" fill="none" stroke="currentColor" stroke-width="2.5"/>
'''
BUS_MASTER = '''
<path d="M12 3V21" fill="none" stroke="currentColor" stroke-width="2"/>
<rect x="6.5" y="7" width="11" height="4" fill="currentColor"/>
<path d="M4 19.5H20" fill="none" stroke="currentColor" stroke-width="2.5"/>
<path d="M6 12.5H18M6 16H18" fill="none" stroke="currentColor" stroke-width="1" opacity=".55"/>
'''

OPEN_1 = '''
<path d="M3 6H9L11 8H21V19H8L3 14Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M4 14L9 19H21" fill="none" stroke="currentColor" stroke-width="1.5"/>
'''
OPEN_2 = '''
<path d="M3 6H9L11 8H21V19H3Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M3 11L7 19H21L18 11Z" fill="none" stroke="currentColor" stroke-width="1.5"/>
'''

ATTACH_REF = '''
<path d="M8 4H3V20H8" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M16 4H21V20H16" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M7 12H17" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M12 7L17 12L12 17" fill="none" stroke="currentColor" stroke-width="2"/>
'''

def state_icon(kind, size, animated=True):
    base = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/>'
    if kind == 'idle':
        inner = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="3.4 3.4" transform="rotate(-45 12 12)"/>'
        cls = 'spin-slow' if animated else ''
    elif kind == 'busy':
        inner = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="42 14" transform="rotate(-90 12 12)"/>'
        cls = 'spin-fast' if animated else ''
    elif kind == 'done':
        inner = base + '<path d="M7.5 12.5 10.5 15.5 16.5 8.5" fill="none" stroke="currentColor" stroke-width="2"/>'
        cls = ''
    else:
        inner = base + '<path d="M12 7.5V13.5M12 16.5V17.5" fill="none" stroke="currentColor" stroke-width="2"/>'
        cls = ''
    return svg(inner, size, cls=cls)

# ---------- layout ----------

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
.adopted{display:grid;grid-template-columns:repeat(4,1fr);gap:16px}
.adopt{border:1px solid var(--line);padding:16px;background:#FFFFFF}
.adopt h3{font-size:16px;display:flex;align-items:center;gap:8px}
.adopt p{font-size:12px;color:var(--muted);line-height:1.55;margin-top:8px}
.tag{font:10px/1 var(--mono);background:var(--ink);color:#fff;padding:3px 5px;font-weight:400}
.sizes{display:flex;align-items:end;gap:14px;margin-top:10px}
.sz{display:flex;flex-direction:column;align-items:center;gap:4px}
.sz b{font:10px/1.2 var(--mono);font-weight:400;color:var(--weak)}
.choices{display:grid;grid-template-columns:1fr 1fr;gap:20px}
.group{border:1px solid var(--line);padding:18px;background:#fff}
.group header{display:flex;justify-content:space-between;align-items:center;gap:16px;padding-bottom:10px;border-bottom:1px solid var(--rule)}
.group h3{font-size:17px;font-weight:600}
.old{display:flex;align-items:center;gap:8px;font:11px/1.3 var(--mono);color:var(--weak)}
.group .note{font-size:13px;color:var(--muted);line-height:1.55;margin:10px 0 12px}
.opts{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.opt{border:1px solid var(--rule);padding:12px;background:#FAFAFA}
.opt h4{font-size:14px;font-weight:600}
.opt p{font-size:12px;color:var(--muted);line-height:1.5;margin-top:8px;min-height:34px}
.states{display:grid;grid-template-columns:repeat(4,1fr);gap:16px}
.statecard{border:1px solid var(--line);padding:16px;background:#fff}
.statecard h4{font-size:16px;font-weight:600}
.statecard p{font-size:12px;color:var(--muted);line-height:1.55;margin-top:8px}
.dot{width:8px;height:8px;border-radius:50%;border:1px solid rgba(0,0,0,.35);display:inline-block;flex:none}
.dot.idle{background:#888888}.dot.busy{background:var(--yellow)}.dot.done{background:var(--blue)}.dot.error{background:var(--red)}
.motion{display:flex;align-items:center;gap:8px;font-size:12px;color:var(--muted);margin-top:10px}
@keyframes spin{to{transform:rotate(360deg)}}
.spin-slow{animation:spin 9s linear infinite;transform-origin:center}
.spin-fast{animation:spin 1.6s linear infinite;transform-origin:center}
@media(prefers-reduced-motion:reduce){.spin-slow,.spin-fast{animation:none}}
.notes{display:grid;grid-template-columns:1fr 1fr;gap:20px;margin-top:24px}
.note2{border:1px solid var(--line);padding:16px 18px;background:#FAFAFA;font-size:13px;line-height:1.6;color:#333}
.note2 h3{font-size:15px;margin-bottom:6px;color:var(--ink)}
footer{margin-top:28px;border-top:2px solid var(--ink);padding-top:14px;display:flex;justify-content:space-between;gap:24px;font-size:13px;color:var(--weak)}
'''.strip()

def size_row(inner, sizes=(32,24,16), cls=''):
    return ''.join(f'<span class="sz">{svg(inner,s,cls=cls)}<b>{s}</b></span>' for s in sizes)

def adopt_card(title, inner, desc, note='采用', cls=''):
    return (f'<article class="adopt"><h3>{title} <span class="tag">{note}</span></h3>'
            f'<div class="sizes">{size_row(inner, cls=cls)}</div><p>{desc}</p></article>')

def choice_group(title, old_name, note, options):
    old_html = f'<span class="old">{old(old_name)}<b>旧 i-{old_name}</b></span>' if old_name else ''
    opts = ''.join(f'<div class="opt"><h4>{label}</h4><div class="sizes">{size_row(inner, cls=cls)}</div><p>{desc}</p></div>'
                   for label, desc, inner, cls in options)
    return (f'<article class="group"><header><h3>{title}</h3>{old_html}</header>'
            f'<p class="note">{note}</p><div class="opts">{opts}</div></article>')

adopted = ''.join([
    adopt_card('新建工程 new', NEW_TAPE, '旧磁带 + 右上角实心加号徽章；16px 以加号为识别点。'),
    adopt_card('品牌标 V', V_A, '实心 V + 1px 内槽，贴近桌面倒角；只用于 OS 表层。'),
    adopt_card('效果器 fx', FX_A, '方形机架机器人头：旋钮眼 + 插槽嘴。'),
])
# project is a <use> reference; build it manually and prepend it.
project_sizes = ''.join(f'<span class="sz">{old("vit",s)}<b>{s}</b></span>' for s in (32,24,16))
adopted = (f'<article class="adopt"><h3>工程 project <span class="tag">沿用</span></h3>'
           f'<div class="sizes">{project_sizes}</div><p>沿用旧磁带 icon，改名为 i-project；不重画。</p></article>' + adopted)

choices = ''.join([
    choice_group('轨道 track', 'track', '结构按 B：三条 lane，中间是不规则细波形；两版只差波形疏密。', [
        ('1 稀疏波形（推荐）', '约 10 段，16px 仍能读成“有波形”', TRACK_A, ''),
        ('2 密波形', '约 18 段，更像真实波形，但 16px 可能糊', TRACK_B, ''),
    ]),
    choice_group('对比 A/B', 'ab', '两个旧案都不要；只保留“一黑一白”的字母对比，去掉多余容器。', [
        ('1 单块分割（推荐）', '一整块，左黑右白，中间 1px 分界；A 反白 / B 墨色', AB_1, ''),
        ('2 字母 + 斜杠', '无容器，实心 A + 空心 B，中间一道斜杠', AB_2, ''),
    ]),
    choice_group('总线 bus', 'bus', '确认是工程总线。旧版逻辑更好；master 是备选。', [
        ('1 旧版精修（推荐）', '两个编组节点汇入 + 粗输出；保留旧逻辑，只调线重', BUS_OLD, ''),
        ('2 Master', '单根 master 推子 + 底部主输出条；更“主控”，但更像推子', BUS_MASTER, ''),
    ]),
    choice_group('打开文件夹 open', 'open', '简化 A：去掉磁带边缘和多余折线；左侧做成斜角，不留左下直角。', [
        ('1 单轮廓斜左（推荐）', '一个外轮廓 + 一条内侧折线；左侧下缘是斜的', OPEN_1, ''),
        ('2 前后两片', '后片 + 斜左前片，更明确“打开”，但多一条结构', OPEN_2, ''),
    ]),
    choice_group('引用轨道/区间 attach', 'attach', '保留旧结构：区间括号 + 右箭头；只把括号收细、箭头加粗。', [
        ('旧版精修（推荐）', '括号 1.5px + 箭头 2px，和 send 的纯箭头区分', ATTACH_REF, ''),
    ]),
])

states = ''
for kind, name, desc, motion in [
    ('idle', '待机', '虚线环；已连接，没有回合在跑。', '9s 慢转'),
    ('busy', '进行中', '实线 3/4 弧；回合/工具执行中。', '1.6s 快转'),
    ('done', '完成', '满环 + 勾；收成回执。', '不转'),
    ('error', '出错', '满环 + 感叹；原因与重试。', '不转'),
]:
    states += (f'<div class="statecard"><h4>{name}</h4><div class="sizes">'
               + ''.join(f'<span class="sz">{state_icon(kind,s)}<b>{s}</b></span>' for s in (16,24,32))
               + f'</div><div class="motion"><span class="dot {kind}"></span>8px 身份点 · {motion}</div><p>{desc}</p></div>')

html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' \
       '<title>Vit v5.1 图标概念 · 第二轮</title><style>' + CSS + '</style></head><body>' + SPRITE + \
       '<main class="page"><header class="top"><div><h1>Vit v5.1 · 图标概念（第二轮）</h1>' \
       '<p>已采用的四件不再选；下面只留还需要你拍板的：轨道波形疏密、A/B 新方案、bus 旧版 vs master、open 斜左简化、attach 旧版精修。' \
       '状态按你选的 B：待机虚线、进行中实线 3/4 弧，两者带旋转；完成/出错满环 + 标记。</p></div>' \
       '<div class="right">概念对照 / 2026-11-14<br>V 只做软件身份；状态已有旋转</div></header>' \
       '<h2 class="section">已采用</h2>' \
       '<p class="lead">工程沿用旧磁带，新建 = 旧磁带 + 右上角加号；V 用 A；effects 用 A。' \
       '这几件不再出变体，直接进最终 symbol。</p>' \
       '<div class="adopted">' + adopted + '</div>' \
       '<h2 class="section">还需要你选</h2>' \
       '<div class="choices">' + choices + '</div>' \
       '<h2 class="section">状态：方案 B（虚线 / 实线弧 / 满环）</h2>' \
       '<p class="lead">待机是虚线环，进行中是实线 3/4 弧；两者都旋转（待机慢、进行中快）。完成/出错是满环 + 中心标记，不转。' \
       '8px 身份点仍只保留颜色 + 文本；形状在 16px 以上出现。HTML 里可直接看旋转，PNG 是静帧。</p>' \
       '<div class="states">' + states + '</div>' \
       '<div class="notes"><div class="note2"><h3>这轮修正</h3>' \
       '待机回到虚线，进行中回到实线弧并保留旋转；新建改成旧磁带 + 右上角加号；轨道改成三 lane + 细波形；' \
       'AB 去掉两个旧案的容器感，回到“字母 + 黑白对比”；bus 优先旧版逻辑；open 去掉磁带边缘、左侧改斜角；attach 回到旧结构。</div>' \
       '<div class="note2"><h3>下一轮只剩收尾</h3>' \
       '你选完后，我把最终 symbol 画进 `round2-sprite.svg`，更新 S3 / S5 / P3，重跑审计；' \
       'A/B 的字母最终用几何笔画重画，不依赖字体；16px 逐个复核。</div></div>' \
       '<footer><span>概念第二轮 · 不进入产品屏</span><span>下一步：选轨道/AB/bus/open 后重画最终 symbol</span></footer>' \
       '</main></body></html>'

(ROOT / 'icon-concepts-v5-round2.html').write_text(html, encoding='utf-8')
print('wrote', ROOT / 'icon-concepts-v5-round2.html', len(html), 'bytes')
