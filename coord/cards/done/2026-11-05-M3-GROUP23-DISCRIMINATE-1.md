# M3-GROUP23-DISCRIMINATE-1：组 2/3 判别 run（链收口判别步）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=248 验收裁定链收口前置件达成 [2026-11-05-M3-CONTINUATION-PARK-STORM-1-pass.md](../../rulings/2026-11-05-M3-CONTINUATION-PARK-STORM-1-pass.md)）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 249；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=248 已在 main（f8f2a751）✓；L1 / PC 执行会话；栈空闲（gate 亲核全零）

## 背景

组 1 已稳定绿（247 首绿+248 三 run 3/3 exit-0，13/15 tool_chain_ok）。242 时代组 2=confirm_broken（plugin_selection_required 外部插件候选确认面停驻，当时聚合为措辞修复候选）、组 3=boundary 形态；此后 v2 触发权句（243）+carry（245）+v3 终局路由（246）+guard 准入（247）全量落地——组 2/3 在新栈上的行为**未知，须判别**。

## 目标

1. **判别 run**：`-Groups 2,3` 单 run（deepseek-flash，载荷+agent log 全开）——组 2「帮我把空间铺开」×5、组 3「30 秒之后动起来」×5（脚本源核对：5 样本/组）。
2. **判定**：以 smoke 脚本自身的组判据为准（exit 0=过）；逐轮 verdict+断点形态如实分列，**不追绿**；残差形态（含组 1 已见的 no_route 旧形态偶发）记录复发率供统一手测参考。
3. **对照记录**：与 242 时代 g2/g3 形态对照（confirm_broken 是否消失/转化），为链收口报告与统一手测清单提供输入。

## 预期与诚实边界

- §8 预算 ≈**¥4-7**（依据=脚本源 10 LLM 样本+setup/choreo 腿；242 全组 run ¥4.94/13 轮+248 ¥9.26/15 轮区间外推；**发卡侧已核脚本源样本数——248 教训落地**）；单 run 无重跑。
- 成功条件=脚本判定 exit 0（组判据过）或新形态取证包（逐组分列，不原样重跑）。
- 本卡**零代码改动**（纯判别 run+判读）；发现新断点只取证不修（转卡）。

## 停止条件

- 脚本功能断言失败→取证包上交（含逐轮 verdict+载荷坐标），不重跑；
- 栈/环境中断→按 §9 记录处置；
- 任何代码修复冲动→越域即停（本卡无实现面）。

## 文件域

零代码改动；产出=coord/runs/M3-GROUP23-DISCRIMINATE-1/ 工件+卡面回执。

## 领取

- 领取时间：2026-11-05 09:32 +0800
- 领取时 origin/main：f1b7d204c0cb03fb5eac55e03713fc79d3417663（=local HEAD，已核对依赖 248 f8f2a751 在内）
- owner：GLM-5.3-Flash（ZCode PC 执行会话，L1）/ 机器=PC / 卡=M3-GROUP23-DISCRIMINATE-1
- 分支：port/m3-group23-discriminate-1（基线=f1b7d204）
- worktree：D:\Vit_DAW_wt_249run（新建，领取时干净）
- 领取基线工作树状态（主检出 D:\Vit_DAW，他流遗留与本卡文件域不相交，不动）：tracked 修改 8 文件（VitApp/Workspace 两 xml、CHAT-AUTH-STAMP-1 卡 blocked→done mv 未提交、UX-AB-AUDITION-IMPL-1 工件 4 文件）；未跟踪副本若干（agent/coord/、webui/design/、coord/runs/ 多目录、todo/2026-11-05-REPO-WORKTREE-CLEANUP-1.md 等）——均不属本卡文件域
- 栈登记：见 coord/resources/PC-RUNTIME-STACK.md（本卡单 run 前另行登记占用，推送 main 后 ≤60s 内起栈）

## 回执

