# FIX-D1-PLUGIDENT-1：D1 插件写路径 normalized-batch 计划 args 补发 plugin_identifier（WaveShell 成员可达）

- 优先级 / 预估 / 依赖：P1 / 0.3 天 / PORT-WL-EQ-1 回执上交 (b)（mac EQ 装载受阻定位于此；用户 2026-09-23 呈报）；**跨族缺口非 EQ 专属**；**执行侧=mac（用户裁定 2026-09-23"让他开始修"）**——本卡为共享仓 Go 代码改动，mac 直接在仓内分支执行；目标 3 的 PC 真栈 spot **改为合入后 PC 侧补验**（mac 无法运行 PC 栈；风险已降级为预期绿，补验由决策侧在合入验收时安排）
- 模型分级：L2 / GLM-5.3（端口/内核解析语义边界，红测试先行）
- **背景（回执取证+决策侧代码复核在案；2026-09-23 用户质询后二次修正）**：白名单绑定路径的计划 args 只发 `plugin_path`+`plugin_name` 漏发 `plugin_identifier`，而 binding.Plugin 里有该字段（admission 层六族都在用，free_state_d1_plan_table.go:102-408）。**PC 侧事实修正：PC 开发环节一直能控制 Waves 效果器**——老路径全部带 identifier 纪律（C2 动态装载 `c2_dynamic_plugin_load.go:213` path+name+identifier 三件套齐发、plugin grabber、PCA 认证链；PC 语义索引 1982 条 VST3 标识含 Q10 Stereo/Mono、Sibilance、Smack Attack 等 WaveShell 成员稳定 CID，C2-PCR 24 主体认证含 Waves 成员）；内核 stable-CID 成员装载正是为此在 PC 上开发（waves-vst3-cid-fix 2026-07-27，两端 pin 均含）。**本缺陷=最新路径（自由态 D1 白名单绑定，八月 v5 时代）丢掉了老路径已有的 identifier 纪律**：D1 白名单 PC 七族碰巧全独立插件（path 即唯一）故 PC 侧不可见；mac 六族全 Waves 壳内成员，path 指向壳内数百成员，内核对无 identifier 的壳路径 fail-closed（正确防御）→ mac EQ 装载/A-B 未走完。两处同病：`d1StaticEQActionArgs`（free_state_d1_plan_table.go:573 args map）与共享 `pluginParamWriteArgs`（:676 args map，compression/de_esser/transient/limiter/gate/multiband 共用）。端口 `staticeq_vsp.go:83-84` identifier 存在即透传，零改动。已有实例复写路径（:627 plugin_id 直达）不受影响——缺口只在全新 instantiate。
- 目标：
  1. `d1PluginParamWhitelistBinding` 增 `PluginIdentifier` 字段并自 `binding.Plugin.PluginIdentifier` 填充；两处 args map 各补一行 `"plugin_identifier": …`
  2. 红测试：绑定路径计划 args 含白名单 plugin_identifier（修前红/修后绿，两个 builder 各一枚）；stub 路径（无绑定）行为不变回归
  3. **PC 回归腿（必做，预期绿——风险经二次核实已降级）**：staticeq_vsp.go 顶部注释明示内核对非空 identifier 独占解析 known-plugin list、不回落 path；**但 PC 老路径（C2/PCA）长期发送 identifier 且 24 主体认证含 Waves 成员全绿**——独占解析语义在 PC 生产栈已被例行行使，本修复=D1 新路径对齐既有可用语义而非开新语义。验收仍含：agent 全量+webui 绿 + PC 真栈一次插件写场景 spot（exit 0）作确认；若意外红（某栈未扫描填充列表）停下上交三选一（内核解析失败回落 path / 仅壳主体带 identifier 判定条件 / 保证扫描前置）——**内核 C++ 改动=域外上交裁定**
  4. mac 侧回归（用户转交）：修复合入 main 后 mac 复跑 EQ 旅程腿原命令（journey1_mac EQ 段装载/A-B 走完=本缺陷闭环证据）
- 文件域：`agent/internal/chat/free_state_d1_plan_table.go`（+测试）；预期零端口/零内核改动（如被迫动则按目标 3 上交）
- 验收：①红测试修前红/修后绿；②agent 全量+webui 绿；③PC 真栈插件写 spot exit 0；④mac EQ 腿回归 exit 0（回执带两端 HEAD）
- 停止条件：内核语义需改动才能双端绿 → 域外上交（附红测试证据）
- 领取：2026-09-23 / origin/main=5c16ef67f24724db609c0309484377b37d8ba898（零入站提交，与发卡基线一致；工作树仅 VitApp/Workspace 两运行时残留文件，非本卡 diff）/ 分支 port/fix-d1-plugident
- 回执：
- 验收：
