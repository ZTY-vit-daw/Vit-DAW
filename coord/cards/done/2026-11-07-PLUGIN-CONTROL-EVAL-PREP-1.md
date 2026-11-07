# PLUGIN-CONTROL-EVAL-PREP-1：冻结当前模型效果器控制实验的首轮计划

- 发卡：Codex / 本次效果器自主控制设计讨论会话 / 2026-11-07。
- 派发确认：**已确认（GLM 主管决策流，2026-11-07 午后）**——池序 **299**；PREP 即派（零栈零 LLM 只读）；主管附加裁定三条：①S1 付费+真栈轮**不随本卡授权**，预算与跑前批准待 PREP 产出后另批；②若 A/B 臂需 test-only 披露缝=改代码卡，挂「答辩前不落实」约定边界，届时用户拍板；③S1 建议分段执行 P→A→B→C（组间 go/no-go 检查点，计划冻结后按段触发），PREP 在 RUNBOOK 中按此分段出预算。执行 owner 由主管指派。
- 验收负责人：PC 主管决策流；本会话不代替最终验收。
- 目标仓库：`D:\Vit_DAW`。
- 池序 / 优先级 / 预估：池序和优先级由主管安排；建议 30–60 分钟，只读勘察与计划冻结，超过粒度交剩余项而不自行扩域。
- 依据：[实验方案](../../reports/2026-11-07-plugin-control-model-evaluation-plan.md)、[同日效果器笔记 v5](../../reports/2026-11-07-effect-control-future-brainstorm.md)、[准入后置裁定](../../decisions/2026-11-07-pca-admission-after-control.md)。
- 依赖：无硬代码依赖；主管先检查是否与现有实验重复。既有音源探针与本卡目标不同，不重复领取。

## 唯一目标

产出一份能直接据以拆发 RUNNER/S1-RUN 小卡的首轮实验包：核清当前实际被测模型、原始参数/语义辅助的边界，冻结候选任务、比较分组、评价规则、轮次、费用上限和入口缺口。**本卡完成不代表模型已经通过能力实验。**

待证假设：当前模型仅靠原始抓手，或增加有界检索，可能已能自主完成效果器控制；现有语义适配是否仍必要，须通过对照判断。不得预设支持或反对任一路线。

## 工作步骤

1. 重新读取 `AGENTS.md`、`coord/PROTOCOL.md`、当前卡池和实验方案。记录 `git rev-parse HEAD`、`git status --short`、`git diff --stat` 与执行分支/worktree；正式领取同步规则照旧。
2. 只读核对模型配置与调用位置，可查已有遥测但不启动新模型请求。列出实际默认模型/路由、fallback、已知预算项和配置证据；无法仅靠静态证据确认实际响应模型时标“待首轮遥测核验”，不能把配置当实测。
3. 核对当前工具披露、原始读写/独立回读、检索权限及字段来源。逐项区分插件原生元数据、通用数值转换、人工语义加工。检查 A/B 是否被入口门或 explain/typed 路径强制接管，列出具体代码锚点与最小缺口，不修改生产门。
4. 根据已有真实参数工件和在库插件选择至少三件效果器、两个族，拟定方案 T1–T6；明确每个案例的状态准备、目标、容差、非目标项规则和可达性证据。只能确认参数面而不能确认实际目标可达时，标为后续零 LLM 真栈已知操作控制前置，不谎报已验证。无适用工件时停止在“缺口＋候选取证入口”，不在本卡启栈取数。
5. 产出 `plan.json` 和 RUNBOOK：预设 P/A/B/C、组序、独立重复、反馈/缓存隔离、首轮次数、成功条件、失败分类、止损规则、各层退出码含义、每轮/批次预算上限与估算依据。默认参考 6＋54 任务轮次，主管可在跑前改定；严禁把任务轮次当 API 次数或把脚本运行称为整体零 LLM 成本。
6. 给出下一张最小 RUNNER/取证卡建议：固定文件域、现有 ps1 接入口、需建设的工具隔离/评价面、具体可执行验收交付要求和栈依赖。确有资料或接口缺口则交 blocked 证据，不强行给“已可执行”。

## 文件域与资源

