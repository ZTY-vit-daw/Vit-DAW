# UI-HOST-1 执行进度（断点续跑协议）

- 卡：`coord/cards/2026-11-14-UI-HOST-1.md`（池序 343，P0）
- owner：freeflash（GLM-5.3 Flash）免费车道子代理 · PC · ZCode 主管会话派发
- 基线 origin/main：`a4759235136725229d48406659dccf37ed9ed8f7`（2026-11-14 fetch 复核，与卡面一致）
- 主仓 worktree：`D:/Vit_DAW_wt_uihost1` 分支 `port/ui-host-1`
- 前端仓 worktree：`D:/Godot/vit-daw-frontend_wt_uihost1` 分支 `port/ui-host-1`（待建）

## 已完成
- [x] S0 读 AGENTS.md / PROTOCOL.md / 卡面 / R0 报告（REPORT.md §1-§4 已读，未知项=真栈取证目标）
- [x] S0 fetch origin 核对：卡在 todo/、派发已确认、基线 a4759235 一致
- [x] S0 doing 三卡文件域核对：CLOSURE-ROUTE-ADAPT-1（chat 路由）、ARRLOOP-AUTHBRIDGE-1（harness 装载）、WAVESHELL-LOADING-FORENSICS-1（VitApp 取证）——均与本卡不相交
- [x] S0 关键决策：烟测腿用**同模式新脚本**（不扩展 dev_agent_smoke.ps1），避开 ARRLOOP 卡"如需烟测加参"的潜在交集
- [x] S0 主仓 worktree 已建：D:/Vit_DAW_wt_uihost1 @ port/ui-host-1（自 origin/main a4759235）
- [x] S1 领取提交：a52c1c07（cherry-pick 至 origin/main 06f333cc 之上推送成功；首推被拒 2 次因远端并发前进=UI-FOUNDATION-1 领取，非同卡冲突；远端卡面 owner 核对 ✓）

## 进行中
- [ ] S6 收尾：脚本修复提交 + 工件入库 → 占用释放登记推送 → 卡面回执 + doing→done 推送

## 下一步
- 无（S6 完成即终态）

## S5 真栈结果（2026-11-14）
- **PASS run = run_20261114_184412，exit 0**（autoload 腿全断言过 + legacy 腿过）
- 核心断言：child_hwnd=4523840 跨 StartPage→VitDockRoot 五证据行全同（同一 HWND）；navigate_count=1；URL http://127.0.0.1:7878/app/ 未变；Win32 枚举恰 1 窗 title=Vit；最小化/召回过；foreground_probe_is_chat=true；legacy 无 boot 行+进程存活
- R0 §4 真栈未知项取证答案：①autoload _ready 时 main_hwnd=2558138 非 0 ✓ ②同一 HWND/页面跨 change_scene_to_packed 存活 ✓ ③detached 顶层窗任务栏条目/Alt-Tab 在场（EnumWindows 可见+前台探针中）✓（部分：DPI/多屏未测，见 RUN-SUMMARY 边界）
- §8 历史：183806（缺 .godot 缓存→补 --import 预跑）/183958/184205（FindWindowExW 装置面→EnumWindows 改造）三失败轮均脚本装置缺陷修复轮，非功能重跑；细节见 RUN-SUMMARY.md
- 拆栈亲核：四端口 free + 四类进程=0 + fixture 源哈希前后一致（F6CA36FA…）

## 已落盘实现（前端 port/ui-host-1）
- bece950 autoload 骨架（agent_chat_window.gd/.tscn + project.godot 注册）
- 719001a dock 委托改造
- 6d85041 start_page 隐藏钩子 + cpp focus/visible/icon
- caaef19 shell32 pragma + dll 重编（debug+release）
- e55e901 sconsign
主仓 port/ui-host-1：b58f42cd 烟测脚本；占用登记 55852f94 → main fee52451 ✓（≤60s 复核净空 ✓）

## 长命令记录（断了重派判断用）
- [2026-11-14 19:13 计划] powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uihost1/scripts/ui_host_smoke.ps1（预期 run 目录 coord/runs/UI-HOST-1/run_<stamp>/，exit 0=PASS；若 run 目录已存在且含 ui_host_smoke_report.json 则该步已执行过，先读报告再决定重跑）

## S3 勘察结论（2026-11-14，前端 worktree @ ff301b3）
- autoload 面已有 `VitAgentShell`（CanvasLayer 胶囊 UI，GHOST/LIST 热键面）——与本卡原生窗 autoload 职责不冲突，AgentChatWindow 用 Node+host 持有形态
- host 创建链（dock）：`_ensure_agent_webui_host`（3329）ClassDB.instantiate VitWebViewHost → `create_window(main_hwnd, "Ask Vit WebUI", w, h)`（detached 顶层窗）→ `_activate_agent_webui_host`（3246）health ready 后 `_navigate_agent_webui`（3302）
- cpp 事实：`create()` parent=0 时 fallback DisplayServer 主窗 handle（vit_webview_host.cpp:836-847）；detached 时 owner_or_parent=nullptr 顶层窗（:363）；WebView2 异步回调 + `pump_until(completed, 10000)` 同步等待（:1123）；析构 close() DestroyWindow（:538-560）；set_visible SW_SHOWNA（:582）；focus 只 MoveFocus（:590）；WM_CLOSE=SW_HIDE（:280-282）
- 重载点定位：dock 的 `_navigate_agent_webui` 是 autoload 模式必须跳过的重载源；dock `_exit_tree`（270）`_close_agent_webui_host`（4227）是销毁源
- 进入工程链：start_page 仅主动链（open_project_by_path 1263 / directive 320 / new 1298）→ `_try_enter_main`（1430）→ `_enter_main_scene`（1451）change_scene_to_packed；**无被动监听**
- 烟测驱动设计：驱动钩子落 **agent_chat_window.gd**（文件域内授权文件），环境变量 `VIT_SMOKE_AUTO_OPEN_PROJECT=<path>` 双门控（须 VIT_AGENT_WINDOW_MODE=autoload 同时设置才生效），轮询 current_scene 有 `open_project_by_path` 公共方法后调用——生产默认零行为；start_page.gd 仅做隐藏/fallback 钩子
- HWND/URL 取证面：host `debug_state()`（cpp:1215）暴露 child_hwnd/current_url/visible/child_rect.visible——autoload 在 boot/navigate/scene 切换各步 print 结构化行 `[AgentChatWindow] <event> {json}` → Godot --log-file → 烟测解析断言
- 烟测脚本模式：复制 journey1_demo_journey_smoke.ps1（fixture 复制隔离=912.vit 模式 / kernel exe 候选序列 / agent go build + -http 7878 / 端口守卫 7878/5555/5556 先 free / Stop-Stack finally 确认端口释放）——新脚本 scripts/ui_host_smoke.ps1
- legacy 腿：复用同 run kernel/agent，仅重启 Godot（VIT_AGENT_WINDOW_MODE=legacy），断言无 boot 行+启动页面板在
- 最小化/召回断言：PowerShell ShowWindow 对烟测自启的被测窗（Start-Process 同性质），IsIconic/IsWindowVisible 断言+句柄留证

## 关键决策记录
- 2026-11-14 烟测腿采用新增脚本（scripts/ui_host_smoke.ps1 或同模式），理由：doing 卡 ARRLOOP-AUTHBRIDGE-1 有"如需烟测加参"条款，避免 dev_agent_smoke.ps1 文件域交集。

## 中断恢复指引
先读卡面（coord/cards/doing/2026-11-14-UI-HOST-1.md）+ 本文件「进行中」段，不重做已完成段。
