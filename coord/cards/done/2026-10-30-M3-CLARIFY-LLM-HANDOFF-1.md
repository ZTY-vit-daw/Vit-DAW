# M3-CLARIFY-LLM-HANDOFF-1：澄清决策上移 LLM——能力目录进上下文+固定问句降级兜底（判断权归属裁定直接实现）

- 发卡：GLM 主管决策流 / 2026-10-30 晚（依据=用户裁定：工程无副歌标记应由 LLM 判断思考并**询问是否打标记**——混音能力环节本有段落标记能力；+用户固化担忧→[2026-10-30 判断权归属决策](../../decisions/2026-10-30-judgment-ownership.md)冻结令：判断类决策不再新增 Go 固定文案）
- 派发确认：已确认（用户 2026-10-30「①…应该由 LLM 进行判断思考…它应该询问是否要进行标记」+「可以开卡了」）
- 验收负责人：GLM 主管决策流
- 池序 225；目标仓库=**主仓 agent**；P1 / 1 天 / 依赖=**222 完成**（neutral 指引=LLM 判断层地基）+**224 提交后领取**（同文件 automation_choreo.go 函数域分界前置）；L2 / PC 执行会话

## 标的（判断权归属裁定的实现面）

词表命中但信息缺失（marker miss / span 候选 / 裸单端 / 类别缺失 / 参数缺失）时，当前出口=Go 固定澄清文案（219 面 1/2/3 刚加固的三面）。裁定后正确出口=**带上下文移交 LLM 轮次**。

## 目标（三面）

1. **澄清出口改造**：上述五类信息缺失形态→组装移交上下文进 LLM 轮次：意图解析结构化结果（category/direction/span/section）+候选（span 秒数/bare 值）+**能力目录披露（含段落标记能力——LLM 据此可提议"要不要我先听一遍把副歌标记打上"）**+工程状态摘要（轨/插件/已有标记）。LLM 输出对话回应（提议/候选问句/反问），**写入仍走既有确认链**（LLM 提议≠写入；不猜原则不变；完全权限直执照 223 后形态）。
2. **固定文案降级兜底**：219 三面固定问句保留为 **LLM 不可用时降级路径**（fail-safe），不再为正常路径。迁移清单核对入回执。
3. **护栏**：判断权冻结令执行面——本卡后 chat 域不得新增固定澄清文案；LLM 轮次失败/超时的兜底行为如实披露（不静默假装成功）。

## 探针义务

- 单测：五类信息缺失形态各一例→LLM 轮次被调用+上下文含能力目录+候选值；LLM 不可用→固定兜底问句+pending 状态机不破；LLM 提议后用户同意打标记→段落标记能力链可用（如越域则断言到能力调用入口）。
- 真栈腿（必跑，承 §8 口径）：真实 LLM 轮次「副歌那 30 秒…」（无标记工程）≥3 轮——预期 LLM 提议打标记或给候选（形态自由），**不得**退化为固定问句；记录轮数/积分。

## 文件域

`agent/internal/chat/automation_choreo.go`（**clarify 函数族+两入口 switch 澄清分支**——confirmation 函数族属 224 已提交基线）+`automation_intent.go`+LLM 轮次装配/上下文组装（勘察定，预判 contextruntime/promptruntime 邻域只读消费）+测试+scripts。越域即停上交。

## 停止条件

- 能力目录披露需动能力注册面（capabilityadapters 等）→锚点上交扩域裁定；
- LLM 轮次与 chat 确定性路由的会话状态互扰（pending 状态机冲突）→锚点+两案上交；
- 222 未完成不得领取（依赖硬约束）。

## 领取

