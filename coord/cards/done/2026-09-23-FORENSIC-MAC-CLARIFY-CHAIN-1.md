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
- 回执：2026-09-24 11:05 (Mac) / 零代码改动（coord-only，无实现 commit）/ 报告=`~/Documents/vit-forensic-clarify1-artifacts/FORENSIC_REPORT.md`（含证据副本 5 件，key 扫描清洁）。两端 HEAD：决策侧发卡=84f3fc1；Mac 领取=84f3fc1（本卡两次 coord push=6234d2e+本次）。
  - **失败链还原（11 秒黑箱=一次 10.3s `message_loop_repair` LLM 调用）**：turn=3 原始输出=形态良好 needs_experiment 提案 JSON（+1.5dB@3kHz 有界实验，conf 0.42）但**缺一个闭括号**（8591 token 异常长推理，绿轮同段 ≤2297）→ 23:52:03.634 parse_failed「计划格式不是有效 JSON」→ 闭包变体 repair 调用（http_ms=10338，prompt 明令禁 clarification）→ **repair 模型违令返回 needs_clarification 问句**「需要先确认哪条是主唱轨…」→ `audio_closure_controller.go:557`（`repairCount>0 && res.NeedsClarification`，随根提交 ee0fac4 2026-09-16 入库）判「model protocol failure」→ EventTaskFailed → 23:52:14.106 turn.failed（error=问句，completed_steps=0）→ driver 见 goal=failed exit 1。
  - **定责：④ 模型输出形态触发（主责，方差 A 原始输出畸形+方差 B repair 违令选 clarify 形态，缺一不失败）+ ⑤ 素材弱协因**（原始输出无退化；mac 夹具无主唱标注给 repair 提供问句动机，PC bx_hybrid V2 7/7 零此形态）。①F2 回归排除（判死策略早 F2 五天入库；失败分类在投递面之前且投递面如实工作；F2 后 PC 同句问句直接路径 park exit 0；bbd0dbf↔HEAD 关键文件零 diff）；②竞态排除（全链确定性序列）；③mac 平台特有排除（代码同源+双端引擎完全一致+「PC 零同形态」存在夹具混淆，无平台差锚点）。
  - **归族**：新族「repair-clarify 协议违约判死」（触发要件=parse_failed+闭包在案+repair 返回 clarify；10 轮 1 例；与 F4 并列，非 F2 呈现面族）。
  - **修复建议单行（另卡）**：repair 解析成功但返回 needs_clarification 且闭包在案时不直接 pause——先追加一次禁-clarify 强化 repair（或剥离该字段按不可恢复降级），仍违令才 557 判死；repair 成功分支补 raw 落诊断。
  - **证据边界**：repair 输出原文未落 durable 面（形态由下游行为反推，置信度高非直证）；R2/R4 repair 原文同边界；PC 引擎一致性由 PC 回函背书。durable 存储未被轮转，全部原件在档。
  - 端测边界声明：本卡=纯只读取证，无运行栈改动，无端测适用面。
- 验收：
