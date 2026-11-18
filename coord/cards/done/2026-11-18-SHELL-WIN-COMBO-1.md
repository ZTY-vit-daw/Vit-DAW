# SHELL-WIN-COMBO-1：391 组合真栈验收（391 解锁卡）

- 池序 414；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚批准方案 A）。
- 依赖：410/411/412 验收合入 + 413 最终产物刷新完成；真栈单 owner（本主管会话放行与登记）。
- 单目标：单次真实产品入口组合验收（VitShell→React 启动面→打开工程→装载完成呈现），覆盖：

1. **409 产品窗口断言**：首显在对话窗下 / 16DIP / 零激活焦点 / 无后纠正（复用 `scripts/vitshell_zproduct_smoke.ps1` 与 `scripts/vitshell_zproduct_verdict.mjs`）。
2. **395 真实事件链**：CDP 断言 `data-window-launch-feedback` 出现、真实步骤与终态（`ready` 或如实降级 `accepted`/`failed`）、`data-native-focus-hint` 正/负边界（正例必须有自然 DAW 前台事实，不可伪造）。
3. **396 retry 腿**：`scripts/ui_entry2_smoke.ps1` 重试路径 exit 0（与单次栈冲突时由同 owner 串行第二腿）。
4. **卫生**：fixture 副本、recents 快照/finally 哈希、权威输入哈希、栈释放亲证。

## 资源与工件

跑前在 `coord/resources/PC-RUNTIME-STACK.md` 登记唯一 owner、推 main 并重读；启动前 ≤60 秒即时核进程/端口净空。run 目录 `coord/runs/SHELL-WIN-COMBO-1/<stamp>/`；跑前成文运行次数、成功条件、失败分类（product/coverage/environment/harness 分开）、止损线。

## 通过条件

script/wrapper/host 均 exit 0 + 断言摘要 + 原始工件。任何 `not_exercised` / 覆盖缺口如实声明，不得抵作 PASS；不以事后正确状态抵销首显反例。

## 停止条件

任一门失败即停并保留原工件；同一确定性断点两次→停转诊断；环境中断单列，不记产品结论。

## 领取/回执/验收

待 410–413 收口后回填（owner=lead/combo 或主管亲跑；分支/worktree、资源记录、执行时间同批登记）。

## 领取（2026-11-18 晚，主管亲跑）

- owner=lead（PC 主管决策流，主管本人执行真栈验收；不委派生产实现）；无实现分支；真栈唯一 owner。
- 前置已达成：410/411/412 实现面 PASS 并合入（前端 main `cf33195b`、主仓 main `dfa86cac`）；413 产物刷新 PASS（VitShell `5D420DED…`、DLL `B270362D…`、webui dist `0886058E…`）。
- 装置扩展（主管实施）：`scripts/vitshell_zproduct_smoke.ps1` 增 `-CardId` / `-PresentationChecks`（默认行为与原 409 一致）+ 新 `scripts/vitshell_combo_cdp.mjs` 读取真实 DOM：feedback request/seq/terminal/progress、`data-loading-phase=completed`、步骤数、无假进度文本、shell 条件下无 DAW 焦点声明与无 focus hint、同窗 sentinel。
- 运行参数（拟）：`-Condition shell -CardId SHELL-WIN-COMBO-1 -PresentationChecks -TimeoutSec 180 -NaturalConditionWaitSec 60`；Stage 于 `%TEMP%/shell414-combo-*`（host/runtime/frontend 布局，fresh DLL）；RunRoot `coord/runs/SHELL-WIN-COMBO-1/run_<stamp>`。
- 资源：跑前在 `coord/resources/PC-RUNTIME-STACK.md` 顶部登记（卡 414、owner、stage/run、单次预算与止损），推 main 重读后 ≤60s 启动；失败即停、无原样重跑。
- 通过：script/wrapper/host exit 0 + 409 分类器 PASS + 395 呈现断言 PASS（terminal=ready、completed 卡在场、无假进度、无焦点误报）。任一门失败按 product/harness/environment/coverage 分类保留原工件。

## 执行记录（2026-11-18 晚）

- attempt01（run_20261118_184423）：product_failure=集成缺陷（410 宿主 stage 被嵌套 observations 同名字段遮蔽）→ 修复卡 415（已 PASS）。409 原生面本已 PASS。
- attempt02（run_20261118_185034）：415 生效，**395 呈现面 PASS**（terminal=ready/completed/steps=3，无假进度、无焦点误报）；409 冻结分类器因 open 后 polling 中出现 Godot 前台（interval.end+2094ms 起 113 样本）且 pre-open 存在非本栈前台切换 → 按冻结规则保留 FAIL，归环境/外部前台干扰（归属未定）。详见 [ATTEMPT-02](../runs/SHELL-WIN-COMBO-1/ATTEMPT-02.md)。
- harness 已调整：presentation CDP 移到 observer Stop 之后，保持 409 冻结观测窗口不变。
- 下一轮：一次 hands-off 条件的 shell 验收（用户配合 ~30–40s 不动键鼠）；若再遇同类外部干扰则停手转环境核对，不原样重跑。

## 终验收（2026-11-18 晚）

- attempt03 PASS（run_20261118_190253，用户 hands-off）：script/wrapper/host exit 0；409 分类器 PASS（initial_below_reference、无纠正、interval 0 激活/0 焦点、707 样本）；395 呈现 PASS（terminal=ready、completed=1、steps=3、无假进度、无焦点误报、同窗 sentinel）；chain_detail=workspace_ready；零 LLM；六份 authority 还原一致、进程/端口/UDP 全零。
- 裁定：[rulings/2026-11-18-SHELL-WIN-COMBO-1-pass.md](../../rulings/2026-11-18-SHELL-WIN-COMBO-1-pass.md)。391 解锁，手测 v7 放行。
