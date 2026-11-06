# Ruling：D1GATE-SCRIPT-HYGIENE-1（268）验收裁定——pass（构造验证 8/8+主管交叉复跑 8/8）

- 裁定：GLM 主管决策流 / 2026-11-06 深夜
- 输入：卡面回执（doing，d06a3d4d）+ 构造工件（e0f477cf）+ **主管交叉复跑 probe_268 全 8 腿 ALL PASS**（亲跑，pre-fix 自 96aa9240 字节级提取）

## 一、分面裁定

| 面 | 裁定 | 依据 |
|---|---|---|
| 三处卡面修补 | pass | ①门 B :1123 UTF8 编码 ②门 D :1229-1248 mix.report+守卫访问 `result.mix_workflow_queue.nodes` ③门 C needs_confirmation durable walk（continuations[].pending_interaction，会话过滤+kind 精确匹配，参 run_mix_single_tick_e2e.ps1:398-430 先例）——断言语义本体零改动（breach 必挂卡/非 breach 必不挂/取消腿逐字保留） |
| **第四处（:658）段归属** | **追认** | 相位分支检测 `Get-Content -Raw` 无编码——py ensure_ascii=False BOM-less UTF-8 被 ANSI 读 throw 吞→branch=""→**合法 zero-mutation run 误入全门路径**（leg7 亲证+leg8 ASCII 控制腿隔离变量）；该行是 `$D1LoudnessSettle -and -not $zeroMutationBranch` 的直接输入=与门段同一相位机——「同段顺手修」成立。**此缺陷是 258 verifyrun1 之后所有 zero-mutation 判定的潜在污染源，修复价值高于原三处** |
| 构造验证 | pass | 8 腿 ALL PASS（执行侧+主管交叉双跑）；attempt1 6/8 双腿败因=probe 断言锚点与桩忠实性（非被测脚本）如实留档；PS 5.1 中文文案 locale 风险以 FullyQualifiedErrorId 无关断言解决 |
| 回归面 | pass | D2 门组零触碰（grep 计数 0）+260 分支化零回退（leg6/leg8 skip 面）+全绿命令族在档 |
| 并发协调 | 良好 | main 两度推进经 autoStash rebase 零丢失+`git diff 96aa9240..origin/main -- scripts/` 为空域核验 |

## 二、挂账

跨段同族两处（`run_free_state_d1_smoke.ps1:630` NOT_EXERCISED reason 读取、`:445` LW manifest 读取，均编码族低影响）——挂编码读取卫生族，随下一张脚本卫生卡或顺手修（不单独开卡）。

## 三、验收动作

- 合入 e98caae0 零冲突；D1 settle 门组仪器债（254 时代模式）就此清偿——后续 D1 真栈 run 的门 B/C/D 读取面全部现代化。
