# UI-CONVERGE-RECON-1 勘察报告：surface 收敛——装修深度+模块拆解+嫁接位+分阶段方案

- 卡：coord/cards/doing/2026-11-15-UI-CONVERGE-RECON-1.md（池序 368，P1）
- 依据：[surface 收敛裁定](../../decisions/2026-11-15-surface-convergence-ruling.md)（用户 2026-11-15）+ [SPEC-V5](../../../agent/webui/design/main-window-v5/SPEC-V5.md)（v5.1 权威不变）
- 基线：HEAD `ead600e6`；零栈只读勘察，未改任何生产文件
- 基线亲证：`npm test`（vitest）= 48 文件 / **584 测试全绿**（2.66s，exit 0，2026-11-15 23:07 实跑）；App.tsx `wc -l` = 11,842 行
- 勘察面：agent/webui/src/**（App.tsx/styles/v5 令牌/components/launcher/project）+ 前端仓 D:/Godot/project/vit-daw-frontend（agent_chat_window.gd/start_page.gd）+ agent/internal/chat/launcher_flow.go 只读

---

## 节 1：App.tsx 功能块盘点（11,842 行）

### 1.1 总体画像：「多重内容堆单一脚本」的精确构成

App.tsx 单文件承载 **7 个功能域 + 4 层基础设施**，其中约 **7,200 行（61%）是可整体搬出而不触碰行为的纯函数与展示组件**，真正的"神经中枢"是 MainApp 一个函数（316–1894，1,578 行）里 40 个 useState + 37 个 useRef + 17 个 useEffect 构成的状态网。

```
App.tsx 11,842 行
├─ 入口与分发            1–315    (315 行, 2.7%)
├─ MainApp 巨石          316–1894 (1,578 行, 13.3%) ← 状态网核心
├─ 设置面 SettingsPage 族 1916–2325 (410 行, 3.5%)
├─ 顶栏 TopStatusBar     2326–2436 (110 行, 0.9%)
├─ DAW 状态解析层         2437–3033 (596 行, 5.0%) ← 纯函数
├─ 通用小件              3034–3101 (68 行, 0.6%)
├─ 对话流 MessageStream  3102–3430 (328 行, 2.8%)
├─ 动作卡体系            3431–6905 (3,475 行, 29.4%) ← 最大域
├─ Composer              6906–7111 (205 行, 1.7%)
├─ 工作台右栏            7112–7348 (236 行, 2.0%)
├─ 资料库 MediaPane      7349–7883 (534 行, 4.5%)
├─ BrowserPane+桥        7884–8566 (682 行, 5.8%)
├─ 宏域 Macro 族         8567–9323 (756 行, 6.4%)
├─ 历史树 History 族     9324–10055 (731 行, 6.2%)
├─ 诊断/空态/上下文卡    10056–10205 (150 行, 1.3%)
├─ 响应解析+会话存储层    10206–11212 (1,007 行, 8.5%) ← 纯函数
└─ 设置config/上下文/工具 11213–11842 (630 行, 5.3%) ← 纯函数
```

### 1.2 逐块明细（行号区间 + 耦合度）

耦合度分级：**低**=纯函数/自持组件（搬出零行为风险）；**中**=经收敛 props 接口（ActionCardProps 等）与 MainApp 相连；**高**=直接读写 MainApp 状态网。

| # | 块 | 行号区间 | 行数 | 耦合度 | 内容与耦合说明 |
|---|---|---|---|---|---|
| A | 入口与分发 | 1–315 | 315 | **低** | imports+常量（1–201）、类型（202–250）、multimodalRouteMeta（251–290）、**App() surface 分发入口（291–314：useState+subscribeEntrySurface+三分支）**。SHELL 系契约挂点，装修不动 |
| B | MainApp 巨石 | 316–1894 | 1,578 | **高（本体）** | 见 1.3 细拆 |
| C | surface/URL 工具 | 1896–1915 | 20 | 低 | appSurface()/initialConversationID()/conversationIDFromURL()；URL 参数语义（浏览器 dev 保留） |
| D | 设置面 | 1916–2325 | 410 | 中 | SettingsPage（1916–2124，208 行，props=runtimeStatus/uiState/onClose）+SettingsTextField/CheckboxField/RouteEditor/AuthorityToggle（2246–2325）。经 authoritySwitchNotice 与 MainApp 弱连 |
| E | TopStatusBar | 2326–2436 | 110 | 中 | props 8 个（connection/health/runtimeStatus/uiState/onOpenHistory/onRefresh/onTransportCommand/transportBusy） |
| F | DAW 状态解析层 | 2437–3033 | 596 | **低** | DawTrack/DawClip/DawPlugin 类型 + dawTracksFromUIState/dawClipFromRecord/actionDawTarget/clipActivitySegments 等 30 个纯函数（uiState JSON→DAW 模型）。有独立测试锚（mom/tom 相关 test 消费面在 Go 侧；ts 侧经 ActionCards 使用） |
| G | 通用小件 | 3034–3101 | 68 | 低 | StatusCell/runtimeDiagnosticLine/runtimeDiagnosticTitle/ModeSwitch |
| H | MessageStream | 3102–3430 | 328 | 中 | 272 行组件，props 24 个（messages/activities/trajectory/turnEventMeta/roundSteps/receipts/audition×3/authorityMode/respondingActionID/isSending/hiddenActionID/bottomInset/onInteractionAction/onInvoke/onSelectArtifact/uiState/onMacro×2/onSendSupplement）。内嵌 audition 卡/轨迹/台账渲染 |
| I | 动作卡体系 | 3431–6905 | 3,475 | **中（接口收敛）** | ActionCards 分发（3431–3562）→ 四种卡：CapabilityProposalCard（3631–3811）、MixTreatmentPendingCard（3872–3941）、MixBoardActionCard（3942–4169）+mixBoard* 标签群 25 函数（4170–4681）、ProjectResultCard+projectResult* 群 30 函数（4682–5127）、StandardActionCard（5128–5225）；interaction 消息归并（5226–5357）；agentEvent 解析（5358–5475）；dismissal/debugConfirmation/summarize 群（5476–5753）；InteractionPager（5754–5849）；标签函数（5850–5945）；confirmation* 确认函数群 25 个（5946–6736）；ActionDetails/InteractionFieldControl（6737–6905）。统一接口 ActionCardProps（3484–3499：action/respondingActionID/onInteractionAction/onInvoke/uiState）——**接口面已收敛，整域可搬** |
| J | Composer | 6906–7111 | 205 | 中 | props 25 个（input/pendingArtifacts/pendingMacroControls/mode/agentTurnRunning/stopTurnBusy/isSending/isUploading/interactionAction/respondingActionID/authorityMode×2/on* 10 个） |
| K | 工作台右栏 | 7112–7348 | 236 | 中 | PanelResizeHandle（7112）、CollapsedWorkbench/WorkbenchTabs/Workbench（7149–7261）、ProjectPane/TracksPane/TrackRow/PluginsPane（7262–7348）。Workbench props 18 个 |
| L | 资料库 | 7349–7883 | 534 | 中 | MediaPane（7349–7776，**427 行单组件**：artifact 网格/上传/右键菜单/重命名删除）+ArtifactPreview（7777–7883） |
| M | BrowserPane+桥 | 7884–8566 | 682 | 低-中 | BrowserPane（7884–8135，251 行）+BrowserWebViewBridge 群（8136–8566，430 行纯函数：postAgentMutations/appendAgentMutationsFromExecution 等）。桥自持 |
| N | 宏域 | 8567–9323 | 756 | 低-中 | MacroPane+MacroModeStrip/MacroControlPanelCard/MacroActionCard/MacroSlider/MacroContextCard（8624–8871）+归一化/补丁函数群 40 个（8872–9323）。normalizeMacroControl/patch* 是纯函数 |
| O | 历史树 | 9324–10055 | 731 | 中 | HistoryPane（9383–9736，**353 行**：SVG 树画布+节点菜单+分支/worktree 行）+historyTree*/historyWorktree* 纯函数群 15 个（9324–9382、9737–10055）。树布局算法（historyTreeLayout）纯函数可独立测 |
| P | 诊断/空态/上下文卡 | 10056–10205 | 150 | 低 | InfoGrid/KeyValue/buildDiagnosticsRows/buildDiagnosticsPayload/EmptyState/ArtifactContextCard |
| Q | 响应解析层 | 10206–10560 | 354 | **低** | assistantMessageFromResponse/confirmationFallbackActionsFromResponse/fullAccessDirectExecutionTargets/invokeMessageFromResponse/mixBoardActionFromPayload/interactionActionsFrom/artifactSummariesFrom——纯函数，有独立导出（测试直接 import） |
| R | 会话存储/身份/消息归并 | 10561–11212 | 651 | **低** | resolveHistorySyncMessages/migrateStoredConversationScope/resolveScopedConversationID/load|saveStoredConversationMessages/sanitizeChatMessageForStorage（10561–10852）+noteSessionSeedMessages/historyMessagesFromUIState/mergeChatMessages/upsertPersistentMixBoardMessage/mergeMessageActions/stableActionIdentity（10853–11212）。**全部纯函数+localStorage 键控，有专测（messageLifecycle/historyScope/chainResultDelivery）** |
| S | settings config/上下文构建 | 11213–11481 | 268 | 低 | defaultSettingsConfig/mergeSettingsConfig/retainSecretInputs+buildChatContext/compactChatContextRecord/uploadMetadata/mediaScopeKey——纯函数 |
| T | artifact 展示工具 | 11482–11643 | 161 | 低 | renderArtifactPreview/artifactPresentation/artifactIcon/artifactLabel/artifactAddress 群 |
| U | 通用工具 | 11644–11842 | 198 | **低** | asRecord/textValue/compactJSON/parseJSONRecord/stableTextHash/clampNumber/formatBytes/uniqueID/conversationTitle/panelWidth 存取——全域被依赖，最后搬 |

