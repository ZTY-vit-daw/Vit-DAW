# LAUNCHER-RECENTS-CLICK-1：启动台工程列表可点击加载（用户需求 2026-11-16 晚）

- 发卡：主管 2026-11-16 晚（用户原话：「启动页窗口左侧的工程列表是只能看不能点击加载的，这个也要修」）
- 池序 383；P1；依赖=SHELL-4（360）内核随窗就绪已合入✓——列表→打开链的内核前提已具备（v6 观测 3/5 的降级根因已被 360 消除）；真栈腿候栈位
- 已知事实：启动台列表数据链=launcherRecentProjectPaths→KernelSendCommand（launcher_flow.go:372 域）；SHELL-4 后内核随窗就绪，该链应可达——本卡把「可看」变「可点加载」。

## 目标

1. **列表项可交互**：启动台左侧工程列表每项可点击（hover/选中态），点击=发起该工程打开（走与抽屉「打开…」相同的 open 链——directive 治理语义沿用：上一工程在 agent 态时出治理三键卡，零摩擦路径直开）。
2. **反馈面**：点击后装载态可见（不误以为没反应）；失败（文件已移动/损坏）有明确文案+列表项不失联。
3. **列表项健康度标注（最小档）**：文件不存在的条目置灰或不显示（按 recents 实际存在性过滤——若现列表已按存在性过滤则免）。
4. **真栈腿**：列表点击→工程打开→工程内对话往返全链 exit 0；渲染烟测补列表可点断言（376 底座）。

## 边界

- 磁带造型升级（工程完整磁带形态）**不在本卡**——设计口径候用户裁定（366 重议），本卡只做功能可点；列表项现有 TapeBadge 样式不动。
- 不改 open/directive 协议本体；内核零改动预期（若取证发现内核清单接口缺口→上交不扩域）。
- 与 352（LAUNCHER-ZEROFRICTION-1）文件域可能相邻（launcher 面）：领取前核对 352 领取状态，若 352 已领未合入→本卡排后或主管协调域分割。

## 文件域

`agent/webui/src/` launcher 面（列表组件+事件）+`agent/internal/` launcher_flow 链（如需）+渲染烟测断言+`coord/runs/LAUNCHER-RECENTS-CLICK-1/` 工件。领取时锚定。

## 验收

真栈腿 exit 0（列表点击→打开→对话往返）+渲染烟测断言 PASS+vitest 不回退；失败路径文案亲证（移走一个工程文件后点列表项）；主管亲核 diff 域。

## 领取（2026-11-16 晚）

- 领取时间：2026-11-16 22:35（PC）
- 领取时 origin/main：`99a28618cd2e1cb65bab53166aaf78e35cddba5f`
- owner：GLM-5.3（ZCode L2 实现子代理，LAUNCHER-RECENTS-CLICK-1 执行流）／PC（Windows）
- 分支：`port/launcher-recents-click-1`；worktree：`D:/Vit_DAW_wt_383click`
- 领取基线：worktree HEAD=`99a28618`，`git status --short` 空、`git diff --stat` 空（干净基线，无领取前已有 diff）
- 本卡发布说明：领取时卡面尚未入 origin/main（主管主检出 untracked todo 件）——本提交将卡面连同领取记录一并发布至 `doing/`（发放依据=主管 2026-11-16 晚直接派发本卡至本执行流）
- 352 域核对：352（LAUNCHER-ZEROFRICTION-1）在 doing 但仅取证腿（只读+coord/runs/ 文档），实施腿候裁定排 271 后未领——与本项目写域不相交，无冲突
- 文件域锚定（领取时）：`agent/webui/src/launcher/ConvergedLauncher.tsx`（列表可点）+`agent/webui/src/launcher/converged.css`（hover/聚焦态，如需）+`agent/webui/src/launcher/launcherData.ts`+`agent/webui/src/launcher/launcherSurface.test.tsx`/`launcherBridge.test.ts`（如需断言）+`agent/internal/chat/launcher_flow.go`（recents 存在性过滤）+`agent/internal/chat/launcher_flow_test.go`（如需）+`agent/webui/src/gallery/render-smoke.mjs`（converged 段列表可点断言）+`scripts/ui_bridge_smoke.ps1`（真栈腿加参，AGENTS §5 扩展现有脚本模式）+`coord/runs/LAUNCHER-RECENTS-CLICK-1/` 工件
- 实现基线事实（领取时亲核）：387 已合入——新壳=ConvergedLauncher（recents 磁带卡 TapeCardFace，`<li data-tape-card>` 现**无**点击语义）；382 已合入；agent 卡片已内嵌 `launcher_directive`（launcher_flow.go:469）但 recents 未按存在性过滤（launcherRecentEntries 无 os.Stat）

## 回执/验收：待回填
