# SMOKE-SCRIPT-HYGIENE-1：烟测脚本卫生批（G3 退出码+RunDir 参数+跑腿前置清单+LW/settle 相位门分支化）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 裁定](../../rulings/2026-11-05-AF-FAMILY-HEALTH-1-pass.md) G3/G7+255 上交 ps1:147）；**2026-11-06 早窗扩项**（第四项并入，依据=[258 裁定](../../rulings/2026-11-06-D1-DISCLOSURE-FORENSICS-1-pass.md) §二-①——LW/settle 相位门分支化）
- 派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 260；目标=scripts/ 四处；P2 / 0.5 天 / 无依赖；L1；零真栈（构造验证）

## 目标

1. G3：`run_c1_frequency_cleanup_smoke.ps1` 三条 go test 后补 `$LASTEXITCODE` 检查（FAIL 仍 exit 0=假绿风险，257 首轮实证）；
2. ps1:147 `$RunDir=""` try 块前无条件清空参数缺陷（255 MOVE-NOTE 取证）——修为尊重传入参数；
3. G7 跑腿前置清单标准化：正斜杠路径+显式 -KernelExe 两前置写入 `scripts/` 下的跑腿 README 或卡模板注记（落点执行侧定，最小面）；
4. **（258 遗留）`run_free_state_d1_smoke.ps1` LW/D1-SETTLE 相位门分支化**：zero-mutation 分支（256 已合法化的 runner 分支）下 watch hook 懒注册无触发器→LW 四门/D1-SETTLE 门不可达，当前实现 throw 拦断后续相位——改为该分支下记「分支不可达」（skip-with-reason 出报告）而非 throw，使任一分支都能产出完整判定面；1-mutation 分支门组语义零改动；构造验证=两分支判定面各自的桩路径。
- 验收=四处修改+构造验证（故意 FAIL 的桩确认 exit≠0；-RunDir 传入生效；分支化两态桩各出判定面）。

## 领取/回执/验收

- 领取：2026-11-06 18:14 +0800 / origin/main=929d6cd9（领取时 worktree 新建于该基线，git status 干净、零已有 diff）/ owner=GLM-5.3 执行流（ZCode 会话，PC）/ 分支=port/smoke-script-hygiene-1 / worktree=D:\Vit_DAW_wt_260impl / 领取提交=bfaeb324（重推基线 ff6b8318=259/261 领取笔之上，文件域无冲突核对后 rebase 重推）
- 回执：2026-11-06 18:4x +0800 / 实现 commit=6b47c178@port/smoke-script-hygiene-1（已推 origin）/ 工件=coord/runs/SMOKE-SCRIPT-HYGIENE-1/construct_20261106/（同 commit 入库，见 [RESULTS.md](../../runs/SMOKE-SCRIPT-HYGIENE-1/construct_20261106/RESULTS.md)）。
  - 四处修改全落地：①c1 三条 go test 后 `$LASTEXITCODE` 检查（throw 点名失败组，finally 清理保留）；②tracklist ps1:147 `$RunDir=""` 删除（param 默认值兜底，-RunDir 传入生效）；③G7 前置清单落 `scripts/SMOKE_TESTS.md` 顶部新节「Smoke-Run Prerequisites (G7)」（卡面授权落点执行侧定，对齐既有 Draft Root 契约节模式，+21 行）；④d1 冒测在 runner exit 0 后读报告 `validation.branch`——zero_mutation_boundary 时 LW/D1-SETTLE 两相位各写 skip-with-reason 判定面（`phase_reachable=false`+全门 `skipped_branch_unreachable`+`skip_reason`，LW 面含 setup 证据）不 throw 不拦断；分支标记缺失/陌生落向全门路径（fail 向诚实门）；1-mutation 分支门组体字节零改动（仅 if 条件+追加 elseif，无重排缩进）。
  - 构造验证（零 LLM 零真栈，桩全在 %TEMP% 隔离仓，pre-fix 对照=git show 929d6cd9 可复现）：probe1 三腿（pre-fix 假绿 exit 0 复现 / post-fix FAIL 桩 exit 1+点名 / post-fix PASS 桩 exit 0）、probe2 四腿（pre-fix 忽略 -RunDir 复现 / post-fix 传入目录 run_meta+teardown 落位 / 默认路径不变 / -PreflightOnly 面完好）、probe3 四腿（post-fix zero-mutation exit 0+两 skip 判定面+BRANCH-SKIP+PASS 尾 / 1-mutation LW 门照跑诚实失败 throw / 1-mutation settle 门照跑诚实失败 throw / pre-fix 258 形态复现=throw 拦断 settle 相位 d1_settle_gates.json 缺席）——三 probe ALL PASS exit 0，机器可读汇总 probe1/2/3_summary.json。
  - 端测边界声明：本卡零真栈口径=卡面定义（构造验证）；桩腿证明脚本层分支调度与判定面产出，不证明真实栈上分支不可达事实（该事实由 258 verifyrun1 真栈工件钉死，本卡在其上做相位门分支化）。probe2 legB/C 以 exit 1 收尾（kernel 桩 3s 听门超时）为桩腿预期形态，RunDir 语义在 kernel 启动前已完全落盘。
  - 文件域偏差披露：卡面写「目标=scripts/ 四处」但第②项目标文件实际在 `coord/runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/run_tracklist_fix_probe.ps1`（ps1:147 所指），按卡面条目文义执行，diff 中含该文件。
  - 上交观察（域外不修）：`coord/runs/TRACK-LIST-GAP-1/run_tracklist_gap_probe.ps1:147`（139 runner，②复制源）同型 `$RunDir=""` 缺陷——是否追认/另开小卡交决策侧。
- 验收（2026-11-06 晚窗，主管）：**pass**——四处全落地+三 probe 11 腿 ALL PASS（pre-fix 对照可复现=取证质量高）；④分支化=258 裁定①落地（1-mutation 门组体字节零改动亲证），258 挂账脚本面清偿；257 G3/G7 关闭。文件域偏差（②目标文件实在 coord/runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/）**追认**——发卡侧行号锚点未带全路径（主管发卡瑕疵入账），执行侧按条目文义+主动披露=良性。域外观察（TRACK-LIST-GAP-1 ps1:147 同型）**裁定不修**：历史取证工件目录不改动（取证一致性），复用时再修，known-issue 记录不入池。合入 631b54a2 零冲突。详见 [rulings/2026-11-06-SMOKE-SCRIPT-HYGIENE-1-pass.md](../../rulings/2026-11-06-SMOKE-SCRIPT-HYGIENE-1-pass.md)。
