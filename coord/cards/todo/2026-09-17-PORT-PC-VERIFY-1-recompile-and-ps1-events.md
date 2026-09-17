# PORT-PC-VERIFY-1：PC 复编译确认（A3+A1）+ ps1 events 探针契约修复

- 优先级 / 预估 / 依赖：P2 / 0.5 天（PC 侧执行，验收尾欠 + 域外缺陷小修合并一张零星卡）/ 无
- 模型分级：L1 / flash（机械验证 + 微修复；MSVC/CMake 报错若涉语义判断再升级）
- 背景：① A3/A1 两卡改动均为平台守卫式，但 Windows 复编译确认按裁定归 PC 会话（累计两卡待闭合）；② C4 验收发现 `scripts/g_runtime_readonly_smoke.ps1` 的 events 探针裸 `?limit=200` 对真实 agent 必 400（chat/events.go 契约要求 conversation_id）——ps1 版潜伏缺陷，mac 版已修正
- 子项 A（复编译确认）：`D:\Vit_DAW` 拉最新 main（含 `d33f933`/`0ea259a`/`79d4757` 等），VitApp CMake 全新 configure + build exit 0（重点验证 A3 的 `LANGUAGES C CXX` 与守卫块、A1 的 SharedMemorySegment 接口在 MSVC 下编译）；agent `go build ./...` exit 0；结果回执本卡（发现编译失败 → 附原始错误转 blocked，不许自行改 VitApp 代码）
- 子项 B（ps1 events 修复）：`g_runtime_readonly_smoke.ps1` events 探针补 `conversation_id=<probe>&`（对照 mac 版 `g_runtime_readonly_smoke_mac.sh` 与 SMOKE_TESTS.md 说明）；改后对 fixture server 与真实 agent 各验一次 exit 0
- 文件域：子项 A 只读验证（零源码改动）；子项 B 仅 `scripts/g_runtime_readonly_smoke.ps1`
- 验收标准：A 与 B 各自 exit 0 证据（命令+退出码）入回执；B 附 ps1↔mac 对照说明
- 停止条件：MSVC 编译失败且原因在守卫块内语义（非明显笔误）→ blocked 上交由决策侧裁定
- 领取：
- 回执：
- 验收：
