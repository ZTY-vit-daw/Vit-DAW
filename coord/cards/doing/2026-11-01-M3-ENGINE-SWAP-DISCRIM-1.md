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

- （执行侧回填）

## 验收

- （主管回填）
