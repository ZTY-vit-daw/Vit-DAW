# M3-OBS-EVIDENCE-FIXTURE-1：229 验收门承接——观察证据面修复（测试工程补确定性音频 clip+capability_blocked 判定归类，主管选项裁定 A+C）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=229 重验腿新根因定案+主管验收裁定：裸轨工程 acoustic package missing→派生观察源证据缺失→模型 capability_blocked 诚实停边——模型行为正确，测试工程形态与 FS 准入语义的结构性交互问题；**A 采（fixture 补 clip）+C 采（判定归类）+B 否（FS 准入门不为测试便利弱化）**，裁定全文见 229 卡验收节）
- 派发确认：待确认（229 验收转出，终态架构阶段②收口卡；发卡即提交，领取按池序）
- 验收负责人：GLM 主管决策流
- 池序 230；目标仓库=**主仓 scripts 域**；P1 / 0.5 天 / 依赖=229 已合入（8954b8e1）✓；L1-L2 / PC 执行会话

## 标的（229 定案，不重考）

220 断言体 setup=两根裸轨（无音频 clip）→acoustic package 状态 missing（.vit_derived 亲证，229 run evidence/ 摘录在卷）→频段动态/电平 levels/多轨关系观察全部不可用→模型合规停在 FS 准入门（r3 capability_blocked 申报原文：「无法确认一个具体候选目标…要继续，需要先恢复该轨道可用的电平或频段观察数据」）。229 验收门（组 1 no_route 消失+set_points 端到端首绿）在裸轨 fixture 上结构性不可达。

## 目标

1. **fixture 补音频 clip（选项 A）**：勘察内核 clip 导入命令面（add_audio_track 已知在场；clip 导入/生成确定性途径勘察定）——setup 工程为两轨补确定性音频内容（脚本生成 wav 导入，或既有样例 clip 复制到 run 目录隔离工作区，§10 权威输入防污染纪律）。验证链逐环亲证：clip 在场→.vit_derived acoustic package 状态 available→轨道级观察返回事实数据。**断言逻辑原口径不动**（三组×5 轮、组级 ≥4/5、五分类语义、同型三连止损全部承 220；改的只是 setup 工程形态）。
2. **判定归类（选项 C，主管授权改判定面）**：模型 capability_blocked 诚实申报不再落 no_route 词表漏配——勘察 220 断言体判定段后定最小改法（五分类新增 capability_blocked 型或并入既有最贴近型的归类裁定，成文记录）；既有四型分类语义零回退。
3. **229 验收门重验（本卡验收门）**：组 1「这段让它越来越突出一点」先行——**组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿**；组 2/3 按 220 口径跑满。

## 排除面（主管裁定，卡面禁改）

- **FS 准入门（G3/G5）语义零触碰（选项 B 否决）**：空轨/无音频工程上模型诚实停边=终态架构期望行为；证据准入纪律不为测试便利弱化。「空轨可写」若为产品需求，属产品级设计讨论另立裁定。
- **agentloop/agent 生产面零改动**：本卡全部在 scripts 域（+必要时 scripts 内测试帮手/素材生成）；若发现观察管线缺陷（补 clip 后 acoustic 仍 missing），越域即停转卡上交。

## 文件域

`scripts/automation_llm_route_smoke.py`（setup 段+判定分类段）+run 目录内自足的 clip 素材生成/复制+必要时 scripts 帮手。越域即停上交。

## 停止条件

- 内核/agent 无确定性 clip 导入途径→锚点+方案上交（不改内核迁就）；
- 补 clip 后 acoustic package 仍 missing→观察管线缺陷锚点上交转卡（非本卡域）；
- 证据齐备（clip+观察事实在场）后组 1 仍三连 no_route→模型能力面锚点上交（229 停止条件 3 承接）；
- LLM 面栈不可用→环境中断如实记录。

## 领取

