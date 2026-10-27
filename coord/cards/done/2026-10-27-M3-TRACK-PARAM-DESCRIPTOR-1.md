# M3-TRACK-PARAM-DESCRIPTOR-1：内核描述符面扩轨级参数（B 案，重设计链首卡）

- 发卡：GLM 主管决策流 / 2026-10-27（依据=[decisions/2026-10-27-envelope-gui-redesign-rulings.md](../../decisions/2026-10-27-envelope-gui-redesign-rulings.md) 用户裁定 B+稳定/高效/质量优先原则；196 探测结论=轨级 volume/pan 曲线面全周期可用+可寻址族恰为两参）
- 派发确认：已确认（用户裁定链）
- 验收负责人：GLM 主管决策流
- 池序 201；目标仓库=**主仓 D:\Vit_DAW**（VitApp C++ 域）；**P1** / 1 天 / 依赖=无
- 模型分级：**L2 / PC 执行会话**（内核契约变更，质量优先口径）
- 并行与资源：真栈腿 §2.2 登记+三纪律+188 守卫；与前端重设计卡（202+）文件域不相交，但 202 消费本卡产出=**本卡合入后 202 才可领取**。
- 领取：2026-10-27 14:39 / origin/main=9671c1b1 / owner=ZCode GLM-5.3 PC 执行会话（201 卡会话）/ 分支 port/m3-track-param-descriptor-1 / worktree D:/Vit_DAW_wt_tpdc1 / 领取提交=coord/m3-track-param-descriptor-1-claim（本提交）

## 目标

1. **描述符面放行轨级参数**：`get_plugin_parameters` 对轨级内置插件（Volume & Pan Plugin，type=volume，196 实证每轨恒挂）回参数行——恰 volume/pan 两行（**与 resolve 可寻址面镜像**：196 逐字留证 master/trim/mute/gain/Level Meter 不可寻址，不得虚列）；行 schema 同外部插件参数行（id/name/unit/值域/当前值+envelope_point_count），envelope_point_count 值源=同参数 getCurve().getNumPoints()（190 单点先例 appendEnvelopePointCountsToParameterDescriptors）。
2. **契约变更边界**：外部插件行**零改动**（diff 级断言）；内置插件旧 error 行为（"Plugin parameters are only available for external plugins"）消失=已裁定契约变更（decisions 在卷）；兼容按 AGENTS §11 纯增行处理，未知消费面忽略即可。
3. **消费方全量回归盘点**（质量优先）：逐面盘点 get_plugin_parameters 消费方——agent 观察/PCA/grabber、前端 rack 层（rack_control_layer_rules.gd）、既有冒测（190/196/192/189 面）——「多出的轨级行」影响+是否存在依赖旧 error 语义的判定（有则如实披露上交，不擅自改消费方）；各面只读回归跑全绿。
4. **真栈冒测**（190 模式扩展）：双轨→各回 volume/pan 行+count=0→set_points 三点→count=3（含跨轨隔离）→clear_region→回 0；外部插件行 diff 级零改动；**连续两轮全绿**（稳定性口径，不以单次 exit 0 交付）。

## 文件域

`VitApp/Source/Service/PluginRackControlService.cpp`（get_plugin_parameters handler+描述符构建段）+`scripts/`（既有冒测扩展优先，190/196 模式）。**agent/ 前端仓零改**（消费方回归只读跑）。越域即停。

## 停止条件

- 描述符构建点与 190 锚点（appendEnvelopePointCountsToParameterDescriptors 族）不符→锚点上交；
- 轨级参数在内核侧无现成单位/值域元数据（需新造元数据面=设计扩大）→两案上交主管裁定；
- 消费方存在依赖旧 error 语义的硬依赖→证据上交裁定，不擅自改。

## 验收标准

内核构建 exit 0+冒测两轮全绿+消费方回归清单全绿+工件入 coord/runs/M3-TRACK-PARAM-DESCRIPTOR-1/（§9 契约）+端测边界声明。

## 回执

- 实现 commit：**port/m3-track-param-descriptor-1** @ e9a49fd0（内核+冒测扩展，3 文件 +359/-2，diff 亲读）；工件 commit 同分支 f6933ed4（分支已推远端）。
- 内核构建：exit 0（worktree 自建 VitApp.exe 31.7MB，VIT_TRACKTION_ENGINE_DIR=主检出只读；仅引擎侧既有 C4819 警告）。
- 真栈冒测：**连续两轮全绿**（R1 15:07+R2 15:11，ps1 exit 0 ×2）——201 新腿（`-TrackParamDescriptorSmoke`，断言体=automation_curve_smoke.py `--probe-descriptor-tracks 2`，**42/42 checks×2**：恰 volume/pan 两行+schema 全字段+值域 0..1/-1..1+count 0→3→0+跨轨隔离+同源对照）+196 面回归（TrackSourceProbe anchor 2/2×2）+190 面回归（EnvelopePresenceSmoke 含**外部插件行 diff 级零改动**×2）。R2 首次尝试=188 守卫活触发拦停（R1 内核残留；非产品轮，归属亲核后手拆，console 留证）。
- 消费方盘点：九面全绿（agent 工具面/观察/PCA/grabber、前端 rack 层、车道层、190/196/189/192 冒测面）——**零旧 error 语义硬依赖**；196 脚本条件断言自然兼容（真栈回归实证）；agent/ 前端仓零改。清单见 RESULTS.md §二。
- 工件：coord/runs/M3-TRACK-PARAM-DESCRIPTOR-1/{trackparam_20261027_r1,trackparam_20261027_r2}/+RESULTS.md（§9 契约：完整命令/退出码/run 时 HEAD 与二进制哈希锚/隔离面/守卫轮留证）。
- 端测边界声明：覆盖=内核 ZMQ 命令面真栈（两轮）+agent 最小腿；**不覆盖**=webui 渲染面/用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界）+Godot 前端腿（202 域，本卡前端零改口径下静态盘点替代）+agent LLM 面（LLM=0 轮）。
- 真栈登记/释放：d71315ff/ebc2981e（coord/m3-track-param-descriptor-1-stack→main）；三纪律+VIT_PROJECT_XML 隔离亲证见 RESULTS.md §四。

- 验收：**整卡 pass**（GLM 主管决策流 2026-10-27；rulings/2026-10-27-M3-TRACK-PARAM-DESCRIPTOR-1-pass.md；合入见 merge 提交。202 依赖解锁）
