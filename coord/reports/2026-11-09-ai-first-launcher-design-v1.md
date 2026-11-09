# AI-first 启动面对话窗口设计 v1（AI-LAUNCHER-DESIGN-1，卡 332）

- 产出：ZCode GLM-5.3 L2-worker PC 设计子代理（会话 332design）/ 2026-11-09 / 零栈零 LLM 纯设计（两仓只读勘察+本文档）
- 权威输入（裁定链，冲突时以裁定链为准）：[裁定 2026-11-09-ai-first-launcher-ruling](../decisions/2026-11-09-ai-first-launcher-ruling.md)（用户定向+展演红线+纪律三条）；[AI_FIRST_INTERACTION_DISCUSSION_2026-10-28](../../docs/AI_FIRST_INTERACTION_DISCUSSION_2026-10-28.md)（门厅三问/交班—回岗——本设计**吸收**门厅为窗口默认内容形态）；[313 设计](2026-11-08-legame-ux-design-v1.md) §2.4/§3/§5（其 onboarding/HUD 分档随裁定转入本轨道）；[314/315/316 回执](../cards/done/)（目录解析/自动开场+恢复播报/重铺副本制——已合入可复用零件）；[VIT_IPC_CONTRACT](../../docs/VIT_IPC_CONTRACT.md)；[生成线裁定](../decisions/2026-11-09-gen-capability-line-ruling.md)（生成引擎产品化已移出 12 月窗口）
- 消费方：主管终验+用户终审（惊艳点与产品形态）→ 拆实现卡（§7）
- 本文不改任何在案裁定；设计张力与裁定点集中在 §6/§8

---

## 0. 现状事实基线（本轮亲核锚点，实现卡直接引用）

