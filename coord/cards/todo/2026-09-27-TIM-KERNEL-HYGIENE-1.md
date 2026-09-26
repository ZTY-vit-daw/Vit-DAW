# TIM-KERNEL-HYGIENE-1：内核卫生小卡——DC offset 断言源+Hygiene Report 回传（GAPS Item 2+5 合卡）

- 优先级 / 预估 / 依赖：P1 / ~1 天 / TIM-KERNEL-GAPS-1 报告 §2 Item 2/Item 5（锚点与工作量依据）；CTES 断言器 v1 已合入（消费方在位）
- 模型分级：L2 / GLM 亲自或强督导（VitApp 域）
- **执行侧（mac 会话，夜池；ctest 双配置门）**
- 目标（两件合卡，均 VitApp 域）：
  1. **Item 5：PluginListHygiene Report 回传**——内核启动清理摘要（"PluginListHygiene: removed ..."行，日志面）结构化为 get_project_state 的卫生块（removed 计数+最近清理时间戳；零清理=显式 0）；agent 侧 TIM 断言器消费（新增 AS-HYGIENE 检查或并入 signal_hygiene 域——执行侧按现有断言器结构最小接入，三态语义保持）。
  2. **Item 2：DC offset（带符号均值）**——L3AcousticAnalyzer 增 DC offset 计算（帧级带符号均值，逐帧进 feature snapshot 透传键表）；agent 侧 AS-SIG 增 P3 断言（|dc_offset| 阈值 warn，阈值常量透出，缺键 NE）。
  3. **红绿**：内核 ctest（新特征字段+卫生块断言，双配置）+agent 红绿（P3 断言+卫生消费）；既有 7 项 ctest+84 包零回退。
  4. 回执：红绿退出码+diff stat+HEAD。
- 约束：零音频线程触碰（分析路径是离线 L3 池）；阈值常量透出可比性键（K5/K7 范式）；文件域 VitApp/Source+Tests+agent/internal/tim。
- 验收：双配置 ctest+agent 全量 0 FAIL+红绿在案
- 停止条件：GAPS 报告锚点与现实不符 → 取证上交
