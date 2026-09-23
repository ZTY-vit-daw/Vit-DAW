# PORT-PLUGIN-SELECT-MAC-1：mac 白名单 v6 升级四 EQ 候选 + LLM 自选验证腿

- 优先级 / 预估 / 依赖：P1 / 0.3-0.5 天 / **FIX-PLUGIN-SELECT-1 合入**（v6 schema+membership 准入+有界披露）；操作序列与验证口径已由实现侧备好：`coord/runs/plugin_select1_artifacts/mac_leg_handoff.md`
- 模型分级：L1 / GLM-5.3 flash 可接（数据扩展+驱动+记账，断言零改动）
- **执行侧（mac 侧卡）**：白名单与工件均机器本地（`~/.vit/free_state_experiment_plugins.json`、`~/Documents/` 工件根）；回执写回 D:\Vit_DAW coord（记两端 HEAD）
- **背景**：PLUGIDENT 已通（EQ 全链 mac 首次走通）；PLUGIN-SELECT 已合入——v6 每族候选列表+候选>1 时模型有界披露自选+membership 准入。mac 已有 **4 个 PCA 认证 EQ 主体**（Q10 Stereo + API-550A + API-560 + EMO-F2，EQ-1 期间晋升）——正是多候选自选的现成试验场。
- 目标：
  1. **白名单 v6 化**：`static_eq` 升为数组并填入四个认证主体条目——条目字段与 v5 逐字段同（plugin_name/manufacturer/format/plugin_identifier/plugin_path/bands[center_hz+gain_param_id_ch1/ch2]）；**四源零手编纪律保持**：Q10 条目 EQ-1 已建可直接入列；API-550A/API-560/EMO-F2 若 bands 参数面数据不全（认证覆盖≠带结构参数面完备），先跑同款 probe 补测，实在不可及的主体**如实少列**（候选 2-3 个也足以验证自选），不得手编凑数；其余五族维持 v5 单对象形态（loader 兼容阀=行为逐字节不变）
  2. **加载与准入校验**：agent 启动读 v6 无错 + overlay go test（loader v5/v6 双形态+四条目 static_eq admission + 其余五族回归）
  3. **LLM 自选验证腿（本卡核心交付）**：按 `mac_leg_handoff.md` 操作序列跑——模型在候选披露面看到 ≥2 个 EQ 主体 → 提案携带所选 plugin_identifier → 准入 membership 放行 → D1 绑定所选条目 → 装载/写参/回读走完；**断言要点**：披露出现且仅含结构性字段（name/manufacturer/format/identifier+能力面，无 path/参数 id）、所选 identifier 进入计划 args、非成员 pin 拒绝路径 spot 一枚（如可行）
  4. 工件：RUN_LEDGER 式逐轮账目（run ID/退出码/所选主体/披露形态），LLM key 零入工件
