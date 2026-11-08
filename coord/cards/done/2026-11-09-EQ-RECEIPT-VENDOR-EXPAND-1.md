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
- 回执：（2026-11-09 02:20 +0800）状态=**执行侧自验完成（doing→done）**，实现 commit=65fce226 @ port/eq-receipt-vendor-expand-1（已推 origin，不合入 main 归主管）；零栈零 LLM 零计费，无 run ID。
  - **形态取舍（回执说明项）**：采 **FabFilter 专用 vendor-scoped kind** `fabfilter.static_eq.eq_channel_live_smoke.v1`（const `SchemaFabFilterEQLive`）。理由：①改动面实测**等大而非更小**（两形态同为 1 枚举+1 case 扩展+1 映射行+配对测试）——卡面 neutral 倾向的成立条件（"若 neutral 改动面更小"）不满足；②fail-closed 最保守：schema 白名单仅增被授权厂商的 1 个字符串，neutral kind 一旦入白名单任何厂商盖同戳即过 schema 门；③命名/溯源与既有两枚一致（vendor.family.channel.v1），promoted attestation 的 EvidenceRef.Kind 溯源明确；④与 FIX-PCA-AUTOSWEEP 验收注记既定实践一致（厂商扩容=独立最小 commit 卡）。**停止条件未触发**：schema_version 零改动（纯加法枚举）、既有收据往返零影响（旧 schema 字符串与解析路径原样）——两红线亲核通过。勘察注记"eqCandidate 厂商无关可复用"亲证兑现：resolveSubject 按 identifier 对语义索引解析不查厂商，新 case 直入既有 eqCandidate 零分支逻辑。
  - **diff 文件清单**（4 文件 +165/-4，全在卡域）：`agent/internal/processorauthority/receipts.go`（枚举+switch 各 1 行级）、`agent/internal/processorauthority/receipts_test.go`（新增 128 行：TestFabFilterEQChannelReceiptAdmitsProQ3 按 sweep 写出器形态 fixture 解析/准入晋升+TestUnknownEQVendorReceiptKindsStillFailClosed（含 neutral 拼写/未知版本/未知厂商三控）+TestWavesEQLiveReceiptStillAdmits）、`scripts/pca_autosweep.py`（EQ_RECEIPT_KIND 增 fabfilter 行+注释与 vendor_kind_missing 错误文案同步真值，未映射厂商仍拒）、`coord/runs/EQ-RECEIPT-VENDOR-EXPAND-1/dryrun_zero_stack.txt`（干跑工件，§6.1 入 port 分支）。
  - **测试命令与退出码**（worktree D:/Vit_DAW_wt_323receipt @ 65fce226）：`go build ./...`→exit 0；`go test ./internal/processorauthority/... ./internal/processorattestation/... ./internal/pluginsemantics/... -count=1`→exit 0 三包 ok（含既有 PA 收据测试=零回退证据）；新测试具名单独复跑 4/4 PASS；gofmt 内容级零差（-l 全包标记为预存 CRLF 状态，origin/main 原始文件同样标记+`gofmt -d` 零内容差亲证，非本卡引入）。
  - **干跑亲证（零栈）**：①生成门（worktree 副本 py_compile OK+certify_eq 查找逻辑复刻）：manufacturer='FabFilter'→kind 解析 `fabfilter.static_eq.eq_channel_live_smoke.v1`，**vendor_kind_missing 分支不再触发**；TDR/未知厂商→None 仍 vendor_kind_missing（生成门 fail-closed 维持）；②导入门（真实语义索引 991 条只读加载+Pro-Q 3 identifier 唯一匹配 1）：按 EQ-channel 写出器精确形态构造的 Pro-Q 3 收据盖新 kind 戳→ImportReceipts 临时库（t.TempDir 级）晋升 1 条（family=static_eq、coverage upsert/undo/modify/disable×bell、evidence kind 正确）；neutral 拼写对照被真实门拒绝（unsupported receipt schema）；③**不落库亲证**：真实 v1 库 `~/.vit/processor_control_attestations.v1.json` 干跑前后 sha256 一致（cbaecd42…6561f），`~/.vit` 本卡零写入，白名单/认证目录未触。
  - **322 解锁条件达成**：两道门（sweep 生成门 EQ_RECEIPT_KIND+agent 导入门 ReadReceipt 枚举）均已扩容，derive/披露链勘察亲证零改动直通（derive_eq_whitelist 按 cert state+explain 证据推导不查 schema 字符串）；322 腿一可按原勘察路径直跑（token 在位亲证承 322 卡面）。
  - **端测边界声明**：零栈卡——无真实栈 run、无 LLM 轮、无渲染面/用户旅程主张；真实认证落库（真栈 certify→import→derive→白名单披露）归 322 腿一执行，本卡仅证明代码门与收据形态。
- 验收：（主管回填）
