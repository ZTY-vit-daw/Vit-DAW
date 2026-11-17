# LAUNCHER-RECENTS-READ-1：工程管理页只读最近列表

- 池序401；P1；388依赖小卡；PC；发卡/验收Codex主管`/root`。
- 派发确认：用户批准入口收敛并开工；主管依执行证据拆必要接缝，不新增产品方向。证据=launcher_flow.go:72-79进工程后关launcher聊天分支，project_flow.go仅拦new/open；返回管理页自动“有哪些工程”会落LLM，不能伪装空列表。
- 目标：新增只读GET `/agent/launcher/recent-projects`，复用现有launcherRecentProjectPaths+launcherRecentEntries+launcherProjectCards（含exists、同schema治理directive），不经chat/LLM，不物化草稿、不改变userEnteredProject/authority/工作区；启动/工程管理两态都可读。内核不可达非空列表，不伪造空态。
- 实现域≤3：`agent/internal/chat/launcher_flow.go`（handler）、`server.go`（路由注册）、`agent/webui/src/api/launcher.ts`（fetch封装）。必要handler/api测试另列；不改预算/其他治理门。
- 验收：新handler GET正常/空/核不可达/错误method/已进工程仍可读+无LLM与工作区激活副作用，旧launcher/chat回归；Go build与相关包测试exit0；388真实栈往返GET列表/磁带卡打开/异常态与telemetry零调用亲证exit0。
- 并行：与271仅验证不同，不动其预算；352未实施不冲突；394原生域/397谱面域不交。server若别卡注册先主管序列化。
- owner=`/root/entry388`（Sol/high，PC），复用`C:/Users/timoz/.codex/worktrees/converge-entry-388/Vit_DAW`、`codex/converge-entry-388`分支；领取时388有5文件未提交改动，全部保留；本卡只提交上述3生产文件+其测试，先形成401独立commit再提交依赖388，禁止混入388骨架。
- 领取基线779477ea（main已至bb10c48b，差异仅协调文件）；领取提交同步共享main核对owner后才实现本卡。真栈仍候主管放行。
- 停止：existing helper强制物化/需改authority或预算→证据上交；同断点连败2次不原样重跑。
- 回执/验收待回填。

## 401 零栈回执与主管复核（2026-11-17）

- b91a8ad9 @codex/converge-entry-388；3生产文件+2测试+原始日志，Go build/chat全包、API7项/前端713项执行侧exit0。
- 主管亲读handler/API与反例，亲复跑go test ./internal/chat -run '^TestLauncherRecentProjects' -count=1 exit0（0.096s）；未将单测当端测，388真实栈GET/往返证明仍欠。
