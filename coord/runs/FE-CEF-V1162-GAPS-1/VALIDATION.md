# FE-CEF-V1162-GAPS-1 执行记录（⑤ipc 两断点收口 + ①⌘V 垫片，Mac）

- 执行：2026-10-15 21:21–23:30 +0800，Mac 执行侧会话（GLM-5.3，ZCode；领取提交
  coord main 49daa63c）。栈：Godot 4.6.stable.official.89cea1439 + 前端仓
  port/fe-cef-v1162-gaps-1（自 port/fe-cef-upgrade-1@140c27cb 续）+ bin/ v1.16.2
  （本轮曾临时换 v1.15.4 正控后换回，换回后 487/487 sha256 复核 pass，
  BIN_V1162.sha256 清单核对 exit 0）+ 探针服务 7879（上轮存活实例，未动）。
- 前端实现 commit：**13d670e** @ port/fe-cef-v1162-gaps-1（GH001 推送限制照旧，
  补丁=本目录 frontend_13d670e.patch + frontend_commit_13d670e.stat）。

## 总判定

1. **⑤ ipc 桥断=假故障**（探针缺陷造成的假阴性）：ipc 通路在 v1.15.4 与 v1.16.2
   均健康。**无需宿主适配、无需上游修复**——上游 ipc 契约源码（v8_handlers/
   render_process/webrender_ipc/ipc_contract）两 tag 逐字节一致，ClassDB API 面
   diff 仅 +2 个 drag 方法，信号/属性面不变。
2. **① ⌘V 垫片实现并验证**：CefTexture gui_input 捕获 → DisplayServer 剪贴板 →
   eval_js 聚焦可编辑框 insertText。paste_diag v2 shim ON 全过 exit 0；shim OFF
   对照全挂 exit 1（开关有效 + 原缺陷同轮复现）。
3. **硬门（autodrive 四步×5）本轮环境不可执行**：v1.16.2 输入门
   （browser_input_available 需 Window OS key）在本机后台会话无法程序化满足，
   合成点击/键入不达页面（取证见 §3）。→ `[等待用户]` 用户在场轮执行，流程见 §4。

## 0. ⑤ 假故障的证据链

- **根因**：UPGRADE 卡的 cef_webui_chain_probe `_ipc_roundtrip` 按 `t=="ipc"` 匹配
  回环条目，但 `_on_ipc` 入队条目只有 channel/type/nonce 无 t 键——**即使消息送达
  也永不命中**；且原探针 abort 路径不打 bridge_msgs 计数，"零送达"结论无直接证据。
- 修复（13d670e）：匹配改 type+nonce；`_on_ipc` 每条 web_message_received 打
  `[CEFWEBUI] ipc.raw` 原始行（送达直接证据）；js 侧加直连 `window.sendIpcMessage`
  腿并回报返回值（gdcef 契约：true=render 侧已发出）。
- 轮次（均 exit 0，本目录留档）：
  - `webui_chain_7879_v1162_repro.log`（修复前）：ipc={}（缺陷复现）；
  - `webui_chain_7879_v1154_control_fixed.log`（**v1.15.4 正控**，上轮缺失项补齐）：
    ipc.raw×2/轮（shim postMessage 与直连 sendIpcMessage 双路送达），sendrc rc=true，
    first/close/reopen/FINAL 全 pass——v1.15.4 同管线 ipc 闭环正常；
  - `webui_chain_7879_v1162_fixed.log`：v1.16.2 同判全 pass；
  - `webui_chain_7878_v1162_accept.log`（**正式验收轮**，默认 URL=webui 主链
    vitagent 7878）：title "Ask Vit"、root ok、ipc.raw×2/轮、close/reopen 全过，
    FINAL pass exit 0。
- v1.15.4 正控的 bin 换装：归档 ~/Documents/vit-daw-cef-backups/
  godot_cef-v1.15.4-bin-20261015/ 拷入（487/487 对 BIN_ARCHIVE_V1154.sha256 校验
  pass）→ 跑完换回 v1.16.2（487/487 对 BIN_V1162.sha256 校验 pass）。
- ClassDB API 面两版 dump：classdb_v1154.log / classdb_v1162.log——CefTexture
  methods 61→63（+drag_source_ended_for_session/drag_source_system_ended_for_session），
  properties(6)/signals(23) 完全一致，ipc_message(message:4) 不变。
- 上游源码勘验（~/Documents/vit-upstream-recon/godot-cef blobless clone，
  tag v1.15.4 vs v1.16.2）：ipc 全链（OsrIpcHandler V8 注册→
  send_process_message_to_browser→browser 端 on_process_message_received→
  EventQueues→emit ipc_message）逐字节一致；变化仅在 cef_pump（#228 消息泵
  SceneTree 化，rendering 正常证明泵活）、焦点重构、profile 分离——均不影响 ipc。

## 1. ① ⌘V 垫片（13d670e）

- 开关：VitDebugFlags.cef_paste_shim_enabled **默认开**（v1.16.2 下产品粘贴唯一
  通路；env VIT_CEF_PASTE_SHIM=0 / JSON cef_paste_shim_enabled:false 关闭）+
  cef_paste_log_enabled（[VitCefPaste] 行为日志，默认关）。
  ⚠️ 决策侧注意：**默认开**是执行侧判断（不修=产品粘贴坏）；若回滚 v1.15.4
  （原生 paste 恢复）必须关闭，否则双插。
