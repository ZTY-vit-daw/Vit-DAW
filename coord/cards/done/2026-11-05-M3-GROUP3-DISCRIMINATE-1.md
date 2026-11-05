# M3-GROUP3-DISCRIMINATE-1：组 3 单组判别 run（链收口判别补腿）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=249 验收裁定 [2026-11-05-M3-GROUP23-DISCRIMINATE-1-pass.md](../../rulings/2026-11-05-M3-GROUP23-DISCRIMINATE-1-pass.md) §三-2；组 3 未采样=249 卡面设计缺口，非执行侧）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 251；目标仓库=主仓 agent；P1 / 0.25 天 / 依赖=249 已在 main（0298790e）✓；L1 / PC 执行会话；栈空闲（249 释放 10:06 亲核）

## 背景

249 `-Groups "2,3"` 单 run 中组 2 连续 3×no_route 触发脚本止损，脚本既有行为连带 skip 组 3（automation_llm_route_smoke.py:1561）——组 3「30 秒之后动起来」（时间窗 boundary 形态）在 v2/v3 措辞链新栈上的行为仍未判别。242 时代组 3=no_route×3（语义歧义「我没太明白你的意思」）。

## 目标

1. **单组判别 run**：`-LlmRouteGroups "3"` 单 run（deepseek-flash，载荷+agent log 全开）——组 3 话术 ×5（止损线=组内连续 3 同型，可能 3-5 样本）。
2. **判定**：脚本组判据（组内 ≥4/5 正确行为轮=过）；逐轮 verdict+断点形态如实分列，不追绿；与 242 时代组 3 语义歧义形态对照（消失/转化/复现）。
3. 产出直接供链收口报告与统一手测清单（组 3 时间窗话术=手测关键场景之一）。

## 预期与诚实边界

- §8 预算 ≈**¥2-3.5**（依据=249 实测 ≈15 调用/样本×3-5 样本+setup/choreo 腿；单轮 ≈¥0.4-0.6）；单 run 无重跑。
- 成功条件=脚本判定 exit 0（组判据过）或新形态取证包（逐轮分列，不原样重跑）。
- 本卡**零代码改动**（纯判别 run+判读）；新断点只取证不修（转卡）。
- **执行要点（249 教训入卡）**：新建 worktree 凡 `-StartKernel` 必显式 `-KernelExe`（wt_228 VitApp.exe f9e3da0a 只读复用，起栈前哈希亲核）。

## 停止条件

- 脚本功能断言失败→取证包上交（逐轮 verdict+载荷坐标），不重跑；
- 栈/环境中断→按 §9 记录处置（零 LLM 损耗的配置修正重启合规）；
- 任何代码修复冲动→越域即停（本卡无实现面）。

## 文件域

零代码改动；产出=coord/runs/M3-GROUP3-DISCRIMINATE-1/ 工件+卡面回执。

## 领取

- 领取时间：2026-11-05 10:29 +0800
- 领取时 origin/main：9adfea3107466a4c3eb8a61a8e641e89e3696af0（=local HEAD，已核对依赖 249 验收 pass 9adfea31 在内）
- owner：GLM-5.3-Flash（ZCode PC 执行会话，L1）/ 机器=PC / 卡=M3-GROUP3-DISCRIMINATE-1
- 分支：port/m3-group3-discriminate-1（基线=9adfea31）
- worktree：D:\Vit_DAW_wt_251run（新建，领取时干净）
- 领取基线工作树状态（主检出 D:\Vit_DAW，他流遗留与本卡文件域不相交，不动）：tracked 修改 8 文件（VitApp/Workspace 两 xml、CHAT-AUTH-STAMP-1 卡 blocked→done mv 未提交、UX-AB-AUDITION-IMPL-1 工件 4 文件）；未跟踪副本若干（agent/coord/、webui/design/、coord/runs/ 多目录、todo/2026-11-05-REPO-WORKTREE-CLEANUP-1.md 等）——均不属本卡文件域
- 栈登记：见 coord/resources/PC-RUNTIME-STACK.md（本卡单 run 前另行登记占用，推送 main 后 ≤60s 内起栈）

## 回执

