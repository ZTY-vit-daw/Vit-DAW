# Ruling：PORT-JOURNEY-1-MAC pass（2026-09-19，移植主线收官）

- 卡：`coord/cards/done/2026-09-19-PORT-JOURNEY-1-MAC-demo-journey-smoke.md`
- 实现：`port/journey1-mac` 6 commits（初版/早退修复/run1 取证/插件预热/run2 取证/SMOKE_TESTS 机器事实）→ 已 cherry-pick 入 main
- 裁定：**pass**

## 决策侧核验

1. **权威 run**：run3 `journey1_mac.bbuoHe7q` exit 0、九断言全绿（A1-A4 语义锚定 + S1-S5 五段显式判定），工件 `~/Documents/vit-journey1-artifacts/run3_*_AUTHORITATIVE` 在位（run1/2 红轮全量存档）。
2. **§8 纪律执行合格**：3/3 有效轮，红→红→绿，两红均定性为确定性环境缺陷且**轮间补锚点后重跑**（stems 字面单文件名/插件库扫描预热/busy 重试），无原样重跑、无止损违规、无环境中断轮混入。sanity×3 未计有效轮次。
3. **隔离契约**：前后哈希一致（用户工程/主仓 Workspace 两文件/.vit_history absent→absent）——**主仓当前工作树的 Settings.xml/default_project.xml 改动系手测内核写入（B4B 已声明预期），非本卡污染**。
4. **语义对照表**：12 段逐项锚定 ps1；唯一机器态替换（PC bx_hybrid V2 → mac C1 comp Mono）符合 U2 收窄口径且留 `--probe-plugin-identifier` 可换。
5. **LLM 前置**：初验缺失 → 用户在场裁定配置 → 三处占位符格式修复 → rightapi.ai/deepseek 真实连通；key 零入仓零入工件零入日志（工件仅布尔与名称）。
6. **边界声明采认**：GUI/渲染面卡面排除；"webui dist mac 构建+`/app`=200 探活"系环境就位顺带项（非文件域），**CEF 内嵌集成=B7 后续卡**——用户随后报告的 Safari 兜底问题正落此处（见下）。

## 移植主线状态：JOURNEY-1-MAC pass = 移植收官条件达成

层 A/B/C/D1 + 真栈手测 + 旅程冒测全过。遗留两件：①**B7：Ask Vit WebUI mac 内嵌**（用户实测发现 Safari 兜底，根因见 B7A 卡面，属审计 §1.3(a) 误判"休眠"的 WebView2 面——ClassDB 字符串引用盲区）；②收官记录文档（B7 收口时一并写，PROTOCOL 届时降级为历史）。
