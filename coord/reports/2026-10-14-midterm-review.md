# 2026-10-14 中期盘点（窗口首日）——12 月总锚第一次校准

> 产出：GLM 主管决策侧 / 早窗 /morning（用户未答复日型确认，按 10-13 gate §4 明日建议序缺省执行：标准日=腿 D+盘点为主）。
> 证据源：queue/reports/2026-10-10~13-gate.md、decision-log 2026-10-11~13、coord/cards/ 现态、PC-RUNTIME-STACK.md、main=459baeed。
> 性质：状态盘点与登记，不做重排；盘点窗口 10-14/15 两日，次日可补裁定。

## 一、八项备料逐项

### 1. 挂账状态（三笔 exit 0）

- 挂账：M2-TIMEFX-1（run 111841）/ M2-TIMEFX-2（run 195448）/ FIX-106——三者销账条件同源=D1 reverb 冒测 exit 0。
- 当前链路位置（109 rerun2 亲核）：**M2 全线（含 gpt-6.1-sol 时代）首次真栈抵达执行后 settle 段**——域选择→提案→准入→装载→干预 applied（readback 0.2427/guitar 轨）→post-action 重观测全通；死点=settle 段 `experiment_materiality` schema 形状非法（state=""/attempt≤0）被 final gate 正确拒收，且拒收消息不教合法形状→续轮瞎猜烧尽预算。
- 销账路径：**109 腿 D**（卡内扩域=拒收消息携带有界 schema 教学+单测钉）→ flash 思考档复跑 exit 0 → 三账同销 → M2 体验场次解锁（用户手测）。失败梯子：同签名 2/2 → 升 v4-pro；再 2/2 → 菜单③（按单元证据采信）+G6 取证卡。
- 腿 D 提示词已在用户手，待粘贴派执行会话（worktree fsdc1r2 在用）。

### 2. 模型基线登记（正式）

- 现行基线=**deepseek-flash 思考档（正名）**，`~/.vit/config.json` defaultModel=deepseek-flash @ api.deepseek.com/v1。
- 迁移经过：gpt-6.1-sol（服务过载+额度中断）→ 用户裁定改 deepseek 官流（10-13）→ chat 旧名误判（PC 决策侧依 100-token 探针错认领，用户质询"绝对是 flash"触发差分探测纠正）→ 正名 flash 实测带思考档（45 reasoning tokens）。
- 两次决策侧自纠在卷：chat=旧名无思考别名；"服务端档位漂移"归因撤回（别名回显非漂移证据）。
- 已知风险：思考档 60s 整超时穿透（1-3/10 次 transient 自恢复）；**腿 D 后仍高频 → 评估 reasoning_effort 接线（小客户端改动）**。

### 3. M3 步度（自动化线，五步骨架在 M3-RECON-1 卡面）

- 步骤 1 M3-RECON-1：done pass（10-12）——tracktion automation 能力完整在场（submodule b4397498），VitApp 命令层无载体；§六遗留六裁定=步骤 2 拆卡输入。
- 步骤 2 M3-KERNEL-1：done pass 合入（10-12 深夜）——automation 命令族 13 检+95 包 0 FAIL。
- 步骤 3（包络车道 UI+机架状态面）/ 步骤 4（agent 范围 goal 编舞单轮终态+意图层包络指令）：**待拆卡**（决策侧立卡活，10-14 视精力）。注：步骤 3/4 文件域与 109（free_state_experiment_runtime/goalrunner_chat）在 goalrunner 意图面有交集窗口，拆卡时注明排后。
- 步骤 5（后段快照统摄层）：未动，先曲线后骨架再统摄。

### 4. GENSHELL 六卡（生成线，Mac 主导）