- **执行窗口**：2026-11-05 09:32 领取 → 10:2x 收口；栈占用 09:34 登记（a8c90dd3）→ 10:06:22 TEARDOWN_ALL_ZERO 亲核释放（PROCS=0+TCP 六口=0+UDP=0）
- **完整命令**（g23run2，worktree 根执行）：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -RepoRoot 'D:\Vit_DAW_wt_249run' -StartKernel -RestartAgent -AutomationLLMRouteSmoke -LlmRouteGroups '2,3' -LlmModelOverride deepseek-flash -KernelExe 'D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe'`
- **退出码**：ps1 exit 0（腿执行完成 COMPLETED；组判据结果在 summary group_pass 字段——**组判据未过**，见下）；console 尾 `[DONE] status=COMPLETED exit=0`
- **run ID/工件坐标**：run 目录=`wt_249run\VitApp\Workspace\Artifacts\smoke\automation_llm_route_20261105_094045\`；判读+工件=coord/runs/M3-GROUP23-DISCRIMINATE-1/g23run2/（随 port/m3-group23-discriminate-1 入库 a30e758c；payload 4.6M/prompt_render 872K/summary 原样 4.4M/rounds 原样留 worktree，clip.wav 已删）
- **HEAD 与工作树状态**：被测 HEAD=a8c90dd3（零代码改动，agent=f1b7d204 基线构建 c66ee610；kernel=wt_228 f9e3da0a66263f0b 只读复用 09:34:44 亲核）；本卡 diff=领取提交 b25d7742（卡面）+a8c90dd3（栈登记）+a30e758c（工件）
- **判定结论（判别目标达成，新形态取证包，不追绿）**：
  - 组 2「帮我把空间铺开」×4 样本（连续 3×no_route 触发脚本止损，5 样本未跑满）：verdicts=capability_blocked, no_route×3，successes=0，**group_pass=false**；
  - 组 3「30 秒之后动起来」**未采样**（stop_loss_in_earlier_group 连带 skip）——**组 3 判别目标未达成**，是否开单组续卡（-Groups 3）由主管裁定；
  - 逐轮形态：r1=capability_blocked（G1-G8 准入全 pass+确认面正常挂起+自由态诚实申报掩蔽维度不可用/提案未受理——非 confirm_broken）；r2=no_route（文本建议+结算失败）；r3/r4=no_route（limit_reached 截停→continuation settled failed/completed 零自动化面，220 时代 turn 预算截停同族）；坐标=rounds_verdicts_extract.json
- **对照记录（vs 242 时代，2026-11-04 CURRENT-VALUE-DISCLOSE 全组 run）**：组 2 confirm_broken×3（plugin_selection_required 停驻）**已消失（0/4）→ 转化为 capability_blocked×1+no_route×3**；组 3 语义歧义形态**无法判别**（未采样，诚实边界）；措辞链部署面健康（guidance/触发权句/时变豁免 52/61、终局直调句 7 行、旧二选句 0、prompt_guidance_rendered=True——242 GUIDANCE_MISSING 缺陷未复现）
- **计费实测（§8）**：g23run2=61 调用 deepseek-flash，in 893,296/out 135,847≈**¥2-2.5 上界**（按 248 单价口径同比例外推），卡面 ¥4-7 预算内；g23run1 abort **LLM 零损耗**；单 run 无重跑达成
- **执行偏差披露（环境中断一笔）**：g23run1（09:36）遗漏卡面指明的 `-KernelExe`→新 worktree 无 Export/staging 产物→kernel 未启动（脚本 warn 非 fail）→python setup 卡住；owner 手拆复核全零后修正参数重启（配置修正，非断点原样重跑）；LLM 零损耗（console 空/telemetry 不存在）；教训=新建 worktree 凡 `-StartKernel` 必显式 `-KernelExe`；取证=g23run1/ABORT-NOTE.md
- **前置腿观察（非本卡场景腿，如实记录）**：Strip Silence 腿两 warn（chat confirmation did not complete / suggest 未产出期望 pending regions），ps1 仍 exit 0；一致性核对超本卡域留主管
- **端测覆盖边界声明**：本卡产出=真栈单 run 判读（组 2/3 模型行为形态+措辞部署面），不涉代码改动；组 3 未采样=判别缺口如实上交；done≠验收通过，终验=GLM 主管决策流

## 验收

- **2026-11-05 pass 合入**（GLM 主管决策流）——[2026-11-05-M3-GROUP23-DISCRIMINATE-1-pass.md](../../rulings/2026-11-05-M3-GROUP23-DISCRIMINATE-1-pass.md)：工件亲核（退出码/summary 判定面/逐轮 verdict/计费 67 行口径注记/零代码 diff/栈两笔/abort 证据链）；判别结论确认=组 2 confirm_broken 消失（242×3→0/4）+转化 capability_blocked×1+no_route×3（limit_reached 族 2）；组 3 未采样=结构性 skip——**发卡侧卡面缺口入档**（未预判止损连带 skip，脚本源 1561），处置=续卡 251 单组 run；g23run1 abort=环境中断零 LLM 损耗合规；计费 ¥2.3-3.0 上界卡面内。
