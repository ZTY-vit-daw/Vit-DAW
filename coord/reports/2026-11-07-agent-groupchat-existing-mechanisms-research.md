# 多 agent 群聊：现有方案与可借鉴机制调研

- 日期：2026-11-07（Asia/Shanghai）；整理：Codex，使用 deep-research 快速调研流程与并行来源核验。
- 对应问题：[本次群聊讨论摘要](2026-11-07-vit-open-studio-agent-group-discussion-summary.md)。
- 设计收口：[Vit 开放录音棚协作方案 v1（设计定稿）](../decisions/2026-11-07-vit-open-studio-collaboration-v1.md)。用户已确认总体方案；本文作为来源与机制参考，不再承担最新设计入口。框架依赖与具体实施仍未选定。
- 范围：无 @ 时首位发言者的产生、后续成员读取新增发言、主动参与与沉默、保留外部 harness，以及 token/延迟代价。
- 证据边界：读取了官方文档、项目 README、关键源码和论文；本次没有安装、启动这些项目或调用付费模型验证。以下“已有实现”不等于已独立证明生产稳定性，也不构成 Vit 的选型定案、任务卡或实施授权。

## 1. 结论先行

有可借鉴的现成方案，无需从零发明共享记录、动态选人和接话意愿判断。但应区分三种成熟度：

1. **已有完整工程模式**：Microsoft Agent Framework、AutoGen SelectorGroupChat 等已经实现“公共历史→选择成员→成员回复→广播新增输出→重新选择”。
2. **贴近用户现有使用方式的开源应用**：agentchattr 连接真实编码 CLI；DSH Agent Arena 直接实现持续群聊、成员意愿判断和管理员选人。它们有实际源码，但各自的重要限制不能忽略。
3. **直接研究自然主动性的原型**：CHI 2025 Inner Thoughts 与公开包 thoughtful-agents，研究何时想说、何时沉默、何时插话；有实验与代码，但不是已验证的跨 harness 音乐协作产品。

本次没有证据支持某一个项目已经同时成熟解决“任意厂商原生 harness＋自然无 @ 接话＋低成本＋真实音乐制作”。更合理的借鉴单位是具体机制，而非整个项目的宣传定位。

## 2. 最值得比较的方案

| 方案 | 首位/下位如何产生 | 后续是否消费新发言 | 对 Vit 的主要价值 | 关键限制 |
|---|---|---|---|---|
| Microsoft Agent Framework Group Chat | 程序选择函数、模型 orchestrator 或自定义策略 | 官方明确先同步/广播，再选择并调用下一位 | 独立会话与公共历史同步的工程协议 | 集中协调不等于自发接话；不同语言实现的广播范围需核查 |
| AutoGen SelectorGroupChat | selector 根据成员描述与公共历史选人，可用规则或候选限制覆盖 | 每次回复后广播并重新选择 | 动态首位选择的可运行基线 | 通常增加 selector 调用；官方已推荐后继 Microsoft Agent Framework |
| DSH Agent Arena | 无 @ 的用户首批会启动全员；续聊先收集各成员意愿，再由管理员批准一位 | 首批并发；后续根据新增记录重判 | 最直接参考“成员想接话＋主持人控场” | 首批仍有群发性质；全员意愿判断与管理员都耗模型调用 |
| agentchattr | @ 唤醒；人类无 @ 可配不唤醒、全员或固定成员 | 成员经工具读取公共记录与增量，再接话 | 真实 CLI/harness 接入、身份、游标、有界接力 | 无 @ 不是语义自选；终端注入实现不宜照搬到 Vit |
| agentschat | 无 @ 开场启动全员；后续按成员未读片段定期投递 | 公共日志持续进入各成员独立会话 | 多 harness 适配、分段收件箱、忙时排队与恢复 | 新项目；全体有未读内容者仍可能被调用，首位没有低成本自选 |
| OpenClaw Ambient room events | 无 @ 也作为安静房间事件交给 agent；调用消息工具才公开说话 | 消费最近房间历史；广播模式另有轮次边界 | 把旁听处理与公开发言分开 | 旁听也推理；多人 broadcast 仍 experimental，同轮顺序运行不等于互读 |
| Inner Thoughts / thoughtful-agents | 各成员形成候选想法、评估表达动机，阈值筛选后仲裁 | 新事件触发成员更新想法与参与选择 | 直接对应“为什么此刻想说话” | 研究原型；沉默也推理，调用开销高；非跨原生 harness |

