# UI-ENTRY-1 执行进度（断点续跑协议）

- 卡：coord/cards/doing/2026-11-14-UI-ENTRY-1.md（池序 354，P0，手测 v3 返工主卡）
- owner：freeflash（GLM-5.3-Flash）免费车道子代理 · PC · ZCode 主管会话派发
- 分支/worktree：port/ui-entry-1 @ D:/Vit_DAW_wt_uientry1（主仓，基线 origin/main 19252208）；port/ui-entry-1 @ D:/Godot/vit-daw-frontend_wt_uientry1（前端仓，基线 380166e）
- 领取提交：7eb334b4（coord-only → main，远端 owner 已核对）

## 设计定案（中断恢复读这段）

**M1 webui（App.tsx + src/launcher/entrySurfaceBridge.ts）**
- App() surface 状态化：useState 初始=appSurface()（URL 参数语义保留，浏览器 dev 直开不变）；
- 订阅宿主→React 消息 `{type:"vit_surface", surface:"project"}`（chrome.webview "message" 事件，
  guard 桥不可用）→ 白名单校验（AppSurface 合法值）→ setSurface（React 内部重渲染切换，
  **零导航零重载**，343 navigate_count 不变量保持）；
- 接受合法 vit_surface 后回执 ack：postMessage `{channel:"ask_vit_entry", type:"vit_surface_ack",
  surface}` → 宿主日志留痕（React 真收并处理证明，烟测断言面）；
- MainApp 内 useMemo(appSurface()) 不动（MainApp 挂载期间 URL 不变）。

**M2 Godot agent_chat_window.gd（autoload 编排层）**
- URL 路由：新常量 `_AGENT_WEBUI_URL_LAUNCHER = "http://127.0.0.1:7878/app/?surface=launcher"`；
  `_navigate_page()` 用它（page_loaded 前缀判定 begins_with 旧 URL 仍成立）；
- 主窗启动隐藏：`_ready()` 中 window_created 成功后第一时间 `DisplayServer.window_set_visible
  (MAIN_WINDOW_ID, false)` + 取证行 `main_window.hidden`（autoload _ready 在首帧渲染前执行；
  残余闪现可能性如实记录，OS 层窗口创建先于 autoload 属引擎级限制）；
- 新公共面：`restore_main_window(reason)`（set_visible(true)+最小化恢复+取证行 `main_window.restore`）；
  `present_project_stage()`（guard 非 mode_active 直接 return=legacy 零行为；restore 主窗+
  post_web_message vit_surface project+取证行 `project_stage.presented`）；
- `_on_host_web_message` 拦截 vit_surface_ack 记日志 `surface.ack`（窗口级取证，类似 vit_os_notify 拦截）。

**M3 Godot start_page.gd / vit_dock_root.gd（场景侧钩子）**
- start_page：`_on_agent_chat_window_unavailable(true)` 与 `_on_agent_chat_window_lifecycle("closed")`
  现有 handler 内追加主窗回补（调 autoload restore_main_window）——React/agent 失败或用户关
  启动台窗 → Godot 启动页兜底（面板恢复链既有，主窗可见性补上）；
- dock：`_ready()` 追加 `_present_main_stage_on_entry()` → 调 autoload present_project_stage()
  （进工程展开接线单点：主窗恢复+React 同窗切 project surface）。

**M4 回退/legacy**
- 回退腿注入=真栈无 agent 起 autoload（health 超时 → unavailable 真链）→ 主窗恢复+启动页可见；
- legacy：flag 不设 → mode.off 全惰性，零行为变化（所有新路径 guard 在 mode_active 内）。

**M5 真栈烟测 ui_entry_smoke.ps1（scripts/ 新脚本，ui_host_smoke 同模式）**
- autoload 腿：EnumWindows 可见顶层窗恰 1 个（VitWebViewHostChildWindow，React 启动台）+
  Godot 主窗不可见 → VIT_SMOKE_AUTO_OPEN_PROJECT 真实 open 链 → 主窗可见 + 同 child_hwnd +
  navigate_count==1 不增 + page_url 不变（launcher URL）+ surface.ack 行（project surface 在场证明）；
