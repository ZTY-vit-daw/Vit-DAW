# SCORE-ENTRY-REACH-1：谱面工作台用户旅程可达性取证+接入（374 M4 回执 FAIL 项）

- 发卡：主管 2026-11-16 晚（承接 374 用户手测 M4 回执：「GUI 和 webui 都没看到」谱面入口）
- 池序 382；P1；依赖=374 已合入✓（score 页签接线在 App.tsx WorkbenchTab）；真栈腿候栈位
- 已知事实：score 页签功能面成立（vitest 668+render 六段+DG1-A 十五断言）；渲染烟测走 webui 直载测试面，**未覆盖 VitShell→工程→谱面真实旅程**（AGENTS §5 JOURNEY-1 债的又一实证）。

## 目标（取证→接入→用户重测）

0. **D4 崩溃锚点（377 上交，2026-11-16 晚补入——取证首要假设之一）**：非 120bpm 工程谱面面板**装载崩溃**——链：`VitApp/Source/Service/MidiService.cpp:1758-1768`（length_seconds 拍数冒充秒）→ `agent/webui/src/scoreworkbench/kernel.ts:131-140`（clipLengthBeats=10.6667）→ `ScoreWorkbenchPanel.tsx:401`+`model.ts:96-102`（渲染期 ModelError 无边界→**整面板卸载**）。120bpm 下 8×120/60=16 恰整数网格不触发——用户工程非 120bpm 时打开谱面即白板，是「看不到谱面」的直接候选根因。处置：D4 修复并入本卡（内核 1 行级+webui 渲染边界，域=377 上交锚点转移授权）或按取证结论拆微卡，主管裁量。
1. **取证腿（真栈）**：拉起 VitShell→打开含 MIDI 内容的工程（可用 373 `lesson_project.vit`），逐步定位断点：
   - webui 独立访问（8080 浏览器直开）时 score 页签在哪一级、是否可见；
   - VitShell 工程态内嵌面是否暴露 workbench/score 页签（疑：ProjectSurface 只渲染 chat/治理面，workbench 不在内嵌面）；
   - Godot GUI 侧无谱面入口=预期（谱面在 React webui）——确认后向用户说明正确入口。
2. **接入腿**：按取证结论把谱面工作台接入用户可达旅程（候选：VitShell 工程面加 workbench/score 入口；或启动台/工程面显式「打开工作台」动作；最小改动档优先，不动 374 组件本体）。
3. **旅程断言**：渲染烟测补一条**旅程级断言**（从工程面可达 score 面板，非直载），防同型回归；JOURNEY-1 面就位前按 CDP 模式。
4. 用户重测 M4 接力：入口可达后候用户做谱面手感抽检（DG3 M 清单回归 374 挂账）。

## 边界

- 不重构 374 谱面组件；App.tsx 改动限入口暴露最小面（若与 368 收敛阶段 1 冲突→记录上交主管裁量，不扩域）。
- 若取证发现 score 页签在 webui 独立面本就可见且可达（纯用户引导问题）→结论降级为「入口说明+文档」，接入腿取消，如实回执。

## 文件域

`agent/webui/src/`（App.tsx 入口面/ProjectSurface 或 launcher 面，领取时锚定）+渲染烟测断言+`coord/runs/SCORE-ENTRY-REACH-1/` 工件。

## 验收

取证断点定位有证据（截图/DOM 锚点/代码行）；接入后旅程级断言 PASS+真栈腿 exit 0；用户 M4 重测放行（用户节点非本卡阻塞项，登记候测）。

## 领取/回执/验收