- 设计卡 GEN-PLUGIN-SHELL-DESIGN-1：done（10-11，Mac 线交付）。
- GENSHELL-CPP-1 骨架：done（10-12）**待 Mac 验收流**。
- 在池五卡：GUI-1 / HOST-1 / PCA-1 / AGENT-1 / AUDITION-1——排序权归 Mac 决策侧自定（10-12 用户裁定）。
- 前置链已就绪：KEYFIELD / INJECT / CONSUME 均闭环。

### 5. G3 复评（挂 10-14/15）

- 终判=维持 push 缺省（QUOTA-RECOVERY 腿②：push 10 vs pull 6 调用，quota_blocked=0；样本薄+pull 侧一轮 checkpoint persist fail）。
- 复评动作：确认是否需要新样本或维持；无新证据则维持并销复评项。

### 6. auto-persist 定级（跨线）

- SETTEMPO-101 已收口（10-13 pass）：内核静默跳写族=差分证据链（基线 exit1 实锤+修复 exit0 八检）；差分冒测腿进 SMOKE_TESTS 常备。
- 待定级=auto-persist 明文重写跨线口径（10-12 gate §3-6 挂账）：哪些内核写路径走 auto-persist、哪些保持明文 save。建议处置：并入 M3 步骤 3 拆卡输入（UI 面写路径定级天然耦合）或单独小卡，10-15 前定。

### 7. 编曲线解冻条件

- 第三线后置；解冻条件=**C4+段落投影消费链就位**（10-10 gate §3-6）。
- 现状：C4 未动，维持冻结。用户笔记过滤标准（LLM 参量 vs 经验逻辑，10-12 确立）已在卷，启动时引用。

### 8. worktree 卫生

- fsdc1（B-i 已并入 main）：**可拆**——腿 D 开跑前拆不影响（腿 D 在 fsdc1r2）。
- fsdc1r2：腿 D 在用，保留。
- fix106：保留至三笔挂账销账。
- 历史存量（盘点登记不清理）：codex worktrees ×6、claude ×1、jsss1、notestream2、region 意图双卡、webui 双卡、startfeed1 等——清理另排卫生项。

## 二、12 月总锚视角（第一次校准，仅登记）

- 11 月评估点输入：judgment_ok 双零（10-10 登记，11 月评估点消费）。
- DEFENSE-SHOW-1：11 月占位；预答辩节目设计用户 10-10 晚已启动讨论。
- 关键路径到 11 月：腿 D 销账 → M2 体验场次（用户手测）→ FE-GUI Phase B/C → M3 步骤 3/4 实施 → GENSHELL Mac 线 → FE 目检清账（FE-GUI 手测 7 条中 1 条挂账+RACK-LIST 6+START-FEED 6 待用户排期）。
- 风险在档：Suno 反代风险；SMOKE A2/A3 汇合点维持；hejuapi 未恢复（探测确认）。

## 三、池/栈终态（2026-10-14 早窗）

- main=459baeed；真栈=空闲；doing=1（109 腿 D 待派）；blocked=1（108 等 109 收口后复验销账）。
- PC 可执行余卡（早窗勘误补全）：110（FIX-CARRIES-REPORT-TIGHTEN-1）/ FE-GUI-B(91) / FE-GUI-C(92，依赖 91) / LOUDNESS-B-TRIGGER(90，前置 A 已 pass) / HARNESS-STRUCT-NORMALIZE(53) / FS-BLOCKED-SURFACE-CLOSURE(54)——池深充足；FE-MOJIBAKE（Mac 线，待用户确认派发）；FS-RECEIPT-REVISION(51) 领取前须核对 FS-LEDGER-PERSIST-1 合入状态。注：53/54 为 10-10 后端小卡批存量，实际派发前由主管复核卡面前提是否仍成立。
- 今日池序（早窗校准，晚窗参考序标在各卡头）：①腿 D 首件 ②中期盘点（本文档）③M3 步骤 3/4 拆卡视精力 ④110 晚窗起步参考①（纯单测不占栈，可与腿 D 并行）/ FE-GUI-B ②（前端仓域不相交，栈腿避让 109 收口时段）。
