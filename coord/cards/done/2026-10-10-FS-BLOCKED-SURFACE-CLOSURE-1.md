# FS-BLOCKED-SURFACE-CLOSURE-1：capability_blocked 边界面闭包分支扩面（LEDGER 顺带核对发现）

- 发卡：GLM 主管决策侧 / 2026-10-10 夜（依据=[NIGHT-BATCH rulings §5-2](../../rulings/2026-10-10-NIGHT-BATCH-rulings.md)+LEDGER 回执顺带核对：机制已精确定位）
- 派发确认：已确认（主管裁定）
- 验收负责人：GLM 主管决策流
- 池序 54；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无（可与 HARNESS-STRUCT-NORMALIZE-1 并行，文件域不相交）
- 模型分级：L1 / flash 可接

## 缺口（机制已定位，勿重复取证）

goalrunner_chat.go 的 `capabilityBlockedBoundaryResponse`（FS-CAPABILITY-BLOCKED-SURFACE-1 产物）检查位于 `!audioClosureActive` 分支内；**闭包激活（自由态标准配置）时 audioClosureResponse 先返回，blocked 边界面永不触发**——run 203348 期间实证 stop=done+goal_status=completed 与 fs_loop capability_blocked 并存形态（修复后机会面已改变，需重新观察）。既有测试只钉非闭包路径（free_state_capability_blocked_surface_test.go）。

## 目标

闭包激活路径补 blocked 边界检查（对齐 FS-CAPABILITY-BLOCKED-SURFACE-1 的边界面语义：blocked 收据在场时终局呈现不冒充完成）；补闭包路径测试（blocked 收据+closure 激活→边界面触发断言）。

## 文件域

`agent/internal/chat/goalrunner_chat.go` + 测试；越域即停。

## 验收标准

`go build ./...`+chat 包+全量 0 FAIL+gofmt 净；新测试绿+既有非闭包路径测试零改动全绿。

## 停止条件

边界面语义与 FS-CAPABILITY-BLOCKED-SURFACE-1 契约冲突（如闭包态本就另有呈现约定）→ 契约引用上交裁定。

- 领取：（2026-10-19 11:00 +0800 / origin/main=5b4e9e84 / GLM-5.3 flash PC 执行会话（用户口令「开工 54」指定领取）/ 分支=port/fs-blocked-surface-closure-1 / worktree=D:/Vit_DAW_wt_fsclosure1 / 领取提交=本提交；领取时核对：卡在 todo 派发已确认（主管裁定）、依赖无、doing 两卡（FE-SENDROUTE-MACRO-GUI-1=前端仓 GD 文件、GEN-SUNO-WEB-DESIGN-1=coord/runs 设计件零生产码）文件域零冲突、同文件卡 CAP-BOUNDARY-1（136）仍在 todo 未领（卡面约定两卡串行本卡先）、纯单测卡不占真栈无资源记录需求）
- 回执：（**done 2026-10-19 11:40 +0800，执行侧自验完成待主管验收**）实现 commit=6a5c8c7b @ port/fs-blocked-surface-closure-1（基线 5b4e9e84，已推 origin）。**实现面**：goalrunner_chat.go 闭包激活分支的 `audioClosureResponse` 提前返回拆除——两分支（plain/闭包激活）统一走新 helper `settledTurnFaceResponse`：capability_blocked 边界检查先于闭包结算面；触发时闭包状态仍经 `bindAudioClosureToResponse` 绑定（前端闭包视图不丢）；非 blocked 回合两分支呈现面与改前完全同序。终态闭包并存语义已核：`settleFreeStateLoopFromClosure` 对 blocked 终态 loop 空操作（freeStateLoopActive=false）、`syncFreeStateSpine` 仅镜像审计投影字段——边界优先无持久化副作用差异。**测试**：新增 `free_state_capability_blocked_closure_surface_test.go`（190 行，3 测试全绿）：TestCapabilityBlockedBoundaryOwnsClosureActiveSettledFace（pin：闭包激活+blocked 收据→边界面触发：StopReason=capability_blocked、收据含 G6_target_evidence+G8_target_consistency、回复为边界话术不冒充完成、闭包视图在场、Workflow≠minimal_audio_closure、GoalStatus=completed 契约不变）；TestClosureActiveSettledFaceWithoutBlockedReceiptKeepsClosureFace（非 blocked 反证控制，闭包面保留）；TestBlockedBoundaryWinsOverTerminalClosureSettlementFace（终态闭包并存，结算面仍不占有）。**既有测试零改动**：free_state_capability_blocked_surface_test.go 零 diff，4 测试全绿（GateRefusalSettlesCapabilityBlockedLoopShape / CapabilityBlockedBoundaryResponseSurface / CapabilityBlockedBoundaryDetectionKeepsExistingPaths / CapabilityBlockedBoundaryDetectsModelTerminalConcession）。**验收命令**：cd agent 后 `go build ./...` OK；`go test ./internal/chat -count=1` ok（118s）；`go test ./... -count=1` 退出码 0、95 包 ok、0 FAIL；gofmt——本卡 delta 净（新测试文件全净；goalrunner_chat.go :181 存在**基线既存**偏差 `}	// AGENT-SENDROUTE-SEMANTICS-1`（130 卡 send 路线遗留，非本卡引入，未顺手修以免 136 行号漂移），上交决策侧知悉）。**文件域最终改动行号清单（供 136 CAP-BOUNDARY-1 领取前核对）**：goalrunner_chat.go=路由分支 :505-514（原 :505-524 区域收拢为统一出口，新调用在 :513）+新增 helper :2105-2132（doc :2105-2120、func :2121-2132），capabilityBlockedBoundaryResponse 及其后内容整体下移约 14 行（原 :2115 起→现 :2134 起）；free_state_capability_blocked_closure_surface_test.go=新文件 :1-190；free_state_capability_blocked_surface_test.go=零改动。**端测边界声明**：纯单测卡（卡面明示不占真栈），零渲染面/用户旅程改动，无端侧烟测义务。**停止条件未触发**：闭包态无另一套呈现约定冲突，FS-CAPABILITY-BLOCKED-SURFACE-1 契约（blocked 收据在场时终局呈现不冒充完成）在闭包路径同义延续。
- 验收：（裁定文件 / 验收 commit）
