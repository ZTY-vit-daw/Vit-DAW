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

- （执行侧/主管回填）
