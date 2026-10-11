# GEN-CTX-INJECT-1：注入与机检（统一化实施③）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=设计件五 卡3）
- 派发确认：已确认（用户 2026-10-11 同步裁定）
- 验收负责人：GLM 主管决策流
- 池序 77；**规格全文=[设计件五 卡3](../../runs/GEN-UNIFY-CTX-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P1 / 1 天 / GEN-CTX-DISCLOSE-1（开发期可 mock 键先行，验收须真键——卡 1 合入后、卡 2 并行期间可领）
- 模型分级：L1 / GLM 或 flash 执行流（新纯库包 agent/internal/genctx/）

## 要点摘录

新包四件（快照组装/RenderGenerationPrompt+Profile/ValidateNotes+ValidateMidiFile/回读核验）+scene 三枚举（in_project 强制、free_creation/experiment_control 豁免+写前门哨兵）；零触碰 capabilitycontext/contextruntime/promptruntime；验收=件三 §6 清单单测+golden 集成（含黑键拒写/网格超容差拒写/合法通过）+真键冒测。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 单测清单）
- 验收：（裁定文件 / 验收 commit）