- 回退腿：无 agent 起 Godot autoload → health.failed → unavailable → main_window.restore 行 +
  Godot 主窗可见；
- legacy 腿：进程存活+零 boot 行（UI-HOST-1 同款）；
- fixture 912.vit 副本+源哈希前后一致+recents 快照还原（AGENTS §10）。

## 步骤

- [x] S0 勘察（卡面/PROTOCOL §2.2/裁定/UI-HOST-1+UI-BRIDGE-1 PROGRESS/栈记录尾部）
- [x] S1 fetch 双仓（主 19252208/前端 380166e）+ doing 域核对 + 卡池状态
- [x] S2 双 worktree 建立（主仓+前端仓）
- [x] S3 领取提交 7eb334b4 → main（远端 owner 核对 ✓）
- [x] S4 RUNBOOK + 本 PROGRESS 落盘
- [x] M1 webui：entrySurfaceBridge.ts + App.tsx surface 状态化 + 单测 3 例
- [x] M2 Godot：agent_chat_window.gd URL 路由 launcher + 主窗隐藏/回补 + 公共面
      （实现注：DisplayServer 无 window_set_visible GDScript API（4.6.1 parse 实证），
      主窗可见性走 root Window 节点面——隐藏=IsWindowVisible false 满足烟测断言面）
- [x] M3 Godot：dock _ready 进工程展开接线（start_page 零改动——回补收在 autoload 内部）
- [x] M4 webui 门槛：npm test 563 全过（560 基线+3 新增）/build 过/test:render 四渲染面 PASS
      （双仓实现 commit：前端 a760c05 / 主仓 3fb532d4+8688b399）
- [x] M5a §2.2 占用登记 da674ce1 → main + owner 重读确认 + ≤60s 复核（五 TCP=0 进程=0，23:25）
- [x] M5b 三腿烟测 7 attempts：1=BOM 装置轮 / 2=$pid 自动变量装置轮 / 3-5=引擎限制发现+确认轮
      （Godot 4.6.1 主窗 hide 降级最小化，probe_232728/232856 铁证）/ 6=extension SW_HIDE face
      落地+脚本竞速暴露 / **7=run_20261114_233821 PASS exit 0 三腿全绿**
- [x] M5c finally 拆栈复核（五 TCP=0+进程=0）+ run 工件精简件+脚本修复随 port/ui-entry-1 入库
      （cd354f2a；前端 extension face=ae24b52）
- [ ] M6 卡面回执 + doing→done + coord 推送 + 双仓 port 分支推送

## M5b 命令记录（长命令跑前落盘）

- 命令：powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uientry1/scripts/ui_entry_smoke.ps1
- 详见 RUN-SUMMARY.md §8 运行历史（7 attempts 根因链）

## 中断恢复指引

读本文件「设计定案」+「步骤」+卡面；git log 双仓看进度；真栈占用状态读
coord/resources/PC-RUNTIME-STACK.md 顶部（本卡占用登记提交 hash 记录于此）。

## 断了几次 / 中断恢复

- 本会话暂未被外部重派中断。

## 终态

- **DONE**：PASS run_20261114_233821 exit 0 三腿全绿；卡片 f79bf70f move doing→done（main，回执待主管决策验收）；双仓 port/ui-entry-1 已推送（主仓 b7571d9d / 前端 ae24b52）；真栈占用 da674ce1→释放 37d5581d（当前空闲）。
- 协调 worktree（coord/uientry1-stack）已清理；实现 worktree 双仓保留（port/ui-entry-1 待主管验收；前端 worktree 含 godot-cpp junction——拆除前须先 cmd //c rmdir 摘链接，PROTOCOL §3）。
- port worktree 内 coord/cards/doing 卡的回执编辑未提交属预期（卡片状态面以 main f79bf70f done 卡为准）。
