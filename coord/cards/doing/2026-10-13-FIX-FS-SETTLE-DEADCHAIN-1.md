# FIX-FS-SETTLE-DEADCHAIN-1：空态 settle 拒收后已准入实验死链修复（108 收口腿解锁）

- 发卡：GLM 主管决策侧 / 2026-10-13 深夜（依据=[TIMEFX-D1-EXIT0-DEEPSEEK-1 blocked 上交](../blocked/2026-10-13-TIMEFX-D1-EXIT0-DEEPSEEK-1.md)+[FORENSICS.md §四](../../runs/TIMEFX-D1-EXIT0-DEEPSEEK-1/FORENSICS.md)修复菜单裁定）
- 派发确认：已确认（主管依已核实上交证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 109；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 108 适配已合入 main（本卡收口腿即 108 遗留）
- 模型分级：L2 / GLM 执行会话（PC，agent 面+真栈腿）

## 缺陷机制（已核实，FORENSICS §三）

deepseek-chat 末轮提案 JSON 同时携带空态 `experiment_materiality={state:none,evaluation:not_ready}` 且 `final=true`（模型幻觉"已应用"，工程实际零插件）→ `free_state_experiment_runtime.go:649` 按字段在场判结算企图拒收 → `goalrunner_chat.go:3119` owed 扣留后不再 arm（lifecycle=none）→ 已准入实验（reverb_decay_adjust）物化/装载/执行链不发生 → interventions=0 → exit 1 "exactly one forward mutation"。两轮同断点（run 231930/232817，工件在 main）。

## 修复方向（裁定=B 主修，seam 由执行侧取证后二选一并写明依据）

- **B-i（谓词精化）**：`settleReportCarried` 不把空态 materiality（state=none 且 evaluation=not_ready 且无 target_response/round_decision）计为结算企图——字段在场≠结算声明。
- **B-ii（恢复臂）**：owed 扣留后对已准入未应用实验 arm 一次物化 continuation。
- **硬约束**：不得弱化证据门——真实 settle 仍需非空 materiality；幻觉 settle 拒收行为保持；只允许"死链→驱动已准入实验执行"的恢复语义。
- 可选硬化腿 C（若并入须注明概率性且不触盲法）：提案轮提示明令禁填 settle 三字段。A（驱动器 nudge）不并入，B 落地后评估。

## 任务

1. 取证复核（短）：读两锚点代码定 seam，B-i/B-ii 二选一并写设计依据。
2. 修复+回归测试：空态 final=true 反例——链不死（interventions≠0 路径恢复），真实空态 settle 拒收语义有测试钉住。
3. 真栈收口：`run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` **exit 0**（建议连过 2 轮：销账+体验场次稳定性）；delay 族可选加分。需登记栈占用；LLM=deepseek-chat（勿改 flash）。
4. `go test ./... -count=1` 0 FAIL+gofmt 净（LF blob 口径）。

## 文件域

agent/internal/free_state_experiment_runtime.go + agent/internal/goalrunner（goalrunner_chat.go）+ 同包回归测试；可选提示组装面（C 腿）；领取时核对 doing 卡。

## 验收标准

diff 亲读+seam 设计依据+回归测试+95 包 0 FAIL+D1 reverb exit 0（工件含 console+summary+模型口径行）；销三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）由决策侧复核后补记。

## 停止条件

B 两半均与证据门语义冲突（修不动）→ 带设计分析上交；域外新发现→上交不扩域。

- 领取：2026-10-13 23:54 +0800 / origin/main=f937a84d（doing/ 空、无文件域冲突；栈空闲已核）/ GLM-5.3 L2 执行会话（PC，ZCode）/ port/fix-fs-settle-deadchain-1 / D:/Vit_DAW_wt_fsdc1 / 领取提交 c664ea94
- 回执：commit=abd65f24（B-i）+aee05996（C 腿）+8e85a820（agent 日志补卷），port 分支已推 origin；run=20261014_000822/001259（B-i 版 A0EA8B1F）+20261014_003339/003935（B-i+C 版 94CBFA68），四轮 exit 1 **全死于准入前、settle seam 未被真栈触达**；seam 依据/证据链见 [coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/REPORT.md](../../runs/FIX-FS-SETTLE-DEADCHAIN-1/REPORT.md)；`go test ./... -count=1` 95 包 0 FAIL+gofmt 净（LF blob）；占用/释放已登记（0854e899/c1be027a）。端测边界声明：真栈覆盖=域选择→提案→准入失败边界 4 轮；未覆盖=准入后路径（物化/settle/试听）因模型档位墙未触达。
- **blocked 上交点（2026-10-14 00:50）**：服务端模型档位漂移——api.deepseek.com /models 现仅 {deepseek-flash, deepseek-v4-pro}，deepseek-chat 请求被别名 flash（探针回显实锤，config 未动）；flash 档下四轮全死于准入前（run1/2=占位三件套+target 错配+final；run3/4=提案已干净但 G6 轨道级观察不恢复，同签名 2/2 §8 停）。exit 0 销账在当前服务档位下不可达，需决策侧裁定：①授权改配置 deepseek-v4-pro 复跑收口腿（恢复基线档位）②等服务端恢复③按单元证据采信 B-i/C 代码+NOT_EXERCISED 类收口。域外发现一并上交：carriesExperimentReport（free_state_reasoning_loop.go:1047）对"实验未存在+占位三件套"的审计跳过是潜在吞链面（C 腿已提示面缓解并真栈验证生效），建议新卡收紧为 loop.Experiment!=nil 才跳过。
- 验收（主管裁定 2026-10-14 凌晨）：**B-i+C 代码采信合入**（merge 随本批；diff 亲读=B-i 豁免范围收纯空态对+零干预轮证据门不动+C 腿去骨架示例 settle 字段；95 包 0 FAIL+目标测试亲跑）。**收口腿裁定=菜单①**：模型名修正+档位换 deepseek-v4-pro（/models 在册清单亲核：仅 flash/v4-pro，chat 旧名被别名 flash——PC 决策侧先前依 100-token 探测误判 chat 为非推理独立档并改掉用户原填 flash，此错认领在卷；config 已切 v4-pro 探测过）。复跑提示词已交用户；若 v4-pro 下仍同签名 2/2 → 转菜单③（按单元证据采信+G6 停滞另立取证卡）。域外发现采纳=立 FIX-CARRIES-REPORT-TIGHTEN-1（110）。卡维持 blocked 至收口腿 exit 0。

