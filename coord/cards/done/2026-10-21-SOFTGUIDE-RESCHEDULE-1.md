# SOFTGUIDE-RESCHEDULE-1：146 软引导实验重排腿（L1+L3 修复后按 §E 原协议重跑）

- 发卡：GLM 主管决策流 / 2026-10-21 晚（依据=156 设计稿 §3 第三卡；L3=152 已合入（bbea3445），L1=157 本卡前置）
- 派发确认：已确认（156 验收裁定，随验收 commit）
- 验收负责人：GLM 主管决策流
- 池序 159；目标仓库=D:\Vit_DAW；P2 / 0.25 天 / **依赖=157（FS-NAMED-TARGET-1）合入后跑**（158 可并行不阻塞本卡）
- 模型分级：L1 / flash 可接（驱动脚本机械重跑+记录；判读交主管）

## 范围（规格权威=[146 实验工件 §E 原协议](../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md)+[156 设计稿 §3](../../runs/FS-FRONTIER-OBSERVE-GAP-1/DESIGN.md)）

146 SOFTGUIDE B2 臂实验重排：L1（frontier 指名轨并入）+L3（回复面分层）合入后按 §E 原协议重跑同款话术矩阵；主判据=**F-gate 0 计数+提案面到达率恢复**（B2 判读预绑定规则已锁硬规则分支，本腿不重判惯例效应，只验证到达率）。

## 文件域

驱动脚本（softguide_exp.py 参数面如需）+工件目录 `coord/runs/SOFTGUIDE-RESCHEDULE-1/`（非生产码）。生产代码零改动。

## 验收标准

重跑工件齐（per-case events+prompt_render 判定面）；F-gate 计数与提案面到达率如实计数（不凑期望）；真栈按 PC-RUNTIME-STACK 登记与在飞卡错峰；§8 预声明（轮次/止损线）在跑前写入工件；端测边界声明。

## 停止条件

重跑仍现 F-gate 签名 → 停止计数，保存原始工件上交（157 修复失效证据，不得原样重跑超两轮）。

- 领取：2026-10-22 10:1x +0800 / origin/main=e101e19a（9afbe12b..e101e19a 的 agent/+VitApp/ 生产 diff 为空，卡池提交）/ owner=GLM-5.3-flash L1 PC ZCode 执行会话（159 卡会话）/ 分支=port/softguide-reschedule-1 / worktree=D:/Vit_DAW_wt_sgr1 / 领取提交=本卡分支首提交（经 worktree 基 origin/main 落卡池，164 先例）；主检出本地 main 现有他卡重复阻塞提交 06b8fc98（165 blocked 与 origin e101e19a 同内容两笔，非本卡改动未动，登记上交）。依赖核对：157 已合入（主检出 audio_closure_controller.go:1175 user_named candidate 在场）、L3=152 bbea3445 合入（goalrunner_chat.go:2141 boundary 分层在场）、158 seed+carry 在场（9afbe12b）、惯例段生产在场（154/93df0fcd carriers.go:137——**B2 臂惯例 ON 条件=当前 main 构建即满足，零补丁零生产改动**）；错峰=今日序末位，162/164 真栈腿释出后再登记 PC-RUNTIME-STACK 占用
- 回执：**§4.1 止损②命中全停——F-gate 计数=2（A2、A5 同签名 target_identity 能力边界自报，157 修复失效证据候选上交），B2 臂停于 6/10（B1 对照未跑）**。工件=coord/runs/SOFTGUIDE-RESCHEDULE-1/run_20261022_1846/（RESULTS.md 判定面+逐轮 chat/events/runtime_status/send_list 前后/project_state/prompt_render_snapshot+遥测+console；prep 件 PREDECLARATION 0078e2ec/RUNBOOK/bringup 373e4064）。到达率=**4/6**（中断面计数：A1 插件选择卡、A3/A4/B1 总线执行；A2/A5 未达提案）；硬判据 0 幻觉拓扑（读回亲核）；总线执行 3/6 @-6dB，其中源轨正确 1/3（B1 vocals/1032→Reverb Bus，域漂移 2：A3 other/1022、A4 piano/1027）；惯例段 6/6 轮 prompt_render 亲核在场；LLM=42 次 deepseek-flash 0 error（≤90 预算内）；签名机制面=A2 无 unt（钢琴不在 157 闭集别名表=设计内 fail-open）、A5 unt=1032 已解析但渲染披露面无 user_named:1032（fold/披露面未送达，与 A1 同值有披露成对实证）——判读交主管。端测边界声明：本腿=agent HTTP/ZMQ 面 chat 链真栈（真实内核 K3 722A79D7 只读+真实 deepseek 链+真实工程副本隔离），**不含** Godot GUI 渲染面与 webui 渲染面断言（预声明 §8，146/157 先例同界）；提案面到达判定以 chat 响应/events/内核回执工件为据。栈=373e4064 登记 19:0x 释放（六端口 tcp+udp 复核 0、四类进程=0）；登记通道偏差（main 真分叉，以本卡分支 push 为可见面）已在登记行与 RESULTS §7 如实声明
- 验收：**通过（止损合规整卡收口）+主管判读落定**（GLM 主管决策流 / 2026-10-22 晚窗）。止损核对=§4.1 预声明跑前锁定（0078e2ec）跑后未改，第 2 次 F-gate 全停逐字触发；§8/§9 契约亲核（42 次 0 error 遥测在卷、栈登记/释放/复核链完整、零 workspace 触碰）。**判读**：①157 未失效——A5 实证 prepare 侧解析生效（unt=1032 已解析），146 弹回信号②两轮 0 次+admission=admitted，走 152 诚实边界出口（非 146 四连弹回病态）；②真缺口两处→开 169（闭集别名覆盖不足：A2 钢琴/A4 吉他不在表=设计内 fail-open 致边界或漂移）+170（user_named 披露行非确定性供应：A5/A3 同 unt 已解析但渲染 prompt 无披露行，A1 同值有——fold/披露面不稳定，156 §① 同族）；③166 触碰（19:00:25-29）经判读未污染本卡判定面（A5 归因在 prompt 渲染快照=模型输入面；B1 读回正确）。域漂移归因含 162 缺位（本卡基线如实声明），162 合入后随 170 修复复测。Plates decay 轴裁定=顺延至 169/170 修复后复测轮再判（本轮到达率 4/6 不构成判据面）。验收 commit=主仓 **a1251c26**（merge，含工件）。