- 实现：vit_cef_web_view_host 挂 CefTexture `gui_input` 信号（Control 内建信号，
  gdcef Rust override gui_input 虚方法不影响信号发射——cef_input_hook_test 页面
  空白/字段聚焦两态实证 press+release 各一跳）；⌘V(keycode V+meta, 非echo) →
  `DisplayServer.clipboard_get()` → eval_js 守卫注入：__vitCefComposing
  （FOCUSFIX 组合态跟踪复用）为 true 拒绝（[VPS] busy）；activeElement 非
  input/textarea/contentEditable 或 readOnly/disabled 拒绝；否则
  `execCommand('insertText')`（文本经 JSON.stringify 转义）。
- 验证（paste_diag v2，7879 探针页）：
  - `paste_diag_v2_full.log`（shim ON）：T0 godot.clipboard pass / T2 insertText
    pass / **T3 focus+⌘V 83ms 送达 / T4 85ms 送达（各自独立剪贴板标记）** /
    **T5 password 框 value=PASTE-PW pass** / **T6 组合守卫 busy（DOM 不变）+
    复位后恢复插入 pass** / FINAL pass exit 0；
  - `paste_diag_v2_shim_off.log`（VIT_CEF_PASTE_SHIM=0 对照）：T3/T5 全挂，
    FINAL fail exit 1——开关生效 + v1.16.2 无垫片确无粘贴（原缺陷复现）；
  - T1 js.clipread NotAllowedError 与前轮一致（非路径，不依赖）。
- 边界如实：T3/T4 聚焦前置=eval focus（原因见 §3 环境限制）；真实点击聚焦→⌘V
  的完整用户路径归 [等待用户] 终验（含真实 IME composition 中的守卫行为）。

## 2. 硬门（autodrive 四步×5 补偿 OFF）——环境阻塞，待用户轮

- `autodrive_v1162_shim_r1.log`：0/5，每轮 synth page_hasFocus=false、键入 got=""、
  shim 零触发（⌘V 由 IME 代理 LineEdit 持焦点，gui_input 不达）。
- 与 UPGRADE 轮（同场景 v1.16.0..2 环境下 s1-s3 曾通过、synth hasFocus=true）的
  差异：该轮全程 `win_focus=true`（OS 窗口 key），本轮所有启动方式均 false。
- 排查矩阵（cef_click_matrix，`/tmp/cef_click_matrix_result.txt`+场景内建日志）：
  - 仅首击达页面（pd=353,133 即校准点击），其后 pointerdown/pointermove 事件计数
    冻结（ne=1340→1347 恒定）——**v1.16.2 输入门在窗口无 OS key 时丢弃合成鼠标
    事件**（browser_input_available: Window.has_focus() 必需，gdcef
    input_routing.rs 实锚）；
  - 已排除：垫片（shim=0 同象）、eval 探针、meta 闩锁、mouse-leave 重入；
  - 已试无效：DisplayServer.window_move_to_foreground（boot/每击）、
    osascript activate（exit 0）、open -a 正规 LaunchServices 激活、隐藏主窗、
    caffeinate -u 唤醒显示器；屏保未运行。结论：**本机后台会话无程序化路径使
    非主窗口获得 OS key**；UPGRADE 轮的 win_focus=true 为当时环境条件（疑似用户
    在场点击），不可复现于当前无人值守会话。
- 影响面评估：产品链路真实用户流=真实点击/真实 OS 焦点，不受此环境限制影响；
  该限制仅阻自动化合成注入。v1.15.4 无此输入门（autodrive 曾可无人值守跑），
  属 v1.16.2 上游行为变化，如实记录不硬修。

## 3. [等待用户] 硬门+终验执行流程（用户在场轮）

```bash
# 0) 探针服务在位（7879；若失效按 KEYINPUT SERVICE.md 模式重启）
curl -s --noproxy '*' -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7879/
# 1) 硬门：autodrive 四步×5（补偿 OFF，垫片默认开）。终端起跑后，
#    **探针窗口弹出时用鼠标点一下窗口标题栏**（给 OS key——本轮阻塞根因），
#    之后勿再切换前台应用直至跑完。
cd /Users/timozty/Documents/vit-daw-frontend
VIT_CEF_DEBUG=1 VIT_CEF_FOCUS_LOG=1 VIT_CEF_PASTE_LOG=1 VIT_CEF_AUTODRIVE_ROUNDS=5 \
  /Users/timozty/Applications/Godot.app/Contents/MacOS/Godot --path . \
  res://vit_dock/diagnostics/cef_focus_autodrive.tscn 2>&1 | tee /tmp/autodrive_user.log
#    通过标准：exit 0，rounds_pass=5/5（键入+粘贴 round1-3 全过即硬门过，
#    round4-5 代理乱序族为已知独立缺陷不阻卡）
# 2) 终验（承 UPGRADE 移交）：从 Godot 前端正常入口打开产品设置页，
#    真实键入 key + 系统剪贴板粘贴（⌘V）+ 中文输入法 composition 切换。
```

## 4. 工件清单（本目录）

- ⑤：webui_chain_{7879_v1162_repro,7879_v1154_control,7879_v1154_control_fixed,
  7879_v1162_fixed,7878_v1162_accept}.log、classdb_{v1154,v1162}.log、
  vitagent_7878_run.log（验收轮 vitagent 自起自停，结束已杀净）；
- ①：paste_diag_{shim_on,v2_shim_on,v2_full,v2_shim_off}.log；
- 硬门：autodrive_v1162_shim_r1.log（环境阻塞轮）；
- 前端：frontend_13d670e.patch、frontend_commit_13d670e.stat；
- 取证场景（前端仓新增，可复跑）：cef_classdb_dump / cef_click_matrix /
  cef_input_hook_test（+probe node）。
