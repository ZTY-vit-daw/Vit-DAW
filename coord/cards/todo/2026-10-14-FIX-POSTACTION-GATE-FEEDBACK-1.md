# FIX-POSTACTION-GATE-FEEDBACK-1：post-action 观察门反馈面三锚点修复（弹回教学/豁免粒度/提示矛盾）

- 发卡：GLM 主管决策侧 / 2026-10-14 晚窗（依据=[FORENSICS-LEG-E.md](../../runs/FIX-FS-SETTLE-DEADCHAIN-1/FORENSICS-LEG-E.md) §一 run8 子形态三锚点+腿 E 上交裁定；109 收口腿预算效率前置）
- 派发确认：已确认（主管依已核实取证证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 113；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无（与 112 文件域不相交可并行）
- 模型分级：L2 / GLM 执行会话（涉门语义，L2）

## 缺陷三锚点（已核实，FORENSICS-LEG-E §一）

1. **弹回教学含不可达分支**：per-view 弹回消息教 "decide from the recorded evidence"，但 requires_post_action_observation=true 时 settle（free_state_reasoning.go:1085）/blocked（:1214）/needs_action（:1026）/satisfied（:1188）四门全部要求本周期成功观察——教的路径被结构性封死。
2. **revision 豁免粒度错位**：freeStateReObservationAdmitted（free_state_gate.go:1055）按**整集 fingerprint** 找 prior（:1061-1076），模型收窄 view 集（合法请求）后 fingerprint 不匹配→豁免够不着→落入 per-view 弹回。
3. **提示矛盾**：ccb_model_prompt.go:670 教 post_action 段首动作="experiment's view set on the applied target"重取，与门的整集豁免/弹回行为未对齐（run8 模型收窄两视图即落入弹回陷阱）。

## 任务（硬约束：**不弱化证据门**——post-action 必须有本周期新鲜观察的语义不变；防重复防浪费语义保持；仅放行"新 revision 豁免成立"或"真正换 view/target"两条路）

1. 弹回消息阶段感知：requires_post_action_observation=true 时不再教"decide from the recorded evidence"，只教可达出口（按豁免形状重取整集 / 换 view/target）。
2. 豁免 fingerprint 子集匹配：请求 view 集为某 receipt view 集的**子集**且 revision 新于该 receipt 时可走 AdmitReObservation 新 revision 路径（收窄不再失灵）；非子集/revision 不新→维持弹回。
3. ccb_model_prompt.go:670 与门行为对齐：教学形状必须是豁免可收的形状（整集重取或子集+新 revision），并明说收窄是允许的。
4. 单测钉：三锚点各一钉+组合形态（post-action 阶段弹回消息不含不可达分支；子集+新 revision=admitted；子集+旧 revision=弹回；提示文本与豁免形状一致）。
5. `go test ./... -count=1` 0 FAIL+gofmt 净（LF blob）。

## 文件域

agent/internal/agentloop/free_state_reasoning.go+free_state_gate.go+chat/ccb_model_prompt.go+同包测试；**不得动** settle/final gate 证据断言与 MaterialityEvaluation（109 腿 D/E 域）；不占真栈（复跑归 109 rerun5）。

## 验收标准

diff 亲读+证据门语义不弱化论证（逐断言核）+四钉亲跑+95 包 0 FAIL。

## 停止条件

子集匹配与防重复语义冲突（子集放行会造成真实重复观察浪费）→ 带冲突场景分析上交。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
