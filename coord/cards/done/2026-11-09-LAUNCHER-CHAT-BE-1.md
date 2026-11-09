# LAUNCHER-CHAT-BE-1：launcher 流意图路由+directive 回执+开场白（332 拆卡 C4，agent 域）

- 发卡：GLM 主管决策流 / 2026-11-09；派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 337；P1 / 1.5 天 / 依赖=零（与 335 并行，契约 JSON 冻结）；L2 / PC；零栈单测+真栈腿并入 339

## 目标

1. chat 请求 `context.launcher=true` 分支（`agent/internal/chat/launcher_flow.go` 新 ~200 行）：工程意图识别→`project.recent` 匹配→唯一命中出 `launcher_directive` 回执/多命中出结果卡（ProjectResultCards 先例）/无命中说明+追问——三分支；
2. 门厅简版开场白（在场问候+最近工程一条+「想从哪开始？」——确定性模板优先，LLM 渲染可选）；
3. `/agent/state` 增 `current_project{path,uuid}` 两字段（窗徽章对账+降级自愈，~10 行）；
4. 无工程态 chat 冒测加固（F11 语义回归）+`launcher:true` 不改 authority_mode 断言（零 authority 代码改动红线）。

## 文件域

`agent/internal/chat/launcher_flow.go`（新）+`server.go`（回执字段+state 披露 ~30 行插点）+测试；**不触** authority/pendingmanager 域。

## 验收标准

三分支表驱动单测+无工程态回归+authority 不变断言全绿；§8 口径成文（真栈腿含 LLM 轮，确定性退路不依赖 LLM）。

## 领取/回执/验收

- 领取：2026-11-09 / origin/main=feb270cbdc0f76efde0ad0e4fe90bd22a67f77a6 / owner=GLM-5.3 L2 执行子代理（会话 337impl）/ PC / 分支 port/launcher-chat-be-1 / worktree D:/Vit_DAW_wt_337impl / 领取提交见回执
  - 基线亲证：HEAD=feb270cb（与 origin/main 一致）；worktree status 干净（仅本卡 mv）；diff --stat=空（领取前无实现 diff）
  - 前置核对：卡在 todo ✓ 派发已确认 ✓ 依赖=零 ✓；doing 三卡（ARRLOOP-AUTHBRIDGE-1=harness 装载确认面、CLOSURE-ROUTE-ADAPT-1=chat closure 预算面、WAVESHELL-LOADING-FORENSICS-1=只读取证）与本卡文件域（launcher_flow.go 新+server.go 两处插点+新测试）不相交 ✓；335（FE 域=前端仓）并行无冲突 ✓
- 回执（2026-11-09 自验完成，done 候主管验收）：
  - 实现分支 `port/launcher-chat-be-1` @85d4b59d（已推远端，不合入 main）；worktree D:/Vit_DAW_wt_337impl；基线=领取提交 a6fb3e63（origin/mainfeb270cb）
  - diff 清单：`agent/internal/chat/launcher_flow.go`（新 542 行）+`agent/internal/chat/launcher_flow_test.go`（新 456 行）+`agent/internal/chat/server.go`（+25/-3：ChatResponse.LauncherDirective 字段、handleChat 分支接入〔handleDevChatCommand 之后/语义入口之前〕、/agent/launcher/greeting 路由、handleState 与 handleRuntimeStatus 两面 current_project 披露）
  - 测试命令与退出码（agent/ 下）：`go build ./...`→0；`go test ./internal/chat/ -count=1`→ok（126.7s 全包绿）；`go test ./internal/chat/ -run 'TestLauncher|TestAgentStateDiscloses' -count=1`→ok；§6 健康检查 history/harness/capabilitycontext/mom 四包指定用例→ok
  - authority 零改动亲证：`git diff a6fb3e63..85d4b59d --name-only` 仅上述三文件，authority/pendingmanager/turn_control 零命中；测试 TestLauncherFlowDoesNotChangeAuthorityMode 断言 launcher 轮前后快照不变+launcher 上下文里 full_project_access 声明照旧被拒（请求体永远不能授予）
  - 契约补充（供 335/C2 消费；§4.1 冻结面之外的小增量，超出部分请主管过目）：①回执新字段 `launcher_directive`（§4.1 原样）；②ProjectResultCards 卡形 `{kind:"launcher_project", type:"project_open_candidate", id, name, file_path, launcher_directive}`（点卡=执行内嵌 directive）；③新端点 `GET /agent/launcher/greeting`→`{status, launcher, reply, recent_project{name,file_path}, recent_count}`（确定性零 LLM，降级模板同确定）；④`/agent/state` 与 `/agent/runtime/status` 增 `current_project{path,uuid}`（无工程=空字符串字段在场）
  - 端测覆盖边界声明（AGENTS §5）：本卡验证面=零栈单测+编译+HTTP handler 级端到端（fake kernel）；真实栈端侧（三件套+渲染面+真内核 get_recent_projects）未测——真栈腿候 339 LAUNCHER-SMOKE-1；行为改动均为增量字段/新端点，旧客户端 omitempty 不受影响
- §8 口径（跑前成文）：
  - 本卡交付腿：零栈零 LLM——全部断言确定性（表驱动+fake kernel），无 LLM 轮，单 run 无重跑
  - 真栈腿（并入 339）：launcher 意图路由路径不调 LLM——指令直达/结果卡/说明三分支每态一次断言即可；若 339 叠加 LLM 轮（一般对话/开场渲染），预算 ≤2 轮/分支；失败分类=intent_unresolved / mis_route / UI 断链 / 环境中断分开记录；同一确定性断点两连败禁原样重跑（先补锚点或拆取证卡）；一次 exit 0=存在成功路径，稳定性结论需另定样本数
- 验收：（主管回填）

### 终验（主管，2026-11-09 深夜）

- **pass 整卡合入 main**——主管亲测：build exit 0+TestLauncher 族定向绿；authority 零改动（diff 三文件亲证+断言测试在卷）+写者不变式（directive 仅回执字段）+F11 回归三红线达标。软动词回落 LLM 的分级设计（防「继续聊聊」误拦截）与契约增量声明（launcher_project 卡形/greeting 端点/state 双面）采纳——对齐动作进 336/339。
