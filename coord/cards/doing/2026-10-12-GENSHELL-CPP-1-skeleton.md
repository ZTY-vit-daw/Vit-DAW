# GENSHELL-CPP-1：生成插件壳体骨架（实施①·壳体）

- 发卡：Mac 决策侧 / 2026-10-12（依据=GEN-PLUGIN-SHELL-DESIGN-1 验收 pass 的实施拆卡规格件五 卡1；PC 调度件排序权移交在案）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 95；**规格全文=[SHELL 设计件五 卡1](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威，本卡为池入口）**
- 归属：Mac 生成线（生成集成面②）；P1 / 2-3 天 / 无依赖（六卡序列之首）
- 模型分级：L2 / GLM 执行会话

## 要点摘录（以规格为准）

/GenShell/ 新目录 CMake 工程——VST3 SDK 直实现壳体（IEditController+IAudioProcessor instrument 静音骨架/T0 消费/T1 五参数/T3 chunk/VSTGUI 编辑器静态六区/T2 信封回环 echo 计数）；产物 GenShell.vst3 部署本机标准 VST3 目录。**卡内第一步=核实 VST3 SDK 获取路径**（tracktion 子模块/PluginProbe native host 来源；Mac worktree 子模块空为工件，取主仓 checkout）+VSTGUI vendoring 方式入卡记录。验收=构建 exit 0+真栈烟测（scan 发现/rack_add_node UI 路径形态/T1 参数断言/SceneMode 读写/chunk 往返/show_editor 不崩）+T2 自环。停止条件=SDK 两端均不可复用/VSTGUI 不可用→降级裁定上交。端测边界：编辑器视觉面零断言。

- 领取：2026-10-13 09:48（Mac 本地）/ origin/main 1ec6ccfa / owner=GLM-5.3 Mac 执行会话 / 分支 port/genshell-cpp-1 / worktree /Users/timozty/Documents/Vit-DAW-genshell1（coord=Vit-DAW-genshell1-coord）/ 领取提交=见本提交（领取随卡面同批）
- 回执：（commit / run ID / SDK 获取路径记录 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
