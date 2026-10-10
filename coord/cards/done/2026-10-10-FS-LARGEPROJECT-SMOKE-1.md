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

- 领取：2026-10-10 晚 / origin/main=3cd4efa404dfe57c294a569ed6f170dd3802cfd7 / owner=GLM-5.3（PC 执行会话，ZCode D:\Vit_DAW） / 分支=port/fs-largeproject-smoke-1 / worktree=D:/Vit_DAW_wt_fsLPS1 / 领取提交=d4ee5f2a（coord checkout D:/Vit_DAW_wt_fsLPS1_coord 推 main）；doing 四卡（FE-RACK-CTX/HYGIENE-GOALPERSIST/PULL-PROBE-METER/REFSCHEMA-M4B）文件域与本卡（scripts/fs_largeproject_smoke.ps1 + coord/runs/FS-LARGEPROJECT-SMOKE-1/）不相交；真栈独占登记=de1d441b
- 回执（2026-10-10 晚，实现 commit=2291d055@port/fs-largeproject-smoke-1 已推 origin）：
  - **verdict=FAIL（exit 1），功能轮 2/2 预算用尽后停止**；run ID=fs_largeproject_smoke_20261010_185519（末轮，完整跑通脚本全流程+teardown 验证）；另一功能轮=20261010_184500（LLM 轮完成、A1 成立后死于脚本 $pid 崩溃，RUN_NOTE 保全）；另有两次工具性中止 183305/183914（零 LLM 轮，ABORTED_NOTE 保全，不计入 §8 概率预算）。
  - **四断言组**：A1 模式/遥测面 **pass**（两轮各 3/4 条 source=pullharness 记录全带 prompt_fingerprint，prefix_bytes 17827 四轮稳定——61 轨大工程上 pull 前缀稳定性成立）；A2 自由态站图 **fail**（两轮 trajectory.round/observation/hypothesis/mix_tick 事件零发射）；A3 AB 试听往返 **fail**（无确认卡→无 approve→无 reobserved）；A4 零固定编排 **pass**（五标记族在响应/事件/agent log 三面零命中）。
  - **失败分记（诚实口径）**：run 184500=model_protocol_failure@processor_selection（infra_error_llm 类）；run 185519=capability_blocked@准入门（receipt status=capability_blocked，G3-G8 fail/G1-G2 pass）。两轮断点不同→止损线未触发；≤2 预算用尽即停。机器 classification="other" 与人工分类分歧已声明：脚本分类阶梯未消费 fs_loop_admission_status（原始字段 run_report.fs_loop_admission_status=capability_blocked 可回溯；hygiene 另立卡）。
  - **高价值发现（独立复核腿修正版，REVIEW-independent.md §F）**：①61 轨真实工程上模型**请求并获得了** mix.multitrack_relationship 部分交付（28.5KB MOM 事实，obs_20261010T105649，物理工件 .vit_agent/observations/ 已入库），但该回执**未存活到终态观察台账**（receipt_count=1 仅 track 级）且 fs_loop cycle=0 与 ≥4 模型轮矛盾→**跨轮台账持久化/循环激活时序缺陷**（锚点 free_state_reasoning_loop.go:817-840 合并路径；free_state_gate.go:206-215——若 mix 回执在台账，G3 本应 pass，即"有合法已交付全曲 scan 收据时准入门仍误杀"）；②receipt project_revision="" 窄回退缺陷（:738 未用 :1865-1871 宽回退链，revision"4"实际在 freshness.project_revision）；③blocked receipt 未接到响应 surface（顶层 done+completed+workflow 缺席，FS-CAPABILITY-BLOCKED-SURFACE-1 边界面在 fs2_capacity_assessed 未接合）；④61 轨导入+bake 预热实测 **30s 就绪**（dad 61/61 ready，122 feature jobs）——L2-2"数十分钟"预期修正为 pcverify1+自有 bake 干扰形态，staging 内核 + 纯后台队列无此问题。
  - **§9 回执面**：完整命令（powershell -NoProfile -ExecutionPolicy Bypass -File scripts/fs_largeproject_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_fsLPS1）、退出码 1、HEAD=3cd4efa4、kernel sha=B6565DCF85D1DA86（G3 同源）、agent 现建非 SkipBuild、E:\ 三 manifest 逐字节一致（61 文件 name/size/mtime 全等）、preheat 轮询史/telemetry census/断言块/teardown(torn_down=true) 俱在 run_report.json；缺口三条（command_line 字段空/exit code 未数值化/agent_binary 死字段）已记入复核报告 §D。
  - **端测边界声明**：本烟测=自动化脚本栈端侧门（AGENTS §5），非用户手测；覆盖=agent HTTP 面+事件流+遥测+内核工具面，**不覆盖** webui 渲染面与用户旅程（本卡零 webui 改动）；两轮均单轮 turn 即终局，未触发 nudge/continuation 面（run 185519 done+completed 即停），多轮长程续跑面未被本采样覆盖。
  - 工件根=coord/runs/FS-LARGEPROJECT-SMOKE-1/（四 run 目录+REVIEW-independent.md）；栈释放记录随本批推 main。
- 验收：**执行 pass+verdict FAIL 正式登记（2026-10-10 晚窗主管）**——[rulings/2026-10-10-EVENING-BATCH-rulings.md](../../rulings/2026-10-10-EVENING-BATCH-rulings.md) §5；cherry-pick 2291d055→main 4ccb02dd（脚本+四 run 工件入库）。执行纪律全合规（≤2 轮/分记诚实/E:\ 三 manifest 一致/栈释放闭环）；A1/A4 pass、A2/A3 fail；高价值发现采信复核修正版归因（mix 收据未存活到终态台账+cycle=0→跨轮持久化/激活时序缺陷=G3 误杀）——**汇合点 blocker 立 FS-LEDGER-PERSIST-1（P1）**，修复+SMOKE 回归通过前三线不启动；后续卡另三张（SCRIPT-HYGIENE-1/RECEIPT-REVISION-1/PROBE-TIER-EXT-1）。
