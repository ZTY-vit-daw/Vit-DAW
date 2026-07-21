package vpsforge

// desktopHTML is a small local launcher for isolated witness sessions. Manual
// listening deliberately occurs inside the native witness process while the
// user operates the VST3 GUI. The workbench does not present post-render WAVs
// as a substitute for that concurrent human observation.
const desktopHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Vit VPS Forge - 人工见证</title>
<style>
:root{color-scheme:dark}body{font-family:"Segoe UI","Microsoft YaHei",system-ui,sans-serif;background:#10141b;color:#eff4fb;margin:0}main{max-width:1080px;margin:38px auto;padding:0 24px 48px}.eyebrow{font-size:12px;font-weight:700;letter-spacing:.12em;color:#74b6ff;margin:0 0 8px}h1{font-size:30px;margin:0 0 10px}h2{font-size:19px;margin:0}.muted{color:#aebdce;line-height:1.65}.panel{background:#1a212c;border:1px solid #344154;border-radius:14px;padding:22px;margin:16px 0}.steps{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}.step{background:#121923;border-radius:10px;padding:14px;border:1px solid #2c394b}.step b{display:block;color:#7cc1ff;margin-bottom:5px}.section-head{display:flex;align-items:center;justify-content:space-between;gap:14px;margin-bottom:10px}.button-row{display:flex;gap:8px;flex-wrap:wrap}.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(270px,1fr));gap:12px}.card{background:#121923;border:1px solid #344154;border-radius:10px;padding:15px}.card h3{margin:0 0 7px;font-size:17px}.badge{display:inline-block;border:1px solid #4a6180;border-radius:999px;padding:3px 8px;margin:3px 3px 0 0;color:#b9d8ff;font-size:12px}.path{word-break:break-all;color:#95a7bc;font-size:12px;margin:8px 0}.primary,.secondary{font:inherit;border-radius:8px;border:1px solid #47617e;padding:10px 14px;color:#fff;cursor:pointer}.primary{background:#246bdb}.secondary{background:#273646}.primary:disabled,.secondary:disabled{cursor:not-allowed;opacity:.55}input{font:inherit;border-radius:8px;border:1px solid #435167;padding:10px;background:#10151c;color:#fff;width:100%;box-sizing:border-box}.callout{border-left:4px solid #4aa4ff;background:#12263a;border-radius:7px;padding:12px 14px;line-height:1.65}.warning{border-left-color:#d9a441;background:#2e281b}.status{white-space:pre-wrap;background:#0c1117;border-radius:8px;padding:13px;max-height:280px;overflow:auto;font-size:12px}details{margin-top:14px}summary{cursor:pointer;color:#c9ddf5}@media(max-width:800px){main{margin:20px auto;padding:0 14px}.steps{grid-template-columns:1fr 1fr}.section-head{align-items:flex-start;flex-direction:column}}@media(max-width:520px){.steps{grid-template-columns:1fr}}
</style>
</head>
<body>
<main>
  <p class="eyebrow">STAGING ONLY · HUMAN WITNESS</p>
  <h1>Vit VPS Forge 人工见证工作台</h1>
  <p class="muted">这里不是 DAW，也不会安装或授予任何能力。它只负责选择隔离预检包，并启动独立原生 VST3 见证窗口。</p>

  <section class="panel steps">
    <div class="step"><b>1 · 选择预检包</b><span class="muted">选择本机 staging/preflight 中已完成预检的包。</span></div>
    <div class="step"><b>2 · 打开实时见证</b><span class="muted">启动隔离的原生 VST3 窗口和本机输出监听。</span></div>
    <div class="step"><b>3 · 操作并聆听</b><span class="muted">在同一时间播放、拖动、切换或 A/B 比较。</span></div>
    <div class="step"><b>4 · 回报见证</b><span class="muted">把截图、听感或 unknown 发回对话归档。</span></div>
  </section>

  <section class="panel">
    <div class="section-head"><h2>可用预检包</h2><button class="secondary" onclick="loadPackages()">刷新列表</button></div>
    <p class="muted">只显示本机 staging/preflight 下、且 manifest 状态为 preflight_complete 的包。</p>
    <div id="packages" class="cards"><p class="muted">正在读取预检包…</p></div>
  </section>

  <section id="selected" class="panel" hidden>
    <div class="section-head">
      <div><h2 id="selectedTitle">尚未选择</h2><p id="selectedMeta" class="muted"></p></div>
      <div class="button-row"><button id="witnessButton" class="primary" onclick="openWitness()">打开实时见证 GUI</button></div>
    </div>
    <div class="callout">新窗口中有 <b>Play</b>、<b>Stop</b>、<b>Loop</b>、<b>Host Bypass A/B</b>、<b>Source…</b> 与 <b>Audio Device…</b>。默认加载随 Forge 安装的 Probe Audio，且不自动播放；请先确认输出设备和音量，再手动点击 Play。需要更贴近音乐的素材时，用 Source… 选择本机 WAV/AIFF。</div>
    <p class="muted">所有 GUI 操作都会立即作用在正在播放的同一 VST3 实例上。Host Bypass A/B 只是在宿主内绕过该实例，绝不写入插件自己的 bypass 参数。关闭窗口会丢弃该临时实例，不会修改 VPS Library、Credential、Catalog 或 SPAL。离线渲染 WAV 仍是自动化 observed 证据，但不再被当作人工实时见证。</p>
    <details><summary>查看所选预检包的机器状态</summary><pre id="workspaceStatus" class="status"></pre></details>
  </section>

  <details class="panel">
    <summary>高级：打开一个已有 workspace</summary>
    <p class="muted">通常从上方列表选择即可。只有 staging/preflight 中的工作区可以启动实时见证 GUI。</p>
    <input id="openPath" placeholder="D:\Vit_DAW\VPSForge\staging\preflight\fabfilter-pro-q-3-20260717-r3">
    <button class="secondary" onclick="openWorkspacePath()">打开此包</button>
  </details>

  <section class="panel"><h2>操作反馈</h2><pre id="message" class="status">请选择一个预检包。</pre></section>
</main>
<script>
let selectedWorkspace = '';
function showMessage(value){document.getElementById('message').textContent=typeof value==='string'?value:JSON.stringify(value,null,2)}
async function request(url,method,body){const options={method:method,headers:{}};if(body!==undefined){options.headers['content-type']='application/json';options.body=JSON.stringify(body)}const response=await fetch(url,options);const data=await response.json();if(!response.ok)throw new Error(data.error||response.statusText);return data}
function text(value){return value==null?'':String(value)}
function renderPackages(items){const target=document.getElementById('packages');target.replaceChildren();if(!items.length){const p=document.createElement('p');p.className='muted';p.textContent='没有找到可用的 preflight workspace。';target.appendChild(p);return}for(const item of items){const card=document.createElement('article');card.className='card';const h=document.createElement('h3');h.textContent=text(item.plugin_name);const meta=document.createElement('div');meta.className='muted';meta.textContent=[item.manufacturer,item.format,item.version].filter(Boolean).join(' / ');const badges=document.createElement('div');for(const badge of (item.candidate_badges||[])){const tag=document.createElement('span');tag.className='badge';tag.textContent=badge;badges.appendChild(tag)}const path=document.createElement('div');path.className='path';path.textContent=item.workspace;const button=document.createElement('button');button.className=item.launchable?'primary':'secondary';button.disabled=!item.launchable;button.textContent=item.launchable?'选择此包':'不可启动';button.onclick=()=>selectWorkspace(item.workspace);card.append(h,meta,badges,path,button);target.appendChild(card)}}
async function loadPackages(){try{const response=await request('/api/workspaces','GET');renderPackages(response.workspaces||[]);showMessage('已读取 '+(response.workspaces||[]).length+' 个 staging 预检包。')}catch(error){showMessage({error:error.message})}}
async function selectWorkspace(workspace){try{const status=await request('/api/open','POST',{workspace:workspace});selectedWorkspace=workspace;const identity=status.manifest&&status.manifest.plugin_identity?status.manifest.plugin_identity:{};document.getElementById('selectedTitle').textContent=identity.name||'已选择 workspace';document.getElementById('selectedMeta').textContent=[identity.manufacturer,identity.format,identity.version].filter(Boolean).join(' / ');document.getElementById('workspaceStatus').textContent=JSON.stringify({workspace:status.workspace,validation:status.validation,evidence_count:status.evidence_count,host_adapter:status.manifest&&status.manifest.host_adapter},null,2);document.getElementById('selected').hidden=false;showMessage('已选择：'+(identity.name||workspace)+'。打开实时见证 GUI 后，请在那个原生窗口中手动点击 Play 再操作插件。')}catch(error){showMessage({error:error.message})}}
async function openWorkspacePath(){const value=document.getElementById('openPath').value.trim();if(!value){showMessage('请先输入 workspace 路径。');return}await selectWorkspace(value)}
async function openWitness(){if(!selectedWorkspace){showMessage('请先选择一个预检包。');return}const button=document.getElementById('witnessButton');button.disabled=true;try{const response=await request('/api/witness/open','POST',{workspace:selectedWorkspace});showMessage('已启动独立实时见证窗口，进程 ID：'+response.process_id+'。请切换到新窗口，确认 Audio Device…，再手动点 Play 并操作插件。')}catch(error){showMessage({error:error.message})}finally{button.disabled=false}}
document.addEventListener('DOMContentLoaded',loadPackages);
</script>
</body>
</html>`
