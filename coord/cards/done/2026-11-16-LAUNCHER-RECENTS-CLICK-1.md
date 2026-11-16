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

## 回执（2026-11-16 晚，执行侧自验完成）

### 实现清单（分支 `port/launcher-recents-click-1`，已推远端；commit 链）

- `95ea95db` **agent**：`launcher_flow.go` launcherProjectCards 每卡附 `exists`=工程文件 os.Stat 实测（fail-soft；Stat 错=exists:false 如实呈报）——UI 置灰档数据源；directive 嵌入不因 exists 缺失而丢（置灰是 UI 态非数据态）+`TestLauncherProjectCardsExistsStat`（t.TempDir 真实文件）。
- `9edf546f` **webui**：ConvergedLauncher recents 磁带卡整卡可点（`openRecentProject`——走与对话流结果卡/「打开…」相同 open 链：executeDirective→宿主治理→内核公共链，requires_governance 恒真沿用）；线程内 directive 占位（「正在打开《X》…」）+桥执行 waiting 块=装载反馈；`exists===false`（agent 实测缺失）置灰禁点+「文件已不在原位置」如实注记，字段缺省（旧 agent）不误灰；hover/聚焦/active 态（converged.css，灰度化保明度对比）；烟测接缝 `launcher_smoke_followup=recents_click[:N]`（经既有 VIT_SMOKE_LAUNCHER_FOLLOWUP env 透传，**Godot/内核零改动**；跳过缺失条目=尊重 disabled 语义；生产 URL 无参零行为）；gallery 缺失置灰样例+vitest 4 例（cardsToEntries exists 映射/Converged 静态渲染三态）。
- `68827dd1` **render-smoke**：converged 段——主样例 3/3 可点按钮+缺失置灰档断言（行标记/禁用/注记/磁带卡结构不丢）；桥 mock pass 扩展（webview addEventListener 记录器+同源 route mock greeting/recents）：**真实 DOM 点击 recents 磁带卡→postMessage 捕获 launcher_directive（action=open_project+file_path 逐卡一致+requires_governance=true）**+waiting 装载态+宿主 failed 回投→降级文案（打开未完成+宿主原因）+重试钮+**列表不失联**（2 行仍在、可点行恢复）+取证截图。
- `0f36a1e3`+`d6266e6f`+`76a0ffa3` **真栈腿脚本**：`ui_bridge_smoke.ps1 -RecentsClickLeg`（RC1/RC2/RC3；L1-L2c 原腿 gate 不动；agent telemetry 落 run 目录；RC 模式 recents 种子含损坏 fixture）。
- `5c25a181`+`b997c406` **工件**：RUNBOOK（§8 跑前口径冻结）+render 取证+三 run 目录。

### 测试命令与退出码（三门+真栈）

- `cd agent/webui && npm test` → **exit 0，706/706**（基线 702+本卡新增 4，零回退）。
- `npm run build` → exit 0（tsc 0 错+vite built）。
- `npm run test:render` → **exit 0 全段 PASS**（FOUNDATION/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/**CONVERGED（含本卡新断言）**/APPENTRY）。
- `cd agent && go build ./...` → exit 0；`go test ./internal/chat -count=1` → **ok 119.4s**（全包，含新增 exists 测试）。
- **真栈腿**（栈登记 7375a28c 占用→本释放行；内核=D:/Vit_DAW 主仓构建 VitApp.exe sha256 A59473614152882B…，agent=worktree 现场构建 sha256 A981604D…，webui=worktree dist，前端=隔离 worktree @d05043e 零改动）：
  - attempt1（run_20261116_2313）：**产品链全绿**（点击→治理 switch→opened+ack→scene 同 HWND→current_project=甲）但装置缺陷（对话往返轮1 时机误置→落 LLM **7 调用 deepseek-flash**=budget_exceeded 如实记）；
  - attempt2（run_attempt2_20261116_2320）：**RC1 全绿含零 LLM 工程内对话往返**（桥接播报 stop_reason=launcher_bridge_broadcast+context_flow=project+volume_switch）；RC2 断言锚缺陷（产品降级链已通：failed+result_ack=failed）；
  - **attempt3（run_attempt3_20261116_2332）PASS exit 0**：RC1 磁带卡点击经桥→治理三键（boot scratch 保守判据，L1 同形态）→直开→dock 场景→/agent/state=烟测桥甲→同会话两轮对话（boot 前确定性 list+进入后桥接播报）；RC2 移走乙后点击→failed+ack=failed+无场景切换+宿主警告「Project file does not exist: …乙.vit」；RC3 损坏文件点击→failed+ack=failed+「open_project failed for …坏.vit」；**telemetry 0 行**；recents 还原哈希亲证 807226F4AF3F；TEARDOWN_ALL_ZERO 亲核。

### LLM 计费上界

**7 调用（deepseek-flash，全部在 attempt1 装置缺陷轮；attempt2/3=0）**——超 RUNBOOK 预算 0 如实分记，产品断言零 LLM 依赖。

### 端测覆盖边界声明（AGENTS §5）

- 渲染面：converged 段浏览器级 DOM 断言全绿（可点性/置灰档/降级文案/列表不失联+对比度/字号/裁切扫描）。
- 真栈面：点击经**真实桥→宿主治理→内核 open→dock**全链 result_ack 亲证；点击驱动=React 烟测接缝（followup=recents_click 调用同一 openRecentProject 处理链）——**非物理鼠标事件**（headless webview 内 DOM 点击由接缝触发）；真实鼠标交互/键盘聚焦观感不在断言面（GUI 最后一米=用户手测，入口=Godot 前端）。
- 未覆盖：磁带造型升级（366 重议域，卡面边界）；跨工程在开治理三键卡的页内 React 形态（本腿治理=宿主 Godot dialog，与「打开…」同链——页内三键卡属对话发起链，渲染面另有 387 fixture 断言）。

### 工件与栈记录

- 工件目录：`coord/runs/LAUNCHER-RECENTS-CLICK-1/`（RUNBOOK+render_smoke+三 run 目录；二进制不入库哈希在 report.isolation）。
- 栈登记：占用 7375a28c / 释放见同文件本日 23:31 行（TEARDOWN_ALL_ZERO+recents 哈希亲证）。
- 前端隔离 worktree `D:/Godot/vit-daw-frontend_wt_383click`（port/fe-383-recents-click-1 @ d05043e，零代码改动）留存候主管复验后清理。

## 验收

- 待回填（候主管：diff 域亲核+三门复跑+真栈工件抽验）
