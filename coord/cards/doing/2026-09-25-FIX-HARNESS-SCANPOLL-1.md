# FIX-HARNESS-SCANPOLL-1：harness scanAvailablePluginRows 轮询落点修复——250ms 全量应答跑在内核 JUCE message thread 饿死出壳扫描收集

- 优先级 / 预估 / 依赖：P2（旅程/烟测驱动面性能缺陷，非产品面）/ 0.3 天 / PORT-PCBATCH-MAC-LEGS-1 腿 3 发现（取证=600s/1800s 双看门狗实证+3s 静默轮询对照，工件 leg3_certauth/）
- 模型分级：L2 / GLM-5.3（agent Go harness 层）
- **执行侧路由建议（2026-09-25 Mac 决策会话）：PC 侧领取**——与 mac 正在执行的 FIX-KERNEL-HYGIENE-BUNDLE-1（内核 C++）零文件域交集/零栈冲突，构成双端并行分工；mac 侧合入后补本卡目标 3 的 journey 扫描段实证腿
- **背景（取证在案）**：harness `scanAvailablePluginRows` 默认 250ms 轮询 `plugin_list_available`，跑在**内核 JUCE message thread** 语境（ZmqGateway 逐条全量应答+落日志 857KB/次）——出壳探测的结果收集被饿死，主壳 600s/1800s 双看门狗触发；静默 3s 轮询同内核 5 分钟 719 全量对照实证。驱动侧旋钮已落地（journey mac 脚本 `scan_poll_interval_ms=2000`，main=ebd0428，断言零变化）——**harness 侧默认值与轮询落点（应在轮询侧做节流/增量化/移出热路径）未修**。
- 目标：
  1. 定位 harness.go 轮询实现，修复方案二选一（执行侧按代码面最小改动定+回执说明）：默认轮询间隔提到合理值（≥2s，与驱动旋钮一致）/ 或应答增量化+日志降频（避免 857KB×4/s 级冲刷）
  2. 红测试：修前红=轮询间隔/应答量断言（形态按实现定）；修后绿
  3. 回归：agent 全量+webui 绿；mac journey 一轮（扫描 warm-up 不再看门狗超时，scan_poll_interval_ms 旋钮可回默认验证）
- 文件域：`agent/internal/harness/`（轮询点）+测试；不改内核/协议
- 验收：①红绿 ②全量绿 ③mac journey 扫描段实证（旋钮默认值下完成）④回执两端 HEAD
- 停止条件：修复需协议/内核配合（增量化需内核面改动）→ 上交拆卡
- 领取：2026-09-26 PC 执行侧（L2/GLM-5.3）/ origin/main=7563b235（领取时 HEAD=当前 main 顶）/ 分支 port/fix-harness-scanpoll；领取时工作树遗留：M CURRENT-STATE.md、M VitApp/Workspace/default_project.xml、coord/runs/ 多卡未跟踪工件——均他卡/运行时域，本卡零接触；本卡新增 diff 将限于 agent/internal/harness/
- 回执：
- 验收：
