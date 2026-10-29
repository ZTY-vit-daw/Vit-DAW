# M3-C4-AUTO-INTENT-1：意图层包络指令——自然语义→段落编舞单轮终态+decision record+workflow 节点（C4 收口=形态案三第二段）

- 发卡：GLM 主管决策流 / 2026-10-29 晚（依据=用户裁定：A-F C4 为底线、往上自然语义为手测期望面（"一句话或不那么指令的行为完成测试"）；197 勘察案二/案三；锚点取 197 RECON §一/§二/§四/§七）
- 派发确认：已确认（用户 2026-10-29 晚方向裁定）
- 验收负责人：GLM 主管决策流
- 池序 218；目标仓库=**主仓 D:\Vit_DAW agent/ 域**；**P1** / 1-1.5 天 / **依赖=217 合入后方可领取**；L2 / PC 执行会话
- 并行与资源：真栈腿 §2.2 登记；与 217 文件域不相交（manifest+路由段 vs chat 意图族+域表+mixboard）串行

## 定位（分层澄清，防执行侧误解）

L2 指令式直达=217（烟测腿+兜底，非手测面）；**本卡=L3 意图层**：自然语义混音话术→agent 自主选择参数/形状/区间→编舞写入。L4 观察证据驱动（「太闷了调亮」→MOM/DOM 证据选参定值）**不在本卡**（本卡目标值来自话术显式值或缺省形状库+确认卡披露，不读观察投影），落地后按效果另裁。

## 目标

1. **自然语义词表三族**（意图路由武装面）：
   - treatment 动词族：复用保全件 free_state_range_goal.go:68-73（197 勘察问 2 保全件在场可读）；
   - 时变限定词族：慢慢/逐渐/渐强/渐弱/越来越 X/推进/铺进来——**触发包络意图的分界词**（无时变限定→既有静态处理，分界指引与 217 路由行衔接）；
   - 语义结果词族→参数类别映射：存在感/能量/音量→电平类（轨 volume ride）；空间/远近/湿→空间类（send/reverb wet）；亮/闷/薄→音色类（EQ/滤波参数，挂了才有，没挂如实报边界不虚写）。
2. **段落区间三通道（不猜原则）**：话术内嵌区间（"副歌那 30 秒"/"0 到 60 秒"）→工程段落标记（若工程有）→反问澄清卡（无区间无标记=问，不自动从音频推断段落——L4+ 候选不伪造）。
3. **形状生成器**：语义→(起值,终值,曲线 c)；显式数值话术用之；无显式值=缺省形状库（渐入/渐出/保持/推进，幅度缺省+**确认卡披露所选参数/形状/值，用户可改后确认**）。
4. **编舞单轮终态**（197 案二形态）：arm→D1-S1 域表新增 automation 包络动作行（点数组载荷，**治理面扩行非 guard 例外**——STOP-NOTE 红线：不得为发令开口子削弱门禁）→materializer 治理发令→set_points 写（唯一前向 mutation）→get_curve 读回断言 settle（复用保全件断言骨架）→终端注记（零自动 revert）。
5. **C4 账面收口**：mix_decision_record.v1 写入（只含 authority 引用，C4 节点）+mix_workflow_queue.v0 节点状态推进（M1-2 骨架）——「现在混音任务进展如何」能报 C4 状态（master plan §6.2）。
6. **webui 编舞呈现最小面**：goal 摘要带范围+动作预告+写确认卡+结构 settle 报告（既有 WorkflowData 卡面零新建为原则；包络写回执摘要形态=勘察问 4 缺口 2 从简补）。

## 探针与验收

- 烟测=**自然话术合成轮**（管线注入+评估器断言，含分界负面例：无时变词静态话术不得触发包络；无区间无标记话术必须走澄清）；真栈腿两轮全绿（话术→编舞→写→读回→record/节点查证）；既有全家零回退。
- 手测终验=用户自然话术（方案另附用户面，非指令式）。

## 文件域

chat 意图族（goalrunner_chat 挂点+free_state_range_goal 保全件改造+experiment 域表扩行）+mixboard（decision record）+workflow queue 面+`scripts/` 冒测+探针。执行侧勘察定案后回执披露文件清单。越域即停。

## 手测终验定义（2026-10-29 晚用户裁定补注）

- **用户手测=一条收口话术+一句读回**（负面例 4b 报边界/无区间反问/静态分界全部留烟测合成轮，不占手测）：「副歌那 30 秒，让这条轨的存在感慢慢强起来——写之前给我看方案，写完汇报一下现在的混音进展。」+「读回刚才那条线」。判定五面：电平类参数自主选择（或理由等效）/确认卡形状+幅度可改/写入+读回一致/C4 进展汇报/听感加分。
- **播放头边界注记**：「从这里开始」类话术第一版合格行为=澄清反问（"从第几秒"），不要求解析播放头位置（transport 位置披露未确证=后续卡候选，不伪造）。

## 停止条件

- 保全件/域表锚点与勘察记录大面积漂移→锚点清单上交；
- 域表扩行无法满足治理面语义（须 guard 例外才能发令）→停止上交两案（红线不越）；
- 段落标记面勘察发现工程格式无段落概念且澄清卡基建缺失→缺口上交，澄清通道降级为话术强制内嵌区间如实声明。

## 领取（2026-10-29 晚）

- 2026-10-29 晚 / origin/main=bf5fa216（领取前即时 fetch 复核=217 验收合入态）/ owner=ZCode GLM-5.3 PC 执行会话（218 卡会话）/ 分支 port/m3-c4-auto-intent-1 / worktree D:/Vit_DAW_wt_m3c4ai1（自 origin/main 干净新建）/ 领取提交=coord/m3-c4-auto-intent-1-claim（本提交）。
- 领取核对：卡在 todo+派发已确认 ✓；依赖 217 已合入（bf5fa216 accept 216+217）✓；doing 唯一卡=GEN-SUNO-WEB-DESIGN-1（Mac 侧设计卡，文件域 webui design 不相交 ✓）；真栈记录最新=217 卡 2026-10-29 21:4x 释放（真栈腿用前即时复核）✓。
- 领取基线记录：主检出 HEAD=bf5fa216、status 脏文件 360 项均为运行时状态（VitApp/Workspace+coord/runs 工件+webui design 未跟踪件），与本卡域（agent/internal chat/experiment/executionports+scripts）无关；实现 worktree 自 origin/main 新建零改动。
- 执行侧勘察定案（领取前完成）：链路=确定性意图路由（时变限定词分界，LLM 前接管对位 INTENT-WIRE 先例 synthesizeClipRangeSplitCommandsPreModel）→pending 确认面（对位 mix_tick 确认链 handlePendingMixTickChat+InteractionFieldControl 可改面）→治理执行链（域表 automation 行 admission+materializer 发令，非 invoke guard 例外——STOP-NOTE 红线遵守）；段落标记面在场（内核 ProjectMarkerService+project.markers.list/upsert 命令族，CommandDispatcher.cpp:2549-2584）；decision record 写入面在场（attachMixboardDecisionProjection 从 terminal PlanningSession 投影，编舞链收口直用）；C4 节点推进=queue 节点挂新 capability id（mixWorkflowQueueNodes C4 行 nil→挂）+decision ledger 按能力路由点亮（既有机制零新建）。停止条件三条均未触发（保全件 5 文件在场可读、域表行族锚点齐、段落标记面+澄清卡基建均在）。