| # | 事实 | 锚点 |
|---|---|---|
| F1 | Godot 主场景=启动页 `app/startup/start_page.tscn`；启动页自举三件套（VitApp 内核/VspHub/VitAgent，候选路径+新鲜度排序+重试），就绪探测=IPC ping（UDP 4445）/agent `/agent/tools`（超时 8s）/hub `/health` | 前端仓 `project.godot:14`；`app/startup/start_page.gd:99-124` |
| F2 | 现行进入工程链（启动页）：`project_client.open_project`（内核 IPC）→ 成功后 fire-and-forget POST `/agent/invoke`（`version.project_opened`+工程身份，`confirmed:true`）→ `change_scene_to_packed(VitDockRoot.tscn)`。**内核打开与场景切换的写者=Godot**；agent 只是事后受播 | `start_page.gd:648-699, 970-1051` |
| F3 | 新建链同构：`new_project`（可带 `project_name`，内核已消费）→ `version.project_new` 播报 → 进 dock | `start_page.gd:766-794`；IPC 契约 new_project 节 |
| F4 | 现聊天窗=`LLM_Chat_Window`（独立 OS Window 节点，VitDockRoot 子节点，默认隐藏，Top_Transport_Bar 切换）；控制器 `LLM_Chat_Controller.gd` 8133 行单体——**展演红线零触碰** | `vit_dock/scenes/vit_dock_root.gd:115,1391-1413,3095`；`app/shell/LLM_Chat_Window.tscn` |
| F5 | **reply 驱动 UI 导航先例已在案**：chat 回执 `side_panel_request` → 打开 artifacts/browser/macro 侧板并选中 artifact；回执 `result` → `select_clip`/`remove_clips` 经节点组驱动时间线。新启动窗的反向导航=同一模式的新字段 | `LLM_Chat_Controller.gd:7378-7395, 7401-7420`；agent 侧 `chat/server.go:980`（side_panel_request 产出） |
| F6 | agent 事件通道 `/agent/events`：按 conversation 增量拉取（GET + `since` seq 游标 + limit），进程内缓冲 500 条——**既有轮询事件面，无需新建** | `agent/internal/chat/events.go:20-88` |
| F7 | `/agent/state` 已披露 kernel/shadow/goal/note_sessions/capability_routes/authority_mode；未直接披露「当前绑定工程身份」（note_sessions 内部已取 `harness.CurrentProjectIdentity`） | `chat/server.go:870-892` |
| F8 | authority 模型：`manual_confirmation`（默认）→ `full_project_access`；服务端状态唯一真源 `/agent/authority`，每 chat 轮绑定（请求体只能复述不能授予）；**跨工程身份边界显式闩自动跌落**（A 工程的 full access 不泄入 B） | `chat/turn_control.go:19-80`；`chat/server.go:7556-7570`（AUTHORITY-LOST-1） |
| F9 | 工程生命周期工具已编目：`project.recent`（get_recent_projects，RiskDirect 只读）/`project.open`（RiskConfirm）/`project.new`（RiskConfirm）；`/agent/invoke`+`confirmed:true` 是程序化执行面（启动页已用） | `agent/internal/tools/catalog.go:1057-1070` |
| F10 | **B5 跨身份切换语义已在案**：activation 观察最终态；跨 uuid/路径切换=旧工作区在飞链显式收尾+内存会话态整体替换；同工程重开=「丢弃未保存态」既有设计边界。这是 agent 侧对「对话中切工程」的结构性背书 | `chat/server.go:7494-7570`（B5 注释块） |
| F11 | 无活跃工程时 chat 可用：`projectUUID==""` → activation 早退（不物化 draft），会话走进程态；写路径（ProjectNew）才物化 default draft。launcher 阶段对话不需要先开工程 | `chat/server.go:7503-7524`；`history.go:226-247` |
| F12 | 会话分卷=按工程：history Repo/ConversationGraph 落工程旁 `.vit_history/`（对话图/worktree/分支）；跨工程不共享卷 | `history/history.go:124-183`；AGENTS §2 |
| F13 | 315 已合入零件：binding sidecar `.vit_agent/legame_binding.json`+自动开场检查点（activation+restore 完成后、确认面之后）+**恢复播报确定性模板**（零 LLM：「接着上局：{节点}（n/total）——继续或问我都行」）——交接播报降级案直接复用 | 315 回执；`chat/edu_jam_chat.go`（eduJamRecoveryBroadcast） |
| F14 | 内核 `save_project` 语义：未命名会话返回 `require_path`（非 error，前端应转另存为）——未保存治理的既有内核钩子；内核遥测无「工程已打开」主题（反向导航不能靠内核遥测） | VIT_IPC_CONTRACT 工程状态机节；`telemetry_manager.gd:318-395` |
| F15 | `ChatResponse` 结构面富余：`Workflow/WorkflowData`（edu_jam_entered 先例）、`TypedEvents`、`ProjectResultCards`、`SidePanelRequest`——launcher 指令/结果卡/流标记有充分先例字段可循 | `chat/server.go:291-326` |
| F16 | `change_scene_to_packed` 只换 current_scene：挂在启动页下的节点随场景销毁；跨启动页→dock 存活的窗口必须挂在 root/持久层或切场景前 reparent | Godot 场景语义；`start_page.gd:1031-1051` |
| F17 | dock 内**无回启动页路径**（dock 脚本无 change_scene 引用）——今天切工程=重启应用；反向导航是新交互面但不在 dock 旧面内 | `vit_dock/scenes/*.gd` 勘察（grep change_scene 零命中） |

---

## 1. 信息架构

### 1.1 一句话

**启动即对话窗**：应用第一面是一个以对话为主体的窗（下称**常驻对话窗**，persistent chat window）；经典工程选择页（磁带流）同屏一键可达；工程打开后对话窗不消失，从「启动形态」切换为「工程形态」继续常驻——**工程是视图，对话是主线**。

### 1.2 三态一窗

```
[态 A · 启动台]                     [态 B · 进入中]              [态 C · 工程内]
┌────────────────────────┐        ┌────────────────────┐      ┌──────────────────────┐
│  ┌──────────────────┐  │        │ 对话窗收窄/过渡      │      │ dock 工作区           │
│  │  对话窗（主面）    │  │  ──▶   │ agent 桥接播报       │ ──▶  │ ┌────────┐ ┌───────┐ │
│  │  消息流+输入       │  │        │ 「正在打开《X》…」   │      │ │ 时间线  │ │对话窗  │ │
│  └──────────────────┘  │        └────────────────────┘      │ │ …      │ │(侧挂)  │ │
│  [磁带流·一键展开] →     │                                   │ └────────┘ └───────┘ │
│  经典页全功能 fallback  │                                   │  (transport 切换钮指向  │
└────────────────────────┘                                   │   常驻对话窗)           │
                                                              └──────────────────────┘
```

