# r2 场景真栈复现腿设计（FIX-GOV-INTERACTION-TTL-1，候栈未执行）

- 状态：**[等待真栈窗口]**——本卡零栈交付；栈窗口开启（候 309 栈腿释放后的轻窗口）时按 coord/resources/PC-RUNTIME-STACK.md 登记协议执行。
- 驱动脚本：同目录 `gov_ttl_replay_driver.py`（复用 320 run 的 frozen driver faces：chat / interaction_respond / invoke / request_json，sys.path 引 runner_frozen，与 proq3_free_driver.py 同模式）。
- 输入工程：复用 322 交付保留的 `实验·ProQ3 裸控 r1.vit` 与 `r2.vit`（`%APPDATA%\Vit-DAW\projects\`），只读打开+副本放置策略沿用 320 装置腿（不污染封存样例）。

## 复现配方（把 r2 的时序做成确定形态）

r2 丢失三要件：①会话先激活工作区 A（=r1）；②随后在工程 B（=r2）上跑 chat（起点激活因 kernel 身份未及上报而静默返回，active 仍=A）；③chat 期间在 B 的会话里 park 确认卡（persist 落进 A 的状态文件），chat 收尾 sync 跨身份切换→陈旧快照整体替换→卡消失。驱动按此序：

1. 依次 open_project A、B、再 A（终态 active=A，模拟 320 装置腿激活序）；
2. 对 B 的会话 POST /agent/chat（冻结 T7 消息）直到 `needs_confirmation=true` 且 `interaction_requests[].kind=improvement_proposal_confirmation`；
3. 立即 POST /agent/interaction/respond（approve，interaction_id=②返回的 id）。

## 断言（三腿，exit 0=PASS）

- 腿 1（r2 重放·不再被吞）：approve 回执**不得**命中任何 catch-all 类（旧「已处理或已过期」/新「未找到待处理交互」/`interaction_already_retired`）——即迟到 approve 真实路由（回执为确认链续答，`stop_reason` 非 `interaction_not_found`/`interaction_already_retired`）。
- 腿 2（重复 approve·幂等回告）：同 id 再 POST 一次 → 断言 `stop_reason=interaction_already_retired`、`goal_status` 为空、reply 含「未执行」。
- 腿 3（未知 id·显式回执）：POST 随机 id → 断言 `stop_reason=interaction_not_found`、`goal_status` 为空、reply 含「未执行」。
- 附加证据（非门槛）：approve 后 `get_plugin_parameters` readback diff（提案落地=Bonus；不作为 PASS 条件——写参数依赖模型当轮提案的具体域，receipt 类才是本卡缺陷面）。

## §8 跑前口径（成文于跑前，跑后不得倒推）

- 运行次数：**1 run**（腿 1-3 同 run 内串行）；失败重跑上限 **2 次**，第 2 次前必须复核 agent 日志锚点（`[workspace] activated` 时序 + `[interaction.respond]` WARN 行）。
- 预算：≤8 次 LLM 调用（对齐 320 r2 实测 5 次 + 容差；腿 1 的确认续链可能再触发若干次——上限 12，超限记 `model_budget_exhausted` 停）。
- 成功条件：三腿断言全过且 exit 0。
- 失败分类（分开记录，不合并）：
  - `no_card_parked`——chat 未产生 improvement_proposal 确认卡（模型行为，非本卡缺陷面；重跑 1 次后仍无则记观察项上交）；
  - `approve_lost`——腿 1 命中 catch-all（=缺陷复现，修复失败的直接证据）；
  - `receipt_wrong`——腿 2/3 断言不符；
  - `env_interrupt`——栈/端口/进程故障（须有原始日志或退出证据才可排除出有效轮次）。
- 止损线：`approve_lost` 连续 2 次（同代码版本同断点）→ 停止重跑，补代码锚点或拆取证卡；环境中断不计入。
