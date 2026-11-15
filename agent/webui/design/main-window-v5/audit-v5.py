"""Headless Playwright capture and measured acceptance for the v5 design only.

Usage: python audit-v5.py references | round-1 | round-2 [--color | --palettes | --primary | --controls | --layered]
No VitApp/Godot/agent runtime is started or changed.
"""
from pathlib import Path
import hashlib
import json
import re
import sys
from PIL import Image
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent
ROUND = sys.argv[1] if len(sys.argv) > 1 else 'round-2'
COLOR_STUDY = '--color' in sys.argv
PALETTE_STUDY = '--palettes' in sys.argv
PRIMARY_STUDY = '--primary' in sys.argv
CONTROL_STUDY = '--controls' in sys.argv
LAYERED_STUDY = '--layered' in sys.argv
SPATIAL_STUDY = COLOR_STUDY or PALETTE_STUDY or PRIMARY_STUDY
NAMED_PRODUCT = PALETTE_STUDY or PRIMARY_STUDY or CONTROL_STUDY or LAYERED_STUDY
OUT = ROOT / 'audit' / (('layered-' if LAYERED_STUDY else 'controls-' if CONTROL_STUDY else 'primary-' if PRIMARY_STUDY else 'palettes-' if PALETTE_STUDY else 'color-' if COLOR_STUDY else '') + ROUND)
OUT.mkdir(parents=True, exist_ok=True)

