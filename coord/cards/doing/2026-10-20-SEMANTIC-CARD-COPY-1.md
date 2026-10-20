# SEMANTIC-CARD-COPY-1：M2 语义链卡面文案合同（措辞诚实/指名目标轨/中文化/去代号）

- 发卡：GLM 主管决策流 / 2026-10-20 晚窗（依据=M2 体验场次①实录 F2+F3：[2026-10-20-m2-experience-session-1.md](../../reports/2026-10-20-m2-experience-session-1.md)）
- 派发确认：已确认（用户 2026-10-20 晚窗「可以发卡」）
- 验收负责人：GLM 主管决策流
- 池序 148；目标仓库=D:\Vit_DAW；P3 / 0.5 天 / 依赖=无（与 147 文件域相邻，串行领取）
- 模型分级：L1 / flash 可接

## 缺陷（场次实录实证，四处）

1. **「唯一可加载」措辞失真**：插件选择卡称「当前工程里唯一可加载的混响是 FabFilter Pro-R」——实测插件库 994 项含数十种混响（H-Reverb/RVerb/TrueVerb/Abbey Road Plates/MannyM Reverb/ZamVerb/IR1/Lofi Space/ValhallaSupermassive…，BridgeReplies/20261020_181747_plugin_list_available_library）。真实语义=「语义可控准入面仅 Pro-R」。可加载≠可控，表述与事实不符。
2. **装载确认卡不指名目标轨**：「将加载效果器 Pro-R 到**当前轨道**」——违反 M2 体验清单 §3.2 提案卡合同（指名目标轨、参数、小步变化量）；目标轨决策（bass）埋在上一条卡的推理正文里，确认面不复述。
3. **typed controller 确认卡英文原文**：node[5] "The semantic plan is ready. No parameter has been written. Confirm to execute the typed controller."——违反清单 §4 自然中文契约。
4. **能力边界终止文案泄漏内部代号**：node[9] 把 failed_gate_ids 原样拼进正文（G3_project_scan、G5_frontier_established、G6_target_evidence、G7_fresh_revision_bound_refs、G8_target_consistency）——锚点 `agent/internal/chat/goalrunner_chat.go:2096-2104`（freeStateCapabilityBlockedReply strings.Join(failed, "、")）；违反清单 §4「不出现内部代号」。

## 目标

四处文案合同化：①准入语义措辞改「语义可控的混响当前只有 X；插件库另有 N 款混响可装载但未接入语义控制」（诚实边界不缩水不夸大，数字来自 attestation 面）；②装载/提案卡指名目标轨名+ID+参数+变化量（数据已在 payload target_ref，渲染层补齐）；③控制器确认卡中文化（复用既有用户语词表）；④边界文案 G 门代号→用户语（如「工程扫描/前沿建立/目标证据/证据新鲜度/目标一致性 五项准入检查未过」），代号保留在结构化 payload 不进正文。

## 文件域

`agent/internal/chat/plugin_recommendation.go`（①②③渲染处）+ `agent/internal/chat/goalrunner_chat.go`（④文案映射）+ 测试/快照；越域即停。

## 验收标准

`cd agent && go build ./...` + chat 包 + 全量 `go test ./... -count=1` 0 FAIL + gofmt 净；受影响文案快照更新绿。纯文案面，端测边界=单测域，回执声明。

## 停止条件

文案模板被多处共享致改动面扩散（如 G 门文案映射被非用户面复用）→ 上交清单收窄，不硬改。

- 领取：2026-10-20 19:06 +0800 / origin/main=9b9854ad152bf8c069bc0dbd31fbd306416c5249（派发基线 a958fc40 已被 146 回执推进，领取时重核远端）/ owner=GLM-5.3-Flash ZCode PC 执行会话（本卡会话，用户指令指定领取）/ 分支=port/semantic-card-copy-1 / worktree=D:/Vit_DAW_wt_scopy1 / 领取提交=本提交。**分段执行**：④腿（goalrunner_chat.go 边界文案去代号）先行；①②③腿文件域 plugin_recommendation.go 与 147 重叠，等 147 合入共享 main 后同卡续做（派发 a958fc40 载明串行）。领取时核对：卡在 todo 派发已确认（用户 2026-10-20 晚窗「可以发卡」）、依赖无；doing 五卡文件域与本卡零冲突（147=plugin_recommendation.go 域未合入、149 零生产代码、150 认证战役零生产代码、HARNESS-STRUCT-NORMALIZE=harness 域、GEN-SUNO=webui 域）；纯单测域不占真栈，无需登记 PC-RUNTIME-STACK.md
- 回执：（commit / 测试名 / 端测边界声明=纯单测域）
- 验收：（裁定文件 / 验收 commit）