- **态 A（启动台）**：`start_page.tscn` 仍是主场景与自举宿主（F1 的三件套启动/重试/就绪探测**零改动**）；新增常驻对话窗组件为视觉第一面（居中主位）；磁带流收为背景层/一键展开抽屉——经典工程选择、文件对话框、引擎设置全部原位可达（G7 信号面直接复用）。
- **态 B（进入中）**：对话内触发 open/create → 治理确认（如有在开工程，§3）→ 复用现行 open 链（F2）→ 窗内一条系统样式过渡播报，非模态、不换窗。
- **态 C（工程内）**：进 dock 后同一对话窗实例侧挂常驻（挂点+存活机制见 §2.4）；消息流中的**分卷分隔条**（volume divider）标记「── 进入工程 · 《名称》 ──」。旧 Ask Vit 窗（LLM_Chat_Controller）保持零触碰、仍可开合，transport 切换钮重指常驻对话窗（dock root 插点，非控制器改动）。

### 1.3 为什么壳不动、窗新增

- 启动自举（F1）、open/new 链（F2/F3）、降级路径全部在 `start_page.gd` 内成熟在案——壳保留即降级保险天然成立（纪律一条：双轨）。
- 对话窗是**新组件新场景文件**，不进 `LLM_Chat_Controller`（红线）；最简形态=消息流+输入框+agent 事件卡三件，其余（附件/计划/确认面/分支栏）展演版不带——工程内的重交互仍由旧 Ask Vit 窗承接（零迁移）。
- 单窗常驻的实现要点是**跨场景存活**（F16）：窗节点挂 root/持久层（autoload 单例持有）或切场景前 reparent；态 A/态 C 只是同一窗的两种布局参数。

---

## 2. 对话流模型（本卡最高价值节）

### 2.1 两流定义

| | launcher 流（启动卷） | 工程流（工程卷） |
|---|---|---|
| 绑定态 | 无活跃工程（activation 早退，F11） | 工程 uuid+路径已绑定（`.vit_history` 卷，F12） |
| agent 能力面 | 只读工程清单（`project.recent`）+ open/create **指令出口**（§3）+ 一般对话 | 全量工程能力，走既有 authority 升级链（F8） |
| 会话持久 | 进程内会话缓冲（展演版不跨重启——诚实边界） | 工程卷持久（既有） |
| 窗口徽章 | 「启动台」 | 「工程 · {名称}」 |

两流共享同一窗口同一消息流；**卷边界**（conversation_id 变化+工程绑定变化）由窗插入分隔条呈现。agent 侧不建新会话系统——分卷即 F12 既有事实，本设计只把它变成**可见的连续性**而非断裂。

### 2.2 agent 自主路由语义

- **判定规则（结构性，不靠猜）**：流归属=当前工程绑定态（F10/F11），不是文本分类。agent 的「自主路由」体现在两处动作：
  1. **launcher 流内识别工程意图**：消息含「打开/继续/新建工程 X」语义 → agent 查 `project.recent` 匹配（路径/名称/模糊）→ 匹配唯一 → 回执带 `launcher_directive`（§4）+一句自然语确认；匹配多个/无 → 回工程**结果卡**（ProjectResultCards 先例，F15）让用户点选（确定性退路）；意图不明 → 正常追问，不出指令。
  2. **工程流内识别跨工程意图**：消息指向另一工程（「换到 X 试试」）→ 同上出 directive，但**必经治理确认**（§3.3）；指向非工程话题 → 留在工程流，不路由。
- **切换时机**：directive 被 UI 执行成功（进工程/切工程完成，`version.project_opened` 受播后）即卷切换；切换由 UI 回执驱动，agent 不预写工程卷状态（B5 收尾语义由 activation 既有路径处理，F10）。
- **上下文携带**：跨卷携带=**桥接播报**（bridge line）而非上下文注入：进入工程后 agent 首条消息=工程态摘要（「已进入《X》。上次……」）——有乐局 binding 的工程直接由 315 自动开场/恢复播报承接（F13，零新增）；无 binding 工程给一句最小摘要模板（工程名+最近修改时间，来自 recent 元数据，零 LLM 或一轮 LLM 均可，实现卡定）。launcher 卷的原对话**不注入**工程卷（防跨卷污染，与 AUTHORITY-LOST-1 同哲学）。
- **用户可见性（轻不吵）**：切换的全部 UI 呈现=①一条系统样式分隔条（进入工程/切换工程）；②composer 头部流徽章变化；③agent 桥接播报一条。无弹窗、无换窗、无闪断。分隔条样式与 315 恢复播报同族（系统消息样式，非气泡）。
- **展演确定性退路**（§8 风险 1 的预案）：结果卡点选开工程全程无 LLM 依赖——「惊艳路径」是白话直达，「可靠路径」是 agent 列表+点选，两者同一链路。

