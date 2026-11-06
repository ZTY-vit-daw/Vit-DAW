# D1GATE-SCRIPT-HYGIENE-1：D1 settle 门组脚本三处卫生修补（254 时代模式活栈验证欠账）

- 发卡：GLM 主管决策流 / 2026-11-06 晚窗（依据=[265 回执](../../cards/done/2026-11-06-D2-STEREO-THIN-1.md) 域外观察②+[265 裁定](../../rulings/2026-11-06-D2-STEREO-THIN-1-pass.md) §三-1——254 时代 D1 settle 门 B/D 脚本模式从未被活栈验证，265 四轮证明至少三处在 1-mutation 分支会同样失败）
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 268；目标=scripts/run_free_state_d1_smoke.ps1（D1 settle 门 B/D 段）+如需配套 py；P2 / 0.25 天 / 无依赖（与 265 已修的 D2 门组四处不同段）；L1 / PC；零真栈（构造验证，模式对齐 265 修复先例）

## 目标

1. 三处修补（对齐 265 在 D2 门组的同款修复模式）：①ANSI 读 UTF-8 store（verdict/needs_confirmation 文件读取编码）②`mix_queue_view` 工具名→现役名（`mix.report` 族）③needs_confirmation 属性访问路径（continuations[].pending_interaction 面 vs goal.status 面——参 262 L3 取证锚点）；
2. 构造验证：桩工程/离线 store 走 D1 settle 门 B/D 两门断言路径（零真栈零 LLM，模式参 260 三 probe）；
3. 回归：260 已合入的相位门分支化语义零回退。

## 约束

文件域=run_free_state_d1_smoke.ps1 D1 settle 门段+如需 py 配套+构造验证工件；禁动 D2 门组（265 已修）；禁动 runner 断言语义本体（只修读取面/工具名/访问路径）；§10 防污染纪律照用。

## 停止条件

构造验证暴露第四处同族缺陷→如实记录一并上交（属同段可顺手修，跨段则另开）；断言语义问题（非读取面）→取证上交转裁定。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
