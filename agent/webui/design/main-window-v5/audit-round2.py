"""Render all four product screens and seven specification panels; audit the prototype."""
from pathlib import Path
import hashlib
import json
import re
import sys
from PIL import Image
from playwright.sync_api import sync_playwright

ROOT=Path(__file__).resolve().parent
RUN=sys.argv[1] if len(sys.argv)>1 else 'round-1'
OUT=ROOT/'audit'/('full-'+RUN)
OUT.mkdir(parents=True,exist_ok=True)

AUDIT=r'''root => {
 const colour=s=>(s.match(/[\d.]+/g)||[]).map(Number);
 const over=(f,b)=>{const a=f[3]??1;return [0,1,2].map(i=>f[i]*a+b[i]*(1-a)).concat(1)};
 const bgFor=e=>{const a=[];for(let p=e;p;p=p.parentElement)a.unshift(p);return a.reduce((b,p)=>over(colour(getComputedStyle(p).backgroundColor),b),[255,255,255,1])};
 const lum=c=>c.slice(0,3).map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4}).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
 const texts=[...root.querySelectorAll('*')].filter(e=>!e.closest('svg,script,style')&&[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim()));
 const contrast=texts.map(e=>{const s=getComputedStyle(e);let bg=bgFor(e),fg=over(colour(s.color),bg);const scrim=e.closest('.daw')&&e.closest('.screen')?.querySelector('.daw-scrim');if(scrim){const c=colour(getComputedStyle(scrim).backgroundColor);fg=over(c,fg);bg=over(c,bg)}const a=lum(fg),b=lum(bg);return {text:e.textContent.trim().slice(0,90),size:parseFloat(s.fontSize),ratio:+((Math.max(a,b)+.05)/(Math.min(a,b)+.05)).toFixed(3)}});
 const clipped=[];
 for(const e of texts){if(e.closest('.daw'))continue;const range=document.createRange();range.selectNodeContents(e);const r=range.getBoundingClientRect();for(let p=e;p&&p!==root.parentElement;p=p.parentElement){const s=getComputedStyle(p),b=p.getBoundingClientRect();if((['hidden','clip'].includes(s.overflowX)&&(r.left<b.left-1||r.right>b.right+1))||(['hidden','clip'].includes(s.overflowY)&&(r.top<b.top-1||r.bottom>b.bottom+1))){clipped.push(e.textContent.trim().slice(0,80));break}}}
 const overflow=[...root.querySelectorAll('button,textarea')].filter(e=>e.scrollWidth>e.clientWidth+2||e.scrollHeight>e.clientHeight+3).map(e=>({text:(e.textContent||e.getAttribute('aria-label')).trim(),width:[e.clientWidth,e.scrollWidth],height:[e.clientHeight,e.scrollHeight]}));
 const graphicOverflow=[...root.querySelectorAll('.history-event')].filter(e=>{const r=e.getBoundingClientRect(),b=e.closest('.history-view').getBoundingClientRect();return r.left<b.left||r.right>b.right||r.top<b.top||r.bottom>b.bottom}).length;
 return {text:root.innerText,minFont:Math.min(...contrast.map(c=>c.size)),minContrast:Math.min(...contrast.map(c=>c.ratio)),contrastFailures:contrast.filter(c=>c.ratio<4.5),clipped,controlOverflow:overflow,graphicOverflow,contrast,
 macroVisualizations:[...new Set([...root.querySelectorAll('[data-visualization]')].map(e=>e.dataset.visualization))],
 macroOptions:root.querySelectorAll('.macro-option').length,
 macroRedCurves:[...root.querySelectorAll('.macro-card:not(.macro-failed) .response')].filter(e=>getComputedStyle(e).stroke==='rgb(219, 81, 70)').length,
 screenDimensions:{width:root.getBoundingClientRect().width,height:root.getBoundingClientRect().height},
 uiTextColours:[...new Set(texts.filter(e=>!e.closest('.daw')).map(e=>getComputedStyle(e).color))]};
}'''