- 领取：2026-11-16 21:3x +0800 / origin/main=b6941a45eca3c269c46e96063b8e539ff0c559c2（387 CONVERGE-SHELL-1 合入后基线；核对其 App.tsx 改动=launcher 分发换挂 ConvergedLauncher，与本卡 project/workbench 分发域无冲突）/ owner=GLM-5.3 L2 执行子代理（ZCode PC 会话，SCORE-ENTRY-REACH-1 专项）·PC / 分支 port/score-entry-reach-1 / worktree D:/Vit_DAW_wt_382reach / 协调分支 coord/382reach-claim / 领取提交=本提交（卡面首次入库——发卡侧原稿在主检出未跟踪，随领取一并入库 doing/）。
- 回执：
  - **实现提交** @ `port/score-entry-reach-1`（已推远端）：`12d9a9e2`（D4 修复 webui 面：ScorePanelErrorBoundary+snapClipLengthToGrid+clipLengthBeatsRaw+回归 4 测）→`d0aaede2`（接入腿：ProjectSurface titlebar「打开工作台」+workbench「返回工程」条）→`1760e333`（旅程断言 J1/J2/J3 扩展进 score_workbench_smoke.mjs+D4 处置注记更正）→`71dd86d9`（App() surface state 下传 MainApp——attempt1 实证修复）→`9c0681dd`（run 工件+runtime_provenance）。
  - **D4 定性更正（377 前提被取证推翻，AGENTS §11）**：kernel-only ZMQ 取证腿（forensic_zmq attempt5 exit 0，F0-F5 全 PASS）证明**内核命令面换算正确**（insert_midi_clip beats8/6 回显 6.0s/4.5s@80bpm；读写一致；save→open 往返稳定）——377「拍数恒等冒充秒」的内核归因不成立；真根因=**373 手写模板 XML 的 MIDICLIP start/length 用拍数值**（length="8.0"=8 拍），tracktion loader 按秒直读（tracktion_Clip.cpp CachedValue<TimePosition> referTo 无换算）。**内核零改动**（卡面「内核 1 行级」前提撤销）；修复面=webui 错误边界+桥面网格化宽容（8.0s@80bpm=10.6667 拍→上取整 10.75+raw 披露，非网格 clip 尾=合法工程状态不再崩）。
  - **越域上交（373 模板资产，主管裁量）**：`coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`（sha256 0CB22BA3…21B3）三个 MIDICLIP（旋律/鼓 length=8.0、主题 length=16.0）均为拍数值被当秒读——装载后 clip 拍长虚增 1/3（80bpm）。最小修=模板数值按秒改写（8.0→6.0、16.0→12.0）+373 manifest/派生 fixture 同步；或不改模板（webui 宽容化后=渲染范围多 4.75 拍空白+status 披露上取整，音乐语义无损）。归 373 域，本卡不扩修。
  - **取证结论（入口）**：①webui 直开（默认 full surface）右侧工作台面板+score 页签本就可见（P1 断言既有覆盖）；②**VitShell 工程态（project surface）无任何 workbench/score 入口**（结构性断点实证：ProjectSurface 渲染 chat/治理面，workbench 面板只在 MainApp）=374 M4「没看到」主因之一；③D4 白板=第二断点（非 120bpm 工程即使入口可达也崩）。接入腿按②落：工程面 titlebar 按钮→React 内部切 surface（零导航，343 不变量）+返回工程条闭环。
  - **真栈验收**：`run_accept_20261116_2217` attempt2 **PASS exit 0：28/28 断言**（A1-A8 十九项零改动+J2-non120-panel-load/clip-grid-snapped/error-boundary-absent〔377 崩溃点回归：面板在场+status「clip 10.75 拍…非网格已上取整」+零 pageerror〕+J1-project-surface-reachable/workbench-entry-present/workbench-surface-switched/score-panel-from-project-journey+J3-return-button-present/return-to-project）；attempt1=实现域缺陷（MainApp 自读 URL surface）修复后过；截图八张在卷。TEARDOWN_ALL_ZERO 22:19:20+recents 终态哈希亲证 807226F4AF3F。
  - **测试命令与退出码**：webui `npm run test`=702/702 PASS exit 0（387 基线 698+新增 4）；`npx tsc --noEmit` exit 0；`npm run build` exit 0（dist 重建×2）；agent `go build ./...` exit 0（agent 零语义改动）；真栈 `dev_agent_smoke.ps1 -StartKernel -RestartAgent -ScoreWorkbenchSmoke -NoChatSmoke` exit 0。
  - **LLM 计费上界=0 兑现**（-NoChatSmoke+全确定性 invoke/ZMQ/UI 面；两腿 RUNBOOK §8 跑前成文：取证 forensic_zmq_20261116_2140+验收 run_accept_20261116_2210，均正式轮单 run 无重跑，attempt 分记在卷）。
  - **端测覆盖边界声明（AGENTS §5）**：旅程断言=CDP/headless Chromium 面（`?surface=project` 浏览器直开等价 VitShell 工程面 webview——surface 状态机/React 内部切换与真宿主一致，但**未在 VitShell.exe 真宿主窗内实点**按钮〔SHELL 系列全链栈成本，本卡最小档未启〕）；VitShell 实机目检建议随用户 M4 重测一并做。渲染面=浏览器级 DOM 断言+八截图。
  - **装置面缺口上交（非产品）**：dev_agent_smoke.ps1 Strip Silence 段在 scoreWb recents 快照窗口外写 recents（attempt2 berth 还原到 Strip 段态 D087232D 而非占用初态）——owner 手工还原 807226F4+终态亲证（strip_silence_state.json 留档 run_accept_20261116_2217）；建议后续把该段纳入快照窗口或统一 berth 级快照。
  - **环境事故记录（已恢复）**：21:5x 主检出 `agent/webui/node_modules` 出现清空形态（仅剩 .vite-temp；worktree junction+npx vitest 启动失败时发现；主检出 21:42 前 387 曾报 vitest 698=698，事发时点与根因无法完全回证——不排除本会话 junction+npx 触发）——处置=主检出 `npm ci` 恢复 170 包（lock 未变，6 秒）+junction 后即抽查条目数入流程。node_modules 非跟踪件，git 面零影响。
  - **用户 M4 重测放行**：入口=VitShell 进工程→titlebar「打开工作台」→谱面页签（非 120bpm 工程不再白板；DG3 M 清单候用户抽检）。
- 验收：（待主管 ruling）
