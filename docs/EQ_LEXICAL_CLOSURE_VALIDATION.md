# EQ 通用词法层收尾验证

日期：2026-07-28
分支：`codex/eq-structural-matrix-unification`

## 修复范围

本次是 EQ 通用识别器冻结前的最后一次小范围修复，只包含身份无关的词法与物理读回统一：

- 工程频率记数：`1k0`、`2k7`、`3k3`；
- 地区小数格式：`0,5`、`0,7`；
- band-position 缩写：`HM/LM/LF/HF` 与 High Mid/Low Mid/Low/High section 对齐；
- 离散物理枚举即使精确命中也不进入连续二分校正；normalized 与物理读回仍严格校验；
- topology discovery 与执行 readback 共用同一 localized-number parser。

没有加入插件名称分支、外部注解层、SPAL、profile、learn、B4 或音频主动探测。

## 自动化验证

- 定向结构测试覆盖 section 合并、工程频率、小数逗号和离散物理写入语义；
- 历史 16 capture：14 recognized / 14 executable，相对修复前 0 回退；
- Waves 50 capture：44 recognized / 38 executable，相对修复前 0 回退；
- 离线产物：`artifacts/eq_lexical_closure/20260728`。

## Knif Audio Soma 真实复测

冻结既有测试对象，不新增外部插件。最终产物：

`artifacts/plugin_alliance_eq_two_level/20260728_134330`

Bell 请求：3400 Hz / -3 dB / Q 0.5。

- High-Mid Frequency：量化为 `3k3`，实际 3300 Hz；
- Gain：`-3.0` exact；
- Bandwidth/Q：`0,5` exact；
- 正式 `operation_ref` undo 恢复三个参数；
- 最终完整分页零漂移。

Low Cut 80 Hz 继续安全拒绝，因为真实枚举只有 Off / 25 / 33 / 50 Hz。

审计为 0 audio probe / learn / profile / SPAL / B4；受保护运行时文件 byte-identical；5555、5556、7878、8787 端口全部释放。

本次完成后冻结 EQ 通用核心，不继续实现 B authoring 或更多特殊 topology。