- 时间：2026-10-31 18:12；origin/main=6bd5b804（fetch 亲核=远端一致，无新提交）
- owner：ZCode GLM-5.3 PC 执行会话（230 卡会话，用户转交提示词指定领取）
- 分支：port/m3-obs-evidence-fixture-1；worktree：D:/Vit_DAW-wt-m3obs（基于 origin/main 6bd5b804 新建）
- 领取基线：`git rev-parse HEAD`=6bd5b804569f90c6ea3935cfe2a344463d73ec13；`git status --short`=空；`git diff --stat`=空（工作树干净，无领取前 diff）
- 前提核对：229 已合入（8954b8e1 在 origin/main 线）✓；卡面标的与排除面不重考；文件域=scripts/automation_llm_route_smoke.py（setup 段+判定分类段）+run 目录素材+scripts 帮手
- 真栈错峰：真栈腿前 PC-RUNTIME-STACK.md fetch 重读置顶+进程/端口全零亲核+登记先于启动（229 先例）；登记 6fda77e5（18:2x）
- §8 口径：LLM 真实计费，回执披露调用数与 token；断言逻辑原口径不动（三组×5 轮、组级 ≥4/5、同型三连止损承 220）

## 回执（2026-10-31，卡留 doing 等主管验收）

### 链路进度（§7 分列一）

- **选项 A（fixture 补确定性 clip）落地并真栈兑现**：setup stage 2——脚本生成确定性 wav（48kHz/16bit/stereo/6.0s，220Hz 基音+1760/1320Hz 渐强高频+升包络，run 目录自足、两次生成 sha 级一致；仓库 *.wav gitignore 挡入库故以脚本重建为可复现面）→ 两轨 `add_audio_clip`（内核 ZMQ 面）→ 三环证据链逐环亲证：①clip 在场（get_project_state `clip_counts={"1007":1,"1010":1}`）②acoustic package **missing→partial**（l3_deep band_energy/loudness/stereo_relation 三 feature **ready**——229 全 missing 亲比对，摘录 rounds/setup_acoustic_package_status.json）③轨道级观察返回事实（`ccb.observation_request(track.basic_energy, target=1010)` **1 次尝试**即 bundle.views 在场非 omissions；注：/agent/invoke 回复为 result.bundle 两层结构，leg1 解析缺陷域内修正后重跑，修正前 leg1 控制台已证观察请求本身成功）。
- **选项 C（capability_blocked 判定归类）落地并真栈兑现**：五分类新增 **capability_blocked 独立失败型**（归类裁定成文=脚本 docstring：非 no_route（路由已找到）、非 boundary_ok（该词表仅覆盖插件/空间范围缺失），既有四型+boundary 语义零回退；进止损集防烧轮）；三重信号面=stop_reason 结构信号（229 r1 形态直击）+申报词表（源自 229 r1/r3 回执原文）+异步续跑 runtime 回执（229 r2/r3 形态，.vit_history/**/agent_runtime_state.json free_state_reasoning_loops 匹配 conversation，15s grace 双查）。真栈 **g1_r2 首次正确归类 capability_blocked**（229 同款申报被误记 no_route）；离线验证=229 三轮原文重放全预期分型+boundary 插件文本仍归 boundary_ok+plain advice 仍落 no_route。
- choreo-pending 腿：**pending_intact**（组 1 r5 嵌入，218 LEG A 形态保持，新 fixture 零干扰）。
- 真栈 ps1 **exit 0 COMPLETED**；kernel=wt_228 VitApp.exe sha256 f9e3da0a 运行前后零漂移（只读复用，本卡 C++ 零改动）。

### 最终验收（§7 分列二）

- **组 1 no_route 消失：未达**——verdicts=[no_route, capability_blocked, no_route, no_route, no_route]，no_route 三连止损（r3/r4/r5），组 2/3 按 220 止损设计 skipped 未消耗。**set_points 确认链端到端首绿：未达**（模型未到执行相）。ps1 exit 0=验证卡完成语义（执行到止损=完成），非验收门通过。
- **新根因（观察管线 identity 绑定缺陷，agent 生产面越域上交，停止条件 2 变体承接）**：补 clip 后 bake 事实**已在场**（l1 peak_rms_summary.ref 携带 rms_dbfs=-11.889/peak_dbfs=-4.188/crest_db=7.701/time_segments=2；l3 三 feature ready）但**同一栈一次运行内四个 project identity 断裂**（内核 edit state 导入 bake 携带 vitproj_03a0…≠agent 观察链解析 vitproj_ce9e…≠落盘工程 vitproj_8093…≠agent 启动默认工程 vitproj_43f8…）→ l1_static 被 MergeStatus identity mismatch 判 **stale（reason=project_mismatch）** → `track.1010.fast.levels` 读出**全 null** → 电平观察事实不可用（229 r3「levels 读取失败」在补 clip 后依然成立；229 r1「工程标识不一致」申报的实体即此）→ 模型 r2 诚实申报「目标证据/目标一致性 两项准入检查未过」capability_blocked 停边（终态架构期望行为；FS 准入门语义零触碰，B 否裁定维持），r1/r3/r4/r5 预算尽转异步续跑（排队文本→no_route）。锚点+证据：coord/runs/M3-OBS-EVIDENCE-FIXTURE-1/obsfixture_20261031_1/evidence/identity_break_and_stale_l1.json（identity 断裂图谱+l1 stale 细节+levels null+r2 申报原文）。**修复面归属 agent 侧工程 identity 解析/观察归一层——越出本卡 scripts 文件域，按 §11 越域即停上交，建议主管折卡承接。**

