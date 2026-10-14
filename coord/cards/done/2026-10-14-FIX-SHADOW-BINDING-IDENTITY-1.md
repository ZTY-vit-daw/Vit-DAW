# FIX-SHADOW-BINDING-IDENTITY-1：VSP 快照缺身份时 shadow 保留最后已知工程身份（109 线硬阻断修复）

- 发卡：GLM 主管决策侧 / 2026-10-14 晚窗（依据=[FORENSICS-LEG-E.md](../../runs/FIX-FS-SETTLE-DEADCHAIN-1/FORENSICS-LEG-E.md) §二 run9 子形态实锤+腿 E 上交裁定；109 收口腿 exit 0 前置）
- 派发确认：已确认（主管依已核实取证证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 112；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：**P1（109 硬阻断）** / 0.5 天 / 无（与 113 文件域不相交可并行）
- 模型分级：L1 / GLM 或 flash 执行会话（agent 面）

## 缺陷机制（已核实，FORENSICS-LEG-E §二.2 锚点链）

mutation 后某次 VSP state.snapshot 回复未带身份三元组（revision/epoch/snapshot_hash 全空，graph_revision 计数器替代形态，run9 mix session latest_change before=11→缺失实录）→ `harness.go:1844 vspLegacyStateWithBinding` 仅在 reply 带身份时写入，refresh 走 `shadow.Initialize` **空身份整体覆盖** → 组装链 mixboard/project_package.go:10→catalog.go:1017→harness/ccb_observation.go:134-157 产出身份空包 → binding unbound（capabilitycontext/free_state_observation.go:522）→ `freeStatePostActionObservationEligible`（chat/free_state_reasoning_loop.go:1275）要求 revision 非空 → 观察永不记 post_action → D1 断言结构性不可满足。对照组 run6 同路径 revision='7' 正常=时序相关间歇缺陷。

## 任务

1. 修复（agent 侧优先）：VSP 快照 reply 缺身份三元组（全空）且 shadow 已有非空身份时**保留最后已知身份**（不空覆盖），并记 telemetry/日志标记（如 identity_preserved_on_incomplete_snapshot）供复跑核验；shadow 本就无身份（从未绑定）→ 维持 unbound 现状（fail-closed 不变）。
2. 单测钉三形态：①reply 带新身份→更新（现语义）；②reply 缺身份+shadow 有→保留旧值+标记；③reply 缺身份+shadow 无→unbound 不变。
3. `go test ./... -count=1` 0 FAIL+gofmt 净（LF blob）。

## 文件域

agent/internal/harness.go（vspLegacyStateWithBinding/shadow 刷新一带）+同包或就近测试；**不得动** mixboard/capabilitycontext/chat 组装链（那是后果不是病因）；kernel/VSP 侧恒带身份为后备方案不在本卡（agent 侧修复后若真栈仍复现身份丢失再议 C++ 面）。

## 验收标准

diff 亲读+三形态单测亲跑+95 包 0 FAIL；修复后由决策侧安排 109 rerun5（真栈复跑归 109 线，本卡不占栈）。

## 停止条件

保留语义与 shadow 既有 checkpoint/回滚语义冲突（保留旧身份会污染别的读方）→ 带冲突读方清单上交。

- 领取：2026-10-14 19:05 +0800 / origin/main=8291d42a（todo 在位、派发已确认；doing 三卡域=experiment_runtime+goalrunner / 前端仓焦点日志 / agentloop free_state_reasoning+free_state_gate+chat prompt，均与本卡 harness 域不相交） / GLM-5.3-Flash L1 执行会话（PC Windows，ZCode） / port/fix-shadow-binding-identity-1 / D:/Vit_DAW_wt_shadowbind1 / 领取提交=本提交
- 回执：commit 6ae5525a（port/fix-shadow-binding-identity-1，已推 origin；领取基线 8291d42a，工作树领取时净）——preserveVSPIdentityFromShadow 同 uuid 回填缺失三元组字段（不空覆盖：reply 带值字段优先），跨工程/未绑定/无可解析值 fail-closed 不回填；接线四处 VSP 快照 Initialize（executeKernelCommandVSP before 快照 harness.go:1605、afterKernelReplyVSP observed:1683 与 get_project_state 回退:1689、refreshShadowWithStatus:12855=run9 实锤路径）；telemetry 标记=`identity_preserved_on_incomplete_snapshot`（Warn 日志，restored 字段清单+uuid+三元组值）+refresh status 新增 `identity_preserved` 键；单测 5 钉=三形态（TestVSPShadowIdentityUpdatesOnCompleteSnapshot/…PreservedOnIncompleteSnapshot 含部分缺失子形态/…StaysUnboundWithoutLastKnown）+跨工程护栏（…NotPreservedAcrossProjects）+日志标记落盘断言。自验：`go test ./... -count=1` 95 包 0 FAIL；gofmt 净（LF blob 口径：工作树 CRLF 为 autocrlf 检出现象，两改动文件 LF 归一后 `gofmt -l` 零输出；index i/lf）。**端测边界声明：本卡按卡面为纯 agent 侧单测收口，未跑端侧烟测/真栈**；对 run9 形态的端到端效果由决策侧 109 rerun5 真栈复跑核验（日志标记与 identity_preserved 键为复跑核验锚点）。覆盖边界：修复仅覆盖 VSP 快照通道；非 VSP legacy 通道（bridge.go:486/499、mix_tick.go:562、afterKernelReply get_project_state）未改——不同数据通道、无 run9 形态实证，按不扩域纪律保持现状。停止条件核查：保留语义无读方冲突——mixboard 组装链/capabilitycontext 绑定/chat post-action 门=修复目标读方；planVitConversationCheckpoint（harness.go:663）缺陷窗口内由「revision 空→无门基同步 checkpoint」变为「陈旧 revision→skip、恢复后迟到 checkpoint」，符合其文档不变式（may arrive late, never missing）；shadow change receipts=缺陷伪变更（identity→空 project diff）消失，uuid 基线重置不受影响（comparator 仅比 uuid，change.go:56-60）；confirmPendingChangeLocked epoch/uuid 校验不受影响；materialize receipts 消费方=伪通知减少。
- 验收：**pass** / 2026-10-14 批量三（[rulings/2026-10-14-BATCH3-rulings.md](../../rulings/2026-10-14-BATCH3-rulings.md)①；四处接线逐一亲核+95 包独立亲跑 exit 0+gofmt blob 净；读方冲突核查与覆盖边界采信；合入 26757b34；端到端效果归 109 rerun5，核验锚点=identity_preserved 标记）
