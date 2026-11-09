# AI-LAUNCHER-DESIGN-1：AI-first 启动面对话窗口设计 v1（展演裁剪版+答辩后全量分档）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=[裁定](../../decisions/2026-11-09-ai-first-launcher-ruling.md)：用户定向启动即对话窗口进展演线+单窗常驻+agent 自主切换对话流）
- 派发确认：已确认（用户 2026-11-09 指令「派一个代理出去帮我们设计一下新的启动页就打开的对话窗口」）
- 验收负责人：GLM 主管决策流+用户终审（惊艳点与产品形态）
- 池序 332；P1 / 1 天 / 依赖=零；L1 / PC；**零栈零 LLM 纯设计**（只读勘察+文档产出）

## 目标（设计报告九节，产出=coord/reports/2026-11-09-ai-first-launcher-design-v1.md）

1. **信息架构**：启动=对话窗口（新 Godot 场景+新最简聊天组件：消息流+输入+agent 事件）；经典工程选择页=同屏可达/一键切换+录播 fallback；工程页打开后**对话窗口单窗常驻**（主对话面，进工程不消失）；
2. **对话流模型（本卡最高价值节——"惊艳"点）**：launcher 流与工程流的定义；**agent 根据对话内容自主路由**的语义（判定规则/切换时机/上下文携带/用户可见性——切换的 UI 呈现要轻不吵）；历史分卷与衔接；**单窗常驻 vs 交接播报降级**两案并列给建议（降级案=复用 315 恢复播报；若单窗常驻对展演过重，降级案同骨架可退）；
3. **工程生命周期命令面**：project.list/open(/create) 产品化——launcher 权限档（只授列/开/建）、治理确认（未保存切换必确认）、与既有 authority 升级链的关系；
4. **反向导航**：agent open→Godot 进工程页（IPC 事件面勘察：VIT_IPC_CONTRACT 现有事件够不够、缺什么）；
5. **启动生命周期**：app 启动拉起 agent+kernel 的接线、异步防白屏、**降级路径**（agent 起不来→经典启动页零阻塞，L1-d 案 A 失败引导同款问题此处一并设计）；
6. **展演裁剪 vs 答辩后全量分档表**（红线：展演零触碰 LLM_Chat_Controller 8133 行单体；onboarding/HUD/门厅全量/聊天面组件化全部列答辩后）；
7. **实施拆卡清单**（每卡=目标/文件域/验收[零栈单测+真栈腿]/估时/依赖序——Godot 仓与 agent 域分列，标注可双执行流并行的边界）；
8. **风险表+降级路径**（反向导航首例风险/单窗常驻范围蔓延风险/录播 feature-freeze 11 月底的时间风险）；
9. **产品差异叙事一段**（"启动即对话、工程是视图、对话是主线"——与传统 DAW 的差异，论文+传播用）。

## 必读输入

- [AI_FIRST_INTERACTION_DISCUSSION_2026-10-28](../../docs/AI_FIRST_INTERACTION_DISCUSSION_2026-10-28.md)（交班—回岗/门厅三问/在场而非常聊——**设计要吸收门厅作为窗口默认内容形态**，非否定）
- [裁定 2026-11-09-ai-first-launcher-ruling](../../decisions/2026-11-09-ai-first-launcher-ruling.md) + [生成线裁定](../../decisions/2026-11-09-gen-capability-line-ruling.md)
- [313 L1 设计报告](../reports/2026-11-08-legame-ux-design-v1.md) §2.4/§3/§5（onboarding 设计与分档——其挂载点随本设计改）
- 已合入零件实测面：314/315/316 卡回执（cards/done/：目录解析/开场+恢复播报/重铺副本制）
- Godot 仓只读勘察：D:/Godot/project/vit-daw-frontend 启动页场景/脚本结构、app/shell 布局、IPC 桥（**LLM_Chat_Controller 只读参考其事件流形态，红线零触碰**）
- agent 侧：internal/chat 会话/conversation 模型、history 包分卷语义、authority 模型（manual_confirmation→full_project_access）、[VIT_IPC_CONTRACT](../../docs/VIT_IPC_CONTRACT.md)
- 开放录音棚 v1 定稿 §3（对话面长期形态参考——群聊面与本窗口的关系声明一句即可，不展开）

## 文件域

产出=coord/reports/2026-11-09-ai-first-launcher-design-v1.md+本卡面；零代码零栈；两仓只读。

## 验收标准

九节齐全+拆卡清单可直接发卡（文件域/估时/验收完整）+分档表展演边界明确+两案（单窗常驻/交接降级）建议有依据；用户终审惊艳点。

## 停止条件

勘察发现 IPC 事件面缺口大到需内核改动才能反向导航 → 如实列为前置卡上报（不自行假设可行）。

## 领取/回执/验收

### 领取

