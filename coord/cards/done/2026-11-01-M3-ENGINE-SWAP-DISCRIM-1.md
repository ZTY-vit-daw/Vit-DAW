# M3-ENGINE-SWAP-DISCRIM-1：换引擎判别实验（deepseek-flash→deepseek-v4，同码同 fixture 只换引擎）——229→237 链终局判别

- 发卡：GLM 主管决策流 / 2026-11-01 晚（依据=237 验收终局定性：确定性层 229→237 全部收官+亲证生效，剩余变量独占模型行为面；234 gate 预埋换引擎判别实验；**用户裁定 2026-11-01：实验引擎=deepseek-v4**（同一 api.deepseek.com 账号））
- 派发确认：已确认（用户 2026-11-01 晚裁定）
- 验收负责人：GLM 主管决策流
- 池序 238；目标仓库=**主仓 scripts 单域**；P1 / 0.5 天 / 依赖=237 已在 main（79443422）✓；L1 / PC 执行会话

## 标的（链终局定性承接，不重考）

229→237 九卡拆完全部确定性层且逐层亲证生效（证据→identity→预算→门→驱动→指引教学→error 披露→结果披露）；237 停止条件终局定性=模型持有在野权威参数面（row54）仍宣称「缺少权威披露」+对 unsupported_projection 视图反复重试 ~30 次违反 no-retry 明文——纯模型行为残差。唯一剩余变量=引擎判断档位。判别实验：**同码同 fixture 同 smoke，只换引擎**。

## 目标（三面）

