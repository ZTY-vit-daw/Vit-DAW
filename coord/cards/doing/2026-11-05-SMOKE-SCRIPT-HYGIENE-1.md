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

- 领取：2026-11-06 18:14 +0800 / origin/main=929d6cd9（领取时 worktree 新建于该基线，git status 干净、零已有 diff）/ owner=GLM-5.3 执行流（ZCode 会话，PC）/ 分支=port/smoke-script-hygiene-1 / worktree=D:\Vit_DAW_wt_260impl / 领取提交=（推送后回填 hash）
- （执行侧回填回执）/（主管回填验收）