- 验收修正（2026-10-14 凌晨二，用户质询触发）：决策侧差分探测三发——chat 别名→flash **无思考档**（1 completion token 零 reasoning，与 108 成功窗口同模式；别名回显 flash 一直如此，非漂移证据）；flash 正名→默认带思考（45 reasoning tokens）；reasoning_effort=low 仅轻微减思。**四轮失败与 108 两轮成功跑的是同一无思考模式**——"服务端档位漂移"归因撤回，修正为：无思考模式提案行为随机性+两真缺陷（占位三件套跳审计=110 卡；G6 gap 反馈不补轨道观察）。**收口腿模型裁定由 v4-pro 修正为正名 deepseek-flash（带思考，链路首次实测；思考模式预期改善 schema 纪律——占位三件套抄袭本是无思考失败形态）**，config 已切；若 flash 思考档同签名 2/2 → 升 v4-pro；再 2/2 → 菜单③+G6 取证卡。

- 收口腿复跑回执（2026-10-14 凌晨三，rerun2 腿，GLM-5.3 L2 执行会话 PC/ZCode）：**模型口径行——基线=deepseek-flash 思考档（卡面验收修正裁定执行，~/.vit/config.json defaultModel=deepseek-flash @ api.deepseek.com/v1 全程未动；启栈前探针 HTTP 200、echo=deepseek-flash、reasoning_tokens=17=思考档在线）**。worktree D:/Vit_DAW_wt_fsdc1r2 分支 port/fix-fs-settle-deadchain-1-rerun2@origin/main 8500fd7b（B-i+C 已在 main，本腿零代码改动）；agent 构建 VitAgent.exe SHA256 89D250BB0434CD1F；内核=主检出 VitApp.exe B6565DCF85D1DA86（C++ 面零改动）。run5b 20261014_011644 + run6 20261014_012927 **均 exit 1 "acoustic materiality record is missing"，同签名 2/2，§8 停止**（另有 run5 011553=调用形态失败腿：新 worktree 传外部 -KernelExe 未配 -SkipBuild，cmake 步抛错未启栈，无进程影响）。**本轮两 run 首次真栈走通 B-i 全下游，与 00:10-00:47 四轮全部死于准入前形成本质差异**：reverb 域选择→decay 提案→插件选择/装载→语义动态确认→干预 applied（actual_readback 0.2427、track 1017 guitar、原子控制 1 次、restore_ref 在卷）→post-action 重观测，两轮一致；死点移至**执行后 settle 段**——模型两次终局尝试 final=true 且语义正确（应用→重观测→交人工 A/B 试听裁决）但 `experiment_materiality` schema 非法（第一式 state=""、第二式 attempt 非正）被 free_state_final_gate 正确拒收 → 7 续轮预算耗尽 → materiality 记录缺失 → D1-S1 FAIL。拒收原始载荷（agent_message_loop_debug_run6.jsonl 两条 free_state_final_gate 记录）在卷。**分类：非 G6 停滞、非准入前死链、非 B-i 豁免回归**（acted 轮非法结算拒收=证据门语义保持，豁免谓词只覆盖未执行轮纯空态对）； owes_outcome 债正确 arm、continuation 全程在飞（lifecycle=pending），108 原始死链形态未复现。LLM 面：run5b 1/10、run6 3/10 次 60s 整超时（思考档时延顶满 60s 单调上限，transient_llm_error 均自恢复续轮，非本轮失败主因）。delay 加分腿未跑（exit 0 前提未达）。占用/释放已登记（edc3dfe6+1a0a5a97+释放行）；工件入库 [coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/](../../runs/FIX-FS-SETTLE-DEADCHAIN-1/)（console_d1_reverb_run5/5b/6.txt+run_20261014_011644/012927 报告与 agent 日志+debug jsonl），重件留盘可回指 worktree artifacts/free_state_d1_s1/。端测边界：真栈覆盖=域选择→提案→准入→装载→执行→post-action 重观测→settle 拒收边界×2；未覆盖=合法 settle 落地与人工 A/B 试听下游。**上交决策侧：exit 0 未达成；卡面升级阶梯（flash 思考档 2/2→升 v4-pro）触发条件已满足但按分工由决策侧发；亦可评估 settle 段 gap 反馈硬化（拒收错误未教模型 state 枚举/attempt 正整数形状）另立卡。**