### 2.3 历史分卷衔接

- 卷内历史：工程卷照旧（`.vit_history`，分支/回溯全既有）；启动卷展演版=会话内存（重启即失，窗内如实不承诺）。
- 卷间衔接：窗持有**本地拼接 transcript**（两卷消息+分隔条连续显示；重开应用后工程卷可由既有 history 面回看，启动卷不回看）。答辩后升级项=跨工程会话持久化+卷缝合回放（裁定后置清单原项）。

### 2.4 单窗常驻（案 A）vs 交接播报降级（案 B）——两案并列

| | **案 A：单窗常驻**（裁定目标形态） | **案 B：交接播报降级** |
|---|---|---|
| 窗口生命周期 | 窗挂持久层，启动页→dock 全程同一实例、同一消息流 | 启动窗随启动页销毁（不做出存活机制）；进工程后由工程侧承接 |
| 工程侧承接 | 同窗侧挂 dock（dock root 挂点+transport 钮重指） | 复用 315 恢复播报（F13）：进工程首条消息=「接着……」接管播报；承接面=旧 Ask Vit 窗（零改动）或新组件第二实例 |
| 连续性呈现 | 视觉连续（分隔条）+播报连续 | 仅播报连续（对话「叙事上」连续、视觉上重启） |
| 新增工作量 | 持久层挂载+切场景时序+dock 挂点+transport 重指 | ≈0（315 零件已合入；仅 agent 侧桥接播报扩展到无 binding 工程） |
| 风险 | 场景切换竞态（F16）、双窗并存（旧 Ask Vit） | 「惊艳」减半；启动卷对话进工程即丢 |
| 退路 | A 的持久层做不稳→当场降 B，其余件不动 | — |

**建议：A 为展演目标、B 为 A 的结构内退化态，实现序=先 B 部件后 A 叠加。** 依据：①裁定定调「单窗常驻+agent 自主切换」是惊艳点的骨架（用户原话「用户仍可继续在对话窗口工作」）；②A 与 B 共享约九成部件（最简组件、directive 链、播报复用、治理确认），A=B+「窗口持久化+dock 挂点」两件增量，增量单独成卡（§7 C3），freeze 前砍 C3 即整体退 B，不动其他卡——降级同骨架可退（卡面要求的性质）；③B 本身不是废案：agent 起不来/持久层异常时，产品仍以 B 形态工作（降级路径，§5）。

---

## 3. 工程生命周期命令面

### 3.1 launcher 权限档（结构性准入，非 authority 新模式）

- **语义**：launcher 档=「无活跃工程态下 agent 能力白名单」：`project.recent`（只读）+ open/create 的 **directive 出口**（agent 不执行打开，见 §4）+ 一般对话。**不新增 authority_mode 枚举**——结构性准入由绑定态天然成立（F11：无工程则工程内工具无对象），agent 侧只需一个 launcher 态提示（chat 请求 context 带 `launcher:true`，用于开场白/意图路由的提示词分支，不是权限声明）。
- **与 authority 升级链的关系**：进工程（directive→UI 开→`version.project_opened`→activation）后一切照旧——manual_confirmation 起步、按工程持久模式恢复（F8）；full_project_access 仅 `/agent/authority` 既有链授予；跨工程切换闩跌落（AUTHORITY-LOST-1）恰好是「对话中换工程」的安全背书。**零改 authority 代码**。

### 3.2 list / open / create 产品化

- **list**：launcher 白话问「我有哪些工程」→ agent `project.recent`（F9，RiskDirect）→ 窗内结果卡（名称+路径+修改时间；点击=直接开，确定性路径）。
- **open**：唯一命中→directive；多命中/无→结果卡/说明。打开动作由 UI 执行（复用 F2 链含加密 `.vit` 的 `encryption` 选项、采样率跟随等既有细节，`start_page.gd:970-978`）。
- **create**：白话「新建一个叫 X 的工程」→ directive `{action:"new_project", name}`→ UI 走 F3 新建链（`project_name` 内核已消费）。乐局工程（有 binding）新建后自动开场由 315 既有机制承接，无需新接。

### 3.3 未保存切换治理确认

