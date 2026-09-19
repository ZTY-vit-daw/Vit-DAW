# Ruling：PORT-SMOKE-MAC-1 pass（2026-09-19，交付面；六件红定性上交 A-G 七项裁定）

- 卡：`coord/cards/done/2026-09-19-PORT-SMOKE-MAC-1-product-path-suite.md`
- 实现：`port/smoke-mac-1` @`8fbba03`（7 个 `_mac.sh` + SMOKE_TESTS 7 条目）→ cherry-pick 入 main `639c8af`
- 裁定：**pass（按停止条件"单件上交、其余继续"口径）**——文件域交付完整、组锚 ab_result 绿（健康检查核心件在 mac 达成 exit 0）、§8 账目诚实（含⑤止损执行）、六件红全部根因定性且**无一为移植质量问题**：4 件系 PC 套件在当前代码上同样无法通过的契约腐化（A/B/C/F，执行侧以代码锚点证明）、2 件系 mac 侧真实缺口（D/E，均在卡文件域外）、1 件预算耗尽待授权（G）。

## 决策侧核验

7 脚本+SMOKE_TESTS+对照表+工件（`~/Documents/vit-smoke-mac1-artifacts/`）在位；ab_result 权威 run `20260919-191440` exit 0（mixboard/mom/chat 9 测试）；⑤ run2/3 前段全绿清单双证；⑦ runA/runB step 级账目在 summary.json；LLM key 零入工件复核口径同 JOURNEY。

## A-G 七项裁定

| # | 事项 | 裁定 |
|---|---|---|
| A | ② ps1 对 .vit 做 ET.parse，而内核无条件存加密容器（VitHeadlessService.cpp:1218，VIT1 magic） | **探针腐化成立**——断言升级为"VIT1 容器 magic + 重开持久化往返"（执行侧栈级检查已绿）；ps1+mac 双端同步升级 |
| B | ③ defer_audio_analysis 已改 opt-in（默认 queue+run），ps1 断言旧语义 | **探针腐化成立**——断言升级为"queued/run 语义"（8 jobs 证据在案） |
| C | ⑥ mom_version 断言 v1.4，实现为 v1.5（types.go:3） | **断言漂移成立**——双端断言升 v1.5 |
| F | ⑤ vocal focus 终态路由=ccb.observation_catalog，ps1 别名组只认 mix.observe 族 | **路由契约漂移成立**——别名组纳入 CCB 路由（run2/3 同断点确定性已证）；PC 套件当前同败 |
| A/B/C/F 合并 | | → 开 **PORT-PS1-SYNC-1**（PC）：四个契约双端断言升级（ps1+`_mac.sh` 断言块），PC 侧跑绿证明契约正确 |
| D | ⑦ dad_probe.py `mmap(tagname=…)` Windows 专属，mac TypeError | **mac 缺口成立**——dad_probe 增 POSIX shm 读取器（scripts/*.py 原卡域外）；L3 内核侧特征完好（l3_acoustic_summary=ready 已证），仅读取器缺口 |
| E | ⑦ mac 前端 ready 相位 L3 顶层行被 VitTelemetryManager 重置（PC 保留 spectral_tile_derived） | **mac 前端行为差异成立**——开取证+修复卡（前端仓域），live 行断言全对的边界采认 |
| G | ④ 驱动侧提取器排序一行修复已 staged，预算 3 轮耗尽 | **授权补跑一轮**（决策侧裁定：修复在测试驱动侧非产品代码、needle 命中证据在案）——并入 SMOKE-MAC-2 执行 |

## 后续批次

1. **PORT-PS1-SYNC-1**（PC）：A/B/C/F 双端断言升级 + PC 跑绿；
2. **PORT-SMOKE-MAC-2**（Mac）：实验链 py 组 5 件（b1×4+b4）+ ②③⑤⑥ 升级后复跑 + ④ G 授权补跑 + D dad_probe POSIX 读取器；
3. **PORT-FE-L3READY-1**（Mac）：E 前端 ready 相位 L3 行重置换证与修复。
