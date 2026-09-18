# PORT-B2P：CEF mac 集成探针（上游 universal framework + 最小工程实测，消 R3）

- 优先级 / 预估 / 依赖：P1 / 1-2 天 / 无（与 B3 并行；审计 B2 的去风险前置件）
- 模型分级：L2 / GLM-5.3（CEF/Godot 集成面 + 渲染路径判断）
- 目标：消除 R3（**最后一个未真机验证的移植风险**，PORT_AUDIT §4）：不依赖前端工程，在 mac 用最小 Godot 4.6 工程实测 godot_cef addon：
  - ① 从上游 [dsh0416/godot-cef](https://github.com/dsh0416/godot-cef) **v1.15.4** release zip 提取 `bin/universal-apple-darwin/Godot CEF.framework`（zip 约 1GB，**不入仓不入 transfer**——超 100MB 限制，落探针工作区）
  - ② 建最小探针工程（本机 Godot 4.6.stable）：装载 addon（.gdextension 清单 macos 条目 + framework 放置路径按 addon 结构）
  - ③ 实测链：CEF 实例化 → 加载本地 HTML（探针工作区自备，含 JS 与 canvas 内容以便目检出图）→ OSR 纹理出图（**Metal 优先，失败降软件渲染，记录实际走的路径**）→ 出图证据（截图 PNG 入工件）
  - ④ helper 进程证据：CEF 子进程拉起的 ps 实拍；退出干净（无僵尸 helper 残留，退出码记录）
  - ⑤ 产出**集成手册草稿** `docs/CEF_MAC_PROBE_2026-09.md`：framework 放置、清单条目、渲染路径、已知坑（含 H.264/AAC 缺席的影响评估——WebUI 无媒体内容则无影响）、给 B2 正式集成的步骤清单
- 文件域：探针工作区 `~/Documents/vit-b2p-probe/`（仓外，自备工件目录）；仓内唯一产出 = `docs/CEF_MAC_PROBE_2026-09.md`（走 port/* 分支）；上游 zip 与 framework 不入任何仓
- 验收标准：①本地 HTML 出图证据（PNG + 渲染路径标注）；②helper 进程 ps 证据 + 干净退出证据；③渲染路径记录（Metal/软件，实测为准）；④手册草稿入分支（含 B2 正式集成步骤清单）；⑤工件目录：run ID、探针工程副本、日志、截图、下载记录（重试/代理使用如实记录）
- 停止条件：上游 v1.15.4 无 mac 资产或 framework 损坏（校验不过/架构不符）→ 上交证据（审计"已确认可用"失效，升级决策侧）；CEF 在 arm64+Godot4.6 组合崩溃 → 按类型记录证据转 blocked（R3 兑现为实际阻塞）；下载持续失败（网络）→ 按环境中断记录，不判功能结论
- 领取：2026-09-18 22:13 CST / origin/main=e8d11e05cdea339478787559e2b973f73a23992d / 分支 port/b2p-cef-mac-probe（独立 worktree ~/Documents/vit-b2p-worktree）
- 回执：完成 2026-09-18 22:47 CST。分支提交 5347a43（领卡）+ 0484111（docs/CEF_MAC_PROBE_2026-09.md 手册，本卡唯一仓内产出）。R3 **消除**：三轮有效实测 exit 0——A3 `--rendering-driver metal`+加速 / B2 默认驱动+加速 / C2 默认+软件；均 `[CefTexture] Creating browser in accelerated|software rendering mode`、res:// HTML `load_finished status=200`、OSR 出图（viewport.png 1280x800 非背景比 0.975，软件轮另有 browser_texture.png 直读；两路径截图均目检确认完整渲染）、console_message/ipc_message 回传、5 个 helper 进程 ps 实拍（PPID=godot，GPU ~12% CPU）、退出零残留。证据更新审计前提：Godot 4.6 mac 默认 Forward+ 即原生 Metal（非 MoltenVK），加速路径无需旗标开箱即达。工件：~/Documents/vit-b2p-probe/（PROBE_SUMMARY.md + runs/run-20260918-224355-A3-metal-accel、run-20260918-224437-C2-software、run-20260918-224449-B2-default-accel 有效轮；223305/224421/224426 为流程性无效轮已标注原因；downloads/ 直连超时→代理 1.3MB/s 13min 如实记录；zip+framework 未入仓未入 transfer）。无效轮说明：223305 为 gdextension 未注册（缺 --import，探针流程坑，已写入手册 §4.1）、224421/224426 为 runner bash3.2 空数组 bug 未起 godot——均非 CEF 功能失败。端测边界：本卡无 Go/webui 代码改动，AGENTS §5 真实栈烟测门槛不适用；验收面=探针工件与手册（卡内验收标准①-⑤全覆盖）。
- 验收：
