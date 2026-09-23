# Ruling：PORT-PCA-FULL-CANDIDATES-1 pass（2026-09-24）

- 卡：`coord/cards/done/2026-09-23-PORT-PCA-FULL-CANDIDATES-1.md`
- 提交：领取 `5469d58` / 回执 `2873f78` / 构建器 `a1f2df6`（port/whitelist-builder-1）经决策侧 cherry-pick 入 main=`c9dbeb1`
- 裁定：**pass**——用户裁定"所有 PCA 批准可用的插件都应该可被选择"在 mac 落地：候选面 3→23

## 决策侧核验（独立复算）

1. **白名单**：v6 实核 23 条目七族齐——static_eq 3（承接）/broadband_compression 2（C1 comp M/S，**共享单参形态 keys=`threshold_param_id` 实证**，即 BROADBAND-SHARED 新形态的首次生产落位）/de_esser 4/limiter 4/multiband 6/gate 2/transient 2；sha256 `324ee2ac…` 与回执一致；三候选版备份（`38176f05…`）在工件目录作演示回滚网。
2. **4 排除合规**（溯源 159 行逐条带 S2 覆盖轴事实）：RDeEsser M/S=coverage 无 threshold 轴；TransX Wide M/S=envelope_emphasis≠envelope_timing（裁定③口径）——EMO-F2 先例的正确延续，"27 预期→23 如实"。
3. **构建器入仓**：`scripts/build_whitelist_v6_full.py`（461 行，五源溯源+broadband 共享单参+ch 形态 fail-closed）——**开源 backlog"白名单构建器未入仓"项兑现**。
4. **overlay**：11 PASS（七族 membership 23/23+活 store admission+空 pin/非成员 spot+共享单参+三候选备份回归）。
5. **验证腿**：journey run2 all_green 9/9、披露 6 次、零 membership 拒绝签名；**非 EQ 族真栈证据=S4 探针 C1 comp Mono `component_create success`**（broadband 候选真实可载）；模型未动机化非 EQ 族如实记录（p01 配方指向 EQ，未触达≠失败）——§8 合规（run1 占位分支记账加跑）。
6. **agent 重建**：23:38 sha `e4552df0…`（含 broadband loader），演示面二进制就绪。

## 遗留移交

- mac 候选面 23 已就绪：明日起演示形态=模型在 23 个认证插件中自选（EQ 3 选+broadband 2 选等按动机化触达）。
- PC 侧：PORT-PCA-FULL-CANDIDATES-PC-1 步 2 可用本卡入仓构建器（c9dbeb1）；AUTOSWEEP 落地后 mac 跑首轮 sweep，白名单按 sweep 结果再生长。
- 队列下一张：FORENSIC-MAC-CLARIFY-CHAIN-1（纯只读取证，读冻结工件零栈占用——与任何卡无物理冲突，随时可发）。