- **触发**：任何 open/create directive 到达时，若**已有工程在开**（态 C 内切换，或启动台残留会话）→ UI 弹**治理确认**：「当前工程《Y》可能有未保存改动。切换前：[保存并切换] [直接切换] [取消]」。启动台→首开（无在开工程）不确认（零摩擦，展演主路径）。
- **dirty 检测（展演版保守，答辩后精确）**：展演版**一律确认**（宁多勿漏——工程切换在展演里本就是低频亮点动作，一次确认不伤体验）；「保存并切换」走 `save_project`（遇 `require_path` 转 `save_as_project`，F14 既有语义）。答辩后升级=dirty 精确检测（agent `project.undo_state`/shadow 比对），无改动静默直切。
- **确认的归属**：治理确认在 **UI 层**（人是确认者），不在 agent RiskConfirm 链重复——agent 出 directive 不带执行权，UI 是唯一写者（F2 语义不变），一次确认即闭环。

---

## 4. 反向导航（agent open → Godot 进工程页）

### 4.1 事件面勘察结论：**够用，零内核改动**（不触发卡面停止条件）

选型：**reply 驱动指令**（主）——launcher 窗是 chat 客户端，回执直接带结构化指令，窗消费后执行 open 链。这完全复刻 F5 先例（side_panel_request/select_clip 模式）：agent 回执→前端组件→场景树动作。

```jsonc
// ChatResponse 新增字段（agent 域，~20 行；字段族先例=F15）
"launcher_directive": {
  "action": "open_project" | "new_project",
  "file_path": "D:/…/X.vit",        // open 时必填（.vit 由 UI 补加密选项）
  "project_name": "X",              // new 时必填
  "reason": "你说要继续上周的混音",   // 播报用一句话（agent 生成，可选）
  "requires_governance": true       // UI 侧仍自查，双保险
}
```

### 4.2 现有事件面盘点（够/缺）

| 面 | 现状 | 用途 |
|---|---|---|
| chat 回执结构字段 | `SidePanelRequest`/`WorkflowData`/`ProjectResultCards` 先例在案（F5/F15） | **主通道**：directive+结果卡+流标记 |
| `/agent/events`（seq 游标拉取） | 既有（F6） | 备份通道：多客户端对账（旧 Ask Vit 窗同步感知工程切换）；展演版可不用 |
| `/agent/state` | 未披露当前工程身份（F7） | **小缺口（可选）**：+`current_project{path,uuid}` 两字段，供窗徽章对账与降级自愈（启动窗重开时恢复态 C 判断）。建议随 BE 卡带上（~10 行） |
| 内核遥测 UDP 4444 | 无工程打开主题（F14） | 不用（反向导航不经内核遥测；kernel 打开仍由 Godot IPC 发起，写者不变） |

### 4.3 写者不变式（安全设计核心）

**agent 决定与呈报，UI 执行打开**：kernel `open_project` 与 `change_scene` 的写者保持 Godot（F2 语义原封）。理由：①单写者无双开竞态（内核 `open_project` 会销毁当前 Edit，双写者=数据面风险）；②未保存治理确认天然落在人手边；③agent 侧 `project.open` 工具（RiskConfirm）保持原语义不豁免——展演线根本不调它，指令出口绕开执行权。此为**治理链不动**红线在本设计的落点。

---

## 5. 启动生命周期

### 5.1 接线（零改动面声明）

自举链完全复用 F1：`start_page.gd` 拉起内核/hub/agent（候选+重试+就绪探测）。常驻对话窗作为 start_page 的子组件在 `_ready` 即挂出（**视觉先行**：窗体+开场白骨架立即渲染），输入框使能跟随 agent 就绪态（`_wait_for_agent_ready` 结果，F1 既有布尔）。

### 5.2 异步防白屏

- 窗体骨架（标题+消息区+输入框禁用态+「正在唤醒搭档…」状态行）在进程启动即出，**无空白期**；
- agent 就绪（≤8s 探测，F1）→ 输入解禁+agent 开场白（门厅简版：在场问候+最近工程一条+「想从哪开始？」——**吸收门厅三问为窗口默认内容形态**：展演简版回答「谁在/有什么/从哪开始」，晨报/账本全量答辩后）；
- 内核就绪独立探测：磁带流/结果卡在内核就绪前禁用（现状行为），agent 对话不被内核阻塞（launcher 流只需 agent）。

### 5.3 降级路径（agent 起不来→经典页零阻塞）