- §8：真栈 1-2 轮（存在性+自选实证口径）；模型若两轮选同一主体=合法（自选≠必选不同），记录选择分布即可；同断点两败止损上交
- 验收：①v6 白名单+溯源（每条目↔实测/认证来源可回指）②overlay 绿 ③自选腿 exit 0 且证据链完整（披露→选择→准入→绑定→写链）④回执记两端 HEAD+白名单文件版本/哈希
- 停止条件：候选披露面不出现（>1 候选在列仍无披露）→ 取证上交（可能是 PC 实现缺陷，域外）；四主体参数面均不可及 → 如实降级为现有五族任一族的双候选试验（需补测该族第二主体）
- 领取：2026-09-23 21:20 / origin/main=1d065b9（=发卡基线，fetch 后零入站提交，pull --rebase 因残留被拒按先例核实无操作） / 无实现分支（零仓库代码改动预期，coord-only）；领取时 HEAD=1d065b9、status=两 Workspace 运行时残留（Settings.xml/default_project.xml，不碰）+ done 卡 FIX-FE-SCANPATH-MAC-1 回填遗留（ruling 已入库而卡内行漏暂存，与 1d065b9 同类时序事故，记录不擅动）+ .zcodeignore/VitApp/Workspace/Artifacts/ 未跟踪；卡面引用 mac_leg_handoff.md 未入库（PC 漏提交已登记），按卡面目标 1-4 自足执行
- 回执：2026-09-23 22:0x CST，自验通过 mv done。**零仓库代码改动达成**（HEAD=189f77d 全程未动，git status=领取时残留原样）。工件根 `~/Documents/vit-pluginselect-mac-artifacts/`（RUN_LEDGER/探测/probe/build_whitelist_v6.py/溯源表/v5 备份/verify/journey 两轮）。两端 HEAD：决策侧发卡=1d065b9（领取中 4b758fe 补推 mac_leg_handoff.md 已入库并按其口径执行）；执行侧=189f77d（领取提交，1d065b9 后零代码差量）。白名单：mac v6 sha256=38176f05a15e37a57c3ebe2d381b36ce779d86c93a195025c3488f1f9fcbf048（PC 端=v5 单候选，未动）。v5 备份=同目录 free_state_experiment_plugins.v5.bak.json sha256=2b893323…（演示安全网）。
  - **①v6 白名单+溯源**：static_eq 三候选数组=[Q10 Stereo（EQ-1 v5 条目逐字段复用）, API-550A Stereo（3 带 50/400/5000Hz，S3 双周零漂+S4 回执 gain=2 锚定）, API-560 Stereo（10 带 31-16k，中心频率源自参数名，S4 回执 gain=4 锚定）]；**EMO-F2 如实不入列**（非 MIDI 表面 9 参数零增益参数——Link Mode+Hpf/Lpf On/Freq×L/R 纯滤波器，v6 条目 gain_param_id 无诚实值；其 PCA 覆盖本为 low_cut 非增益形），候选 3 个仍满足自选验证；其余五族 v5 单对象原样（loader 兼容阀回归绿）。溯源表 provenance_table_pluginselect.json 56 行四源可回指（S0 活 v5 哈希钉死/S1 语义索引/S2 attestation 晋升交叉核对/S3 本卡双周探测/S4 EQ-1 认证回执）。
  - **②overlay 绿**：verify/verify_run.log `go test -overlay` 零源码树写入全 PASS——三候选 membership 各自解析+活 v1 store admission PASS、空 pin 拒绝（"requires a pinned plugin_identifier to select among 3"）、**非成员 pin fail-closed spot**（伪造 identifier→"admits one of"+点名三成员集）、550A/560 nearest-band 与 S4 锚点吻合、五族 v5 形态回归 admission PASS、v5 备份过兼容阀（单候选空 pin 可解析）；仓库既有锚点 experimentplugins 全包+chat PluginSelect/D1Plan 系+agentloop 披露三测全绿。
  - **③自选腿 exit 0 证据链五环**（run2_20260923-214216 all_green 9/9）：轮 1（run1_20260923-212931 assertions_red 7/9）=模型随机分支前提案轮预算耗尽（首答占位文本未返回 needs_experiment，非准入防御签名非断言红），按 §8 记账加跑；轮 2 五环：披露 6 次 PLUGIN CANDIDATES 三候选结构性字段零泄漏→模型 needs_experiment 提案 parameter_bounds.plugin_identifier=Q10（披露集内逐字回显=自选）→零 membership 拒绝签名→FrozenPlan args 绑定 Q10+param 12/12（120Hz→Q10 表 125Hz 带）→rack_add_node ok+intervention applied+applied_revision+audition.ready。纪律核查：22 条送达文本零插件身份泄漏；模型选择分布：run2 选 Q10（run1 未达提案）。
  - **偏差声明**：handoff"原命令"为全默认（含内核临时构建）；本卡 --kernel-bin 复用 EQ-1 旅程内核（sha256=a94578fa…，59a6cc8..189f77d VitApp/ 零差量核实，libzmq FetchContent 被墙改道先例决策侧已认可）+--workdir/--output 落工件根；agent 每轮 HEAD=189f77d 新建（含 c832530）。LLM key 零入工件（deepseek-v4-flash，config 默认）。
  - 端测边界：旅程断言面=agent HTTP 面 9 断言+对话事件流（渲染面/用户旅程门槛层不在本卡范围，沿 EQ-1 同边界）；非成员拒绝走 overlay 单测覆盖（卡面许可，未强求真栈）。
- 验收：