1. **预检腿（先于真栈）**：核实 api.deepseek.com 上 v4 模型 id（`/models` 或单调用探针；若 `deepseek-v4` 非有效 id，记录实际可用强档 id 并采用之，回执披露）；记录 v4 计费单价（对照 flash）。预检产物入 run 目录。
2. **实验腿**：`VIT_AGENT_LLM_MODEL` 环境变量注入引擎（[config.go:231](agent/internal/config/config.go#L231) 原生支持；ps1 env 块承 235 payload capture 先例，用户级 config **零触碰**）；**agent 生产代码零改动**（被测=main HEAD 79443422 原样）；AutomationLLMRouteSmoke 组 1 同 fixture 重跑；载荷捕获+needle+param_surface 测量面全开。ps1 env 作用域勘察定案（整 run 生效可接受——其他腿也走 v4 属可承受成本，如实披露；如能只作用于 LLM 路腿更优）。
3. **判读面（对照=237 alignrun1，成文对照表）**：组 1 verdicts/逐轮收敛数据（calls/turns/obs/limit_reached）/形态分型三面 flash vs v4 逐项对照。判别结论两支必居其一成文：
   - **A 收敛/显著改善**（组 1 门过或提案到达+set_points 形态出现）→ flash 判断档位不足实锤，生产决策轮引擎分层议题上呈用户（判断面强模型+量产面 flash，与用户全局调度律同构）；
   - **B 仍不收敛**（同型兜圈/止损，v4 也无视在野参数面）→ 引擎变量排除，prompt 架构/循环设计面回归主管重新归因（届时 processor.* 披露候补一并重评），附 v4 载荷形态逐轮分型。

## 预期与诚实边界（发卡时即声明）

不预设方向；两支结论等价有价值（A=引擎分层实锤；B=归因收窄到架构面）。预算：组 1 单 run 为主（v4 用量预期 ≤237 同型 ~90 调用量级，v4 单价高如实披露）；组 2/3 不跑（判读留主管）。单次实验=存在性判别非稳定性证明（§8：一次 exit 0 只证明存在一条成功路径）。

## 停止条件

- 预检发现 v4/强档 id 不可用（账号无权限/模型不存在）→预检产物+可用模型清单上交，不跑实验腿；
- 引擎面栈错误（401/限流/超时）→环境中断如实分类记录，不消耗判别结论；
- 实验腿组 1 三连同型止损→**即为止损**（本卡判读面本就覆盖该形态，证据包照常成文），不开第二 run 碰运气（§8）；
- ps1 env 注入造成非引擎面行为差异（其他腿异常红）→如实分列，判读只取 LLM 路腿。

## 文件域

`scripts/dev_agent_smoke.ps1`（env 块扩模型注入+作用域勘察）+`scripts/automation_llm_route_smoke.py`（如判读对照需微扩，telemetry 已带 model 字段）+`coord/runs/M3-ENGINE-SWAP-DISCRIM-1/` 工件。**agent 生产代码零改动**。越域即停上交。

## 验收标准

- 预检记录（模型 id+单价）+真栈 run 工件（ps1 场景腿 COMPLETED+载荷三件套）+flash vs v4 对照表+两支判别结论之一成文；栈登记/释放全流程；§8 计费披露（v4 单价对照 flash）。

## 领取

- 2026-11-01 21:00 +0800 / origin/main=24374340 / owner=GLM-5.3 PC 执行会话（ZCode，L1，238 卡会话）/ 分支=port/m3-engine-swap-discrim-1 / worktree=D:\Vit_DAW_wt_238impl
- 领取基线三件：HEAD=24374340（=origin/main；agent 生产代码=main HEAD 原样，24374340 相对 79443422 仅增本卡 todo 文件，被测代码内容与 79443422 一致）；impl worktree `git status --short` 空、`git diff --stat` 空（干净基线，无领取前 diff）
- 主工作树 D:\Vit_DAW 既有未提交改动（VitApp Workspace/coord runs 工件等）与本卡文件域（scripts 两文件+coord/runs 本卡目录）不相交，不动
- 真栈腿：实验前按 §2.2 登记 PC-RUNTIME-STACK.md（登记前复核+push 后 ≤60s 即时复核）

## 回执

- 实现/工件：port/m3-engine-swap-discrim-1 @ **2fa78247**（+28,325 行：ps1 +20 env 注入块（`-LlmModelOverride` 参数→`VIT_AGENT_LLM_MODEL`，设置点在 AutomationLLMRouteSmoke 腿 env 块=agent 启动前）+coord/runs/M3-ENGINE-SWAP-DISCRIM-1/ 全工件；**agent 生产代码零改动**、smoke py 零改动（telemetry model 字段+三测量面原生在场））。证据包 **EVIDENCE.md**（三面对照表+结论+计费）+ precheck/PRECHECK.md。
- **红线三件全守**：agent 生产代码零改动（被测=main HEAD 24374340 原样构建，console 亲证）/用户 config 零触碰（defaultModel=deepseek-flash 只读亲核，引擎只经 env 注入）/先预检后真栈。
- **链路进度（三面目标 1/2）：全达**——①预检腿：`/models` 亲证账号可用 id=`deepseek-flash`+`deepseek-v4-pro`，**`deepseek-v4` 非有效 id→按卡面采用 `deepseek-v4-pro`**（单调用探针 200/echo_model 回显亲证）；官方 pricing 抓取：v4-pro in 未命中 ¥3/M vs flash ¥1/M、out ¥6/M vs ¥2/M、cache ¥0.025/M vs ¥0.02/M（≈3×）。②实验腿：AutomationLLMRouteSmoke 场景腿 **COMPLETED exit=0**，引擎注入三面亲证（ps1 console env 行+telemetry/payload 逐行 `model=deepseek-v4-pro`+summary `usage.models`）；载荷捕获+needle（20/24）+param_surface 测量面全开；三同成立（同码/同 fixture/同栈 kernel f9e3da0a 复用）。
- **最终判读（目标 3）：两支结论均不成立——引擎面超时环境中断主导，判别未获结论，上交主管**（非 A 非 B 的第三形态，卡面停止条件 2 超时类+条件 4 并发命中的如实分类路径）：
  - 组1 [no_route×3] 三连同型止损，但**三轮 verdict_detail 逐字同型「本轮因 LLM 服务临时错误暂停」**——LLM 路腿 28 行=16 ok+**12 超时（43%）**；v4-pro OK 行延迟中位 **29.3s**/p90 44.4s（flash 7.4s/14.8s，91 行 0 超时）分布尾持续越线；**60s client timeout 硬编码 `agent/internal/llm/client.go:171` 无 env 覆盖**（改=改生产代码=本卡红线，未动）。
  - **A 不成立**：无收敛/无提案/set_points 零调用；**B 不成立**：v4 从未到达 get_parameters ok（param_surface 实收 **0/24** vs flash 1/86 row54）——非"持有参数面仍无视"，是**推理链被超时截断**（payload 亲证：r3 行21 参数面意识出现→行22 即超时 RAW 空）；v4 有效调用内推理方向正确（意图=时变包络✓/Track 1010 锁定✓）——存在"档位可能足够但被栈截断"迹象（16 ok 样本太薄，不作为 B 支结论，EVIDENCE 有限观察节披露）。
  - 上交主管三件：timeout 适配前置（代码卡：client timeout 对 reasoning 引擎不足，需 env override 或按模型配置）/适配后重跑预算 ≈¥0.93 上界不构成障碍/有限行为面观察支持"先适配再判别"而非"判 v4 无效"。
- **ps1 整体 exit 1 如实分列**：失败腿=无关腿 Chat route smoke（ps1:501 Invoke-WebRequest 超时，其 2/2 LLM 调用全撞 60s timeout）——**卡面停止条件 4 命中**（env 注入造成非引擎面行为差异），判读只取 LLM 路腿（236 alignrun1 无关腿超时同族先例）；场景腿本体 COMPLETED exit=0（§5 门槛：场景腿+工件+栈流程全达，ps1 整体 exit 分列披露）。
- **栈登记/释放全流程**：占用 eb4b30d6（登记前复核全零+push 后 ≤60s 即时复核全零 21:06:46；登记时间原误写 21:25→释放记录修正为 21:06，同 M1-3 先例）→释放 262ffe17（21:06–21:41，TEARDOWN_ALL_ZERO：VitAgent 40708/VitApp 27324 owner 手动停+PROCS=NONE+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0 亲核）。
- **§8 计费披露（v4-pro 对照 flash）**：计费行 19 ok=242,780 in/33,280 out（v4-pro 单价 3×）≈**¥0.93 上界**（对照 flash 同口径 ≈¥0.27）；**14 超时行客户端中止无 usage，服务端已发生 reasoning tokens 不可核实**（如实标注未计入估算）；单 run 存在性实验，组1 三连止损即止损未开第二 run。

## 验收

- （主管回填）

## 验收（GLM 主管决策流 / 2026-11-01 深夜）

- **实现 pass 合入**：红线三件亲核采信（agent 生产代码零改动=main HEAD 原样构建 console 亲证/用户 config 零触碰=引擎只经 VIT_AGENT_LLM_MODEL env（config.go:231 原生）/预检先行）；ps1 +20 注入块 diff 亲核（同 run 作用域+不落盘+注释完备）；载荷 row0 model=deepseek-v4-pro 亲证。
- **判别结论=两支均不成立采信（超时环境中断主导，§8 环境中断分类正确）**：12/28 超时 43%+param_surface 0/24（未到达非不采信）+r3 行21→22 关键推理步截断——v4-pro 判别被 60s 硬线（client.go:169-171 默认无覆盖，主管亲读）物理截断；B 支前提不成立的论证严谨（对照 237 flash row54 持有仍无视 vs 本轮从未持有）。三轮 verdict_detail 逐字同型=超时暂停型与 237 三轮各异的行为型形态质变，分型正确。
- **有限观察采信为方向性迹象（非结论）**：16 OK 行推理方向全对（意图分类/Track 1010 锁定/参数面意识）——支持「先 timeout 适配再判别」处置，不采信为 B 支。
- ps1 整体 exit 1=停止条件 4 命中如实分列采信（chat route 腿 2/2 超时同因）；计费 ¥0.93 上界+14 超时行不可核实披露采信。
- **处置**：开 239（llm client timeout env 适配，生产正当性独立于实验——引擎分层未来必需；默认 60s 不变向后兼容）+适配后按 238 同设计重跑判别（timeout 建议 240s 起：v4-pro p90 44.4s+越线尾 3-4× 余量）。手测统一测试欠账继续挂起。
