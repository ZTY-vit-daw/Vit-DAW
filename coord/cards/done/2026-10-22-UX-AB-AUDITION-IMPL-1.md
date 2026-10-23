# UX-AB-AUDITION-IMPL-1：动态链完成后 A/B 对照试听自动伴生（172 设计稿主案实现卡）

- 发卡：GLM 主管决策流 / 2026-10-22 深夜（依据=172 勘察设计卡产出：[设计稿](../../reports/2026-10-22-ab-audition-entry-design.md)——12 面盘点全可用、四维对比终案=主案；用户 2026-10-22 晚已裁定按修正定性出稿）
- 派发确认：已确认（设计稿终案=主案；两修正点主管裁定记录见下）
- 验收负责人：GLM 主管决策流
- 池序 174；目标仓库=D:\Vit_DAW（agent Go 侧，零内核/零 webui/零前端改动）；P1（M2 可感收口） / 1 天 / 依赖=无；模型分级：L2 / GLM 执行会话

## 规格权威=设计稿 §3（实现以设计稿为准，本卡只列骨架）

①**触发**：`executeSemanticDynamicTicket` 内、PCA admission 与 surface/拓扑校验通过后挂 **mix-tick 形态 audition bracket**：before render（写前 revision，失败 fail-open 继续+记 degraded）→ applySemanticDynamicController（零改动）→ after render → mount audition session（复用 d1AuditionCandidatesForAssignment+盲测抽签+AuditionPrepare）→ audition.ready 随 semanticDynamicExecutedResponse 同轮发出。单点挂接覆盖全调用者族（C2 批/C2 运行时/147 全访问链/确认续行）。
②**效果类 mutation 触发条件**：按设计稿 §3.5 定义执行（含纯观察轮不挂 bracket 的判定）。
③**判定第三身份通道**：judgment POST 扩展动态链会话路由（mix-tick 同模式），不触实验身份校验（实验契约不变）。
④**会话命名域隔离**：与实验会话命名隔离（设计稿 §3.6）。
⑤webui 零改动（TrajectoryAuditionPanel 判定卡自动出现）；PlanBar 契约不动（入口不挂 PlanBar，6.4s 自清场）。

## 主管对设计稿两修正点的裁定（2026-10-22 深夜）

1. **首版整工程 before/after 对照**（非轨级 bypass）：接受——内核 `start_render` 无轨级参数（TransportAudioService.cpp:1185 实证），轨级粒度=内核增强后置卡，不阻塞首版。
2. **判定第三身份通道**：接受（唯一新增服务端面，实验契约不动）。
（两裁定待用户最终知情确认，实现卡可先行——若用户改轨级粒度，属后置卡不影响本卡。）

## 文件域（领取时按设计稿 §4 卡 A 核对固定）

- `agent/internal/chat/semantic_dynamic_workflow.go`（executeSemanticDynamicTicket 挂点邻域）
- `agent/internal/chat/semantic_dynamic_audition.go`（新增：bracket/mount/身份/判定落账，mix_tick_audition.go 模式）
- `agent/internal/chat/audition_events.go`（handleAuditionJudgment 第三路由邻域）
- `agent/internal/chat/server.go`（仅限 Server struct 新增复用缝字段，mix-tick server.go:61-64 先例）
- `agent/internal/chat/semantic_dynamic_audition_test.go`（新增单测，既有测试文件零改动）
- 零内核/零 webui/零 Godot；mix-tick 复用件（ensureD1Render 低层 helpers、d1AuditionCandidatesForAssignment、AuditionPrepare、emitAuditionEvent、auditionBlindSettingsFor、drawAuditionBlind）verbatim 共享不修改。越域即停。

## 验收标准

单测（bracket 时序 fail-open/幂等/第三路由身份校验/实验会话 409 保持）+真栈 chat 腿=场次④话术复刻（动态链完成→**audition.ready 事件在场+webui 判定卡可渲染**[事件面断言]+双 render 资产落盘+盲测抽签可选手动）；全量 0 FAIL+gofmt 净；端测边界声明。真栈腿登记 PC-RUNTIME-STACK。

## 停止条件

bracket 时序与 settle/continuation 链冲突（写后仍有续行轮）→ 锚点上交（可能需 settle 后延迟 mount 变体）。

