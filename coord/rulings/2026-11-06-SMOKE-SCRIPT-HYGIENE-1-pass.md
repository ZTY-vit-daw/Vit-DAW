# Ruling：SMOKE-SCRIPT-HYGIENE-1（260）验收裁定——pass（G3/G7 关闭+258 遗留④落地）

- 裁定：GLM 主管决策流 / 2026-11-06 晚窗
- 输入：卡面回执（done，1fd7a830）+ impl 6b47c178 + 工件 construct_20261106（三 probe 11 腿+机器可读 summary）

## 一、分面裁定

| 面 | 裁定 | 依据 |
|---|---|---|
| ①G3 假绿 | **pass** | c1 三条 go test 后 $LASTEXITCODE 检查；probe1 三腿：pre-fix 假绿 exit 0 复现→post-fix FAIL 桩 exit 1+点名→PASS 桩 exit 0 |
| ②RunDir 参数 | **pass** | ps1:147 无条件清空删除（param 默认值兜底）；probe2 四腿含 pre-fix 忽略 -RunDir 复现+传入生效+默认不变+-PreflightOnly 完好 |
| ③G7 前置清单 | **pass** | scripts/SMOKE_TESTS.md 顶部新节（+21 行，对齐既有 Draft Root 契约节模式） |
| ④LW/settle 相位门分支化（258 遗留） | **pass** | zero-mutation 分支读报告 validation.branch→两相位各写 skip-with-reason 判定面（phase_reachable=false+skipped_branch_unreachable+skip_reason，LW 面含 setup 证据）不 throw 不拦断；分支标记缺失/陌生落向全门路径（fail 向诚实）；**1-mutation 分支门组体字节零改动**（仅 if 条件+追加 elseif）；probe3 四腿含 pre-fix 258 形态复现对照 |
| 端测边界 | 接受 | 零真栈口径=卡面定义（构造验证）；桩腿证明脚本层分支调度与判定面产出，「真实栈上分支不可达」事实由 258 verifyrun1 真栈工件钉死——边界声明规范 |

## 二、两笔披露处置

- **文件域偏差追认**：第②项目标文件实为 `coord/runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/run_tracklist_fix_probe.ps1`（ps1:147 所指），非 scripts/ 下——发卡侧引用行号锚点未带文件全路径（主管发卡瑕疵，与 249 止损连带同账）；执行侧按条目文义执行+主动披露=良性。
- **域外观察（TRACK-LIST-GAP-1 ps1:147 同型缺陷）裁定不修**：该文件属历史取证工件目录（139 runner 一次性工件），改动污染工件取证一致性；若未来复用该 probe 再修（known-issue 记录，不入池）。

## 三、验收动作

- 合入 631b54a2 零冲突（scripts 三文件+coord/runs 一 ps1+SMOKE_TESTS.md+工件）；257 G3/G7 关闭；**258 裁定①（LW/settle 相位门分支化）就此落地**——后续 D1 冒测任一分支均产出完整判定面，258 挂账脚本面清偿。
