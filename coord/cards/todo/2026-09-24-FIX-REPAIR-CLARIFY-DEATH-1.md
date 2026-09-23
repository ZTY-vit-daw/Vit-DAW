# FIX-REPAIR-CLARIFY-DEATH-1：repair-clarify 协议违约判死策略软化——557 二次强化 repair + raw 诊断落盘

- 优先级 / 预估 / 依赖：P2（新失败族 10 轮 1 例，鲁棒性非演示阻塞）/ 0.3 天 / FORENSIC-MAC-CLARIFY-CHAIN-1 取证结论（[rulings/2026-09-24-FORENSIC-CLARIFY-1-pass.md](../rulings/2026-09-24-FORENSIC-CLARIFY-1-pass.md)，用户裁定修复另卡）
- 模型分级：L2 / GLM-5.3（chat 协议处理层，红测试先行）
- **背景（取证定责在案）**：`audio_closure_controller.go:557`（`repairCount>0 && res.NeedsClarification`→"model protocol failure" 判死，ee0fac4 入库）在「原始输出畸形+repair 违令返回 clarify」双方差同时发生时直接判死整轮——repair 明明解析成功（`message_loop.go:4322-4327` 闭包变体禁令在 prompt 里），模型却违令；策略把协议违令直接升级为任务终态，无缓冲重试。取证工件 `~/Documents/vit-forensic-clarify1-artifacts/`（秒级时间线+代码锚点）；证据边界=repair 原文未落 durable（反推高置信）。
- 目标：
  1. **557 软化**：repair 解析成功但返回 needs_clarification 且闭包在案时——**不直接判死**：先追加一次「禁-clarify 强化 repair」（在 repair prompt 显式重申禁令+指明上一轮违令形态，或按实现最小化：剥离 needs_clarification 字段按不可恢复解析降级，二选一由执行侧按代码面最小改动定，卡面倾向前者）；仍违令才走 557 判死原路径。红测试：修前=违令一次即死（取证形态复现）；修后=违令两次才死、二次合规则轮继续
  2. **repair 原文落盘**：repair 成功分支补 raw 输出诊断落 durable/遥测面（正对取证证据边界——下次同类事件可直证不再反推）
  3. 回归：F4 判死路径行为不变（真畸形不可修复仍判死）；PC 双端共用（chat 包），单边开发两端合入；agent 全量+webui 绿
- 文件域：`agent/internal/chat/audio_closure_controller.go` + `agent/internal/chat/message_loop.go`（repair 分支）+ 测试；不改 durable schema（诊断落既有遥测/日志面）
- 验收：①红测试修前红/修后绿（双方差复现+二次合规继续两态）②agent 全量+webui 绿 ③10 轮 spot（或与下轮统计批合并观测）零新形态 ④回执两端 HEAD
- 停止条件：软化需动 durable schema 或 prompt 协议层大改 → 上交拆卡；与 settle/F2 面发现纠缠 → 上交
- 领取：
- 回执：
- 验收：
