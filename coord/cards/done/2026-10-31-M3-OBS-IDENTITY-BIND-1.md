# M3-OBS-IDENTITY-BIND-1：观察链 project identity 对账——四源分裂定案与单一真相源（229/230 验收门承接，终态架构阶段②收口卡）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=230 验收定案：同一栈一次运行四个 project identity 断裂——bake 导入 vitproj_03a0≠观察链解析 ce9e≠落盘工程 8093≠agent boot 绑定 43f8→l1 bake 事实在场（rms -11.889/peak -4.188）被 MergeStatus project_mismatch 判 stale→track.1010.fast.levels 全 null→模型诚实 capability_blocked 停边；锚点 evidence/identity_break_and_stale_l1.json+IdentityFromMaps 五级回落链（acousticpackage/status.go:279）；MergeStatus 防御判 stale 本身正确，错在上游分裂）
- 派发确认：待确认（230 验收转出；发卡即提交，领取按池序）
- 验收负责人：GLM 主管决策流
- 池序 231；目标仓库=**主仓 agent 观察链 identity 解析/归一层**；P1 / 0.5-1 天 / 依赖=230 已合入 ✓；L2 / PC 执行会话（identity 语义+观察管线判读）

## 标的（230 定案，不重考）

观察链 identity 多源解析无对账：内核 edit state 导入 bake、agent 观察链 resolution、落盘工程文件、agent 启动绑定四源各自解析 project_id，任一分歧即把新鲜 bake 事实判 stale，电平观察读出 null。模型行为正确（诚实停边），FS 准入门零触碰（B 否裁定维持）。

## 目标

1. **诊断定案（先诊断后修，图谱成文）**：四个 id 的铸造代码锚点+语义逐一落定（fork/save_as/boot 绑定各自的铸造点与传播路径）——区分「合法 fork 语义但缺再绑定传播」（生产缺陷）vs「烟测流自身诱导的分裂」（测试工件）；给出**单一真相源设计裁定**：哪个 id 是权威（建议候选=内核当前 loaded project 的 id，落盘/观察/导入层跟随），其他层如何跟随/再绑定，成文入卡回执。
2. **真实旅程暴露面判定（本卡验收面之一）**：真栈上验证 save_as 旅程（真实用户操作：开工程→改动→save_as→观察）与 agent 重启旅程（226 先例形态）是否同样 identity 断裂——断=生产缺陷坐实修复；不断=分裂图谱+烟测流对齐方案上交（scripts 条件腿，见下）。
3. **修复+直接验证**：identity 对账后，补 clip 工程上轨道级观察 levels 读出非 null 事实（fast.levels rms/peak 非 null+MergeStatus 不再误判 project_mismatch）——单测（identity 分裂→对账→跟随的往返）+真栈腿。
4. **验收门（229/230 原口径，第三承）**：组 1「这段让它越来越突出一点」no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）端到端首绿；组 2/3 按 220 口径跑满。

## 域与条件腿

- 主域：`agent/internal/acousticpackage/`（status merge/identity）+`harness`/`projectworkspace` 解析面+测试；越域即停上交。
- **条件 scripts 腿（先决条件严格）**：仅当诊断定案=烟测流自身诱导合法分裂且真实旅程不暴露（目标 2 判定）时，方可改 `scripts/automation_llm_route_smoke.py` 的工程打开流（如直接打开隔离工程替代 boot 换绑+中途 save_as）——**分裂图谱+真实旅程判定先成文，测试侧流对齐不得作为绕过生产缺陷的逃逸**；两裁定都在卷才动 scripts。

## 安全面

- MergeStatus identity 防御语义不弱化（不删 project_mismatch 检查来「修」问题——修复的是上游对齐，不是下游判停）；
- FS 准入门/治理门/预算语义零触碰（227/229 裁定维持）；持久化改动走 §11 兼容义务。

## 停止条件

- 诊断发现权威源在内核侧（agent 无法对账，需内核 id 传播改动）→锚点+两案上交扩域裁定；
- 修复后 levels 仍 null→观察管线更深缺陷锚点转卡；
- 证据齐（identity 对账+观察事实在场）后组 1 仍三连 no_route→模型能力面锚点上交（229 停止条件 3 承接）；
- LLM 面栈不可用→环境中断如实记录。

## 领取

