"""Capture the isolated depth study beside the current second-round baseline."""
from pathlib import Path
import ast
import hashlib
import json
import re
import sys
from PIL import Image
from playwright.sync_api import sync_playwright

ROOT=Path(__file__).resolve().parent
RUN=sys.argv[1] if len(sys.argv)>1 else 'round-1'
OUT=ROOT/'audit'/('depth-'+RUN)
OUT.mkdir(parents=True,exist_ok=True)
tree=ast.parse((ROOT/'audit-round2.py').read_text(encoding='utf-8'))
AUDIT=ast.literal_eval(next(n for n in tree.body if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='AUDIT' for t in n.targets)).value)
report={'run':RUN,'viewport':[1440,900],'variants':{},'errors':[]}
asset=(ROOT.parent/'main-window-v3/SPEC.md').read_text(encoding='utf-8').split('```js\n')[1].split('// 气 atmosphere')[0].rstrip()

with sync_playwright() as pw:
    shells=sorted((Path.home()/'AppData/Local/ms-playwright').glob('chromium_headless_shell-*/chrome-headless-shell-win64/chrome-headless-shell.exe'))
    browser=pw.chromium.launch(headless=True,**({'executable_path':str(shells[-1])} if shells else {}))
    for name,path in [('baseline',ROOT/'v5-product.html'),('depth',ROOT/'v5-depth-study.html')]:
        page=browser.new_page(viewport={'width':1440,'height':900},device_scale_factor=1)
        errors=[];requests=[]
        page.on('pageerror',lambda e:errors.append(str(e)))
        page.on('request',lambda r:requests.append(r.url) if not r.url.startswith(('file:','data:')) else None)
        page.goto(path.as_uri());page.evaluate('document.fonts.ready')
        for suffix,selector in [('', '#p3'),('-window','#p3 .vit-window'),('-control','#p3 .macro-card')]:
            target=OUT/f'{name}{suffix}.png'
            page.locator(selector).screenshot(path=str(target),animations='disabled')
            im=Image.open(target);im.resize((im.width//2,im.height//2),Image.Resampling.LANCZOS).save(OUT/f'{name}{suffix}-half.png')
        result=page.locator('#p3').evaluate(AUDIT)
        result.pop('text')
        result['depthStyles']=page.locator('#p3').evaluate('''root=>{
          const names=['.vit-window','.macro-card','.macro-options','.macro-option[aria-checked=true]','.macro-option[aria-checked=false]','.contour'];
          return names.map(selector=>{const e=root.querySelector(selector),s=getComputedStyle(e);return {selector,border:s.borderTopWidth+' '+s.borderTopColor,shadow:s.boxShadow,background:s.backgroundColor,radius:s.borderTopLeftRadius}})
        }''')
        result['composerGap']=page.locator('#p3').evaluate("r=>r.querySelector('.composer').getBoundingClientRect().top-r.querySelector('.comparison').getBoundingClientRect().bottom")
        result['symbolsMissing']=page.evaluate("[...document.querySelectorAll('use')].map(e=>e.getAttribute('href')).filter(h=>!document.querySelector(h))")
        result['pageErrors']=errors;result['network']=requests
        html=path.read_text(encoding='utf-8')
        result['tapeExact']=asset in html
        result['sha256']=hashlib.sha256(path.read_bytes()).hexdigest()
        group=page.locator('#p3 [role=radiogroup]');group.locator('[role=radio]').nth(1).click();group.locator('[role=radio]').nth(1).press('ArrowRight')
        result['selectionWorks']=group.locator('[role=radio]').nth(2).get_attribute('aria-checked')=='true'
        page.emulate_media(reduced_motion='reduce')
        result['animationsUnderReducedMotion']=page.evaluate("[...document.querySelectorAll('*')].filter(e=>getComputedStyle(e).animationName!=='none').length")
        if result['minFont']<13 or result['minContrast']<4.5 or result['clipped'] or result['controlOverflow'] or result['composerGap']<16 or errors or requests or result['symbolsMissing'] or not result['tapeExact'] or not result['selectionWorks'] or result['animationsUnderReducedMotion']:
            report['errors'].append(name+': acceptance failed')
        report['variants'][name]=result
        page.close()
    browser.close()
(OUT/'acceptance.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps({k:{'font':v['minFont'],'contrast':v['minContrast'],'clipped':v['clipped'],'overflow':v['controlOverflow'],'composerGap':v['composerGap']} for k,v in report['variants'].items()},ensure_ascii=False,indent=2))
print('PASS' if not report['errors'] else 'FAIL: '+str(report['errors']))
sys.exit(1 if report['errors'] else 0)
