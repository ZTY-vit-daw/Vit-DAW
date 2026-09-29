# Ruling：VITNOTE-RECON-1 pass（2026-09-29，决策侧）

- **裁定：pass**。报告合入 main（cherry-pick 42a0659a）；八节全落+锚点密集+汇总三态表+端测边界声明，超出卡面验收线。
- **决策侧核验（非转述）**：报告全文核读+承重锚点抽查五处——A2 chat/server.go `go func` 计数=0（grep 实测）✓；A3 OwnerPolicy.Select=引擎归属仲裁语义（EngineLegacy/V1+PlanningSession）✓；A4 CompleteImageUnderstanding 全仓零生产调用方 ✓；A1 空 conversation 新建 `"chat_"+randomID()` 在（server.go:2258，报告行号微漂 5 行内）✓；A5 E2E 工件另见 FIX-AUDITION ruling。
- **核心产出采信（八节八关键事实，DESIGN 输入）**：
  1. §1 编排面**轨×clip×时间窗语义端到端在位**（marquee 全局 rect 广播协议+跨轨命中收集+TimelineEditStore 三套选区+request_context 组包链现行存活）；跨视窗统一层需新开（两先例：全局 rect 广播+TimelineInputArbitrator 全局输入仲裁）；viewport_registry 的 VIEW_DESCRIPTORS=selection semantics provider 天然挂点。
  2. §1 缺口：marquee 原始矩形不组包（空框选无 clip 命中时不产生时间窗）+音符组上下文无出口（钢琴卷帘框选在但 get_selected_note* 零命中）。
  3. §2 IME 生产实证（主 chat 输入框即 LineEdit 中文在用）；多 note 焦点路由需新开面。
  4. §3 **口径修正**：chat 入口=7878 HTTP 非 vsp_hub（vsp=agent↔内核数据面）；空 conversation_id 服务端新建——"每 note 一对话流"前端侧只需各持空 ID。
  5. §4 **并发模型=多会话并行**（十步代码证据链：每请求一 goroutine 同步跑完整 loop、无全局闸、写串行化仅命令/CAS/文件锁三粒度）——**vit note 写并发治理不能指望 loop 层天然串行化**，v1 即需 RiskCeiling 执行闸。
  6. §5 webui 刷新同步在（事件回放水合）；会话列表+上下文徽章需新开面（FocusSummary 可仿）；**E2E-WEBUI-1 playwright 基建未建**（package.json 无 e2e）——与 FIX 系列卡的渲染验收基建现状交叉印证。
  7. §6 orchestration 三处零实现（Worker 实例化/RiskCeiling 执行消费方/authority_conflict 队列）+**OwnerPolicy 语义澄清（引擎归属仲裁非会话所有权；会话所有权=controllerOwners 按 conversation_id）**——概念文档勘误随本 ruling 落。
  8. §7 鼠标跟随气泡**不存在**（现为固定位置提示行：VitContextManager hover 栈+GHOST 态固定胶囊）——用户记忆中的"跟随鼠标气泡"为未实现构想；共享空间底座需新开面。
  9. §8 llm 多模态客户端已建已测零接线+Godot 视窗截图零实现+隐私外发清单（唯一外发点=provider endpoint；遥测无内容；browsercapture user_approved 批准语义可援引）。
- **对 VITNOTE-DESIGN 的直接含义**：v1 面孔清晰——编排面 L1 先行（链路全在+两个缺口小开面）+便签容器三档先例+每 note 空 conversation_id+RiskCeiling 执行闸新建（不可省）；L2 视觉兜底接线=CompleteImageUnderstanding 接工具面+Godot 截图封装；写并发治理（观察真并行/写分级闸）进 DESIGN 首节。
- 零代码无复跑项；只读卡端测边界声明合规。