- **agent 探测超时**（重试后仍失败）：窗降级为**引导卡形态**——「搭档暂时不在。可以先直接选工程（右侧/下方经典页全功能），或查看排查指引」+「重试」钮（重跑就绪探测）。产品化文案+可操作出口，不裸错（与 313 §2.4 L1-d 案 A 失败引导**同款问题同款解法**——引擎托管已归生成线裁定移出 12 月，但「依赖服务起不来的产品级引导」交互模式在此沿用）。
- **经典页全程可达**：对话窗降级后自动收起为侧条，磁带流恢复主位——**双轨保险**（纪律一条）落成：新面不 ready 按老路走，open/new 链不经 agent（F2），零功能损失。
- **agent 中途崩溃**（对话中）：窗内联错误条+自动重探；消息流保留本地 transcript；重连后徽章经 `/agent/state` 对账恢复（4.2 小缺口兑现后）。

---

## 6. 展演裁剪 vs 答辩后全量分档表

| 项 | 12 月展演（裁剪版） | 答辩后全量 | 依据/说明 |
|---|---|---|---|
| 最简对话窗组件（消息流+输入+事件卡+分隔条+流徽章） | **做** | 组件化重构与旧窗合流 | 红线：新场景新组件，`LLM_Chat_Controller` 8133 行零触碰 |
| 经典工程页同屏可达/一键切换 | **做** | — | 纪律一条双轨保险 |
| 反向导航 directive 链（§4） | **做** | `/agent/state` 身份披露扩展为多客户端事件总线化（按需） | reply 驱动为主 |
| launcher 权限档（结构性准入）+切换治理确认（保守版） | **做** | dirty 精确检测；退出工程回启动台（dock→start 导航，F17 现无） | |
| 案 A 单窗常驻（持久窗+dock 挂点+transport 重指） | **做**（可整体退案 B） | 旧 Ask Vit 窗退役合并（届时才允许动控制器域） | |
| 交接/恢复播报桥（315 复用+无 binding 工程摘要模板） | **做** | 全量桥接（携带摘要/账本摘录） | |
| 启动降级（agent 不 ready 引导卡+经典页零阻塞） | **做** | — | |
| 门厅三问全量（晨报 90s/账本/待决清单/交班—回岗循环） | 不做 | **全量** | 讨论 §4.2；展演简版开场白≠门厅全量 |
| onboarding 三段式（313 卡 5/6/7） | 不做 | **迁入本启动面轨道** | 裁定关联修订；12 月人格面=persona_slot 底版+first-run 既有机制 |
| Session HUD（313 卡 7） | 不做 | 迁入本轨道 | |
| 聊天面组件化重构（LLM_Chat_Controller 拆分） | **禁**（红线） | 全量 | |
| 跨工程会话持久化/卷缝合回放 | 不做（启动卷会话内存，如实声明） | 全量 | 裁定后置清单 |
| 开放录音棚群聊面 | 不做 | 全量 | 与本窗关系一句话：**本窗=主对话面（单 agent 主线），群聊面=多 agent 场的扩展形态**，共用观察/授权内核，不共用窗口 |
| 语音对讲/交班预算 | 不做 | 全量 | 讨论 §4 |

**展演边界一句话**：新增面=一个新窗组件+一条 directive 链+一个持久层挂载；其余全部是既有链复用（open 链/authority/315 播报/自举）。旧控制器一行不动。

---

## 7. 实施拆卡清单（6 卡，展演版合计 7.5-9.5 人日中值 8.75）

> 验收通则：零栈单测先行；真栈腿按 AGENTS §5 三件套模式（真实栈+脚本扩展+exit 0）；Godot 侧无单测体系→装配面断言（生产场景树节点在场+数据源实接）+渲染面边界显式声明（E2E-WEBUI-1 建设前口径）；涉 LLM 的真栈腿按 §8 跑前口径成文。

### Godot 域（PC，执行流甲）

**C1 LAUNCHER-CHAT-FE-1（最简对话窗+启动台挂载）** 2 天
- 目标：新组件 `app/startup/`（launcher_chat_panel.gd+.tscn：消息流/输入/agent 事件卡/分隔条/流徽章/未就绪禁用态/降级引导卡 §5.3）+start_page 挂载（视觉主位）+磁带流一键收展。
- 文件域：前端仓 `app/startup/`（新文件）+`start_page.gd`（挂载+就绪态接线 ~30 行插点）；**不触** `LLM_Chat_Controller.gd`/`LLM_Chat_Window.tscn`。
- 验收：装配面断言（窗节点在场+agent 就绪前输入禁用/就绪后解禁）+headless 冒测（`godot --headless --script` 探针：节点树+信号接线）+渲染面边界声明；真栈腿并入 C6。
- 依赖：无。可与 C4 并行（契约 JSON 先在两卡面冻结）。

