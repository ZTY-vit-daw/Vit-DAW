# B4-ABLATION-1 判定面报告：B4 首拆 ablation——分支①成立，B4 判 A 态候选（5/5 tool_chain_ok，harness 自主复现成立）

- 卡：B4-ABLATION-1（池序 264，A-F 主线第二张 ablation）｜run：`abrun1`（b4_ablation_20261106_192330，2026-11-06 19:23:30–19:26 +0800）
- 被测：`port/b4-ablation-1` @ 98fc4ca4（Go 缝 2 文件+配对单测+py/ps1 ablation 面）｜kernel=wt_228 `F9E3DA0A…67A` 同款只读复用（19:21 亲核）｜引擎 deepseek-flash（-LlmModelOverride，238-263 先例）
- 判据（RUNBOOK 跑前成文）：success={tool_chain_ok, boundary_ok} ≥4/5；lexicon_hit=钩未达=配置失败（exit 4 INVALID）；钩生效三重验证；两分支判据不预设

## 一、判定结果

| 组（单组） | 话术 | 样本 | verdicts | successes | group_pass |
|---|---|---|---|---|---|
| ablation 目标面 | 分析一下当前工程的低频关系 | 5 | **tool_chain_ok×5** | **5** | **true（≥4/5，超线 5/5）** |

**结论：successes 5/5 ≥4/5 → 卡面判据①成立 → B4 判 A 态候选**（harness 自主复现成立，证据见 §二/§三）。A 态终局确认归主管裁定；若接受，A 态节点数 0→1（终局裁定 §二-2 计量口径）。

## 二、机制面（与 263 组 3 对照——同钩不同入口命运）

263 组 3（裸「30 秒之后动起来」）：钩拆除词表武装 → semantic_entry 仲裁器 conf<0.50 全灭拦截（251 形态）。本 run B4 话术（「分析一下当前工程的低频关系」）：钩拆除 B4 词表+canary dispatch → **semantic_entry 仲裁器放行**（目标/动作/域三要素明确，无需词表武装）→ message loop 进入 → 模型自主执行 `mix.observe`（正是 B4 canary 冷启动同款观察入口——band 分析自动触发面）→ 单轮完成+done。

五轮同型稳定：executed=[mix.observe]，stop_reason=done，goal_status=completed，needs_confirmation=false（只读观察无确认卡，符合 B4 只读语义），零写入（mix_state 前后快照零变更亲证）。

## 三、证据面（r1 全文在档 rounds_slim/r1_response.json）

模型产出为**真实 fixture 数据支撑的低频关系分析**（非模板话术）：

- **频段占用**：Sub Bass Tone 76.6 dB（归一化 1.0）vs Secondary A/B 各 64.8 dB（0.63）——典型低频重叠候选（真实三轨 fixture 的实际测量值）；
- **时间窗**：100Hz 素材 10s（0-10s）vs Secondary 各 3s（0-3s）→ 指出 0-3s 为三轨低频叠加窗口（真实 clip 长度推导）；
- **电平余量**：Secondary 峰值 0 dBFS 余量 0（真实峰值统计），建议先留 2-3 dB 余量；
- **诚实边界**：「L3 深度频段包络、响度汇总、**遮蔽分析尚未就绪**，'哪条轨具体在多少 Hz 互相遮蔽'还不能下结论，只能给出频段级别的重叠判断」——遮蔽深度面如实声明未就绪，不虚构；
- **建议面**：可执行方向（Secondary 低切 60-100Hz 起步/让位 100Hz 以上）+征询确认，未越权执行。

**残差观察项（如实记录，交主管 A 态裁定参考）**：①LLM 轮单轮内 L3 遮蔽深度未就绪即作答（canary 路径有 poll 等待+重问循环，harness 轮以诚实边界收口——能力主面[低频关系观察+建议]复现成立，遮蔽深度判据指引仍是 257 §五 B4 行原缺口项）；②五轮话术为 canonical 同款（263 同话术×N 先例），话术族变体面未扫。

## 四、钩生效验证（三重，全过）

1. ps1 console：`B4 ablation hook: VIT_LOW_END_RELATION_ABLATION=1`+`engine override: VIT_AGENT_LLM_MODEL=deepseek-flash`（dev_agent_smoke_console.txt 在档）；
2. py summary：`ablation_hook: true`；
3. 5 轮 verdict 全集 {tool_chain_ok:5}，**零 lexicon_hit**（任何 workflow=capability_runtime_v1/canary_stage/low_end_summary 形态即 exit 4）——钩确实到达运行中 agent，五轮均为真实 LLM 轮（telemetry 16 调用佐证）。