以上支持资料分别见后文的一手链接。表格中的适配价值为本次分析，不是厂商承诺。

## 3. Microsoft Agent Framework：公共历史与独立会话

[官方 Group Chat 文档](https://learn.microsoft.com/en-us/agent-framework/workflows/orchestrations/group-chat)明确：

- 每个成员保留独立 `AgentSession`，不共用同一会话对象。
- orchestrator 在成员每轮执行前同步公共历史。
- 某成员回复后，先广播，再决定下一位并发起请求。
- 支持 `selection_func`、`orchestrator_agent` 或自定义 orchestrator，以及终止与最大轮次控制。

原文关键句：

> “After broadcasting the response, the orchestrator decides the next speaker … updated conversation history …”

这直接支持用户提出的因果要求：B 的输入可以包含已经发生的 A 的发言，而非把几个预先完成的答案依次显示。

边界：Python 默认同步非用户消息的文本，过滤函数调用、结果、审批等工具控制内容。公共讨论不等于复制每个成员的全部内部工具轨迹。它仍是集中协调的轮次机制，没有直接证明自然插话、自愿沉默或原生 Codex/Claude Code harness 已可无缝加入。

[框架仓库](https://github.com/microsoft/agent-framework)提供实现与示例。文档完整度和持续开发不能代替 Vit 场景实测。

## 4. AutoGen SelectorGroupChat：动态选人已有明确实现

[官方教程](https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/selector-group-chat.html)和[源码](https://github.com/microsoft/autogen/blob/main/python/packages/autogen-agentchat/src/autogen_agentchat/teams/_group_chat/_selector_group_chat.py)显示：

- selector 读取公共历史与成员名称/描述，选择第一位或下一位发言者。
- 被选者回复后广播，下一次选择使用更新后的历史。
- 可通过 `selector_func` 明确选人，或使用 `candidate_func` 缩小候选；具体组合受 API 约束，不能假定任意串联。
- 只剩一位候选时跳过模型选人；默认限制同一成员连续发言，但可关闭。
- 选择失败存在重试和回退，因此调度调用数并非恒定一次。

这说明“第一位怎么产生”已有可运行基线，但选择仍依赖模型判断与成员描述，不代表已经找到自然群聊的最优策略。

维护边界：[AutoGen 当前 README](https://github.com/microsoft/autogen)表示不再由 Microsoft 维护并推荐 Microsoft Agent Framework；本次 API 查询的 `archived` 标记仍为 false。应称其为官方推荐后继框架的旧路线，不能精确写成“GitHub 已归档”。

补充：[Swarm](https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/swarm.html)让当前成员通过 `HandoffMessage` 指定接手者，接手者继续使用公共上下文，可减少中央 selector；但首位通常预设，不解决开放话题下的首次自发参与。

## 5. DSH Agent Arena：最贴近本次讨论的持续群聊实现

[项目与版本说明](https://github.com/Tikzen/dsh-agent-arena)称 v0.7.0 对齐 DSH Web `0.2.0-rc.2`，每位角色使用独立顶层 Agent 会话并继承 DSH Preset 的工具、技能与审批策略。兼容性声明只覆盖其指定版本，不能推导所有 DSH 安装都可直接使用。

本次读取了[核心源码 `src/index.mjs`](https://github.com/Tikzen/dsh-agent-arena/blob/main/src/index.mjs)，核对到：

- `collectReplyIntents`（约 2070 行起）用 `Promise.all` 让可参与成员分别判断 `shouldSpeak` 和理由；输入包含公共群聊记录、当前焦点、自己的已发言内容与人格。
- 提示明确区分：回应新观点、纠错、补充关键遗漏或提交进度可接话；重复、附和、等待人类信息或已经收束时应沉默。
- `guardContinuation`（2116 行起）由管理员查看意愿和公共记录，过滤跑题/重复，并只批准最适合接下一句话的一位。
- 该成员说完后再收集意愿，不预排整轮固定顺序。
- 可以关闭自动接话总开关，或关闭某成员的独立意愿判断、交由管理员判断；这是明确的成本控制接口。

**重要反例：它没有完全解决用户的首位问题。** `queueRoomReplies`（2740 行起）在用户无 @、无管理员点名时把所有未静音成员加入目标；`runRoomReplies`（2684 行起）使用 `Promise.all` 执行首批。因此是“首批并发征询＋后续逐位续聊”，不能直接声称从第一句话起就是单一成员自然接话。

成本：开启全部独立意愿判断时，N 位可参与成员的一次续聊判定会包含 N 次意愿运行；有候选时再有一次管理员运行，被批准者再正式运行。以 N=4 为例，这一流程约涉及 6 次角色/模型运行，而非只看见一条回复就只调用一次。实际 completion 数、token、缓存与工具续跑需运行测量；无候选时管理员函数可直接返回，不必调用模型。

## 6. agentchattr：把现成 CLI 放进公共聊天室

[agentchattr 项目](https://github.com/bcurts/agentchattr)使用公共频道与 MCP 工具，让真实编码 CLI 读取和发送消息；@ 可唤醒对应客户端，A 也可以 @B 让其读取更新后的群聊再回答。

核对版本：`ff8d22e01eb57ef543d07556773ead5b2257c63a`。关键源码：

- [router.py](https://github.com/bcurts/agentchattr/blob/ff8d22e01eb57ef543d07556773ead5b2257c63a/router.py)：人类无 @ 默认不唤醒，可配置唤醒全部或固定成员；agent 自己的无 @ 消息不继续自动唤醒。
- [mcp_bridge.py](https://github.com/bcurts/agentchattr/blob/ff8d22e01eb57ef543d07556773ead5b2257c63a/mcp_bridge.py)：`chat_read` 按成员游标读取初始近期历史或之后新增内容，并提醒不要空轮询浪费 token。
- [wrapper_windows.py](https://github.com/bcurts/agentchattr/blob/ff8d22e01eb57ef543d07556773ead5b2257c63a/wrapper_windows.py)：Windows 采用控制台输入注入；Unix wrapper 也通过终端机制投递。

值得借鉴身份、公共记录、增量游标和接力上限；无 @ 的语义接话并未实现。Vit 既有规则禁止主机控制手段，不应照搬终端输入注入；真正接入方式仍应另查受支持的运行时接口。本次没有执行这些 wrapper。

它比仅有概念的小仓库更有公开版本和使用积累，但发布记录、星数不证明对全部 CLI 版本稳定兼容。

## 7. Inner Thoughts：直接研究无 @ 时为何开口

准确论文：Xingyu Bruce Liu 等，*Proactive Conversational Agents with Inner Thoughts*，CHI 2025，DOI `10.1145/3706598.3713760`。

- [论文全文](https://arxiv.org/html/2501.00383v2)
- [出版元数据](https://api.crossref.org/works/10.1145/3706598.3713760)
- [作者开源实现 thoughtful-agents](https://github.com/xybruceliu/thoughtful-agents)

机制不是始终预测唯一下一位：先判断是否已明确指定某人，否则标为 `anyone`；各成员根据新事件检索记忆、形成候选想法、评估表达动机，再根据普通阈值或较高的插话阈值决定参与。公开包仍有集中仲裁，从成员已选想法中选择发言者。

其动机标准涉及相关性、信息缺口、影响、紧迫性、连贯性、原创性和参与平衡。新消息可以改变成员想法；Slack 原型在待处理消息队列未清空时暂不公开发言，提供了“构思期间又有新消息”的处理参考。

成本不能美化：[默认成员处理源码](https://github.com/xybruceliu/thoughtful-agents/blob/4c8d5beb249f852465463ed236fbbff95e0b6b0d/thoughtful_agents/models/participant.py#L146)为每位非发言者生成 1 个 system1 和 2 个 system2 想法，并逐个评估；按该默认调用结构推导约 5 次 completion，另有公共轮次判断、最终措辞、embeddings 等调用。[广播源码](https://github.com/xybruceliu/thoughtful-agents/blob/4c8d5beb249f852465463ed236fbbff95e0b6b0d/thoughtful_agents/models/conversation.py#L190)会让成员实际处理事件，因此沉默也消耗推理。上述是源码推导，不是 token 或账单实测。

证据范围：形成性研究 24 人；模拟为 100 段四 agent、每段 15 turns；真人 Slack 研究 12 人，以随意社交聊天为主。作者承认阈值依赖试调、可能漏接或突兀插话，任务型协作仍需进一步研究。论文关于 next-speaker prediction 的弱表现仅针对其数据与当时模型，不能外推为当前所有模型都不能做路由。

实现提供 PyPI 包、示例与基础测试，采用自己的模型调用与角色记忆，不等于保留外部 Codex/Claude Code 原生 harness。属于值得复现实验的研究原型，不能称为成熟通用群聊产品。

## 8. 其他值得留意的线索与版本陷阱

- **agentschat 的分段投递**：[项目](https://github.com/mocias/agentschat)与[room.cjs 源码](https://github.com/mocias/agentschat/blob/main/lib/room.cjs)直接实现一份公共日志、各成员独立持久 harness session。首条无 @ 消息会并发启动全体；后续未读内容进入每成员片段，默认每 30 秒向空闲且有待读片段的成员投递。@ 提供立即投递与强调，忙成员排队。已开始读取的片段不再合并新消息，新内容排到后面，避免不断重启。JSONL 与成员游标支持恢复。普通成员可返回“无补充”，但判断沉默仍已发生模型调用。这非常贴近持续群聊的数据流，却没有解决廉价的首位自选。README 列有 Codex、DSH、Claude Code 等适配，但仓库创建于 2026-10，不能称为长期稳定产品，也未独立验证全部适配声明。
- **OpenClaw 的安静旁听**：[Ambient room events](https://docs.openclaw.ai/channels/ambient-room-events)将无 @ 消息作为 `room_event` 交给 agent，普通 final 保持私有，只有显式 `message(action=send)` 才公开发言。需关闭相应 mention gate 并提供消息工具。这是“读到不等于说出”的实际机制，不是无需推理的监听；可选房间事件在拥堵时可被丢弃，正常用户请求有更高保留优先级。
- **OpenClaw 的广播反例**：[Broadcast groups](https://docs.openclaw.ai/channels/broadcast-groups)明确标 experimental；同轮可并发或顺序运行，但官方特别说明顺序只改变启动顺序，同轮后者不会看到前者输出。互读通过下一轮有界 sibling final 摘要实现；有轮次和调用预算，但状态在内存中、重启不可恢复。不能把这种 sequential broadcast 写成用户要求的逐次群思考。
- [AG2 Network](https://docs.ag2.ai/docs/user-guide/network/overview/)：有权威 Hub、具名成员、WAL、`causation_id`、非 LLM 的 HumanClient、多进程/跨机链路与收件箱/速率约束，适合作为公共消息底座参考。但内置多人 discussion 仍为 round-robin；最新 1.0 与旧 Classic GroupChat 的 API 要区分，不能因框架名相同混用教程。
- [AgentScope v1 MsgHub](https://doc.agentscope.io/tutorial/task_pipeline.html)支持自动观察广播，但不负责自然选人；[2.0 文档](https://docs.agentscope.io/latest/en/)说明有破坏性升级，不应把旧 MsgHub 说明直接写成最新版本能力。固定 debater＋moderator 示例也不是本次目标。

## 9. 对 Vit 后续讨论的建议

### 9.0 用户澄清的产品前提：公共上下文来自 DAW 现场

用户在进一步讨论中明确：Vit 本身是 DAW，工程、素材与制作活动先构成公共现场，群体进入录音棚/排练室，随后才应用通用群聊机制。下述三个项目分别提供上下文投递、参与协调与调用选择的经验，不应把 Vit 定义为通用多 agent 聊天室加若干音乐工具。

因此，需要先确定哪些工程/媒体版本、轨道/时间范围、试听与实验结果、创作意图和公共讨论可供成员按权限读取，以及变化如何被引用和补读。已有 DAW 数据基础不等于所有外部接口均已实现；聊天主张也不自动成为工程事实。相关定位已补入[讨论摘要 §11](2026-11-07-vit-open-studio-agent-group-discussion-summary.md#11-用户定位澄清daw-提供公共现场群体进入排练室)。

用户提出 Vit 配置模型可能担任录音棚管理员，当前仅记为候选角色。调度应该能消费实际制作焦点与事件，不能只围绕最后一句聊天做文本选人；事件被记录、进入收件箱与启动推理仍须区分，保持既定成本约束。

### 9.1 用户最新确认的成本边界与优先研究方向

用户在阅读调研后明确指出：未发言成员也持续推理虽能支持自主参与，但会消耗大量 token，对本产品目标不现实；同时认可最终汇报提出的三个组合值得优先研究。此处据该反馈更新此前建议，不把原有开放选项继续当作同等优先候选。

| 优先研究部分 | 主要参考 | 要保留的价值 | 不直接沿用的机制 |
|---|---|---|---|
| 公共历史同步与动态选人 | Microsoft Agent Framework；SelectorGroupChat 作机制对照 | 独立会话、先同步新增发言再选择下位、可替换调度策略 | 不因此认定模型 selector 的成本与自然感已经达标 |
| 跨 harness 接入与收件机制 | agentschat | 公共日志、各自持久会话、增量片段、忙时排队与恢复；用户主动入群首轮可单独采用批次回应（见 §9.4） | 不将普通输入或定时投递默认变为全员推理 |
| 参与协调与有限续聊 | DSH Agent Arena；Inner Thoughts 仅作概念研究参考 | 明确点名、参与意愿与公开发言的区分、根据新内容续聊、停止与预算约束 | 不照搬每条新消息都让全员判断意愿，也不采用 Inner Thoughts 的全员持续生成与评估 |

**成本约束作用在公开发言之前。** 如果系统仍然唤醒所有成员，只让大部分成员输出“无补充”或“不想接话”，就没有消除这笔主要开销。群消息可以先保存在公共记录或成员收件箱中，存储、投递与启动模型必须分开讨论；具体何时、凭什么唤醒哪些成员仍未解决。

后续重点应是：如何用有界的调度成本，只启动部分相关成员，并保证其实际收到最新讨论；成员数增加时，不能默认每条消息按全体成员数增加隐藏判断调用。预算数值、是否使用轻量 selector、候选规模与回退策略均待研究，尚无成本测量。

### 9.2 保留的辅助参考与边界

agentchattr 的共享 MCP/游标可辅助比较，但不照搬终端注入。OpenClaw 的安静房间事件可参考其公开发言边界，但不以“旁听不公开”为理由忽略模型开销。Inner Thoughts 的表达动机标准可以启发参与策略；其高成本执行结构退出默认实现候选。

外部 harness 接入与自然参与策略应分别评价：接入成功不证明会自然群聊，自然群聊原型也不证明能保留各家真实工具/会话。后续若做验证，应区分路由、成员意愿、正式回复、工具续跑和失败重试的成本；本次尚未规定实验预算或启动实验。

### 9.3 追加核查：在启动成员前，用有限成本选择谁参与

用户进一步追问上述三个项目是否已经有这方面经验。本次重新读取当前源码与官方文档，得到以下更具体的证据。这里的启动主要指发起成员的模型推理/工具执行轮次；进程保持连接、创建空会话与实际推理成本须分开记账。

**Microsoft 路线提供最清楚的分层接口。** [SelectorGroupChat 的选择入口](https://github.com/microsoft/autogen/blob/main/python/packages/autogen-agentchat/src/autogen_agentchat/teams/_group_chat/_selector_group_chat.py#L152)按以下路径工作：

- `selector_func` 已返回合法成员时，直接采用，不调用 selector 模型（约 163–177 行）。
- 候选只有一位时，直接选择，同样跳过 selector 模型（约 210–215 行）。
- 多候选时，用独立 `ChatCompletionClient` 选人；源码直接调用 `create`/`create_stream`，此选择过程无需启动每个候选的正式 agent 工具循环（约 232–269 行）。
- selector 的 `model_context` 可以单独配置；源码示例用 `BufferedChatCompletionContext(buffer_size=5)` 限制读取的历史消息数。默认却是 `UnboundedChatCompletionContext`，不能把开箱默认视为已限制成本。
- 选人失败默认最多尝试 3 次后回退；调用数和重试可以单独约束。候选筛选也不保证按比例减少输入，因为当前实现仍拼入所有成员描述；消息数窗口亦不等于 token 硬上限。

[Microsoft Agent Framework 官方文档](https://learn.microsoft.com/en-us/agent-framework/workflows/orchestrations/group-chat)也区分 `selection_func` 和 `orchestrator_agent`。后者明确是有工具和上下文的完整 Agent，不能一概称为廉价路由。低成本价值在于把选人接口与成员执行分开，具体模型、输入范围与预算仍需设计和测量。

**DSH Arena 已有跳过全员意愿判断的现成分支。** [collectReplyIntents](https://github.com/Tikzen/dsh-agent-arena/blob/main/src/index.mjs#L2070)在成员 `autoReplyDisabled=true` 时，直接返回由管理员判断的候选，不执行该成员的 `startRoleRun`。如果所有成员都关闭独立判断，续聊阶段便可成为“管理员一次判断→批准一位→该成员正式执行”，而非“全员先判断意愿→管理员判断→正式执行”。[guardContinuation](https://github.com/Tikzen/dsh-agent-arena/blob/main/src/index.mjs#L2116)同时可以决定停止。

这一分支由真实代码支持，但有三项边界：用户无 @ 首批仍然启动全员；管理员本身仍经 `startRoleRun` 调用子 agent（约 1617 行起），并读取完整群聊记录，未自动变成小上下文、廉价直调；关闭成员判断后，候选条目只有名称/ID 和通用说明，不等价于管理员掌握每个成员的真实当前意愿。

**agentschat 主要减少触发频率与重复投递，不做语义首位选择。** [room.cjs](https://github.com/mocias/agentschat/blob/main/lib/room.cjs#L1451)在没有未读内容时不执行；已有运行中的成员不会重入；多条消息可合成一段，只向空闲且有待处理片段的成员投递。读取开始后新内容另排后续片段，避免无限取消重算。约 1472 行的 `_deliver` 对 busy/empty 直接返回。这些都是无需模型的有效节流；然而只要全体都有新内容，全体仍可能执行，所以它不直接回答“谁值得被唤醒”。

单次成功续聊的高层运行数对照（排除首批、工具续跑、重试及供应商内部计算；不是 token 实测）：

| 路径 | 选人/意愿运行 | 正式成员执行 |
|---|---|---|
| Microsoft：程序已确定人选 | 0 次 selector 模型调用 | 选定的 1 位 |
| Microsoft：模型选人且首次成功 | 1 次 selector 调用 | 选定的 1 位 |
| DSH Arena：N 位成员各自判断 | N 次意愿运行，有候选时再加 1 次管理员运行 | 批准的 1 位 |
| DSH Arena：全部关闭独立意愿判断 | 1 次管理员运行；也可判断停止 | 批准的 1 位，或无人执行 |
| agentschat：无新片段 | 0 次 | 0 次 |

可以从三者组合出待验证的方向：程序先处理明确点名、不可用状态、无新增内容与批处理；模糊情形再做一次受预算约束的选择；选定成员补读自己的未读公共记录并正式执行；输出落入公共记录后再判断继续或停止。该组合是本次综合建议，不是三个项目已经联合验证的功能。

尚未得到的证据是：这种有界选择在开放音乐讨论中选人是否合理、是否压制异议、真实 token/延迟是否可接受。下一步不应再问泛泛的“能否调度”，而应核验选人质量与明确预算下的总体讨论效果；本次未启动实验。

### 9.4 用户新增交互方向：主动邀请时全员首轮，之后选择性续聊

用户进一步提出：在 Vit 工程中主动拉入多个 agent，入群时让这些成员同时各回应一轮，建立初始状态。这样首轮由用户邀请触发，之后管理员可以从真实发言、当前制作焦点与分歧中选择接话者。

这改变了前文“无 @ 首批全员”限制的适用范围：DSH Arena 与 agentschat 的批次首轮在用户主动建群/邀请阶段有直接借鉴价值；原有问题集中到首轮之后，不能沿用其逐消息全员触发。并发首轮属于独立初步意见，后续被选者消费新增同伴发言才形成实际接续，不能以顺序展示首轮输出替代群思考。

当前可讨论的组合是：Vit 提供工程上下文→用户邀请批次初始化→agentschat 式公共日志/增量收件→管理员依据已发生内容选择下一位或停止→被选成员补读并执行；明确点名/单一候选可借 Microsoft 的跳过选择逻辑。入群只承担一次明确批次的成员调用，持续阶段保持有界调度，不恢复全员意愿推理。

具体入场简报、首轮失败/超时、用户中途插话、新成员加入与续聊预算仍需定义。该交互方向已记录于讨论摘要 §12，尚无新运行验证。

用户随后认可管理员作为消息投递与回复提醒的统一协调入口，详见讨论摘要 §12.1。借鉴三个项目时，应将公共日志存储、未读收件、调度决定与模型执行保持可区分：Vit 保存原始公共上下文，管理员决定何时交由哪些成员处理；被唤醒者获得相关原始记录并继续自己的会话。统一入口可以直接执行明确规则，不必为每个事件都调用管理员模型。此为当前综合设计方向，不是已运行证明。

### 9.5 用户提出操作分支：工具执行独立于群聊管理员

用户进一步提出允许外部 agent 直接操作 Vit，操作请求不经过群聊管理员。该方向把管理员职责限定于公共对话投递/唤醒；各成员自身 harness 的工具调用走 Vit 独立的权限与执行入口，结果直接返回调用成员，回执及工程变化再进入公共现场。参见[讨论摘要 §13](2026-11-07-vit-open-studio-agent-group-discussion-summary.md)。

本次仓库核对发现已有 `/agent/invoke`、结构化工具目录、执行协调器与单进程工程写租约，具备复用基础；VSP 第三方扩展默认仍拒绝写命令。外部身份授权、工具覆盖和跨成员共享执行协调尚需设计及验证。因此不能把不经管理员模型解释为绕开 Vit 的操作权限或并发控制，也不能将此方向记为已有外部多 agent 写入闭环。

## 10. 检索与局限

检索围绕 `agent group chat dynamic speaker selection`、`agent chatroom codex claude`、`agentchattr`、`multi agent chat group harness`、`proactive multi-party conversation`。通用搜索的可用性不稳定，Google 返回跳转页、DuckDuckGo 要求验证码，Bing 部分结果明显无关，未将这些无关结果作为证据；转用 GitHub repository search、官方文档直读、论文与 Crossref 核验。GitHub API 后续限流，未穷尽所有项目。

优先纳入可核查的一手文档、源码和公开论文，排除仅凭营销文案推导实现的结论。项目自报验证与本次独立核对分开陈述；没有统一环境运行对比，也没有测量自然感、token 或延迟。本文为 AI 辅助快速调研，不是系统性文献综述或生产选型验收。
