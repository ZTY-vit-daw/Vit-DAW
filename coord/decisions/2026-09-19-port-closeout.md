# 决策记录：Mac 移植收官（2026-09-19）

- **PORT 主线完结**：PORT_AUDIT_2026-09 四层全部落地——层 A（内核+agent 配套，A1-A5）、层 B（前端，B1/B1M/B2P/B3/B4A/B4B/B6/B7A/B7B）、层 C（校准，C1/C2/C2-PCR/C3/C4）、D1（私有仓+submodule）；计划外修复 FLAKY-1/HARNESS-FIX-1/PC-VERIFY-1/PC-ADOPT-1；收官件 JOURNEY-1-MAC（五段旅程真栈全绿）。**审计九项风险（R1-R9）全部有实证结论**；用户真栈手测两轮通过（B4 启动链 + B7 WebUI 内嵌）。
- **关键资产事实**（后续开发按此）：mac 校准面=24 Waves 主体（U2 语义=全部 Waves，"23"系收窄算术产物双端勘正）；参数面指纹跨平台逐字节稳定（24/24 实证）；`~/.vit/` 双端机器本地态；校准链与 pluginprobe 探针脚本两端可重复；Godot 前端仓双端在位（mac `~/Documents/vit-daw-frontend`、PC `D:\Godot\project\vit-daw-frontend`，均 git 化无远端——同步走 transport 模式或后续建 mirror）。
- **PROTOCOL 降级**：`coord/PROTOCOL.md` 自本记录起降级为历史记录，常规纪律回归 AGENTS.md（PROTOCOL §适用条款原文）；coord/ 目录结构（cards/rulings/decisions）继续沿用，双端协作惯例（port/* 分支验收、ruling 归档、执行侧仅卡片变更直推 main、推送纪律增补条款）作为长期纪律保持。
- **遗留清单**（按需开卡，非移植欠账）：
  1. vsphub mac 移植（agent/cmd/vsphub 现仅 Windows 可用；移植后翻回 `vsp_realtime_adapter.gd` ws 默认值并还原 `start_page.gd` 两处 mac 安静化分支为真告警）；
  2. dock 核心区退出期 ObjectDB 泄漏 2 行（viewport_registry，先在缺陷，B7B 取证在档）；
  3. Ask Vit Browser overlay 面的 mac 适配（B7B 降级清单 a 项，依赖 overlay 宿主 mac 化）；
  4. export templates 未装（mac 导出预设条目就位、导出链路未执行）；
  5. 层 D 的 D2-D5（CI/机器检查/每夜构建，按需）；
  6. AGENTS.md 文档白名单更新（B2P 手册/本记录入现行索引——归 CURRENT-STATE 例行维护）。
