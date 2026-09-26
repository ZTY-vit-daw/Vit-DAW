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
