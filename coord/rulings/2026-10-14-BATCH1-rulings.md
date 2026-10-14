# 2026-10-14 批量一裁定（110 / 91 / 109 腿 D）

> 验收人：GLM 主管决策流（ZCode 主会话）。触发：用户"三张任务卡都已完成，可以检查"。
> 方法：diff 亲读+独立亲跑+工件亲核（详见各节）；main 起点=d72d7978。

## ① 卡 110 FIX-CARRIES-REPORT-TIGHTEN-1 —— **pass**

- diff 亲读：`carriesExperimentReport` 加 `loop.Experiment != nil &&` 前提（free_state_reasoning_loop.go:1047 域），注释锚定 109 rerun2 证据（占位三件套绕审计机制）；+154 行测试两面钉（无实验+占位三件套=新准入审计 / 有已准入实验=settle 豁免保留），红面验证（还原旧谓词主测试 FAIL）在回执自述。
- 独立亲跑：执行 worktree（crrt1@43dc0010 谱系）`go test ./... -count=1` **exit 0，95 包全 ok，零 FAIL**（本会话后台任务输出在卷）；gofmt blob 级亲核两文件净（LF 形态）。
- 域外备注采信登记：agentloop/free_state_reasoning.go:80 同名谓词为输出门形状校验非审计跳过，**不联动**；若后续 109 线出现输出门逃逸形态再议。
- 端测边界=纯单测收口（卡面授权），符合 §5 门槛。
- 处置：实现 cherry-pick 入 main（43dc0010→本批）；worktree crrt1 拆除（分支已推 origin 留档）。

## ② 卡 91 FE-GUI-B-FOURSLOT（前端收缩 Phase B）—— **pass（4 项偏离全部采信）**

- 实现亲核：前端仓 port/fe-gui-shrink-b-1 @1c61482（16 文件 +2041/-39，vit_dock/ 域+tools/diagnostics/，已推 origin 亲核）。
- 红线独立复核：diff 924318d..1c61482 变更 24 文件，**9 红线文件零命中**（本会话独立跑 git diff 核对，非仅采信 ps1 门）。
- 探针亲核：Phase B 119 钉×2 连跑（日志 120 PASS 行含 PROBE PASS）+Phase A 69+vitnote 系+FE-RACK 76 电池全量日志在卷 coord/runs/FE-GUI-B-FOURSLOT/20261014_112758/；vitnote input 探针为取证型（verdict 记录型），其 verdict 与 91 之前 VITNOTE-NOTESTREAM-2 跑逐字一致=回归中立。
- 真栈亲核：SMOKE_EXIT=0（console 尾行亲读），mix.observe obs_20261014T034250_80770a0c23d5（红线#8 链路活证）+no-pending-confirm 绿；占用/释放登记在卷，当前栈=空闲亲核。
- **4 项偏离裁定**：①F2 入口移主区抽屉柄簇（红线 9 文件零触碰门所致）——**采信**；若用户后续要回归走带栏位，另立红线豁免卡。②台后卡数按 viewport_registry spawnable=6 派生（规格原型 5 口径）——**采信**（证据驱动派生，探针按 registry 断言）。③chip 点击浮窗/⊞ 浮窗按钮=Phase C 面——**采信**（与卡 92 范围一致）。④底部状态栏不在断言②③④⑥ 面——**采信**，登记为 Phase C 候选输入（不强制）。
- 端测边界声明合规（headless probe=端侧门，FE-RACK 先例口径）；视觉观感/手感归用户手测。
- 处置：手测清单 8 条交用户排期（手测入口=用户从 Godot 拉起前端进工程，AGENTS §5 红线）；复验 worktree 前端仓 fe-gui-shrink-b 由决策侧拆除（本批执行）；Phase C（92）解锁（基线=port/fe-gui-shrink-b-1 @1c61482）。前端仓 main 归一卫生项维持挂账（用户裁定项，不因 B 合入自动触发）。

## ③ 卡 109 FIX-FS-SETTLE-DEADCHAIN-1 腿 D —— **代码采信合入；exit 0 未达成；梯子再修正=腿 E（取证先行+打地鼠硬化），v4-pro 暂缓**

- 腿 D diff 亲读：MaterialityEvaluation.Validate 三字段错误携带合法形状（常量驱动枚举、零 fixture 真值）+agentloop 三测钉（含 run6 实形状复刻 fixture+scope control）；settle 示例 evaluation 有意子集不动的论证核过（not_ready 须留 schema 合法，B-i 豁免对依赖）。独立亲跑 95 包（fsdc1r2@4737a0c6 谱系）见本批回填。
- 真栈证据亲核（REPORT §七）：run7/8/9 三轮 applied 一致（readback 0.2427 族）；**教学真栈在场实锤**（run7/9 debug 流拒收消息含枚举原文）；三形态互异=run7 transient 超时主导（教学有效性未检验）、run8 防重复门连弹 5 次+模型不换 view→capability_blocked、run9 观察包部分返回（工程绑定缺失）不入 D1 门+settle 打地鼠（state↔attempt 交替、只修被指字段、evidence_refs 亦缺）。
- **裁定**：
  1. 腿 D 代码采信合入（cherry-pick 4737a0c6→本批）；卡维持 doing；**三笔挂账未销**（exit 0 未达成，如实）。
  2. **梯子再修正（证据推翻既定梯，第二次先例）**：v4-pro 升级暂缓。理由：run8/9 terminal 签名相同但机制互异，run9 的 terminal 阻断（post-action 观察包部分返回不计入门）是**产品面缺口候选**——若属实，任何模型档位都无法过门（settle 完美亦死同门），v4-pro 有空烧风险；打地鼠又有确定性硬化修法（菜单②）。
  3. **腿 E（卡内扩域，rerun4）**：①取证先行（只读）：post-action 观察门两子形态——run8 防重复弹回语义（弹回是否正当+替代 view 是否可达）与 run9 部分返回（工程绑定缺失锚点定位，产品缺陷 vs 模型行为判定）；**若实锤产品缺口→停手上交裁定，不烧栈**。②打地鼠硬化（与取证结论无关，独立推进）：Validate 短路改全字段收集（一次列出全部违反项）+拒收消息附完整 materiality JSON 骨架（四字段全列零真值）+单测钉。③取证未实锤产品缺口→flash 思考档复跑 2 轮：exit 0=三账同销；同签名 2/2→升 v4-pro（梯子恢复）；再 2/2→菜单③+取证卡。
  4. run7 调用形态失败类与 60s 超时观察维持在档（4/14→1/14→0/13，无升级趋势；腿 E 后仍高频再评 reasoning_effort 接线）。

## 合入与工件

- main 合入：43dc0010（110）+4737a0c6（109 腿 D）cherry-pick，本批执行。
- 裁定文件：本文件；卡面回填随本批 commit。
- 亲跑工件：/tmp/gotest_110.log（95 包 exit 0）+/tmp/gotest_legd.log（腿 D 谱系全量，回填行引用）；run 工件以 coord/runs/{FIX-CARRIES-REPORT-TIGHTEN-1→port 分支工件,FIX-FS-SETTLE-DEADCHAIN-1,FE-GUI-B-FOURSLOT}/ 在卷为准。
