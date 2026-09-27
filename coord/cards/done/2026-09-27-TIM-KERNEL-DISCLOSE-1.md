# TIM-KERNEL-DISCLOSE-1：project state 披露扩展——块长一致性+per-instance 装载态（GAPS Item 3+4 合卡）

- 优先级 / 预估 / 依赖：P1 / ~1 天 / TIM-KERNEL-GAPS-1 报告 §2 Item 3/Item 4；AS-SR/AS-PLUGIN 断言器已合入（消费方在位）
- 模型分级：L2 / GLM 亲自或强督导（VitApp 域+agent 消费）
- **执行侧（mac 会话，夜池；与 TIM-KERNEL-HYGIENE-1 文件域有 CommandDispatcher 交集——若同夜执行按顺序串行，后卡 rebase 前卡）**
- 目标：
  1. **Item 3：块长一致性入 project state**——audio_settings 块增 block_size（当前内核实际使用值）；agent 侧 AS-SR 扩 P2（轨级块长缺=NE，工程级缺=断言器 NE 保持现状不降级）。
  2. **Item 4：per-instance 插件装载态出内核**——get_project_state 的 rack nodes 增 per-instance 装载态字段（plugin_instance_ready/plugin_load_state——现仅 rack_add_node 回执，GAPS 锚点 CommandDispatcher.cpp:3155-3172；只读快照查询，零装载动作）；agent 侧 AS-PLUGIN 扩 P2（装载态=failed→fail 码，缺字段 NE）。
  3. **红绿**：内核 ctest（两披露字段断言，双配置）+agent 红绿（两断言扩展）；零回退门同上。
  4. 回执：红绿+diff stat+HEAD。
- 约束：纯只读披露扩展（get_project_state 不触发任何装载/分析动作）；§11（新增字段旧 agent 忽略=零破坏）。
- 验收：双配置 ctest+agent 全量 0 FAIL+红绿在案
- 停止条件：per-instance 装载态在内核侧无现成可读源（需新状态存储）→ 停止上交（存储设计=决策侧裁定，不擅自建）
- 领取：2026-09-27 04:27 UTC / a9e47e0c604323cb3b5930685cef329f8ccf6310（origin/main）/ port/tim-kernel-disclose-1（分支基点=②分支尖 08b8986，串行 rebase 约束）
- 回执（2026-09-27 mac 夜池）：
  - 红绿在案：RED `12b8322`（内核 VitProjectStateDisclosureTests 新 ctest 目标对桩 exit 134+agent 4 测试 exit 1）→ GREEN `799c31e`（内核 exit 0 全过+agent 4/4+tim 包过）。分支 port/tim-kernel-disclose-1 已推 origin，**分支基点=②分支尖 08b8986（串行约束：决策侧合入须先 ② 后 ③，否则 CommandDispatcher.cpp 冲突）**。
  - 门：**ctest 双配置 Debug 9/9 + Release 9/9（各 exit 0，既有 8 项零回退+新增 VitProjectStateDisclosureTests）**；主工程 VitApp Release 链接 exit 0——该门抓到两处真错（tracktion DeviceManager 须走 `.deviceManager` 内层 juce 访问；externalPluginLoadStateFields 误入匿名命名空间致链接失败）均已修复；agent `go test ./... -count=1` **84 包 ok exit 0**+vet/gofmt 干净（新文件）。
  - diff stat：内核 5 文件（PluginLoadState.h 新增纯分类助手 std-only/CommandDispatcher.cpp 两处接线/PluginRackControlService.cpp+.h 结构化装载态+describe 重构日志格式保真）+测试 2 文件（ProjectStateDisclosureTests.cpp 新+CMakelists）+agent 5 文件（asserter.go/projection.go/types.go/新 asserter_disclose_test.go+**mixboard 1 行**）。
  - **越域 1 行（显著标记，决策侧裁定追认）**：`agent/internal/mixboard/tim_asserter_legs_test.go:89` 断言过宽（plugin_legality 任意 check 非 pass 即 Fatal）撞上新 load_state 检查对无披露字段的诚实 NE——收窄为 `Check=="known_path"`（保留原意图"已知路径应 pass"）。备选方案（无披露时静默不发行）违反三态原则（缺证据必须显式 NE），不取。卡面文件域为 agent/internal/tim——此 1 行越域已在此声明。
  - 设计点（供决策核）：①轨级块长现无任何内核数据源——TrackFact.BlockSize 读取器已接（acoustic/evidence 透传键 block_size），未披露前恒 NE（卡面"轨级缺=NE"字面实现，升级路径数据驱动）；②装载态聚合语义=failed 支配（真失败证据优先），pending/未披露→NE（async 瞬态窗，GAPS Item4 风险对策）；③plugin_load_error 字段为卡面两字段外加项（失败时透传原始错误串，§11 安全）。
  - 端测覆盖边界：组件级门已过（ctest 双配置+app 编译+agent 单测）；get_project_state 两披露字段的真栈端到端未跑（夜间无真栈），留决策侧合入后安排。
  - HEAD：`799c31e`（port/tim-kernel-disclose-1 已推 origin）。
