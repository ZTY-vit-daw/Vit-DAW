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
- 领取提交：`8e9c630f`（已推共享 main 并核对远端卡面 owner）
- 领取时工作树状态：worktree 全净（`git status --short` 空）；并行核对：doing 在飞=336（前端仓文件域）、337 已验收、无同文件域在飞卡；338 领取时仍在 todo、派发已确认
- 文件域补充声明（领取时如实登记，待主管复核）：卡面文件域列 launcher_flow.go+测试；实现需 ChatResponse/Server 的最小接线插点（server.go 新增 1 结构字段 +2 行管线插点，全部逻辑仍落 launcher_flow.go，edu_jam_chat 既有段与 authority 域零触碰零重写）。依据：目标 1/2 均为回执级活件，struct 字段与管线调用点在 server.go 属结构性必经（337/C4 同一接线先例）；如主管裁定不扩域，回执 commit 单提交可整体回退。

### 回执（执行侧自验 2026-11-09）

- **实现 commit**：`port/launcher-bridge-be-1` @ `81fa6f27`（已推远端，不合入 main——合入经主管验收）；server.go 实际接线=+16 行纯插点（1 结构字段 ContextFlow + writeChat 标记调用 1 行 + 桥接检查点调用块 1 处），零删改既有行；`git diff --stat`：launcher_flow.go +161 / launcher_flow_test.go +362 / server.go +16，共 3 文件 539 插入 0 删除
- **三件交付**：
  1. 回执 `context_flow` 三态（launcher=launcher 分支自标；project=工程绑定态 writeChat 默认标，`applyChatContextFlowMark` 逻辑在 launcher_flow.go；空=未绑定无标记回执，omitempty 序列化缺席零回归）+卷切换 typed_event `volume_switch`（含 project_name/path/uuid，落在桥接播报回执上——agent 观察绑定态后补发，不预写工程卷状态，设计稿 §2.2 切换时机口径）；
  2. 桥接播报两分支：无 binding=`launcherBridgeBroadcastCheckpoint` 确定性摘要（「已进入《X》。最近修改于 …——想从哪儿继续？」，工程名+os.Stat mtime，stat 失败 fail-soft 如实降级，零 LLM）；有 binding=315 `eduJamAutoIntroCheckpoint` 既有机制承接——本检查点对 binding 工程 fail-closed 让位，`edu_jam_chat.go` **零改动**（`git diff` 亲证），衔接验证测试两腿（恢复播报 stop=edu_jam_recovery_broadcast / 自动开场 stop=edu_jam_entered 在 launcher 起源会话链路下照常触发+opened_at 回写核对）在卷；触发条件=launcher 起源会话+绑定态+无 binding+（会话,工程）一次性（LoadOrStore 原子领取防并发双发）+斜杠/乐局命令词放行（315 同款守卫，不消耗一次性名额）——普通工程进入零差异；
  3. directive 自然语确认句=337 既有文案复用（「好的，打开/继续/新建…《X》。」即回执 Reply，与 directive 同响应），本卡只加回归测试（确认句在场+《X》恰一次不重复+Reason 一致），零新文案代码。
- **验收命令与退出码**（均 exit 0，worktree D:/Vit_DAW_wt_338impl/agent）：
  - `go build ./...` → 0
  - `go test ./internal/chat/ -run 'TestLauncherContextFlowThreeStates|TestLauncherBridgeBroadcastNoBindingProject|TestLauncherBridgeBindingProjectHandsOffToEduJam|TestLauncherDirectiveConfirmationSentence' -count=1` → ok（14 子测试全绿）
  - `go test ./internal/chat -run 'TestLauncher' -count=1` → ok（337 既有面不回退）
  - `go test ./internal/chat -run 'TestEduJam' -count=1` → ok（315 既有面不回退）
  - `go test ./internal/chat -run 'Test.*(Event|Message|Lifecycle)' -count=1` → ok（§6 健康检查）
  - `go test ./internal/chat/ -count=1`（全包）→ ok 124s，零回退
  - 说明：首次运行 1 例断言失败（HTTP 子测试用原始 Windows 路径做 JSON 字符串直比，反斜杠转义致误报）——属测试断言缺陷非功能缺陷，改结构化 JSON 解码断言后全绿；实现代码零改动。同一代码版本无同断点两连败。
- **红线核对**：authority/pendingmanager 零代码改动（diff 仅 3 处插点，无 authority 字样）；edu_jam_chat 既有段零触碰（不在改动文件列表）；测试运行时状态全部 t.TempDir（伪工程文件+sidecar 隔离落盘）；sealed/holdout fixture 未触碰
- **端测覆盖边界声明（AGENTS §5）**：本卡为零栈零 LLM 单测卡（卡面载明真栈腿并入 339）。单测证明 chat 回执面契约（context_flow/typed_event/桥接播报内容与幂等），**不**证明：①前端对 context_flow/typed_event 的渲染呈现（消费面=335/336 前端卡与 C6 冒测）；②真栈三件套下的进工程首条播报可见性；③webui 浏览器级 DOM。此三类留给 339/C6 真栈腿，不作为本卡完成声明。
- **§8 口径（本次运行）**：零栈确定性单测运行，无 LLM 参与轮次；成功条件=上述命令组全 exit 0（一次达成）；失败分类：编译失败/断言失败/环境中断（未发生环境中断）；止损线=同一代码版本同一确定性断点两连败即停并上交（未触发）。
- **语义边界备忘（供主管）**：337 注释曾言「有活跃工程时的跨工程意图归 C5 治理链」——本卡（C5）卡面目标不含工程流内跨工程意图路由（发卡目标三件均未列），本卡未实现该路由；若需此面应另开卡。另：typed_event 为回执契约面，前端消费在 336/C6 落地。
- **协调提交**：卡面 doing→done 回执提交仅含 coord/ 变更（经干净基线 cherry 路径推 main，port 分支保持纯实现）

### 验收

- （主管回填）

### 终验（主管，2026-11-10 凌晨）

- **pass 整卡合入 main**——主管亲测 build exit 0+定向三族绿；315 承接零重写（edu_jam_chat.go 零 diff）+volume_switch 不预写工程卷状态（B5 语义合规）+确认句复用零新文案；server.go +16 纯插点域补充**追认**（字段归属结构性必需，单提交可回退形态合规）。断言缺陷自纠（测试面）如实。
- **裁定**：①工程流内跨工程意图路由（337 注释归 C5 面）确不在本卡三目标——登记**候选小卡**（demo 主路径=launcher→open 已覆盖；态 C 内切换为展演加分项，候 freeze 余量定夺）；②flash-worker 首单合格记一笔。
