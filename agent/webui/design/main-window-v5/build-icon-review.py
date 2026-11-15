"""Build the v5.1 icon review sheet for user sign-off before implementation cards."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SPRITE = (ROOT / 'round2-sprite.svg').read_text(encoding='utf-8')

ICONS = [
    ('new', '新建工程', '工程', 'P1 工程抽屉 / 启动页动作', '与 i-open 同为方框+折角结构，16px 需对照'),
    ('open', '打开文件夹', '工程', 'P1 工程抽屉', '与 i-new 同族；区分靠上方开口与文件页'),
    ('minimize', '最小化', '窗口', '对话窗标题栏', '单横线，16px 可能被读成普通分隔线'),
    ('send', '发送', '输入', '输入框主按钮', '箭头形，与 i-attach / i-render 的箭头语汇需区分'),
    ('stop', '停止执行', '输入', '输入框执行中', '两侧括号 + 方心；与 i-idle 的角标不同族'),
    ('track', '轨道', '控件', 'DAW 轨道行 / 任务栏', '左侧实心条与轨道名并置，16px 需看'),
    ('bus', '总线 / 编组', '控件', '工程上下文目标 / 宏标题', '两个方块+汇入线；与 i-track 的矩形语言接近'),
    ('fx', '效果器', '控件', '机架', '方框内插接图形；16px 内部可能糊'),
    ('macro', '宏', '控件', '宏标题 / 应用按钮', '三条竖线+三个方块，信息密度最高的一枚'),
    ('ab', '对比 A/B', '控件', '对比按钮', '双矩形+交换箭头；16px 可能过密'),
    ('play', '播放', '走带', '建议动作 / 走带', '三角形，小尺寸稳定'),
    ('record', '录音', '走带', '走带（红）', '实心圆；颜色不是唯一编码'),
    ('render', '渲染 / 导出', '走带', '动作 / 通知', '向下箭头+托盘，与 i-send 的箭头方向不同'),
    ('undo', '撤销', '编辑', '通知 / 快照回执', '回转箭头；与 i-render 的箭头需对照'),
    ('snapshot', '快照', '编辑', '建议动作 / 回执', '四角框+内框；16px 可能像“选择框”'),
    ('attach', '引用轨道 / 区间', '编辑', '工程上下文目标', '括号+向右箭头；与 i-open 的开口框需对照'),
    ('idle', '待机', '状态', 'Vit 状态', '两端角标+中心方点，状态族基准'),
    ('busy', '进行中', '状态', 'Vit 状态 / 当前步骤', '阶梯+当前点，与 i-idle / i-done 同族'),
    ('done', '完成', '状态', '轨迹 / 回执', '折线+角标，与 i-idle / i-busy 同族'),
    ('alert', '出错', '状态', '失败 / 通知', '菱形+感叹；与 i-stop 的方框语义分开'),
    ('vit', 'Vit 标志', '品牌', '标题栏 / 任务栏 / 通知', '磁带壳体+双盘；16px 需看 knockout 是否糊'),
]

CSS = r'''
:root{
  --ink:#141414;--paper:#FFFFFF;--workspace:#F0F1F2;--muted:#555555;--weak:#666666;
  --blue:#31567D;--yellow:#F6E7AA;--dark:#14181C;--line:#D3D3D3;--rule:#E2E2E2;
  --sans:Inter,"Segoe UI","Microsoft YaHei","PingFang SC",sans-serif;
  --mono:Consolas,monospace;
}
*{box-sizing:border-box}
html,body{margin:0;width:1440px;background:#E4E4E4;color:var(--ink);font:16px/1.6 var(--sans)}
h1,h2,h3,p{margin:0}
.page{width:1440px;padding:48px 56px 64px;background:#FFFFFF}
header.top{display:flex;justify-content:space-between;align-items:end;gap:32px;padding-bottom:28px;border-bottom:2px solid var(--ink)}
header.top h1{font-size:28px;font-weight:700;line-height:1.3}
header.top p{font-size:14px;color:var(--muted);line-height:1.55;margin-top:10px;max-width:62em}
header.top .right{font-size:13px;color:var(--weak);white-space:nowrap;text-align:right;line-height:1.6}
.brief{display:grid;grid-template-columns:1fr 1fr 1fr;gap:24px;margin:28px 0 40px}
.brief section{border:1px solid var(--line);padding:20px 24px;background:#FAFAFA}
.brief h2{font-size:16px;margin-bottom:10px}
.brief p,.brief li{font-size:14px;line-height:1.6;color:#333333}
.brief ul{margin:0;padding-left:20px}
.sprite{position:absolute;width:0;height:0;overflow:hidden}
.icon{display:inline-block;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:butt;stroke-linejoin:miter;overflow:visible;vertical-align:middle}
.solid{fill:currentColor;stroke:none}
.selected-fill{fill:var(--icon-fill,none)}
.knockout{fill:var(--icon-bg,#FFFFFF);stroke:none}
.detail{stroke:var(--icon-mark,currentColor)}
.alert-mark{fill:var(--icon-mark,currentColor);stroke:none}
.is-selected{--icon-fill:currentColor;--icon-mark:var(--icon-bg,#F6E7AA)}
.family{margin-bottom:40px}
.family h2{font-size:20px;margin-bottom:16px}
.family-row{display:flex;flex-wrap:wrap;gap:10px;padding:18px 20px;border:1px solid var(--line);background:#FAFAFA;margin-bottom:12px}
.family-row.dark{background:var(--dark);color:#F6F4EC;border-color:#2B3037;--icon-bg:var(--dark)}
.family-row .cell{display:flex;flex-direction:column;align-items:center;gap:6px;min-width:64px}
.family-row .cell b{font:10px/1.2 var(--mono);font-weight:400;opacity:.72;letter-spacing:0}
.family-row .cell .icon{color:inherit}
.family-row .cell.dark{--icon-bg:var(--dark)}
.grid{display:grid;grid-template-columns:1fr 1fr;gap:24px}
.card{border:1px solid var(--line);background:#FFFFFF;padding:20px;display:flex;flex-direction:column;gap:16px;break-inside:avoid}
.card-head{display:flex;align-items:center;gap:20px}
.hero{width:104px;height:104px;flex:none;display:grid;place-items:center;background:var(--paper);border:1px solid var(--rule);--icon-bg:var(--paper)}
.hero .icon{color:var(--ink)}
.idline{min-width:0}
.idline code{display:inline-block;font:13px/1.4 var(--mono);background:#F0F0F0;border:1px solid var(--rule);padding:2px 8px;margin-bottom:8px}
.idline h3{font-size:19px;line-height:1.3;font-weight:600}
.idline .role{font-size:13px;color:var(--weak);margin-top:4px}
.idline .use{font-size:14px;color:#444444;margin-top:6px}
.sample-row{display:flex;align-items:end;gap:18px;flex-wrap:wrap;padding-top:2px}
.sample{display:flex;flex-direction:column;align-items:center;gap:6px;color:var(--ink);--icon-bg:#FFFFFF}
.sample b{font:11px/1.2 var(--mono);font-weight:400;color:var(--weak)}
.surface-row{display:flex;align-items:stretch;gap:8px}
.surface{flex:1;min-width:0;display:flex;flex-direction:column;align-items:center;gap:6px;padding:10px 4px;border:1px solid var(--rule)}
.surface b{font:10px/1.2 var(--mono);font-weight:400;opacity:.78}
.surface.light{background:#FFFFFF;color:var(--ink);--icon-bg:#FFFFFF}
.surface.workspace{background:var(--workspace);color:var(--ink);--icon-bg:var(--workspace)}
.surface.blue{background:var(--blue);color:#FFFFFF;--icon-bg:var(--blue);border-color:var(--blue)}
.surface.yellow{background:var(--yellow);color:var(--ink);--icon-bg:var(--yellow);border-color:#CBB96D}
.surface.dark{background:var(--dark);color:#F6F4EC;--icon-bg:var(--dark);border-color:#2B3037}
.risk{font-size:13px;line-height:1.55;color:#555555;border-top:1px dashed var(--rule);padding-top:12px}
.risk strong{color:var(--ink);font-weight:600}
.feedback{display:flex;align-items:center;gap:12px;font-size:13px;color:var(--muted);border-top:1px solid var(--rule);padding-top:12px}
.feedback span.box{display:inline-block;width:13px;height:13px;border:1px solid #777777;margin-right:4px;vertical-align:-2px}
.feedback .line{flex:1;border-bottom:1px solid #CCCCCC;height:1px}
footer{margin-top:40px;border-top:2px solid var(--ink);padding-top:20px;display:flex;justify-content:space-between;gap:32px}
footer p{font-size:14px;line-height:1.6;color:#333333;max-width:56em}
footer code{font:13px/1.4 var(--mono);background:#F0F0F0;border:1px solid var(--rule);padding:1px 6px}
'''.strip()

def icon(name, size, extra='', bg='#FFFFFF'):
    return (f'<svg class="icon {extra}" style="width:{size}px;height:{size}px;--icon-bg:{bg}" '
            f'aria-hidden="true"><use href="#i-{name}"/></svg>')

def family_row(icons, size, dark=False, bg='#FFFFFF'):
    cells = ''.join(
        f'<span class="cell {"dark" if dark else ""}">{icon(name, size, bg=bg)}<b>{name}</b></span>'
        for name, _, _, _, _ in icons)
    return f'<div class="family-row {"dark" if dark else ""}" style="--icon-bg:{bg}">{cells}</div>'

def card(meta):
    name, label, group, usage, risk = meta
    sizes = ''.join(
        f'<span class="sample">{icon(name, s)}<b>{s}px</b></span>' for s in (48, 32, 24, 20, 16))
    surfaces = ''.join([
        f'<span class="surface light">{icon(name, 24, bg="#FFFFFF")}<b>浅底</b></span>',
        f'<span class="surface workspace">{icon(name, 24, bg="#F0F1F2")}<b>工作区</b></span>',
        f'<span class="surface blue">{icon(name, 24, bg="#31567D")}<b>蓝底</b></span>',
        f'<span class="surface yellow">{icon(name, 24, "is-selected", bg="#F6E7AA")}<b>选中</b></span>',
        f'<span class="surface dark">{icon(name, 24, bg="#14181C")}<b>深底</b></span>',
    ])
    return f'''<article class="card">
  <div class="card-head">
    <div class="hero">{icon(name, 64)}</div>
    <div class="idline"><code>i-{name}</code><h3>{label}</h3><p class="role">{group}</p><p class="use">{usage}</p></div>
  </div>
  <div class="sample-row">{sizes}</div>
  <div class="surface-row">{surfaces}</div>
  <p class="risk"><strong>小尺寸观察：</strong>{risk}</p>
  <div class="feedback"><span class="box"></span>保留 <span class="box"></span>修改 <span class="box"></span>替换 <span class="line"></span></div>
</article>'''

brief = '''<div class="brief">
<section><h2>怎么看</h2><p>先看 16px 的轮廓能不能认出，再看 24 / 32px 的细节；每个样本同时给了浅底、工作区、深蓝标题、浅黄选中和深底。</p></section>
<section><h2>判据</h2><ul><li>形状优先，不靠颜色区分；</li><li>同一族（工程 / 走带 / 状态）要像一家人；</li><li>选中态填充后仍能读出主体。</li></ul></section>
<section><h2>怎么反馈</h2><p>按 ID 说“保留 / 修改 / 替换”，附一句原因即可，例如：<code>i-open 修改</code>，<code>i-macro 替换</code>。</p></section>
</div>'''

family = f'''<section class="family">
<h2>家族总览</h2>
{family_row(ICONS, 32)}
{family_row(ICONS, 16)}
{family_row(ICONS, 24, dark=True, bg='#14181C')}
</section>'''

cards = '\n'.join(card(meta) for meta in ICONS)

html = '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">' \
       '<meta name="viewport" content="width=device-width,initial-scale=1">' \
       '<title>Vit v5.1 图标复审</title><style>' + CSS + '</style></head><body>' + SPRITE + \
       '<main class="page"><header class="top"><div><h1>Vit v5.1 · 图标复审</h1>' \
       '<p>20 枚功能图标 + i-vit。所有图标同一套 24×24 网格、20×20 活动区、2px 描边。' \
       '这一页只用于逐枚审阅，不进入产品屏。</p></div>' \
       '<div class="right">v5.1 / 2026-11-14<br>共 21 枚 · 252 个矩阵样张之外的放大与底面检查</div></header>' + \
       brief + family + '<section class="grid">' + cards + '</section>' + \
       '<footer><p>反馈格式：<code>i-xxx 保留</code> / <code>i-xxx 修改（原因）</code> / <code>i-xxx 替换（想要的方向）</code>。' \
       '收到反馈后只改 SVG symbol、S3 / S5 / P3 与雪碧图，同步重跑审计；不改动其它已确认的设计语义。</p>' \
       '<p>复审页本身不是交付屏；产品与规格板仍以 <code>v5-product.html</code> / <code>v5-system.html</code> 为准。</p></footer></main></body></html>'

(ROOT / 'icon-review-v5.html').write_text(html, encoding='utf-8')
print('wrote', ROOT / 'icon-review-v5.html', len(html), 'bytes')
