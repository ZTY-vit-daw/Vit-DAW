# FIX-PCA-AUTOSWEEP-1：全库批量认证扫描——一次扫描分类+认证+派生全部可用效果器

- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / PORT-PCA-FULL-CANDIDATES-1 的构建器入仓（本卡复用其派生层）；**用户裁定 2026-09-23"应设计一套认证就能过滤清楚所有能用效果器的机制"**
- 模型分级：L2 / GLM-5.3（分类器规则=族轴拓扑定义，核心设计面）
- **目标：把"哪个插件可用"从人工逐个认证变成机器可回答**——一条命令扫全库：分类 → 批量确定性认证 → 白名单自动派生 → 例外如实报告。
- **背景**：现管线三层割裂——扫描层全自动（语义索引全量在库：PC 719 Waves 主体+PA/FabFilter）、认证层手工逐主体（PCA job：PC 24/mac 27，冰山一角）、派生层刚自动化（FULL-CANDIDATES 构建器）。瓶颈=认证层的手工性；EQ-1 已证明**零 LLM 确定性认证通道**可行（phase3_live_smoke 收据+pcactl import，Q10 即此路径认证）。
- **设计冻结**：
  1. **族分类器（确定性，零 LLM，零名称猜测）**：输入=probe 参数面拓扑+厂商分类；规则=每族实验轴的结构特征——static_eq：≥3 组（频率锚,增益参数）对；broadband：threshold(+ratio/makeup) 单频段；limiter：ceiling 参数存在且无多频段阈值；gate_expander：threshold+range+展开向参数；de_esser：threshold+sibilance 频段结构；transient_shaper：attack+独立 sustain 参数；multiband：≥3 组频段阈值/分频点。一主体可属多族（如实多列）；不匹配任何族轴=非实验类效果器（混响/延迟/仪表等）如实排除。**分类只用结构性参数拓扑，不用厂商命名**（名称仅作 ambiguity 提示不作判据）。
  2. **批量认证扫（零 LLM，复用 EQ-1 确定性通道）**：对分类命中主体逐个跑——identifier 实例化（PLUGIDENT 纪律）→ 该族轴覆盖写（upsert/modify/disable/undo）→ readback → 快照比对 → phase3_live_smoke 式收据 → pass 自动 pcactl import 晋升；fail/歧义进**例外队列**（证据+原因，不认证不硬凑）。幂等可重跑；批次 RUN_LEDGER。
  3. **派生层**：现有构建器（FULL-CANDIDATES 规则冻结版：promoted ∧ 族轴覆盖 ∧ 四源参数锚点）消费扩大后的晋升集 → v6 白名单全候选。
  4. **产物**：`sweep_report` = 全库 → 分类结果/认证通过/例外 三桶清单 + 各族候选计数表——"Waves 包各族到底有多少能用"从此是跑一次就有准数的问题。
