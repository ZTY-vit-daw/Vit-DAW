# FS-NAMED-DYNAMIC-WIRE-1：用户指名轨接入语义动态链目标选择（157 接线补全）

- 发卡：GLM 主管决策流 / 2026-10-21 深夜（依据=M2 场次③取证：用户话术「给人声加点空间感」，157 确定性解析在自由态循环面成立，但旅程经语义入口路由到 147 全接入动态链后，计划目标轨=模型自选 piano/1027——用户指名 1032 被链路丢弃）
- 派发确认：已确认（场次③取证在卷）
- 验收负责人：GLM 主管决策流
- 池序 162；目标仓库=D:\Vit_DAW；P1（用户意图保真）/ 0.5 天 / 依赖=无（157/158 已合入）
- 模型分级：L2 / GLM 执行会话

## 现象与机制（场次③实录，22:46 栈）

- 22:47:40 `[free-state.seed] booked revision=2 scan_satisfied=true`（158 生效，无边界停）；22:48:50 `track.send.connect source=piano/1027 → auto_create Reverb Bus/1037 @-6.0` + `rack_add_node Pro-R→1037`；22:48:56 goal completed（journal source=free_state_d1_s1，semantic_dynamic_execution_completed）。
- **链路机械面全对、目标轨错**：用户指名「人声」，模型把 send 接到了钢琴。157 的 `resolveFreeStateUserNamedTarget` 只挂了自由态循环 prepare（`free_state_reasoning_loop.go`）→ closure frontier；模糊意图在 full-access 下经语义入口路由的**动态链计划目标轨是模型自选**（plugin recommendation plan 的 track），确定性指名指针在交接面被丢弃。

## 范围（规格方向=C4 裁定延伸：用户指名=意图搬运，不经过模型裁量）

语义入口→动态链交接面（free-state decision → plugin recommendation / semantic dynamic 工作流的计划构建/校验点）消费 `user_named_target`：**指名解析成功时计划目标轨必须=指名轨**（模型另选时确定性改写+回复披露「按你的指名，目标轨=人声/1032」；模型建议轨不同可在回复里附带说明，但不改变执行目标）；解析失败/多义=fail-open 现状（模型自选不变）。与 155 写门互动作核对：指名轨必然在观察披露（解析来源=内核轨集），写门零冲突。第一步=锚点设计（交接注入点的精确选择），第二步实现+测试。

## 文件域

设计先行锚点：`agent/internal/chat/free_state_reasoning_loop.go`（UserNamedTarget 读取面）+语义计划交接面（`plugin_recommendation.go` 计划校验/`semantic_dynamic_workflow.go`/`goalrunner_chat.go` 目标消费点——以勘察结论为准固定精确文件域后再实现，越域即停）。

## 验收标准

单测（指名改写+披露文案+fail-open 现状+写门联动）+真栈 chat 腿=场次③话术复刻（「给人声加点空间感」→ connect source=**vocals/1032**）；全量 0 FAIL+gofmt 净；端测边界声明。

## 停止条件

交接面牵动 semantic_entry 分类器契约（146 §② 红线：注入点不进分类器）→ 停，锚点上交换注入点。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（设计锚点 / commit / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
