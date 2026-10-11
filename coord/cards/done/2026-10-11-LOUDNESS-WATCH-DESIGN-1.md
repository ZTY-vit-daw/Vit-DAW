# LOUDNESS-WATCH-DESIGN-1：响度监听（编译器式响度合规门）设计

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=用户功能定义：新 harness 红利——广播/平台响度标准+编译器机制类比+手动开关+用户自定阈值）
- 派发确认：已确认（用户 2026-10-11 晚窗定义）
- 验收负责人：GLM 主管决策流
- 池序 74；目标仓库=D:\Vit_DAW；设计卡零生产代码
- 归属：PC 混音线（D1 响度域）；实现排 M1-4 合入后（观察 pack 为其消费面），设计卡可立即领
- 优先级 / 预估 / 依赖：P2 / 设计 1 天 / 无硬依赖
- 模型分级：L1 / GLM 或 flash 执行会话

## 功能定义（用户原意钉死）

工程音频被 agent 或用户处理后，后台按广播/平台响度标准校验当前响度未越设定范围——**编译器机制类比**：处理=写代码，响度检查=编译/lint，越界=告警。触发三态可选（实时监听/周期/**处理后触发（主案）**——实时若影响性能则退后两者）；**手动开关**；**阈值用户自定**。

## 已就位底座（设计起点，勿重建）

- 标准预设：`internal/rlm/profile.go:44-86`——EBU R128（-23 LUFS±0.5/-1 dBTP/BS.1770）+AES TD1004（-16~-20 LUFS）+LoudnessBand{min/max_lufs} 阈值形态；
- 测量口径：RLM integrated/short_term_max/momentary_max LUFS；内核 L3AcousticAnalyzer+AudioFeatureTypes 响度面；FXM Measurement 含 LUFS/Peak/Crest；
- 绑定面：render.profile.bind（chat/server.go+harness.go，RLM-PROFILE-2 已收）；
- harness 红利：probe 计量账本（PULL-PROBE-METER-1 已合入）+probe 分级+pull 后台探针（G3-REOPEN-COSTCMP-1 将补物理成本证据）。

## 设计五件

1. **触发机制**：主案=处理后触发（mix 动作/settle 完成钩子→自动响度 probe→比对→告警/通过）；周期与实时为可配选项；性能影响评估用 probeSpent 账本证据（引用 G3-REOPEN 对比轮数据若已出）。
2. **配置面**：工程级用户配置——开关（默认关）、标准预设选择（R128/AES/自定义）、min/max LUFS+真峰阈值自定；持久化与旧工程缺省语义（AGENTS §11）。
3. **告警面**：双消费者——agent（自由态循环可见=证据面，模型可据此提降噪/限幅提案）+用户（webui/前端呈现；涉渲染面按 §5 渲染面烟测门槛）。
4. **测量口径核对**：integrated/short-term/momentary+true peak 全口径就位度核对（RLM/内核 L3 现有 vs 缺口）；缺则列内核补齐面（单独实现卡）。
5. **实施拆卡建议**：达到可直接发卡程度（预计 2-3 张：配置面/触发与探针接线/告警双面）。

## 文件域

设计零生产代码；产出落 coord/runs/LOUDNESS-WATCH-DESIGN-1/。

## 验收标准

设计五件齐+锚点（配置/触发/告警三面给到文件:行或 grep 实证）；性能影响结论有账本证据或明确实验设计；实施卡规格可直接发卡。

## 停止条件

发现 RLM profile/绑定面与卡面锚点重大不符 → 上交。

- 领取：2026-10-11 21:46 +0800 / origin/main=8e66e674c331e1d8e8e9337451e3ad1b65de9d3f / owner=GLM-5.3-Flash PC 执行流（ZCode 会话）/ 分支 port/loudness-watch-design-1 / worktree D:/Vit_DAW_wt_loudwatch1（领取时 status 干净、diff --stat 空）/ 领取提交=本提交（coord-only 直推 main）
- 回执：设计基线=[coord/runs/LOUDNESS-WATCH-DESIGN-1/README.md](../../runs/LOUDNESS-WATCH-DESIGN-1/README.md)（设计五件 01-06 全齐+anchor-verification.txt 机械核验 41 项 41 PASS/0 FAIL）；五件摘要=①触发主案=处理后三钩子（settle 桥 `chat/free_state_conclusion_bridge.go:65` 主+动作观察搭车 `harness/harness.go:4279` 零 bake 复用+LatestProjectChange 带诚实边界），周期可配默认关，实时=生产能力缺失配置可存激活报 unavailable ②配置=`loudness_watch_config.v0` 挂 projectAgentRuntimeState（RLM-PROFILE-2 先例 server.go:164/198/7245/7256）默认关+rlm 五内置预设+自定阈值借 ValidateDeliveryProfile ③告警=agent 四披露位+用户 chat 事件/三视图段双消费者同源，grade 强制 approximate 标注 ④口径=近似 integrated+sample peak+crest 就位（全 clip 域），BS.1770 门控/ST/M/TP/母线域五缺口列内核候选卡 KERNEL-LOUDNESS-BS1770-1 ⑤拆卡三张达可直接发卡（A 配置面无依赖可立即发/B 触发接线依赖 A+M1-4+PULL-PROBE-TIER-EXT-1/C 告警双面依赖 B）+内核候选卡登记不发；性能结论=账本机制已接线但 G3-REOPEN 对比轮未跑无物理数字可引（如实），设计侧钉死主案回执延迟增量=0（后台化+纯函数搭车+dedupe+≤4 clip 上限）LLM 增量=0，E1/E2 实验设计全项（06 件）；停止条件核对=无重大不符（DRIFT-1：预设集为卡面超集 67-118 五项，方向有利）；端测边界声明=本卡零生产代码零真栈（卡面明定），烟测门槛不适用，锚点核验以 anchor-verification.txt 实测 grep 为证；交付=本提交（卡 done+设计基线，全 coord/ 路径 coord-only，同 GEN-UNIFY-CTX-DESIGN-1 09e5df45 先例双推 main+port 分支）
- 验收：（裁定文件）
