# G3-REOPEN-COSTCMP-1：probe 计量真实成本对比轮（G3 重开条件①取证）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=[G3 终裁](../../rulings/2026-10-09-G3-FINAL-ruling.md)重开条件①"probe 计量源接入后真实成本对比"+PULL-PROBE-METER-1 已合入（层 a 工具墙充分层））
- 派发确认：已确认（主管依终裁既定条件发卡）
- 验收负责人：GLM 主管决策流
- 池序 73；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / PULL-PROBE-TIER-EXT-1（池序 52，0.25h）建议先收
- 模型分级：L1 / GLM 执行会话（PC，真栈对比轮）

## 目标

G3 终裁不切换的唯一实质依据=成本门槛（轮级 LLM 调用 17 vs 6 + probe 计量 unknown）。计量源已接入后，跑**真实成本对比轮**补齐物理口径证据：

1. 同话术族×双模式（push 生产缺省 vs pull `VIT_DAW_HARNESS=pull`）真栈对比，probe 物理计量（probeSpent/probeCostKnown 账本位）入账——对照 G3-ATTRIB-1 三话术族模式（run 工件在 coord/runs/G3-ATTRIB-1/）。
2. 产出成本口径对比报告：轮级调用数+逐 turn 注入+**probe 物理成本（墙钟，含调度噪音声明）**三口径并列——G3 重开判定输入。
3. **判定归主管**：本卡只交证据与建议，不自行切换 harness 缺省。

## 随轮附加断言（主管补注 2026-10-11 晚夜，M1-5 验收挂账）

任一 run 收尾后调 `/agent/invoke` `mix_report`（M1-5 三视图面）：断言**最近 settle 结论行为有结论形态**（latest_conclusion_id 非空+结论内容与盘上 free_state_conclusions 记录一致）——补 M1-5「有结论」分支的真栈行使（其两轮烟测 round 均无 settle 落盘只走了诚实缺省分支，单测已覆盖，真栈正面证据由本卡对比轮顺带产出，不另开卡）。

## 概率面纪律（AGENTS §8）

对比轮≥2 run（双模式各≥2）；失败分类分记（话术随机分支/环境中断/断言失败）；模型随机分支触发时同话术重跑不换族。

## 文件域

零生产代码或最小脚本扩展（现有 harness_ab 体系模式）；报告落 coord/runs/G3-REOPEN-COSTCMP-1/。

## 验收标准

双模式各≥2 run 退出码记录+三口径对比表+重开建议；工件可回指原始 run。

## 停止条件

计量账本位与 PULL-PROBE-METER-1 合入形态不符 → 上交对齐。

- 领取：2026-10-12 晚窗 / origin/main 3bcaa39a（前置 PULL-PROBE-TIER-EXT-1=e6cacb88 已在 main，merge-base --is-ancestor 实核）/ owner=GLM-5.3 PC 执行流（ZCode 会话，用户指派）/ 分支 port/g3-reopen-costcmp-1 / worktree D:/Vit_DAW_wt_g3rc1 / 领取提交=coord-claim-20261012-g3rc1（卡 mv 协调提交，随本回填推 main）
- 回执：（run ID / 对比报告链接）
- 验收：（裁定文件）
