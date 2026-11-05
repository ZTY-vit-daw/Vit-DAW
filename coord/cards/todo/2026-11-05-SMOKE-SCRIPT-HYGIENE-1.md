# SMOKE-SCRIPT-HYGIENE-1：烟测脚本卫生批（G3 退出码+RunDir 参数+跑腿前置清单）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 裁定](../../rulings/2026-11-05-AF-FAMILY-HEALTH-1-pass.md) G3/G7+255 上交 ps1:147）
- 派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 260；目标=scripts/ 三处；P2 / 0.25 天 / 无依赖；L1；零真栈（构造验证）

## 目标

1. G3：`run_c1_frequency_cleanup_smoke.ps1` 三条 go test 后补 `$LASTEXITCODE` 检查（FAIL 仍 exit 0=假绿风险，257 首轮实证）；
2. ps1:147 `$RunDir=""` try 块前无条件清空参数缺陷（255 MOVE-NOTE 取证）——修为尊重传入参数；
3. G7 跑腿前置清单标准化：正斜杠路径+显式 -KernelExe 两前置写入 `scripts/` 下的跑腿 README 或卡模板注记（落点执行侧定，最小面）。
- 验收=三处修改+构造验证（故意 FAIL 的桩确认 exit≠0；-RunDir 传入生效）。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