results={'run':RUN,'viewport':[1440,900],'screens':{},'panels':{},'errors':[]}
forbidden=r'对话即工作台|AI-FIRST|数字音频工作台|搭档在线|开台白|第\s*\d+\s*期|本期|栏目|图\s*1|注\s*1|上期批注|一钮多参|不暴露原始数值|对话流内状态控件|宏快照语义|Z-TOP|Z-BOTTOM|打开态|always-on-top|TAPE LIST|STATE|STORYBOARD|PROOF SHEET'
asset=(ROOT.parent/'main-window-v3/SPEC.md').read_text(encoding='utf-8').split('```js\n')[1].split('// 气 atmosphere')[0].rstrip()
with sync_playwright() as pw:
    shells=sorted((Path.home()/'AppData/Local/ms-playwright').glob('chromium_headless_shell-*/chrome-headless-shell-win64/chrome-headless-shell.exe'))
    browser=pw.chromium.launch(headless=True,**({'executable_path':str(shells[-1])} if shells else {}))
    sprites=[]
    for kind,ids in [('product',['p1','p2','p3','p4']),('system',['s4','s3','s1','s2','s5','s6','s7'])]:
        page=browser.new_page(viewport={'width':1440,'height':900},device_scale_factor=1)
        errors=[];network=[]
        page.on('pageerror',lambda e:errors.append(str(e)))
        page.on('request',lambda r:network.append(r.url) if not r.url.startswith(('file:','data:')) else None)
        path=ROOT/f'v5-{kind}.html';html=path.read_text(encoding='utf-8')
        page.goto(path.as_uri());page.evaluate('document.fonts.ready')
        page.screenshot(path=str(OUT/f'{kind}-full.png'),full_page=True,animations='disabled')
        for id in ids:
            loc=page.locator('#'+id)
            loc.screenshot(path=str(OUT/f'{id}.png'),animations='disabled')
            im=Image.open(OUT/f'{id}.png');im.resize((im.width//2,im.height//2),Image.Resampling.LANCZOS).save(OUT/f'{id}-half.png')
            v=loc.evaluate(AUDIT)
            if kind=='product':
                v['forbiddenWords']=re.findall(forbidden,v.pop('text'),re.I)
                if v['forbiddenWords'] or v['screenDimensions']!={'width':1440,'height':900} or v['macroRedCurves']:
                    results['errors'].append(id+': product words/geometry/colour')
            else:v.pop('text')
            if v['minFont']<13 or v['minContrast']<4.5 or v['clipped'] or v['controlOverflow'] or v['graphicOverflow']:
                results['errors'].append(id+': text/readability/overflow')
            results['screens' if kind=='product' else 'panels'][id]=v
        checks={'tapeExact':asset in html,'symbols':page.locator('symbol').count(),'missingSymbols':page.evaluate("[...document.querySelectorAll('use')].map(e=>e.getAttribute('href')).filter(h=>!document.querySelector(h))"),'network':network,'pageErrors':errors,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
        sprites.append(re.search(r'<svg class="sprite".*?</svg>',html,re.S).group(0))
        page.emulate_media(reduced_motion='reduce')
        checks['reducedMotionAnimations']=page.evaluate("[...document.querySelectorAll('*')].filter(e=>getComputedStyle(e).animationName!=='none').length")
        checks['externalRefs']=re.findall(r'(?:src|href)=[\"\'](?!#|data:)([^\"\']+)',html)
        if not checks['tapeExact'] or checks['symbols']!=23 or checks['missingSymbols'] or network or errors or checks['reducedMotionAnimations'] or checks['externalRefs']:
            results['errors'].append(kind+': assets/runtime')
        if kind=='product':
            checks['p2InputDisabled']=page.locator('#p2 textarea').is_disabled()
            checks['p4VitWindowCount']=page.locator('#p4 .vit-window').count()
            checks['p4NotificationActions']=page.locator('#p4 .notification button').count()
            checks['p3WindowRatio']=page.locator('#p3 .vit-window').bounding_box()['width']/1440
            group=page.locator('#p3 [role=radiogroup]')
            group.locator('[role=radio]').nth(1).click()
            checks['radioClick']=group.locator('[role=radio]').nth(1).get_attribute('aria-checked')=='true'
            group.locator('[role=radio]').nth(1).press('ArrowRight')
            checks['radioKeyboard']=group.locator('[role=radio]').nth(2).get_attribute('aria-checked')=='true'
            checks['selectionDoesNotClaimApplied']='已选择' in page.locator('#p3 .applied').inner_text()
            checks['p3ComposerGap']=page.locator('#p3').evaluate("e=>e.querySelector('.composer').getBoundingClientRect().top-e.querySelector('.comparison').getBoundingClientRect().bottom")
            if not checks['p2InputDisabled'] or checks['p4VitWindowCount'] or checks['p4NotificationActions']>2 or not checks['radioClick'] or not checks['radioKeyboard'] or not checks['selectionDoesNotClaimApplied'] or checks['p3ComposerGap']<16:
                results['errors'].append('product: interactions/state/spacing')
        else:
            checks['iconSpecimens']=page.locator('.icon-sample').count()
            checks['noNanContrasts']=page.locator('.contrast-pair').evaluate_all("es=>es.every(e=>/^\\d+\\.\\d{2}:1$/.test(e.textContent))")
            checks['auditionCancel']=page.locator('.macro-audition button').filter(has_text='取消试听').count()==1
            checks['unselectedApplyDisabled']=page.locator('.macro-default .macro-actions .primary').is_disabled()
            checks['tokensMatchTable']=page.locator('#s4 .board-table tbody tr').evaluate_all("""rows=>rows.every(row=>{const cells=row.querySelectorAll('td'),name=cells[0].textContent.trim(),value=cells[1].textContent.trim();return getComputedStyle(document.documentElement).getPropertyValue(name).trim().toUpperCase()===value.toUpperCase()})""")
            checks['soundLogBeforeClick']=page.evaluate('window.prototypeSoundLog')
            for sound in ['done','attention','error']:
                page.locator(f'[data-sound={sound}]').click()
            page.wait_for_function('window.prototypeSoundLog.length===3')
            checks['soundLogAfterClicks']=page.evaluate('window.prototypeSoundLog')
            page.locator('#mute-sounds').click();page.locator('[data-sound=done]').click()
            checks['mutePreventsPlay']=page.evaluate('window.prototypeSoundLog.length===3')
            checks['soundFeedback']=page.locator('#sound-feedback').inner_text()
            if checks['soundLogBeforeClick'] or not checks['mutePreventsPlay'] or len(checks['soundLogAfterClicks'])!=3 or any('error' in e or e['duration']>.4 for e in checks['soundLogAfterClicks']):
                results['errors'].append('system: audio contract')
            if not checks['noNanContrasts'] or not checks['tokensMatchTable'] or not checks['auditionCancel'] or not checks['unselectedApplyDisabled']:
                results['errors'].append('system: tokens/component states')
        results[kind+'Checks']=checks
        page.close()
    results['sharedSpritesExact']=sprites[0]==sprites[1]
    if not results['sharedSpritesExact']:results['errors'].append('shared sprites differ')
    browser.close()
(OUT/'acceptance.json').write_text(json.dumps(results,ensure_ascii=False,indent=2),encoding='utf-8')
for category in ['screens','panels']:
    print(category, json.dumps({k:{'font':v['minFont'],'contrast':v['minContrast'],'fail':v['contrastFailures'],'clipped':v['clipped'],'overflow':v['controlOverflow']} for k,v in results[category].items()},ensure_ascii=False))
print('PASS' if not results['errors'] else 'FAIL '+str(results['errors']))
sys.exit(1 if results['errors'] else 0)
