"""Build standalone second-round product and system boards from approved assets."""
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent
BASE = (ROOT / 'v5-layered.html').read_text(encoding='utf-8')
CSS = (ROOT / 'round2-common.css').read_text(encoding='utf-8')
SPRITE = (ROOT / 'round2-sprite.svg').read_text(encoding='utf-8')
TAPE = (ROOT.parent / 'main-window-v3/SPEC.md').read_text(encoding='utf-8').split('```js\n')[1].split('// 气 atmosphere')[0].rstrip()
TAPE += '\nfunction rgba(c,alpha){return "rgba("+c[0]+","+c[1]+","+c[2]+","+alpha+")";}'

def icon(name, size=20, label=''):
    return f'<svg class="icon" style="width:{size}px;height:{size}px" aria-hidden="true"><use href="#i-{name}"/></svg>'

def button(text, name='', cls='', attrs=''):
    return f'<button class="{cls}" {attrs}>{icon(name) if name else ""}{text}</button>'

def tape(name='鼓组混音 v2', p=None, w=96, h=64):
    progress = '' if p is None else f' data-progress="{p}"'
    return f'<span class="tape-art" data-tape="{name}" data-w="{w}" data-h="{h}"{progress} aria-label="{name}的磁带图形"></span>'

def titlebar(title='鼓组混音 v2', state='idle', mark='project'):
    mark_html = ''
    if mark:
        mark_html = f'<span class="vit-mark">{icon(mark,24)}<span class="identity-dot {state}" aria-label="{dict(idle="待机",busy="正在工作",done="已完成",error="出错")[state]}"></span></span>'
    return f'<header class="titlebar">{mark_html}<h1>{title}</h1>{button("","minimize","minimize", "aria-label=\"最小化窗口\"")}</header>'

def rail():
    return f'<nav class="project-rail" aria-label="工程操作">{button("<span>新建工程</span>","new","rail-action")}{button("<span>打开文件夹</span>","open","rail-action")}</nav>'

def composer(mode='default'):
    disabled = mode == 'disabled'
    action, name = ('停止','stop') if mode=='busy' else ('发送','send')
    return f'<form class="composer {"disabled-input" if disabled else ""} {"demo-focus" if mode=="focus" else ""}" onsubmit="return false"><textarea aria-label="消息" placeholder="{"打开完成后可继续输入" if disabled else "继续描述你想要的声音…"}" {"disabled" if disabled else ""}></textarea>{button(action,name,"primary", "disabled" if disabled else "type=\"button\"")}</form>'

def trace(current='正在渲染对比 A/B', steps=('已读取鼓组总线','已应用「温暖」')):
    return '<div class="trace" aria-label="执行轨迹">'+''.join(f'<div class="trace-step">{icon("done",16)}<span>{s}</span><time>{["0.2","0.4"][i]} 秒</time></div>' for i,s in enumerate(steps))+f'<div class="trace-step current"><span class="current-icon">{icon("busy",16)}</span><span>{current}</span></div></div>'

macro = re.search(r'<section class="macro-card".*?</section>',BASE,re.S).group(0)
def macro_card(state='applied', suffix='p3', compact=False):
    s = macro.replace('id="macro-title"',f'id="macro-title-{suffix}"').replace('aria-labelledby="macro-title"',f'aria-labelledby="macro-title-{suffix}"')
    s = s.replace('class="macro-card"',f'class="macro-card macro-{state} {"compact" if compact else ""}"')
    if state=='default':
        s=s.replace('aria-checked="true"','aria-checked="false"').replace('已应用 · 可撤销','请选择一种声音')
        s=s.replace('<button class="primary">','<button class="primary" disabled>')
    elif state=='audition':
        s=s.replace('已应用 · 可撤销','试听中 · 可取消')
        s=s.replace('</div>\n          </section>', button('取消试听','stop','text-button')+'</div>\n          </section>')
    elif state=='failed':
        s=s.replace('已应用 · 可撤销','未应用')
        s=s.replace('<div class="macro-actions">', '<p class="failure-reason">'+icon('alert',16)+'插件未响应，工程保持原样。</p><div class="macro-actions">')
        s=s.replace('>应用</button>','>重试</button>')
    if compact:
        s=s.replace('高频柔和，鼓声更厚','柔和高频，增加厚度').replace('镲片清晰，空气更足','镲片清晰，空气充足').replace('低频收紧，起音集中','收紧低频，集中起音')
    return s

def notification(state='done', demo=False):
    title='鼓组混音 v2'
    copy={'done':'已应用「温暖」，A/B 已渲染好','attention':'覆盖导出会替换现有文件，请确认','error':'插件未响应，改动尚未应用'}[state]
    actions={'done':[('打开 Vit','open'),('撤销','undo')],'attention':[('去确认','alert')],'error':[('查看','alert')]}[state]
    return f'<aside class="notification {state}" aria-label="{copy}"><div class="notification-app"><span class="notification-logo">{icon("vit",24)}<i class="identity-dot {state}"></i></span><strong>{title}</strong></div><p>{copy}</p><div class="notification-actions">'+''.join(button(t,i,'notification-button') for t,i in actions)+'</div></aside>'

def status_card(state='busy', p=.64, compact=False):
    if state=='done':
        return f'<div class="complete-receipt">{icon("done")}<span>已打开《鼓组混音 v2》</span>{button("查看轨迹","track","text-button")}</div>'
    if state=='error':
        return f'<div class="state-card error"><h3>{icon("alert")}无法打开《鼓组混音 v2》</h3><p>音频设备已断开，工程尚未打开。</p>{button("重试","open","primary")}</div>'
    return f'<section class="state-card busy {"compact-status" if compact else ""}"><h3>正在打开《鼓组混音 v2》</h3><div class="state-body">{tape(p=p)}{trace("正在连接音频设备",("已读取工程文件","已恢复 14 条轨道"))}</div><p class="status-help">完成后可继续输入</p></section>'

DAW = re.search(r'<div class="daw".*?<div class="daw-scrim"',BASE,re.S).group(0).rsplit('<div class="daw-scrim"',1)[0]
DAW = DAW.replace('id="tracks"','class="tracks-fill"').replace('id="lanes"','class="lanes-fill"')

