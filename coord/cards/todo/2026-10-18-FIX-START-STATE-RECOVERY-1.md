# FIX-START-STATE-RECOVERY-1：启动状态恢复语义与成本（按内核启动 uuid 恢复旧工程 42MB/≈4s）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=FE-START-EMPTY-DRAFT-1(118) [ATTRIBUTION.md](../../runs/FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md) §2.3 + 13:45 live 复现）
- 派发确认：**待用户确认**（P1 候选；启动性能 + 恢复语义面；建议下一窗口派发）
- 验收负责人：GLM 主管决策流
- 池序 138；目标仓库=D:\Vit_DAW；P1 候选 / 预估 0.5-1 天 / 无硬依赖
- 模型分级：L2 / GLM 执行会话（agent Go 启动状态面）

## 现象与机制（118 已取证）

- activation 后按**内核启动时报告的 uuid**（非"最近工程"）在 draft 根找同 uuid 最新 runtime state：`RecoverableProjectPathForUUID`（working_session.go:602-670）逐候选整读 42MB+unmarshal → 切到旧 draft（server.go:7316）→ `RecoverWorkingSession`（:7386-7389）再整读 → `ReadAgentRuntimeState`（server.go:7397 / history.go:1016）再读 → `persistActiveProjectWorkspaceLocked`（server.go:7479-7494）整写回 42MB。
- 实测目标=2026-09-05 旧 draft `draft_20260905T134132_3866b080`（41,980,404B）；13:45 live 复现窗口≈4s（13:45:28 建 A→13:45:32 42MB 被整体重写）。
- 加载结果随后被版本切换丢弃（属被丢弃的前置恢复成本）；与「最近工程恢复」语义不符。

## 待决（先上交建议，获裁定后落代码）

1. 语义：**不恢复**（内核报告 uuid 属上一内核工程，可能非用户意图）or **恢复最近工程**（显式用户入口）or 保持现状但异步/按需？
2. 成本：若恢复，能否异步/懒加载/容量上限（42MB 读≥2 次+写 1 次）；启动时间目标与测量口径。
3. 与首装空根（无历史 draft）的关系；与 118 修复的边界。

## 文件域

`agent/internal/history/{history.go,working_session.go}`；`agent/internal/chat/server.go`（恢复路径）；同包测试；`coord/runs/FIX-START-STATE-RECOVERY-1/` 工件。越域即停。

## 验收标准

1. 决策点建议先上交获裁定；实现按裁定（若"不恢复/最近工程"需定义旧状态缺省语义与兼容，AGENTS §11）。
2. 单测覆盖三语义边界 + 旧记录反序列化兼容；`go build ./... && go test ./... -count=1` 0 FAIL + gofmt 净。
3. 真栈启动测量：读/写次数、42MB 文件处理耗时、总启动时间（run ID/工件/被测二进制 SHA）；不退化 118 行为。
4. 回执含语义裁定引用、diff、证据、边界。

## 停止条件

- 语义未获裁定前不落代码；不得静默丢弃/改写用户工程状态；需内核/前端协议改动先上报。

## 关联

- 发现卡：FE-START-EMPTY-DRAFT-1（118，done）；FIX-START-DRAFT-SIDE-EFFECT-1（137）
- 证据：118 ATTRITION §2.3 + 13:45 live5 旁证（用户 92 手测现场）
