# UI-PROJECT-1 执行进度（freeflash 免费车道子代理，PC）

- 卡：coord/cards/doing/2026-11-14-UI-PROJECT-1.md（池序 346，P1）
- 分支：port/ui-project-1；worktree：D:/Vit_DAW_wt_uiproj1
- 基线 origin/main：45b9ed8f（依赖 344+345 已核：b05184f9/69219696/2f5cab6d ancestor OK）

## 断点恢复指南

先读卡面（doing/）+ 本文件，从最后一个未完成 M 继续。每个 M 完成即提交到 port/ui-project-1（progress-only 或混实现提交均可，回执时说明）。长命令先记命令再跑。

## 步骤日志

| # | 时间 | 动作 | 产物/退出码 | 下一步 |
|---|------|------|------------|--------|
| 1 | 2026-11-14 | fetch 复核基线 45b9ed8f、依赖三提交 ancestor OK、与 doing 卡域核对（UI-OS-1=Godot/extension/notifications，不相交） | - | worktree |
| 2 | 2026-11-14 | git worktree add D:/Vit_DAW_wt_uiproj1 -b port/ui-project-1 origin/main | HEAD=45b9ed8f | 领取提交 |
| 3 | 2026-11-14 | 领取提交：卡回填+git mv todo→doing+add，coord-only push HEAD:main | a0544d96 推 main 成功 | npm install |
| 4 | 2026-11-14 | npm install（worktree agent/webui，Playwright 钉 1.65.0 未动浏览器） | exit 0 | 勘察 |
| 5 | 2026-11-14 | 勘察：SPEC-V5/IMPLEMENTATION-HANDOFF 全文 + 344 primitives（MacroCard/StatusCard/TextField/Message/SnapshotReceipt/ActionCard 签名在卷）+ 345 LauncherSurface 分发模式 + api/launcher.ts 接缝模式 + render-smoke.mjs 断言模式 + lib/api.ts 与 types.ts 真实端点面 | 结论见 M1 提交信息 | M1 |
| 6 | 2026-11-14 | **M1 完成**：src/project/projectData.ts（线程视图模型+语义 reducer+requestMacroApply 守卫+ProjectService 接缝）+ projectData.test.ts 21 例全绿（npx vitest run src/project/projectData.test.ts，exit 0）。红线：applied 只经 macroApplySucceeded（需 pending+选择一致+非空 ref）；撤销绑定同 ref；failed 保留选择；试听不产应用；输入四态优先级 disabled>replying>focus>default；轨迹 busy/done/error；A/B preparing→ready 携带真实渲染引用 | 产物 src/project/projectData.ts(.test.ts) | M2 |

## M1 架构裁定（供断点恢复者）

- 复用 344 primitives 与其类型（MacroOption/MacroCardState/TraceStep 只 import 不改）；CSS 走 src/project/project.css（域内新增），styles.css 零改动（卡面「仅新增」的子集）。
- live 数据面（零 agent 改动）：上下文头=GET /agent/ui_state（AgentUIState.project）；消息=POST /agent/chat（本 surface 独立会话，同 345 模式）；停止=POST /agent/stop_turn；轨迹=chat 请求生命周期驱动 turnStarted/turnSucceeded/turnFailed。宏快照应用/撤销/A/B 渲染在 agent 侧无现成协议 → 本卡按卡面 fixture 例外走接缝替身（fixture 显式标注），live 模式不伪造；若判定需要新协议 → 停止条件交主管。
- gallery/render-smoke 用 fixture 线程直接渲染各态（345 先例），交互型 fixture 页用 ProjectService 替身。