## 五、零栈验收（开关默认 unset=生产行为不变）

- `go build ./...` exit 0；`go test ./internal/chat -count=1` 全绿（128.1s 存量零回退）；`./internal/agentloop` 全绿（5.8s）；
- 新增配对单测 `TestLowEndRelationAblationHookPair` 三态：默认 unset=词表武装+canary dispatch 保持（handled=true+workflow=capability_runtime_v1）/env=1=入口直通 LLM 零接管（自然话术+显式 capability_id 两面均 handled=false）/仅精确 "1" 生效（""/"0"/"true"/"off" 均不生效）；
- 生产面 diff：capability_owner.go+capability_runtime_canary.go 两文件 29 行（含注释；代码行=env const+helper 8 行+B4 词表分支守卫 1 处+dispatch 守卫 1 处）；py 判定面离线抽查 8/8；ps1 PSParser 零错误。

## 六、§8 运行记录

- 单 run 无重跑兑现：5/5 全 success 无止损触发（stop_loss=null）；无环境中断；**PS1_EXIT=0 §5 门槛过**；
- 计费：16 调用全 deepseek-flash，in 219,358 / out 13,090 ≈ **¥0.9-1.3**（263 换算基准 106 调用 in 1,486,923 ≈¥1.8-2.4），卡面预算 ≈¥2-4 内；
- 占栈 fd 7f2d197b（净空亲核 19:20:56+即时复核 19:23:21）/释放见本报告发布后栈记录更新；owner 亲拆 VitAgent 11644/VitApp 39152，**TEARDOWN_ALL_ZERO 亲核 19:28:36**（五口 TCP 7878/5555/5556/8787/18787=0+UDP 4444/4445=0+VitApp/VitAgent/VspHub/Godot=0+python=0）；
- 工件：`coord/runs/B4-ABLATION-1/abrun1/`（RUNBOOK+summary+双 console+telemetry 20K+prompt_render 276K+rounds_slim 五件，随 port/b4-ablation-1 入库）；大件留盘 worktree `VitApp/Workspace/Artifacts/smoke/b4_ablation_20261106_192330/`（rounds 全量 5×~1MB+payload_capture 900K）。

## 七、透传缝实现声明（卡面文件域预写项的落地形态）

卡面文件域预写「scripts b13b 场景 ablation 参数（py+ps1，263 加参先例）」，落地为：

- **py**=`scripts/b4_low_end_relation_agent_smoke.py` 加 `-AblationHook`+`--rounds`（默认 5）等参数（+207 行含注释：ablation 判定面/止损/工件；默认关=固定面 canary 流程字节不变）；
- **ps1**=`scripts/dev_agent_smoke.ps1` 加 `-B4AblationSmoke` 腿（+82 行含注释：env 窗口[telemetry+引擎注入+VIT_LOW_END_RELATION_ABLATION=1]+隔离块并入+py 透传+exit 码映射；默认关字节不变）；
- **「b13b masking frontier 场景」的实现解释**（执行侧裁量，如实声明）：b13b 腿按 257 §1.2 即「B4 观察面 frequency_relationship+masking」域；本 run 以 B4 低频观察 canonical 话术×5（263 同话术×N 先例）+py 自建三轨低频遮蔽 fixture（Sub Bass 100Hz+双 Secondary，b4 py 固定面同款）落地该场景域。`b13b_frontier_masking_smoke.ps1` 本体零改动——其话术（「帮低音轨做个均衡实验」）不含 B4 词表标记、与钩正交，改之无判定意义；257 未跑的 b13b frontier E2E 腿是否补跑归主管另行裁量（本卡判定面不依赖）。

## 八、移交主管事项

1. **A 态裁定**：判据①字面满足（5/5≥4/5），证据面成立（真实数据+诚实边界+零写入）；残差观察项（§三-①②）是否阻断 A 态确认请裁定；
2. 钩保留形态：与 263 VIT_AUTOMATION_INTENT_ABLATION 同款 test-only env，默认 unset 字节不变——后续族 ablation（B2/B3/C1 等）可复用同模式；
3. 257 拆减矩阵 B4 行更新建议：现三态 F→（本裁定后）A 候选；「低频遮蔽判据指引」缺口项在 harness 轮以诚实边界形态呈现（§三-①），维持为指引面待办非阻断项。
