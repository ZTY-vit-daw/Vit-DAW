# 风险表：逐条结论（卡面验收项）

- 归属：GEN-PLUGIN-SHELL-DESIGN-1 / 2026-10-12
- 卡面点名三项（VST3 SDK 许可/签名分发/异步边界）+勘察新增三项，共六项逐条结论。

| # | 风险 | 结论（裁定+依据） | 残余风险与缓解 |
|---|---|---|---|
| 1 | **VST3 SDK 许可** | VST3 SDK 双许可（GPLv3 ∨ Steinberg VST3 Plug-In SDK Licensing Agreement）。本项目闭源/学术发行**走 Steinberg 协议**（免费、允许闭源、须随发行附许可文本与商标条款——对外宣传按其 trademark 附件措辞，不自称"VST 兼容"超范围表述）。SDK 获取路径已有仓内先例：PluginProbe native host 已用 VST3 Hosting 层（PluginProbe/native-host/src/main.cpp:5,619-642 亲勘）。VSTGUI=Apache-2.0（宽松，可闭源随行） | 风险表内可控；卡 1 第一步核实执行端 SDK vendoring 实路径（PC 完整检出；Mac worktree 子模块空=worktree 工件非仓库态），结论入卡 1 回执；许可文本随发行包分发=发版流程项（release 脚本挂接，非本设计域） |
| 2 | **签名与分发** | 本产品自产自宿主（宿主=VitApp 内核，同机装载标准 VST3 目录）——**macOS 本地开发 ad-hoc 签名即可装载**（arm64 本机运行不受 Gatekeeper 外来隔离面）；对外分发（含给第三方宿主用户）需 Developer ID 签名+公证，Windows 无签名强制。发行渠道=随应用包/独立 zip，安装即拷贝至 VST3 目录 | 对外分发前补签名流水线（发版卡范围，不在本组卡）；未签名分发给他人=已知不可装载边界，发行说明如实标注 |
| 3 | **异步边界**（真难点，前提 3 点名） | 逐子项裁定：①提交可靠性=队列+轮询 at-least-once（件二 §4，不依赖 PUB at-most-once）；②agent 重启=孤儿清扫→interrupted（件二 §7）；③内核重启=壳 chunk+job record 双持久自愈；④provider 侧不可真取消=放弃产物不落轨，费用如实告知；⑤渲染期互斥=runner 等 render_done 后再 ingest（件二 §6 时序纪律）；⑥take 栈内存态=跨会话降级为文件重指提示，不冒充（件二 §6） | 轮询延迟（≤2s+500ms GUI）在分钟级作业上可忽略（接受）；`cancel_requested` 经 job_state 字符串承载（值域扩展）——既有读方未知值不崩已声明（件二 §1 §11） |
| 4 | **通道检索锚点**（IConnectionPoint 取得路径） | 主选=经 JUCE VST3PluginInstance 取 IEditController→queryInterface（件一 §2）；tracktion_engine 子模块源码在 PC 完整检出可核（本设计 worktree 空=工件态）。降级 A=参数面+chunk 编码（语义无损，延迟+百毫秒）；降级 B=loopback socket（仅 VitApp 宿主） | 主选/降级 A 均不通才触发卡 2 停止条件上交（概率低——JUCE 宿主侧 VST3PluginInstance 为公开类，包装层源码在子模块内可审视）；降级 A 启用时厂商兼容面收窄至"参数面可达"（如实入契约文档） |
| 5 | **壳实现基选择**（VST3 SDK 直实现 vs JUCE 插件） | 主选=SDK 直实现+VSTGUI（件一 §7：T2 控制权+免 patch 子模块+厂商标准栈）；fallback=JUCE 插件+通道降级 A | VSTGUI 表达力不足表达四类型表单→卡 4 停止条件（自绘降级，范围上交）；SDK 直实现的样板代码量（UIDescription/编辑器框架）已计入卡 1 预估（2-3 天） |
| 6 | **壳崩溃=内核崩溃**（无进程外隔离） | 接受风险（自产小代码面二进制，vs pluginprobe 的 crash-isolated 只读 worker 面向第三方插件——vst3host/adapter.go:24-30 亲勘对照）；缓解=壳音频线程零分配零锁纪律、解码/文件 IO 全后台线程+异常包络、VSTGUI 仅主线程、卡 1 烟测含 show_plugin_editor 存活断言 | 壳缺陷致内核崩溃=同其他自研内核代码同级风险（不因插件形态额外隔离——"未来厂商插件"若引入才需评估进程外隔离，v2 议题如实声明） |

补充边界（非风险项，如实声明）：

- **上下文真值双源风险**：已由件一 §6 设计钉死（注入真值唯一=genctx 快照；IProcessContext/context_snapshot 仅展示）——列此备查，实现卡单测覆盖（壳内无自动块拼装路径的代码级断言）。
- **GUI 测试边界**：插件编辑器视觉面不属自动化端测（AGENTS §5 渲染面/旅程门槛现状）；各实施卡端测边界声明已写入件五。
- **take 栈跨会话**：既有内核边界（VitTakeHistoryStack 进程内存态），壳不修复不冒充（件二 §6）——如需跨会话候选=另开内核卡（决策侧）。
