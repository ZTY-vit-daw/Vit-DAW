# Ruling：EXPADMIT-EVIDENCE-RULE-1（269）终验裁定——pass（契约链整程真栈走通）

- 裁定：GLM 主管决策流 / 2026-11-07 凌晨（自主批次）
- 输入：实现腿轻验（G7 门本体 0 行亲证）+ verifyrun1 工件 + 合入 f0896ade 后交叉复跑

## 一、终验

| 面 | 裁定 | 依据 |
|---|---|---|
| ②门校验放宽 | pass | 「任一 cited ref 解析到同目标且同 revision」镜像 G7 判据零新造；G7 门本体 0 行 diff 亲证 |
| ①拒绝路径 park 化 | pass（防御层） | 挂确认面选型（terminalAdjudicationParking 先例族）；verifyrun1 未触发（②已解决到达路径）=park 待真实触发面检验，7 例单测钉语义 |
| 验证腿 | **pass（exit 0）** | observe turn 83.7s 直停 `improvement_proposal_confirmation_required`+确认卡落库+**双跳链全绿**（提案确认→工具确认→应用回读 0→-1dB→D1 终态）——262 原断言目标达成；失败链 3/4 环（blocked 改写/终局 done/孤儿收口）零踪迹（goal=waiting_continue+continuation.owed 诚实 withhold）；§8 单 run 兑现+栈协议全履约+计费 ¥0.25-0.5 |

## 二、诚实边界接受

本 run 随机分支引用面宽（含最新观察）=②直接准入；262 精确形态（只引早期观察）的判定性覆盖由配对单测钉死——run 实证=契约链整程，单测实证=规则边界，两证据面互补完整。

## 三、语义收口

- mix_tick 确认卡契约（257 G6）就此**全链闭合**：模型自主提案（262 验证腿实证）→准入（本卡②）→确认双跳→应用回读→D1 终态——needs_confirmation 契约维持的原裁定在真栈完整兑现；
- 262→267（重定性）→269 链条：三层失败链全部处置完毕（L1 弹回=A patch 防御层/L2 反馈=在库无缺陷/L3+新形态=本卡②①）。