- 时间：2026-11-05 19:29–19:35 +0800
- origin/main：**5ac45a12**（领取核对首 fetch 时=d520c4f7，核对窗口内主管推送 250 Phase 2 dispatch → worktree 按最新远端建立，依赖卡状态复核对未变：222/224 pass 合入 ✓、227 闸开放落地 ✓、253 词表武装合入 ✓、doing 两卡文件域不相交 ✓、PC 栈空闲（最近释放 19:09）✓）
- owner：ZCode GLM-5.3 L2 PC 执行会话（本会话，225 卡专会话）
- 分支：port/m3-clarify-llm-handoff-1 / worktree：D:/Vit_DAW_wt_225impl（新建，领取基线=干净树 `git status --short` 仅本卡 mv，无既有 diff——主检出他流痕迹不随 worktree）
- 领取提交：18de1d34（coord/225-claim → main fast-forward 已推送，远端 owner 已核对）
- 真栈错峰：真栈腿启动前按 §2.2 登记；§8 运行口径于真栈执行前成文入本卡，跑前不得改

## 真栈腿 §8 运行口径（2026-11-05 20:0x +0800 成文，跑前不得改）

- **命令**：`dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_225impl -StartKernel -RestartAgent -AutomationLLMRouteSmoke -LlmRouteGroups "1,3,4" -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe -LlmModelOverride deepseek-flash`（+载荷捕获全开，先例=238-253；kernel SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A 已亲核=wt_228 同款只读复用）
- **运行次数**：单 run 无重跑（红线）
- **成功条件**（全部满足才 pass）：
  1. ps1 exit 0（§5 门槛）；
  2. 组 1 回归红线：group_pass=true（≥4/5 tool_chain_ok/boundary_ok，确认一次→写达内核→读回不回退）；
  3. 组 3 回归：group_pass=true（≥4/5 lexicon_hit——225 后 clarify 出口文案来源升级为 handoff LLM 轮，判据 workflow/stop_reason 不变；**不得**出现仲裁器 unresolved 回归）；
  4. **组 4 新面（本卡验收主体）**：group_pass=true 3/3 clarify_handoff_llm——「副歌那 30 秒…」无标记工程话术→LLM 提议打标记或给候选（形态自由=clarify 卡 LLM 文案或治理确认卡命令提议），**0 轮**退化为固定问句（clarify_fallback_fixed）；
  5. 组 4 choreo-pending 嵌入腿 pending_intact（handoff 与 pending 状态机并存不破）；
  6. 计费如实披露，预估 ≈¥2.5-3.5（253 同栈组 1+3≈¥2.3 上界+组 4 三轮增量）。
- **失败分类**（预写）：clarify_fallback_fixed=功能断言失败（fallback 原因在 WorkflowData clarify_llm=fallback:* 披露+agent 日志取证，上交不重跑）；no_route（组 4 未达 clarify 出口）=接线回归取证上交；env_interrupt=栈/LLM 环境失败（原始日志留证，§9 处置）——若 telemetry 显示 handoff 轮全 HTTP 失败（flash 限流类，非模型行为），记 env_interrupt 候选交主管裁定，不自行改判；组 1/3 回归失败=回归红线破，如实记录上交，不得为凑绿重跑。
- **止损线**：单 run 消耗后无论结果不再重跑；组 4 no_route 立即停（脚本内 break）；修复冲动→越域即停上交。

## 回执（执行侧自验完成，待主管验收）