with sync_playwright() as pw:
    # Reuse the installed headless shell; no desktop browser or GUI automation.
    shells = sorted((Path.home() / 'AppData/Local/ms-playwright').glob('chromium_headless_shell-*/chrome-headless-shell-win64/chrome-headless-shell.exe'))
    browser = pw.chromium.launch(headless=True, **({'executable_path': str(shells[-1])} if shells else {}))
    page = browser.new_page(viewport={'width': 1440, 'height': 900}, device_scale_factor=1)
    if ROUND == 'references':
        for name in ['v4-1-refined', 'v4-2-refined']:
            page.goto((ROOT.parent / 'main-window-v3' / (name + '.html')).as_uri())
            page.locator('.screen').last.screenshot(path=str(OUT / (name + '.png')))
            im = Image.open(OUT / (name + '.png'))
            im.resize((720, 450), Image.Resampling.LANCZOS).save(OUT / (name + '-half.png'))
        browser.close()
        print('Reference captures complete. Exit 0.')
        sys.exit(0)

    results = {'round': ROUND, 'colorStudy': COLOR_STUDY, 'viewport': [1440, 900], 'files': {}, 'errors': []}
    if COLOR_STUDY:
        results['authorizedRuleChange'] = 'User approved colorful composition with quiet reading surfaces; RGB grades apply to content annotations. Overall neutral >=85% is not required for this separate study.'
    if PALETTE_STUDY:
        results['authorizedRuleChange'] = 'User rejected vivid primary fields and asked for independent alternatives without deferring to earlier colour preferences. These candidates add muted spatial palettes; original RGB content annotation colours and frozen assets are retained.'
    if PRIMARY_STUDY:
        results['authorizedRuleChange'] = 'User asked to preserve explicit red/yellow/blue collision after muted alternatives lost the triad. This independent study concentrates three identifiable hues in one functional component; their spatial uses remain separate from content grades.'
    if CONTROL_STUDY:
        results['authorizedRuleChange'] = 'User settled on monochrome backgrounds and typography, with colour in non-text control graphics and feedback. New neutral surfaces and a bounded component outline replace the previous primary-colour spatial study.'
    if LAYERED_STUDY:
        results['authorizedRuleChange'] = 'User clarified that whole controls may have coloured backgrounds, borders, or content. This study keeps the app shell and typography neutral and gives the target and macro component coloured surfaces, nested boundaries, and restrained relief.'
    symbol_sets = []
    token_sets = []
    targets = [(f'palette-{key}', ['#p3']) for key in 'abc'] if PALETTE_STUDY else [('system', ['#s4', '#s3']), ('product', ['#p3'])]
    if PRIMARY_STUDY:
        targets = [('primary-balance', ['#p3'])]
    if CONTROL_STUDY:
        targets = [('controls', ['#p3'])]
    if LAYERED_STUDY:
        targets = [('layered', ['#p3'])]
    for name, selectors in targets:
        is_product = name != 'system'
        page = browser.new_page(viewport={'width': 1440, 'height': 900}, device_scale_factor=1)
        path = ROOT / ('v5-' + name + ('-color' if COLOR_STUDY else '') + '.html')
        errors = []
        page.on('pageerror', lambda error: errors.append(str(error)))
        external = []
        page.on('request', lambda request: external.append(request.url) if not request.url.startswith(('file:', 'data:')) else None)
        page.goto(path.as_uri())
        page.evaluate('document.fonts.ready')
        page.screenshot(path=str(OUT / (name + '-full.png')), full_page=True, animations='disabled')
        for selector in selectors:
            stem = name if NAMED_PRODUCT else selector[1:]
            page.locator(selector).screenshot(path=str(OUT / (stem + '.png')), animations='disabled')
            im = Image.open(OUT / (stem + '.png'))
            im.resize((im.width // 2, im.height // 2), Image.Resampling.LANCZOS).save(OUT / (stem + '-half.png'))
        if NAMED_PRODUCT:
            page.locator('.vit-window').screenshot(path=str(OUT / (name+'-window.png')), animations='disabled')
            if LAYERED_STUDY:
                page.locator('.macro-card').screenshot(path=str(OUT / 'macro-detail.png'), animations='disabled')
        info = page.evaluate('''() => {
          const all = [...document.querySelectorAll('body *')];
          const texts = all.filter(el => !el.closest('svg,script,style') && [...el.childNodes].some(n => n.nodeType === 3 && n.textContent.trim()));
          const clipping = [];
          const color = s => (s.match(/[\\d.]+/g) || []).map(Number);
          const over = (fg,bg) => {const a=fg[3]??1;return [0,1,2].map(i=>fg[i]*a+bg[i]*(1-a)).concat(1);};
          const bgFor = el => {const stack=[];for(let p=el;p;p=p.parentElement)stack.unshift(p);return stack.reduce((v,e)=>over(color(getComputedStyle(e).backgroundColor),v),[246,244,236,1]);};
          const lum = c => c.slice(0,3).map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4;}).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
          const contrasts = texts.map(el => {
            const st=getComputedStyle(el);let bg=bgFor(el),fg=over(color(st.color),bg);
            // P3 has one real sibling scrim above the complete DAW window.
            if(el.closest('.daw')){const scrim=color(getComputedStyle(document.querySelector('.daw-scrim')).backgroundColor);bg=over(scrim,bg);fg=over(scrim,fg);}
            const a=lum(fg),b=lum(bg),ratio=(Math.max(a,b)+.05)/(Math.min(a,b)+.05);
            return {text:el.textContent.trim().slice(0,80),ratio:+ratio.toFixed(3),size:parseFloat(st.fontSize),foreground:fg.slice(0,3),background:bg.slice(0,3)};
          });
          for (const el of texts) {
            const range=document.createRange();range.selectNodeContents(el);
            const r=range.getBoundingClientRect();
            for(let p=el;p && p!==document.body;p=p.parentElement){
              const st=getComputedStyle(p),b=p.getBoundingClientRect();
              if((['hidden','clip'].includes(st.overflowX) && (r.left < b.left-1 || r.right > b.right+1)) || (['hidden','clip'].includes(st.overflowY) && (r.top < b.top-1 || r.bottom > b.bottom+1))){clipping.push(el.textContent.trim().slice(0,80));break;}
            }
          }
          return {minFont:Math.min(...contrasts.map(x=>x.size)),minContrast:Math.min(...contrasts.map(x=>x.ratio)),contrastFailures:contrasts.filter(x=>x.ratio<4.5),contrasts,clipping,
            symbols:[...document.querySelectorAll('symbol')].map(x=>x.id),
            missingIcons:[...document.querySelectorAll('use')].map(x=>x.getAttribute('href')).filter(h=>!document.querySelector(h)),
            statusBlocks:[...document.querySelectorAll('#p3 [data-state-color]')].map(x=>({state:x.dataset.stateColor,meaning:x.getAttribute('aria-label')||x.textContent.trim()})),
            spatialBlocks:[...document.querySelectorAll('#p3 [data-spatial-color]')].map(x=>({color:x.dataset.spatialColor,cls:x.className})),
            contentAnnotations:[...document.querySelectorAll('#p3 [data-info-level]')].map(x=>({level:x.dataset.infoLevel,text:x.textContent.trim()})),
            sections:[...document.querySelectorAll('main>section,body>section')].map(x=>x.id),
            bodyText:document.body.innerText,
            tokenContrasts:window.tokenContrasts||null,
            windowRatio:document.querySelector('.vit-window') ? document.querySelector('.vit-window').getBoundingClientRect().width/1440 : null,
            dialogFont:[...document.querySelectorAll('.message')].map(x=>getComputedStyle(x).fontSize),
            radii:[...document.querySelectorAll('#p3 *')].filter(x=>!x.closest('svg')&&getComputedStyle(x).borderTopLeftRadius!=='0px').map(x=>({tag:x.tagName,cls:x.className,radius:getComputedStyle(x).borderTopLeftRadius})),
            heavyBorders:[...document.querySelectorAll('#p3 *')].filter(x=>!x.closest('svg')).flatMap(x=>['Top','Right','Bottom','Left'].filter(side=>getComputedStyle(x)['border'+side+'Width']==='2px' && getComputedStyle(x)['border'+side+'Color']==='rgb(9, 11, 14)').map(side=>({cls:x.className,side}))) };
        }''')
        if is_product:
            bad = r'对话即工作台|AI-FIRST|数字音频工作台|搭档在线|开台白|第\s*\d+\s*期|本期|栏目|图\s*1|注\s*1|上期批注|一钮多参|不暴露原始数值|对话流内状态控件|宏快照语义|Z-TOP|Z-BOTTOM|打开态|always-on-top|TAPE LIST|STATE|STORYBOARD|PROOF SHEET'
            info['forbiddenWords'] = re.findall(bad, info.pop('bodyText'), re.I)
            im = Image.open(OUT / ((name if NAMED_PRODUCT else 'p3')+'.png')).convert('RGB')
            pixels = im.get_flattened_data() if hasattr(im,'get_flattened_data') else im.getdata()
            colorful = sum(1 for r,g,b in pixels if max(r,g,b)-min(r,g,b)>55 and (b>r*1.25 or r>g*1.35 or (r>180 and g>140 and b<100)))
            info['neutralPixelFraction'] = round(1-colorful/(im.width*im.height),6)
            if SPATIAL_STUDY or CONTROL_STUDY or LAYERED_STUDY:
                box = page.locator('.vit-window').bounding_box()
                window_image = im.crop((int(box['x']),int(box['y']),int(box['x']+box['width']),int(box['y']+box['height'])))
                pixels = window_image.get_flattened_data() if hasattr(window_image,'get_flattened_data') else window_image.getdata()
                colorful_window = sum(1 for r,g,b in pixels if max(r,g,b)-min(r,g,b)>55 and (b>r*1.25 or r>g*1.35 or (r>180 and g>140 and b<100)))
                info['windowColorPixelFraction'] = round(colorful_window/(window_image.width*window_image.height),6)
                if NAMED_PRODUCT:
                    info['quietReadingSurface'] = page.evaluate("""() => {
                        const probe=document.createElement('span');
                        probe.style.color=getComputedStyle(document.documentElement).getPropertyValue('--read-bg');
                        document.body.append(probe);
                        const expected=getComputedStyle(probe).color;probe.remove();
                        return getComputedStyle(document.querySelector('.conversation')).backgroundColor==='rgba(0, 0, 0, 0)' && getComputedStyle(document.querySelector('.vit-window')).backgroundColor===expected;
                    }""")
                else:
                    info['quietReadingSurface'] = page.evaluate("getComputedStyle(document.querySelector('.conversation')).backgroundColor==='rgba(0, 0, 0, 0)' && getComputedStyle(document.querySelector('.vit-window')).backgroundColor==='rgb(246, 244, 236)'")
                info['attentionLabelSeparate'] = page.locator('[data-spatial-color][data-info-level]').count() == 0
                if CONTROL_STUDY or LAYERED_STUDY:
                    info['controlBoundaries'] = page.evaluate("""() => {
                        const chromatic=s=>{const c=(s.match(/[\\d.]+/g)||[]).slice(0,3).map(Number);return Math.max(...c)-Math.min(...c)>3};
                        const textElements=[...document.querySelectorAll('#p3 *')].filter(e=>!e.closest('svg,script,style')&&[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim()));
                        const surfaces=['.vit-window','.titlebar','.project-rail','.project-context','.composer'];
                        const controlSurfaces=['.context-target','.macro-card','.macro-actions',...Array.from({length:3},(_,i)=>`.macro-option:nth-child(${i+1})`)];
                        const rgb=s=>(s.match(/[\\d.]+/g)||[]).slice(0,3).map(Number);
                        const luminance=s=>rgb(s).map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4}).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
                        const graphicContrast=[...document.querySelectorAll('.response')].map(e=>{const a=luminance(getComputedStyle(e).stroke),b=luminance(getComputedStyle(e.closest('.contour')).backgroundColor);return +((Math.max(a,b)+.05)/(Math.min(a,b)+.05)).toFixed(3)});
                        const contrast=(a,b)=>{a=luminance(a);b=luminance(b);return +((Math.max(a,b)+.05)/(Math.min(a,b)+.05)).toFixed(3)};
                        const panel=getComputedStyle(document.querySelector('.macro-card')),selected=getComputedStyle(document.querySelector('[aria-checked=true]'));
                        return {colouredTypography:textElements.filter(e=>chromatic(getComputedStyle(e).color)).map(e=>e.textContent.trim()),
                          colouredSurfaces:surfaces.filter(s=>chromatic(getComputedStyle(document.querySelector(s)).backgroundColor)),
                          colouredControlSurfaces:controlSurfaces.filter(s=>document.querySelector(s)&&chromatic(getComputedStyle(document.querySelector(s)).backgroundColor)),
                          controlSurfaceColours:controlSurfaces.filter(s=>document.querySelector(s)).map(s=>({selector:s,background:getComputedStyle(document.querySelector(s)).backgroundColor,border:getComputedStyle(document.querySelector(s)).borderTopColor})),
                          selectedBorder:getComputedStyle(document.querySelector('[aria-checked=true]')).borderTopWidth,
                          componentBorder:getComputedStyle(document.querySelector('.macro-card')).borderTopWidth,
                          displayCount:document.querySelectorAll('.contour').length,
                          curveColours:[...document.querySelectorAll('.response')].map(e=>getComputedStyle(e).stroke),
                          graphicContrast,
                          outerBoundaryContrast:contrast(panel.borderTopColor,getComputedStyle(document.querySelector('.vit-window')).backgroundColor),
                          selectedBoundaryContrast:contrast(selected.borderTopColor,selected.backgroundColor),
                          composerGap:document.querySelector('.composer').getBoundingClientRect().top-document.querySelector('.comparison').getBoundingClientRect().bottom};
                    }""")
                if PRIMARY_STUDY:
                    info['primaryJunction'] = page.evaluate("""() => {
                        const red=document.querySelector('.macro-object'),blue=document.querySelector('.macro-tool'),yellow=document.querySelector('.macro-option[aria-checked=true]');
                        const r=red.getBoundingClientRect(),b=blue.getBoundingClientRect(),y=yellow.getBoundingClientRect();
                        return {red:getComputedStyle(red).backgroundColor,blue:getComputedStyle(document.querySelector('.macro-card')).backgroundColor,yellow:getComputedStyle(yellow).backgroundColor,
                          redYellowAdjacent:Math.abs(r.bottom-y.top)<1 && Math.abs(r.left-y.left)<1 && Math.abs(r.right-y.right)<1,
                          redBlueAdjacent:Math.abs(r.right-b.left)<1 && Math.abs(r.top-b.top)<1 && Math.abs(r.bottom-b.bottom)<1};
                    }""")
            page.emulate_media(reduced_motion='reduce')
            info['reducedMotionAnimations'] = page.evaluate("[...document.querySelectorAll('*')].filter(e=>getComputedStyle(e).animationName!=='none').length")
        else:
            info.pop('bodyText')
            info['iconSpecimenCount'] = page.locator('.icon-sample').count()
        html = path.read_text(encoding='utf-8')
        symbol_sets.append(re.search(r'<svg class="sprite".*?</svg>', html, re.S).group(0))
        token_sets.append(re.search(r':root\{.*?\n\}', html, re.S).group(0))
        source = (ROOT.parent / 'main-window-v3' / 'SPEC.md').read_text(encoding='utf-8').split('```js\n')[1].split('// 气 atmosphere')[0].rstrip()
        info['tapeExact'] = source in html
        expected = {'ink':'#090B0E','paper':'#F6F4EC','warm':'#FBF9F1','canvas':'#E8E4D7','blue':'#065BC7','blue-lt':'#73B8F5','red':'#DB1214','yellow':'#FACC1A','dark':'#14181C'}
        info['anchoredTokensExact'] = all(re.search('--'+re.escape(k)+r'\s*:\s*'+v+r'\s*;',html,re.I) for k,v in expected.items())
        info['svgInstances'] = page.locator('svg.icon').count()
        info['externalReferences'] = re.findall(r'(?:src|href)=[\"\'](?!#|data:)([^\"\']+)', html)
        info['sha256'] = hashlib.sha256(path.read_bytes()).hexdigest()
        info['pageErrors'] = errors
        info['externalRequests'] = external
        results['files'][name] = info
        if info['minFont'] < 13 or info['contrastFailures'] or info['clipping'] or info['missingIcons'] or not info['tapeExact'] or info['externalReferences'] or errors or external:
            results['errors'].append(name + ': measurable acceptance failed')
        if is_product and (info['forbiddenWords'] or len(info['statusBlocks'])+len(info['contentAnnotations'])>5 or (not SPATIAL_STUDY and info['neutralPixelFraction']<.85) or info['reducedMotionAnimations'] or not .58<=info['windowRatio']<=.62):
            results['errors'].append('product: content/geometry/state acceptance failed')
        if is_product and SPATIAL_STUDY and (not info['quietReadingSurface'] or not info['attentionLabelSeparate']):
            results['errors'].append('color study: reading/annotation separation failed')
        if PRIMARY_STUDY and (not info['primaryJunction']['redYellowAdjacent'] or not info['primaryJunction']['redBlueAdjacent']):
            results['errors'].append('primary study: colour junction is separated')
        if CONTROL_STUDY and (info['controlBoundaries']['colouredTypography'] or info['controlBoundaries']['colouredSurfaces'] or info['controlBoundaries']['colouredControlSurfaces'] or info['controlBoundaries']['selectedBorder']!='2px' or info['controlBoundaries']['componentBorder']!='1px' or min(info['controlBoundaries']['graphicContrast'])<3 or info['controlBoundaries']['composerGap']<16):
            results['errors'].append('control study: monochrome typography/surfaces or control boundaries failed')
        if LAYERED_STUDY and (info['controlBoundaries']['colouredTypography'] or info['controlBoundaries']['colouredSurfaces'] or len(info['controlBoundaries']['colouredControlSurfaces'])<3 or info['controlBoundaries']['selectedBorder']!='2px' or info['controlBoundaries']['componentBorder']!='1px' or min(info['controlBoundaries']['graphicContrast'])<3 or info['controlBoundaries']['outerBoundaryContrast']<3 or info['controlBoundaries']['selectedBoundaryContrast']<3 or info['controlBoundaries']['composerGap']<16):
            results['errors'].append('layered study: neutral shell/text, coloured controls, readability or spacing failed')
        if not info['anchoredTokensExact'] or len(info['symbols']) != 21:
            results['errors'].append(name + ': token/icon assets failed')
    results['sharedSpriteExact'] = all(s == symbol_sets[0] for s in symbol_sets)
    results['sharedTokensExact'] = all(s == token_sets[0] for s in token_sets)
    if not results['sharedSpriteExact'] or not results['sharedTokensExact']:
        results['errors'].append('shared assets differ')
    browser.close()

(OUT / 'acceptance.json').write_text(json.dumps(results,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps({k:{'minFont':v['minFont'],'minContrast':v['minContrast'],'clipping':v['clipping'],'failures':v['contrastFailures'],'pageErrors':v['pageErrors']} for k,v in results['files'].items()},ensure_ascii=False,indent=2))
print('PASS' if not results['errors'] else 'FAIL: '+str(results['errors']))
sys.exit(0 if not results['errors'] else 1)
