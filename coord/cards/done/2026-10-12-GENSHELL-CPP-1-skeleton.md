# GENSHELL-CPP-1：生成插件壳体骨架（实施①·壳体）

- 发卡：Mac 决策侧 / 2026-10-12（依据=GEN-PLUGIN-SHELL-DESIGN-1 验收 pass 的实施拆卡规格件五 卡1；PC 调度件排序权移交在案）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 95；**规格全文=[SHELL 设计件五 卡1](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威，本卡为池入口）**
- 归属：Mac 生成线（生成集成面②）；P1 / 2-3 天 / 无依赖（六卡序列之首）
- 模型分级：L2 / GLM 执行会话

## 要点摘录（以规格为准）

/GenShell/ 新目录 CMake 工程——VST3 SDK 直实现壳体（IEditController+IAudioProcessor instrument 静音骨架/T0 消费/T1 五参数/T3 chunk/VSTGUI 编辑器静态六区/T2 信封回环 echo 计数）；产物 GenShell.vst3 部署本机标准 VST3 目录。**卡内第一步=核实 VST3 SDK 获取路径**（tracktion 子模块/PluginProbe native host 来源；Mac worktree 子模块空为工件，取主仓 checkout）+VSTGUI vendoring 方式入卡记录。验收=构建 exit 0+真栈烟测（scan 发现/rack_add_node UI 路径形态/T1 参数断言/SceneMode 读写/chunk 往返/show_editor 不崩）+T2 自环。停止条件=SDK 两端均不可复用/VSTGUI 不可用→降级裁定上交。端测边界：编辑器视觉面零断言。

- 领取：2026-10-13 09:48（Mac 本地）/ origin/main 1ec6ccfa / owner=GLM-5.3 Mac 执行会话 / 分支 port/genshell-cpp-1 / worktree /Users/timozty/Documents/Vit-DAW-genshell1（coord=Vit-DAW-genshell1-coord）/ 领取提交=eece953d（coord-only 直推 main）
- 回执（2026-10-13 收工）：
  - **实现 commit**：`3337bd7e` @ `port/genshell-cpp-1`（已推 origin；1187 文件含 vendoring，/GenShell/** 全新+scripts/gen_shell_skeleton_smoke_mac.sh；领取基线 1ec6ccfa、领取时工作树零 diff）
  - **构建 exit 0**：`cmake -S GenShell -B GenShell/build -DGENSHELL_VST3_SDK_ROOT=<主仓>/tracktion_engine/.../VST3_SDK` + `cmake --build` → `GenShell.vst3`（arm64，1.7MB）+ 壳单测进程 `genshell_selftest`；部署 `~/Library/Audio/Plug-Ins/VST3/GenShell.vst3`（cmake `deploy_vst3` 目标）
  - **SDK 获取路径记录（卡内第一步核实结论）**：VST3 SDK=仓库 tracktion_engine 子模块内 JUCE 内嵌 VST3_SDK（`tracktion_engine/modules/juce/modules/juce_audio_processors_headless/format_types/VST3_SDK`，PluginProbe native-host 同源，两端可复用）；该 SDK 为 JUCE 裁剪版（无 public.sdk/source/main、无 vstgui4、无 vstgui editor 基类/AudioEffect 便捷基类）→ 工厂手写 IPluginFactory3+bundleEntry/Exit、processor 直承 Component、编辑器自承 CPluginView。**VSTGUI vendoring=浅克隆 steinbergmedia/vstgui master d646b60f（Apache-2.0）普通源码入库 thirdparty/vstgui4（非 submodule——文件域限 /GenShell/**）；JSON=vendoring nlohmann/json v3.11.3 单头（MIT）thirdparty/json**；来源记录见各自 VENDORING.md
  - **真栈烟测**：`scripts/gen_shell_skeleton_smoke_mac.sh --kernel-bin <主仓>/VitApp/build/VitApp_artefacts/Debug/VitApp` **两次 all_green exit 0**（同代码版本重复成功）——run ID `genshell_skel_mac_20261013-185035`（首次全绿）与 `genshell_skel_mac_20261013-185107`（复跑），工件 `~/Documents/vit-genshell-artifacts/skeleton/<run>/`（report/prereq/replies/logs）。内核二进制=主仓 10-02 构建 sha256 8624fe9e…（本卡零内核改动，§9 记录）；工程走 VIT_PROJECT_XML 隔离副本，worktree 无残留 diff。leg 明细：S 自测 46 PASS（T2 自环 echo 计数 debug 参数+未知 schema 负探针+chunk 全字段保真往返含 form_draft/pending_requests 对象数组/job_history/audition_ref）/K1 scan→plugin_list_available 发现 identifier `VST3-GenShell-15b5622d-b818da1c`/K2 add_track+rack_add_node（UI 路径形态直发内核命令）+get_plugin_parameters 五参数在场/K3 set_plugin_param SceneMode 0.5 读回一致（value_text=free_creation）/K4 save_project(.vit)→open_project 后 SceneMode 保真（EditItemID 跨存取稳定）/K5 show_plugin_editor ok+3s 进程存活。调试期 RED run 两次（180701：K4 chunk 丢值+K5 编辑器段错误；184735：K2 断言口径误含宿主不可见 debug 参数——非缺陷）均已定位处理，crash 报告留档 ~/Library/Logs/DiagnosticReports/VitApp-2026-10-13-180730.ips
  - **关键实现事实（供卡 2/4 锚点）**：① VSTGUI 4.10+ 必须在 bundleEntry 调 `VSTGUI::init(bundleRef)` 否则 CTextLabel 构造期字体全局空指针段错误（真栈实证）；② JUCE 宿主直连 processor↔controller 两个 IConnectionPoint（juce_VST3PluginFormatImpl.h:3260-3261）且 ComponentBase::connect 单 peer——**卡 2 宿主服务接 T2 时需核实该占用冲突（主选路径的既有占用方）**；③ JUCE 恢复顺序=component setState→controller setComponentState→controller setState，controller setState 必须回写参数（本卡 scene_mode 往返依赖此）；④ 壳内部 processor←controller 同步消息 messageID=`genshell_state_sync`（非 T2 契约信封）——卡 2 宿主端须按未知 messageID 丢弃；⑤ debug 计数参数（kIsReadOnly 非 automatable）不进 JUCE automatable 面（宿主行为）——T2 计数读数走单测进程/未来宿主服务直读
  - **端测边界声明**：编辑器视觉面零断言（六区=结构构建+进程存活，K5）；GUI 无业务逻辑（卡 4 接线）；form_draft 等无宿主写入口字段的 chunk 保真由壳单测进程覆盖（真栈面以 SceneMode 为代表字段）；手测入口按 AGENTS §5=用户从 Godot 前端拉起栈后 show_plugin_editor 目检（未执行，[等待用户] 非本卡验收门）；Windows 端构建未做（规格允许"执行端先一端"）

- 验收：（2026-10-16 晚 pass——[ruling](../../rulings/2026-10-16-GENSHELL-CPP-1-pass.md)；合入=cherry-pick 3337bd7e→7745719a@main；决策侧独立复验=合入树零差异+Release 构建 exit 0+壳自测 GREEN+两次全绿 run 工件亲读并补档 coord/runs/GENSHELL-CPP-1/；瑕疵=回执构建命令简写缺 BUILD_TYPE（README 权威）；解锁池序 96/99）
