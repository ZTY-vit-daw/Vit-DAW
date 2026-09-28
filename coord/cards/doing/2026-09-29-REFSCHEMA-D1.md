# REFSCHEMA-D1：内核 C++ 五处 D 类生成点 ref 迁移（G1 ruling 排程钩子兑现）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / G1 ruling #6：D 类内核 5 处迁移独立卡，排在 refschema-L0-1 落地后（已落地）；L1-1 报告 §2 D 类清单（内核 C++ 侧 evidence_ref 同名字段独立生成格式族，5 处）
- 模型分级：L2 / GLM 亲自（VitApp 域，ctest 双配置门）
- **执行侧（mac 夜池；与 L1-3-IMPL-A 文件域零交叉可并行）**
- 目标：
  1. **五处清单核对**：L1-1 报告 D 类 5 处生成点逐个锚点核对现状（报告基线后内核有多轮改动——行号漂移先实取）。
  2. **迁移实现**：内核侧 ref 生成统一走 L0 文法（`vit://` 格式的 C++ 序列化——与 agentprotocol refschema 的文法镜像；前缀注册表常量在内核侧的镜像定义——编译期常量+注释指向 agent 权威源）。
  3. **红绿**：ctest 断言（新格式 ref 生成+旧消费方兼容性——内核发出的 ref 被 agent 侧消费的接口测试若在 agent 侧则双端）。
  4. **门**：ctest 双配置全绿+agent 全量 0 FAIL（消费面零回退）。
  5. 回执：五处迁移前后样例对照（旧格式→新格式各一行）+红绿+diff stat。
- 约束：只迁格式不改语义（ref 承载的信息不变）；§11 旧 agent 兼容（旧格式解析路径保留宽限期——agent 侧 opaque 宽容解析已有）；提交显式列文件+推分支+切回 main。
- 验收：五处迁移样例对照+双端测试绿
- 停止条件：某处生成点的消费方硬编码旧格式（改内核会断消费）→ 该处挂起+消费方清单上交
- 领取：2026-09-29 / origin/main `1f4bd9067a7334983534c1703ec30468bb25ce56`（worktree 基 `9566c07c`＝origin/main+SETTLE-CHAIN-1 领取提交，coord-only）/ 分支 `port/refschema-d1` / 独立 worktree `D:/Vit_DAW_wt_refschemad1`（与 SETTLE-CHAIN-1 并行）
- 实勘备注（领取时）：行号已漂移——D3 :465→:548、D4 :573→:658、D5 报告 4 处→现 7 处（:672/:695/:719/:745/:858/:901/:925，segmentation_primitives 三处为报告后新增）；D2 另有报告漏列第三处生成点 `CompressorDualTapEvidence.cpp:238`（result.evidenceRef→:339 写入 artifact JSON）
- 回执：（待回填）
