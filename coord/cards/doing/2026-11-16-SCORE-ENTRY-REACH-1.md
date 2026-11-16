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
- 回执：（待回填）
- 验收：（待回填）
