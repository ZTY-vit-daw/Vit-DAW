# 409 主管验收：当前 Windows 产品开发入口通过

DAW 由 VitShell 打开时，使用仅本次启动生效的最小化引导、显式 HWND/PID/nonce 引用及 Godot 自有 UI 线程守卫；首次普通窗口显示锚定在对话窗下。最多一次非激活后纠正仍保留，但本次两条成功腿均未使用。

主管：Codex `/root`，用户 2026-11-18 指定接任 PC 主管。生产代码由独立执行子代理完成，主管定向审查、修订验收装置、运行最终真实栈并裁定。领取和交接见 [CLAIM-HISTORY.md](CLAIM-HISTORY.md)，逐次运行授权见 [RUNTIME-PLAN-01.md](RUNTIME-PLAN-01.md)。

## 最终证据

| 条件 | run | script / wrapper / host exit | 样本 | 首显在 reference 下 | 激活 / 焦点消息 | 后纠正 |
|---|---|---|---:|---|---|---|
| VitShell 自然前台，用户仅配合置前 | runtime_shell_06 | 0 / 0 / 0 | 792 | 是 | 0 / 0 | 无 |
| 当前 Codex 独立前台，指定 PID 稳定 2 秒后开工程 | runtime_external_07 | 0 / 0 / 0 | 805 | 是 | 0 / 0 | 无 |

- 两轮均走真实 VitShell、Go agent、VspHub、VitApp、官方 Godot 4.6.1 GUI 可执行文件和完整产品工程；工程打开、agent 当前工程与项目 DOM 均通过，无 reload sentinel 丢失。
- 实际 Godot PID 分别为 16672、29932，reference PID 分别为 22712、8668；引用准入有效。两轮 native rect 均为 `[28,28,2532,1412]`，workarea `[0,0,2560,1440]`、DPI 168，即 16 DIP 内边距。
- 每轮使用全新 TEMP 产品/运行副本及 fixture 副本；产品 LLM 调用均为 0。宿主正常退出，无脚本强制清理；主管另行核对全部 Vit/Godot 进程、六口 Listen/Bound、UDP4445 净空。
- recents、权威 fixture 源、主仓隔离检出及原主检出的 Settings/default_project 共六份受保护输入，运行后逐份核哈希一致。未把临时工程、用户配置快照或构建二进制纳入源码提交。
- 真实产物：VitShell SHA256 `0A9541F823E76D7B6C1105764AE1CAF9C4CC40AFCFB83F0A02DCF3D61770D73D`；Debug GDExtension `8323F43BFD6CD6C91404DDD120B6B10BA38FEFE27A34B8DFA256BC4678927EF4`。agent/kernel/hub/WebUI 哈希及完整命令在各原始报告中。

原始报告、快照、逐次采样、agent/hub/bridge/Godot 日志、前后哈希及清理证明在 [evidence/runtime_shell_06](evidence/runtime_shell_06/) 与 [evidence/runtime_external_07](evidence/runtime_external_07/)；原始文件复制哈希见 [source-hashes.json](evidence/source-hashes.json)。目录属性关闭文本换行转换，保留原始字节。

最终原生 Release 构建和 CTest exit 0，67 checks、0 failures、0 windows；分类器 29 项合成检查通过。完整扩展已编译链接并在 4.6.1 注册/实际加载。早段 Go agent、WebUI 构建均通过；本卡未改两者生产源码，未重复全仓测试。见 [evidence/no-window](evidence/no-window/)、[pre-runtime](pre-runtime/) 和 [执行回执](STAGE2-PREP-20261118.md)。

## 提交与范围

- 前端源码已合入 `main@bc8daf6bce103c901bcf8679bb536ef124dbd34c`，16 个限定路径；提交前逐文件核对与被测产品副本一致。没有带入 394/402 失败实现，没有修改 396 的 agent_chat_window 或 406 取证文件。
- 主仓工具已合入 `main@83bd5e77b236c824a38378cd646f3c448e90bc52`，四个 smoke/observer/classifier 文件；原执行提交 `f5adb958`，主管序列化 cherry-pick 后逐文件核对一致。
- shell06 使用最终日志捕获和默认准备路径；之后新增的可选 ExpectedExternalPid 分支及操作等待上限只影响 external 准备条件。external07 使用最终脚本 SHA256 `8C1AD902CD9282251997127AB99636CA2620779138DB4F027EB002CF05DBBF35`；生产候选、DLL、host 和冻结功能分类器均未变化。

## 覆盖边界与原失败轮

此次通过只证明两个指定自然前台条件各存在一条成功路径。消息计数覆盖自有 subclass 挂载至拆除；前后轮询存在采样间隙，不证明每个渲染帧或整个开工程区间的绝对零激活，也不构成稳定性成功比例。native 首显和任何后纠正分别记录；未以事后正确状态抵销首显反例。Win32 失败分支、无效引用的真实窗口恢复及不同显示环境未穷尽。

验收对象是官方 Windows editor/project 直启开发产品入口；导出模板、发行打包和实际手测入口产物刷新未在本卡完成。`override.cfg` 日志落点未生效的根因仍未定位；最终装置对默认日志在开前留原始副本/哈希，并按本轮 Godot/reference PID、HWND、tick 和轮询窗口匹配拒绝 stale，完整 raw 捕获随 run 保存。

所有 13 次装置调用保留原始退出码和分类，索引见 [attempts/summary.json](evidence/attempts/summary.json)。早段装置的 lease 解析、参数引号、就绪同步、UTF8、退出等待问题在修正后才另跑新目录；缺自然条件的轮次均为 exit 3，未算成功。external01 原 exit 4 保留，身份匹配的补日志分类 PASS 仅为补充，不改写原判。

external05 原 exit 1 的原因是观测期间外部前台发生变化。用户报告一次切回 Codex，以及 WeChat 自行弹更新并确认；原生采样记录了多个其他窗口/空前台变化。该轮 DAW 首显/geometry/guard、DOM/agent state 均正确，未观测到 DAW 前台或焦点、消息计数为 0，host exit 0，未见崩溃/脚本错误。测试随后主动 `/close` 收栈符合用户观察到的“开工程后自动关闭”；这不是把该轮改为 PASS，也未逐次推断外部窗口变化的来源。

391 整体体验与手测仍待 395 的装载呈现兼容复核、组合真栈及实际入口刷新。本轮用户配合仅设置自动化前台条件，不替代功能验证或放行 391。下一步先复核 395 对新窗口基线的依赖，当前不再启动新运行。

## 调度与消耗

6.2 显式子代理覆盖在工具层未获支持；旧代理恢复后还实际遇到 gpt-6.2-sol 通道 503，已关闭并登记交接。接替代理显式配置 GPT-6.1 Sol/high；未让 Astra 承担实现或常规复核。无 watcher、后台调度或其他聊天自动投递。

截至 14:17 的本地去重用量检查点见 [TOKEN-CHECKPOINT-1417.md](TOKEN-CHECKPOINT-1417.md)。计数用于 18 点比较投入与验收产出，不等同于计费或供应商额度。
