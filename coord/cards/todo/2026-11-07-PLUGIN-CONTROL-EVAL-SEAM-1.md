# PLUGIN-CONTROL-EVAL-SEAM-1：能力评估 A/B 纯臂四缝（test-only，B4/C4 钩先例）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[299 PREP seam_anchors](../done/2026-11-07-PLUGIN-CONTROL-EVAL-PREP-1.md)+plan.json+[300 P 基线落地](../done/2026-11-07-PLUGIN-CONTROL-EVAL-RUNNER-1.md)——P=5/6 但 T2/T3/T4 首动作均 typed 接管=产品语义机器承重，模型裸能力未测；四锚点污染面已定位）
- 派发确认：已确认（主管发卡·装置类，B4/C4 ablation 钩先例=答辩前同类先做过的 test-only env 模式）；验收负责人：GLM 主管决策流
- 池序 305；目标仓库=主仓 agent（四锚点 test-only 缝）；P1 / 0.5-1 天 / 依赖=300 runner 在库（其 driver 的臂面已备 A/B 钩位）；L2 / PC；零栈

## 四缝（全 Go 侧，单 env 总开关 `VIT_PLUGIN_CONTROL_EVAL_SEAM`，默认 unset 字节不变）

1. **S1 typed 接管旁路**：goalrunner_chat.go explicit_control 强制 typed 路径（:260-307 域）——缝开时控制类请求放行通用工具循环（不进 typed planner）；
2. **S2 语义字段摄取剥离**：内核回复烘焙字段（normalized_role/display_group/control_relevance/template_role/alias 语义覆盖——PREP 字段来源三分表）在 agent 摄取/披露面剥离（插件原生 unit/range/enum/当前值保留）；
3. **S3 工具描述净化**：catalog.go:1173 域工具描述中的语义引导文本净化（裸参数描述保留）；
4. **S4 message loop typed 路由**：message_loop.go:4344-4351 typed 路由规则旁路。

## 目标

1. 四缝实现（每缝独立可评注释；默认 unset 生产行为字节不变——B4/C4 配对单测三态同款：unset=现行为/env=1=旁路生效/仅精确 "1"）；
2. 配对单测：缝开时 A 臂条件亲证（typed 不接管+语义字段不披露+裸工具描述）+缝关字节不变+逐缝开关独立性（如四缝可独立开关则各配一测，实现裁量）；
3. 与 300 driver 联调：dryround 在缝开 env 下跑通一案例（模型面可后验）；
4. 回归：`go build ./...`=0；`go test ./internal/chat ./internal/agentloop ./internal/tools -count=1` 全绿零回退。

## 约束

- 文件域=四锚点文件±测试；**生产行为默认不变红线**（生产启动路径永不设置该 env）；不触 300 scripts（其 driver 已备钩位）；
- 禁止 git reset --hard / git clean；不动主工作树；零真栈（dryround 联调如需栈则候队列登记）。

## 停止条件

某缝的旁路与在案治理裁定冲突（如 typed 接管曾有安全裁定）→该缝取证上交单独裁定，其余缝不受阻。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
