# FIX-START-STATE-RECOVERY-1：启动状态恢复语义与成本（按内核启动 uuid 恢复旧工程 42MB/≈4s）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=FE-START-EMPTY-DRAFT-1(118) [ATTRIBUTION.md](../../runs/FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md) §2.3 + 13:45 live 复现）
- 派发确认：**已确认（用户 2026-10-18 晚窗）+ 语义裁定=方案一「不恢复」**（内核报告 uuid 不作为恢复依据，启动即新会话；「最近工程」显式入口留后续增强；旧状态缺省语义与兼容按 AGENTS §11 在实现中定义）；验收负责人：GLM 主管决策流
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
- 领取：（2026-10-18 21:05 +0800 / origin/main=445c49ce7312bd8127b016faad07a485038639e1a / GLM-5.3 L2 PC 执行会话 ZCode（本卡会话，用户任务领取指令指定）/ 分支=port/fix-start-state-recovery-1（worktree D:/Vit_DAW_wt_fxstartrec1）/ 领取提交=本提交；领取时核对：卡在 todo 且派发已确认（用户 2026-10-18 晚窗裁定=方案一「不恢复」，语义面已锁定）、无硬依赖、doing 仅 GEN-SUNO-WEB-DESIGN-1（Mac 勘察卡零生产代码，文件域无冲突）、真栈现空闲（PC-RUNTIME-STACK 当前占用=无 + 六端口/四类进程实测=0；测量腿启动前按 §2.2 另行登记占用））
- 回执：（**执行侧自验完成 2026-10-18 21:45 +0800**；实现 commit=70f4b86a@port/fix-start-state-recovery-1（基线 origin/main=45dcef05 代码面）；语义裁定引用=用户 2026-10-18 晚窗「方案一不恢复：内核报告 uuid 不作为恢复依据，启动即新会话；最近工程显式入口留后续增强」。**实现**：working_session.go 新增 `StartupWorkspaceRecovery` 单一决策点（禁用态+零 IO+显式 reason=startup_recovery_disabled_new_session；不恢复=不读不写不删除；未来显式入口的唯一挂点，扫描原语保留并继续被测）+ server.go activateCurrentProjectWorkspace 移除 uuid 扫描/切旧 draft 链（RecoverableProjectPathForUUID[InRoot] 生产调用点归零）保留 137 的 CurrentDraftProjectPath 进程内兜底。**验收①**：chat 三语义边界（不恢复+种子全树零触碰③翻转 / 兼容位=钩子+原语测试 / 异常态 fail-closed④）+ history 钩子单测 + AGENTS §11 旧记录兼容（v1 reopen_reset / v2 未知字段 ReadAgentRuntimeState 原样读回）。**验收②**：go build ./... 0 错 + go test ./... -count=1 **95 包 0 FAIL** + gofmt 触碰文件净（仓库其余 305 文件 CRLF 为基线现象未动）。**验收③**：真栈 4 腿（占用 85ba26ff/释放 70df1976，run=baseline_clean_v2/fixed_clean/fixed_sentinel/baseline_sentinel，工件 coord/runs/FIX-START-STATE-RECOVERY-1/ + RESULTS.md）——①基线现象复现（uuid 认领旧 draft recovered=true conversations=158 + **42MB 启动期重写**）②修复腿 activation pending 新 reason + **种子全腿零触碰**（sha+mtime+全树 identical）+ project_new 恰 1 draft + reader 只读 ③独占哨兵全程持有=行为零差异+零 sharing violation（**读=0 反证**）④哨兵对基线=恢复被阻断（阳性对照）；被测 SHA：修复 99123003…/基线 7537AB63…/kernel stock B6565DCF…；计时：激活判定基线 1566ms vs 修复 535ms（隔离根 1 draft=下界差分，真实根 10185 draft 只会更差）；118/137 零退化（A2/B/D 断言全过）。**上报不修（主管裁定）**：C2 切换窗口旧 uuid 残留——project_new 后影子滞后 tick 以（缓存新 draft 路径+滞后旧 uuid）激活，在新 draft 物化惰性旧 uuid workspace 行后自愈；基线二进制在哨兵阻断恢复时同款出现（非本修复引入，118 §4 同族签名）；锚点 server.go CurrentDraftProjectPath 兜底分支+history.go BindProjectIdentity，候选修法=同路径异 uuid 且已有活动工作区时保持 pending。**端测边界声明**：单测+真栈 kernel/agent 面覆盖，未启 Godot UI（UI 手测不在本卡义务）；42MB 种子为真实状态副本（源文件零触碰），副本已从工件裁剪不入库；测量在隔离根，真实根扫描行走成本未计入基线数字。）
- 验收：（裁定文件 / 验收 commit）
- 验收（2026-10-18 深夜 GLM 主管决策流）：**pass**——语义按用户裁定落实（不恢复=不读不写不删+pending reason=startup_recovery_disabled_new_session 显式+StartupWorkspaceRecovery 单一决策点留未来显式入口挂点）；独立复验 worktree fxstartrec1 `go test ./... -count=1` **0 FAIL**；四腿真栈工件亲核（①基线现象复现 42MB 启动期重写②修复零物化+种子全树零触碰③独占哨兵=零 open 反证④哨兵对基线=阳性对照——反证设计严谨；激活判定 1566→535ms）；118/137 零退化断言过。**C2 上报裁定**：非本修复引入（基线同款+118 §4 同族+惰性物化后自愈）→登记为本卡已知行为+候选修法（同路径异 uuid 且已有活动工作区时保持 pending）归档卡面，不另立卡（低频自愈型；后续用户路径撞见再升级）。impl 70f4b86a 已 cherry-pick 入 main=317d5eab。卡终态=done。
