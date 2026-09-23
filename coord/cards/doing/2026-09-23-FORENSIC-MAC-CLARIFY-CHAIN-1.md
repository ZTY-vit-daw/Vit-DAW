# FORENSIC-MAC-CLARIFY-CHAIN-1：⑤ R3 clarify 链失败新形态取证（LLM 成功后 turn failed）

- 优先级 / 预估 / 依赖：P1 / 0.3 天 / mac 跟随回执 2026-09-23 裁定请求 1（10 轮 1 例，未到两败线但落在 F2 修复触碰面邻域——**修复回归优先排除**）
- 模型分级：L1 / GLM-5.3（只读取证，工件与 durable 存储已冻结）
- **执行侧（mac 侧卡）**：素材在本机 `~/Documents/vit-mac-followup-artifacts/retest_20260922/` + agent durable 对话存储；回执写回 D:\Vit_DAW coord（本卡移 doing 时记两端 HEAD）
- **背景（mac 回执 §四.1）**：run `vit_product_path_mac_20260922-235030` vocal 轮 turn=3——LLM 调用 **42.2s err=false 成功返回**（23:52:03），11 秒后 `agent_loop_chat status=failed stop=failed completed_steps=0`（total 52.6s）；turn.failed 详情引用的恰是模型澄清问句「需要先确认哪条是主唱轨」。排除：LLM 超时环境项（调用成功）、settle 超时（即时终止于 failed）。失败点=LLM 返回后的 turn 处理/clarify-park 路径——**FIX-F2-SURFACE-REPLY 触碰面邻域**（goalHasLiveContinuationOwner / clarify-park 终态事件发射 / pending payload 回填）。
- **假设空间（五选一，⑤为用户 2026-09-23 增补）**：①F2 修复回归 ②既有竞态可见化 ③mac 平台特有 ④模型输出形态触发 ⑤**素材退化形态**——④⑤ 夹具=仓内合成音（100Hz 正弦+3s 目标音，双端同源，不具真实音频工程条件），模型在此类素材上的输出形态分布可能漂移（叙事化/边界措辞变化），可能触发处理路径边缘。注意：⑤成立也不免除①-④的核查——**形态良好的 clarify 输出把回合走到 failed 始终是处理路径健壮性问题**，素材归因与代码缺陷可以并存。
- 目标（只读，定责不开刀）：
  1. durable 存储还原 turn=3 全链：模型返回原文（是否含合规 clarify 决策/问句）→ 调度链处理点 → failed 的 error 字段原文与堆栈/阶段标记
  2. 对照 PC 侧 F2 修复路径（continuation_scheduler.go 谓词 / goalrunner_chat.go 投递 / turn_trajectory_events.go）判定：修复引入的回归？既有竞态被 F2 改动后可见？mac 平台特有（路径/并发时序）？模型输出形态触发？
  3. PC 侧 10+ 真栈轮零同形态（F2 卡⑤轮+FOLD 轮+GATE 轮）——若 mac 平台特有需给出平台差锚点
  4. **素材形态对照（假设⑤取证）**：失败轮与绿轮的项目素材状态/模型输出原文形态逐字对照（叙事比例、边界措辞、决策形态），判素材退化是否为触发条件或协因
  5. 产出：定责结论+证据指针；若=基础设施缺陷→附最小复现与修复建议单行（修复另卡）；若=模型方差/素材形态触发→登记归族并注明素材层处置（真实素材见 PORT-REALSTEMS-MAC-1）

- 约束：零代码改动（如需探针日志，只读现有工件优先）；durable 存储只读；LLM key 零入工件
- 验收：①失败链时间线（LLM 返回→failed 之间每步证据）；②定责四选一结论+锚点；③修复建议或归族登记
- 停止条件：durable 存储已被轮转覆盖 → 如实记录证据边界上交
- 领取：2026-09-24 10:15 (Mac) / origin/main=84f3fc1（与决策侧发卡基线一致）/ 无 port 分支（零代码改动，coord-only 推 main）；Mac 侧 HEAD=84f3fc1，领取前工作树残留=VitApp/Workspace 两文件+Artifacts/+.zcodeignore（不碰）
- 回执：
- 验收：