EXTRA_CSS = r'''
:root{
 --warning-shell:#F7E9E8;--warning-edge:#9D3430;--warning-header:#943733;--warning-rim:#FFF6F5;
 --attention-shell:#F6E7AA;--attention-edge:#8E782F;--control-shadow:0 2px 4px rgba(0,0,0,.10);
 --edge-highlight:inset 0 1px 0 #F5F9FF;--display-grid:#39414A;--control-well:#BDCEE1;
}
.identity-dot{position:absolute;right:-2px;bottom:1px;width:8px;height:8px;border-radius:50%;border:1px solid var(--ui-text);background:#888888}
.identity-dot.busy,.identity-dot.attention{background:var(--yellow)}.identity-dot.done{background:var(--blue)}.identity-dot.error{background:var(--red)}
.vit-mark{--icon-bg:var(--read-bg)}.titlebar .minimize{margin-left:auto}
.tape-art{display:inline-block;flex:none;line-height:0}.tape-art svg{display:block}
.tone-warm,.tone-bright,.tone-tight{--curve:var(--control-blue)}
.macro-option[aria-checked="true"]{--curve:var(--control-yellow)}
.macro-default .level-dot{background:#888888}.macro-audition .level-dot,.macro-pending .level-dot{background:var(--yellow)}
.macro-failed{background:var(--warning-shell);border-color:var(--warning-edge);box-shadow:inset 0 1px 0 var(--warning-rim),var(--control-shadow)}
.macro-failed .macro-heading{background:var(--warning-header);border-color:var(--warning-edge)}
.macro-failed .level-dot{background:var(--paper)}.macro-failed .macro-options{background:#EACCCC;border-color:#B37773}
.macro-failed .macro-actions{background:#FCF2F1;border-color:#CA918E}
.failure-reason{display:flex;gap:8px;align-items:center;margin:12px 18px 0;font-size:14px;color:var(--ui-text)}
.macro-default .macro-option{background:#F7FBFF}.macro-default .macro-option[aria-checked=true]{background:var(--control-choice-selected)}
.macro-default .macro-actions .primary:disabled{background:#E5E5E5;color:#555555;border:1px dashed #777777;box-shadow:none}
.notification{position:absolute;width:400px;padding:20px;background:#FFFFFF;border:1px solid #B8B8B8;box-shadow:var(--shadow-popover);color:var(--ui-text);z-index:30}
.notification-app{display:flex;align-items:center;gap:12px}.notification-app strong{font-size:16px;font-weight:600}.notification-app time{font-size:13px;color:var(--ui-muted);margin-left:auto}
.notification-logo{position:relative;display:block;--icon-bg:#FFFFFF}.notification p{font-size:14px;line-height:1.55;margin-top:16px;white-space:nowrap}
.notification-actions{display:flex;gap:8px;margin-top:20px}.notification-button{flex:1;font-size:14px;border:1px solid #A9A9A9;padding:8px;background:#F5F5F5}
.notification-button:hover{background:#EAEAEA}.notification.attention{border-color:var(--attention-edge)}.notification.error{border-color:var(--warning-edge)}
.taskbar{position:absolute;bottom:0;left:0;right:0;height:48px;background:#151515;color:#F4F4F4;border-top:1px solid #444444;display:flex;align-items:center;justify-content:center;gap:12px;z-index:10}
.taskbar button{height:40px;padding:8px 16px;font-size:14px;--icon-bg:#151515}.taskbar .os-start{display:grid;grid-template-columns:6px 6px;gap:3px;margin-right:12px}.os-start i{width:6px;height:6px;background:#F4F4F4}
.taskbar .vit-entry{position:relative;background:#303030;border-bottom:2px solid #F4F4F4;border-radius:4px}.vit-entry .small-id{position:relative;display:block}.taskbar .clock{position:absolute;right:24px;font:13px/1.5 var(--mono)}
.p4 .notification{right:24px;bottom:72px}.p4 .daw{bottom:48px}.p4 .workspace{height:656px}.p4 .daw-bottom{height:48px}
.startup-window{width:1120px;height:744px;left:160px;top:64px}.startup-body{display:flex;min-height:0;flex:1}
.project-drawer{width:288px;background:#F1F1F1;border-right:2px solid var(--ui-text);padding:24px 16px;flex:none}
.drawer-actions{display:flex;gap:8px;margin-bottom:32px}.drawer-actions button{flex:1;padding:8px;font-size:14px;border:1px solid var(--control-shell-edge);background:var(--control-shell-light)}
.drawer-heading{display:flex;align-items:center;gap:12px;font-size:14px;color:#555555;padding:0 8px;margin-bottom:16px}
.project-item{width:100%;height:104px;display:flex;align-items:center;justify-content:start;gap:12px;padding:12px 8px;border:1px solid transparent;background:transparent;font-weight:400;text-align:left;margin-bottom:8px}
.project-item.active{background:var(--control-choice-selected);border:2px solid var(--ui-text);padding:11px 7px}.project-item .project-copy{display:flex;flex-direction:column;gap:6px}.project-copy strong{font-size:16px;font-weight:600;white-space:nowrap}.project-copy small{font-size:13px;color:#555555;font-weight:400}
.startup-main{flex:1;display:flex;flex-direction:column;padding:80px 56px 32px}.startup-main h2{font-size:28px;font-weight:700;line-height:1.3}.startup-main>p{font-size:16px;line-height:1.65;margin-top:16px;max-width:36em}
.suggestions{margin-top:32px;display:grid;grid-template-columns:1fr 1fr;gap:12px}.suggestion{display:flex;align-items:center;justify-content:start;gap:12px;min-height:76px;padding:16px;text-align:left;background:var(--control-shell-light);border:1px solid var(--control-shell-edge);box-shadow:var(--edge-highlight);font-size:16px;font-weight:400}
.suggestion:hover{background:var(--control-shell)}.startup-main .composer{margin: auto 0 0}.startup-main .suggestion .icon{width:24px;height:24px}
.p1{background:#E4E4E4}.p2{background:#E4E4E4}.rising-daw{position:absolute;left:64px;top:172px;width:1312px;height:680px;overflow:hidden;box-shadow:var(--shadow-window);border:2px solid #111111}.rising-daw .daw{width:1312px;height:900px}
.loading-window{width:1008px;height:752px;left:216px;top:48px}.loading-conversation{flex:1;display:flex;flex-direction:column;min-width:0;padding:32px}.loading-conversation>.message{margin-bottom:24px}.loading-conversation .state-card{margin-top:16px}.loading-conversation .composer{margin:auto 0 0}.loading-conversation .status-help{display:none}
.state-card{background:var(--control-shell);border:1px solid var(--control-shell-edge);box-shadow:var(--edge-highlight),var(--control-shadow);padding:24px;color:var(--ui-text)}
.state-card h3{font-size:18px;font-weight:600;line-height:1.4;display:flex;align-items:center;gap:8px}.state-card .state-body{display:flex;align-items:center;gap:24px;margin-top:24px}.state-card>p{font-size:14px;line-height:1.55;color:#444444;margin-top:20px}
.state-card.error{background:var(--warning-shell);border-color:var(--warning-edge)}.state-card.error>button{margin-top:20px}
.complete-receipt{display:flex;align-items:center;gap:12px;background:var(--control-shell-light);border:1px solid var(--control-shell-edge);padding:12px;font-size:14px}.complete-receipt .text-button{margin-left:auto;font-size:14px;white-space:nowrap}
.disabled-input{background-color:#F1F1F1;background-image:repeating-linear-gradient(135deg,transparent 0 8px,rgba(0,0,0,.045) 8px 10px);border:1px dashed #777777}
.disabled-input textarea{background:transparent;color:#555555}.disabled-input textarea::placeholder{color:#555555}.disabled-input button{background:#E5E5E5;color:#555555;opacity:1}
.context-target{background:var(--control-shell-light);border:1px solid var(--control-shell-edge);box-shadow:var(--edge-highlight);color:var(--ui-text)}
.mini-taskbar .state-dot.safe-dark{background:var(--blue-lt)!important}.taskbar .identity-dot.done{background:var(--blue-lt)}
.demo-focus{outline:2px solid var(--ui-text);outline-offset:2px}.danger{background:var(--red);color:#FFFFFF;border:1px solid #8F100E}.danger:hover:not(:disabled){background:#B60E10;color:#FFFFFF}
.secondary{background:#FFFFFF;border:1.5px solid #444444}.busy-motion{animation:advance 1.2s steps(3,end) infinite}
'''

