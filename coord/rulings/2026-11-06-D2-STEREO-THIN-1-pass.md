# Ruling：D2-STEREO-THIN-1（265）验收裁定——条件 pass（产品面全过；exit 0 补跑腿挂今夜非高峰）

- 裁定：GLM 主管决策流 / 2026-11-06 晚窗
- 输入：卡面回执（done，0aba1e4f）+ d2run1 工件（四轮 console+RUNBOOK+证据）+ 合入 7d6a5abe 后主管交叉复跑（build+chat 130.7s+loudnesswatch+mixboard 全绿）

## 一、分面裁定

| 面 | 裁定 | 依据 |
|---|---|---|
| 判据+验证器 | pass | d2AssessStereoRisk 三态纯函数（阈值复用 M1-4 MonoRisk 同款常量）+12 零 LLM 单测；verdict store stereo_check_verdicts.v0 LW 同款纪律（fail-closed/dedupe/裁剪） |
| **置顶申报①（v1 纯披露零挂卡）** | **接受** | 「不发明动作」红线（loudnesswatch/action.go 同款先例）正确——确认链五操作无一能改变声道相关性（gain 对 mono 抵消比例不变/pan 不修复相位），挂无效动作卡=伪交互；动作面待 E3（母带）建成后按剂量定义最小加；queue 节点恒 in_progress=卡面两态允许内 |
| **置顶申报②（不建词表）** | **接受** | 裸话术真栈两轮过 semantic_entry（run1/run3 非 unresolved）——263 教训的正确应用，卡面「能过就不建」兑现；深面入口=显式 d 标记+检查意图词（判据腿零 LLM）；指引面维持等 B4 校准 |
| 产品面真栈 | pass | 门 A（stop_reason/verdict/action_face/零 mutation）run1+run3 脚本化绿+run4 手动绿；门 B verdict store 在盘三轮亲证；门 C 零挂卡零 mutation 脚本化绿；门 D queue 节点点亮持久化注记亲证 |
| exit 0 | **未达成→补跑腿** | 死因分列：run2/run4=排干族瞬态（非本卡面，254 armrun1 同型晚高峰）；run1/run3=本卡门组脚本管线 bug 四处（**均已取证修复入库**：吞异常盲搜/StrictMode 短路序/ANSI 读 UTF-8/mix_queue_view→mix.report）——修复后脚本化 exit 0 未跑过 |
| 计费 | 接受 | 四轮 ≈¥3-6 诚实分账（排干重跑腿 ¥2.5-4.5 非本卡面如实入账；D2 面归属 ≈¥0.8-1.5 卡面口径内） |

## 二、exit 0 处置：选项①——补跑腿挂今夜非高峰自主批次

- §5 门槛（脚本 exit 0）是交付门槛，修复后未跑=门槛未闭合；晚高峰排干族风险高（run2/run4 实证），**补跑窗口=今夜 23:00 后非高峰**，由主管自主批次派子代理执行（单 run ≈¥0.5-1.5，260 相位门分支化已合入=zero-mutation 分支不再拦断）；补跑过=终验闭合，再失败=按 §8 取证上交不重跑。

## 三、域外观察处置

1. **D1 门组脚本卫生开卡 268**：254 时代 D1 settle 门 B/D 脚本模式（ANSI 读 UTF-8 store、mix_queue_view 工具名、needs_confirmation 属性访问）从未被活栈验证，265 run1-4 证明至少三处在 1-mutation 分支同样会失败——与 260（已合入的 LW/settle 相位门分支化）同族但不同段（D1 门组代码），开卫生小卡；
2. 50e6f859 合并说明与 diff 不符（主管 merge message 描述瑕疵，实只含 262 领取笔）——认账留痕，无实质影响；教训=merge message 描述须与 stat 核对。

## 四、验收动作

- 合入 7d6a5abe 零冲突+交叉复跑三包全绿；D2 认知更新：D2 节点由 not_built → **判据+披露薄层点亮（in_progress 态）**，动作面挂 E3。
