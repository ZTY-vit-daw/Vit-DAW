# FULLACCESS-REPLY-COPY-2：准入边界句移出聊天回复面+typed 完成面中文化（148 修订）

- 发卡：GLM 主管决策流 / 2026-10-21 晚（依据=用户手测裁定：准入边界计数「我只需要我知道它可以，不是要输出在回复内容中」；148 合同修订）
- 派发确认：已确认（用户手测现场原话，2026-10-21 M2 场次②）
- 验收负责人：GLM 主管决策流
- 池序 160；目标仓库=D:\Vit_DAW；P1 / 0.25 天 / 依赖=无；模型分级：L1 / flash 可接

## 背景（M2 手测场次②实录）

轮 2 full-access 自主装载的回复里出现「准入边界：本机插件库检索到 63 款混响，其中 30 款已通过 语义控制准入；其余 33 款可装载，但未接入本类语义控制，不在本次推荐面。」——148 的「full-access 自主选择同缀」设计把选择卡诚实计数句前置到了聊天回复。用户裁定：该信息保留在结构化面（用户可查），不进聊天回复正文。另：typed 控制器**完成面**回复 `semantic_dynamic_workflow.go:937` 为英文硬编码（148③ 只覆盖确认卡）。

## 范围

①`agent/internal/chat/full_access_semantic_chain.go:148` 邻域：移除回复正文中的准入边界前置句（`pluginRecommendationAdmissionBoundaryReplySuffix` 不再拼入 Reply）；结构化披露 `plugin_recommendation_admission_boundary`（workflow_data）**保持不变**——审计面零丢失。②`agent/internal/chat/semantic_dynamic_workflow.go:937`：完成面回复中文化（语义=「类型化语义处理器调整已执行完成，带回读与事务证据。原目标标记完成前需要一次新的动作后观察。」；workflow_data 字段名与 schema 不动）。③**选择卡面（manual/confirmation 路径）的准入边界句维持现状不动**——148① 卡面合同不变，本卡只动聊天回复正文与完成面文案。

## 文件域

`agent/internal/chat/full_access_semantic_chain.go` + `agent/internal/chat/semantic_dynamic_workflow.go` + 既有语义卡文案测试（semantic_card_copy_test.go 邻域）+新测。越域即停。

## 验收标准

单测：full-access 自主路径 Reply 不含「准入边界」串+workflow_data 仍含 admission_boundary 结构；选择卡面路径准入边界句原样保留；完成面回复为中文化文案（workflow_data 零变化）；148 既有测试相应断言更新+全量 0 FAIL+gofmt 净；端测边界声明（纯文案域，真栈腿可选）。

## 停止条件

发现准入边界句还有第三处用户面消费点 → 锚点上交。

- 领取：2026-10-21 晚窗 / origin/main=612beb18 / owner=GLM-5.3-flash（ZCode PC 执行会话，160 卡会话）/ 分支 port/fullaccess-reply-copy-2 / worktree D:/Vit_DAW_wt_farc2 / 领取提交=本条随协调分支 coord/farc2-claim 推 main（hash 见 git log 卡面路径）
- 回执：**执行侧自验完成，待主管验收**。commit=port/fullaccess-reply-copy-2@389e7c70（实现+测试同 commit，3 文件 +70/−6；已推 origin）。单测（agent/internal/chat，5 例点名全 PASS）：**新** TestFullAccessSelectionReplyOmitsAdmissionBoundaryKeepsStructuredDisclosure——full-access 自主 Reply 不含「准入边界」串（148 既有断言 TestFullAccessSelectionReplyCarriesAdmissionBoundary 反转改名，Reply 仍含主推荐自报）+WorkflowData.semantic_post_load_request_context 内 plugin_recommendation_admission_boundary 结构化披露 40/1 逐字段保留（锚点由生产 eq post-load 链生成，非测试 fixture）；**新** TestSemanticDynamicExecutedCompletionReplyIsChinese——完成面逐字「类型化语义处理器调整已执行完成，带回读与事务证据。原目标标记完成前需要一次新的动作后观察。」+无英文回退+WorkflowData 7 字段集与取值零变化+execution_receipt/post_action_verification 合同字段保留；选择卡面原样三测 PASS（TestPluginRecommendationSelectionCardDisclosesAdmissionBoundary[回复句+卡体句+workflow_data admission_boundary]/TestPluginRecommendationSelectionCardWithoutDisclosureStaysBackCompatible/TestPluginRecommendationAdmissionBoundaryLineCoversAllCountShapes）。go build ./... OK；全量 go test ./... -count=1 真实退出码 **exit 0（96 包 0 FAIL）**；触碰三文件 gofmt 净（剥 CR 比对口径，工作树 CRLF 为本机检出既有态）。
- 实现形态说明：②完成面在 semantic_dynamic_workflow.go:937 提取为 semanticDynamicExecutedResponse(interaction, receipt) helper+semanticDynamicExecutedReply 常量——WorkflowData 构造自原行逐字搬移，schema 零变化；完成 face 依赖 s.harness+PCA 准入活链路，nil-harness 单测不可达，故 helper 直测即合同测试，:937 调用点唯一（grep 可核）。诚实上交（越文件域未动）：①移除调用后 plugin_recommendation.go 的 pluginRecommendationAdmissionBoundaryReplySuffix 成为无调用函数（编译/vet 不受影响），删除或保留待主管裁定。
- 端测边界声明：纯文案域卡（卡面标注真栈腿可选），本轮未起真栈。覆盖=full-access 自主回复面文案+结构化披露保留+选择卡面不变+完成面中文化+workflow_data 零变化（全部单测钉住）；未覆盖=真栈 chat 端到端往返、webui 渲染面与 Godot 旅程（本卡零 webui/前端改动）。
- 验收：2026-10-21 深夜 / GLM 主管决策流 / **整卡 pass**：实现 389e7c70 cherry-pick 入 main（2a79d9e6）；亲读 diff——①准入边界句不再拼入 Reply 正文、workflow_data 结构化披露逐字段保留（40/1 锚点经生产 eq 链）②完成面中文化逐字合同+WorkflowData 7 字段零变化+helper 直测=合同测试（:937 调用点唯一）③选择卡面三测原样零回退；用户裁定（回复正文不出现准入边界）落实到位；真栈腿卡面可选未行使裁定单测足以交付（纯文案域）；**无调用函数 pluginRecommendationAdmissionBoundaryReplySuffix 裁定=保留待下次触碰 plugin_recommendation.go 时删除**（零功能影响，不另开卡）；集成态全仓 0 FAIL+二新测点名 PASS+gofmt 净；验收 commit=本提交
