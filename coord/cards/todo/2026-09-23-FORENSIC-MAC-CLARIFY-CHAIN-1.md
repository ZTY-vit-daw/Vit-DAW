# FORENSIC-MAC-CLARIFY-CHAIN-1：⑤ R3 clarify 链失败新形态取证（LLM 成功后 turn failed）

- 优先级 / 预估 / 依赖：P1 / 0.3 天 / mac 跟随回执 2026-09-23 裁定请求 1（10 轮 1 例，未到两败线但落在 F2 修复触碰面邻域——**修复回归优先排除**）
- 模型分级：L1 / GLM-5.3（只读取证，工件与 durable 存储已冻结）
- **执行侧（mac 侧卡）**：素材在本机 `~/Documents/vit-mac-followup-artifacts/retest_20260922/` + agent durable 对话存储；回执写回 D:\Vit_DAW coord（本卡移 doing 时记两端 HEAD）
- **背景（mac 回执 §四.1）**：run `vit_product_path_mac_20260922-235030` vocal 轮 turn=3——LLM 调用 **42.2s err=false 成功返回**（23:52:03），11 秒后 `agent_loop_chat status=failed stop=failed completed_steps=0`（total 52.6s）；turn.failed 详情引用的恰是模型澄清问句「需要先确认哪条是主唱轨」。排除：LLM 超时环境项（调用成功）、settle 超时（即时终止于 failed）。失败点=LLM 返回后的 turn 处理/clarify-park 路径——**FIX-F2-SURFACE-REPLY 触碰面邻域**（goalHasLiveContinuationOwner / clarify-park 终态事件发射 / pending payload 回填）。
- 目标（只读，定责不开刀）：
  1. durable 存储还原 turn=3 全链：模型返回原文（是否含合规 clarify 决策/问句）→ 调度链处理点 → failed 的 error 字段原文与堆栈/阶段标记
  2. 对照 PC 侧 F2 修复路径（continuation_scheduler.go 谓词 / goalrunner_chat.go 投递 / turn_trajectory_events.go）判定：修复引入的回归？既有竞态被 F2 改动后可见？mac 平台特有（路径/并发时序）？模型输出形态触发？
  3. PC 侧 10+ 真栈轮零同形态（F2 卡⑤轮+FOLD 轮+GATE 轮）——若 mac 平台特有需给出平台差锚点
  4. 产出：定责结论+证据指针；若=基础设施缺陷→附最小复现与修复建议单行（修复另卡）；若=模型方差/既有形态→登记归族
- 约束：零代码改动（如需探针日志，只读现有工件优先）；durable 存储只读；LLM key 零入工件
- 验收：①失败链时间线（LLM 返回→failed 之间每步证据）；②定责四选一结论+锚点；③修复建议或归族登记
- 停止条件：durable 存储已被轮转覆盖 → 如实记录证据边界上交
- 领取：
- 回执：
- 验收：
