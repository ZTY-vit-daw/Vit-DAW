# FE-CEF-UPGRADE-1 七项回归记录（gdcef v1.15.4 → v1.16.2，Mac）

- 执行：2026-10-15 18:28–19:40 +0800，Mac 执行侧接管会话（GLM-5.3，ZCode；接管记录
  coord main f8e56f92）。栈：Godot 4.6.stable.official.89cea1439（本机 Godot.app）+
  前端仓 port/fe-cef-upgrade-1@34072f0 + bin/ 已换 v1.16.2（资产 sha256
  `e8f7a24486e77156862baf5a4433e4b9c9da540fa26205be3fd21e1a027b4117`，1,063,681,432 字节，
  zip 枚举 1,919 文件；universal-apple-darwin 487 文件 drop-in，清单
  BIN_V1162.sha256，CEF 运行时 Chromium 152.0.5.0 → 154.0.26.0）+ 探针页 7879
  （vite preview，KEYINPUT SERVICE.md 模式）。
- 总判定：**升级未通过**——硬门① FAIL（两态各 1 轮 0/5）+ ⑤ FAIL（webui 主链 ipc
  断）；②⑥ PASS；③结构在场（真实 composition 留用户轮）；④主目录兼容+新增 devtools
  旁挂；⑦ WINDOWS_UPGRADE_STEPS.md 已回填实测值与本结论。停止条件（②不可绕）未触发，
  未回滚；前端仓保留 v1.16.2 现场，一键回滚=仓外归档
  `~/Documents/vit-daw-cef-backups/godot_cef-v1.15.4-bin-20261015/`（与替换前逐字节一致已核）。

## 0. 替换过程纪要（含环境事件，如实）

- gdextension：v1.16.2 与我方 v1.15.4 版**不逐字节同构**（上游删 Windows EGL 条目——zip 内
  已无该二文件，自洽；Linux 节展开；我方旧版多出的 macos 导出映射块系 B4A 运输快照携带）。
  已整文件换用上游版；`[libraries].macos` 两版一致（框架路径不变）。
- **provenance 挂起事件**：zip 解包件全带 `com.apple.provenance` xattr（浏览器下载来源，
  macOS 26 darwin 27），首启 dyld 签名校验（mapSegments→fcntl）无限挂起（裸 python
  dlopen 同挂；旧 v1.15.4 tar 解包件 3.11s 通过=对照成立）。`xattr -d` 被 SIP 拒绝。
  处置：整树 `cp -R` 重物化（bit-identical，sha256 清单复核 487/487 一致）+ 用户放行一次
  系统安全弹窗后 dlopen 5.03s 通过。此后全部回归可正常起栈。
- 上轮（被弃执行流）三项 v1.15.4 基线复用：resize/watchdog 完整；webui_chain 基线被
  截断于页面加载（未达 ipc 步，⑤无 v1.15.4 正控——但产品 v1.15.4 Ask Vit 桥在生产使用中
  正常，见 KEYINPUT/FOCUSFIX 底账）。

## 1. 七项回归明细

### ① 焦点面四步序列 ×5 轮（硬门）——FAIL

- OFF 轮：`autodrive_v1162_off.log`，exit 1，rounds_pass=0/5。
- ON 轮：`autodrive_v1162_on.log`，exit 1，rounds_pass=0/5（与 OFF 逐形态一致；无
  compensation.fire——v1.16.2 下合成卡死态不再成立，宿主补偿已无靶点）。
- **#228 修复面生效**：5/5 轮 `synth page_hasFocus=true`（v1.15.4 为 false 卡死态 3s+ 保持）；
  焦点链无乒乓倒手风暴。
- **新失败形态=⌘V 粘贴不送达**：round 1 s1/s2/s3 键入全过，s4 粘贴失败（got="hello1"
  want="hello1PASTE-1"）；round 2-3 同型（PASTE-* 全缺）；round 4-5 另现键入乱序
  （"abc4xy3"/"hellhello5o4" 穿插——v1.15.4 已见的代理缓冲重放族残留）。
- 页面事件对照（探针 [:INPROBE] 镜像）：
  - v1.15.4（autodrive_off_control.log i=116-121）：KeyV keydown×2 → **beforeinput
    insertText "PASTE-1" → input → onChange → commit**（粘贴送达）；
  - v1.16.2（autodrive_v1162_off.log i=112-113）：KeyV keydown×2（key="Unidentified"，
    与旧版同型）→ **无任何 beforeinput/input/paste 事件**。
- 方向性诊断（`paste_diag_v1162.log`，新诊断场景 cef_paste_diag，管线与 autodrive 同源）：
  - T0 godot.clipboard=pass（pbcopy 写入后 Godot 进程回读="PASTE-DIAG"，系统剪贴板在位可读）；
  - T1 js.clipread=NotAllowedError（页面 Clipboard API 无权限——与 ⌘V 原生路径独立，
    v1.15.4 下同样不靠它）；
  - T2 js.insertText=pass（execCommand insertText 正常入 DOM——编辑命令通路健康）；
  - T3/T4 cmdv=fail/fail（两次点击+⌘V，各 6s 轮询窗 DOM 值零变化）。
  - **定位：加速键→粘贴编辑命令的翻译层在加速 OSR 断裂**（keydown 已到页面、剪贴板可读、
    DOM 可编辑，唯 CEF 侧不产生 paste 编辑命令）。非焦点态、非剪贴板权限、非 DOM 编辑。

### ② accelerated OSR（Metal）渲染+resize——PASS

- `resize_v1162.log` exit 0：`[CEFRESIZE] FINAL pass render=pass resize=pass`；三档
  resize（1400x1000→700x560→1100x880）worst_black_streak=0（无闪黑）、视口跟随正常；
  截图 shots_v1162/（initial/resize_0-2/final）。与 v1.15.4 基线（resize_baseline_v1154.log
  同判 pass）行为等价。

