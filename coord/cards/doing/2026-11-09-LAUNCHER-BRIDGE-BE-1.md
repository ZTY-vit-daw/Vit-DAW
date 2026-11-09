# LAUNCHER-BRIDGE-BE-1：流标记+桥接播报（332 拆卡 C5，agent 域·案 B 承接核心）

- 发卡：GLM 主管决策流 / 2026-11-09；派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 338；P1 / 1 天 / 依赖=337（C4）；L2 / PC；零栈单测+真栈腿并入 339

## 目标

1. 回执 `context_flow`（launcher/project）+卷切换 typed_event（前端据此呈现卷分隔语义）；
2. **桥接播报（案 B 承接核心——工程侧首条=现有浮动窗里的一句话）**：有乐局 binding 工程=315 自动开场/恢复播报既有机制承接（验证衔接零重写）；无 binding 工程=确定性摘要模板（工程名+最近修改时间，零 LLM）；
3. launcher 终答→directive 的自然语确认文案（「好的，打开《X》」一句，与回执同发）。

## 文件域

`agent/internal/chat/launcher_flow.go`（扩展 ~120 行）+测试；不触 edu_jam_chat 既有段（只验证衔接）。

## 验收标准

流标记三态+衔接播报两分支单测全绿；真栈腿并入 339（进工程首条=桥接播报在现有浮动窗出现）。

## 领取/回执/验收

### 领取（执行侧回填 2026-11-09）

- 领取时间：2026-11-09（晚窗）
- 领取基线：origin/main `cec2acb1`（含 3a19ac72 卡面要求基线，亲证 `git rev-parse`）；基线 `go build ./...` exit 0
- owner：GLM-5.3 Flash 量产执行子代理（卡 338bridge）/ PC / ZCode 执行会话
- 分支：`port/launcher-bridge-be-1`；worktree：`D:/Vit_DAW_wt_338impl`（独立 worktree，主工作树未触碰）
- 并行核对：doing 在飞=336（前端仓文件域）、337 已验收、无同文件域在飞卡；338 领取时仍在 todo、派发已确认
- 文件域补充声明（领取时如实登记，待主管复核）：卡面文件域列 launcher_flow.go+测试；实现需 ChatResponse/Server 的最小接线插点（server.go 新增 1 结构字段 +2 行管线插点，全部逻辑仍落 launcher_flow.go，edu_jam_chat 既有段与 authority 域零触碰零重写）。依据：目标 1/2 均为回执级活件，struct 字段与管线调用点在 server.go 属结构性必经（337/C4 同一接线先例）；如主管裁定不扩域，回执 commit 单提交可整体回退。

### 回执

- （执行侧回填）

### 验收

- （主管回填）
