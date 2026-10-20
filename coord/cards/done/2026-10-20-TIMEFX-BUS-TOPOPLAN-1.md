# TIMEFX-BUS-TOPOPLAN-1：时域族默认总线拓扑——拓扑计划对象+确定性模板+提案卡呈现+装载链消费

- 发卡：GLM 主管决策流 / 2026-10-20 深夜（依据=146 实验腿判读：B2 0/2 ≪80% → 预绑定规则=硬规则接管默认拓扑计划生成；原拆卡 A+B 合并——软提示词段随硬模板同卡落笔）
- 派发确认：已确认（146 卡实验腿验收裁定）
- 验收负责人：GLM 主管决策流
- 池序 154；目标仓库=D:\Vit_DAW；P1 / 1-1.5 天 / 依赖=147 已合入（plugin_recommendation.go 域干净）；**与 155 串行（154 先）**
- 模型分级：L2 / GLM 执行会话
- **规格全文=[146 设计稿 §二](../../reports/2026-10-20-146-timefx-design-and-experiment.md)（权威）+实验判读 [RESULTS.md §0](../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md)**

## 范围

1. **拓扑计划对象** semanticTopologyPlan（Mode=bus|inline_exception / FXTrack=create|reuse / Loads / Sends / Exception{Code,Reason}）——设计稿 §二.1 结构为准；
2. **确定性默认模板**（硬规则，136 posture）：时域族词表命中（复用 c3SpacePluginPathMarkers/c3SpaceParamMarkers，抽公共引用）且无显式拓扑指示 → bus 模板（复用既有可用时域 FX 轨或新建默认名 Reverb Bus/Delay Bus→装载→源轨 send.connect 默认发送量 −6dB 起步入提案供改）；inline 仅凭例外码放行（user_explicit/single_track_ab/experiment_probe——取证④清单×5）；
3. **软面同卡**：自由态系统提示词惯例段（设计稿 §一.D 文本）+例外路由解释——解释与遵循已给选项，不发明拓扑；
4. **提案确认卡呈现拓扑计划**（建什么轨/连谁/默认发送量，卡面合同遵 148 修复后指名合同）；装载链消费 topology plan（去 selected_track_id 直通——六锚点 :125/286/528/568/814/843）；
5. 129 宏卡接口位预留（fx_track/sends 可绑目标）不实现；C3 观察侧零改动；入口分类器零改动（内容盲契约保持）。

## 文件域

`agent/internal/chat/plugin_recommendation.go`、`agent/internal/chat/semantic_treatment_strategy.go`（plan 结构）、`agent/internal/chat/plugin_grabber_workflow.go`（装载面消费）、`agent/internal/agentloop/ccb_model_prompt.go`+carriers（惯例段）、`agent/internal/chat/c3_space_depth_runtime.go`（词表导出，只动导出）+测试。越域即停。

## 验收标准

单测（模板触发/例外码/词表命中/复用既有轨/提示词惯例段）+`cd agent && go build ./...`+chat+agentloop 包+全量 0 FAIL+gofmt 净；真栈烟测腿（dev_agent_smoke 扩腿或同模式新腿，按 §5）：模糊+显式时域话术→提案卡含拓扑计划→确认→建轨/装载/send.connect→track.send.list 读回断言；inline 例外场景对照断言。端测覆盖边界回执声明（渲染面边界按 §5 显式声明）。

## 停止条件

- 复用既有 FX 轨判定需 C3 之外的观察面 → 上交锚点收窄；
- 提案链消费点与 K2 send_route 确定性工作流冲突（显式 send 请求被误拦）→ 锚点上交，K2 面零触碰红线保持。

