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