允许新建且仅新建以下三件实验准备工件，并按协议回填本卡领取/回执：

1. `coord/reports/2026-11-07-plugin-control-model-evaluation-preflight.md`
2. `coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json`
3. `coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/RUNBOOK.md`

源码、脚本、配置、现有卡、POOL-INDEX、现有设计笔记、fixture、attestation 库和共享栈记录均只读。阅读相关源码与已有工件不受上述输出文件数限制；密钥、完整敏感配置不抄入报告。

本卡不占真实栈，不调用模型，不发起联网检索，不下载手册，不安装插件，不修改默认提示词，不创建后台调度或跨会话投递。资料可达性无法只读确认的，写入后续待验证项。若实现隔离缝、独立探针、修接口或准备真实夹具需要改代码，列锚点并另卡。

可与当前文件域不相交的卡并行；后续实验真栈腿由主管与 `B3-ABLATION-1`、`D1D3-REGRESS-STACK-1`、`KERNEL-OFFRENDER-FORENSICS-1`、`VSTI-PARAM-PROBE-1` 等按最新状态错峰，本卡不抢占、不重排。

## 验收标准

1. 三件工件存在，链接可解析，`plan.json` 可解析且至少含：`status`、`baseline`、`model_inventory`、`arms`、`cases`、`run_counts`、`budgets`、`ordering_and_isolation`、`evaluation`、`failure_taxonomy`、`stop_rules`、`pending_approvals`、`next_card_scope`。尚待证的值显式标为 pending，不以虚构值填满。
2. preflight 逐项区分“静态核对/历史工件/待新运行验证”；明确 P 与 A/B/C 的差别、字段泄露风险、原始路径是否可达、C 共同支持集合及实际模型响应待核项。
3. RUNBOOK 有计划轮次和预算依据；无价格/用量依据时给出缺项及独立校准建议，不授予无限预算。正式付费/真栈运行的批准项列清。
4. 如果运行入口尚需建设，给出对应代码锚点与下张卡的可执行命令交付要求；尚不存在的命令明确写“待建设交付接口”，不得伪装成已存在或通过。
5. 无生产改动、无栈启停、无付费请求、无旧工件覆盖；本卡可以以“实验前提受阻＋完整证据”完成取证交付，由主管决定下一步。

在独立执行 worktree 根目录进行以下文档检查，并记录命令与退出码；它们不构成端侧或模型能力测试：

```powershell
$evalPlan = Get-Content -Raw coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json | ConvertFrom-Json
$requiredFields = @('status','baseline','model_inventory','arms','cases','run_counts','budgets','ordering_and_isolation','evaluation','failure_taxonomy','stop_rules','pending_approvals','next_card_scope')
foreach ($field in $requiredFields) { if ($null -eq $evalPlan.PSObject.Properties[$field]) { throw "Missing plan field: $field" } }
Get-Item coord/reports/2026-11-07-plugin-control-model-evaluation-preflight.md, coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/RUNBOOK.md -ErrorAction Stop
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'git diff --check failed' }
```

期望退出码 0；既有 diff 的检查问题与本卡新增问题分列，禁止整理其他人的改动。无需为本卡运行 Go/webui 全量测试或真实栈烟测。

## 停止条件

- 原始路径不可达、参数面来源不可分、没有适用真实工件、需要越域改代码或与在飞卡重叠：记录最小证据及建议，停下交主管。
- 无法识别当前模型实际路由、预算无法估算：保留静态证据与待核项，不替模型选型、不自行调用来补证。
- 需要改 sealed/holdout、现有样例或用户工程来造考题：停止；后续夹具必须走独立临时副本。
- 单卡超出 60 分钟且仍有重大不确定性：交已完成工件、未定项及后续拆卡建议，不把范围扩成实验实现。

## 领取 / 回执 / 验收

