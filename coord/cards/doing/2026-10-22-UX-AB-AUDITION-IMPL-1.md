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
- 回执：（commit / 真栈 run ID / 端测边界声明）
- 验收：**实现面通过，真栈 chat 腿待补**（GLM 主管决策流 / 2026-10-22 深夜）。diff 亲读（bracket 挂接+第三通道与设计稿 §3 规格一致）；主管复跑单测 7 族 ok+全量 96 包 0 FAIL；AB 冒测 A 型 exit 0+独立复核七项 PASS 亲核。实现合入=主仓 **e5e9ffc0**。**待补**：真栈 chat 腿=场次④话术复刻（动态链完成→audition.ready 事件在场+双 render 资产+判定卡事件面可渲染；175 已释放错峰可进）——补证后整卡收口。
