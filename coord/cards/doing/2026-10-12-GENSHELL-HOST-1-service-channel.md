# GENSHELL-HOST-1：宿主侧服务+通道（实施②）

- 发卡：Mac 决策侧 / 2026-10-12（依据=件五 卡2）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 96；**规格全文=[件五 卡2](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P1 / 1-1.5 天 / **GENSHELL-CPP-1 合入后（✓ 2026-10-16 pass 合入 7745719a——依赖满足，可领；锚点消费卡 1 回执"关键实现事实"五条）**
- 模型分级：L2 / GLM 执行会话

## 要点摘录（以规格为准）

内核 GenerationShellService（壳实例注册表+IConnectionPoint host 端+提交队列+新读命令 gen_shell.pending+五类下行推送）。文件域=GenerationShellService.{h,cpp} 新文件+CommandDispatcher.cpp 仅注册区 2784-2803 邻域（**不触披露块 3291-3448——GEN-CTX 域避让**）+CMakeLists 登记+冒测脚本。验收=真栈烟测（伪提交/pending 应答/JobState 镜像/cancel/ghost 断言）+主选路径核实结论+agent 零改动零 diff。停止条件=裸 VST3 接口无路径（主选+降级 A 均不通）→上交降级 B 决策点。

- 领取：2026-10-16 21:34（Mac 本地）/ origin/main ba820dcf / owner=GLM-5.3 Mac 执行会话 / 分支 port/genshell-host-1 / worktree /Users/timozty/Documents/Vit-DAW-genshell2（coord=Vit-DAW-genshell2-coord）/ 领取提交=（coord-only 直推 main 后回填）
- 回执：（commit / run ID / 主选路径核实结论 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
