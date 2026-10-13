# FIX-TIMEFX-AXISFILTER-1：reverb/delay 轮内治理过滤空集缺陷修复（no_axis_covered）

- 发卡：GLM 主管决策侧 / 2026-10-12 深夜（依据=QUOTA-RECOVERY-LEGS-1 腿①按停止条件上交；[NIGHT-BATCH3 裁定①](../../rulings/2026-10-12-NIGHT-BATCH3-rulings.md)）
- 派发确认：已确认（主管依已验收卡的上交换证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 106；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无（TIMEFX-1 已合入 main）
- 模型分级：L2 / GLM 执行会话（PC，取证+修复+真栈腿）

## 缺陷画像（上交证据浓缩）

D1 reverb 全链（run 20261012_211011，新模型 gpt-6.1-sol）：G1-G7 准入全绿+selected_domains=[reverb]+自由态触发（loop 至 fs6_target_confirmed）+冻结轴单主轴 decay_time（=9ae3aa91 修复目标形态）——但**轮内治理过滤输出空集**→stop=no_axis_covered_candidates→0 干预→"exactly one forward mutation" 门 fail。独立面三绿：pcactl query-v2 eligible=promoted+指纹匹配（leg1_d1_211011/pcactl_query_v2_proR_decay_time.json）+~/.vit 白名单 reverb 节+v2 认证 promoted 覆盖 decay_time+semantics（machine_state_snapshot.json）→**断点在运行时过滤路径，非机器数据**。旧模型 110817 曾命中同门（当时修复后无再跑机会）——本轮为该修复首轮真栈验证=不充分。

## 目标

1. **取证定位**：沿「intent 家族→PCA v2 coverage 轴表→attestation 读取器→候选过滤」链逐层打点，钉死空集产生层（锚点+最小复现）；与 9ae3aa91（TIMEFX-1 收口轮）的修复面 diff 对照。
2. **修复**：按取证定界（候选域=processorattestation/processorintent/chat seam 注册面，领取时与 M3 步骤 3+ 卡核对）。
3. **收口**：修复后 `run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` 真栈 **exit 0**——即 M2-TIMEFX-1（111841）/M2-TIMEFX-2（195448）双挂账+本缺陷三账同收；delay 族同话术一轮可选加分。

## 文件域

以取证定界为准（候选=TIMEFX-1 注册面域）；真栈 PC-RUNTIME-STACK 排队登记。

## 验收标准

取证锚点+修复 diff+D1 reverb 真栈 exit 0（工件含 console+summary）+`go test ./... -count=1` 0 FAIL+gofmt 净；**工件随实现分支入库**（*.log 除外）——回执声明与 git 状态核对。

## 停止条件

取证指向机器数据面（与上交证据矛盾）→ 带锚点上交复核。

- 领取：2026-10-13 09:43 / origin/main=e7089c78（领取时本地 main=6413d23c 落后，以远端为准） / owner=GLM-5.3 执行会话（PC，ZCode 本会话，L2） / 分支=port/fix-timefx-axisfilter-1 / worktree=D:/Vit_DAW_wt_fix106 / 领取提交=本提交（仅含 coord/ 卡状态变更）
- 回执：（commit / run ID / 取证层定位 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
