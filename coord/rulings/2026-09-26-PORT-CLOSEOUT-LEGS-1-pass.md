# Ruling：PORT-CLOSEOUT-LEGS-1 pass（2026-09-26）——三天弧线终验

- 卡：`coord/cards/done/2026-09-26-PORT-CLOSEOUT-LEGS-1.md`（Mac 决策会话开卡，mac 执行侧执行，Mac 决策会话验收）
- 提交：回执（coord 收口提交）；回撤分支 `9e5f7b6` 经决策侧审查 cherry-pick 入 main=`2c32dcd`；前端仓 `port/leg5-telemetry-probe`=`95e362f`（本地仓决策侧直审采信）
- 裁定：**pass**

## 决策侧核验

1. **腿 1a（默认旋钮实证）**：run3b 全形态 **all_green**（决策侧直读 journey1_report 分辨：leg1c 的 inconclusive=`--skip-reopen` 设计态非红）；扫描段 4/4 零看门狗+719 全量；**实测 poll 间隔 2.00s×4 轮与 harness 默认 2000ms 精确吻合**（131/262s 级折算，旧 250ms 形态应 4 倍频——定量实证非仅"没超时"）。
2. **腿 1b（回撤）**：diff -8/+1 亲核（纯回撤 ebd0428 冗余传参）；回撤后冒烟（设计态 inconclusive）+run3b 全形态双绿；已合入 main=`2c32dcd`。
3. **腿 2（补件应用）**：前端仓 diff **+83/零删除纯增量亲核**（telemetry_manager.gd）+probe +323 落位 tools/diagnostics/ sha 一致；**增量叠加而非整覆盖的决策采信**——投递件缺 mac 侧两块已提交功能（L3READY 对账+B4A same_material，即 PC 剥离的"他人 WIP"在 mac 是已提交内容），整覆盖=回退，执行侧判断正确且 commit 注记源 sha；probe headless 真跑全断言 PASS+autoload 零 SCRIPT ERROR（standalone check-only 误报形态性说明采信）。
4. **F6 登记（§11 处置）**：模型自挂 EQ 后以无观察证据拒写参数的 a3/s5 红形态——原始输出保全+同代码复绿+与本卡 diff 零关联**三条件齐**，登记为**模型方差族 F6**（与 F4/repair-clarify 族并列入账，供后续统计批观测分布），非缺陷不开卡；复发成簇再议。
5. §8/§9 账目：journey 五轮（两红一设计态一环境中断一全绿）逐轮工件不覆盖，环境中断（FetchContent 断连）有原始日志排除合规。

## 弧线总账（2026-09-23 → 09-26，REALSTEMS 起点）

| 里程碑 | 卡 |
|---|---|
| 真实素材首次端到端 + EQ 动机化实证 | PORT-REALSTEMS-MAC-1 |
| Q10 认证+白名单 static_eq 补齐 | PORT-WL-EQ-1 |
| identifier 补发（shell 成员可达）双端 | FIX-D1-PLUGIDENT-1 |
| 前端 mac 扫描路径+两层机器态清污 | FIX-FE-SCANPATH-MAC-1 |
| v6 多候选+LLM 自选复活（用户设计回归） | FIX-PLUGIN-SELECT-1+MAC 腿 |
| 候选面全量派生（用户裁定"全量可选"） | FULL-CANDIDATES mac/PC |
| clarify 链黑箱全解+判死软化 | FORENSIC+FIX-REPAIR-CLARIFY-DEATH |
| 全库认证扫描双端（跨平台确定性 0/705） | AUTOSWEEP+MAC 首轮 |
| 认证 token 解锁（104 例） | CERTAUTH-TOKEN+吸收腿 |
| 插件表卫生+bundle 谓词（持久化恢复） | HYGIENE+BUNDLE |
| 扫描轮询修复+telemetry 标注 | SCANPOLL+腿5 应用 |

**终态**：mac 候选面 **83（EQ 43）**、PC 100；重启零清理持久化；84 包 0 FAIL；模型自选+全链（假设→准入→装载→写参→A/B）双端常态。新失败族账本：F6（模型方差）+repair-clarify（已修）。

## 遗留

- 转交 PC：HYGIENE-BUNDLE 合入后 Windows 形态烟测复跑（37ae4f4 后）。
- 前端仓 port/leg5-telemetry-probe 分支保留待 PC 侧对齐其仓状态后择机并入其主线。
- F6/负载型 flaky（F5 并发双写）两观察项入统计批。