- 领取：2026-11-07 18:20 +0800 ｜ origin/main=b352ba898c0f5016962d7c4b5e5d3d5fbfdd8014（与本地 main 一致）｜ owner=GLM 主会话 L2-worker 子代理（ZCode/PC）｜ 分支=port/plugin-control-eval-prep-1 ｜ worktree=D:/Vit_DAW_wt_299prep ｜ 领取基线：HEAD=b352ba89、status 全净、diff --stat 空（fresh worktree）｜ 前置核对：卡在 todo/ ✓ 派发确认=已确认（池序 299，HEAD 提交即派发登记）✓ doing 三卡（ARRLOOP-AUTHBRIDGE-1/B3-ABLATION-1/CLOSURE-ROUTE-ADAPT-1）文件域与本卡（仅 coord/reports/、coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/ 新建三件+本卡）不相交 ✓ 领取提交见回执。
- 回执（2026-11-07 19:10 +0800，执行侧自验完成待验收）：
  - **领取提交**：a93dd97c（已推 main，fetch 重读确认 owner）；**工件提交**：b9465b44（分支 `port/plugin-control-eval-prep-1`，3 文件 +504 行，零生产改动——仅新建三件工件）。领取后 main 另有 4 笔他卡领取提交（3f222128 等），文件域不相交已复核。
  - **三件工件**：① [preflight 报告](../../reports/2026-11-07-plugin-control-model-evaluation-preflight.md)；② [plan.json](../../runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json)（13 必需字段全在，待证值显式 pending，全文 ASCII——PS5.1 `Get-Content -Raw` 默认码页下可解析）；③ [RUNBOOK.md](../../runs/PLUGIN-CONTROL-EVAL-PREP-1/RUNBOOK.md)（P→A\*→B\*→C 分段 go/no-go，token 上限授权+货币 pending+3 轮校准样本建议）。
  - **检查命令与退出码**（worktree D:/Vit_DAW_wt_299prep 根目录执行，卡面 PowerShell 命令逐字运行）：`ConvertFrom-Json+13 字段校验+Get-Item+git diff --check` → **exit 0**（DOC_CHECK_PASS；首轮曾因 UTF-8 中文被 PS 默认 GBK 码页误读致假 PASS，已改 ASCII 后复跑真 PASS，exit 0 记录于本轮）。
  - **关键结论**：①实际被测模型=deepseek-flash（`~/.vit/config.json`+遥测 228 条非错误记录双证；响应模型/采样参数待首轮请求捕获核验）；②A/B 纯臂**不可达**——产品聊天入口 explicit_control 强制 typed 接管（goalrunner_chat.go:260-307 + orchestrationcontroller/types.go:83）+内核回复无条件烘焙语义字段（PluginRackControlService.cpp:3326-3436）+目录描述与 message loop 骨架提示词引导（catalog.go:1173/message_loop.go:4344-4351）——四处 test-only 缝锚点已列（S1-S4），不改代码只能跑污染臂 A\*/B\*；③字段来源三分完成：normalized_role/display_group/control_relevance/template_role=人工词表（VitPluginTemplateRegistry.cpp:85-202，实测常见错误归类如 Decay Rate→mod_rate），alias=原生默认+持久化语义覆盖混合，display_probe/value_text=通用探测；④T1-T6 冻结自 61 件兼容矩阵真实参数面（Pro-R/Plates/H-Delay/IR-L，reverb+delay 2 族 4 件），目标可达性=抽验先例级，逐案 S0 已知操作腿待批执行。
  - **未决项（交主管）**：①污染臂先跑下界 vs 先建缝跑纯臂 vs 两段都跑（缝卡挂「答辩前不落实」用户拍板）；②货币预算缺项（DeepSeek 价目未核，建议 3 轮校准样本）；③P 臂确认卡应答策略冻结；④S0 真栈腿窗口（与 296/297/298 等栈腿错峰）；⑤响应模型身份首轮核验。
  - **建议后续卡**：PLUGIN-CONTROL-EVAL-RUNNER-1（文件域 scripts/ 新 py/ps1+coord/runs/PLUGIN-CONTROL-EVAL-\*；pgtdc_compat_verify.ps1+b3 先例；交付=夹具构建+S0 零 LLM 验证器+臂驱动+独立评价器+工件/退出码；详见 plan.json next_card_scope）。
  - **端测边界声明**：本卡为零运行的实验准备——无模型成绩、无真栈结论、无栈启停、无付费请求、无旧工件覆盖；引用遥测为既有文件只读采样。
- 验收：待 PC 主管决策流裁定；执行 done 不代替最终验收。