# v5.1：用户 2026-11-14 裁定采用「空间修订」（A）。结构由面差、接触影与柔影承担，
# 不再给每一层套等重黑框；选中项保留深墨轮廓并加 1px 外环，投影仍低于未选项（按下语义）。
DEPTH_CSS = r'''
:root{
 --depth-workspace:#F0F1F2;--depth-line:#718094;
 --depth-contact:0 1px 2px rgba(0,0,0,.18);
 --depth-control:0 6px 16px rgba(21,36,54,.12);
 --shadow-window:0 2px 4px rgba(0,0,0,.28),0 18px 48px rgba(0,0,0,.28),0 48px 96px rgba(0,0,0,.18);
}
.vit-window{border:1px solid #5B626A;box-shadow:var(--shadow-window)}
.titlebar{border-bottom:1px solid #D9DDE1;box-shadow:inset 0 1px 0 #FFFFFF}
.conversation,.startup-main,.loading-conversation{background:var(--depth-workspace)}
.project-context{background:#FAFBFC;border-bottom-color:#D0D5DB}
.project-rail,.project-drawer{background:#E9ECEF;border-right-color:#CAD0D7}
.context-target{border-color:#8292A5;box-shadow:inset 0 1px 0 #FFFFFF,0 1px 1px rgba(0,0,0,.10),0 3px 7px rgba(21,36,54,.10)}
.macro-card:not(.macro-failed){border:0;box-shadow:var(--depth-contact),var(--depth-control);background:#D4E1EF}
.macro-card:not(.macro-failed) .macro-heading{border-top:1px solid #7794AF;border-bottom:2px solid #172C42;box-shadow:inset 0 1px 0 rgba(255,255,255,.14)}
.macro-card:not(.macro-failed) .macro-options{border:0;box-shadow:inset 0 2px 4px rgba(29,44,65,.16),0 1px 0 rgba(255,255,255,.7);background:#BDCDDF;margin:16px 18px 0;padding:10px;gap:12px}
.macro-card:not(.macro-failed) .macro-option{border:1px solid #9BA8B7;box-shadow:inset 0 1px 0 #FFFFFF,0 1px 2px rgba(0,0,0,.12),0 4px 8px rgba(29,44,65,.10)}
.macro-card:not(.macro-failed) .macro-option[aria-checked=true]{border:2px solid #141414;box-shadow:inset 0 1px 0 #FFF8D8,inset 0 -2px 0 #D7C47E,0 0 0 1px rgba(20,20,20,.20),0 1px 2px rgba(0,0,0,.10)}
.macro-option .selection-lamp{border-color:#777777}
.macro-card:not(.macro-failed) .macro-option:not([aria-checked=true]):hover{border-color:#61758D;background:#FAFCFF;box-shadow:0 2px 3px rgba(0,0,0,.10),0 6px 12px rgba(29,44,65,.12)}
.contour{border:0;box-shadow:inset 0 2px 4px rgba(0,0,0,.55),0 1px 0 rgba(255,255,255,.8)}
.macro-card:not(.macro-failed) .macro-actions{border-top:1px solid #A3B6CD;box-shadow:inset 0 1px 0 #F9FBFF;background:#EDF3F9;margin-top:16px}
.comparison{border-top-color:#D5D9DE}
.composer{border-color:#9BA4AE;box-shadow:inset 0 1px 2px rgba(0,0,0,.045),0 2px 8px rgba(0,0,0,.055)}
'''

COMMON_SCRIPT = r'''
document.querySelectorAll('[data-tape]').forEach(el=>{
 const name=el.dataset.tape;let hash=2166136261;for(const ch of name){hash^=ch.charCodeAt(0);hash=Math.imul(hash,16777619)}
 const colours=[[6,91,199],[250,204,26],[115,184,245]];let a=name==='ProQ3 裸控实验'?null:{L:.8,R:.8,c1:colours[(hash>>>0)%colours.length]};
 if(el.dataset.progress!==undefined){const p=+el.dataset.progress;a={L:1-.8*p,R:.2+.8*p,c1:a?a.c1:[200,200,196]};}
 el.innerHTML=tapeSVG(+el.dataset.w,+el.dataset.h,a);
});
const tracks=['鼓组总线','底鼓','军鼓','闭镲','头顶麦','房间麦'];
document.querySelectorAll('.tracks-fill').forEach(el=>el.innerHTML=tracks.map((name,i)=>`<div class="track"><div class="track-name"><svg class="icon"><use href="#i-${i?'track':'bus'}"/></svg>${name}</div><div class="track-controls"><span>静音</span><span>独奏</span><i class="level" style="width:${72-i*6}px"></i></div></div>`).join(''));
document.querySelectorAll('.lanes-fill').forEach(el=>el.innerHTML=tracks.map((name,i)=>{let d='';for(let n=0;n<200;n++){const x=n*5,h=3+Math.abs(Math.sin(n*(i+2)*1.87))*13*(n%7===0?1.7:1);d+=`M${x} ${24-h}V${24+h}`;}return `<div class="lane"><div class="clip"><span class="clip-label">${name}</span><svg class="waveform" viewBox="0 0 1000 48" preserveAspectRatio="none" aria-hidden="true"><path d="${d}"/></svg></div></div>`}).join(''));
document.querySelectorAll('[role=radiogroup]').forEach(group=>{
 const choose=el=>{if(group.closest('.macro-failed'))return;group.querySelectorAll('[role=radio]').forEach(e=>{e.setAttribute('aria-checked',String(e===el));e.tabIndex=e===el?0:-1});const card=group.closest('.macro-card');card.classList.add('macro-pending');const status=card.querySelector('.applied>span:last-child');if(status)status.textContent='已选择「'+el.querySelector('.macro-name').textContent+'」';const primary=card.querySelector('.macro-actions .primary');if(primary)primary.disabled=false};
 const choices=[...group.querySelectorAll('[role=radio]')],chosen=choices.find(e=>e.getAttribute('aria-checked')==='true')||choices[0];choices.forEach(e=>e.tabIndex=e===chosen?0:-1);
 group.querySelectorAll('[role=radio]').forEach(el=>{el.addEventListener('click',()=>choose(el));el.addEventListener('keydown',event=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;event.preventDefault();const items=[...group.querySelectorAll('[role=radio]')],i=items.indexOf(el),next=event.key==='Home'?0:event.key==='End'?items.length-1:(i+(event.key==='ArrowRight'?1:-1)+items.length)%items.length;choose(items[next]);items[next].focus()})});
});
'''

def document(title, body, extra_style='', script='', body_class=''):
    return '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>'+title+'</title><style>'+CSS+'\n'+EXTRA_CSS+'\n'+extra_style+'\n'+DEPTH_CSS+'</style></head><body class="'+body_class+'">'+body+SPRITE+'<script>'+TAPE+'\n'+COMMON_SCRIPT+'\n'+script+'</script></body></html>'

def product():
    projects=''.join(f'<button class="project-item {"active" if i==0 else ""}">{tape(n,w=84,h=56)}<span class="project-copy"><strong>{n}</strong><small>{time}</small></span></button>' for i,(n,time) in enumerate([('鼓组混音 v2','今天 14:07'),('小星星乐局','昨天 21:18'),('ProQ3 裸控实验','11 月 12 日')]))
    suggestions=''.join(button(text,name,'suggestion') for text,name in [('继续《鼓组混音 v2》','play'),('查看上次快照','snapshot'),('继续《小星星乐局》','play'),('查看鼓组轨道','track')])
    p1=f'<section id="p1" class="screen p1" aria-label="启动台"><div class="vit-window startup-window">{titlebar("Vit", mark=None)}<div class="startup-body"><aside class="project-drawer"><div class="drawer-actions">{button("新建工程","new")}{button("打开文件夹","open")}</div><p class="drawer-heading">最近的工程</p>{projects}</aside><div class="startup-main"><h2>下午好，想从哪里继续？</h2><p>《鼓组混音 v2》的快照已经保存，可以接着听。</p><div class="suggestions">{suggestions}</div>{composer()}</div></div></div></section>'
    p2=f'<section id="p2" class="screen p2" aria-label="正在打开工程"><div class="rising-daw">{DAW}</div><div class="vit-window loading-window">{titlebar(state="busy")}<div class="window-body">{rail()}<div class="loading-conversation"><p class="message user-message">打开《鼓组混音 v2》。</p><p class="message">我正在恢复上次的工程和轨道。</p>{status_card()}{composer("disabled")}</div></div></div></section>'
    main=re.search(r'<main id="p3".*?</main>',BASE,re.S).group(0).replace('<main id="p3"','<section id="p3"').replace('</main>','</section>')
    main=main.replace('class="screen"','class="screen p3"').replace('id="tracks"','class="tracks-fill"').replace('id="lanes"','class="lanes-fill"')
    main=re.sub(r'<section class="macro-card".*?</section>',lambda m:macro_card(),main,flags=re.S)
    main=main.replace('<svg class="icon" aria-label="快照已保存"><use href="#i-snapshot"/></svg>','')
    main=re.sub(r'<header class="titlebar">.*?</header>',lambda m:titlebar(state='busy'),main,count=1,flags=re.S)
    p4=f'<section id="p4" class="screen p4" aria-label="Vit 已最小化，改动已完成">{DAW}<div class="taskbar"><span class="os-start" aria-hidden="true"><i></i><i></i><i></i><i></i></span>{button("鼓组混音 v2","project") }<button class="vit-entry" aria-label="打开 Vit，任务已完成"><span class="small-id">{icon("vit",24)}<i class="identity-dot done"></i></span>Vit</button><time class="clock">14:08</time></div>{notification()}</section>'
    css='.product-board{display:grid;grid-template-columns:1440px 1440px;gap:40px;width:2920px}html,body{width:2920px;min-height:1840px;background:#E4E4E4}.product-board>.screen{flex:none} .p3 .daw-scrim{background:rgba(0,0,0,.10)}'
    (ROOT/'v5-product.html').write_text(document('Vit 主窗口 · 四个工作场景','<main class="product-board">'+p1+p2+main+p4+'</main>',css),encoding='utf-8')

