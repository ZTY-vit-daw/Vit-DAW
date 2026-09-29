# 2026-09-29：vit note 概念定案——框选便签子代理（用户命名，三轮设计讨论收敛）

## 概念

**vit note**：DAW 前端（Godot）中，鼠标框选区域 → 原位弹出常驻便签式对话小窗 → 输入自然语言 → 携框选空间上下文（轨/clip/时间窗）进入 agent 工作流。**每个 vit note = 一个独立子代理会话**；webui = 主代理入口。

## 三线合流（各有仓库准备，vit note 是交点）

1. **空间锚定 UI**：框选交互（钢琴卷帘已有框选先例）+原位便签容器+跟随鼠标气泡（hover 快捷键提示层，与便签共享空间感知底座）。
2. **会话体系**：conversation/history 本就多会话并存（对话图/分支）——每 note 一个对话流，上下文隔离、历史独立；**共享工程级观察投影**（DAD/CCB 不属会话）→ 新 note 零冷启动（自动带工程观察现状）。
3. **子代理结构**：PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1 的 Worker 概念（:275，v1 未要求、留口未落实）；`agent/internal/orchestration` 已有 OwnerPolicy（会话所有权仲裁+authority_conflict）+CapabilityDefinition（Effects/RiskCeiling）+filelock——协调底座在。

## 关键设计裁定（本轮讨论定案）

1. **主/子代理**：webui=主代理（全权）；vit note=子代理（受 RiskCeiling 约束的受限代理）。
2. **权限分级映射**：vit note 默认 `bounded_reversible` 写权（小操作直接执行）；超出风险级（装载插件/批量处理）→ 升级提案给主代理确认。"子代理受监督"复用能力注册表面，不新发明权限系统。
3. **写并发**：v1 会话级隔离+读写分级（观察真并行、写按 RiskCeiling）；v2 引入 orchestration Worker 实例化+owner 仲裁（authority_conflict 队列）。
4. **webui 同步**：输入必须走 chat 主管道（禁止旁路触发）→ 会话历史天然同步，webui 刷新即见；webui 侧加"框选上下文徽章"（范围摘要显示）小卡。
5. **常驻/临时统一**：轨道监督代理=常驻 vit note（锚定轨、长生命周期）；框选 vit note=临时（任务完成归档）。原"每轨道一监督子代理"设计被统一。
6. **演进线**：原位结果卡（v1）→ 便签多轮（v1）→ 回复即 DAW 对象（与 GUI 线判定卡/确认卡合流，v2+）。

## 落地节奏

- 原"混音能力闭环后再做子代理"排序部分保留：**vit note v1=子代理语义、单代理实现**（独立对话流+读写分级，不被编排协议阻塞）；v2=orchestration Worker 真实例化。
- 前置卡：VITNOTE-RECON-1（只读勘察，见 cards/todo/）→ DESIGN（决策侧亲自）→ v1 IMPL。
- 手测节点：入 manual-test-nodes 表为 M8（框选触发 vit note——全新交互形态质变点）。
- 答辩/论文价值：空间锚定上下文传递是对话流与 MCP 协议均不可达的形态，直接支撑"AI 必须长在 DAW 里"论据；音频 DAW 领域无先例（图像领域 PS 生成填充有成功范式可引）。

## 用户原话锚（设计意图存档）

- "直接通过鼠标框选位置然后出现文字输入框输入内容进入 agent 工作流触发任务……这绝对不会被任何人说是已有的产品形态，它可以证明 DAW 侧是完全有必要的，我们的 AI 融入比起其他的插件或 agent mcp 更紧密。"
- "当我们框选然后打开输入框后，这里是常驻了一个便签一样的对话小窗……然后还可以在输入框继续要求修改……这样我们甚至可以多开了！那这样设计对话流的逻辑就要发生变化了，可能每一个框选就是一个新建对话流。"
- "所有的便签都从子代理结构落实，只有打开 webui 才是主代理……一个框选便签就是一个子代理，我称之为 vit note！"
