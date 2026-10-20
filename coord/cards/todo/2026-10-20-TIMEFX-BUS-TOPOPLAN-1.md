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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 单测名 / 真栈 run ID 与断言面 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
