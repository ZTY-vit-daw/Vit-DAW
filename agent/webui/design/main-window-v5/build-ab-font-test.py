"""Quick check of the neat AB letterforms before integrating the final sprite."""
from pathlib import Path

ROOT = Path(__file__).resolve().parent
NI = 'viewBox="0 0 24 24"'

AB = '''
<rect x="2" y="5" width="10" height="14" fill="#FFFFFF"/>
<rect x="12" y="5" width="10" height="14" fill="#141414"/>
<path d="M7 5.6L11.1 18.4H8.75L8.2 16H5.8L5.25 18.4H2.9Z M7 8.6L7.98 13.4H6.02Z" fill="#141414" fill-rule="evenodd" stroke="none"/>
<path d="M12.9 5.6H17.3C22.2 5.6 22.2 11.8 17.3 11.8C22.7 12.2 22.7 18.4 17.3 18.4H12.9Z M15.2 7.6H16.9C19 7.6 19 10.9 16.9 10.9H15.2Z M15.2 12.7H17.1C19.6 12.7 19.6 16.4 17.1 16.4H15.2Z" fill="#FFFFFF" fill-rule="evenodd" stroke="none"/>
<rect x="2" y="5" width="20" height="14" fill="none" stroke="#141414" stroke-width="0.8"/>
<path d="M12 5V19" fill="none" stroke="#141414" stroke-width="0.8"/>
'''

CSS = '''
body{margin:0;width:1000px;background:#E4E4E4;font:14px/1.5 "Segoe UI","Microsoft YaHei",sans-serif;color:#141414}
.page{width:1000px;padding:40px;background:#fff}
h1{font-size:22px;margin:0 0 8px}p{color:#555;margin:0 0 24px}
.row{display:flex;align-items:end;gap:40px;margin:24px 0;padding:20px;border:1px solid #D3D3D3}
.sz{display:flex;flex-direction:column;align-items:center;gap:6px}
.sz b{font:11px/1.2 Consolas,monospace;font-weight:400;color:#666}
.detail{padding:20px;border:1px solid #D3D3D3;background:#FAFAFA;margin-top:20px}
'''

def svg(size):
    return f'<svg {NI} style="width:{size}px;height:{size}px" aria-hidden="true">{AB}</svg>'

html = '<!doctype html><html><head><meta charset="utf-8"><title>AB letterform check</title><style>' + CSS + '</style></head><body><main class="page">' \
       '<h1>AB 字面检查</h1><p>左白底黑 A、右黑底白 B；A 是加粗实心字，靠白色字腔与底部空隙；B 是加宽的白色实心字，靠黑色字腔。看 A/B 是否居中、B 两个碗是否等高。</p>' \
       '<div class="row">' + ''.join(f'<span class="sz">{svg(s)}<b>{s}px</b></span>' for s in (144,96,64,48,32,24,16)) + '</div>' \
       '<div class="detail">' + svg(288) + '</div>' \
       '</main></body></html>'
(ROOT / 'ab-font-test.html').write_text(html, encoding='utf-8')
print('wrote')
