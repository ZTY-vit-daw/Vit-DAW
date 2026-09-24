# FIX-PCA-AUTOSWEEP-1 sweep_report

- run: `20260924_005622` generated 2026-09-23T21:06:00+00:00
- library effect subjects: 974 (probe ok/fail: 970/4; instruments excluded)

| family | classified | promoted now | certified this sweep | whitelist derived |
|---|---|---|---|---|
| multiband | 33 | 10 | 0 | 8 |
| limiter | 41 | 21 | 14 | 9 |
| de_esser | 32 | 10 | 1 | 6 |
| gate_expander | 25 | 12 | 7 | 3 |
| transient_shaper | 34 | 5 | 0 | 2 |
| broadband_compression | 51 | 56 | 33 | 24 |
| static_eq | 68 | 0 | 0 | 1 |

- bucket_certified: 55 rows
- bucket_exceptions: 187 rows (each with reason)

Full lists: sweep_report.json

## 验收④：新增多候选族（broadband_compression）真栈自选腿五环证据链

- journey run2 `journey2/`（exit 0，JOURNEY1_VERDICT **all_green** 4/4）：
  - 披露：模型收到 24 条 broadband 候选披露（run1 回复明确列举 AMEK/ADPTR/API-2500 且标注"PCA 已准入"）
  - 选择：模型自选 **Pro-C 2**（`VST3-Pro-C 2-f1a6549f-763029ad`，本次扫描 pca_job_9fe8a46a59d180d3 认证、v1 store pca1_ccf143bc9bf73f8 promoted、白名单 24 候选之一）
  - 准入：`pca_load_gate_denials=0`，journey_turn_landed=True
  - 绑定+写链：`proposal_applied=True`、`card_mounted=True`、audition events（prepare.started→candidate.ready→ready×2）+ `mix_tick.pending`
  - a2 直探针（脚本腿）：**CLA-2A Mono**（另一新晋升候选）rack 实例化成功（plugin_id 1040、instance_ready、UI 投影落面、gate 零拒绝）
- run1 如实记录：a3 红（模型在"先观察"门前收工，回复披露了候选但未装载）→ 按概率运行纪律换链路顺序提示词后 run2 全绿