### 1.3 MainApp 内部细拆（316–1894）

| 区段 | 行号 | 内容 | 耦合点 |
|---|---|---|---|
| 声明区 | 316–397 | **40 useState + 17 useRef**（连接/健康/uiState/messages/activities/trajectory/turnEventMeta/roundSteps/taskTrajectory/audition×2/conversationID/mainConversationID/mode/authority×2/stopTurnBusy/settingsOpen/input/pendingArtifacts/pendingMacroRefs/activeTab/rightPanel×2/selectedArtifact/isSending/isUploading/respondingActionID/dismissedInteractionIDs/messageBottomInset/agentEventPolling/turnReceipts/transportBusy/error + 17 ref：runtimeOwnedErrorRef/fileInput/composerDock/notice/workspace/historyScope 族 6 个/turnReceipts 族 2 个/mediaScope/agentEventSeq/macroValueOverrides） | 状态网本体 |
| 基础回调 | 394–445 | applyTurnReceipts + refreshState（8s 轮询源） | 被 12+ effect/handler 消费 |
| 轮询与 scope | 446–655 | 初次 refresh+8s 轮询（446–458）、authority 恢复（459–481）、消息持久化双 effect（482–535）、**agent 事件轮询（GUI-F5 忙态空拍门+250/500ms 定频+链活重拉，536–655）**、mediaScope 清理 | agentEventPolling 状态机 |
| scope 演进 | 656–852 | CONTRACT-3 会话内 scope 演进≠工程切换（classifyHistoryScopeChange）+消息恢复 | historyScope 族 6 ref |
| 台账/布局 | 853–1014 | turnReceipts 恢复+TRAJ-IMPL-3 终态收口写、measureBottomInset（PLANBAR-1 停靠列）、FIX-CONFIRM-CARD-1 投影自动应答、cross-window storage 事件 | 5 effect |
| **handleSend** | 1015–1347 | **333 行巨型函数**：发送链全部（乐观消息→chat 请求→回执分流→确认 fallback→fullAccess 直执→消息归并→上传） | 触碰 20+ state |
| invokeDawAction | 1348–1396 | DAW 工具直调（useCallback） | uiState/messages |
| handleInteractionAction | 1397–1586 | **190 行**：交互卡应答链（字段收集→payload→消息移除→确认闸→dismissal） | 触碰 12+ state |
| audition 三 handler | 1587–1640 | select/stop/judgment | auditionState×2 |
| 面板管理 | 1641–1673 | rightPanel 尺寸（beginRightPanelResize/updateRightPanelWidth） | rightPanel×2 + localStorage |
| JSX | 1674–1894 | workbenchPanel（1675–1698）+conversationPanel（1700–1794）+workbench 分支（1795–1819）+full/main 返回（1821–1894：TopStatusBar→SessionFlowSidebar→workspace-grid（SettingsPage|conversationPanel+PanelResizeHandle+workbenchPanel）） | props 链出口 |

