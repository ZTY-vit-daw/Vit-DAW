# 回执：JOURNEY-4-REV——A/B 试听判定旅程烟测（J3 链承接 + FS-CAP 增强后的确定性判定面）

- 卡：coord/cards/todo/2026-10-08-JOURNEY-4-REV.md（池序 24，2026-10-08 晚窗领取）
- 实现：`port/j4-rev` @ worktree `D:/Vit_DAW_wt_j4rev`，base=origin/main=509e37f9（领取时 HEAD，与 origin/main 一致；领取时工作树除本卡 diff 外无其他改动）
- 提交：（commit hash 见本分支 HEAD，回执随实现分支提交）

## 一、改动面申报

单文件 `scripts/dev_agent_smoke.ps1`：**+1071 / -3，纯新增场景段 + 参数/接线三处小改**，既有场景段（journey_first / journey_plugin_load / journey_free_state_nl / 其余 all 族）逐字零触碰。

1. 新参数：`-JourneySeatWaitSeconds`（默认 240，座位等待预算）、`-JourneyJudgmentArmSeconds`（默认 240，requested 武装等待）、`-JourneyBlindAudition`（盲听腿开关：泊位 env 注入 `VIT_DAW_AUDITION_BLIND=1`）。
2. 接线：`$journeyBerth` 泊位族 + 场景合法值表 + 工件根 `coord\runs\JOURNEY-4-REV\<ts>`；`-StartKernel` 强制、不入 `all`、拒绝占用栈（照泊位族既有约定）。
3. 盲听 env 注入：泊位 setup 段（kernel/agent 启动前）注入并写 `blind_mode.json` 工件。
4. 新场景段 `journey_ab_judgment`：J3 骨架逐字承接（fixture→open→authority→NL 轮 settle loop→confirm hop loop），本卡新增腿：
   - **LEG 6 座位等待（概率面）**：轮内轮询 `/agent/events`，取最后一条带完整判定身份（session_id+turn_id+round_id+project_revision，reply_gen_toolgate 机器行走同形）的 `audition.*` 行；未挂座=分类分记不 throw。
   - **LEG 7 判定腿（确定性，挂座即门）**：每次尝试重发现当前 seat（防链推进导致的身份过期，见"执行中修复"②）→ `/agent/audition/status` 断言 → select(candidate-a)+select(candidate-b) 双驱断言 → `trajectory.user_judgment.requested` 武装等待（POST 自身 bind 路径兜底）→ judgment POST（身份全部取自 seat，不虚构）→ `trajectory.user_judgment.requested`/`recorded` 双双点名本 seat 断言 → `/agent/audition/status` 终态核对。POST 身份不匹配拒绝=一次重试，其余拒绝即 throw。
   - **盲听变体**（`-JourneyBlindAudition` 且 seat.blind=true）：判定前偏差面断言（seat 无 `blind_disclosure` 键、事件流无 `audition.blind_disclosure` 行、requested details 无 `candidate_a_physical`/`candidate_b_physical`/`blind_disclosure`/`mapping_source`）+ recorded 行 `details.blind=true` 断言；判后揭示（`audition.blind_disclosure` 事件/响应 `blind_disclosure`）如实记录不设门。blind 轮挂到 canonical 座（非 D1-S1 准入）= `blind_not_eligible` 如实分记不 throw，canonical 腿照常驱动。
   - **FS-CAP 分类面**：turn/hop 响应自然携带 `stop_reason=capability_blocked` 时，断言 WorkflowData 边界面（`capability_blocked=true`+`mutation_performed=false`+`free_state_admission_receipt` 非空对象）；**不为凑分类人为构造拒绝**（卡面约束），触发而不完整=确定性面破坏即 throw。
   - 分类器：`judgment_ok` / `judgment_ok_blind` / `hung_unjudged` / `environment_interrupt` / `capability_blocked`（一等类，FS-CAP 旅程面增益）/ `card_mounted:*` / `auto_applied_no_seat` / `chain_stall` / `no_candidate_found` / `model_pure_text` / `other_terminal:*`。exit-0 门只载确定性子面（J1 三腿、hop 路由门、判定腿、盲听断言面、FS-CAP 面），概率腿如实分记不 throw。

## 二、§8 预算申报与轮次实况（执行前声明：canonical 腿 N=3 成功=≥1 挂卡；盲听腿至挂卡为限至多 3 轮；失败分类四类分记）

| 轮 | run 工件 | 模式 | 结果分类 | 关键面 | 判定 |
|---|---|---|---|---|---|
| C1 | `coord/runs/JOURNEY-4-REV/20261008_193027/` | canonical | `no_candidate_found` | 1 turn 后 goal terminal，无 audition 面（candidate_ready_seen=False） | 概率 miss，如实分记 |
| C2 | `coord/runs/JOURNEY-4-REV/20261008_193642/` | canonical | **`judgment_ok`** | seat via `audition.ready`（`audition:turn:free_state_1d5dfccc9c98a3c7:round-1-fb85ad08cd5564b9`，candidate.ready 在场）；status→select A/B→requested×2→POST ok→recorded×1→终态 ok，全链走通 | **挂卡腿成功条件达成（2/3 轮，止于成功）** |
| B1 | `coord/runs/JOURNEY-4-REV/20261008_194622/` | blind | 判定 POST 409 `audition session identity mismatch`（throw，teardown 正常） | seat `blind=true` 已坐实（env 注入生效，A/B 物理分配相对 C2 对调）；挂卡后链仍在推进，POST 时 loop 身份已前进 | 真实失败形→脚本加固（见三②）；工件保全+证据转录在案 |
| B2 | `coord/runs/JOURNEY-4-REV/20261008_200153/` | blind | **`judgment_ok_blind`** | seat `blind=true`；判定前偏差面干净；requested×2→POST ok（响应携判后揭示：selected=b→physical=after→action=retain，`mapping_source=render_revision`）；recorded 行 `blind=true`；`audition.blind_disclosure` 事件判后落场；终态 ok（attempt=1，无 seat 轮转） | **盲听腿通过** |

