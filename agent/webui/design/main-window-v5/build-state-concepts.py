"""Build a focused concept sheet for the four Vit runtime states (discussion artifact)."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent

def ring(state, size, variant='A'):
    base = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2"/>'
    if variant == 'A':
        extra = {
            'idle': '',
            'busy': '<circle cx="12" cy="12" r="2.6" fill="currentColor"/>',
            'done': '<path d="M7.5 12.5 10.5 15.5 16.5 8.5" fill="none" stroke="currentColor" stroke-width="2"/>',
            'error': '<path d="M12 7.5V13.5M12 16.5V17.5" fill="none" stroke="currentColor" stroke-width="2"/>',
        }[state]
    else:
        if state == 'idle':
            extra = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="3.4 3.4" transform="rotate(-45 12 12)"/>'
            base = ''
        elif state == 'busy':
            extra = '<circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="42 14" transform="rotate(-90 12 12)"/>'
            base = ''
        elif state == 'done':
            extra = '<path d="M7.5 12.5 10.5 15.5 16.5 8.5" fill="none" stroke="currentColor" stroke-width="2"/>'
        else:
            extra = '<path d="M12 7.5V13.5M12 16.5V17.5" fill="none" stroke="currentColor" stroke-width="2"/>'
    return (f'<svg class="ring" viewBox="0 0 24 24" style="width:{size}px;height:{size}px" '
            f'aria-hidden="true">{base}{extra}</svg>')

STATES = [
    ('idle', '待机', '已连接，没有回合在跑；当前步骤未开始。'),
    ('busy', '进行中', '一个回合或工具正在执行；当前步骤在动。'),
    ('done', '完成', '步骤或回合结束；收成一行回执，可展开轨迹。'),
    ('error', '出错', '写明原因与恢复动作；不把失败画成普通完成。'),
]

CSS = r'''
:root{--ink:#141414;--muted:#555555;--weak:#666666;--line:#D3D3D3;--rule:#E2E2E2;
--workspace:#F0F1F2;--dark:#14181C;--paper:#F6F4EC;--yellow:#FACC1A;--blue:#065BC7;--red:#DB1214;
--sans:Inter,"Segoe UI","Microsoft YaHei","PingFang SC",sans-serif;--mono:Consolas,monospace}
*{box-sizing:border-box}html,body{margin:0;width:1440px;background:#E4E4E4;color:var(--ink);font:16px/1.65 var(--sans)}
h1,h2,h3,p{margin:0}
.page{width:1440px;padding:44px 56px 56px;background:#FFFFFF}
header.top{display:flex;justify-content:space-between;align-items:end;gap:32px;padding-bottom:24px;border-bottom:2px solid var(--ink)}
header.top h1{font-size:28px;font-weight:700;line-height:1.3}
header.top p{font-size:14px;color:var(--muted);line-height:1.55;margin-top:10px;max-width:64em}
header.top .right{font-size:13px;color:var(--weak);text-align:right;white-space:nowrap;line-height:1.6}
h2.section{font-size:20px;margin:32px 0 16px}
.lead{font-size:14px;color:var(--muted);line-height:1.6;max-width:72em;margin-bottom:16px}
.grid{display:grid;grid-template-columns:repeat(4,1fr);gap:16px}
.card{border:1px solid var(--line);padding:18px;background:#FFFFFF;display:flex;flex-direction:column;gap:14px}
.card h3{font-size:18px;font-weight:600;line-height:1.3}
.card .scene{font-size:13px;color:var(--muted);line-height:1.55;min-height:44px}
.dotrow{display:flex;align-items:center;gap:16px;padding:12px;background:var(--dark);color:var(--paper)}
.dotrow.light{background:#FFFFFF;color:var(--ink);border:1px solid var(--rule)}
.dot{display:inline-block;width:8px;height:8px;border-radius:50%;border:1px solid rgba(40,40,40,.45)}
.dot.idle{background:#888888}.dot.busy{background:var(--yellow)}.dot.done{background:var(--blue)}.dot.error{background:var(--red)}
.dotrow .caption{font:11px/1.3 var(--mono);opacity:.75}
.rings{display:flex;align-items:end;gap:18px;padding:4px 0}
.ring{display:block;flex:none}
.ring.idle{color:#777777}.ring.busy{color:var(--ink)}.ring.done{color:var(--ink)}.ring.error{color:var(--ink)}
.ringrow{display:flex;flex-direction:column;align-items:center;gap:4px}
.ringrow b{font:10px/1.2 var(--mono);font-weight:400;color:var(--weak)}
.table{width:100%;border-collapse:collapse;font-size:14px}
.table th,.table td{border-bottom:1px solid var(--rule);padding:14px 12px;vertical-align:middle;text-align:left}
.table th{font-size:13px;font-weight:400;color:var(--muted)}
.table td:first-child{width:170px;color:var(--muted);font-size:13px}
.table .statecell{display:flex;align-items:center;gap:12px}
.table .statecell .name{font-size:14px}
.context{border:1px solid var(--rule);background:var(--workspace);padding:20px;display:flex;flex-direction:column;gap:14px}
.titlebar{height:48px;background:#FFFFFF;border-bottom:1px solid #D9DDE1;display:flex;align-items:center;gap:10px;padding:0 16px;box-shadow:inset 0 1px 0 #FFFFFF}
.brand{width:26px;height:26px;display:grid;place-items:center;border:2px solid currentColor;font:700 16px/1 var(--mono)}
.titlebar b{font-size:16px}.titlebar .title{margin-left:6px;font-size:15px}
.trace{display:flex;align-items:center;gap:10px;background:#FFFFFF;border:1px solid var(--rule);padding:10px 14px;font-size:14px}
.trace time{margin-left:auto;color:var(--weak);font-size:13px}
.statuscard{background:#FFFFFF;border:1px solid var(--rule);padding:14px 16px;display:flex;gap:12px;align-items:flex-start}
.statuscard b{font-size:16px;font-weight:600}.statuscard p{font-size:13px;color:var(--muted);margin-top:4px}
.notes{display:grid;grid-template-columns:1fr 1fr;gap:24px;margin-top:28px}
.note{border:1px solid var(--line);padding:18px 20px;background:#FAFAFA}
.note h3{font-size:16px;margin-bottom:8px}.note p,.note li{font-size:14px;line-height:1.6;color:#333333}
.note ul{margin:0;padding-left:20px}
.recommend{background:#F8F1D3;border:1px solid #8E782F;padding:18px 20px;margin-top:24px;font-size:14px;line-height:1.65}
footer{margin-top:32px;border-top:2px solid var(--ink);padding-top:16px;display:flex;justify-content:space-between;gap:32px;font-size:13px;color:var(--weak)}
'''.strip()

def state_card(state, name, scene):
    dots = (f'<div class="dotrow"><span class="dot {state}"></span><span class="caption">8px 身份点 · 深底</span></div>'
            f'<div class="dotrow light"><span class="dot {state}"></span><span class="caption">8px 身份点 · 浅底</span></div>')
    rings = ''.join(f'<span class="ringrow">{ring(state, s)}<b>{s}px</b></span>' for s in (16, 24, 32))
    return f'''<article class="card"><h3>{name}</h3><p class="scene">{scene}</p>{dots}
    <div class="rings">{rings}</div><span class="caption" style="font-size:12px;color:#666">方案 A：同一环 + 中心标记</span></article>'''

def state_card_b(state, name):
    rings = ''.join(f'<span class="ringrow">{ring(state, s, "B")}<b>{s}px</b></span>' for s in (16, 24, 32))
    return f'<article class="card"><h3>{name}</h3><div class="rings">{rings}</div><span class="caption" style="font-size:12px;color:#666">方案 B：用环的完整度区分</span></article>'

cards_a = ''.join(state_card(*s) for s in STATES)
cards_b = ''.join(state_card_b(state, name) for state, name, _ in STATES)

context_rows = ''
for state, name, _ in STATES:
    context_rows += f'''<tr><td>{name}</td><td><div class="statecell">{ring(state,16)}<span class="name">16px 轨迹 / 步骤</span></div></td>
    <td><div class="statecell">{ring(state,24)}<span class="name">24px 状态卡 / 强调</span></div></td>
    <td><div class="statecell"><span class="dot {state}"></span><span class="name">8px 标题栏身份点</span></div></td></tr>'''

html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">' \
       '<title>Vit 状态表达 · 概念对照</title><style>' + CSS + '</style></head><body><main class="page">' \
       '<header class="top"><div><h1>状态表达 · 四态怎么显示</h1>' \
       '<p>这里只回答一件事：待机 / 进行中 / 完成 / 出错四个“运行态”在界面里怎么出现。它们不是按钮，也不是四个用户动作；' \
       '用户能点的只有发送、停止、对比、撤销这些动作，状态只负责告诉用户“现在发生了什么”。</p></div>' \
       '<div class="right">概念对照 / 2026-11-14<br>不进入产品屏，先定方向</div></header>' \
       '<h2 class="section">方案 A · 同一基座，四种中心标记（备选，未采用）</h2>' \
       '<p class="lead">四种状态共用同一个 2px 圆环；区别只在环内：空 = 待机，实心点 = 进行中，勾 = 完成，感叹 = 出错。' \
       '环保证“同一族”，中心标记保证“黑白也能区分”。8px 时环和标记都看不清，所以身份点只保留颜色，文字/提示负责说明。</p>' \
       '<div class="grid">' + cards_a + '</div>' \
       '<h2 class="section">方案 B · 用环的完整度区分（已选）</h2>' \
       '<p class="lead">同一套环，待机 = 虚线环，进行中 = 实线 3/4 弧，完成 = 满环 + 勾，出错 = 满环 + 感叹。' \
       '待机与进行中都带旋转（待机慢、进行中快）；静帧用虚线环 / 3/4 弧 + 文字。8px 身份点只保留颜色，形状在 16px 以上出现。</p>' \
       '<div class="grid">' + cards_b + '</div>' \
       '<h2 class="section">在界面里的样子</h2>' \
       '<div class="context"><div class="titlebar"><span class="tape-mark">' \
       + '<svg viewBox="0 0 24 24" style="width:24px;height:24px" fill="none" stroke="currentColor" stroke-width="2">' \
       '<rect x="3" y="6" width="18" height="12"/><rect x="6" y="9" width="12" height="6" stroke-width="1"/>' \
       '<circle cx="9" cy="12" r="2" fill="currentColor"/><circle cx="15" cy="12" r="2" fill="currentColor"/></svg>' \
       '</span><b>鼓组混音 v2</b><span class="dot busy"></span></div>' \
       '<div class="trace">' + ring('busy',16) + '<span>正在渲染对比 A/B</span><time>0.8 秒</time></div>' \
       '<div class="trace">' + ring('done',16) + '<span>已读取鼓组总线</span><time>0.2 秒</time></div>' \
       '<div class="statuscard">' + ring('error',24) + '<div><b>无法打开《鼓组混音 v2》</b><p>音频设备已断开，工程尚未打开。可重试。</p></div></div></div>' \
       '<table class="table" style="margin-top:16px"><thead><tr><th>状态</th><th>轨迹 / 步骤</th><th>状态卡 / 强调</th><th>标题栏身份点</th></tr></thead><tbody>' + context_rows + '</tbody></table>' \
       '<div class="notes"><div class="note"><h3>为什么不再画四枚具象图标</h3><ul>' \
       '<li>四枚各自造形，16px 要重新学一遍；用户问“待机和进行中是什么场景”就是信号。</li>' \
       '<li>四枚图形在 8px 标题栏根本读不出，最后仍靠颜色/文字。</li>' \
       '<li>同一环 + 完整度，既是一家人，也能黑白投影区分。</li></ul></div>' \
       '<div class="note"><h3>状态只回答“现在发生什么”</h3><ul>' \
       '<li>待机：已连接，没有回合在跑；步骤未开始。</li>' \
       '<li>进行中：一个回合/工具在执行；当前步骤在动。</li>' \
       '<li>完成：步骤或回合结束，收成一行回执。</li>' \
       '<li>出错：写明原因与恢复动作。</li></ul></div></div>' \
       '<div class="recommend"><strong>已选 B：</strong>待机用虚线环（慢转）、进行中用实线 3/4 弧（快转）、完成/出错用满环 + 中心标记。' \
       '8px 身份点只保留颜色 + 文本/提示；16px 以上才出现环与完整度。下一步按这个细化 S2 / S5 的样张。</div>' \
       '<footer><span>概念对照 · 不进入产品屏</span><span>下一步：按 B 细化 S2/S5；图标逐组选 A/B 后重画 symbol</span></footer>' \
       '</main></body></html>'

(ROOT / 'state-concepts-v5.html').write_text(html, encoding='utf-8')
print('wrote', ROOT / 'state-concepts-v5.html', len(html), 'bytes')
