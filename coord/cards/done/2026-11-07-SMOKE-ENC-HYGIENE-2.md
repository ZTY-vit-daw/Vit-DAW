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

- 领取：2026-11-07 / owner=GLM 主会话 freeflash 车道子代理（ZCode GLM-5.3-Flash，PC）/ 领取时 origin/main=8732bd213a901bfa86ef63e857983f2332e15bd8 / 分支=port/smoke-enc-hygiene-2 / worktree=D:/Vit_DAW_wt_270impl
- 回执：2026-11-07 / 实现 commit=4a6c0f9b（port/smoke-enc-hygiene-2，基于领取基线 8732bd21）——`scripts/run_free_state_d1_smoke.ps1` 仅 2 行改动（+2/-2）：:445 LW manifest 读取、:630 NOT_EXERCISED reason 读取各补 `-Encoding UTF8`，断言语义零改动，268 已修四处零回退（行号未漂移，与卡面一致）
- 构造验证（零真栈零 LLM）：probe_270 attempt3（20261107_001930）**exit 0，5/5 ALL PASS**——leg1/leg3=pre 基线行对 UTF-8 无 BOM 中文桩读取乱码实证（GBK CP936 误读形态），leg2/leg4=post 行 clean_parse 逐字节正确读回（try/catch 同脚本 :629-:633 原形制），leg5=静态扫描 post 快照 7 处 Get-Content 全带 -Encoding（268 四处+本卡两处零回退全覆盖，268 leg1 形制由 leg2/4 同构覆盖未重做）；第三处同族=无（全文件扫描）。工件：coord/runs/SMOKE-ENC-HYGIENE-2/construct/（RESULTS.md 证据链+probe_270.ps1+pre/post 快照+attempt1/2 失败留档+attempt3 全套）
- 执行侧自验完成 → done（待主管验收；本卡零真栈构造验证，无需真栈烟测；attempt1=提取 pattern 歧义、attempt2=桩漏最外层 `}`，均为 probe/桩面问题已留档，非被测脚本问题）
- （主管回填）
- 验收（2026-11-07 凌晨，主管）：**pass**——两行修补（:445/:630 -Encoding UTF8）+构造验证 5/5+**主管交叉复跑 5/5 ALL PASS 亲跑**（pre GBK 乱码实证/post 正确对照/全库扫描 postfix 未编码=0 且 pre 残留恰 2 处对照成立）；268 四处零回退；第三处同族=无；probe 三轮留档（前两轮失败均 probe 自身问题如实归档）=freeflash 车道第二单质量合格（零断线）。合入随本批。
