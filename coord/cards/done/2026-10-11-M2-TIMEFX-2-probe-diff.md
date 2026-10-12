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
- 回执：实现 25a2b91a+9cd14877@port/m2-timefx-2（已推）；真栈冒测 run=artifacts/timefx_probe/20261012_195100（reverb）+20261012_195128（delay）双 **PASS exit 0** 各 28/28 门（工件精简件 coord/runs/M2-TIMEFX-2/）；差分样本=Pro-R：baseline rms −30.94 vs processed −30.32 dBFS（rms Δ+0.59/peak Δ−5.74/crest Δ−6.33 dB，bypass≠processed）；Timeless 3：rms Δ+0.116 dB（peak Δ0 如实——延迟重复峰不超原峰）；直发探针往返=发令→_ready 事件→工件 sha256 逐字节核对过；内核构建 exit 0+RefSchemaTests（含 T5c timefx ref）+RenderWatchdogTests exit 0；go test ./... -count=1 exit 0（95 包 0 FAIL）+gofmt 净。**设计发现**：Pro-R/Timeless 内部调制致复渲染真差（correlation 0.946/0.991）≠测量误差→确定性证明诚实降级 partial+方差数值披露+差分照测（同窗单渲染对仍有效），硬门仅留格式/NaN/零信号/覆盖；agent 侧 partial→投影 partial、可观察不可 preflight。**附加腿**：D1 reverb 复跑 run 20261012_195448 exit 3=预扣费额度失败（余 0.000190 需 0.011956）——API 额度仍未恢复，环境中断如实记录（同 M2-TIMEFX-1 run 111841 因），挂账腿继续挂 [等待用户]，不阻塞本卡。**端测边界声明**：覆盖面=内核 ZMQ 探针往返+agent HTTP 语义写参面+pub 事件面（真实 Godot 拉栈）；webui 渲染面与用户旅程未涉（本卡零 webui 改动）；冒测迭代修复=域感知写参/rack x/y/非 JSON 事件跳过（脚本面，非功能缺陷）。
- 验收：**pass（D1 额度腿继续挂账→池序 102）** / 2026-10-12 晚窗（[rulings/2026-10-12-EVENING-BATCH2-rulings.md](../../rulings/2026-10-12-EVENING-BATCH2-rulings.md)①；决策侧合并态亲跑 95 包 0 FAIL+工件差分数值亲证；复渲染方差降级=设计发现采信登记；impl merge 合入 main；**M2 时域线收口**→体验场次触发条件就位）