- 2026-11-09 / origin/main `f12635af`（fetch 重读亲证，卡仍在 todo、无 owner）/ owner=ZCode GLM-5.3 L2-worker PC 设计子代理（会话 332design，池序 332）/ 分支 `port/ai-launcher-design-1` / worktree `D:/Vit_DAW_wt_332design`
- 领取基线：HEAD=`f12635af`（分支自 origin/main 新建）；`git status --short` 空；`git diff --stat` 空（净树建枝）
- 性质=零栈零 LLM 纯设计：两仓只读勘察+报告产出，无真栈腿（不涉 PC-RUNTIME-STACK 登记）

### 回执（2026-11-09，零栈零 LLM，两仓只读）

- **报告**：[coord/reports/2026-11-09-ai-first-launcher-design-v1.md](../../reports/2026-11-09-ai-first-launcher-design-v1.md)（分支 `port/ai-launcher-design-1`，commit 见分支头；九节齐全）
- **九节自检表**：①信息架构=三态一窗（启动台/进入中/工程内），壳不动窗新增，经典页同屏一键可达，单窗常驻 §1 ✓；②对话流模型 §2：launcher/工程流定义（绑定态结构性判定非文本分类）、自主路由语义（意图→directive/结果卡/追问三分支+确定性退路）+轻 UI（分隔条+流徽章+播报一条，无弹窗）、历史分卷衔接（工程卷既有+启动卷会话内存+本地拼接 transcript+诚实边界）、**两案并列+建议（§2.4）**✓；③命令面 §3：launcher 档=结构性准入非 authority 新枚举（零改 authority 代码）、list/open/create 产品化（recent 结果卡+directive+新建链复用）、未保存切换保守确认（一律确认+save/require_path 既有语义）✓；④反向导航 §4：**结论=事件面够用零内核改动（停止条件不触发）**——reply 驱动 directive 为主（side_panel_request 先例）+/agent/events 备份+/agent/state 补 current_project 两字段（可选小缺口）；写者不变式=agent 出指令 UI 执行 ✓；⑤启动生命周期 §5：自举链零改动复用、窗体骨架先行防白屏、降级=agent 起不来→引导卡+经典页零阻塞（L1-d 案 A 同款问题同款解法）✓；⑥分档表 §6：展演边界一句话（新窗+directive 链+持久挂载，其余复用），LLM_Chat_Controller 零触碰、门厅全量/onboarding 三段式/HUD/聊天面组件化/跨工程持久化/群聊面全列答辩后（群聊面与本窗关系一句声明=§6 末行）✓；⑦拆卡 §7 ✓；⑧风险表 §8：卡面三硬项（反向导航首例含三层确定性退路/单窗蔓延含 C3 整卡可砍/ freeze 时间含砍序预案）+双窗并存/卷丢失观感/切场景竞态 ✓；⑨差异叙事 §9 一段 ✓
- **拆卡数与总量级**：6 卡（Godot 域 C1-C3 / agent 域 C4-C5 / 收口 C6），展演版合计 **7.5-9.5 人日，中值 8.75**（C1=2/C2=1.5/C3=1.5-2/C4=1.5/C5=1/C6=1）；双执行流并行（Godot‖agent 文件域零相交），关键路径≈6-7 天，日历≈4-5 天；最薄展演预案（freeze 应急砍序）≈5 天
- **两案建议结论**：**A（单窗常驻）为展演目标、B（交接播报降级=复用 315）为 A 的结构内退化态，实现序=先 B 部件后 A 叠加**——A=B+窗口持久化+dock 挂点两件增量（单独成卡 C3），freeze 前砍 C3 即整体退 B 不动其他卡；agent 起不来/持久层异常时产品自动以 B 形态工作（§5.3 降级路径）。依据=裁定定调（单窗常驻+自主切换是惊艳点骨架）+两案共享约九成部件
- **勘察结论上报项**：无 blocked 项；反向导航零内核改动（§4）；唯一可选小缺口=/agent/state 补 `current_project` 身份两字段（~10 行，随 C4 卡）
- 零栈纯设计卡：无测试命令/无 run ID/无栈登记；diff=本报告+本卡面（`git show --stat`：报告 1 新文件+卡面回填）

### 终验（主管，2026-11-09）

- **pass（主管段）——候用户终审惊艳点后拆卡**。九节齐全+17 项现状锚点（F1-F17）我抽查三项亲证吻合（F5 side_panel_request 先例 server.go:980+前端消费面在场/F8 AUTHORITY-LOST-1 闩/F11 无工程态 activation 早退）；停止条件核查结论（事件面够用零内核改动）成立。
- **设计裁定（主管采纳）**：①流判定=结构性（工程绑定态）非文本分类+确定性退路三层（结果卡/磁带流/治理确认）——惊艳点建立在可靠退路之上，正确形态；②写者不变式（agent 决定与呈报、UI 执行打开）——治理红线落点干净；③案 A/B 分水岭=C3 整卡可砍——降级同骨架；④launcher 档=结构性准入零改 authority 代码。
- **候用户终审两点**：惊艳点呈现形态（分隔条+流徽章+桥接播报的"轻不吵"三件）是否达标；展演开场白（门厅简版：在场+最近工程+从哪开始）文案口径。
- 终审通过后按 §7 拆 C1-C6 六卡双执行流开工（Godot 域‖agent 域）。
