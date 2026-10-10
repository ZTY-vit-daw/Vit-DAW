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

- 领取：（时间 / origin/main hash / owner / 分支 / 领取提交）
- 回执：（commit / 测试名 / 端测边界声明=纯单测域）
- 验收：（裁定文件 / 验收 commit）