- 文件域：`scripts/`（扫描驱动+分类器+报告器，与构建器同域）+ 机器本地工件；零 agent 代码改动预期（认证/装载/白名单机制全复用）
- 验收：①PC 全库扫（719+ 主体）：sweep_report 三桶齐全+各族计数表；②抽样 ≥3 主体覆盖收据核验（含 ≥1 个 Waves 壳内主体走 identifier 实例化）；③派生白名单 overlay 全族回归；④≥1 个新增多候选族真栈自选腿五环证据链；⑤例外队列无静默丢弃（每条有因）；⑥零手编零 LLM 全程（收据可证）
- 停止条件：某族轴分类规则需厂商命名判据才能达可用精度 → 上交裁定（content-blind 边界问题）；认证扫需 LLM 参与才能过覆盖 → 上交（违反零 LLM 冻结）
- 领取：2026-09-24 GLM-5.3（PC 执行侧）。origin/main=6234d2e3（pull --ff-only already up to date）；工作树：M VitApp/Workspace/default_project.xml（运行时状态，非本卡文件域）+ 未跟踪 coord/runs 工件目录若干（他卡产物，不动）+ extension 构建产物 + godot-cpp/。本卡新增 diff 预期仅落 scripts/ 与 coord/runs/FIX-PCA-AUTOSWEEP-1/。
- 回执：（2026-09-24 05:4x CST 自验完成，待决策验收）
  - **一条命令**：`python scripts/pca_autosweep.py --phase all`（分相可续跑：preflight→probe→classify→certify→derive→verify→report；state.json 幂等断点）。run=coord/runs/FIX-PCA-AUTOSWEEP-1/20260924_005622/（ledger/报告/溯源在仓工件；探测快照 970×2+收据在 ~/.vit 机器本地）
  - **探测**：974 效果器主体（乐器 17 排除）→ ok 970 / fail 4（FreeEQ8/Jeesonic EQ Pro/ZL Compressor 文件不存在、RipLink 原生快照无名——各有因）。探测走 pluginprobe 观察宿主（独立进程，零内核依赖）；Waves 壳成员 uid 由 identifier 尾段离线派生（kernel plugin_search 5/5 对表）
  - **分类**（零 LLM 零插件名判据，参数拓扑）：static_eq 68 / broadband 51 / limiter 41 / transient 34 / multiband 33 / de_esser 32 / gate 25（一主体可多族如实多列）；分类器对 19 个已知正例/排除形态校准 19/19 全绿（Waves/FabFilter/PA 三家词汇：Frq/Gate Open/Floor/Sense+Duration/Monitor/前缀带名/尾缀带名/Output-Level+Lookahead 形 ceiling）
  - **认证扫**（零 LLM，EQ-1 确定性通道=HTTP runner 五步+自动 import 晋升）：172 目标 → **53 新晋升**（broadband 31/limiter 14/gate 7/de_esser 1 + 手工通道验证腿 CLA-2A Mono）+ 46 already_promoted + 113 诚实失败（Go 探测器边界码：unsupported_gate_expander/unresolved_limiter_surface/unplanned_parameter_change 等，每条有因入例外队列）。**57 张批次收据 audit llm_call_count=0 全检通过**。栈需内核 plugin list 先扫描（driver 自动 scan_plugins+settled 等待）
  - **裁定②执行**：C1 comp Mono/Stereo 双通道形态在 v1 store 已是 promoted（deterministic_strong_receipt_passed，非盘点时的状态）——扫描如实记 already_promoted；compressor→broadband 映射由分类器落地（C1 comp-gate 双族主体：comp 段 attack=3 与 gate 段 attack=10 是两个物理旋钮，两族独立认证均在库）
  - **派生**：构建器零改动重跑（晋升集=store 文件）。**跨族同角色漂移的诚实处理**：冻结构建器按名收全族收据，comp+gate 一体机双段同名角色 fatal——本卡以**分族收据视图逐族跑构建器、逐族取段缝合**（每段在完整同族收据下全检，S1-S4 校验零弱化，构建器零改动）；发现本身（多族主体同名角色=不同物理参数）记录在案。v5→v6 live 归一化副本作 --live（live 原件未动直至验收④落位）
  - **白名单 v6 全候选**：static_eq 1（S0 承接）+ **broadband 24**（v5 时仅 1）+ limiter 9 + multiband 8 + de_esser 6 + gate 3 + transient 2 = **53 条目**（v5=7）；sha256=bc9fa1a5fef2b41b…（落位 ~/.vit，v5 备份=377f200aab593b3c 在 run 工件）
  - **overlay 全族回归**：pcactl（活 Go admission 路径）56 检查 **0 失败**（53 成员 eligible + 3 非成员 spot 全拒）
  - **验收④五环**：journey run2 **all_green 4/4 exit 0**——模型自选 **Pro-C 2**（本扫新晋升 v1 pca1_ccf143bc9bf73f8）于 24 候选披露中：选择→pca_load_gate 零拒绝→journey_turn_landed→proposal_applied→card_mounted→audition(prepare.started→candidate.ready→ready)+mix_tick；a2 直探针 **CLA-2A Mono**（另一新晋升）rack 实例化+UI 投影落面。run1 a3 红如实记录（模型在观察门前收工，披露已见候选）
  - **收据抽核**：L3 UltraMaximizer Stereo（Waves 壳/limiter，roles threshold+ceiling）+ Pro-C 2（broadband，roles threshold+input_drive）+ AudioTrack Stereo（Waves 壳/gate，roles output_gain+attack）——apply/restore exact、track deleted、snapshot_restored、mismatches=[]、零 LLM
  - **例外队列**：187 条全有因（68 static_eq 无 runner=EQ 通道待后续卡；113 认证边界拒绝；4 破损主体；含 static_eq 68 命中无 runner 的诚实记录——EQ 族认证通道 phase3+pcactl import 属另一张卡的面）
  - **执行中发现并修复的缺陷**：identifier uid 尾段为去前导零可变长 hex（非 8 位），首版 8-hex 正则致 53 主体无 uid 挂起——修复+壳成员 uid 缺失快速失败护栏；agent plugin_search 回包形状（result.plugins）校准
  - **工作树**：新增 scripts/pca_autosweep.py（唯一代码 diff）+ 本卡 coord 工件；栈已停，pluginprobe 残留已清
- 验收：**pass（2026-09-24 PC 决策会话）**——①关键证据决策侧实核：`~/.vit` 白名单 v6/53 条目/sha256 `bc9fa1a5fef2b41b` 与回执逐字吻合、v5 备份在案；sweep_report 三桶+七族表与回执一致（分类 static_eq 68/broadband 51/limiter 41/transient 34/multiband 33/de_esser 32/gate 25，多族如实多列）；53 新晋升+46 already+187 例外全有因采信（68 static_eq 命中但无 EQ 认证通道=诚实留后续卡 **FIX-PCA-EQCHANNEL-1**）。②红线核验：57 批次收据 llm_call_count=0 全检、零手编（五源溯源/双周期零漂移/S4 交叉）、分类器零厂商命名判据（19/19 校准）；跨族同角色漂移的分族收据缝合处理采信（S1-S4 零弱化+构建器零改动，发现记录在案）。③裁定②执行核对：C1 comp 已 promoted 状态如实记 already（盘点时点差异如实），compressor→broadband 映射经分类器落地。④五环证据链采信：journey run2 all_green 4/4（模型 24 候选披露中自选 Pro-C 2，零门拒→落轨→应用→卡挂载→audition+mix_tick）+a2 CLA-2A Mono 直探针；run1 a3 红如实记录（概率族，非断言契约缺陷）。⑤uid 变长 hex 修复+壳成员缺失快速失败护栏=执行中缺陷自修，记录在案。⑥工件入库口径：ledger/报告/溯源/验证入仓，journey 工作区/探测快照/收据/state.json 留机器本地（惯例）。**PC 侧全库候选面就此闭环：7→53 条目**；EQ 族多候选待 EQCHANNEL 卡。
