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
- 回执（**④段完成，卡留 doing**）：实现 commit=96cb44a9@port/semantic-card-copy-1（已推 origin；3 文件 +96/-8：goalrunner_chat.go 门代号→用户语映射 freeStateGateUserNames+freeStateCapabilityBlockedGatePhrase，回复改「本次有界改进提案未通过自由态准入门（工程扫描/…/目标一致性 N项准入检查未过），任务在能力边界停止…」，未知门回退「其他」保计数诚实；代号仍在结构化 WorkflowData["free_state_admission_receipt"].failed_gate_ids，payload 零变化）；测试=surface+closure 两测试 Face3 断言反转（原断言要求代号进正文=固化缺陷，改为用户语在/代号零泄漏）+新增 TestCapabilityBlockedReplyRendersGateNamesWithoutCodenames（卡面例句逐字/顺序保持/两项~八项计数/未知门回退）；验收=go build ./... OK+全量 go test ./... -count=1 96 包 0 FAIL+gofmt 三改动文件 CR 剥离后幂等净（本机 worktree=CRLF 环境性全包列名，非本卡漂移）；端测边界声明=纯单测域，未占真栈；停止条件核查=文案模板仅此一处用户面无扩散；**①②③腿（plugin_recommendation.go 域）待 147 合入共享 main 后同卡续做**
- 验收（**④段 pass / 2026-10-20 晚窗，GLM 主管决策流**）：diff 亲读（G1-G8→用户语映射+未映射折叠「其他」保计数+代号留结构化 payload；未知门回退设计正确防未来代号泄漏）；Face3 断言反转定性=修复缺陷固化（原断言要求代号进正文）非弱化，采信；实现 96cb44a9 cherry-pick 入 main。**①②③腿解锁条件已满足**（147 已验收合入共享 main）——同卡续做，完成后再走整卡验收。
- 回执（**①②③段完成，整卡待主管验收** / 2026-10-20 晚窗，同执行会话续做）：领取核对=主管解锁指令（147=18ea58ec 已合入共享 main，①②③解锁同卡续做）；基线重取 origin/main=18ea58ec（④段 96cb44a9 内容已在 main，`git checkout -B port/semantic-card-copy-1 origin/main` 移分支指针，零内容丢失；领取时 worktree 干净）。实现=4 生产文件+1 测试文件（+157/-6 净）——①plugin_recommendation.go 渲染层确定性准入边界句「准入边界：本机插件库检索到 T 款X，当前通过 PCA 语义控制准入的只有/其中有 M 款…；其余 N 款可装载，但未接入本类语义控制，不在本次推荐面」（两断面计数由 caller 在目录面/准入面实测冻结入 requestContext plugin_recommendation_admission_boundary，payload 顶层 admission_boundary 同数字可审计；T==M 全准入/N==0 两形态不夸大不虚构；dynamics 宽检索面标注「动态类效果器」不缩窄；planPluginRecommendation prompt 增一条禁则=禁「唯一可加载/库面可装载性」声明，边界句权威在渲染层）；full-access 自主选择回复缀同句（full_access_semantic_chain.go）。②plugin_grabber_workflow.go 装载确认卡「将加载X Y 到轨道「名」（ID: id）」（名解析=state tracks 行→ID 一致的 selected_track_name→裸 ID 三级降级，绝不回退「当前轨道」）+「本次只装载插件，不会写入任何参数」（装载卡无参数写，按实声明）+plan.WorkflowData 增 track_name。③semantic_dynamic_workflow.go typed controller 确认卡中文化：标题「确认执行语义参数控制器」、回复「语义控制计划已就绪，尚未写入任何参数；确认后才会执行参数控制器，取消则不会改动工程。」、动作「确认执行/取消」（复用 c2_dynamic_batch/improvement_proposal 既有词表样式）；同卡面取消回复一并中文化「已取消语义参数控制；没有修改任何参数。」；结构化 preview payload 保持机器可审计不做散文翻译。**文件域偏差如实记录**：卡面列①②③渲染处=plugin_recommendation.go，实测②渲染在 plugin_grabber_workflow.go:218、③在 semantic_dynamic_workflow.go:816-818（实录 F2/F3 锚点所指唯一用户面，均在 chat 包内，无模板共享扩散）——按主管解锁指令以三腿结果与实录锚点为准执行，越域与否交主管验收裁定。测试=新增 semantic_card_copy_test.go 8 测试（边界句四计数形态逐字+宽面标签/选择卡含边界句+结构化数字+交互卡 Body 同句/无披露直接调用面零发明/full-access 回复含边界句与主推荐/装载卡指名「到轨道 1007」+无参合同+plan track_id/轨道名纯函数四分支含 ID 不匹配拒绝/确认卡标题回复动作逐字+语义状态保持/取消回复逐字+Cancelled 语义）；既有钉住面全保持（full_access_semantic_chain_test「将加载」「确认后」Contains 兼容新措辞；semantic_dynamic 既有绑定隐藏/模型自证断言不受影响）。验收=go build ./... OK+chat 包 -count=1 ok（140s）+全量 go test ./... -count=1 96 包 0 FAIL+gofmt 五文件 CR 剥离后幂等净（本 worktree=CRLF 环境性全包列名，同④段核对法）；端测边界声明=纯文案渲染面单测域，未占真栈；停止条件核查=三处文案模板均单点用户面、无共享扩散。
