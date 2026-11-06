# CLOSURE-ROUTE-ADAPT-1：分析类请求误入 closure 治理路径的适配（276 rerun1 转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[276 rerun1 取证](../runs/EDU-SYMBOL-CORE-1/smokerun1/rerun1/evidence.md)+[276 终验裁定](../rulings/2026-11-07-EDU-SYMBOL-CORE-1-pass.md) §二——三因素：semantic_entry LLM 非确定性分类（同话术两轮 201/168 tok）×closure 治理路径武装入口×candidate-selection 轮 MaxTurns=1（agent/internal/chat/audio_closure_controller.go:529））
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 271；目标仓库=主仓 agent（路由/闭包预算面）；P2 / 0.5-1 天 / 依赖=276 工件在库；L1-L2 / PC；验证腿可搭后续真栈 run 便车（优先零栈构造）

## 背景

276 rerun1：分析类请求（「分析这段 MIDI 的变换关系」）被 semantic_entry LLM 分类**随机**路由进 minimal_audio_closure 治理路径（run1 同形消息未进），closure candidate-selection 轮 MaxTurns=1×模型单轮 86tok 无工具调用→limit_reached 停轮、executed 零、durable continuation 落库未终局——命令面根本未触达。

## 目标

1. **路由面取证+适配设计**（先行零栈）：closure 武装入口对「分析类请求」（无音频编辑意图的只读分析）的判据——理想=分析类不进 closure 治理路径（或进了也能走分析命令）；语义面属判断权归属裁定域（2026-10-30），**方案须先交主管裁定再动手**；
2. **预算面**：candidate-selection 轮 MaxTurns=1 的合理性取证（1-turn 是否足以选 candidate；若为有意设计记录边界，若欠账则最小修）；
3. **驱动面（可选腿）**：烟测驱动对 waiting_continue 停泊的有界续跑腿（发「继续」走完 continuation）——若路由/预算面修复后仍需；
4. 验证=零栈构造（路由判据单测+预算面单测）优先；真栈可搭后续 run 便车（276 chat exit 0 尾巴随本卡收口）。

## 约束

文件域=chat 路由/closure 预算面+测试+（可选）冒测驱动；禁动判断权归属裁定边界（方案先行交裁定）；治理链红线不变。

## 停止条件

取证发现路由为有意设计（在案裁定）→只记录边界转裁定；构造验证失败取证不重跑。

## 领取/回执/验收

### 领取（2026-11-07，勘察腿）

- Owner：GLM 主会话 freeflash 子代理（**勘察腿**——路由面取证+适配方案设计，读档零 LLM 零代码改动，产出=recon-design.md 交主管裁定，不动代码）
- 领取基线：origin/main `a8be057ed772f9030ab148eb9a6b704d0f07e064`（worktree 新建于该点，`git status --short` 空、`git diff --stat` 空）
- Worktree：`D:/Vit_DAW_wt_271impl` @ 分支 `port/closure-route-adapt-1`
- 主仓 coord/ 存在其他会话未提交改动（UX-AB-AUDITION-IMPL-1、FE-* runs 等）——不属本卡文件域，未触碰
- 工件目录：`D:/Vit_DAW/coord/runs/CLOSURE-ROUTE-ADAPT-1/`（PROGRESS.md 断点续跑协议执行中）

### 回执（勘察腿）

- 完成时间：2026-11-07；产出=[recon-design.md](../runs/CLOSURE-ROUTE-ADAPT-1/recon-design.md)（取证+3 案+裁定请求清单）+ [PROGRESS.md](../runs/CLOSURE-ROUTE-ADAPT-1/PROGRESS.md)；零代码改动、零 LLM 调用、纯读档（grep/git log/读文件）
- **路由面结论**：判据链 6 环全锚定（semantic_entry.go:79-99 分类语义 / :285-286 observation 与 open_semantic 同派 MinimalAudioClosure / capability_routing.go:308 无条件加固 / audio_closure_controller.go:108-119 武装+ModeDiagnostic 分派 / :356-367 模式双向映射）。分析类请求无专用类，discussion vs observation 判据边界摇摆=276 随机路由根因；准入侧无只读/编辑意图过滤（唯一区分=Mode 字段）；工具披露面已含分析命令（276 卷内实证），失败因素=预算×模型单轮行为而非工具不可达
- **预算面结论**：MaxTurns=1 为基线期冻结设计（38ecd24b 2026-08-19 注释逐字同今，"one probe + one verdict" 假设）+ 229 有意保留（测试钉 TestAudioClosureBudgetDiagnosticRoundKeepsClamp）；但 goalrunner_chat.go:800-807 诊断轮 base=5 turns 与 closure clamp=1 并存=**欠账形态直接证据**（同一诊断语义两个预算值）
- **方案建议**：主推**案 B（预算面最小修：诊断闭包轮承认基础预算，零判断权边界触碰）**；**案 A（路由面：observation 拆纯分析身份派 OrdinaryConversation，治本消解分类摇摆）提请判断权边界裁定**；案 C（工具面）兜底；案 D（续跑腿）独立验证腿
- **裁定请求 5 项**见 recon-design.md §6（含 2 项标红边界：route 语义划界授权、诊断轮 clamp 测试意图修订）
- **取证勘误**：276 rerun1 evidence.md「frontier 有候选=2」与卷内 chat_response.json 不符（frontier 实际空，MaxTurns=1 生效）——已记 recon-design.md §4，处置待裁定项 5
- 验证边界：本腿为勘察产出，无测试/构建/真栈运行；实现腿验收须另按卡面验证口径（零栈构造单测优先）

### 验收（主管）

- （主管回填）
- **勘察腿验收+裁定（2026-11-07 凌晨，主管）：pass+案 B 先行**——根因链钉死（LLM 分类摇摆×capability_routing.go:308 无条件派 closure×ModeDiagnostic 1-turn clamp[38ecd24b 基线期冻结]）；**裁定**：①案 B 获准（诊断闭包轮承认 base 预算 5-turn 同源——依据=同语义双预算欠账+229 automation 轮 48/80 预算面先例+结构性判据零判断权触碰[mode/phase 字段，禁话术判断入预算]）②**测试意图修订授权**（TestAudioClosureBudgetDiagnosticRoundKeepsClamp 以 276 卷内证据=one-probe-one-verdict 假设被打破为依据改意图+注明修订依据，禁静默改期望值）③案 A（纯分析身份派 OrdinaryConversation）**挂 PD-3 用户裁定**（判断权归属域）④案 C 不做（B 超集+触 227 定案边界）⑤276 遗留 continuation=fixture 内状态无收口义务记录⑥276 evidence frontier 笔误以本卡勘误为权威修正。续跑腿=验证腿便车兜底。
