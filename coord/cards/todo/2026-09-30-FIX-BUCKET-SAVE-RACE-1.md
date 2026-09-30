# FIX-BUCKET-SAVE-RACE-1：webui 会话桶 save/restore 时序缺口——仅存本地桶的末轮消息刷新后结构性丢失

- 池序 8；来源=OPT-OBSERVE-OUTPUT-1 上交①（裁定=[rulings/2026-09-30-OPT-OBSERVE-OUTPUT-1-pass.md](../../rulings/2026-09-30-OPT-OBSERVE-OUTPUT-1-pass.md)）
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无；webui 域（与 OPT/VITNOTE-IMPL-5 同域串行）
- 模型分级：L1 / flash 可接
- 已核实事实（执行侧取证，决策侧已稳定化证据工件）：
  - 现象：reload 时 scope 物化拍的 initial **同步 setMessages（图消息）触发 save 效应覆写本地消息桶**，而 restore 效果因 scopedConversationRef 同 commit 内晚置、最早须等下一轮询拍（~8s）才首跑——两者时序颠倒。
  - 后果：**仅存本地桶的驱动消息（未及服务端 checkpoint 的末轮）刷新后结构性丢失**；真水合/服务端图路径不受影响。
  - 证据：coord/runs/OPT-OBSERVE-OUTPUT-1/evidence/（mini_repro.mjs——无图消息时 restore 正常 ~10s 合并回屏，反证时序归因；run-190233/190740 dom-observe-reload-samples.json 两轮采样）。
- 目标：
  1. 修复时序：restore 在同 scope 首次 save 生效前完成，或 save 对"restore 未完成的桶"跳过/合并而非整桶覆写（实现面领取后实锚 App.tsx 水合/save 效应段，选破坏最小方案并申报）。
  2. 回归用例：复刻 mini_repro 场景——有图消息+仅存本地桶的末轮驱动消息→刷新后该消息仍在（单测或组件测层面）。
  3. E2E 加组（可选，若单测已覆盖充分则回执申报理由）：reload 保消息断言。
- 文件域：agent/webui/src/（App.tsx 水合/save 效应段+涉及测试）；不动 agent Go 侧。
- 验收标准：npm run test 全绿含新回归用例+build 过+（若加组）E2E-WEBUI-1 exit 0。
- 停止条件：取证发现既有桶覆写时序另有功能依赖（修复会破坏其他路径）→ 实证上交定方案。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 修复方案一句话 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
