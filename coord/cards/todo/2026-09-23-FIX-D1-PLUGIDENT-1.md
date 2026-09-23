# FIX-D1-PLUGIDENT-1：D1 插件写路径 normalized-batch 计划 args 补发 plugin_identifier（WaveShell 成员可达）

- 优先级 / 预估 / 依赖：P1 / 0.3 天 / PORT-WL-EQ-1 回执上交 (b)（mac EQ 装载受阻定位于此；用户 2026-09-23 呈报）；**跨族缺口非 EQ 专属**
- 模型分级：L2 / GLM-5.3（端口/内核解析语义边界，红测试先行）
- **背景（回执取证+决策侧代码复核在案）**：白名单绑定路径的计划 args 只发 `plugin_path`+`plugin_name` 漏发 `plugin_identifier`，而 binding.Plugin 里有该字段（admission 层六族都在用，free_state_d1_plan_table.go:102-408）。独立插件（PC 七族全独立）path 即唯一——PC 从未暴露；**WaveShell 包裹主体（mac 全部六族）path 指向壳内数百成员**，内核对 identifier 缺席的壳路径 fail-closed（正确防御）→ mac EQ 装载/A-B 未走完。两处同病：`d1StaticEQActionArgs`（free_state_d1_plan_table.go:573 args map）与共享 `pluginParamWriteArgs`（:676 args map，compression/de_esser/transient/limiter/gate/multiband 共用）。端口 `staticeq_vsp.go:83-84` identifier 存在即透传，零改动。已有实例复写路径（:627 plugin_id 直达）不受影响——缺口只在全新 instantiate。
- 目标：
  1. `d1PluginParamWhitelistBinding` 增 `PluginIdentifier` 字段并自 `binding.Plugin.PluginIdentifier` 填充；两处 args map 各补一行 `"plugin_identifier": …`
  2. 红测试：绑定路径计划 args 含白名单 plugin_identifier（修前红/修后绿，两个 builder 各一枚）；stub 路径（无绑定）行为不变回归
  3. **PC 回归腿（必做，语义风险）**：staticeq_vsp.go 顶部注释明示内核对非空 identifier **独占解析 known-plugin list、绝不回落 path**——identifier 进 payload 后 PC 独立插件装载也从"按 path"变为"查扫描列表"。若某些栈未扫描填充列表会回归。验收必须含：agent 全量+webui 绿 **+ PC 真栈一次插件写场景 spot**（EQ 腿或等价 d1 插件域，exit 0）。若实测 PC 栈不保证扫描填充，停下来上交三选一（内核 identifier 解析失败回落 path / 仅壳主体带 identifier 的判定条件 / 保证扫描前置）——**内核 C++ 改动=域外上交裁定**
  4. mac 侧回归（用户转交）：修复合入 main 后 mac 复跑 EQ 旅程腿原命令（journey1_mac EQ 段装载/A-B 走完=本缺陷闭环证据）
- 文件域：`agent/internal/chat/free_state_d1_plan_table.go`（+测试）；预期零端口/零内核改动（如被迫动则按目标 3 上交）
- 验收：①红测试修前红/修后绿；②agent 全量+webui 绿；③PC 真栈插件写 spot exit 0；④mac EQ 腿回归 exit 0（回执带两端 HEAD）
- 停止条件：内核语义需改动才能双端绿 → 域外上交（附红测试证据）
- 领取：
- 回执：
- 验收：
