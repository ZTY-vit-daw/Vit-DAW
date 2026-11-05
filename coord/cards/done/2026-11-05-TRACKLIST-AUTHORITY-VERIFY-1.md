# TRACKLIST-AUTHORITY-VERIFY-1：内核回复权威缺陷复验与修复（观察完整性地基）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[2026-11-05-af-line-survey.md](../../reports/2026-11-05-af-line-survey.md) §二/§三）
- 派发确认：已确认（主管发卡；用户 2026-11-05「先勘察，发卡」口头授权）
- 验收负责人：GLM 主管决策流
- 池序 255；目标仓库=主仓 agent；P1 / 0.5-1 天 / 依赖=无；L1 / PC 执行会话；复验优先，修复腿视复验结果

## 背景（勘察锚点）

- 139 TRACK-LIST-GAP-1 诊断（已在 main：coord/runs/TRACK-LIST-GAP-1）钉死形态：**tool face 返回 shadow Summary 忽略 kernel reply；delta-only 轨道在权威快照刷新前成孤儿**（六面探针同帧 tool==shadow 全阶段）。
- 分支 port/track-list-gap-1（ahead5，worktree wt_tracklist1 保留中）有：fix 草稿卡（tool face kernel-reply authority+shadow delta refresh trigger）+harness 修复两件+确定性外部 new/import 两轮+official GO run PASS（run_20261018_175229）。
- 未知项：此后 main 上的 fix-tracklist-shadow-freshness-1 / VSP-SHADOW-REFRESH 系工作**是否已覆盖**此缺陷——须先复验再动手。

## 目标

1. **复验（先行，只读）**：在当前 main 真栈或最小复现面复测 139 诊断形态（外部新增/导入轨→tool face 读数是否反映 kernel reply 而非旧 shadow）——分支 GO run 探针（tracklist_gap_probe.py 已在 main runs 内）可改造复用；
2. 分叉：**缺陷仍在**→按分支草稿实现修（文件域=harness/shadow 投影刷新面，kernel-reply 权威化+delta 触发刷新），补单测+真栈复验 run；**已被覆盖**→证据包收口（复验输出 vs 139 形态对照）关线，分支残余一并定性；
3. 真栈验证（修复腿时）：外部 new+import 两轮确定性探针，tool face 读数与 kernel reply 一致=过；§8 口径跑前成文（预算 ≈¥0.5-2，修复腿 LLM 参与若为零则更低）。

## 约束

- 复验先行：未复验不得动修复面；
- 文件域=harness/shadow 投影面（按草稿），**不触 chat 治理链/loudnesswatch**（与 254 不相交）；
- 探针复用 139 先例（确定性轮，无 LLM 依赖优先）；
- 禁 git reset/clean；分支取材只读。

## 停止条件

- 复验发现形态与 139 诊断不同型→如实分列上交（可能是新缺陷面）；
- 修复根因超出草稿域→取证转卡；
- 真栈断言失败→取证包不重跑。

## 领取

- 2026-11-05 21:02 +0800 / origin/main=c56a1695（领取基线=254 领取提交之上，干净检出）/ owner=GLM-5.3 L1 PC 执行会话（ZCode，255 卡会话，sess 本流）/ worktree D:/Vit_DAW_wt_255impl / 分支 port/tracklist-authority-verify-1 / 领取基线 HEAD=c56a1695，status --short 空、diff --stat 空（干净基线）/ 真栈占用：领取时最近记录=2026-11-05 20:11 M3(225) 释放 TEARDOWN_ALL_ZERO，栈空闲；真栈腿启动前另行登记（§2.2），与 254 错峰 / 领取前只读勘察已核：143 修复 88e88327（A 面公共回包 kernel-reply 权威+shadow_stale 披露）与 vsp-shadow-1 ee4a991e 均在 main 祖先链，三面静态锚点在场——复验判定以真栈行为面为准

## 回执

- **复验结论：139 形态不在当前 main（已被 143 修复 88e88327 超集覆盖且存活三周演进）→ 走卡面分叉②证据包收口，修复面未动（零生产代码改动）**。证据包=[RESULTS.md](../../runs/TRACKLIST-AUTHORITY-VERIFY-1/RESULTS.md)：同 kernel 字节（B6565DCF=139/143 run 同款）+143 修复验收探针原样复用，verifyrun1 单 run **PS1_EXIT=0 probe PASS 七面全过**；决定性帧 S1 r1：kernel=6/hub=6/state=6/**shadow=0/tool=6**（tool≠shadow、tool≡kernel、shadow_stale=true 双计数披露）——139 同帧为 tool=0==shadow≠kernel；silent-stale 阶段=[]、undisclosed=[]；B 面 S1 两轮 heal（0.26s/0.01s）；sanity 三面 ID+uuid 全阶段一致；自愈/S3/同帧 VSP ack 全过。
- 命令与退出码：`run_tracklist_fix_probe.ps1 -RepoRoot D:\Vit_DAW_wt_255impl -KernelExe <stock> -SkipBuild -RunDir <本卡目录>` → **exit 0**（-RunDir 因 ps1:147 参数自清空缺陷未生效，工件原样移至本卡目录+MOVE-NOTE 留痕；脚本缺陷属 143 探针文件域，上交知悉不越域修）。
- run ID=verifyrun1（2026-11-05 20:55:47 起）；被测 VitAgent=EDD851A6…（worktree 5733f17a 构建）/vsphub=3E43FAB1…；worktree HEAD@run=b619c5b8；栈登记 20:54（b619c5b8）→释放 20:57（5642a66f），≤60s 即时复核 20:55:38 TCP=0/PROCS=0，拆栈 teardown remaining=0/0+独立复核 20:57:10 全零；LLM=**0** 亲证（agent_last.log 零 llm 行）≈¥0（§8 跑前成文=RUNBOOK.md，单 run 无重跑兑现）。
- 分支残余定性：port/track-list-gap-1（ahead5，wt_tracklist1）零生产代码，`coord/runs/TRACK-LIST-GAP-1/` 与 main 内容完全一致（cherry-pick 已入册），fix 草稿被 143 超集覆盖→**建议关线清理（决策侧裁定）**。
- 端测边界：本复验覆盖 agent HTTP/ZMQ 接口面，不涉 webui 渲染面与用户旅程（本卡无相关改动）。
- 工件：coord/runs/TRACKLIST-AUTHORITY-VERIFY-1/（RUNBOOK+verifyrun1/ 全件，随 port/tracklist-authority-verify-1 入库）。

## 验收

- （主管回填）

## 验收

- **2026-11-05 pass 合入，卡终结**（GLM 主管决策流）——[2026-11-05-TRACKLIST-AUTHORITY-VERIFY-1-pass.md](../../rulings/2026-11-05-TRACKLIST-AUTHORITY-VERIFY-1-pass.md)：证据链亲核（同 kernel 字节探针复用+决定性帧 tool=6≡kernel≠shadow=0+静态三面锚点互证）；139 形态不在 main 定案，143 修复覆盖存活；分支终局=ledger 残件备份后 worktree+分支主管亲关（19→18）；fixup 自体手术不构成违规；ps1:147 RunDir 缺陷记入脚本卫生清单。