- 领取：（2026-10-20 深夜 +0800 / origin/main=f7459c5681466543490d8ab10bee80d9d08b06ad / GLM-5.3 L2 PC 执行会话 ZCode（本会话，用户口令指定领取） / 分支=port/timefx-bus-topoplan-1（worktree D:/Vit_DAW_wt_topoplan154）+ 协调 worktree D:/Vit_DAW_wt_topoplan154_coord（coord/topoplan154-claim）/ 领取提交=本提交；领取核对：卡在 todo 派发已确认、147 已合入（f7459c56 含 148 验收链）、155（WRITEGATE）仍在 todo 串行前提成立、doing 两卡（GEN-SUNO 零生产代码 / BUDGETSTOP goalrunner_chat.go）文件域零冲突）
- 回执（2026-10-20 深夜收口，GLM-5.3 L2 PC 执行会话）：
  - **实现 commits**：`port/timefx-bus-topoplan-1@aa6118de`（主实现 52944fb5=拓扑计划对象+确定性模板+选卡/续接消费+装载面 bus 分支+finish 两腿派发+carriers 惯例段+新测试文件；补修 c9708940=无 PCA receipt 的 full-access bus 装载授权回落——空指纹降级 token 会先死于装载门指纹/完整性检查，full-access 下不铸造让门自行解析已晋升准入；脚本迭代 8 commits+工件 3 commits）。
  - **单测**：`semantic_topology_plan_test.go` 新 16 测——TestSemanticTopologyTemplate{BusCreate,DelayFamily,FuzzyEnglishVocabularyHit,NonTimeDomainNil,NoSourceNil,K2Defer,InlineExceptionCodes×3,ReuseByName,ReuseByFamilyPlugin,NoReadFallsBackToCreate}+PayloadRoundTrip+CardLinesContract；TestRunSemanticTopologyBusLoadWorkflowPlanShape（Decisions 空+WorkflowData 携计划+148 合同文案）；TestFinishPluginGrabberLoadWorkflow{ExecutesConnectLeg,ReuseCaseUsesFxTrackID,BusConnectRefusedHonestFailure}（真 harness 派发面：connect 腿经写闸授权键真实 Invoke；装载腿单测如实记录被 PCA 门指纹面拒=物理插件文件缺席，完整两腿归真栈腿）；TestCarriersNeutralFamilyCarriesTopologyConventionSection（§一.D 逐字+例外路由+CacheKey）。
  - **命令面**：`cd agent && go build ./...` exit 0；`go test ./... -count=1` **全仓 0 FAIL**（chat 108s 全绿+agentloop+carriers+contextruntime+全包）；gofmt 本卡 5 文件净（CR 剥离同 148 先例）。
  - **真栈 run**：`TIMEFX-BUS-TOPOPLAN-1/20261020_235001`——`dev_agent_smoke -StartKernel -RestartAgent -Scenario timefx_bus_topoplan -KernelExe build-feguik3(722A79D7)` **exit 0**（AGENTS §5 门槛）。断言面：E2 确定性 bus 腿=授权 full_project_access+显式装载话术+context 携带拓扑计划→零停泊→executed replies 含 track.send.connect ok（source=vocals/1032，auto_create Reverb Bus→FX/1037）+rack_add_node ok（落 FX 轨≠源轨）+回执文案含「总线拓扑已建立」+无「当前轨道」（148 合同）→track.send.list 读回 vocals→Reverb Bus **@-6.0dB**；E3 inline 对照=inline_exception 计划（code=user_explicit）→rack 落源轨 1032+零 connect 腿+无总线文案。NL 旅程三探（显式/inline 措辞/模糊空间感）=记录面：**F-gate 族×3**（runs 224614/225625/232517，自由态准入门弹回「目标证据/证据新鲜度/目标一致性」——146 止损②同机制=156/E 域上游，证据在卷含 mix.send_topology 观察轮）。九 run 迭代与执行侧缺陷（推荐问句自答/漏 146 前缀/staging 内核无 send 命令族）如实记录于栈记录与工件。
  - **端测覆盖边界声明（§5）**：①渲染面：本卡零 webui 改动；卡面文案与 topology_plan payload 经 HTTP 接口断言，未经浏览器级 DOM 断言（E2E-WEBUI-1 未就位，渲染面边界如实保留）；②用户旅程：NL 话术→选卡→确认的完整旅程被上游自由态准入门阻断（F-gate 族，156/E 域），本卡消费面以确定性 E2/E3（147 LEG E 哲学）+单测钉住，旅程级复验待 156/E 落地；③文件域偏差如实记录：carriers 实际路径=`agent/internal/contextruntime/carriers/carriers.go`（卡面「+carriers」所指=146 实验 b168fba6 验证的活性注入点）；`ccb_model_prompt.go` 与 `c3_space_depth_runtime.go` 零改动（骨架函数活性不可达=146 attempt2 已证；词表 chat 包内直接复用无跨包消费者故无需导出——「只动导出」条如实偏差）；④内核选型：staging B6565DCF 早于 send-route 命令族，烟测腿改 K3 内核 build-feguik3 只读复用（本卡零内核改动，栈记录 6630030b 修正留痕）。
  - 栈占用：22:4x 登记–23:5x 释放（c3ae8c25/6630030b/318805ec），五端口复核=0。
- 验收：（裁定文件 / 验收 commit）