- 实现 commit：**de7c1da2**（port/m3-clarify-llm-handoff-1，基于领取提交 18de1d34）+coord 提交 f19ea6c3/e9c54afd（§8 口径+栈登记）；diff=`chat` 三文件+py：automation_clarify_handoff.go 新增（handoff 核心）、automation_choreo.go（两入口 12 处 clarify 调用点迁移+affirm face2 分流）、automation_clarify_ux_test.go（唯一受影响 219 pin 加死端口 env 守护，断言原文零弱化）、automation_clarify_handoff_test.go 新增（6 例）、automation_llm_route_smoke.py（组 4 opt-in，默认组集 1,2,3 字节不变，ps1 零改动）。automation_intent.go **零改动**（handoff 只读消费）。
- **单测**：go build ./... 全树 OK；`go test ./internal/chat/ -count=1` **全包 PASS（112s，exit 0）**；handoff 新测 6 例（五类形态 LLM 轮请求含能力目录 project.markers.*+候选值/兜底披露+pending 状态机不破含 adopt 链/marker 提议→治理确认卡（能力调用入口断言）/affirm 同意→能力命令/affirm 文本判断 pending 保留/入口全链 category miss）。
- **真栈腿**：run ID automation_llm_route_20261105_195543，PS1_EXIT=0；§8 六条成功条件全过——组 1 **5/5 tool_chain_ok**（回归红线绿）+组 3 **5/5 lexicon_hit** 零 unresolved（组 3 clarify 出口=handoff LLM 轮，判据不变亲证）+**组 4 3/3 clarify_handoff_llm**（「副歌那 30 秒」→LLM 候选区间+类别确认+**建议先打副歌标记再写包络**=裁定目标行为真栈实证，0 轮固定问句退化）；choreo-pending 腿三组 pending_intact；计费 100 调用全 deepseek-flash in 1,426,974/out 112,016≈¥2.2-2.4（预算内）；telemetry 8 行 fingerprint=automation-clarify-handoff:v1 亲证；单 run 无重跑。工件 coord/runs/M3-CLARIFY-LLM-HANDOFF-1/armrun1/（RECEIPT.md+summary_slim+console+telemetry+rounds；payload 6.0M+prompt_render 3.4M 大件留 worktree）。
- **栈两笔在册**：占用 e9c54afd（19:55 净空复核 PROCS=0/TCP=0/UDP=0 后同窗启动）/释放随本批（TEARDOWN_ALL_ZERO 亲核 20:11:43）。
- **迁移清单核对**（判断权决策 §2 护栏 2）：①span 降级候选问句（face 1）②肯定词族引导（face 2，affirm 分流 region_span_candidate 机械采纳表=执行层保留）③裸单端候选问句（face 3）——三面全部迁移为「LLM handoff 正常路径+219 固定文案逐字保留为 fail-safe 兜底」，WorkflowData clarify_llm=handoff*/fallback:原因 如实披露；另两入口 region/category/param 固定问句（219 时代）同步迁移。**track kind 卡面五类之外未迁移**（保留原固定路径=边界申报）。判断权冻结令执行面：本卡 diff 零新增用户面固定澄清文案（handoff system 指引=协议指令非问句文案）。
- **意外真实调用披露**：测试 bring-up 期间 1 次单测意外触达真实引擎（automation_clarify_handoff_test.go 初版 env 置空法失效——config.Load 文件配置兜底，~/.vit/config.json 引擎在场；已改 httptest 显式注入+broken-engine 形态，复跑全绿），计费约数百 token 级，如实记录。
- **端测覆盖边界声明（§5）**：已覆盖=真实栈 LLM handoff 轮（组 4 三轮+组 3 五轮真实计费）+五类形态单测（fake 引擎请求体断言）+治理链入口单测+chat 全包回归；**未覆盖**=「LLM 提议打标记→用户同意→marker 实际写入内核」的完整端到端真栈轮（组 4 判定面=第一回复，命令提议形态 cancel 收尾不 drive 确认链——marker 写入纵深属段落标记能力线/A5 链，单测已断言到 chatResponseForCommands 治理入口）；webui 渲染面（clarify 卡 LLM 文案渲染）未过浏览器级断言（E2E-WEBUI-1 建设卡未就绪，卡结构同形复制断言由 vitest 面覆盖=既有渲染测试同构）。
- 纪律核对：§12 领取基线在卡面（干净树）；worktree 独立（主检出他流痕迹零触碰）；工件独立 runs 目录不覆盖旧 run；无 stop 未披露项。

## 验收

- **2026-11-05 pass 合入**（GLM 主管决策流）——[2026-11-05-M3-CLARIFY-LLM-HANDOFF-1-pass.md](../../rulings/2026-11-05-M3-CLARIFY-LLM-HANDOFF-1-pass.md)：实现面亲核（判断权冻结令执行面=零新增固定澄清文案；LLM 提议≠写入走既有治理链；253 词表面未触）+测试 worktree 亲复跑 PASS+真栈判定面亲读（组 1 **5/5**/组 3 5/5/组 4 3/3+needle 81/95 旧句 0+¥2.2-2.4 卡面内）+意外调用与端测边界两面披露核可；工件随 port 分支合入 main。M3 线至此无尾巴。