- **执行窗口**：2026-11-05 10:29 领取 → 10:5x 收口；栈占用 10:31 登记（d2a9f4ad）→ 拆栈 TEARDOWN_ALL_ZERO 亲核 10:50:22（PROCS=0+TCP 六口=0+UDP=0，owner 手拆 VitAgent 46040/VitApp 10816）
- **完整命令**：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -RepoRoot 'D:\Vit_DAW_wt_251run' -StartKernel -RestartAgent -AutomationLLMRouteSmoke -LlmRouteGroups '3' -LlmModelOverride deepseek-flash -KernelExe 'D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe'`（-KernelExe 249 教训执行到位，kernel f9e3da0a 10:30 哈希亲核）
- **退出码两笔**：①启动一 10:32:50 exit 127=Git Bash 反斜杠路径转义吞字（AGENTS §5 已知陷阱），ps1 未执行、零进程零 LLM 损耗，配置修正重启合规（卡面停止条件 2），取证 ABORT-NOTE.md 留档；②主 run 10:36:35 起，python 腿 `[DONE] status=GUIDANCE_MISSING exit=2`（10:37:36）→ ps1:8779 throw → **ps1 exit 1**——判定数据在 exit 1 前已完整落盘，exit 1=验证面 stop condition 2 非判别中断；run ID=`automation_llm_route_20261105_103635`
- **判定（组判据）**：组 3「30 秒之后动起来」3 样本 verdicts=[no_route×3]，successes=0，**group_pass=false**（≥4/5 未过），连续 3 同型止损（stop_loss={group 3, no_route, consecutive 3}）；choreo_pending_leg=null（第 4 样本窗口未达）；lexicon_hit（组 3 成功形态）0/3 未发生（裸话术与跟进「0 到 30 秒」均不在确定性词表）
- **断点形态（新证据，逐轮分列）**：三轮全同型，断点层=**语义入口仲裁器**（chat/semantic_entry.go 独立 2896 字符分类小面）——12/12 交换 route=unresolved（conf 0.18-0.35），拒绝理由为模型真实逐次生成（主话术=「动起来无明确声学结果」三版措辞；跟进=「仅时间范围不构成请求」）；回复「我没太明白你的意思——…」=agent 确定性模板（semantic_entry.go:395 亲核）；自由态/自动化面零到达（13/13 载荷零 neutral-family 特征）
- **与 242 对照（判别结论）**：顶层复现（no_route×3 同判定）；r1/r2 仲裁层形态+模板**逐字复现**；242 r3「entry 解析成功→goal 武装→limit_reached 空转→settled completed 零写入」形态**未复现**（主话术解析率 1/3→0/3，小样本如实记录）；**链收口判别意义=v2/v3 措辞链效应面在自由态装配之后，对组 3 话术家族不可达——组 3 改进杠杆在仲裁器解析（模型能力/仲裁面设计），不在动作路径措辞**
- **ps1 exit 1 根因（结构性误报，非 242 装配缺陷回归）**：全仲裁形态下 message loop 零进入→prompt_render 插桩（message_loop.go:532 唯一调用点）零行落盘=文件未创建→check_prompt_guidance 读缺失文件 False→exit 2；装配函数本身无条件追加（ccb_model_prompt.go:61 亲核）+f1b7d204→9adfea31 零非 coord 改动亲核排除回归；**脚本盲区双重观察项上交**（文件缺失≠装配缺陷不可区分；全仲裁形态指导缺席结构性），供主管裁脚本判别面修订卡（本卡越域即停未动）
- **前置 Strip Silence 腿全绿零 warn**（needs_confirmation→确认→执行→清理四步 ok）——249 两 warn 未复现，其低优先观察项本 run 未见
- **计费实测（§8，归档 telemetry 全行口径）**：13 调用全 deepseek-flash（13/13），in 9,079 / out 3,452 tokens，≈**¥0.01-0.02 量级**——远低于卡面 ¥2-3.5 预算（仲裁小面短调用+止损@3 无长链；归档行数 13=summary usage 13 无窗口差，249 注记的窗口差异本 run 未出现）；启动一零损耗；单 run 无重跑达成
- **端测覆盖边界声明**：真栈单 run 在真实三件套完成（kernel wt_228 f9e3da0a 只读复用+worktree 构建 agent+ZMQ/HTTP 面亲核），非 mock；本卡零代码改动达成（全程仅 coord/ 卡面+资源记录+工件，agent/kernel/scripts 零触碰）
- **工件**：coord/runs/M3-GROUP3-DISCRIMINATE-1/g3run1/（RECEIPT.md+summary 全量 70KB+telemetry+payload 76KB 全量仲裁理由逐行+rounds g3_r1..3 全量+rounds_verdicts_extract+ps1 console 两笔+ABORT-NOTE），随 **port/m3-group3-discriminate-1 @ 31b574a7** 入库（分支已推远端）；clip.wav 已删，.vit_* 运行态留 worktree；详见该分支 g3run1/RECEIPT.md

## 验收

- （主管回填）
