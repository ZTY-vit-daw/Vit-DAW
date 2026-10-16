# GENSHELL-HOST-1：宿主侧服务+通道（实施②）

- 发卡：Mac 决策侧 / 2026-10-12（依据=件五 卡2）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 96；**规格全文=[件五 卡2](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P1 / 1-1.5 天 / **GENSHELL-CPP-1 合入后（✓ 2026-10-16 pass 合入 7745719a——依赖满足，可领；锚点消费卡 1 回执"关键实现事实"五条）**
- 模型分级：L2 / GLM 执行会话

## 要点摘录（以规格为准）

内核 GenerationShellService（壳实例注册表+IConnectionPoint host 端+提交队列+新读命令 gen_shell.pending+五类下行推送）。文件域=GenerationShellService.{h,cpp} 新文件+CommandDispatcher.cpp 仅注册区 2784-2803 邻域（**不触披露块 3291-3448——GEN-CTX 域避让**）+CMakeLists 登记+冒测脚本。验收=真栈烟测（伪提交/pending 应答/JobState 镜像/cancel/ghost 断言）+主选路径核实结论+agent 零改动零 diff。停止条件=裸 VST3 接口无路径（主选+降级 A 均不通）→上交降级 B 决策点。

- 领取：2026-10-16 21:34（Mac 本地）/ origin/main ba820dcf / owner=GLM-5.3 Mac 执行会话 / 分支 port/genshell-host-1 / worktree /Users/timozty/Documents/Vit-DAW-genshell2（coord=Vit-DAW-genshell2-coord）/ 领取提交=eec9d65b（coord-only 直推 main，重订基于并发 PCA-1 领取 6ed8824d 之上）
- 回执（2026-10-16 收工）：
  - **实现 commit**：`dce49fac` @ `port/genshell-host-1`（已推 origin；5 文件：GenerationShellService.{h,cpp} 新 830 行+CommandDispatcher.cpp 注册面 13 行（include+lambda 持有 shared_ptr 注册，不触 .h 与披露块 3291-3448）+CMakeLists 12 行（源登记+VST3 SDK include 路径经 tracktion 子模块）+scripts/gen_shell_host_smoke_mac.sh；领取基线 ba820dcf，agent/ 零改动零 diff 亲核）
  - **T2 主选路径核实结论（卡验收 2）**：**主选不通→降级 A 启用，队列语义不降（烟测同断言全过）**。证据链（静态亲勘+运行时探针双证）：① JUCE 装载期互联壳 processor↔controller 两端点（juce_VST3PluginFormatImpl.h interconnectComponentAndController， CPP-1 回执事实②证实）；② SDK ComponentBase::connect 单 peer fail-closed（已有 peer 再 connect=kResultFalse，vstcomponentbase.cpp:66-77 亲读）；③ 独立类壳 component-qi(IEditController)=kNoInterface（运行时探针实测；JUCE fetchController 工厂回退路径佐证）；④ JUCE 包装层活体 IEditController* 私有成员无公开访问器（getPlatformSpecificData 仅出借 IComponent*）；⑤ 无外部 disconnect 通道（disconnect 需 peer 引用，无 getter）。**降级 A 载体全公开 JUCE API**：te::ExternalPlugin::getAudioPluginInstance→参数面（te::AutomatableParameter，与 set_plugin_param 同径）+getState/setStateInformation；服务含 pending_requests chunk 轮询 scaffold（卡 4 壳侧写入口就位即活）与 IConnectionPoint host 端点（探针 connect 目标+未来厂商壳上行入口，notify 按 fact④ 丢弃 genshell_state_sync）
  - **真栈烟测**：`scripts/gen_shell_host_smoke_mac.sh` **两次连续 all_green exit 0**（同代码版本 run `genshell_host_mac_20261016-221150`+`-221207`，26 ok/0 red，工件 `~/Documents/vit-genshell-artifacts/host/<run>/`）。内核=本卡 worktree 构建 sha256 a08f7593…（含全部本卡改动）；工程走 VIT_PROJECT_XML 隔离副本。leg 明细：S 壳单测 46 PASS 回归（已部署 bundle 未 redeploy——PCA-1 在飞指纹基线不受扰动）/H1 scan→rack_add_node（UI 路径形态）五参数在场/H2 注册表+探针（qi_edit_controller=kNoInterface+qi_connection_point=kResultOk+connect_attempt=kResultFalse+t2_transport=downgrade_a_param_chunk）/H3 伪提交（debug_submit 测试钩子=卡面许可路径）：pending 应答 queued+request 载荷+providers 形态+jobs 快照 job_state=queued/ghost=mute/source=gen_shell:\<node\>/request_hash 在场+重复 job_id 拒绝负探针/H4 伪推进（aigc_register_job 整记录 upsert=agent runner 同面）：JobState 镜像 running(2/7)→done(5/7) value_text+normalized 双断言/H5 取消：cancel_requested 应答+快照/H6 终态 cancelled：ghost mute→bypass 清除+JobState 镜像=cancelled
  - **既有命令面零回归**：agent 全量 `go test ./... -count=1` **0 FAIL**（584 包 ok）+agent/ 零 diff；内核全量编译 Built target VitApp，本卡文件零警告（基线既有警告=tracktion 子模块内部+CommandDispatcher 既有 unused-function 行 1316-1771，均在 diff 外）
  - **调试期 RED 两次已根修**（run 220552：appendPendingAnswer 对 snapshotProjectJobs 临时返回数组的悬垂指针——map 改持 var 引用；脚本 python 格式串/标识断言两处；run 221004：debug 钩子先于注册表刷新的顺序缺陷——首调用即提交场景）
  - **边界如实**：① providers 恒空数组（内核无 genapi 知识，卡 3 runner 自带目录；形态按件二 §4 在场）；② cancel_requested/interrupted 无 JobState 参数枚举值不镜像（记录态照常应答，件一 §3 枚举无此二值）；③ context_snapshot/provider_catalog/audition_ready 壳侧消费者在卡 4（骨架壳无下行消费面），v1 由服务状态承载（GUI 500ms 进程内轮询语义）；④ 请求载荷 v1 服务内存态（重启恢复依赖壳 chunk 写入口=卡 4）；⑤ requestHash=sha256 为服务内自含实现（JuceHeader 无 cryptography 模块）；⑥ 端测边界：内核数据面断言（ZMQ 直驱），无 Godot 前端旅程断言，无 webui 改动；⑦ VitApp/Workspace/Settings.xml 为内核运行残留未入 commit

- 验收：（2026-10-16 深夜 pass——[ruling](../../rulings/2026-10-16-GENSHELL-HOST-1-pass.md)；合入=cherry-pick dce49fac→db84003e@main；决策侧独立复验=diff 域亲读（CommandDispatcher 仅注册面+13，未触披露块）+两次全绿 run 工件亲读（26ok/0red×2）并补档 coord/runs/GENSHELL-HOST-1/+降级 A 锚点亲核（vstcomponentbase.cpp 单 peer fail-closed+H2 探针腿在案）+agent 零 diff；构建采信执行侧两次 run 内核 sha a08f7593（复验边界如实入 ruling）；解锁池序 97 AGENT-1）
