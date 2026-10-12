# M2-TIMEFX-2：reverb/delay 成对渲染探针+FXM 差分生产者（时域效果器·差分侧）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=用户三线同步；MIX-CAP-RECON §2.B 就位度结论"通用时域差分接入 ❌——压缩器样板已立其余族待复制"+roadmap L2-4）
- 派发确认：已确认（用户 2026-10-11 晚窗同步）
- 验收负责人：GLM 主管决策流
- 池序 72；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 1-2 天 / **M2-TIMEFX-1 合入后领取**（有语义参数面才有差分对象）
- 模型分级：L2 / GLM 执行会话（内核 C++ 探针+agent 接线双面）

## 目标

1. **内核 reverb/delay 成对渲染探针**：复制 compressor_dual_tap_probe 样板（CommandDispatcher.cpp:3149→TransportAudioService.cpp:1405 一带；bypass/processed 成对渲染+身份串模式（com2rp.v1 先例）+窗口上限+作用域强校验）到 reverb/delay 族——领取时以 grep 实际行号为准。注意透明性探测对 reverb/delay 特有参数面的障碍处理（现探针把 automationcurve 当障碍的前例，TransportAudioService.cpp:57-61）。
2. **agent 侧 FXM 差分生产者**：fxm.Input 结构齐备但全仓无生产路径（唯一消费点 mixboard.go:3992-4011 反序列化）——建自动 bypass/processed 喂入：语义控制（M2-TIMEFX-1）写参后自动触发成对探针→组装 fxm.Input（Chain/Conditions/Baseline/Processed）→FXM 投影产出。
3. **差分断言**：同源同窗同路由 bypass vs processed 差分有效性（fxm 差分门控 source_revision/window/format 不一致即 blocked/stale 的契约测试）。

## 文件域

VitApp/Source/Service（探针注册+渲染）+ agent/internal/fxm（生产者接线）+ 单测；与在飞卡核对文件域（内核注册区与编曲线/生成线共用面，PROTOCOL §2.1）。

## 验收标准

- 内核构建 exit 0；探针真栈往返（发令→成对回执）；
- FXM 投影产出断言（差分测量 RMS/LUFS/Peak/Crest 真实数值非零且 bypass≠processed）；
- `go test ./... -count=1` 0 FAIL + gofmt 净；真栈烟测 exit 0（扩展现有 ps1 体系）。

## 随轮附加腿（主管补注 2026-10-12 晚窗，M2-TIMEFX-1 验收挂账）

真栈烟测轮顺带复跑 `scripts/run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` 一轮——补 M2-TIMEFX-1 run 111841 API 额度中断腿（[rulings/2026-10-12-EVENING-BATCH1-rulings.md](../../rulings/2026-10-12-EVENING-BATCH1-rulings.md) ①）：修复后全链单 run **exit 0 即收挂账**，工件归 run 目录并在回执单列。若 API 额度仍未恢复则如实记录环境中断，不阻塞本卡其他验收项。

## 停止条件

dual-tap 样板结构与锚点不符（内核已重构）→ 锚点上交。

- 领取：2026-10-12 18:46 +0800 / origin/main 76ace424 / GLM-5.3 PC 执行流（L2，ZCode PC 会话，M2 时域线）/ 分支 port/m2-timefx-2 / worktree D:/Vit_DAW_wt_m2_timefx2 / 领取提交=本提交（领取前核对：远端 doing 为空无文件域冲突；M2-TIMEFX-1 dbac17ea 为 origin/main 祖先=依赖合入亲验；PC 栈远端状态空闲）
- 回执：（commit / run ID / 差分数据样本 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
