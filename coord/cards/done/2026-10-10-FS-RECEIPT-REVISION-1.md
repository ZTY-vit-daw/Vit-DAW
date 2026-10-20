# FS-RECEIPT-REVISION-1：receipt project_revision 宽回退链接入（:738 窄回退 hygiene）

- 发卡：GLM 主管决策侧 / 2026-10-10 晚窗（依据=[EVENING-BATCH rulings §5](../../rulings/2026-10-10-EVENING-BATCH-rulings.md) 次级发现+[REVIEW-independent.md §F-3](../../runs/FS-LARGEPROJECT-SMOKE-1/REVIEW-independent.md)）
- 派发确认：已确认（主管裁定）
- 验收负责人：GLM 主管决策流
- 池序 51；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.25 天 / **FS-LEDGER-PERSIST-1 合入后领取**（free_state_reasoning_loop.go 文件域相交，串行）
- 模型分级：L0 / flash 可接

## 缺口（SMOKE 实证）

`agent/internal/chat/free_state_reasoning_loop.go:738` 构造 admission receipt 时 project_revision 只读 LatestProjectChange 与 observation_binding.project_binding——SMOKE run 中该 CCB bundle 的 revision "4" 落在 freshness.project_revision，回执 project_revision="" （窄回退）。**宽回退链 :1865-1871（freeStateObservationProjectRevision）已在库未在此处使用**。不改门结果（复核已证），但收据字段空洞影响新鲜度评估面。

## 目标

:738 构造点改用既有宽回退链（对齐 :1865-1871 先例，不新造逻辑）；补单测：revision 落在 freshness 键时回执携带正确值。

## 文件域

`agent/internal/chat/free_state_reasoning_loop.go` + 测试；越域即停。

## 验收标准

`go build ./...`+chat 包+全量 0 FAIL+gofmt 净；新单测绿。

## 停止条件

宽回退链语义与 :738 场景不符（如该函数有副作用）→ 形态上交。

- 领取：2026-10-20 18:45 / origin/main 03fea178 / owner=GLM-5.3-Flash ZCode PC 执行会话 / 分支=port/fs-receipt-revision-1 / worktree=D:/Vit_DAW_wt_fsrr1 / 领取提交=本提交（coord/fsrr1-claim 仅含本卡面变更 fast-forward 推 main）。领取时核对：doing 池仅 GEN-SUNO-WEB-DESIGN-1（webui 域），文件域零冲突；宽回退链 freeStateObservationProjectRevision 实测为纯提取函数无副作用，停止条件不触发
- 回执：实现 commit=92f51e5e（origin/port/fs-receipt-revision-1，基于 a2a04355；2 文件 70+/3-）——recordFreeStateAdmissionReceipt 的 observation 侧窄读 project_binding 换为既有宽回退链 freeStateObservationProjectRevision（顶层/project_binding/freshness/audit_receipt.freshness 四级；LatestProjectChange 主读位次保持不变；链函数纯提取无副作用，停止条件不触发）。注意行号漂移：卡面 :738/:1865-1871 系 2026-10-10 基线，a2a04355 实际为 :760 构造行/:1909-1916 链函数。测试=TestFreeStateAdmissionReceiptProjectRevisionFallbacks 4 子测试全绿（freshness 键=SMOKE 复现/project_binding 回归/LatestProjectChange 位次回归/audit_receipt 嵌套）。门：go build ./... OK+chat 包 -count=1 ok(113.8s)+全量 go test ./... -count=1 exit0=96 包 0 FAIL+gofmt 净（worktree CRLF 检出为既有状态致 -l 全列噪声；剥 EOL 与入库 LF blob 双验均净）。端测边界声明=纯单测域（L0 卡未占真栈、无真栈 run；构造点改动不改门结果）
- 验收：**pass / 2026-10-20 晚窗（GLM 主管决策流）**——diff 亲读（observation 侧窄读换既有宽回退链 freeStateObservationProjectRevision，LatestProjectChange 主读位次不变；链函数纯提取，停止条件不触发）；实现 92f51e5e cherry-pick 入 main=6da04303；独立复跑=主管集成态全量 go test（51/53/147/148 同批，见当日验收 commit）；回执行号漂移说明（:738→:760）核对属实。
