# FS-LARGEPROJECT-SMOKE-1：超大真实工程自由态烟测（sattelites 61 轨首发，pull 模式，全曲混音判断→AB 试听→零固定编排）

- 发卡：GLM 主管决策侧 / 2026-10-10
- 派发确认：已确认（用户 2026-10-10 口径裁定：验收=行为面三断言，见"验收标准"——非 push 对照双臂设计）
- 验收负责人：GLM 主管决策流
- 池序 45；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 1 天 / 无硬依赖（PULL-PROBE-METER-1 并行中，合入后成本面有读数；不阻塞本卡）；**真栈独占**（PC-RUNTIME-STACK 登记）
- 模型分级：L2 / GLM 执行会话 + **独立复核腿**（L2 标准配置，A 链范式）

## 目标（用户 2026-10-10 验收口径）

超大真实工程上，pull 模式自由态全链路烟测：**全曲"检查一下当前工程有什么问题"级问题 → 触发自由态时期同款输出（诊断轮/观察/改善提案）→ AB 试听链路走通 → 全程零固定编排调用**。

## 工程与素材

- 首发：**sattelites**（`E:\BaiduNetdiskDownload\yingge - sattelites tracks out`，61 轨 3.8GB，L2-2-SEG-SMOKE-1 已有全量导入实证+管线基线）；第二样本 Weekend Lover（104 轨）与长河分轨（93 轨，嵌套子目录）在本卡 PASS 后另立扩样本卡。
- **§10 红线**：E:\ 源目录只读权威素材——导入走 new_project → save_as 隔离工程 → import_preflight → import_folder_as_stems（L2-2 先例命令链），一切写入落隔离工作区。

## 脚本（AGENTS §5：复用 ps1 体系）

`scripts/fs_largeproject_smoke.ps1`（同模式新增，参照 `seg_primitives_smoke.ps1`/stems import 体系）：拉起真实三件套（-StartKernel -StartUI）→ 隔离工程+61 轨全量导入 → **bake 预热段**（L2-2 实测：每 clip 一个整曲 L3 bake、单线程串行池积压数十分钟——脚本须显式预算等待或预热轮询，预算耗尽=环境中断分类非功能失败）→ 设 pull 模式 env（双模入口接线面，**不走 chat 直连**——OQ-H3 未迁移）→ goal 入口注入任务话术（"检查一下当前工程有什么问题"级）→ 真实 LLM 自由态运行 → 断言组 → 工件落盘。

## 验收标准（exit 0 = 四断言组全过）

1. **模式/遥测面**（一行检查非对照臂）：run 遥测含 `source=pullharness`+前缀指纹键——确认在测 pull。
2. **自由态站图**：诊断轮/CCB 观察/改善提案或 mix tick 候选在事件流可指认——与自由态时期同款输出形态（站图锚点参照 G3-ATTRIB-1 站图法）。
3. **AB 试听往返**：确认卡 → approve → audition → `mix_tick_applied_reobserved` → completed 事件链（断言模板=G3 run `harness_ab_confirm_roundtrip.json` 先例）。
4. **零固定编排**：全程无 C1/C2/B4 runtime 激活标记（semantic_eq_batch / c2.dynamic_plugin_load_batch 回执族、c1/b4/c2 runtime 事件与日志标记零命中）——fastpath preflight 不在此列（FastPathRouter 是新 harness 组成，允许）。

## 概率运行纪律（AGENTS §8/§9，执行前写明）

- 运行次数 ≤2；成功=四断言组全过 exit 0；失败分类分记：no_candidate_found / capability_blocked / llm_error / budget_exhausted / 环境中断（ bake 积压超时、端口冲突等——须有原始日志佐证）；同一确定性断点连续两败 → 停止上交不重跑。
- 预算显式配置：max_cycles 给足长程预算（61 轨全曲判断是多轮任务）；budget_exhausted 若触发=合法终态分类，但断言组 2/3 须在止损前已成立方算部分证据（如实分记"止损前站图"与"全链完成"）。
- 回执按 §9：完整命令/退出码/run ID/HEAD/工作树状态/agent 原始日志/关键开关。

## 已知真栈形态（L2-2 情报，脚本设计须吸收）

① L3 bake 串行积压（见上）；② AudioFeatureService 30s merge 窗口静默去重同 key 请求——重复观测隔 ≥30s 或变 key；③ agent 落盘白名单只认 band/stereo/loudness 三型——观察面断言按此现实设计，勿依赖白名单外特征落盘；④ **启动前清场检查**（BOUNDARY-PERSIST-1 观察项 B 并入）：脚本拉栈前确认无残留 agent/内核进程——push→pull 双进程并存窗口是 runtime state 租约竞争温床（2026-10-09 run 实证），必须先清场再起栈。

## 文件域

`scripts/fs_largeproject_smoke.ps1`（新增）+ 工件目录 `coord/runs/FS-LARGEPROJECT-SMOKE-1/`；零 agent/Go 代码改动（发现缺陷→锚点+最小复现上交，不本卡私修）。

## 停止条件

- pull 双模入口对 goal 长程任务存在接线缺口（如任务在 pull 下无法进自由态循环）→ 锚点+形态上交（这是高价值发现，正是本烟测的目的之一）；
- 61 轨导入/bake 在预热预算内无法就绪 → 环境中断分记，可申请放宽预算一轮；
- 断言组 2 自由态站图与既有形态系统性不符（非概率波动）→ 取证上交。

## 并行与资源

真栈独占（PC-RUNTIME-STACK 登记唯一 owner）；与 PULL-PROBE-METER-1/M4B/M5（纯单测域）并行安全；E: 盘源素材只读。

- 领取：（时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交）
- 回执：（run ID / 退出码 / 四断言组结果 / 失败分记 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
