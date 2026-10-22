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

- 领取：2026-10-22 09:54 +0800 / origin/main=1ae57f23d284abab0af2967f7b71e6bbc00e2c33 / owner=GLM-5.3 L2 PC ZCode 执行会话（162 卡会话） / 分支=port/fs-named-dynamic-wire-1 / worktree=D:/Vit_DAW_wt_ndw1 / 领取提交=本提交（coord/ndw1-claim）
- 回执（执行侧自验完成，待主管验收）：
  - **设计锚点**：注入点=「bindFreeStateAuthoritativeTrack」（free_state_reasoning_loop.go，原 :2418）——勘察结论：它是自由态决策→语义计划交接的**唯一确定性咽喉**（7 个消费点全过此函数：plugin_recommendation×3+axis_coverage+improvement_proposal+goalrunner_chat:504；goalrunner:504 bind 后 selected_track_id→attachSemanticTopologyPlan 源→topology plan Sends[0].SourceTrackID→bus 装载 connect source，一处改写全链贯通）。指名非空且模型另选→selected_track_id 改写=指名轨+user_named_target_rewrite 标记；指名非空模型未绑→指名供目标（无标记）；解析失败/无指针=fail-open 逐字节现状。披露=bindFreeStateContextToResponse（全回复面公共包装器）见标记即追加机器句「按你的指名，目标轨=名/ID。」（Contains 去重防内外双包装重复）+WorkflowData 结构化披露。**文件域实际固定=仅 free_state_reasoning_loop.go+新测试文件+驱动脚本**；plugin_recommendation/semantic_dynamic_workflow/goalrunner_chat/full_access_semantic_chain/plugin_grabber_workflow 零改动；146 §② 红线未触（不进 semanticEntryDecisionFromContext/分类器任何输入，停止条件未触发）。
  - **commit**：实现 a6702c56+工件 227cc261，均 @ port/fs-named-dynamic-wire-1（已推 origin）；单测四面全过（TestFSNamedDynamicWire{BindRewritesPlanTarget,DisclosureOnResponseFace,BindFailsOpenWithoutPointer,WriteGateAcceptsNamedSource}）+全仓 go build OK+go test ./... 0 FAIL+gofmt 内容级净（checkout 全目录 CRLF 为 core.autocrlf 既有属性非本卡引入）。
  - **真栈 run ID**：fs_ndw1_chat_20261022_101808（驱动 exit 0 PASS）——场次③话术逐字「给人声加点空间感」单轮直达 completed，**F1 send 边 source=vocals/1032→FX/1037@-6.0 dB**；判别性=指名指针供目标分支真栈实证（loop.target_ref=null+user_named_target=1032→request_context.selected_track_id=1032，main 同形态无源不成 bus 计划）；「模型另选→改写+披露」面本轮模型随机分支缺席未观测，由单测钉死（诚实边界）。占用登记 bee74f6f/释放 0d0dd944，拆栈复核七端口 tcp+udp=0、四类进程=0。
  - **端测边界声明**：本腿覆盖 agent HTTP 面（chat/invoke/events/send.list）+内核 ZMQ 真实派发（connect+rack_add_node 在卷）；不覆盖 webui 渲染面与用户旅程（无 UI 启动，E2E-WEBUI-1/JOURNEY-1 层级缺位同既往）；模型另选改写的真栈形态未在本轮观测（见 RESULTS.md 判读节）。
- 验收：（裁定文件 / 验收 commit）
