# Ruling：JOURNEY-3 验收 pass（2026-10-07，决策侧）

- 对象：自由态 NL 旅程烟测（实现 a943b188，cherry-pick → main **f25f6682**）
- 裁定：**pass**（建设卡口径）；止损上交的产品发现立卡 FS-NL-PROPOSAL-RECON-1（P2，抖动勘察）。

## 证据链（决策侧亲核）

1. **diff 亲读**：scripts/dev_agent_smoke.ps1 单文件 +707/-4（卡面域内，agent 代码零改动）；场景接线（不入 all/强制 -StartKernel/泊位四件沿 J1）+四个预算参数+nudge 机制（journey1 证据工具先例）核对在位；exit 门设计=确定性子面（J1 腿逐字复用+条件确认 hop 结构断言），NL 形状失误分类记录不炸 exit——卡面口径逐条落位。
2. **工件亲读**：summary_N3（schema journey3.n3_summary.v1，三轮逐字段）+R3 journey_nl_summary（11 事件/audition=0/mix_tick=0/applied_like=0/时间线）；R1 chain_stall=驱动缺陷（v1 无 nudge，v2 修）过程记录诚实；R2/R3 分类修正（classifier v3）留痕。
3. **§8 纪律合规**：N=3 全跑、成功 0/3 如实记录、失败分类分记（chain_stall=1/终态无治理面提案=2/显式 no_candidate=0/纯文本=0/环境中断=0）、连续 2 轮同形即止损上交（锚点清单两文件+处置选项）——止损线执行教科书级；"轮外瞬败不计轮次"（调用侧路径失误无栈启动）处置正确。
4. **我方复跑（关键）**：主树负向门先验（无实现时场景正确被拒）；cherry-pick 后 `-Scenario journey_free_state_nl -StartKernel` **exit 0**（run **20261007_202322**）——确定性子面全绿，且 **NL 轮完整走通严格成功链**：提案挂卡（mix_tick_confirmation/interaction_card）→确认 hop 1 次（gate=pass）→应用→A/B 卡（success_strict_card_chain=True+success_policy_chain=True，脚本自判面）。**第 4 轮达成卡面成功条件**——聚合 1/4。
5. **泊位纪律**：执行侧三轮+决策侧一轮均自起自拆、领取时端口零监听申报；主树单流期间与 IMPL-C（未启）无冲突。

## 附加裁定

1. **成功条件的验收口径**：执行侧 0/3 + 决策侧复跑 1/1 全链——按卡面（验收标准=3 轮工件+exit 判定正确+如实记录+决策侧复跑确定性子面）判 pass；成功条件本身由 R4 补足证据，**不把 0/3 改写、也不因 R4 抹平 R2/R3 的发现**。
2. **产品发现立卡**：FS-NL-PROPOSAL-RECON-1（P2，已随验收入池）——R3 文本提案未挂治理面的抖动勘察，H3（模型概率行为）先验因 R4 显著升高，产出=抖动率评估+最低成本加固建议。
3. **J4 前置已解锁**：J3 链已实证可产出确认卡与执行链；J4 立卡时按设计 §3 先核 A/B candidate 确定性装卡入口（R4 的 A/B 卡挂出即其输入面样本）。
4. **旅程门槛层覆盖**：站点 1-4 确定性回归+站点 3 NL 面概率探针就位（journey_free_state_nl 入场景库）。