**C2 LAUNCHER-NAV-FE-1（directive 消费+open 链+治理确认）** 1.5 天
- 目标：窗消费 `launcher_directive`（§4.1 schema）→ 复用 F2/F3 链（含 `.vit` 加密/采样率跟随/lifecycle 播报）→ 治理确认对话框（§3.3 保守版）→ 结果卡点选开工程（确定性退路）。
- 文件域：前端仓 `app/startup/`（C1 组件扩展+确认对话框新文件）；`start_page.gd` open/new 链抽公共函数供窗调用（~20 行重构，行为不变）。
- 验收：单测级=契约解析表驱动（headless 探针）；真栈腿并入 C6（directive→进 dock 全链）。
- 依赖：C1；契约面与 C4 冻结一致。

**C3 LAUNCHER-PERSIST-FE-1（单窗常驻：持久层+dock 挂点+案 B 降级开关）** 1.5-2 天
- 目标：窗节点持久化（autoload 单例持有或切场景 reparent，实现卡定）+dock root 挂点（侧挂布局位）+transport 聊天钮重指常驻窗+**降级开关**（开关关=窗随启动页销毁=案 B 形态，供 freeze 前应急回退）。
- 文件域：前端仓 `app/startup/`+autoload 新文件+`vit_dock/scenes/vit_dock_root.gd`（挂点+钮重指 ~40 行插点，不动其聊天窗逻辑语义）。
- 验收：装配面断言（切场景后窗在场+消息保留+徽章态 C 正确）+切换时序腿（切场景竞态回归：窗 reparent 与 change_scene 顺序）；真栈腿并入 C6。
- 依赖：C1+C2。**此卡=案 A/案 B 分水岭，可最后决断砍留。**

### Agent 域（PC，执行流乙，与 Godot 域并行）

**C4 LAUNCHER-CHAT-BE-1（launcher 流：意图路由+directive 回执+结果卡+开场白）** 1.5 天
- 目标：chat 请求 `context.launcher=true` 分支——工程意图识别→`project.recent` 匹配→唯一命中出 `launcher_directive`/多命中出结果卡/无命中说明；门厅简版开场白（在场+最近工程+从哪开始）；`/agent/state` 增 `current_project` 身份两字段（§4.2）；无工程态 chat 冒测加固（F11 语义回归）。
- 文件域：`agent/internal/chat/launcher_flow.go`（新 ~200 行）+`server.go`（回执字段+state 披露 ~30 行插点）+测试；**不触** authority 代码（§3.1）。
- 验收：零栈单测（意图→directive/结果卡/追问三分支表驱动；无工程态回归；`launcher:true` 不改 authority_mode 断言）；真栈腿并入 C6。
- 依赖：无（契约 JSON 与 C1/C2 冻结）。
- §8 口径：真栈腿含 LLM 轮——跑前成文（次数/预算/失败分类/止损），确定性退路（结果卡）不依赖 LLM。

**C5 LAUNCHER-ROUTER-BE-1（流标记+桥接播报）** 1 天
- 目标：回执 `context_flow`（launcher/project）+卷切换 typed_event（窗据此插分隔条）；无 binding 工程的进入摘要模板（确定性，零 LLM：工程名+最近修改时间）；有 binding 工程由 315 既有开场/恢复播报承接（验证衔接，不重写）。
- 文件域：`agent/internal/chat/launcher_flow.go`（扩展 ~120 行）+测试。
- 验收：零栈单测（流标记三态+衔接播报分支）；真栈腿并入 C6。
- 依赖：C4。

### 收口

**C6 LAUNCHER-SMOKE-1（真栈烟测腿）** 1 天
- 目标：现有冒测脚本体系加参数（如 `dev_agent_smoke.ps1 -LauncherFlow` 或同模式新脚本段）：三件套拉起→启动台白话「打开 X」→directive→治理确认（有在开工程时）→进 dock→常驻窗在场+分隔条+桥接播报→对话继续一轮（工程流标记）→exit 0；降级腿：不启 agent→经典页 open 可用+引导卡在卷。§8 跑前口径成文（LLM 轮次数/失败分类/止损线）。
- 文件域：`scripts/`（脚本扩展+工件目录规范）；栈登记按 §2.2。
- 验收：脚本 exit 0+工件（report.json+日志）回指。
- 依赖：C1-C5。

