# FIX-GOV-INTERACTION-TTL-1：治理确认交互的过期窗口与时序（PENDING 分流③）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=gate 2026-11-09 §三 + 322 run9 r2 形态：模型经治理链提案 1000Hz/Q1.0/-2.0dB，驱动 approve 抵达时服务端交互已过期→未落地，模型终答「这个交互已处理或已过期」；322 终验裁定②记 PENDING 候产品层）
- 派发确认：已确认（gate 2026-11-09 §六 首排④ PENDING×4 分流）
- 验收负责人：GLM 主管决策流
- 池序 327；P2 / 1 天 / 依赖=零；L1 / PC；勘察先行零栈

## 目标

1. 勘察：治理确认交互（improvement_proposal 的确认卡）服务端过期窗口当前值/过期语义（静默丢弃 vs 显式状态）；approve 抵达晚于过期时的服务侧行为与模型侧回告路径（run9 r2 工件在卷可作语料）；
2. 形态设计：过期窗口与驱动-模型交互时长的匹配策略（延长窗口/approve 幂等重开/过期显式回执三选一或组合），**fail-closed 原则**（不得静默吞 approve）；
3. 实施最小修（agent 域）+ 单测（过期/迟到 approve/正常三态）；
4. 真栈验证腿候便车（可搭后续栈窗口，r2 场景复现：提案→延迟 approve→断言显式回执与终态一致）。

## 文件域

agent 治理交互域（pendingmanager/chat 交互面，以勘察锚点为准）+测试；零内核；工件 `coord/runs/FIX-GOV-INTERACTION-TTL-1/`。

## 验收标准

单测三态退出码 0 + 真栈腿 r2 场景断言 exit 0（迟到 approve 得到显式回执，不再静默过期）；端测边界声明。

## 停止条件

过期窗口语义牵涉内核交互存储 → 取证上交，不越域。

## 领取/回执/验收

### 领取（327）

- 时间：2026-11-09 晚窗
- origin/main 基线：`5915bb9c646528d98c7e25b52237872ab3c28374`（worktree 亲证：status 干净、diff --stat 空）
- owner：GLM-5.3 执行子代理 / PC / Vit-DAW 327 卡 L2 实现会话
- 分支：`port/gov-interaction-ttl-1`（实现）；领取提交=本提交（coord/327-claim，仅含本卡状态变更）
- worktree：`D:/Vit_DAW_wt_327impl`
- 前置核对：卡仍在 todo ✓ 派发已确认 ✓ 依赖=零 ✓；doing 四卡（EDU-ASK-ANCHOR-1[goalrunner_chat.go/ruleset 域]、ARRLOOP-AUTHBRIDGE-1[harness 装载确认面]、CLOSURE-ROUTE-ADAPT-1[audio_closure_controller.go，impl 分支已收口不触 server.go 交互域]、WAVESHELL-OFFRENDER-FIX-1[VitApp 内核域]）文件域与本卡（chat 交互生命周期面+测试）不相交 ✓
- 勘察结论先行（零栈，语料对账见回执节）：**服务端不存在时间型 TTL**；r2 交互丢失点=chat 收尾 `syncCurrentProjectWorkspace` 身份切换时全量重激活，用目标工作区的陈旧磁盘快照整体替换内存 `s.interactions`——交互在 chat 期间创建、却被持久化进前一工作区（r1）的状态文件，approve 抵达时 map 中已无此卡。过期语义牵涉的是 agent 侧 chat 工作区生命周期，**不涉内核交互存储**，未触发停止条件。