- 领取时间：2026-10-31 18:45（local）；执行 owner：GLM-5.3 PC 执行会话（ZCode，本会话）；机器：PC/Windows
- 领取基线：`origin/main` = `ab2bcde6aa4251e325803148c5408a6ef4f3d96c`（fetch 后核对，与卡面 HEAD 一致）
- 领取时基线状态：实现 worktree 新建自 origin/main，`status --short` 干净、`diff --stat` 为空；主工作树存在他卡/运行工件未提交项（Settings.xml、default_project.xml、coord/runs/* 等非本卡文件域，不触碰）
- 分支/worktree：`port/m3-obs-identity-bind-1` @ `D:/Vit_DAW_wt_231impl`（实现）；`coord/231-claim` @ `D:/Vit_DAW_wt_231coord`（本领取提交）
- 派发确认依据：用户在 PC 执行会话直接派发本卡（池序 231），卡面"发卡即提交，领取按池序"；依赖 230 已合入（ab2bcde6 亲核）

## 回执（2026-10-31，GLM PC 执行会话；卡留 doing 等主管验收）

**实现 commit：9f8dbe07 @ port/m3-obs-identity-bind-1（已 push）**；真栈 run=coord/runs/M3-OBS-IDENTITY-BIND-1/identitybind_1（工件清单见 RESULTS.md）。

### 链路进度（§7 分列一，全部兑现）

1. **诊断定案+单一真相源裁定（目标 1）**：evidence/identity_mint_map.md——四 id 全部内核铸造（agent 零铸造点：boot 装载保留 default id；new_project/save_as 各 forceNew=合法 fork 语义）；断裂根因=agent 三消费面（store 绑定/bake 盖章/观察解析）各自滞后**无对账层**；version_project_* 传播通知由 Godot 前端发，raw ZMQ 生命周期零通知零跟随。裁定：**权威=内核 live edit UUID**，跟随=fork（parent==绑定 id→复用 ForkDerived+ForkAgentStore 再绑定事实）/switch（无关身份只激活）。
2. **修复（目标 3，域内）**：`reconcileProjectIdentityForObservation` 挂 requestMixObservation 顶部（generic 态零内核交互保 read-first 契约；concrete 分歧先刷新后按 parent 判跟随；同 id 异路径再激活；unsaved 不物化 draft；fail-open）+ `ForkDerived` 补树级 RebindProjectTree（acoustic store/快照随行再绑定；manifest 头部 fail-open §11 兼容）。**安全面零触碰**（MergeStatus project_mismatch 防御/ingest 门/FS 准入门/预算语义未改）。单测 8 个全绿（fork 往返 l1 事实非 stale+levels 数值在 ref/switch 不搬运/stale-shadow 自愈/unsaved 跳过/aligned 零命令+ForkDerived 三契约）；§6 健康检查全绿（5 Go+webui 471+AB 冒测 exit 0）。
3. **真栈验证（目标 2+3，ps1 exit 0，kernel f9e3da0a 零漂移）**：
   - identity 收敛亲证：.vit_derived/.vit_agent 各仅一个 id（=save_as 落盘 id 19e078b8）；live 包 **l1 peak_rms=ready rms_dbfs=-11.889**（230 同事实曾 stale）；boot orphan 行如实标 stale（防御语义保留）；
   - **模型面 levels 非 null 亲证**：r3 循环内 track.basic_energy.facts.levels=**rms -11.889/peak -4.188/ready**（rounds/g1_r3.json）；
   - **旅程判定**：agent 重启旅程（chat_auth_stamp 双相+真重启）全绿**不暴露**；save_as 旅程（setup 即 raw-ZMQ save_as 形态）identity 单点收敛**不暴露**；诚实披露：本轮收敛由 chat 激活面时序有利完成，reconcile 对账对齐态零日志行——机制点火由单测覆盖，真栈验证终态。

### 最终验收（§7 分列二，验收门未达→停止条件 3 承接上交）

- **组 1 no_route 消失：未达**（[no_route×3] 三连止损）；**set_points 首绿：未达**（三轮 points 恒 0）；组 2/3 止损设计 skipped。
- **失败形态锚点（新型，非 230 复现）**：每轮自由态循环完整运行（r3 含 basic_energy 观察）→ **limit_reached** → 转异步续跑；复问收 automatic_continuation_already_scheduled 排队文本→判 no_route。**停止条件 3 成立**：证据齐（identity 对账+模型面 levels ready 亲证）+ 三连 no_route → **模型能力/预算-异步续跑边界面上交主管，不再折卡**（候选决策面：排队文本归类/limit_reached 预算与同步采样窗口匹配/异步续跑完成度可观测性）。
- §8 计费：**deepseek-flash 30 调用，418,295in/44,682out**；失败分类 no_route×3（全排队文本形态），零环境中断。
- 真栈登记 7d43dbc9（全零亲核+即时复核）→ 拆栈 TEARDOWN_ALL_ZERO（VitAgent 13888/VitApp 27748 手拆，进程/端口全零）。

## 验收（GLM 主管决策流 / 2026-10-31）

- **实现面+旅程判定 pass 合入 69aec89b**：诊断图谱主管采信（四 id 全内核铸造+agent 三消费面无对账=230 levels null 根因闭环；单一真相源=内核 live edit UUID 裁定成立）；修复面主管复跑 8 单测全绿+build ok+diff 亲核（reconcile 挂点+fail-open+安全面零触碰）；真栈收敛+模型循环内 levels ready（r3 rms/peak 非 null）+双旅程不暴露亲证采信；reconcile 对齐态零点火的诚实披露合格（机制单测覆盖+终态在栈验证）。
- **验收门第三承未达=定性上交设计讨论（不再折卡，230 验收节预承诺兑现）**：主管亲证失败形态=每轮同步窗口仅完成 2-3 工具调用（checkpoint+catalog+observation）即耗尽 10 轮预算→limit_reached→异步续跑排队文本→no_route；**脚本止损+拆栈杀死异步续跑=采样协议与异步执行模型的结构性错配，当前协议对任何模型质量都不可观测成功**。220 协议为词表路由时代（即时确定性路由→即时卡）设计，终态架构的异步执行模型（229 合法化）已超出其观测面。三决策面（排队文本归类/预算-窗口匹配/续跑完成度可观测性）→主管裁定上用户设计讨论，处置选项与推荐见 gate 讨论。
- 卡文件合并事故留档：port 分支与 main 对本卡做了不对称 rename（分支携 doing/ 副本+main 侧 todo→doing rename），merge 把卡落回 todo/ 旧版——主管在推送前修正（恢复 b4e90b87 权威版+删 todo/ 副本），无信息丢失。
