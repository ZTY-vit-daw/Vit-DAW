"""Build the first v5.1 icon concept sheet: 10 redraw groups + the chosen state family B."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SPRITE = (ROOT / 'round2-sprite.svg').read_text(encoding='utf-8')

NI = 'class="ni" viewBox="0 0 24 24"'

def svg(inner, size, extra=''):
    return f'<svg {NI} style="width:{size}px;height:{size}px" aria-hidden="true">{inner}</svg>'

def old(name, size=24):
    return (f'<svg class="icon" style="width:{size}px;height:{size}px;--icon-bg:#FFFFFF" '
            f'aria-hidden="true"><use href="#i-{name}"/></svg>')

# ---------- concept drawings (24x24, flat geometry) ----------

PROJECT_A = '''
<rect x="3" y="6" width="18" height="12" fill="none" stroke="currentColor" stroke-width="2"/>
<rect x="6" y="9" width="12" height="6" fill="none" stroke="currentColor" stroke-width="1"/>
<circle cx="9" cy="12" r="2.2" fill="currentColor"/><circle cx="9" cy="12" r=".8" fill="var(--bg,#fff)"/>
<circle cx="15" cy="12" r="2.2" fill="currentColor"/><circle cx="15" cy="12" r=".8" fill="var(--bg,#fff)"/>
'''
PROJECT_B = '''
<rect x="3" y="6" width="18" height="12" fill="none" stroke="currentColor" stroke-width="2.5"/>
<rect x="6.5" y="9.5" width="11" height="5" fill="none" stroke="currentColor" stroke-width="1"/>
<circle cx="9" cy="12" r="2.4" fill="currentColor"/><circle cx="9" cy="12" r=".9" fill="var(--bg,#fff)"/>
<circle cx="15" cy="12" r="2.4" fill="currentColor"/><circle cx="15" cy="12" r=".9" fill="var(--bg,#fff)"/>
<path d="M5 17H19" stroke="currentColor" stroke-width="1"/>
'''

V_A = '''
<path d="M4 3L12 21L20 3H16L12 12.5L8 3Z" fill="currentColor"/>
<path d="M8.4 4.6L12 12.2L15.6 4.6" fill="none" stroke="var(--bg,#fff)" stroke-width="1"/>
'''
V_B = '''
<path d="M4 3L12 21L20 3" fill="none" stroke="currentColor" stroke-width="2.5"/>
<path d="M7.8 4.6L12 15L16.2 4.6" fill="none" stroke="currentColor" stroke-width="1"/>
'''

NEW_A = '''
<rect x="3" y="6" width="18" height="12" fill="none" stroke="currentColor" stroke-width="2"/>
<rect x="6" y="9" width="12" height="6" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M12 10.5V13.5M10.5 12H13.5" fill="none" stroke="currentColor" stroke-width="2"/>
'''
NEW_B = '''
<rect x="3" y="6" width="18" height="12" fill="none" stroke="currentColor" stroke-width="2"/>
<circle cx="8.5" cy="12" r="1.8" fill="currentColor"/><circle cx="15.5" cy="12" r="1.8" fill="currentColor"/>
<rect x="17" y="2.5" width="5" height="5" fill="currentColor"/>
<path d="M19.5 3.5V6.5M18 5H21" fill="none" stroke="var(--bg,#fff)" stroke-width="1.2"/>
'''

OPEN_A = '''
<path d="M3 6H9L11 8H21V19H3Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M3 10.5H21M3 10.5L6 19H21" fill="none" stroke="currentColor" stroke-width="1.5"/>
<rect x="9" y="13" width="6" height="3" fill="none" stroke="currentColor" stroke-width="1"/>
'''
OPEN_B = '''
<path d="M3 6H9L11 8H21V19H3Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M12 11V17M9 14L12 17L15 14" fill="none" stroke="currentColor" stroke-width="2"/>
'''

SEND_A = '''
<path d="M3 12L21 3L14 21L11 14L3 12Z" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M3 12L11 14" fill="none" stroke="currentColor" stroke-width="1.5"/>
'''
SEND_B = '''
<path d="M3 12H18" fill="none" stroke="currentColor" stroke-width="2.5"/>
<path d="M13 7L18 12L13 17" fill="none" stroke="currentColor" stroke-width="2.5"/>
'''

ATTACH_A = '''
<path d="M6 5V9M6 5H18M18 5V9" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M6 19V15M6 19H18M18 19V15" fill="none" stroke="currentColor" stroke-width="2"/>
<rect x="9" y="9" width="6" height="6" fill="currentColor"/>
'''
ATTACH_B = '''
<rect x="4" y="9" width="9" height="6" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M16 7V10M19 7V10M16 14V17M19 14V17M16 7H19M16 17H19" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M13 12H17" fill="none" stroke="currentColor" stroke-width="2"/>
'''

TRACK_A = '''
<path d="M3 8H21M3 16H21" fill="none" stroke="currentColor" stroke-width="1.5"/>
<rect x="5" y="9" width="10" height="6" fill="currentColor"/>
'''
TRACK_B = '''
<path d="M3 7H21M3 12H21M3 17H21" fill="none" stroke="currentColor" stroke-width="1.5"/>
<rect x="6" y="10" width="8" height="4" fill="currentColor"/>
'''

BUS_A = '''
<path d="M6 4V10L12 14M18 4V10L12 14" fill="none" stroke="currentColor" stroke-width="1.5"/>
<rect x="10" y="12" width="4" height="4" fill="currentColor"/>
<path d="M12 16V21" fill="none" stroke="currentColor" stroke-width="2.5"/>
'''
BUS_B = '''
<path d="M6 4V10M18 4V10" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M4 10H20" fill="none" stroke="currentColor" stroke-width="2.5"/>
<path d="M12 10V21" fill="none" stroke="currentColor" stroke-width="2"/>
'''

AB_A = '''
<rect x="3" y="5" width="8" height="14" fill="currentColor"/>
<rect x="13" y="5" width="8" height="14" fill="none" stroke="currentColor" stroke-width="2"/>
<text x="7" y="15.6" text-anchor="middle" font-size="9" font-weight="700" font-family="Segoe UI,sans-serif" fill="var(--bg,#fff)">A</text>
<text x="17" y="15.6" text-anchor="middle" font-size="9" font-weight="700" font-family="Segoe UI,sans-serif" fill="currentColor">B</text>
'''
AB_B = '''
<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M12 3A9 9 0 0 0 12 21Z" fill="currentColor"/>
<text x="7.6" y="15.2" text-anchor="middle" font-size="8" font-weight="700" font-family="Segoe UI,sans-serif" fill="var(--bg,#fff)">A</text>
<text x="16.4" y="15.2" text-anchor="middle" font-size="8" font-weight="700" font-family="Segoe UI,sans-serif" fill="currentColor">B</text>
'''

FX_A = '''
<rect x="3" y="4" width="18" height="16" fill="none" stroke="currentColor" stroke-width="2"/>
<circle cx="8.5" cy="10" r="2" fill="currentColor"/>
<circle cx="15.5" cy="10" r="2" fill="currentColor"/>
<path d="M7 15H17" fill="none" stroke="currentColor" stroke-width="2"/>
<path d="M5 4V2.5M19 4V2.5" fill="none" stroke="currentColor" stroke-width="1"/>
'''
FX_B = '''
<rect x="3" y="4" width="18" height="16" fill="none" stroke="currentColor" stroke-width="2"/>
<circle cx="12" cy="10" r="3.5" fill="none" stroke="currentColor" stroke-width="1.5"/>
<path d="M12 6.5V13.5M8.5 10H15.5" fill="none" stroke="currentColor" stroke-width="1"/>
<path d="M7 16H17" fill="none" stroke="currentColor" stroke-width="2"/>
'''

GROUPS = [
    ('工程 project', 'vit', '磁带从 i-vit 迁到 i-project；应用身份改用 V。', [
        ('A', '简化双盘：壳 2px、窗口 1px、实心盘 + 1px 孔', PROJECT_A, True),
        ('B', '重壳 2.5px、实心盘更大、底部标签线', PROJECT_B, False),
    ]),
    ('V 品牌标', None, '只用于 OS 表层：应用图标、任务栏 Vit 条目、通知；不在标题栏反复出现。', [
        ('A', '实心 V + 1px 内槽，保留桌面图标的倒角感', V_A, True),
        ('B', '双线 V：外 2.5px + 内 1px，更线框化', V_B, False),
    ]),
    ('新建工程 new', 'new', '磁带 + 加号，不再用通用文档框。', [
        ('A', '空白磁带 + 窗口内加号，直接读“新带”', NEW_A, True),
        ('B', '有盘磁带 + 右上角实心加号徽章', NEW_B, False),
    ]),
    ('打开文件夹 open', 'open', '打开的文件夹 + 一截磁带；开轮廓，与闭合磁带对比。', [
        ('A', '斜开前盖 + 磁带边缘，开轮廓', OPEN_A, True),
        ('B', '文件夹 + 下箭头，读作“导入”', OPEN_B, False),
    ]),
    ('发送 send', 'send', '不套方框；实心/粗头细尾，和 render 的向下箭头区分。', [
        ('A', '纸飞机，折线 1.5px', SEND_A, True),
        ('B', '粗头细尾箭头，最紧凑', SEND_B, False),
    ]),
    ('引用轨道/区间 attach', 'attach', '不是箭头；用区间括号夹住一段 clip。', [
        ('A', '上下区间括号 + 实心 clip', ATTACH_A, True),
        ('B', 'clip + 右侧链接括号，语义更“引用”', ATTACH_B, False),
    ]),
    ('轨道 track', 'track', 'DAW 的一条 lane，不是通用卡片。', [
        ('A', '单条 lane + 实心 clip，16px 最清楚', TRACK_A, True),
        ('B', '三条 lane + 一条 clip，读作“轨道列表”', TRACK_B, False),
    ]),
    ('总线/编组 bus', 'bus', '工程总线，不是对话分支；汇入线由细到粗。', [
        ('A', '两路 1.5px 汇入 + 2.5px 输出 + 节点', BUS_A, True),
        ('B', '汇入一条粗母线，再单路输出', BUS_B, False),
    ]),
    ('对比 A/B', 'ab', '字母入图，一黑一白；16px 可退化为两块对比。', [
        ('A', '两个并置方块，A 反白 / B 墨色', AB_A, True),
        ('B', '分割圆，左右黑白各半', AB_B, False),
    ]),
    ('效果器 fx', 'fx', '方形机架机器人头：旋钮眼 + 插槽嘴；与磁带横向比例区分。', [
        ('A', '机器人头：双旋钮 + 插槽 + 机架耳', FX_A, True),
        ('B', '单旋钮 + 插槽的机架模块，更“设备”不更“脸”', FX_B, False),
    ]),
]

def variant_block(label, desc, inner, recommended):
    sizes = ''.join(f'<span class="sz">{svg(inner, s)}<b>{s}</b></span>' for s in (32, 24, 16))
    tag = '<span class="tag">推荐</span>' if recommended else ''
    return (f'<div class="variant"><h4>{label} {tag}</h4><div class="sizes">{sizes}</div>'
            f'<p>{desc}</p></div>')

def group_card(name, old_id, note, variants):
    old_html = f'<span class="old">{old(old_id)}<b>旧 i-{old_id}</b></span>' if old_id else '<span class="old"><b>旧：磁带 i-vit</b></span>'
    blocks = ''.join(variant_block(*v) for v in variants)
    return (f'<article class="group"><header><h3>{name}</h3>{old_html}</header>'
            f'<p class="note">{note}</p><div class="variants">{blocks}</div></article>')

# ---------- states (B chosen) ----------

def state_icon(kind, size):
    base = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/>'
    if kind == 'idle':
        inner = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="42 12" transform="rotate(-45 12 12)"/>'
    elif kind == 'busy':
        inner = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="42 14" transform="rotate(-90 12 12)"/>'
    elif kind == 'done':
        inner = base + '<path d="M7.5 12.5 10.5 15.5 16.5 8.5" fill="none" stroke="currentColor" stroke-width="2"/>'
    else:
        inner = base + '<path d="M12 7.5V13.5M12 16.5V17.5" fill="none" stroke="currentColor" stroke-width="2"/>'
    return svg(inner, size)

STATE_ROWS = [
    ('idle', '待机', '环留一个开口；已连接，没有回合在跑。'),
    ('busy', '进行中', '3/4 弧；回合/工具执行中，动效旋转。'),
    ('done', '完成', '满环 + 勾；步骤收成回执。'),
    ('error', '出错', '满环 + 感叹；原因与重试。'),
]

def state_section():
    cards = ''
    for kind, name, desc in STATE_ROWS:
        dots = f'<span class="dot {kind}"></span>'
        cards += (f'<div class="statecard"><h4>{name}</h4><div class="statering">'
                  + ''.join(f'<span class="sz">{state_icon(kind, s)}<b>{s}</b></span>' for s in (16, 24, 32))
                  + f'</div><div class="statedot">{dots}<span>8px 身份点</span></div><p>{desc}</p></div>')
    return cards

CSS = r'''
:root{--ink:#141414;--muted:#555555;--weak:#666666;--line:#D3D3D3;--rule:#E2E2E2;--yellow:#FACC1A;--blue:#065BC7;--red:#DB1214;--sans:Inter,"Segoe UI","Microsoft YaHei","PingFang SC",sans-serif;--mono:Consolas,monospace}
*{box-sizing:border-box}html,body{margin:0;width:1440px;background:#E4E4E4;color:var(--ink);font:16px/1.65 var(--sans)}
h1,h2,h3,h4,p{margin:0}
.page{width:1440px;padding:44px 56px 56px;background:#FFFFFF}
header.top{display:flex;justify-content:space-between;align-items:end;gap:32px;padding-bottom:24px;border-bottom:2px solid var(--ink)}
header.top h1{font-size:28px;font-weight:700;line-height:1.3}
header.top p{font-size:14px;color:var(--muted);line-height:1.55;margin-top:10px;max-width:66em}
header.top .right{font-size:13px;color:var(--weak);text-align:right;white-space:nowrap;line-height:1.6}
h2.section{font-size:20px;margin:32px 0 14px}
.lead{font-size:14px;color:var(--muted);line-height:1.6;max-width:74em;margin-bottom:16px}
.icon,.ni{display:inline-block;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:butt;stroke-linejoin:miter;overflow:visible;vertical-align:middle}
.solid{fill:currentColor;stroke:none}.selected-fill{fill:var(--icon-fill,none)}.knockout{fill:var(--icon-bg,#fff);stroke:none}.detail{stroke:var(--icon-mark,currentColor)}.alert-mark{fill:var(--icon-mark,currentColor);stroke:none}
.sprite{position:absolute;width:0;height:0;overflow:hidden}
.groups{display:grid;grid-template-columns:1fr 1fr;gap:20px}
.group{border:1px solid var(--line);padding:18px;background:#FFFFFF}
.group header{display:flex;justify-content:space-between;align-items:center;gap:16px;padding-bottom:10px;border-bottom:1px solid var(--rule)}
.group h3{font-size:17px;font-weight:600}
.old{display:flex;align-items:center;gap:8px;font:11px/1.3 var(--mono);color:var(--weak)}
.note{font-size:13px;color:var(--muted);line-height:1.55;margin:10px 0 12px;min-height:40px}
.variants{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.variant{border:1px solid var(--rule);padding:12px;background:#FAFAFA}
.variant h4{font-size:14px;font-weight:600;display:flex;align-items:center;gap:8px}
.variant p{font-size:12px;color:var(--muted);line-height:1.5;margin-top:8px;min-height:34px}
.tag{font:10px/1 var(--mono);background:var(--ink);color:#FFFFFF;padding:3px 5px;font-weight:400}
.sizes,.statering{display:flex;align-items:end;gap:12px;margin:10px 0 0}
.sz{display:flex;flex-direction:column;align-items:center;gap:4px}
.sz b{font:10px/1.2 var(--mono);font-weight:400;color:var(--weak)}
.states{display:grid;grid-template-columns:repeat(4,1fr);gap:16px}
.statecard{border:1px solid var(--line);padding:16px;background:#FFFFFF}
.statecard h4{font-size:16px;font-weight:600;margin-bottom:8px}
.statecard p{font-size:12px;color:var(--muted);line-height:1.55;margin-top:8px}
.statedot{display:flex;align-items:center;gap:8px;font-size:12px;color:var(--muted);margin-top:10px}
.dot{width:8px;height:8px;border-radius:50%;border:1px solid rgba(0,0,0,.35);display:inline-block;flex:none}
.dot.idle{background:#888888}.dot.busy{background:var(--yellow)}.dot.done{background:var(--blue)}.dot.error{background:var(--red)}
.table{width:100%;border-collapse:collapse;font-size:14px;margin-top:12px}
.table th,.table td{border-bottom:1px solid var(--rule);padding:10px 12px;text-align:left;vertical-align:middle}
.table th{font-size:13px;font-weight:400;color:var(--muted)}
.table td:first-child{width:150px;color:var(--muted);font-size:13px}
.notes{display:grid;grid-template-columns:1fr 1fr;gap:20px;margin-top:24px}
.note2{border:1px solid var(--line);padding:16px 18px;background:#FAFAFA;font-size:13px;line-height:1.6;color:#333}
.note2 h3{font-size:15px;margin-bottom:6px;color:var(--ink)}
footer{margin-top:28px;border-top:2px solid var(--ink);padding-top:14px;display:flex;justify-content:space-between;gap:24px;font-size:13px;color:var(--weak)}
'''.strip()

groups = ''.join(group_card(*g) for g in GROUPS)

usage_rows = [
    ('project 磁带', 'P1 工程卡/抽屉 · P3 标题栏 · P4 任务栏工程条目', '工程对象，不是应用 logo'),
    ('V', '应用图标 · P4 任务栏 Vit 条目 · 通知', '只做软件身份，不反复出现在标题栏'),
    ('vit 旧磁带', '不再使用；由 project 接手', '—'),
    ('new / open', 'P1 抽屉', '工程动作'),
    ('send / stop', '输入框', '对话动作'),
    ('attach', 'P3 上下文目标', '引用轨道/区间'),
    ('track', 'DAW 轨道行', '一条 lane；任务栏工程条目改用 project'),
    ('bus', 'P3 上下文目标 · 宏标题', '工程总线/编组；对话分支另设 i-branch'),
    ('fx', 'DAW 机架', '机架上的效果器'),
    ('macro', '宏标题/应用', '保持现有推子组，不重画'),
    ('ab', '对比按钮', 'A/B 试听'),
    ('idle/busy/done/error', '标题栏身份点 · 轨迹 · 状态卡', '运行态；8px 点只保留颜色'),
]
usage = ''.join(f'<tr><td>{a}</td><td>{b}</td><td>{c}</td></tr>' for a,b,c in usage_rows)

html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' \
       '<title>Vit v5.1 图标概念 · 第一轮</title><style>' + CSS + '</style></head><body>' + SPRITE + \
       '<main class="page"><header class="top"><div><h1>Vit v5.1 · 图标概念（第一轮）</h1>' \
       '<p>10 组重画 + 已选的状态方案 B。每组给 A / B 两个方案，32 / 24 / 16px 并排；旧图标放在右上角对照。' \
       '这一页只讨论形与职能，不进入产品屏；选定后只改 SVG symbol、S3 / S5 / P3 与雪碧图。</p></div>' \
       '<div class="right">概念对照 / 2026-11-14<br>V 只做软件身份；状态采用环的完整度</div></header>' \
       '<h2 class="section">重画组</h2>' \
       '<p class="lead">统一的线重分层：结构 2 / 2.5px，细节 1.5px，引导 1px；物理质量用实心，选中/激活用反白或实心；' \
       '开轮廓与曲线只给需要它们的对象（文件夹、区间括号、磁带盘、旋钮）。</p>' \
       '<div class="groups">' + groups + '</div>' \
       '<h2 class="section">状态：方案 B（已选）</h2>' \
       '<p class="lead">同一圆环，用完整度区分：待机留开口、进行中 3/4 弧、完成满环+勾、出错满环+感叹。' \
       '进行中在产品里让弧旋转；静帧用 3/4 弧 + 文字。8px 身份点只保留颜色，形状在 16px 以上出现。</p>' \
       '<div class="states">' + state_section() + '</div>' \
       '<h2 class="section">用法与出现位置</h2>' \
       '<table class="table"><thead><tr><th>图标</th><th>出现位置</th><th>职能</th></tr></thead><tbody>' + usage + '</tbody></table>' \
       '<div class="notes"><div class="note2"><h3>这轮改动的原因</h3>' \
       '旧版全家族都是 2px 直角闭合框，密度一样、明暗一样，所以看起来像同一个模子；macro 是例外，因为它有实心推子帽和节奏。' \
       '新版把线重、实心/空心、曲线/直角分开用，并让每枚图标说出它实际代表的对象或动作。</div>' \
       '<div class="note2"><h3>还没进这一轮</h3>' \
       'render、snapshot、undo、play、record、stop、minimize、alert 保持现有造型，下一轮顺手统一；' \
       'A/B 字母在最终 symbol 里会用几何笔画重画，不依赖系统字体。</div></div>' \
       '<footer><span>概念第一轮 · 不进入产品屏</span><span>下一步：每组选 A/B → 重画最终 symbol → S3/S5/P3 重建 + 审计</span></footer>' \
       '</main></body></html>'

(ROOT / 'icon-concepts-v5.html').write_text(html, encoding='utf-8')
print('wrote', ROOT / 'icon-concepts-v5.html', len(html), 'bytes')