SYSTEM_CSS = r'''
html,body{width:1440px;min-height:900px;background:#E4E4E4;color:var(--ui-text)}
.system-board{width:1440px;display:flex;flex-direction:column;gap:40px}
.system-panel{width:1440px;padding:48px 56px;background:#FFFFFF;position:relative}
.board-heading{display:flex;justify-content:space-between;align-items:end;margin-bottom:32px;gap:32px}
.board-heading h1{font-size:28px;font-weight:700;line-height:1.3}.board-heading p{font-size:14px;color:#555555;line-height:1.55;margin-top:12px}
.board-heading>span{font-size:14px;line-height:1.55;color:#555555;white-space:nowrap}
.system-panel h2{font-size:22px;font-weight:700;line-height:1.25;margin:32px 0 20px}
.system-panel h3{font-size:18px;font-weight:600;line-height:1.4;margin:0 0 16px}
.system-panel p{font-size:16px;line-height:1.65}.system-panel .caption{font-size:13px;line-height:1.5;color:#555555}
.system-panel .secondary-copy{font-size:14px;line-height:1.55;color:#555555}
.board-table{width:100%;border-collapse:collapse;text-align:left;font-size:14px;line-height:1.55}
.board-table th{font-size:13px;font-weight:400;color:#555555;padding:12px 16px;border-bottom:1px solid #D3D3D3}
.board-table td{padding:12px 16px;border-bottom:1px solid #E2E2E2;vertical-align:middle}
.board-table th:first-child,.board-table td:first-child{padding-left:0}.board-table th:last-child,.board-table td:last-child{padding-right:0}
.token-swatch{display:inline-block;width:28px;height:28px;vertical-align:middle;margin-right:12px;border:1px solid rgba(0,0,0,.18)}
.mono-value{font-family:var(--mono)}.type-sample{display:flex;justify-content:space-between;align-items:baseline;border-bottom:1px solid #E2E2E2;min-height:40px;gap:20px}
.type-sample .secondary-copy{font-size:13px;white-space:nowrap}
.board-columns{display:grid;grid-template-columns:1fr 1fr;gap:32px}.board-columns.three{grid-template-columns:repeat(3,1fr)}.board-columns.four{grid-template-columns:repeat(4,1fr)}
.spec-box{padding:24px;background:#F5F5F5;border:1px solid #D3D3D3}.spec-box>p+p{margin-top:12px}.spec-box button{margin-top:12px}.spec-box h3{margin-bottom:12px}
.accepted-note{background:var(--control-shell-light);border:1px solid var(--control-shell-edge);padding:20px 24px;margin-bottom:24px}
.pending-note{background:#F8F1D3;border:1px solid var(--attention-edge);padding:20px 24px;margin-top:32px}.pending-note h3{margin-bottom:8px}
.dimension-demo{display:flex;gap:16px;align-items:end;margin:20px 0}.dimension-demo span{display:flex;flex-direction:column;align-items:center;gap:8px;font-size:13px;color:#555555}.dimension-demo i{display:block;width:24px;background:#777777}
.icon-grid-head{display:grid;grid-template-columns:240px 1fr 440px;gap:40px;align-items:center;margin-bottom:32px}
.construction-art{width:240px;height:240px;background:#F5F5F5}.construction-art .grid-line{stroke:#CCCCCC;stroke-width:.1}.construction-art .active-zone{stroke:#666666;stroke-width:.1;stroke-dasharray:.3 .3;fill:none}
.icon-family{display:flex;gap:32px;align-items:center}.icon-matrix{table-layout:fixed}.icon-matrix col:first-child{width:200px}.icon-matrix th,.icon-matrix td{text-align:center;padding:6px 4px}.icon-matrix th:first-child,.icon-matrix td:first-child{text-align:left}.icon-matrix .dark-cell{background:#1C1C1C;color:#FFFFFF;--icon-bg:#1C1C1C}.icon-label{font-size:14px}.icon-label span{display:block;font-size:13px;color:#666666}
.icon-sample{width:42px;height:34px;border:2px solid transparent;border-radius:4px;display:flex;align-items:center;justify-content:center;margin:auto}
.icon-sample.is-selected{background:var(--control-choice-selected);color:#141414;border-color:#141414;--icon-bg:var(--control-choice-selected)}
.surface-icon-set{padding:24px;display:flex;align-items:center;gap:24px}.surface-icon-set.blue{background:var(--control-header);color:#FFFFFF;--icon-bg:var(--control-header)}.surface-icon-set.yellow{background:var(--control-choice-selected);color:#141414;--icon-bg:var(--control-choice-selected)}.surface-icon-set.red{background:var(--warning-header);color:#FFFFFF;--icon-bg:var(--warning-header)}
.surface-icon-set strong{font-size:16px;min-width:116px}.surface-icon-set p{font-size:14px;margin-left:auto}
.logo-row{display:flex;align-items:center;gap:32px;padding:24px;background:#F5F5F5}.logo-row.deep{background:#1C1C1C;color:#FFFFFF;--icon-bg:#1C1C1C}.logo-row.mono{background:#000000;color:#FFFFFF;--icon-bg:#000000}
.logo-sample{display:flex;align-items:center;flex-direction:column;gap:12px}.logo-sample span{font-size:13px}.logo-row strong{font-size:16px;min-width:116px}
.z-frame{height:192px;background:#E4E4E4;border:1px solid #CCCCCC;position:relative;overflow:hidden;margin-bottom:16px}.z-frame .diagram-daw{position:absolute;background:#202020;border:1px solid #4F4F4F;left:12px;right:12px;top:54px;bottom:12px}.z-frame .diagram-daw:after{content:"";position:absolute;inset:16px;background:repeating-linear-gradient(0deg,#424242 0 1px,transparent 1px 24px)}.z-frame .diagram-vit{position:absolute;background:#FFFFFF;border:1px solid #5B626A;left:44px;right:44px;top:18px;bottom:24px;box-shadow:var(--shadow-popover)}.diagram-vit:before{content:"";position:absolute;left:0;right:0;top:24px;border-top:1px solid #CCCCCC}.z-frame .diagram-vit:after{content:"";position:absolute;left:20px;right:20px;height:36px;top:56px;background:var(--control-shell);border:1px solid var(--control-shell-edge)}
.z-frame.stage-1 .diagram-daw{display:none}.z-frame.stage-2 .diagram-daw{top:30px;left:8px;right:8px;bottom:6px}.z-frame.stage-2 .diagram-vit{right:80px;bottom:36px}.z-frame.stage-3 .diagram-daw,.z-frame.stage-4 .diagram-daw{inset:0}.z-frame.stage-3 .diagram-vit{left:68px;right:68px;top:28px;bottom:20px}.z-frame.stage-4 .diagram-vit{display:none}.z-frame.stage-4:after{content:"";position:absolute;height:16px;bottom:0;left:0;right:0;background:#090909}
.motion-rule{display:flex;gap:32px;margin-top:32px;background:#F5F5F5;padding:24px;border:1px solid #D3D3D3}.motion-rule p{font-size:14px;flex:1}
.state-matrix .state-mark{display:flex;gap:12px;align-items:center}.state-matrix .state-dot{width:10px;height:10px;display:inline-block;border:1px solid #333333;border-radius:50%}
.state-matrix .mini-taskbar{background:#151515;color:#FFFFFF;display:inline-flex;padding:8px;gap:8px;align-items:center;--icon-bg:#151515}.mini-id{position:relative;display:inline-block}
.trajectory-frame{padding:24px;background:var(--control-shell-light);border:1px solid var(--control-shell-edge);min-height:188px}.trajectory-frame h3{font-size:16px;margin-bottom:20px}.trajectory-frame .trace-step{margin-top:8px}
.tape-frames>div{background:#F5F5F5;border:1px solid #D3D3D3;padding:24px;display:flex;align-items:center;gap:24px}.tape-frames p{font-size:14px}.tape-frames .caption{margin-top:8px}
.system-panel .macro-card{width:100%;margin:0}.component-state h3{font-size:16px;margin-bottom:12px}.component-state .macro-heading h2{font-size:18px;font-weight:600;line-height:1.4;margin:0}.component-state .macro-heading{padding:0 16px}.component-state .applied{font-size:13px;white-space:nowrap}.component-state .macro-options{margin-left:12px;margin-right:12px;padding:8px;gap:8px}.component-state .macro-option{height:180px;padding:10px}.component-state .macro-option[aria-checked=true]{padding:9px}.component-state .macro-description{white-space:normal;min-height:44px;line-height:1.55}.component-state .contour{height:54px;margin-top:10px}.component-state .macro-actions{padding:12px 16px}.component-state .macro-name{font-size:22px}.component-state .failure-reason{margin:12px 16px 0}
.component-grid{display:grid;grid-template-columns:1fr 1fr;gap:32px}.component-state .macro-actions button{font-size:14px}.component-state .macro-actions .primary{padding:8px 16px}
.action-card{padding:24px;background:var(--control-shell-light);border:1px solid var(--control-shell-edge);box-shadow:var(--control-shadow)}.action-card h3{display:flex;align-items:center;gap:8px;font-size:16px}.action-card p{font-size:14px;line-height:1.55}.action-card .actions{display:flex;gap:12px;margin-top:20px}.action-card.attention{background:#F8EFCD;border-color:var(--attention-edge)}.action-card.dangerous{background:var(--warning-shell);border-color:var(--warning-edge)}.level-word{font-size:14px;display:inline-flex;gap:8px;align-items:center;margin-bottom:16px}.level-word .icon{width:16px;height:16px}
.input-spec .composer{margin:0;min-height:68px}.input-spec h3{font-size:16px;margin-bottom:16px}.input-spec{padding:24px;background:#F5F5F5;border:1px solid #D3D3D3}
.status-spec .state-card{padding:20px}.status-spec .state-card h3{font-size:18px}.status-spec .state-body{gap:16px}.status-spec .trace-step{font-size:14px}.message-demo{padding:24px;background:#FFFFFF;border:1px solid #D3D3D3}.message-demo .user-message{margin-bottom:16px}.message-demo .receipt{margin-top:24px}
.history-test{padding:24px;background:#F5F5F5;border:1px solid #D3D3D3}.history-test h3{margin-bottom:12px}.history-view{height:112px;background:#FFFFFF;border:1px solid #A9A9A9;display:flex;align-items:center;padding:24px;gap:8px;margin:20px 0}.history-event{display:block;height:28px;background:var(--control-shell);border:1px solid var(--control-shell-edge)}.history-event.attention{background:var(--control-choice-selected);border-color:var(--attention-edge)}.history-event.dangerous{background:#D79993;border-color:var(--warning-edge)}.history-view.grid-view{background-image:linear-gradient(#DEDEDE 1px,transparent 1px),linear-gradient(90deg,#DEDEDE 1px,transparent 1px);background-size:24px 24px}.history-view.history-rows{display:grid;grid-template-columns:repeat(8,1fr);gap:5px;padding:12px}.history-rows .history-event{height:20px}.history-legend{display:flex;gap:24px;font-size:14px;color:#444444;margin-top:16px}.history-legend span{display:flex;gap:8px;align-items:center}.legend-dot{width:12px;height:12px;background:var(--control-shell);border:1px solid var(--control-shell-edge)}
.notification-spec{position:relative;min-height:206px;background:#E4E4E4;padding:16px}.notification-spec .notification{position:relative;left:auto;right:auto;top:auto;bottom:auto;width:100%;padding:16px}.notification-spec .notification p{font-size:14px;white-space:normal}.notification-spec .notification-app strong{font-size:16px}.notification-spec .notification-actions{gap:8px;margin-top:16px}.notification-spec .notification-button{font-size:14px;padding:8px}.sound-controls{display:flex;gap:16px;align-items:center;margin-top:24px}.sound-controls button{background:var(--control-shell-light);border:1px solid var(--control-shell-edge)}.sound-controls button[aria-pressed=true]{background:var(--control-choice-selected);border:2px solid #141414}.sound-feedback{font-size:14px;color:#444444;margin-top:16px;min-height:24px}
.sound-table button{font-size:14px;padding:8px 12px;border:1px solid var(--control-shell-edge);background:var(--control-shell-light)}
.review-decisions{display:grid;grid-template-columns:1fr 1fr;gap:24px;margin-top:24px}.review-decisions p{font-size:14px;line-height:1.55}.review-decisions h3{font-size:16px;margin-bottom:12px}
'''