**依赖序/并行边界**：C4‖C1（契约冻结后双流并行）→ C2（接 C1）‖C5（接 C4）→ C3（接 C1+C2）→ C6 收口。关键路径=C1→C2→C3→C6≈6-7 天；双执行流并行后日历≈4-5 天。Godot 仓文件域与 agent 域零相交（并行裁定条件满足）。

---

## 8. 风险表+降级路径

| # | 风险 | 概率/影响 | 降级路径 |
|---|---|---|---|
| 1 | **反向导航首例**：白话意图→directive 的 LLM 判定不稳（错开/漏开工程） | 中/高（惊艳点即成败点） | 确定性退路三层：结果卡点选（零 LLM）→磁带流原路径（零改动）→治理确认拦错开。跑前口径：单 run 失败分类记录（intent_unresolved/mis_route/UI 断链），同断点两连败禁原样重跑（AGENTS §8） |
| 2 | **单窗常驻范围蔓延**（消息缝合回放/退出回启动台/onboarding 迁入挤进展演） | 高/中 | §6 分档表锁边界；C3 整卡可砍=退案 B 不动其他件；蔓延项全列答辩后 |
| 3 | **feature-freeze 11 月底时间风险**（8.75 人日 vs 开工时点） | 中/高 | 砍序预定义：C3（退 B）→C5（流标记降为徽章静态）→C4 开场白简化为模板；最薄展演=C1+C2+C4 意图最小版+C6≈5 天，白话开工程+播报仍在 |
| 4 | 双窗并存（旧 Ask Vit+常驻窗）双客户端同会话并发发送 | 低/中 | 展演纪律：transport 钮重指后旧窗不主推；同 conversation 并发发送由 agent 会话轮串行化既有语义兜底；风险记录入 C3 卡验收说明 |
| 5 | 启动卷 transcript 重启即失被当缺陷 | 中/低 | 窗内显式「本次会话」标注+§2.3 诚实边界；答辩后卷缝合 |
| 6 | 切场景竞态（窗 reparent vs change_scene，F16/F17） | 低/中 | C3 切换时序腿为硬验收；异常时窗重建=案 B 形态自愈（徽章经 /agent/state 对账） |

---

## 9. 产品差异叙事（论文+传播用，一段）

传统 DAW 的第一面是一张空桌：工程列表、轨道视图、插件架，AI 以插件或侧边助手的形态事后挂载——人先搭建工作环境，再「叫用」智能。Vit-DAW 把这个顺序倒过来：**启动即对话**——应用打开的第一面是搭档在场的一句问候，工程不再是被浏览的对象，而是对话中被唤起的视图；授权、痕迹与判断沿对话流动（launcher 只授「列/开/建」，工程内逐级升权，跨工程自动收权），人始终是确认者。一句话：**启动即对话、工程是视图、对话是主线**——DAW 的会话史第一次从「工程文件的编辑史」变成「人与搭档的协作史」，而这正是它作为「AI 的 DAW」与传统工具的分水岭：不是给 DAW 加一个聊天窗，是把聊天窗长成 DAW 的骨架。

---

*本设计稿由 332 AI-LAUNCHER-DESIGN-1 卡产出（零栈零 LLM，两仓只读）；所有裁定引用以原文为准，本稿不自创裁定。裁定点（案 A/B 砍留时点、展演开场白文案终审）待主管+用户拍板后按 §7 拆卡。*

---

## 附：v2 终选裁定（2026-11-09 用户确认，主管补记）

- **方向终选**：蒙德里安家族（C 稿）迭代 v2——`agent/webui/design/launcher-route-mondrian-v2.html` 为视觉规格权威；三版初稿（soundviz/panel/mondrian v1）与迭代记录在 agent/webui/design/（direction-approved-launcher.md 门文件在卷）。
- **三点裁定**：①保留现有启动页磁带 icon（TapeIconArt 生产规格逐项复刻，工程卡=磁带卡带确定性气氛渐变+带轮量）；②**态 C=现有浮动对话窗（案 B 形态）**——不建右侧嵌入格，**C3（单窗常驻）整卡移答辩后**，工程侧承接=桥接播报落现有浮动窗首条；③色板统一到现有磁带流令牌（纸 #F6F4EC/墨 #090B0E/蓝 #065BC7/浅蓝 #73B8F5/黄 #FACC1A/红 #DB1214）。
- **工期修订**：6 卡→5 卡（C1/C2/C4/C5/C6），合计约 6.5-7.5 人日，双执行流日历 3-4 天。
