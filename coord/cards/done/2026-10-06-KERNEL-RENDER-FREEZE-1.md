# KERNEL-RENDER-FREEZE-1：start_render 冻死 JUCE 消息线程——内核侧锚点补证+修复方案上交（MIDI-RECON-1 K1 腿）

- 池序 8（MIDI-RECON-1 验收产出；**取证先行卡**——修复方案上交决策后另立实现卡）；目标仓库=D:\Vit_DAW（PC 执行侧）；来源=[rulings/2026-10-06-MIDI-RECON-1-pass.md](../../rulings/2026-10-06-MIDI-RECON-1-pass.md) 缺口 K1
- 优先级 / 预估 / 依赖：P0（渲染链路不可用+冻结波及全命令面）/ 取证 0.5 天 / 无
- 模型分级：L2 / GLM 首选（内核死锁分析）；codex 可接取证腿（小粒度+验收关口）

## 已核实事实（MIDI-RECON-1 双轮真栈复现，勿重复跑）

1. `render.start`（canonical VSP 命令）回 "Render started" 后，内核 JUCE 消息线程停止处理一切后续命令（只入队不处理）；两轮独立栈同型复现（run `coord/runs/MIDI-RECON-1/20261006_190707` 日志行 39438 后零 processing；run `20261006_191932` 行 10775=render.start 最后处理）。零 render_done/render_failed 遥测、零产物文件。
2. 122 秒渲染看门狗（`VitProductionCoordinator.cpp:23655` 附近）未生效——疑因看门狗自身挂在消息线程上。
3. 冻点链路：`startOfflineRender` → `te::EditRenderer::render` 异步回调链（`VitProductionCoordinator.cpp:630-739`）。
4. 环境共同点：headless 烟测栈（无音频设备/无 GUI）。

## 待证假设（本卡核心）

- H1：headless 无音频设备下 EditRenderer 上下文初始化/设备查询在消息线程上同步阻塞或死锁。
- H2：异步回调从未被调度（线程池/消息循环未跑）vs 回调被调度但内部死等——两者修复面不同，须区分。
- H3：看门狗失效机理（挂消息线程=与冻点同线程共死？还是计时器未启动？）。

## 目标（取证，只读+诊断日志）

1. 代码锚点补证：沿 `VitProductionCoordinator.cpp:630-739` 渲染链逐帧列出调用栈（同步段/异步段分界、哪些调用发生在消息线程）；对照 tracktion EditRenderer 源（本仓 third_party 内）确认设备查询点。
2. 最小复现：烟测栈起内核→单发 render.start→线程 dump/日志证据（可用内核诊断日志或进程级线程栈抓取，工具自选申报）；区分 H1/H2。
3. 修复方案建议（不实施）：候选面如"渲染前设备探测短路+明确 render_failed 遥测""渲染搬离消息线程""headless 模式渲染降级声明"——各自影响面列清，上交决策侧定。
4. 产出报告落 coord/reports/，工件 run 目录。

## 文件域

- 只读取证；若需内核诊断日志，仅限 `VitProductionCoordinator.cpp` 渲染链相关行（申报制，≤10 行），出报告后随实现卡定去留。

## 停止条件

- 复现不了冻结（环境差异）→ 工件+环境差异记录上交，不硬凑结论。
- 需要 tracktion 侧补丁才能修复 → 停在方案建议，不碰 third_party。

## 领取：2026-10-06 晚 / origin/main c8d274004d62fc097807b532f2625bad27d3505a / port/kernel-render-freeze-1（领取时工作树仅既有 Workspace 运行时状态文件与 coord 工件，无源码改动；doing/ 目录缺失已补建）
## 回执：报告 [coord/reports/2026-10-06-KERNEL-RENDER-FREEZE-1.md](../../reports/2026-10-06-KERNEL-RENDER-FREEZE-1.md) / 主复现 run `coord/runs/KERNEL-RENDER-FREEZE-1/20261006_210431`（当前 HEAD 内核 959c06cf，冻结复现+栈+WCT+minidump 三件套；首试 20261006_203747 行为级复现，栈因工具停滞未取得已申报）/ **诊断日志 0 行、生产代码改动 0**（port/kernel-render-freeze-1 空分支可清理）/ 核结论：ABBA 死锁——完成回调 join 渲染线程 vs 渲染线程收尾 callBlocking 等消息线程；H1 否、H2 后半成立、H3 坐实（看门狗=消息线程 timer 共死）；失败路径确定性冻（非竞态）；修复建议 A（Handle 析构搬离消息线程，根因）+B（无音频内容前置短路，纵深）+C（结构性搬离，长期不推荐）+看门狗线程化加固，上交决策 / 拆栈：graceful 无回包→taskkill /F 33280→端口复查净
## 验收：**pass 2026-10-06 晚（决策侧）**——裁定 [rulings/2026-10-06-KERNEL-RENDER-FREEZE-1-pass.md](../../rulings/2026-10-06-KERNEL-RENDER-FREEZE-1-pass.md)。亲核：代码锚点逐条对上（~Handle join/失败路径不 reset/~NRC:168 callBlocking/NRC:94-96 早退/VPC 四锚）；工件亲读（46 线程双栈逐帧+WCT 等待链 7188→34304 直连+probe 冻结指纹 5s 整超时）；ABBA 机制与 n≥5 同型复现自洽。零代码改动+Sep-06 战役复用认可。修复裁定=立 KERNEL-RENDER-FREEZE-FIX-1（方案 A+A'+B 同卡，P0 例外条款）；render_wedge_evidence 沉淀上交用户；空分支清理。
