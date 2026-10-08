# L1-4-IMPL-D-REVIEW-1：IMPL-D 交付独立复核（留用裁定的复核腿，只读评审+复跑门）

- 池序 21（用户 2026-10-08 晚窗裁定：IMPL-D 留用+独立复核——本卡即复核腿）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无（被审四 commit 已在 main：2bc8843c/16d3293c/4cd09238/b5fd1375）
- 模型分级：L0 / **flash 可接**（diff 亲读+复跑门+清单核对，无生产代码写入）
- 背景：IMPL-D 由决策侧 L2 于 2026-10-08 早窗完成（执行模式违规已另案裁定），用户裁定留用——**本卡把该交付当一份普通执行回执独立复核**，结论供决策侧终审。规格面=[回执](../../runs/L1-4-IMPL-D/receipt.md)+[卡面](../done/2026-10-08-L1-4-IMPL-D.md)+[CONTEXT_LAYERING_V1_DESIGN.md](../../../docs/CONTEXT_LAYERING_V1_DESIGN.md) §3-§6/§9。
- **并行域：与 FS-CAPABILITY-BLOCKED-SURFACE-1（chat/goalrunner_chat.go，独立 worktree）可并行——本卡零生产代码写入，测试复跑在自有 worktree。**
- 复核面（六项，全部独立执行不采信回执自述）：
  1. **diff 亲读**：`git show` 四 commit 全量（16 文件申报面）——对照回执"文件数申报"，找未申报的行为变更或越域文件；重点：chat/server.go 与 message_loop.go 的生产路径改动是否限于挂点/装载/翻转三类。
  2. **红线对账抽查**（回执六条逐项独立验）：五处既有退场机制零改动（chat 截尾 12/快照 recent_turns=8/观察账本窗口 24/冷引用/ExpiresAfterContextChange 锚点 diff）；queryengine/agentprotocol 零 diff；账本只追加（WriteRetains 全经 carriers.AppendLedgerEntry）；ruleset 既有七段资源文件零字节改动；22 键 allow-list 不扩。
  3. **门复跑**：`cd agent && go build ./...` + `go test ./... -count=1`（全量 0 FAIL）+ 触碰文件 `gofmt -l` 零输出 + `go test ./internal/chat/ ./internal/agentloop/ -run 'Parity|TestChatSystem|TestNeutralFamily' -count=1`。
  4. **测试充分性审读**：9 个新测试文件（exit_hook_consumption/exit_retain/carrier_report/carrier_assembly/carrier_neutral + 两 parity 翻向）——断言面是否覆盖卡面验收（retain 落盘/advisory 形态/写失败可见/去重幂等/缺层字节兼容/层序）；缺口只记录不补写。
  5. **端测边界核验**：回执六条边界声明逐条对代码实态（渲染面零触碰=webui/Godot 零 diff；通用路径 L1 界外=messageLoopSystemPrompt 未走 carriers；genesis 未接线=AppendGenesis 零生产调用方；OQ-3/OQ-5 开放态）。
  6. **可选腿（≥40 分钟余量才做）**：真栈烟测复跑 `-Scenario context_layering -StartKernel`（自有泊位，端口须空闲）验证 exit 0 可复现。
- 交付：`coord/runs/L1-4-IMPL-D-REVIEW-1/REPORT.md`——六面逐项结论+复跑退出码+问题清单（分级：blocker/应修/建议）+总判定（pass / pass-with-notes / needs-rework）。**发现问题只记录，不修复**（修复走返工卡）。
- 约束：零生产代码改动（唯一写入面=run 工件目录）；真栈泊位纪律（AGENTS §9）；测试态不写源码树（AGENTS §10）。
- 验收标准：REPORT.md 六面齐全+复跑退出码在案+判定明确；决策侧（GLM）对 REPORT 抽查复核后出裁定。
- 停止条件：复跑门红且定位为被审代码缺陷 → 停（这本身就是 needs-rework 证据，附原始输出）；环境故障（端口/进程）按环境中断记录不归因代码。
- 领取：2026-10-08 晚窗派发 flash（用户转交）/ origin/main=2d471a5c / 复跑用独立 worktree（分支名执行侧自定并回填）
- 回执：（REPORT.md 路径 / 复跑退出码 / 判定 / 问题清单条数）
- 验收：**pass（2026-10-08 晚窗决策侧，rulings/2026-10-08-L1-4-IMPL-D-REVIEW-1-pass.md）**——报告五点机械抽查全证实（红线 diff 空/文件计数/门日志/泊位复跑遥测/环境中断分类）；pass-with-notes 采信并升级为 IMPL-D 里程碑验收通过；R-1 已处置（b8557f5f）；R-4 缺口归后续卡顺带；worktree/分支随裁定清理