def heading(number,title,subtitle,right='第二轮 / 2026-11-14'):
    return f'<header class="board-heading"><div><h1>{number}　{title}</h1><p>{subtitle}</p></div><span>{right}</span></header>'

def panel(number, body):
    return f'<section class="system-panel" id="{number.lower()}">{body}</section>'

def system():
    tokens=[
        ('--read-bg','#FFFFFF','窗口与卡片面','--ui-text','#141414'),('--depth-workspace','#F0F1F2','对话阅读面 / 工作区','--ui-text','#141414'),('--ui-text','#141414','正文 / 墨色主按钮','--read-bg','#FFFFFF'),
        ('--ui-muted','#555555','辅助文字 / 时间戳','--read-bg','#FFFFFF'),('--control-header','#31567D','复合控件标题层','白字','#FFFFFF'),
        ('--control-shell','#D0DFEF','普通控件的蓝色外壳','黑字','#141414'),('--control-shell-light','#ECF2F8','动作区 / 目标选择控件','黑字','#141414'),
        ('--control-shell-edge','#3E5B7C','控件边界','白底','#FFFFFF'),('--control-well','#BDCEE1','选项嵌入的内槽','黑字','#141414'),
        ('--control-choice-selected','#F6E7AA','选中项 / 需要留意的面','黑字','#141414'),('--warning-header','#943733','失败 / 更高警示标题层','白字','#FFFFFF'),
        ('--warning-shell','#F7E9E8','失败原因与恢复动作','黑字','#141414'),('--warning-edge','#9D3430','警示控件边界','白底','#FFFFFF'),
        ('--control-pane','#1D2228','凹入的曲线显示区','--control-blue','#73B8F5'),('--control-yellow','#FACC1A','选中曲线 / 当前步骤标记','--control-pane','#1D2228'),
    ]
    token_rows=''.join(f'<tr><td><span class="token-swatch" style="background:{value}"></span>{token}</td><td class="mono-value">{value}</td><td>{role}</td><td><span class="contrast-pair" data-a="{fg}" data-b="{value}"></span><span class="caption">　{fgl}</span></td></tr>' for token,value,role,fgl,fg in tokens)
    type_rows=''.join(f'<div class="type-sample"><span style="font-size:{size}px;font-weight:{weight};line-height:{line}">{label}</span><span class="secondary-copy">{size}px / {weight} / {line}</span></div>' for label,size,weight,line in [('说明与时间戳',13,400,1.5),('次要文字',14,400,1.55),('对话正文',16,400,1.65),('正文强调与按钮',16,600,1.65),('控件标题',18,600,1.4),('温暖',22,700,1.25),('下午好，想从哪里继续？',28,700,1.3)])
    s4=panel('S4',heading('S4','令牌与控件层级','已确认：普通复合控件蓝底，选中浅黄，红色用于更高警示。')+f'<div class="accepted-note"><p>应用整体使用黑白灰；阅读面用浅中性灰，复合控件用蓝色外壳、内槽与独立选项建立边界。文字随深浅底切换黑白，普通内容不借用红色。空间关系由面差、接触影与柔影承担，不用等重黑框包住每一层。</p></div><table class="board-table"><thead><tr><th>令牌</th><th>当前色值</th><th>职责</th><th>实测对比度</th></tr></thead><tbody>{token_rows}</tbody></table><div class="board-columns" style="margin-top:32px"><div><h3>字阶</h3>{type_rows}<p class="caption" style="margin-top:16px">统一系统无衬线字族。中文正文字距 0；Consolas 仅用于数值、时间码与键帽。</p></div><div><h3>间距、边界与深度</h3><div class="dimension-demo">'+''.join(f'<span><i style="height:{n}px"></i>{n}px</span>' for n in [4,8,12,16,24,32,40,48])+f'</div><p class="secondary-copy">8px 基准 / 4px 细部。窗口 1px 灰边，控件外壳 1px 深蓝灰；选中 2px 墨框 + 1px 外环；内槽以内阴影凹入，普通选项以短影微抬。面圆角 0，交互控件 4px，状态点正圆。</p><p class="secondary-copy" style="margin-top:16px">接触影 0 1px 2px 墨色 18%；控件浮起 0 6px 16px 深蓝灰 12%。窗口：0 2px 4px / 0 18px 48px / 0 48px 96px 三层柔影；弹出层 8px / 24px。普通消息无框无影；选中项投影低于未选项，表达按下。</p><p class="secondary-copy" style="margin-top:16px">本轮 DAW 压暗：黑色 10%。按合成后的前景与背景实测对比度，正文 ≥4.5:1，必要图形 ≥3:1。</p></div></div><div class="board-columns three" style="margin-top:32px"><div class="spec-box"><h3>默认</h3>{button("选择鼓组总线","bus","context-target")}<p class="secondary-copy">蓝色系区分可操作组件。</p></div><div class="spec-box" style="background:#F6E7AA"><h3>选中 / 需留意</h3>{button("温暖","macro","secondary")}<p class="secondary-copy">浅黄 + 轮廓 + 形状，保留文字提示。</p></div><div class="spec-box" style="background:#F7E9E8;border-color:#9D3430"><h3>更高警示</h3>{button("确认覆盖","alert","danger")}<p class="secondary-copy">原因、后果、恢复或确认动作必须同时出现。</p></div></div><p class="caption" style="margin-top:24px">9 个原始锚定色保留为基线常量；已认可控件色阶为独立派生令牌。旧探索稿不再是颜色语义的实施依据。</p>')

    icons=[('project','工程'),('new','新建工程'),('open','打开文件夹'),('minimize','最小化'),('send','发送'),('stop','停止执行'),('track','轨道'),('bus','总线 / 编组'),('fx','效果器'),('macro','宏'),('ab','对比 A/B'),('play','播放'),('record','录音'),('render','渲染 / 导出'),('undo','撤销'),('snapshot','快照'),('attach','引用轨道 / 区间'),('idle','待机'),('busy','进行中'),('done','完成'),('error','出错'),('alert','错误 / 警示'),('vit','Vit 标志')]
    sizes='<tr><th></th>'+''.join(''.join(f'<th class="{"dark-cell" if group>1 else ""}">{size}px</th>' for size in [16,20,24]) for group in range(4))+'</tr>'
    icon_rows=''.join('<tr><td class="icon-label">'+label+'<span>i-'+name+'</span></td>'+''.join(''.join(f'<td class="{"dark-cell" if group>1 else ""}"><span class="icon-sample {"is-selected" if group%2 else ""}">{icon(name,size)}</span></td>' for size in [16,20,24]) for group in range(4))+'</tr>' for name,label in icons)
    grid=''.join(f'<path class="grid-line" d="M{n} 0V24M0 {n}H24"/>' for n in range(25))
    logo_rows=''.join(f'<div class="logo-row {theme}"><strong>{label}</strong>'+''.join(f'<div class="logo-sample">{icon("vit",size)}<span>{size}px</span></div>' for size in [16,20,24,32])+'</div>' for theme,label in [('', '浅底'),('deep','深底'),('mono','单色高对比')])
    coloured_icons=''.join(f'<div class="surface-icon-set {theme}"><strong>{label}</strong>'+''.join(icon(i,24) for i in ['vit','bus','macro','alert'])+f'<p>文字 / 底面 <span class="contrast-pair" data-a="{fg}" data-b="{bg}"></span></p></div>' for theme,label,fg,bg in [('blue','普通控件','#FFFFFF','#31567D'),('yellow','选中控件','#141414','#F6E7AA'),('red','高警示控件','#FFFFFF','#943733')])
    s3=panel('S3',heading('S3','图标集','23 枚原创资产：22 枚功能图标 + i-vit。')+f'<div class="icon-grid-head"><svg class="icon construction-art" viewBox="0 0 24 24">{grid}<rect class="active-zone" x="2" y="2" width="20" height="20"/></svg><div><h3>构造规则</h3><p class="secondary-copy">24×24 网格，20×20 活动区。线重分层：结构 2 / 2.5px，细节 1.5px，引导 1px；物理质量用实心，选中/激活用反白；开轮廓与曲线只给需要它们的对象。</p></div><div><h3>同族资产</h3><div class="icon-family">{icon("project",32)}{tape("ProQ3 裸控实验",w=96,h=64)}</div><p class="caption" style="margin-top:16px">工程标志保留磁带壳体、窗口和双盘；`i-vit` 是 V 品牌标，只用于 OS 表层。原始 tapeSVG 不缩到 48px 以下，结构逐字保留。</p></div></div><table class="board-table icon-matrix"><colgroup><col><col span="12"></colgroup><thead><tr><th>含义 / 标识</th><th colspan="3">浅底默认</th><th colspan="3">浅底选中</th><th class="dark-cell" colspan="3">深底默认</th><th class="dark-cell" colspan="3">深底选中</th></tr>{sizes}</thead><tbody>{icon_rows}</tbody></table><h2>小尺寸身份标志</h2>{logo_rows}<h2>控件表面适配</h2>{coloured_icons}<p class="caption" style="margin-top:24px">禁用用中性弱色与虚线边；四种状态共用同一状态环：待机 3/4 虚线、进行中 3/4 实线、完成满环 + 勾、出错满环 + 感叹；颜色不是唯一编码。</p>')

    stages=[('独占前台','启动页即对话窗；工程尚未打开。'),('DAW 升起','对话窗保持前台，DAW 在其后展开。'),('落位沉后','DAW 全屏，对话窗为 60% 宽主面板。'),('最小化退场','对话窗完全退出画面，仅 OS 入口召回。')]
    s1=panel('S1',heading('S1','窗口二态与层序','只存在打开在前、完全最小化两态。')+'<div class="board-columns four">'+''.join(f'<div><div class="z-frame stage-{i+1}"><div class="diagram-daw"></div><div class="diagram-vit"></div></div><h3>{i+1}　{title}</h3><p class="secondary-copy">{desc}</p></div>' for i,(title,desc) in enumerate(stages))+'</div><div class="motion-rule"><p><strong>打开 / 升起</strong><br>窗口 240ms，cubic-bezier(.2,.8,.2,1)；DAW 320ms，起步延后 80ms。保持焦点与层序，不让 DAW 越过对话窗。</p><p><strong>落位</strong><br>DAW 在下，对话窗在上；always-on-top 仅在对话窗打开态成立。最小化由 OS 接管，不模拟中间态。</p><p><strong>减少动态</strong><br>直接切换终态，执行状态改用文字与静态步骤；无把手、悬浮条或半收拢窗口。</p></div><div class="board-columns" style="margin-top:32px"><div class="spec-box"><h3>召回路径</h3><p>任务栏 / Alt+Tab / 系统通知，均召回同一个对话窗口。</p></div><div class="spec-box"><h3>窗口关闭与最小化</h3><p>本原型只定义最小化。退出应用沿用宿主行为，不能把“关闭”悄悄改成收拢。</p></div></div>')

    state_rows=''
    for name,code,col,desc,progress in [('待机','idle','#777777','未开始；无需要关注的事',0),('进行中','busy','#FACC1A','步骤推进；当前步骤小标记',.64),('已完成','done','#065BC7','结果已保存且可回退',1),('出错','error','#DB1214','说明原因并给出恢复入口',.64)]:
        ic={'idle':'idle','busy':'busy','done':'done','error':'error'}[code]
        state_rows+=f'<tr><td><span class="state-mark">{icon(ic)}{name}</span></td><td>{desc}</td><td><span class="mini-taskbar">{icon("vit",24)}<i class="state-dot {'safe-dark' if code=='done' else ''}" style="background:{col}"></i>Vit</span></td><td><span class="mini-id">{icon("vit",24)}<i class="identity-dot {code}"></i></span></td><td>{tape(p=progress,w=84,h=56)}</td></tr>'
    frame_bodies=[
        '<div class="trace-step">'+icon('busy',16)+'<span>正在读取工程文件</span></div>',
        '<div class="trace-step">'+icon('done',16)+'<span>已读取工程文件　0.2 秒</span></div><div class="trace-step current"><span class="current-icon">'+icon('busy',16)+'</span>正在恢复轨道</div>',
        trace('正在连接音频设备',('已读取工程文件','已恢复 14 条轨道')),
        '<div class="trace-step">'+icon('done',16)+'<span>已打开《鼓组混音 v2》</span></div><p class="caption" style="margin-top:20px">三步收起为一行回执，可展开轨迹。</p>'
    ]
    s2=panel('S2',heading('S2','状态与执行轨迹','普通控件蓝色；选中与当前关注用黄；高警示用红，形状与文字同步。')+f'<table class="board-table state-matrix"><thead><tr><th>Vit 状态</th><th>内容表达</th><th>任务栏</th><th>i-vit 状态点</th><th>磁带载体</th></tr></thead><tbody>{state_rows}</tbody></table><h2>执行轨迹 · 四个关键帧</h2><div class="board-columns">'+''.join(f'<div class="trajectory-frame"><h3>{i+1}　{t}</h3>{b}</div>' for i,(t,b) in enumerate(zip(['请求开始','已有完成步骤','进行到一半','完成并收起'],frame_bodies)))+'</div><p class="secondary-copy" style="margin-top:20px">新步骤出现 160ms，由下方 4px 位移进入；当前阶梯 1.2s、3 个节拍上下移动 1px。进度不定时不伪造剩余秒数。一张卡最多同时出现 3 步；prefers-reduced-motion 下全部静止。</p><h2>磁带带量 · 三态</h2><div class="board-columns three tape-frames">'+''.join(f'<div>{tape(p=p)}<div><strong>{label}</strong><p>L={1-.8*p:.2f} / R={.2+.8*p:.2f}</p><p class="caption">{note}</p></div></div>' for p,label,note in [(0,'待机','左满右少'),(.64,'进行中','从左盘绕向右盘'),(1,'完成','左少右满')])+'</div><p class="caption" style="margin-top:20px">出错时冻结当前带量，不假装完成。磁带与进度条二选一；本轮用磁带，不再叠加进度条。</p>')

    macros=''.join(f'<div class="component-state"><h3>{label}</h3>{macro_card(state,"s5-"+state,True)}</div>' for state,label in [('default','默认 · 尚未选择'),('audition','已选中 / 试听中'),('applied','已应用 · 可撤销'),('failed','失败 · 原工程保留')])
    actions=''.join(f'<div class="action-card {cls}"><span class="level-word">{icon(ic,16)}{level}</span><h3>{title}</h3><p>{desc}</p><div class="actions">{buttons}</div></div>' for cls,ic,level,title,desc,buttons in [('', 'done','可撤销','调整鼓组总线','先保存快照，再应用这一组改动。',button('应用','macro','primary')),('attention','idle','需确认','替换 6 条轨道的处理链','已有设置会被替换，快照保留原始状态。',button('确认','','primary')+button('取消','','secondary')),('dangerous','alert','不可逆','覆盖同名导出文件','原文件将被替换，此动作无法撤销。',button('确认覆盖','','danger')+button('取消','','secondary'))])
    inputs=''.join(f'<div class="input-spec"><h3>{label}</h3>{composer(mode)}</div>' for mode,label in [('default','默认'),('focus','键盘焦点'),('disabled','装载期间禁用'),('busy','Vit 回复或执行中')])
    s5=panel('S5',heading('S5','组件与全部状态','同一个控件体系覆盖选择、试听、执行结果与恢复。')+'<div class="component-grid">'+macros+'</div><h2>执行状态卡</h2><div class="board-columns status-spec"><div>'+status_card(compact=True)+'</div><div>'+status_card('error')+'<div style="margin-top:24px">'+status_card('done')+'</div></div></div><h2>动作后果</h2><div class="board-columns three">'+actions+'</div><h2>消息与快照回执</h2><div class="message-demo"><p class="message user-message">让鼓组暖一点，保留底鼓的冲击。</p><p class="message">我保留了底鼓的起音，让高频更柔和。</p><div class="receipt">'+icon('snapshot',16)+'<span>已保存快照 <time>14:07</time> ·</span>'+button('撤销','','text-button')+'</div></div><p class="caption" style="margin-top:16px">用户消息仅右对齐，Vit 消息左对齐，不叠加色块徽章。时间戳仅在跨时段时出现，快照只保留一行。</p><h2>输入框</h2><div class="board-columns">'+inputs+'</div><div class="pending-note"><h3>待用户选择 · A/B 的启动方式</h3><p>选项 A：渲染好后，用户点击「对比 A/B」。选项 B：用户预先确认，渲染好后开始试听。P3 停在渲染中，原型不触发任何音频播放；执行前再确定此项。</p></div>')

    events=[('safe',72),('safe',40),('attention',48),('safe',96),('dangerous',32),('safe',64)]
    def event_marks(scale=1):
        return ''.join(f'<i class="history-event {cls}" style="width:{int(w*scale)}px" aria-label="{dict(safe="可撤销改动",attention="需确认改动",dangerous="高警示事件")[cls]}"></i>' for cls,w in events)
    s6=panel('S6',heading('S6','改动历史网格实验','《百老汇爬梯舞》的节奏进入事件排列，网格线保持中性。')+'<div class="history-test"><h3>A　单行时间带</h3><p class="secondary-copy">每一段对应一次改动，宽度表示持续时间，颜色表示该事件的关注等级。</p><div class="history-view">'+event_marks()+'</div><p class="secondary-copy">优点：最接近真实历史组件。代价：需要时间与筛选交互，不能仅靠几何块承担记录。</p></div><div class="history-test" style="margin-top:24px"><h3>B　中性网格上的事件</h3><p class="secondary-copy">浅灰正交网格承载少量事件，保留音乐节奏的联想。</p><div class="history-view grid-view">'+event_marks(.8)+'</div><p class="secondary-copy">优点：节奏鲜明。代价：网格本身不增加当前对话任务的信息，容易变成装饰。</p></div><div class="history-test" style="margin-top:24px"><h3>C　按对象分行</h3><p class="secondary-copy">将事件按轨道或总线分行，对比多对象上的连续改动。</p><div class="history-view history-rows">'+''.join(f'<i class="history-event {"attention" if i==5 else ""}" style="grid-column:{(i%4)*2+1}/span 2;grid-row:{i//4+1}"></i>' for i in range(12))+'</div><p class="secondary-copy">优点：有跨对象观察价值。代价：密度大，需要独立历史入口与真实数据。</p></div><div class="pending-note"><h3>评估结论 / 待裁定</h3><p>推荐保留 A 作为未来“改动历史”的候选；B、C 暂不进入主窗口。P3 已有快照与执行轨迹，当前加入历史条会重复表达。本轮三案都留在规格板，不进入产品屏。</p></div>')

    notes=[('done','完成','C6 → E6','1046.50 → 1318.51 Hz','0–130ms / 165–315ms'),('attention','需要确认','D6 → D6','1174.66 → 1174.66 Hz','0–110ms / 170–280ms'),('error','出错','E5 → C5','659.25 → 523.25 Hz','0–140ms / 185–345ms')]
    sound_rows=''.join(f'<tr><td>{label}</td><td>{pitch}</td><td class="mono-value">{freq}</td><td class="mono-value">{times}</td><td>{button("试听","play","",f"data-sound=\"{state}\"")}</td></tr>' for state,label,pitch,freq,times in notes)
    s7=panel('S7',heading('S7','通知与提示音','只在对话窗不在前台时通知；原型提示音仅由点击试听触发。')+'<div class="board-columns three">'+''.join(f'<div><h3>{label}</h3><div class="notification-spec">{notification(state,True)}</div></div>' for state,label in [('done','完成'),('attention','需要确认'),('error','出错')])+'</div><p class="secondary-copy" style="margin-top:24px">通知本体召回原窗口并定位到对应消息；最多两个动作。尊重系统勿扰，Vit 设置可关闭提示音。上方是系统通知版式样张，浏览器没有申请或发送真实 OS 通知。</p><h2>提示音试听</h2><table class="board-table sound-table"><thead><tr><th>类型</th><th>音程 / 音高</th><th>频率</th><th>两音时段</th><th>仅点击播放</th></tr></thead><tbody>'+sound_rows+'</tbody></table><div class="sound-controls">'+button('投影静音','stop','','id="mute-sounds" aria-pressed="false"')+'<span class="secondary-copy">所有试听均通过同一个主音量开关。</span></div><p id="sound-feedback" class="sound-feedback" aria-live="polite">点击试听后才生成声音。</p><div class="board-columns" style="margin-top:24px"><div class="spec-box"><h3>音色与包络</h3><p class="secondary-copy">正弦波；Attack 12ms / Release 45ms；每音峰值增益 0.035，约 −29dBFS。每组时长 315 / 280 / 345ms，均少于 400ms。两音不重叠，干净、无混响。</p></div><div class="spec-box"><h3>实施校准</h3><p class="secondary-copy">原型音量为固定较低增益，不能声称已低于每台机器的系统默认提示音。执行阶段在目标设备上校准相对响度，并验证勿扰、通知权限与音频输出路径。</p></div></div><div class="review-decisions"><div class="pending-note"><h3>待选 · A/B 开始方式</h3><p>手动开始 / 预先确认后开始，见 S5。未定前不建立自动播放契约。</p></div><div class="pending-note"><h3>待选 · 历史组件</h3><p>S6 的 A / B / C 均为实验；推荐 A，当前不进产品屏。</p></div></div>')

    script=r'''
const rgb=h=>h.match(/[a-f\d]{2}/gi).map(v=>parseInt(v,16));
const lum=c=>c.map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4}).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
document.querySelectorAll('.contrast-pair').forEach(el=>{const a=lum(rgb(el.dataset.a)),b=lum(rgb(el.dataset.b));el.textContent=((Math.max(a,b)+.05)/(Math.min(a,b)+.05)).toFixed(2)+':1'});
const soundSpecs={done:{frequencies:[1046.5,1318.51],starts:[0,.165],lengths:[.130,.150]},attention:{frequencies:[1174.66,1174.66],starts:[0,.170],lengths:[.110,.110]},error:{frequencies:[659.25,523.25],starts:[0,.185],lengths:[.140,.160]}};
window.prototypeSoundLog=[];window.soundSpecs=soundSpecs;let audioContext=null,masterGain=null,muted=false,activeSoundNodes=[];
document.querySelector('#mute-sounds').addEventListener('click',()=>{muted=!muted;document.querySelector('#mute-sounds').setAttribute('aria-pressed',String(muted));if(masterGain)masterGain.gain.setValueAtTime(muted?0:1,audioContext.currentTime);document.querySelector('#sound-feedback').textContent=muted?'投影静音已开启。':'投影静音已关闭。'});
document.querySelectorAll('[data-sound]').forEach(button=>button.addEventListener('click',async()=>{
 const kind=button.dataset.sound;if(muted){document.querySelector('#sound-feedback').textContent='投影静音已开启，本次未播放。';return}
 try{if(!audioContext){audioContext=new (window.AudioContext||window.webkitAudioContext)();masterGain=audioContext.createGain();masterGain.gain.value=1;masterGain.connect(audioContext.destination)}
 await audioContext.resume();activeSoundNodes.forEach(node=>{try{node.stop()}catch{}});activeSoundNodes=[];const spec=soundSpecs[kind],now=audioContext.currentTime+.015;
 spec.frequencies.forEach((frequency,i)=>{const oscillator=audioContext.createOscillator(),gain=audioContext.createGain(),start=now+spec.starts[i],end=start+spec.lengths[i];oscillator.type='sine';oscillator.frequency.value=frequency;gain.gain.setValueAtTime(0,start);gain.gain.linearRampToValueAtTime(.035,start+.012);gain.gain.setValueAtTime(.035,end-.045);gain.gain.linearRampToValueAtTime(0,end);oscillator.connect(gain);gain.connect(masterGain);oscillator.start(start);oscillator.stop(end+.005);activeSoundNodes.push(oscillator)});
 window.prototypeSoundLog.push({kind,trigger:'click',peakGain:.035,duration:Math.max(...spec.starts.map((s,i)=>s+spec.lengths[i]))});document.querySelector('#sound-feedback').textContent='已试听'+{done:'完成',attention:'需要确认',error:'出错'}[kind]+'提示音。';
 }catch(error){document.querySelector('#sound-feedback').textContent='当前浏览器无法播放，请检查音频输出。';window.prototypeSoundLog.push({kind,error:String(error)})}
}));
'''
    (ROOT/'v5-system.html').write_text(document('Vit 主窗口 · 设计系统', '<main class="system-board">'+s4+s3+s1+s2+s5+s6+s7+'</main>',SYSTEM_CSS,script),encoding='utf-8')

if __name__ == '__main__':
    product()
    system()
