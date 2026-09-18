# 决策记录：U2 收窄——Mac 定位为展示机，校准面 31→23 主体（2026-09-16 晚）

- **用户裁定**（经 Mac 执行侧回执附录转达，[docs/MAC_OPEN_RECEIPT_2026-09-16.md](../../docs/MAC_OPEN_RECEIPT_2026-09-16.md) §6）：mac 侧定位为**展示机**，只安装 Waves 系列；PA（5 主体）与 FabFilter（3 主体）不装
- **影响**：校准面 31→**23 主体（全部 Waves）**；**U1（PA mac 许可覆盖）自动免除**；U2 以"23 主体已装且授权覆盖"结案；C2 校准脚本与 C3 重探针/重认证的执行计划按 23 主体口径；PCA 白名单 mac 预期同步收窄
- **文件层核查证据**（Mac 侧，2026-09-16）：WaveShell1-VST3 17.1.42 @ `/Library/Audio/Plug-Ins/VST3/`（含 ARA 变体），universal（x86_64+arm64）；12 目标 bundle 全在位（C1/C4/C6/L1/L2/LinMB/DeEsser/RDeEsser/Sibilance/Smack Attack/TransX/PSE）；Ultimate Full License、Native 平台、无过期
- **风险登记更新**：R2（WaveShell mac 加载）文件层前置全部就绪，真机宿主枚举仍待 A5/C3；R9 新增细节——shell 为 adhoc/linker-signed，dev 期未签名内核加载预期无碍，公证分发期重估
- 附录 commit `6c6e2df9`（port/c1-plugin-scan-paths）验收 pass，cherry-pick 入 main

> **更正注记（2026-09-18，决策侧，依据 PORT-C2-PCR 双端实证）**：本决定中的"23 主体"系收窄算术（31 全场面 − 5 PA − 3 FabFilter = 23）的推导数，**双端机器枚举从来都是 24 体**（12 族 × Mono/Stereo；PC 当前扫描与 2026-07-28 旧基线索引均 24 体）。校准面权威口径已由 2026-09-18-C2-pass / C2PCR-pass 裁定为 **24**，本决定的语义（全部 Waves、无 PA/FabFilter）不变。
