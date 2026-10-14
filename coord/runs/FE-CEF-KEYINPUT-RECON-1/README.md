# FE-CEF-KEYINPUT-RECON-1 run 工件索引

- 卡：coord/cards/doing/2026-10-14-FE-CEF-KEYINPUT-RECON-1.md（池序 111）
- 执行：Mac 执行侧（GLM-5.3 L2，ZCode），2026-10-14
- worktree：/Users/timozty/Documents/Vit-DAW-fecef1（分支 port/fe-cef-keyinput-recon-1，
  基线 fa9c579f=origin/main 领取后）；前端仓分支 port/fe-cef-keyinput-recon-1（基线 95e362f4，
  领取时分支 port/leg5-telemetry-probe 干净、工作树无已跟踪改动——只记录未整理）

## 工件清单

| 文件 | 内容 |
|---|---|
| SERVICE.md | 探针构建+服务方式（端口 7879/启动命令/健康检查/dist 防护） |
| USER_REPRO_INSTRUCTIONS.md | [等待用户] 单轮复现采集指引（R1 探针/R2 产品页/R3 Safari 对照） |
| browser_direct_baseline_iab_chrome146.json | 浏览器直开对照基线时间线（ZCode IAB，Chromium 146/Electron 41） |
| browser_direct_baseline_overlay.png | 对照基线浮层截图 |
| dist_mtime_before.txt / dist_sha_before.txt | 产品部署 dist 防护基线（主工作树，2026-10-13 21:15:34 版） |
| dist_mtime_after.txt / dist_sha_after.txt | 全流程后核对（须与 before 逐字节一致） |
| dist_probe_snapshot/ | 探针构建产物快照（input_probe/dist） |
| frontend_focus_log.diff | 前端仓焦点链日志+URL 钩子补丁（=前端仓 commit 6e5a459） |
| gdcheck_host_compile_ok.log | 宿主脚本 autoload 桩工程完整编译验证记录 |

## 浏览器直开对照结论（执行侧部分）

引擎：ZCode IAB（Chromium 146.0.7680.80 / Electron 41.0.3）。四步序列结果：**全部正常**——
A 原生框与 B React 受控框均接受逐键输入（input 事件+onChange.exec+commit+value.check
verdict=state-synced），切换输入框正常，值不被抹回；粘贴事件可到达（合成 ClipboardEvent，
trusted=false 如实标记）。**边界声明**：IAB 自动化无法注入真实可信键盘事件（keydown 层仅
合成，click 无真实事件——CUA type 实为 paste 注入，已在时间线如实呈现），真实键盘+真 ⌘V
的直开对照按卡面指定由用户轮 R3（Safari）补全。

## 探针三层规格（实现于 agent/webui/input_probe/）

- 焦点层：focusin/focusout/window focus/blur + 300ms 轮询 document.hasFocus() /
  activeElement / visibility（变化才记录）
- 页面事件层：keydown/keyup（window capture，含 key/code/修饰键/isComposing/trusted）、
  beforeinput/input、paste（剪贴板类型与长度）、compositionstart/end、pointerdown、
  pageerror/unhandledrejection
- React 层：受控 input onChange 执行记录 + 值快照对比（wipe-back/dom-drift/state-synced
  判定）+ commit 日志；原生 uncontrolled 框作对照组
- 出口：页面浮层就地可视化 + `[:INPROBE]` console 镜像（VIT_CEF_DEBUG=1 时落 Godot 日志）
  + localStorage 持久化 + 下载/复制 JSON + `window.__inputProbe` API

## 前端仓焦点链日志（commit 6e5a459）

- VitDebugFlags 增 `cef_focus_log_enabled`（env VIT_CEF_FOCUS_LOG=1 / debug_flags.json 键，
  默认关，JSON 热重载 0.8s 生效）
- vit_cef_web_view_host：CefTexture focus_entered/exited + Window gui_focus_changed +
  set_visible/grab_focus 前后焦点态 + 0.5s 焦点态心跳（win_focus/browser_focus/focus_owner，
  变化才打点）+ host.created/close
- vit_dock_root：navigate.focus_called / dock.set_visible / dock.focus_agent_webui_window 三点
- 取证钩子：`VIT_AGENT_WEBUI_URL` env（默认空=生产行为不变；探针轮次指 7879）
