# ARRLOOP-AUTHBRIDGE-1：agent-loop 能力轮装载授权桥（277 治理缝收口）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[277 段 2 取证](../runs/ARR-SRCLOAD-1/smokerun1/evidence.md)+[裁定 D 先行+B 治本]——能力轮披露面完整但无授权桥：不可伪造选择授权只在装载工作流确认期[plugin_grabber_workflow.go:1044-1065]/UI 浏览器选中/full-access promoted 三路径产生；CAP-BOUNDARY-1 钉过同型拦截为正确行为，本卡补的是编曲常规路径的授权产生面而非绕门）
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 273；目标仓库=主仓 agent（harness/workflow 授权面）；P1 / 0.5-1 天 / 依赖=277 段 2 工件在库；L1-L2 / PC；验证腿需真栈一次（可复用 arr_srcload_smoke 驱动器）

## 目标

1. **授权桥实现（方案 B）**：agent-loop 能力轮的装载类动作（rack_add_node 等 PCA 门面）在**确认期**产生不可伪造选择授权——与装载工作流确认期同款治理（Authorize* 盖章语义等价），不新增第三套授权机制；用户在环保持（确认卡→用户确认→盖章）；
2. 治理红线：PCA 门本体零改动（enforceAgentProcessorLoadGate 不动——补的是授权产生面）；零裸写；确认链语义与既有装载工作流一致；
3. 配对单测（能力轮装载确认→盖章→过门；未确认→仍拦）+277 段 2 同型 run 真栈验证（编曲话术常规路径走通：search→rAdd 确认→过门→executed）；
4. §8 跑前成文（≈¥1-3 单 run）。

## 约束

文件域=harness 装载确认面+授权盖章接线+测试+（如需）烟测加参；禁动 PCA 门判据本体；禁动判断权归属域（纯执行层授权产生）；261 的 G5 授权面先例（ensure_full_project_access 服务端路径）可参但不混用（那是 authority 切换非选择授权）。

## 停止条件

实现发现能力轮确认期与装载工作流确认期语义冲突（在案裁定）→取证上交转裁定；验证腿断言失败取证不重跑。

## 领取/回执/验收

## 领取

- 2026-11-07 / 段 1 领取（本轮零栈先行：实现+配对单测；真栈验证腿候栈——栈队列第 3 位：281 烟测→277 段 3a→273 验证）
- origin/main hash：`24a8247b3bb632d2dc1c67d8b5cc6928a61a3d37`（领取时 fetch 重读；任务书基准 09f284d8 已被 281 占栈登记笔 [纯 coord/ 变更] 推进，无代码面差异）
- owner：GLM 主会话 L2-worker 子代理（GLM-5.3 / ZCode / PC）
- 分支：`port/arrloop-authbridge-1`（自 origin/main 新建）；worktree：`D:/Vit_DAW_wt_273impl`
- 领取基线：HEAD=`24a8247b3bb632d2dc1c67d8b5cc6928a61a3d37`；worktree `git status --short`=空；`git diff --stat`=空（新 worktree 自 origin/main 检出全净，无领取前已有 diff）
- 前置核对：卡仍在 todo/ ✓ 派发已确认 ✓；doing 四卡（ARR-SRCLOAD-1[impl 已完在 port/arr-srcload-1 分支]、EDU-LEVEL-RUNTIME-B、CLOSURE-ROUTE-ADAPT-1、D1D3-DIGEST-WIRING-1）文件域与本卡不相交 ✓（goalrunner_chat.go 与 277 触发词面同文件不同段，277 段 1 已验收待合入，合入归主管）

## 回执

（段 1 执行侧回填）

## 验收

- （主管回填）
