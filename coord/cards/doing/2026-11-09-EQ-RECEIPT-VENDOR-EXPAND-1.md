# EQ-RECEIPT-VENDOR-EXPAND-1：static_eq 收据厂商扩容（FabFilter 收据种类——Pro-Q 3 认证解锁件）

- 发卡：GLM 主管决策流 / 2026-11-09 凌晨（依据=[322 勘察上交](../blocked/2026-11-09-PROQ3-CERTIFY-AND-T7-1.md)——两道厂商门锚点+2026-09-24 sweep 铁证[Pro-Q 3 三条 cert_done 全诚实拒 vendor_kind_missing]+[FIX-PCA-AUTOSWEEP 验收注记](../done/2026-09-23-FIX-PCA-AUTOSWEEP-1.md)「vendor_kind_missing 扩容=独立最小 commit 卡候选」预言在案）
- 派发确认：已确认（主管发卡·解锁 322）；验收负责人：GLM 主管决策流
- 池序 323；目标仓库=agent processorauthority+scripts 映射行；P2 / 0.25 天；L2 / PC；零栈

## 目标

1. **agent 收据种类扩容**：`agent/internal/processorauthority/receipts.go`（:21-27 枚举+ :155-193 switch）增 **FabFilter static_eq 收据种类**——倾向 **vendor-neutral** 形态（勘察注记：eqCandidate 厂商无关可复用——若 neutral 改动面更小则采 neutral，回执说明取舍）；fail-closed 红线维持（未知厂商仍拒）；
2. **脚本映射行**：`scripts/pca_autosweep.py` :348-351 EQ_RECEIPT_KIND 增 FabFilter→static_eq 映射（1 行级）；
3. **配对单测**：FabFilter static_eq 收据解析/准入通过；未知厂商 fail-closed 维持；既有 waves/PA 收据零回退（全包 processorauthority+相关包回归）；
4. **干跑亲证**（零栈）：Pro-Q 3 sweep 收据生成路径 dry-run——vendor_kind_missing 消失、产出合法收据形态（真实认证落库归 322 腿一，本卡不落库）；
5. **解锁 322**：本卡合入后 322 回 doing 跑认证+T7×3 一次栈窗口收官。

## 约束

文件域=processorauthority/receipts.go±测试+autosweep 一行；不触白名单 derive/披露链（现成链路勘察亲证无需改）；持久化兼容（§11：收据种类枚举新增=旧收据零影响，fail-closed 语义不变）。

## 停止条件

勘察发现 neutral 形态需动 schema_version 或破坏既有收据往返→取证上交转裁定（不硬改）。

## 领取/回执/验收
- 领取：2026-11-09 02:10 +0800 / origin/main=d0215c897e192b8e54d3fd488594387f56a7035c（fetch 亲证=发卡收口提交）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 323 eq-receipt-vendor-expand，派发已确认）/ 协调分支=coord/323-claim（独立 worktree D:/Vit_DAW_wt_323coord，fresh 自 origin/main）/ 执行分支=port/eq-receipt-vendor-expand-1（独立 worktree D:/Vit_DAW_wt_323receipt，自领取后 origin/main 新建）/ 领取提交=本卡 todo→doing 推 main。doing 五卡（EDU-ASK-ANCHOR/ARRLOOP-AUTHBRIDGE/CLOSURE-ROUTE-ADAPT/WAVESHELL-OFFRENDER-FIX/S1B）文件域 grep 亲证零重叠；322 铁证账本亲读（FIX-PCA-AUTOSWEEP-1/20260924_005622/run_ledger.jsonl：Pro-Q 3 cert_done vendor_kind_missing×2 在卷）；Pro-Q 3 identifier（VST3-Pro-Q 3-a1aba170-ba5f7ab2）真实语义索引唯一匹配亲证（991 条中 1 条）。零栈卡不登记 PC-RUNTIME-STACK。
- 回执：（执行侧回填）
- 验收：（主管回填）