### 工件与命令

- 实现+工件 commit：port/m3-obs-evidence-fixture-1 **c9440601**（已 push）；run=coord/runs/M3-OBS-EVIDENCE-FIXTURE-1/obsfixture_20261031_1（RESULTS.md+summary+console+rounds 5 轮+choreo 腿+acoustic 摘录+evidence+logs；派生态 .vit_agent/.vit_history/.vit_derived/工程文件摘录后剔除按 218/222/229 先例）。
- 测试：`python -m py_compile scripts/automation_llm_route_smoke.py` exit 0；离线判定面验证全绿（229 原文重放/boundary 零回退/wav sha 确定性/runtime 回执 glob）；真栈 `dev_agent_smoke.ps1 -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe <wt_228 F9E3DA0A>` **exit 0**（2 腿：leg1 setup 解析缺陷域内修正非 §8 同版本原样重跑，leg2 一次到位无迭代轮）。
- 栈：登记 6fda77e5（18:2x 置顶先于启动）/释放见 PC-RUNTIME-STACK.md（拆栈复核进程/端口全零亲核）。

### 计费披露（§8）

- LLM **56 调用真实计费**（deepseek-flash；**798,418 input / 87,721 output tokens**——本轮遥测 usage 字段有值，229 时为空，token 明细在 logs/agent_llm_telemetry.jsonl 逐调用）。失败分类：no_route×4（异步续跑排队形态）+capability_blocked×1（正确归类）；零 env_interrupt/confirm_broken/write_fail/lexicon_hit/wrong_tool。止损后按设计停止，无迭代轮，不原样重跑（根因=identity 绑定结构性缺陷已定位，非偶发）。

## 验收（GLM 主管决策流 / 2026-10-31）

- **两面实现 pass 合入**：选项 A 三环链主管亲证（clip_counts 在场→setup_acoustic_package_status.json partial+l3 三 feature ready 亲读→observation_request 返回 views）；选项 C 分型主管亲证（g1_r2 stop_reason=capability_blocked 结构信号+申报原文亲读；diff 级核过=仅 5 行删除均为文档/重构/分类集扩展，断言核心三组×5 轮+组级 ≥4/5+止损原口径未动）；choreo-pending 腿 intact；离线判定面重放验证在卷。
- **验收门未达=转出 231 承接**：identity 四断裂证据主管亲证（evidence/identity_break_and_stale_l1.json 亲读：bake 导入 03a0≠观察链 ce9e≠落盘 8093≠boot 43f8；l1 事实 rms=-11.889/peak=-4.188 在场被 project_mismatch 判 stale；fast.levels 全 null；IdentityFromMaps 五级回落链亲查=多源解析无对账）。**归属裁定=agent 生产面为主**：MergeStatus 防御判 stale 本身正确，错在上游 identity 分裂；但 231 须先诊断后修——烟测流（boot 换默认工程+中途 save_as）贡献的分裂要区分「合法 fork 语义缺再绑定传播」vs「测试流工件」，save-as 属真实用户旅程，真旅程暴露面判定是 231 验收面之一。B 否裁定维持（FS 准入门零触碰，模型诚实停边=终态期望行为）。
- 遗留观察：r1/r3-r5 的 no_route verdicts 为异步续跑排队文本归类（预算尽形态），与 r2 同根因（identity→levels null）；231 修复后该形态应随观察事实可用而消失，若残留另属续跑回执分类面，届时再议。