### ③ 中文 IME composition 路径——结构在场，真实 composition 留用户轮

- 代理结构在：焦点链 `CefTexture/@LineEdit@6`（隐藏 LineEdit IME 代理仍按上游机制创建/接管）；
- 键入经代理送达页面正常（① s1/s2/s3 round 1-3 全过）；代理文本缓冲乱序缺陷在 round 4-5
  仍现（①日志有案，#236 家族残留形态）；
- 探针页 compositionstart/compositionend 监听器在位（input_probe main.tsx:242-246，
  用户轮可采）；真实 IME composition 需真实 OS/输入法交互（v1.15.4 卡同一边界），
  保留 [等待用户]。

### ④ #231 per-process profile 后 cef-data 落点——主目录兼容 + 新增 devtools 旁挂

- `app_userdata/Vit_DAW_Frontend/cef-data/` 位置不变，今日全部 v1.16.2 运行持续写入同一
  目录（Default/ 等既有结构）——既有会话兼容，未出现 per-pid 目录；
- 新增旁挂 `cef-data-devtools/`（内含 9229.lock，远程调试端口 profile 分离）。

### ⑤ webui 主链（Ask Vit 打开→加载→ipc→关闭重开）——FAIL（ipc 腿）

- `webui_chain_v1162.log` exit 2（abort at first open）：页面加载 pass（title "Ask Vit"、
  http 200）、React 根挂载 pass（root ok:true）、**ipc 闭环 fail**——页面侧
  `window.chrome.webview.postMessage`（shim ok:true，调用发出）但宿主侧
  `CefTexture.ipc_message` 零触发（`web_message_received` 无送达，close/reopen 腿未达）。
- 影响面：产品 webui 的 browserBridge 即 `window.chrome.webview`（App.tsx:8067-8132），
  **宿主↔webui 桥通道在 v1.16.2 断裂=产品主链级回归**（页面装载/渲染本身正常）。
- 边界：v1.15.4 探针基线被截断未达 ipc 步（无同探针正控）；生产 Ask Vit 桥在 v1.15.4
  正常使用（KEYINPUT/FOCUSFIX 底账）。

### ⑥ helper watchdog——PASS

- `watchdog_v1162_run.log` + `watchdog_v1162_sigkill_procs.txt`：5 个 CEF helper
  （85318/85319/85321/85326/85327）在父进程 SIGKILL（19:17:36）后 t+0.5s 起全部消失，
  8s 观察窗零残留。与 v1.15.4 基线行为一致（无孤儿 helper）。

### ⑦ Windows 侧留档——完成（只记录不执行）

- `WINDOWS_UPGRADE_STEPS.md` 已回填：资产 sha256、gdextension 不同构结论与替换建议、
  Mac 侧①⑤失败结论（PC 排期前必读）、provenance/MotW 平台对照注意事项。

## 2. 复现命令（工件可回指）

```bash
# 探针服务（7879；KEYINPUT SERVICE.md 模式）
curl -s --noproxy '*' -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7879/
# ① OFF / ON（前端仓 port/fe-cef-upgrade-1，bin/ = v1.16.2）
cd /Users/timozty/Documents/vit-daw-frontend
VIT_CEF_DEBUG=1 VIT_CEF_FOCUS_LOG=1 VIT_CEF_AUTODRIVE_ROUNDS=5 \
  /Users/timozty/Applications/Godot.app/Contents/MacOS/Godot --path . \
  res://vit_dock/diagnostics/cef_focus_autodrive.tscn 2>&1 | tee <log>   # ON 轮加 VIT_CEF_FOCUS_COMPENSATION=1
# ①附 粘贴方向诊断
VIT_CEF_DEBUG=1 ... res://vit_dock/diagnostics/cef_paste_diag.tscn
# ② resize / ⑤ webui chain（⑤需 vitagent 7878 在线）
VIT_CEF_RESIZE_OUT=<dir> ... res://vit_dock/diagnostics/cef_resize_probe.tscn
VIT_CEF_DEBUG=1 ... res://vit_dock/diagnostics/cef_webui_chain_probe.tscn
```

- 判定行 grep：`CEFAUTODRIVE.*(synth|verdict|FINAL)`、`CEFPASTE`、`CEFRESIZE`、`CEFWEBUI`。
- 工件清单：本目录 autodrive_v1162_{off,on}.log、paste_diag_v1162.log、resize_v1162.log、
  webui_chain_v1162.log、watchdog_v1162_{run.log,procs.txt,sigkill_procs.txt}、
  shots_v1162/、BIN_V1162.sha256、vitagent_7878_run.log、v1.15.4 基线三件（上轮留）。

## 3. 验收结论对照卡面

| 卡面项 | 结果 |
|---|---|
| 升级执行（归档+drop-in+sha256） | 完成（含 provenance 事件处置，见 §0） |
| 硬门①四步×5 轮补偿 OFF 全过 | **未达**（0/5；#228 焦点面修复生效，⌘V 翻译层断裂） |
| 七项全记录 | 达成（本文件） |
| 停止条件②不可绕→回滚 | 未触发（② pass）；未回滚，现场保留 |
| [等待用户] 终验 | 硬门未过，终验暂无意义；待决策侧处置（issue/回滚/换版本）后另排 |

- 下一步（按卡面验收标准）：`UPSTREAM_ISSUE_DRAFT.md`（双卡取证数据包+本卡新证据）
  交决策侧过目；升级线在根修到位前**不放行产品**，前端仓 port/fe-cef-upgrade-1 分支
  保留 v1.16.2 现场+仓外归档可一键回滚。