- 领取：2026-10-22 22:18 +0800 / origin/main=1c76ee4c467c7c2040b2cd5bb502c903e871dfd8 / owner=GLM-5.3 L2 PC 执行会话（ZCode，174 卡会话，用户口令指定领取）/ 分支=port/ux-ab-audition-impl-1（worktree D:/Vit-DAW-aba174）+ 协调分支 coord/aba174-claim / worktree=D:/Vit-DAW-aba174 / 领取提交=coord/aba174-claim tip
- 回执：（进行中——实现腿完成，真栈 chat 腿待错峰执行）实现 commit=6f7a0621（port/ux-ab-audition-impl-1@origin；领取基线 fbc2b855）。单测=新增 7 族全 PASS：bracket 时序（before render 先于参数写）/fail-open 三降级码（before_render_failed/after_render_failed/audition_prepare_failed 只降级不阻塞）/mutation 失败不挂卡错误原样上抛（settlement fail-closed 语义逐字保留）/命名域隔离+同 ticket 幂等（audition:dynamic:* vs 实验 audition:<exp>:* vs mix-tick audition:mix_tick:* 字面互斥）/第三路由落账（loop.Experiment 零接触，实验契约不动）/身份不符 409/盲测每会话一次抽签。go test ./... 全量 96 包 0 FAIL exit 0；go test ./internal/chat ok 137s；run_ab_result_smoke.ps1 判 A 型纯进程内（worktree -RepoRoot 执行）exit 0。gofmt：两个新文件零输出；三个编辑文件仅既有 CRLF 基线在列，去 CR 后 gofmt -d=0 字节、与领取基线差异集相同（无本卡新增格式问题）。独立复核七项 PASS（域合规 5 文件/挂点时序/第三路由插入位/复跑/gofmt 基线对比/测试纪律零落盘/AB 冒测）。真栈 chat 腿=场次④话术复刻**待执行**——按派发令错峰序=末位（175 复跑之后），PC-RUNTIME-STACK 未占用（腿执行时按 §2.2 登记）。端测边界声明：本轮=进程内单测+隔离型 AB 冒测，**不含真实栈端侧验收**；判定落账只记 disposition 不执行物理参数 rollback（rollback_preferred/rollback_executed=false，回复文案明示）；mutation 失败路径 before-render WAV 可能残留工件（无参数写无挂卡）；D1-owner 路径上实验 settle 尾随的试听会话与 dynamic 卡共享内核单一试听席位（播放可能被接替，判定落账不受影响，与 mix-tick×实验同类共存形态）。
- 回执补（真栈补证腿，2026-10-23 凌晨）：run=coord/runs/UX-AB-AUDITION-IMPL-1/aba174_chat_20261022_233614/；被测件=主仓 VitAgent.exe SHA256 前 8=7538E6CE（bringup 构建门硬校验，174 已含）+kernel K3 722A79D7A3F53797+VspHub B10CC412；栈占用 09d03081 登记；释放=如实交接记录（自录 pid 拆净复核 GONE，活体已被未登记手测形态新占用 00:21 起，按 §2.2 上交归属，未写空闲）。场次④话术「给人声加点空间感」真栈复刻：goal=completed；conversation=aba174_round4_20261023_000846，session=audition:dynamic:dyn_75291e34fad3d653，turn/round=dynamic_turn|dynamic_round:dyn_75291e34fad3d653，project_revision 5→6。断言面：C 双 render 资产 PASS（before_revision_5.wav sha256=539357cb…/after_revision_6.wav sha256=7b20ffa8…，实算与 render_revision 溯源逐位一致、before≠after、文件名含 revision）；D 判定第三路由 PASS（错 turn_id→409 identity mismatch；伪实验域 session→409 走自由态支路=实验 409 保持；正确身份→200 证据 judg_ff6b2804c9c06f52，decision=retain/label=B/physical=after，semantic_dynamic.audition.judgment+trajectory.user_judgment.recorded 事件 seq 44/43 在卷）；E 盲测面 PASS（VIT_DAW_AUDITION_BLIND=1 生效 blind=true、抽签 swapped=false、label 恒 A/B、解盲披露随判定 outcome 体+judgment 事件 payload 在场）；F webui 事件面契约 PASS（audition.ready 带 schema_version=vit.kernel_audition.v1+session.turn_id/round_id+双候选 ready；judgment.requested 带 node_kind/status/details.audition_session_id）；B 事件面 PASS（audition.ready seq 25/27 与 trajectory.user_judgment.requested seq 26 同轮相邻、同 session）。A/B 的 stamp 子面结构性 FAIL 上交裁定：executed 响应 WorkflowData（含 semantic_dynamic_audition stamp）在调度续跑投递边界被丢弃（emitSchedulerChainResultEvent 只送 Reply 文本）——锚点 agent/internal/chat/turn_trajectory_events.go:325-372（域外既有属性，非本卡链路缺陷；同轮证明已由事件面 seq 相邻+单测时序钉覆盖）。轮次口径如实：§8 预算 3 轮，round1 初次=驱动器 mkdir 缺陷崩溃（已修，零 prompt）后重跑完成捕获；round2/3 发 prompt 前崩（authority 409 阻塞/卫兵误判，env_interrupt 形态零 LLM 消耗）；round4 补驱动+--resume 收口，断言终跑 exit 1 仅上述 stamp 子面。附带实证：resume 期第二次语义执行自动挂出第二会话 audition:dynamic:dyn_3bb825c45a53c3bf（bracket 对每次语义动态执行均触发）；实验域 settle 尾随会话共存与预判一致。teardown Stop exit 0 只杀自录 pid。端测边界：本腿=kernel+vsphub+agent 真件 HTTP 面断言，无浏览器渲染面/Godot——webui 判定卡以事件面契约字段（F）代证。
- 验收：（裁定文件 / 验收 commit）
- 补证腿（2026-10-22 23:36–00:2x，run=aba174_chat_20261022_233614 四轮，占用登记 09d03081）：round4=场次④话术复刻「给人声加点空间感」+nudge——主管逐工件亲核五判定面：①**audition.ready ×6+candidate.ready ×3+prepare.started ×3 事件在场**；②audition 会话=audition:dynamic:dyn_75291e34fad3d653（动态链命名域）status=ready+**盲测 blind=true**+双候选（A=before checkpoint 溯源 semantic_dynamic:dyn_…:before，B=after，2.2s 双声道资产）；③**judgment_ok=HTTP 200 第三路由落账**（turn_id=dynamic_turn:dyn_…+会话匹配）；④伪实验 ID **409**（judgment_409_experiment=实验路由拒绝动态链形态）+伪身份 **409**（judgment_409_identity）；⑤goal completed+send_list 读回在卷。
- 验收：**整卡通过**（GLM 主管决策流 / 2026-10-23 凌晨收口）。实现面（7 族单测+全量 96 包+冒测+独立复核，e5e9ffc0）+补证腿五判定面工件亲核——**172 设计稿主案完整落地：提示词「hand the result to the user's A/B audition」承诺兑现，动态链完成后用户拿到 A/B 判定卡（盲测可选）**。端测边界=事件/路由/资产面断言；webui 判定卡渲染归手测场次⑤目检；回执边界四条（rollback 文案明示/before 残留面/单席位共存）注记在卷。验收 commit=主仓 e5e9ffc0+本收口。
- 验收补（执行侧补证回执，2026-10-23 凌晨）：真栈 chat 腿完成——卡面验收面全数 PASS（audition.ready 事件在场+webui 事件面可渲染[契约字段]+双 render 资产 sha256/revision 溯源+盲测抽签面+判定第三路由落账+实验 409 保持+同轮事件面）；附加 stamp 子面结构性 FAIL 上交裁定（域外锚点 turn_trajectory_events.go:325-372）；run=aba174_chat_20261022_233614；执行侧收口移 done 待主管终验。- 验收补二（stamp 子面裁定 / GLM 主管决策流 2026-10-23 早窗）：stamp 子面结构性 FAIL 裁定=**记录豁免不开卡**——executed 响应 WorkflowData 在调度续跑投递边界被丢弃（turn_trajectory_events.go:325-372，域外既有属性）不影响本卡判定面：同轮证明已由事件面 seq 相邻（audition.ready 25/27+judgment.requested 26）+单测时序钉双重覆盖。重开条件=出现 stamp 面真实消费缺口（审计/回放工具需要 executed stamp 完整性）时另开域外卡。
