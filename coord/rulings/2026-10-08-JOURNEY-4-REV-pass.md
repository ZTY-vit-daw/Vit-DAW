# Ruling：JOURNEY-4-REV —— pass（2026-10-08 晚窗，决策侧）

- 执行侧：flash（用户转交）；实现 `16f7a762` @ `port/j4-rev`（worktree D:/Vit_DAW_wt_j4rev，base 509e37f9）
- 落位：cherry-pick → main `c937e27e`；五轮 run 工件（执行侧 4+决策侧复跑 1）保全主树 coord/runs/JOURNEY-4-REV/（不入库）；worktree/分支随裁定清理

## 验收亲核（结构/工件/复跑三层）

1. **diff 结构核验**：6 hunk=参数块 2+泊位接线 3+场景大段 1（+1020 行）——既有场景段逐字零触碰申报属实。
2. **执行侧四轮工件亲读**（summary 关键面逐轮核对与回执相符）：C1=`no_candidate_found`（概率 miss 如实分记）；C2=`judgment_ok`（seat 挂出→status→select A/B→requested→recorded→终态全链）；B1=真实失败（座位身份过期 409→驱动脚本加固：每尝试重发现当前 seat+mismatch 单次重试——加固有据非凑通过）；B2=`judgment_ok_blind`（盲听偏差面断言全过+判后揭示如实记录不设门）。
3. **决策侧复跑（直采退出码）**：`-Scenario journey_ab_judgment -StartKernel` → **exit 0**（run `20261008_201949`），本轮直接挂座走通全判定链（LEG 6 seat found→LEG 7 full chain）——判定腿确定性在第五个独立轮上成立。
4. **§8 纪律**：预算申报在前（canonical N=3 止于成功 2/3 轮；盲听腿至挂卡为限）；分类器一等类含 capability_blocked（FS-CAP 面）；该形 5 轮均未自然触发（未人为构造——卡面约束遵守）；环境中断 0。
5. **执行中修复两处**均为脚本自身缺陷（Get-OptionalProperty 空串语义误读/座位过期加固），处置如实记录。

## 挂账移交（随本裁定）

1. `VitApp/Workspace/agent_runtime_config.json` UTF-8 BOM 致 blind 配置面 fail-closed（运行时文件不在卡域）——小修卡候选（BOM 剥离或写入方修正），gate 记录。
2. D1 渲染 source_ref/preview_ref 文件名含 before/after 字样、盲听会话负载可见——是否算泄漏属 B12 域语义，观察项记录不裁。
3. capability_blocked 形真栈样本仍未采得（累计挂账：FS-CAP 裁定+本裁定）——留自然出现。

## J4 原卡关闭

JOURNEY-4（blocked）复活条件①已走完：FS-CAP 落地→J4-REV 承接 J3 链→判定腿+盲听腿真栈通过——原卡关闭转 done，判定面由 J4-REV 场景承载（`journey_ab_judgment`）。
