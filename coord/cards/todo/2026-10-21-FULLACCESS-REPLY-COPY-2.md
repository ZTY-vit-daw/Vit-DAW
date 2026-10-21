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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 测试名 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
