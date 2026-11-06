# SMOKE-ENC-HYGIENE-2：run_free_state_d1_smoke.ps1 编码读取卫生残留两处（268 挂账清偿）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[268 裁定](../../rulings/2026-11-06-D1GATE-SCRIPT-HYGIENE-1-pass.md) §二挂账——跨段同族两处记录不修，随下一张卫生卡）
- 派发确认：已确认（主管发卡，自主批次·freeflash 车道）；验收负责人：GLM 主管决策流
- 池序 270；目标=scripts/run_free_state_d1_smoke.ps1 两处；P3 / 10 分钟级 / 无依赖；L1 / PC；零真栈（构造验证）

## 目标

1. `:630` NOT_EXERCISED reason 读取加 `-Encoding UTF8`（268 同族第四/五处）；
2. `:445` LW manifest 读取同款；
3. 构造验证：两读取点桩路径（离线文件，模式参 268 probe；最小两腿即可——pre-fix 乱码/吞 throw 对照+post-fix 正常读取）。

## 约束

文件域=该 ps1 两行+构造工件；断言语义零改动；268 已修四处零回退。

## 停止条件

发现第三处同族→顺手修+记录（同文件域内）。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
