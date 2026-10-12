# GEN-CTX-INJECT-1：注入与机检（统一化实施③）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=设计件五 卡3）
- 派发确认：已确认（用户 2026-10-11 同步裁定）
- 验收负责人：GLM 主管决策流
- 池序 77；**规格全文=[设计件五 卡3](../../runs/GEN-UNIFY-CTX-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P1 / 1 天 / GEN-CTX-DISCLOSE-1（开发期可 mock 键先行，验收须真键——卡 1 合入后、卡 2 并行期间可领）
- 模型分级：L1 / GLM 或 flash 执行流（新纯库包 agent/internal/genctx/）

## 要点摘录

新包四件（快照组装/RenderGenerationPrompt+Profile/ValidateNotes+ValidateMidiFile/回读核验）+scene 三枚举（in_project 强制、free_creation/experiment_control 豁免+写前门哨兵）；零触碰 capabilitycontext/contextruntime/promptruntime；验收=件三 §6 清单单测+golden 集成（含黑键拒写/网格超容差拒写/合法通过）+真键冒测。

- 领取：2026-10-12 早窗 / origin/main=813cbf58 / owner=Mac 决策侧会话兼执行（用户 2026-10-12 裁定排序权移交，PC 调度件 005893b7 在案）/ 分支 port/gen-ctx-inject-1 / worktree ~/Documents/Vit-DAW-geninj1
- 回执：done（自验全绿）——实现 <本提交>@port/gen-ctx-inject-1。genctx 四件全落地：snapshot（music_context 真键组装+markers→sections+scene fail-closed）/render（单一模板引擎+profile 只裁+用户块逐字）/validate（11 scale 集合+网格 ±1/64+音域密度长度+SMF 预解析三检）/readback（二道门+集合子集）+GuardWriteCommand 哨兵；避让红线兑现（capabilitycontext/contextruntime/promptruntime 零触碰）。门：go vet 0+gofmt 净+全量 94 包 0 FAIL exit 0+真键冒测 PASS exit 0（run genctx_truekey_mac_20261012-101819：内核直连 set_tempo 96→music_context.tempo_bpm=96 联动断言+pre-KEYFIELD 调性 null=占位可见语义，工件 coord/runs/GEN-CTX-INJECT-1/）；测试 12+golden 三例+形状黄金串。端测边界（AGENTS §5）：纯库面（零 webui/旅程改动），真栈面=内核直连只读冒测；生成端到端（LLM 产 note→门→写入）归 CONSUME 卡与烟测腿
- 验收：pass（2026-10-12，Mac 决策侧先行裁定——PC 调度件 005893b7 授权+复核留档）——[rulings/2026-10-12-GEN-CTX-INJECT-1-pass.md](../../rulings/2026-10-12-GEN-CTX-INJECT-1-pass.md)；合入=cherry-pick 705916e5+6a56f065；全量 94 包 0 FAIL+真键冒测 exit 0+避让红线兑现
