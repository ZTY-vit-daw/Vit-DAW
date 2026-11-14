# UI-LOADING-1 PROGRESS（断点续跑工件）

- 卡：coord/cards/doing/2026-11-14-UI-LOADING-1.md（池序 347，P1，v5.1 链最后一张）
- owner：freeflash（GLM-5.3 flash）· PC · ZCode 免费车道执行会话
- 分支：port/ui-loading-1 · worktree：D:/Vit_DAW_wt_uiload1
- 本文件随里程碑提交到 port/ui-loading-1 分支（不入 main，实现进 main 经主管验收）

## 步骤日志

- S0 [2026-11-14 21:26] 领取完成。fetch 复核 origin/main=073b594f 与卡面基线一致；346（f05a0351）+348（073b594f）已合入；doing 三卡域不相交（ARRLOOP/CLOSURE agent Go 域，WAVESHELL 只读取证）。领取提交：首推被拒（远端新增 a682b2e6=352 卡领取，预告内的并行流），按 PROTOCOL §2.1.3 rebase --onto 重放为 1ef32914 后成功推送（coord-only fast-forward）。下一步：npm install + 读设计规范。
- S0.5 [2026-11-14 21:30] PROGRESS.md 建立（S0 记录）。工作树 git rev-parse HEAD=1ef32914；status 干净（除本文件）。

## 里程碑

- M1 loading 组件+状态机（src/project/loading/**）——进行中
- M2 App.tsx 接入（ProjectLoadingGate）——未开始
- M3 错误/中断路径（并入 M1 组件能力+测试）——未开始
- M4 gallery+渲染烟测+三命令自验——未开始

## 设计裁定记录（M1 开工前，2026-11-14 21:40）

1. **live 装载事件源**：agent ui_state 无装载分步事件（已核实 projectService.ts/agent 侧）。live 只消费既有 fetchUIState 往返（pending/成功/失败）= 真实单步事件；三步细分（已读取工程文件/已恢复轨道/正在连接音频设备）作为**组件能力+gallery fixture 样张**呈现，live 不伪造推进（不新增 agent 语义=卡面硬要求；细分事件源若需要，属停止条件交主管）。
2. **接入形态**：App.tsx surface=project 分支包 ProjectLoadingGate（loading/** 内实现）——probe ui_state：不可达=装载中（LoadingCard+无输入框=禁用）；成功=放行 ProjectSurface（composer 恢复）；失败=错误态+重试。不改打开链/不新增 surface 值/不改 ProjectSurface.tsx（不在文件域）。
3. **磁带带量**：tapeSVG 从 main-window-v3/SPEC.md §3 逐字移植（结构冻结；rgba 助手 SPEC 未附，按常规 [r,g,b]→css 实现并注明）。进度 p 只来自显式事件：无 p 显示待机帧 L=1,R=0.2 静止（未知进度不伪造）；有 p 推进 L=1-0.8p,R=0.2+0.8p；完成帧 L=0.2,R=1。进度表达只用磁带带量（卡面定磁带，SPEC §8.2 二选一）——不复用 344 StatusCard 进度条。
4. **错误/中断区分**：failed（失败，可重试）与 interrupted（中断，原因+恢复入口）两类事件，都写明原因与恢复入口。
5. **完成回执**：收一行（i-done+结果行），轨迹可展开（原生 details，尊重 prefers-reduced-motion 的 CSS 由样式层处理）。
6. **样式落点**：src/styles.css 末尾仅新增 v5-loading-* 规则（卡面指定）；gallery 样张页与 render-smoke 断言段按 345/346 先例最小扩展（回执声明）。

## 长命令登记

- npm install（worktree webui）——已跑完 exit 0（exec_026ccd62，21:35 前后）
- npx vitest run src/project/loading——exit 0（23 passed，21:40）
- npx tsc --noEmit——exit 0（21:41）

## M1 实现注记（2026-11-14 21:41，bb721db9）

- 文件：tape.ts（tapeSVG 逐字移植+tapeReelAmounts）/ loadingData.ts（reducer+红线）/ LoadingCard.tsx（四态组件）/ ProjectLoadingGate.tsx（live 接缝）/ loading.css（loading/** 内，styles.css 零改动）/ 两个测试文件。
- 样式落点改判：loading.css 放 loading/** 域内而非 styles.css——styles.css 含 html/body overflow:hidden 全局规则，gallery fixture 页加载会破坏现有三页渲染烟测；组件级 import 为 345 launcher.css 同款先例。styles.css 保持零改动。
- 修复过程：①reducer loadingStarted 空标题拒绝会卡死 live probe（无工程名常态）→放行空标题（渲染层兜底通用句式）；②failed 磁带帧回卷违背"不回卷"→冻结进度帧；③gate fixture 终态从 effect 移到初始化（静态渲染可见）。
- M1 commit：bb721db9。

## 失败分类与停止记录

- （无）