**Props 链深度实测**：MainApp→MessageStream 24 props、MainApp→Composer 25 props、MainApp→Workbench 18 props、MainApp→TopStatusBar 8 props、MainApp→SessionFlowSidebar 11 props——即拆分时每域组件的 props 接口是现成的收敛面，不需要重新设计。

### 1.4 回答用户之问

「多重内容被写入单一脚本里全堆在这里」成立的精确形态：**1 个 1,578 行的函数（MainApp）持有 40+37 状态单元，向 6 个方向以 18–25 个 props 的宽接口下传；另有约 7,200 行纯函数/自持组件与它同文件共居**。后者搬运即拆（零行为风险），前者需要按域抽 hooks 才能解。动作卡体系 3,475 行是最大单域，但其对外接口只有 ActionCardProps 一张（5 个字段），耦合已被早期设计压住。

---

## 节 2：样式体系深度

### 2.1 旧体系（MainApp 面）的存在形态

| 形态 | 计量 | 说明 |
|---|---|---|
| 全局 css 文件 | **styles.css 5,084 行**（main.tsx 全局导入） | 唯一大样式表；29 个 :root 旧变量（--ink #111318/--muted/--line/--blue/--red/--yellow/--panel/--shadow 等） |
| 全局元素规则 | `*` `html,body,#root` `body` `button,input,textarea` `button:disabled` `h1,h2,h3` `body.panel-resizing *` `button:focus-visible`（styles.css:25–57,339–356,4367–4368,4819） | **作用于全页含 v5 面**——现 v5 面靠 v5- 前缀类自抵；装修时反向风险见节 7 |
| App.tsx 内联 style={{ | **仅 6 处**（3370 message-scroll-anchor 高度 / 7010 composer-interaction-shell 高度 / 7693 contextMenu 定位 / 9690 history-tree-canvas 尺寸 / 9701 树节点定位 / 9718 右键菜单定位） | 全部是布局动态值，非装修面——**装修工作量不在内联样式** |
| className 密度 | **309 处**，类名无统一前缀（icon-button/action-detail/work-section/pane-body/history-section-heading/tool-button/settings-section…抽样 top15 见勘察日志） | 全部映射进 styles.css——装修=按域把旧类替换为 v5 组件类 |
| 域专属小 css | composer/planbar.css 66 行、trajectory/trajectory.css、trace/trace.css、taskTrajectory/details.css（行数 60–150 级） | 自带前缀、按组件导入，无冲突 |

styles.css 中 launcher/v5 残留 = **0**（grep 亲证）——345/346/347 三卡迁移时未留死类，分离干净。

### 2.2 v5 令牌体系（挂载点与冲突面）

| 文件 | 行数 | 内容 | 挂载点（import 实测） |
|---|---|---|---|
| styles/tokens.css | 115 | v5.1 全量令牌，**--v5- 前缀**；哈希回指 design-tokens-v5.json（sha256 1d985d5e…，文件头注释） | launcher/LauncherSurface.tsx:89、project/ProjectSurface.tsx:51、project/loading/LoadingCard.tsx:17、gallery/main.tsx:29 |
| components/components.css | 727 | **116 个类全 v5- 前缀**（Button/MacroCard/Message/StatusCard/StatusRing/TextField/ProjectBadge/ActionCard/SnapshotReceipt/VitMark 十组件） | 同上四面 |
| design/sprite.css + sprite.tsx | 105 | 23 symbol 图标雪碧图；.v5-sprite 0 尺寸隐藏 + .v5-design .v5-icon 渲染契约 | **main.tsx 全局挂载**（361 卡：不导入会退化为裸 svg） |
| styles/launcher.css | 527 | 57 类 v5-launcher-* | LauncherSurface.tsx:91 |
| project/project.css | 244 | 27 类 v5-project-* | ProjectSurface.tsx:53 |
| project/loading/loading.css、styles/tape.css(20)、gallery/gallery.css | — | 装载门/TapeBadge/gallery 专属 | 各自组件 |

### 2.3 冲突面清单（装修工作量的决定项）

1. **变量双体系并存**：旧 --ink=#111318（styles.css）vs v5 --ui-text=#141414（tokens.css）——tokens.css 头注释明示策略：「全部加 --v5- 前缀避免冲突；**未来主 App 接入时两种前缀并存互不覆盖**」。即装修**不迁移旧变量**，逐域以 v5 组件类替换旧类，变量层零合井。
2. **全局元素规则反向污染**：styles.css 的 button/input/h1-h3/body 规则已作用于 v5 面（现靠 v5 类优先级自抵）。MainApp 逐域换皮时，同一 DOM 里有旧类（styles.css）与新 v5 类（components.css）并存——需按域删旧类时同步删 styles.css 对应段，否则死样式持续生效（无副作用但干扰对比度审计）。
3. **.v5-design 隔离契约**：sprite.css 的 .v5-design 作用域是 v5 图标渲染前提——MainApp 换皮域需在根部挂 v5-design 类才能用 `<use href="#i-*">`（gallery/LauncherSurface 已有先例）。
4. **对比度审计工具链在位**：render-smoke 的 contrastAssert（正文 ≥4.5:1/图形 ≥3:1）按面跑——装修每域都可直接挂进该扫描（无需新机制）。

### 2.4 装修工作量结论

旧面**零内联装修债**（6 处 style 全是动态布局值）；工作量集中在 **309 处 className 按 7 域逐一映射到 v5 令牌组件** + **styles.css 5,084 行按域切段退役**。v5 组件库（727 行/10 组件）已覆盖消息/宏卡/状态卡/输入框/按钮——对话流与动作卡的替换件现成；缺的替换件主要是：MediaPane 网格/HistoryPane 树/SettingsPage 表单组/BrowserPane（阶段 2 需补 3–4 个 v5 组件）。

---

## 节 3：T9 模块拆解方案（联合规划核心）

### 3.1 拆分顺序（依赖驱动三批）

**第一批：纯函数与类型出域（零 UI 变化，风险最低，先扫清）**
| 步 | 搬出内容 | 源区间 | 目标 | 回归面 |
|---|---|---|---|---|
| 1a | DAW 解析层（F） | 2437–3033 | `src/daw/parse.ts` | vitest 全量+新增 daw.parse 专测（纯函数易测） |
| 1b | 会话存储/身份/消息归并（R） | 10206–11212 | `src/chat/storage.ts`+`src/chat/merge.ts` | **messageLifecycle/historyScope/chainResultDelivery/chainResultMessage 等既有专测直接改 import 即跑** |
| 1c | 宏归一化/补丁群 | 8872–9323 | `src/macro/normalize.ts` | vitest+macro 相关单测 |
| 1d | historyTree 布局群 | 9324–9382+9737–10055 | `src/history/tree.ts` | 新增 tree 布局专测 |
| 1e | confirmation/mixBoard/projectResult 标签群 | 4170–4681、5946–6736、4682–5127 的函数部分 | `src/actions/labels.ts`+`src/actions/confirmation.ts`+`src/actions/projectResult.ts` | 新增专测 |
| 1f | 通用工具（U）+settings config（S） | 11644–11842、11213–11481 | `src/lib/`+`src/settings/config.ts` | 既有设置相关测试 |

**第二批：展示组件出域（props 接口现成）**
| 步 | 搬出 | 目标 | 回归面 |
|---|---|---|---|
| 2a | 动作卡体系全域（I，3,475 行） | `src/actions/`（ActionCards/CapabilityProposalCard/MixBoardActionCard/ProjectResultCard/StandardActionCard/ActionDetails/InteractionPager） | capabilityProposalCard/interactionCardKit/fullAccessDirectExecution 等专测改 import |
| 2b | MessageStream+AssistantTextContent（H） | `src/chat/MessageStream.tsx` | observeOutputLayered/observePresentation 专测 |
| 2c | Composer（J） | `src/chat/Composer.tsx` | composer 相关测试 |
| 2d | MediaPane+ArtifactPreview+artifact 工具（L+T） | `src/media/` | bucketSaveRace 专测 |
| 2e | Macro 族组件（N） | `src/macro/` | — |
| 2f | HistoryPane（O 组件） | `src/history/HistoryPane.tsx` | historyScope 专测 |
| 2g | BrowserPane+桥（M） | `src/browser/` | — |
| 2h | Workbench 族（K）+TopStatusBar（E）+诊断件（P）+SettingsPage 族（D） | `src/workbench/`+`src/topbar/`+`src/settings/` | topStatusBar/settingsConfigEdit/authorityToggle 专测 |

**第三批：MainApp 状态网抽 hooks（风险最高，最后做，每步一个域）**
| 步 | 抽取 | 说明 |
|---|---|---|
| 3a | `useAgentEventPolling`（536–655） | GUI-F5 忙态空拍门逻辑原样搬，不动判定 |
| 3b | `useHistoryScope`（656–852） | CONTRACT-3 六 ref 组原样搬 |
| 3c | `useTurnReceipts`（853–923+394–397） | TRAJ-IMPL-3 收口写原样搬 |
| 3d | `useAudition`（1587–1640+audition state） | — |
| 3e | `usePanelLayout`（1641–1673+measureBottomInset 924–945） | PLANBAR-1 逻辑原样搬 |
| 3f | MainApp 缩壳+App.tsx 收尾（App()+MainApp 壳 ≤400 行） | 1015–1347 handleSend、1397–1586 handleInteractionAction 拆入 `src/chat/useSendChain.ts`/`src/actions/useInteractionAction.ts`（逻辑原样搬） |

**铁律（从注释缺陷史得出）**：第三批一律「逻辑原样搬位置」，禁顺手改判定；每个 hooks 抽取是独立 commit，坏了可单独回退。

### 3.2 目标目录结构

```
agent/webui/src/
  main.tsx               # 不动（sprite/styles.css 全局导入面）
  app/                   # App.tsx（分发壳 291–314 原样）+ MainApp 壳 + appSurface.ts
  chat/                  # MessageStream/Composer/storage/merge/agentEvent 解析/useSendChain
  actions/               # 动作卡全家 + labels/confirmation/projectResult + useInteractionAction
  daw/                   # parse.ts + ProjectPane/TracksPane/PluginsPane（2h 并入）
  media/                 # MediaPane/ArtifactPreview/artifact 工具
  macro/                 # MacroPane 全家 + normalize
  history/               # HistoryPane + tree.ts
  browser/               # BrowserPane + bridge
  settings/              # SettingsPage 族 + config.ts
  workbench/             # WorkbenchTabs/Workbench/PanelResizeHandle
  topbar/                # TopStatusBar + 诊断件
  lib/                   # 通用工具
  launcher/ project/ components/ design/ styles/ gallery/ trajectory/ trace/ taskTrajectory/ composer/  # 现状不动
```

### 3.3 每步可回归面（基线亲证）

- **vitest**：48 文件 / 584 测试（本次领取实跑全绿 2.66s）——每步必跑，任一域搬运后数字不得变（只允许 import 路径变更）。
- **tsc**：`npm run build`（tsc --noEmit && vite build）——类型面回归。
- **test:render**：`node src/gallery/render-smoke.mjs`——**现役 8 段**（gallery 基线 / launcher fixture / project fixture / loading fixture / fidelity / appentry 真实入口 / shadowbox / rec-tape；卡面写「六段」为 361 卡时点，362/365 两卡已扩展至 8 段，本卡如实记录）。appentry 段断言真实入口 sprite 挂载——拆分后 main.tsx 导入面不变即天然守护。
- 每批收尾加一次 `npm run build`+test:render 全跑；阶段 1 出口加 ui_entry 烟测（节 6）。

### 3.4 独立验收性论证

- 第一批/第二批每步产出物是**纯移动+import 重写**，vitest 584 数字不变即证明行为零变化——验收命令可完全机械判定。
- 第三批每步（单 hooks）行为面被既有测试覆盖：GUI-F5 门（sessionFlow/事件轮询测试）、TRAJ-IMPL-3 收口（messageLifecycle）、CONTRACT-3（historyScope）、PLANBAR-1（test:render 裁切扫描）——**每个精调逻辑都有对位测试**，这是三年缺陷修复史留下的护栏。
- 拆分与装修解耦的交接面：第二批完成后即可开始阶段 1 壳装修（不必等第三批）；第三批可与阶段 2 并行（文件域不相交：第三批动 src/app/+hooks，阶段 2 动各域目录）。

---

## 节 4：工程管理层嫁接位

### 4.1 桥协议保留清单（SHELL 系契约零破坏逐项核对）

| 契约 | 定义处 | 消费点（App.tsx 行号） | 核对结论 |
|---|---|---|---|
| `vit_surface`（宿主→React surface 切换） | launcher/entrySurfaceBridge.ts:24 `ENTRY_SURFACE_MESSAGE_TYPE` | **App() 293–302 唯一消费**（subscribeEntrySurface→setSurface+postEntrySurfaceAck） | 保留：App() 分发逻辑装修不动；343 不变量（零导航零重载）由该点守护 |
| `vit_surface_ack`（React 取证回执） | entrySurfaceBridge.ts:25–26 | App() 300 | 保留：烟测断言面（agent_chat_window.gd:571–583 react_ready 留痕），回执时序不得改 |
| `ask_vit_launcher` 通道（React→宿主） | launcherBridge.ts:31 `LAUNCHER_BRIDGE_CHANNEL` | LauncherSurface 经 postLauncherDirective/postLauncherChooseFile | 保留：桥函数自持（launcherBridge.ts 212 行零 App.tsx 依赖），平移进新壳即可 |
| `vit_launcher_result`（宿主→React 终态） | launcherBridge.ts:35 | LauncherSurface useEffect（message 监听） | 保留：honest 终态纪律（opened 只能来自宿主回执）随组件平移 |
| `vit_governance_required`（页内治理三键确认） | launcherBridge.ts:40 | LauncherSurface governance 卡 | 保留：367 卡正在修的不变式面，新壳确认卡沿用同载荷 |
| 请求 type 面 5 个 | launcherBridge.ts:47–53（launcher_surface_ready/launcher_directive/launcher_choose_file/launcher_result_ack/launcher_governance_choice） | LauncherSurface 全部 | 保留：零新增零改名 |
| `launcher_result_ack` 烟测通道（URL launcher_smoke=1 门控） | launcherBridge.ts + LauncherSurface parseSmokeParams:158 | 烟测取证 | 保留 |
| agent 端点 2 个 | api/launcher.ts（GET /agent/launcher/greeting；POST /agent/chat context.launcher=true） | LauncherSurface loadGreeting/loadRecents | 保留：agent 侧 chat/launcher_flow.go（LAUNCHER-GATE-FIX-1 门判定+SHELL-5 directive+requires_governance 恒 true）本卡不动、装修不动 |
| 磁带头像场用位 | SPEC-V5 §7.3.1 v5.1.1 表 + components/ProjectBadge.tsx（tape.css 20 行） | LauncherSurface recents 行/结果卡 | 保留：新壳 recents 行直接复用 TapeBadge；366 卡设计落位=此组件+launcher.css 磁带卡段 |

### 4.2 嫁接锚点（App.tsx 行号级）

1. **surface 分发壳（装修不动点）**：App() **291–314**。五态白名单 appSurfaces（283–289）已含 launcher/project——收敛方案=**303–305 的 `<LauncherSurface fill/>` 替换为新壳组件**（暂名 ConvergedLauncher），App() 其余零改动；vit_surface=project 时切 ProjectSurface 的既有行为保留（307–311，含 ProjectLoadingGate 装载门）。
2. **MainApp 对话面复用锚点**：MainApp 巨石（316 起）暂按现状服务 full/main/workbench 三 URL 面；阶段 1 新壳的对话流**复用 MessageStream/Composer 组件**（第二批拆出后 import 即用），不复用 MainApp 实例——新旧两套对话流在收敛切换瞬间完成交接（见节 7 双轨）。
3. **recents 数据流锚点**：LauncherSurface **loadRecents（356–394）**（sendLauncherChat(conversationID,"有哪些工程")→cardsToEntries→drawerEntries/empty/error 三态）、**loadGreeting（396–418）**、live 加载 effect（401–409）——三个函数+effect 原样平移进新壳（自持，无 App.tsx 级耦合）。
4. **新建/打开锚点**：directNewProject（337–347，SHELL-5 直连 directive：buildDirectNewDirective→executeDirective）、executeDirective（300–323：桥可用性检查→postLauncherDirective→waiting 态）、chooseFileViaBridge（325–335）、retryLastDirective（349–352）——四函数平移。
5. **治理确认锚点**：LauncherSurface governance 渲染段（728–760 级）+ vit_launcher_result 监听 effect（479–494 级）——平移；367 卡的确认卡全 surface 渲染修复按其卡面文件域继续。
6. **「回到启动页」交互（新交互面，零新桥协议）**：工程内返回 = setSurface("launcher")（React 内部 setState，零导航）；需要双向时宿主已有 vit_surface 通道（agent_chat_window.gd:295 _pending_surface="project"）——反向（project→launcher）目前宿主无发送点，**React 内部切换即可先行**，宿主侧展开/沉后语义另行对齐（见节 6 卡 1D 边界注记）。
7. **装载门复用**：ProjectLoadingGate（project/loading/ProjectLoadingGate.tsx，47 行组件+loading.css）——新壳若需装载态直接复用。
8. **顶栏工程态**：TopStatusBar（2326–2436）已消费 uiState（工程名/路径）——新壳标题栏工程名语义与 SPEC-V5 §3（无工程="Vit"/有工程=工程名）对齐时，此组件是现成数据源。

---

## 节 5：旧启动入口收编清单

### 5.1 工程内旧 webui 对话框全部启动路径枚举（实测 grep）

| # | 入口 | 构造点 | 产物 surface | 收编状态 |
|---|---|---|---|---|
| 1 | Godot autoload boot（生产主链） | agent_chat_window.gd:432–441 `_navigate_page()`→`_AGENT_WEBUI_URL_LAUNCHER`（:54 = `http://127.0.0.1:7878/app/?surface=launcher`），navigate_count 门=只导航一次 | launcher | **已收编**（UI-ENTRY-1：启动态唯一门面=React 启动台） |
| 2 | **load_error 重试导航** | agent_chat_window.gd:444–449 `_retry_page_navigation()`→`_AGENT_WEBUI_URL`（:51 裸 URL） | **full（旧巨石面）** | **残留——收编点 A**：重试应改导航 launcher 参数（同一 URL 前缀判定 :547 不破坏） |
| 3 | 进工程展开 | agent_chat_window.gd:295–320：宿主 post_web_message `vit_surface=project`（_pending_surface 兜底 fallback_timer:318–320 + react_ready:582–583） | project | 已收编（零导航切面） |
| 4 | 关窗语义 | agent_chat_window.gd:490–508 `_teardown_after_close`：autoload 关 React 窗=全栈退出（UI-ENTRY-2 修订） | — | 已裁定，无关 |
| 5 | legacy 模式（VIT_AGENT_WINDOW_MODE 缺省） | agent_chat_window.gd:7/14：mode off 行为零变化——**Godot 原生启动大屏 start_page.gd 为主窗**（新建/打开工程/引擎设置，经 ProjectClient/ProjectStore；其自身也消费 launcher 端点：start_page.gd:36–40 _AGENT_CHAT_URL/_LAUNCHER_GREETING_URL + LAUNCHER-NAV-FE-1 卡 336） | 不开 webui | **收编点 B**：用户裁定 4「旧对话框不再独立启动，统一经新窗」的完全体须评估 Godot start_page 退役时点——文件域在 Godot 仓，建议阶段 1 验收后另卡（前端仓不在本卡文件域） |
| 6 | launcher smoke 导航 | start_page.gd:613–656（gallery.html?surface=launcher&launcher_smoke=1） | gallery fixture | 测试专用，保留 |
| 7 | 浏览器 dev 直开 | `npm run dev`→任意 ?surface=*；appSurface()（1896–1903）URL 参数语义 | 任意 | 保留（开发通道，与勘察卡面「URL 初始 surface 语义保留」一致） |
| 8 | 烟测脚本 5 个 | scripts/ui_entry_smoke.ps1 / ui_entry2_smoke.ps1 / vitshell_smoke.ps1 / vitshell_full_smoke.ps1 / vitshell_open_chain_smoke.ps1（grep surface= 命中） | 各测各面 | 保留：阶段 1 落地后 vitshell_full_smoke 指向需复核对齐 |
| 9 | VitApp 内核侧 | grep VitApp 7878/app = **0 命中** | — | 内核不开 webui，无关 |

### 5.2 关闭/重定向方案

- **收编点 A（Godot 仓 1 行级改动）**：`_retry_page_navigation` 的 `_AGENT_WEBUI_URL` 改 `_AGENT_WEBUI_URL_LAUNCHER`（page_loaded 前缀判定 :547 用的是 `_AGENT_WEBUI_URL`，query 不破坏 begins_with，注释 :53 已声明）——随阶段 1 收尾卡进 Godot 侧微改卡（前端仓改动须单独发卡，本卡只登记）。
- **收编点 B**：Godot 原生 start_page 退役=用户裁定 4 的终态；建议以「阶段 1 新壳过 ui_entry 烟测+用户手测通过」为启动条件，退役前 start_page 与新窗并存（autoload 模式生产链已不经过它）。
- **重定向不变式**：所有路径最终汇于 App() 单一分发点（291–314）——收编=让入口 URL/消息面收敛到 launcher+project 两态，App() 代码零分支增加。

---

## 节 6：分阶段实施卡草案

> 拆分依据：节 3 三批顺序 + 节 4 锚点。每卡一个目标、固定文件域、机械可判定验收。天数按免费车道单卡 ≤5 文件/≤1h 缰绳折算为「执行会话次数」近似。

### 阶段 1：壳装修+工程管理挂载（「成为新启动页」时刻，含 366 磁带卡落位）

| 卡 | 目标 | 文件域 | 验收标准 | 预估 |
|---|---|---|---|---|
| **T9-PRE-1a** | 第一批纯函数出域（daw/macro normalize/history tree/chat storage+merge/lib/settings config；§3.1 1a–1f 六步可并一卡或拆两卡） | App.tsx+新建 src/{daw,macro,history,chat,lib,settings}/（仅移动+re-export 兼容层，App.tsx 保持可编译） | `npm test`=584 全绿（数字不变）；`npm run build` 过；test:render 8 段全过 | 1–1.5 天 |
| **T9-PRE-1b** | 第二批组件出域（actions 全域/chat 两件/media/macro 组件/history/browser/workbench/topbar/settings；§3.1 2a–2h） | 同上延伸 | 同上+capabilityProposalCard/interactionCardKit 等专测 import 改写后全绿 | 1 天 |
| **CONVERGE-SHELL-1** | 新壳组件+壳装修：LauncherSurface 桥面/数据流平移（节 4.2 锚 3–5）+对话流复用（MessageStream/Composer import）+v5 令牌接入（tokens/components/sprite+v5-design 根类）+磁带卡落位（TapeBadge recents 行，366 设计） | 新建 src/launcher/ConvergedLauncher.tsx+launcher/converged.css；App.tsx 303–305 替换 | test:render 新增 converged fixture 段（DOM 断言：recents/新建/打开/建议/对话流/治理卡+v5 令牌生效断言）；vitest 584 不减；ui_entry_smoke.ps1 exit 0 | 2–3 天 |
| **CONVERGE-ENTRY-1** | 工程管理层挂载+进出语义：recents 抽屉/新建/打开进新壳（若 CONVERGE-SHELL-1 未含全量则此卡收尾）+「回到启动页」交互（setSurface 内部切换）+宿主 vit_surface 双向对齐登记 | src/launcher/ + App.tsx（App() 不改分发逻辑，仅必要时白名单）；Godot 侧收编点 A 另微改卡 | 桥协议五 type 零破坏断言（launcherBridge.test.ts 平移不删）；ui_entry2_smoke.ps1 exit 0；vit_surface→project→launcher 往返烟测 | 1–2 天 |
| **CONVERGE-CLEANUP-1** | 旧 LauncherSurface 退役+fixture 烟测迁移（launcher fixture 段改指新壳）+styles.css 死段清理启动 | launcher/LauncherSurface.tsx 归档、render-smoke.mjs 段迁移 | test:render 8 段（launcher 段改断新壳）全过；CONVERGE-SHELL 旧组件零引用 | 0.5–1 天 |

**阶段 1 合计：5.5–8.5 天（含缓冲）。** 出口判据=用户从 Godot 拉起→新启动页可完成 recents/新建/打开/进工程全旅程（手测入口按 AGENTS §5：用户从 Godot 前端拉起）。

### 阶段 2：工具页逐块 v5 化（每卡独立发卡、独立验收）

| 卡 | 域 | 文件域 | 验收 | 预估 |
|---|---|---|---|---|
| CONVERGE-UI-CHAT-1 | 对话流（MessageStream/Composer）v5 化 | src/chat/ + styles.css 对应段退役 | test:render 对话面新断言（消息阴影框 362 裁定/字阶/对比度）+584 | 1–2 天 |
| CONVERGE-UI-ACTIONS-1 | 动作卡体系 v5 化（CapabilityProposal/MixBoard/ProjectResult/Standard 四卡） | src/actions/ | 四卡全状态 DOM 断言+确认卡 367 不变式不回退 | 2 天 |
| CONVERGE-UI-WORKBENCH-1 | 右栏三 Pane（Media/History/Macro）v5 化 | src/{media,history,macro,workbench}/ | test:render 右栏断言+裁切/对比度扫描 | 1–2 天 |
| CONVERGE-UI-SETTINGS-1 | 设置面 v5 化 | src/settings/ | settingsConfigEdit 专测+渲染断言 | 1 天 |
| CONVERGE-UI-TOPBAR-1 | 顶栏+诊断 v5 化 | src/topbar/ | 渲染断言 | 0.5–1 天 |
| CONVERGE-UI-BROWSER-1 | BrowserPane v5 化（可延后/降级） | src/browser/ | 渲染断言 | 1 天 |
| （并行轨）T9-PRE-2 | 第三批 hooks 抽取（§3.1 3a–3f，与阶段 2 并行，文件域不相交） | src/app/+新 hooks 文件 | 584 不减+每 hooks 一 commit | 2–3 天 |

**阶段 2 合计：7.5–11 天（不含并行轨）或 8–11 天（并行轨抵充）。**

---

## 节 7：风险与工作量定价

### 7.1 巨石装修风险点（按 severity 排）

| # | 风险 | 证据锚 | 缓解 |
|---|---|---|---|
| 1 | **MainApp 状态网 behavior drift**：40 state+37 ref+17 effect，注释全是缺陷归因（GUI-F5 空拍门/TRAJ-IMPL-3 收口判据/CONTRACT-3 演进分离/AB-JUDGMENT-CARD-1 错误保护/PLANBAR-1 高度预留）——精调逻辑被"顺手清理"即回归 | App.tsx 316–1894 注释带；E2E J1 实测案例（:364–367） | 第三批 hooks 一律原样搬；每 hooks 独立 commit；584 基线数字不变验收 |
| 2 | **双对话流并存期语义漂移**：新壳（launcher 卷确定性路由）与 MainApp（LLM 卷）是两套消息格式（v5-launcher-msg vs message/动作卡体系）——收敛切换瞬间若 MainApp 面仍有入口，用户可能回到旧对话卷 | 节 1 H/I 域；launcher_flow.go 门判定（context.launcher 主门） | 阶段 1 出口即收编收编点 A+退役旧 launcher；full URL 面保留但引导文案统一 |
| 3 | **styles.css 全局规则反向污染**：button/input/h1-h3/body 级规则作用于新壳；新壳删旧类时若 styles.css 段未同步退役，对比度审计被死样式干扰 | styles.css:25–57/339–356/4819；v5 面现状靠优先级自抵 | 新壳根部 .v5-design 隔离（先例在 gallery）；每域换皮同 PR 删 styles.css 对应段 |
| 4 | **vit_surface 时序**：宿主 post 可能早于 React 挂载（WebView2 冷加载），已有 _pending_surface 双兜底（fallback_timer+react_ready）——新壳不得改 App() 订阅时序，否则兜底链断 | agent_chat_window.gd:289–320、582–583 | CONVERGE 系卡文件域显式排除 App() 293–302 |
| 5 | **test:render fixture 断言悬空**：launcher/project fixture 段断言的是现 LauncherSurface DOM（data-launcher-*）——退役后断言对象消失，烟测绿但守护为零 | render-smoke.mjs:564–846（launcher 段）、902+（project 段） | CONVERGE-CLEANUP-1 专卡迁移断言（先迁后删） |
| 6 | **366 磁带卡双返工**（裁定已防）：磁带造型若先做在旧 LauncherSurface 再装修即双返工 | 裁定「366 暂缓随阶段 1」 | 磁带卡落位只进 CONVERGE-SHELL-1（TapeBadge 组件现成，只做布局位） |
| 7 | **Godot 仓跨界**：收编点 A/B 在 D:/Godot/project/vit-daw-frontend（仓库外文件域）——webui 卡不可顺手改 | agent_chat_window.gd:444–449 | Godot 侧微改另发卡；本卡只登记锚点 |
| 8 | **584 基线漂移误判**：拆分期间并发其他卡改测试（如 367 卡）会动基线数字 | 池序 367 候领取 | 每卡领取记录当日基线数；数字变化须归因到具体卡 |

### 7.2 双轨过渡方案与退役时点建议

```
现在 ──────────────► 阶段 1 期间 ──────────────► 阶段 1 出口 ──────► 阶段 2
旧 LauncherSurface(825行)   新壳 CONVERGED          新壳唯一门面
MainApp(full/main/workbench) ──并存──► MainApp 三 URL 面保留 ──► 逐域 v5 化
ProjectSurface(496行)        不动                     不动（367 修于其上）
```

- **LauncherSurface 退役时点**：CONVERGE-CLEANUP-1（阶段 1 最后一卡）——条件=新壳过 ui_entry/ui_entry2 烟测+converged fixture 段断言全绿。旧文件归档保留（git 历史可回指），import 面清零。
- **ProjectSurface 退役评估**：**不建议本周期退役**——367 卡正修其确认交互面（修的是不变式，换底座成立）；它在收敛后的角色=「进工程主窗」，阶段 2 可将其对话流与新壳对话流同源化（同用 chat/ 域组件）后再评估合并。裁定「启动页与对话窗为同一连续窗」的最终形态=新壳+ProjectSurface 共享 chat 域组件，而非一个组件吃下两态。
- **MainApp full 面**：退役时点=阶段 2 末（工具页全部 v5 化后）；期间保留三 URL 面作为回归对照（烟测 vitshell_full_smoke 仍指它），避免「拆了旧梯子新梯子未焊稳」。

### 7.3 总工作量定价

- 阶段 1：**5.5–8.5 天**（壳装修+挂载+收编；含 366 落位）
- 阶段 2：**7.5–11 天**（六域 v5 化；与 T9 第三批并行可压缩 2–3 天）
- T9 拆分全程：**4–5.5 天**（第一批 1–1.5+第二批 1+第三批 2–3，其中前两批已计入阶段 1 头部）
- **合计：约 13–20 个执行日**（免费车道折算：约 20–30 张单日卡）；关键路径=CONVERGE-SHELL-1（唯一需要设计裁量的一卡，其余均机械可判定）。

---

## 勘察方法与诚实边界

- 行号/计数全部来自本次实跑命令（grep/wc/sed 亲证），非文档转述；vitest 584 与 test:render 段数为领取时实跑/实读。
- MainApp 中段块边界（446–1014 区间的 effect 主题归属）按 useEffect 行号+首行内容归类，个别相邻 effect 的精确语义以注释卡号为准（GUI-F5/CONTRACT-3/TRAJ-IMPL-3/FIX-CONFIRM-CARD-1 均有卡号锚）。
- 未跑：`npm run build`（tsc）、ui_entry 烟测——只读勘察不启动构建/栈；两命令已登记为实施卡验收面。
- Godot 前端仓（D:/Godot/project/vit-daw-frontend）只读 grep/sed，未验证其运行时行为（navigate/消息链描述来自源码+在档卡注释，以 SHELL 系烟测为准）。