- 环境中断：0 轮。capability_blocked 形：4 轮均未自然触发（face=not_triggered 如实记录，未人为构造）。
- 泊位：每轮自起自拆（每轮 netstat 核 5555/7878/5556 空闲后起栈；teardown 两行 ok；B1 的 throw 路径 teardown 亦正常执行）。

## 三、执行中修复（两处，均为脚本自身缺陷，非 agent 侧）

1. **`disclosure_in_response` 误读**：`Get-OptionalProperty` 对缺失键返回 `""`，`$null -ne ""` 恒真——C2 摘要曾误报 `disclosure_in_response=True`（响应工件实无该键，已亲核）。改为字符串非空判定。C2 该字段作废，以工件为准；B2 的 True 为真值（盲听揭示）。
2. **LEG 7 座位过期加固**：B1 坐实"挂卡后链仍在推进，早先捕获的 seat 身份过期→判定 POST 409 identity mismatch"。重构为每次尝试重发现当前 seat（对齐 webui 驱动当前挂出卡的语义）+ identity mismatch 单次重试；B2 在加固后脚本上一次通过（attempt=1 未触发重试）。

## 四、断言表面（判定腿+盲听腿，全部落在服务端/事件拥有面）

- 判定腿（canonical）：status 路由 ok；select A/B 双路由 ok；`trajectory.user_judgment.requested` 在场且 `details.audition_session_id==seat`；`trajectory.user_judgment.recorded` 在场且点名 seat；终态 status ok 且 session 身份一致。
- 盲听腿（全部通过）：`session.blind==true`；判定前 seat 无 disclosure 键、流无 disclosure 事件；requested details 无物理分配键；recorded 行 `details.blind==true`；判后揭示事件/响应字段如实记录（不设门——揭示的 presence 依赖 disposition 可解析，属产品语义非旅程门）。
- 未设门而如实记录的面：`audition.selected` 事件计数、`judgment.settled` 事件（C2/B2 均在场，见 `journey_ab_judgment_events.json`）、requested 行数（流中 2 行=prepare 武装+重发/restore 形，均点名 seat，链断言不受影响）。

## 五、真实栈与工件契约（§9）

- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -Scenario journey_ab_judgment -StartKernel [-JourneyBlindAudition] -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe`（在 `D:/Vit_DAW_wt_j4rev` 下执行；无 `-SkipBuild`，agent 二进制每轮由 worktree 现编 `go build ./cmd/vitagent`）。
- 被测 HEAD：四轮 head.txt 均 `509e37f9`；kernel 部署二进制 sha256 `b6565dcf…3123b` 与 KERNEL-RENDER-FREEZE-FIX-1 回执申报一致（主树 Export/staging 部署运行时，本卡零内核改动）。
- 退出码：C1/C2 由管道缓冲未能直采（`| tail` 缺陷，B 轮起已移除）；两轮 summary.json+scenario 尾段均走完=pass 形。**B1 真实失败（throw→非零）；B2 真实 exit 0（无管道直采）**。决策侧复跑以直采退出码为准。
- LLM：真 LLM 概率面（`~/.vit/config.json` 凭据；盲听 tier 的 env 面优先于 config 面）。
- 运行工件不入库（本回执与脚本随分支提交；run 目录留 worktree/主树盘面可回指）。

## 六、端测边界声明

- 本卡改动仅 `scripts/dev_agent_smoke.ps1`（场景段+参数/接线），无 agent/webui/内核代码改动；断言全部落在服务端 HTTP 面/事件拥有面，无渲染面与用户旅程面改动（渲染面烟测属 E2E-WEBUI-1 域）。
- 盲听轮的"判定请求不带标签偏置"断言按 AUDITION-PLAY-1/B12-1 既有字段语义实锚（session.blind 布尔面、判前无揭示、requested details 无物理分配键）；**观察项（不设门，供决策侧裁断）**：D1 渲染对候选的 source_ref/preview_ref 文件名含 before/after 字样，会话负载可见——盲听语义的"抽签不出快照"指 draw 本身，引用面是否算泄漏属 B12 域语义，本卡不改不断言。
- capability_blocked 分类面在本轮采样中未被概率触发（未人为构造），FS-CAP 旅程面验证**未获得真栈触发样本**——该形样本仍按 FS-CAP 裁定挂账（留自然出现或后续旅程轮采样）。

## 七、遗留与移交

1. **观察项**：主树 `VitApp/Workspace/agent_runtime_config.json`（blind 配置文件面）为 UTF-8 带 BOM，Go json.Unmarshal 拒读→agent 按 fail-closed 语义报 `config_invalid` 并禁用 blind tier（日志行在案）。env 面不受影响（本卡盲听腿走 env 注入）。该文件为运行时状态、不在本卡文件域——修 BOM（或改配置写入方）需另立卡/由用户处理。
2. 决策侧复跑判定腿 1 轮按卡面验收标准执行：`-Scenario journey_ab_judgment -StartKernel -KernelExe <部署内核>`（如需盲听面加 `-JourneyBlindAudition`），直采退出码。
3. 领取行已回填主树卡面；卡状态移动（todo→done）待决策侧验收后代行。
