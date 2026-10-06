# KERNEL-RENDER-FREEZE-FIX-1：渲染 ABBA 死锁修复——Handle 析构搬离消息线程 + 看门狗线程化 + 无声内容前置短路

- 池序 5（取证卡 KERNEL-RENDER-FREEZE-1 验收产出，P0 产品缺陷修复；按 2026-10-06 刹车决策例外条款立卡——独立于节目线评估点）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 来源=[rulings/2026-10-06-KERNEL-RENDER-FREEZE-1-pass.md](../../rulings/2026-10-06-KERNEL-RENDER-FREEZE-1-pass.md) + [取证报告](../../reports/2026-10-06-KERNEL-RENDER-FREEZE-1.md)（机制/锚点/方案全部现成，勿重新勘察）
- 优先级 / 预估 / 依赖：P0 / 0.5-1 天 / 无（报告已闭合根因）
- 模型分级：L1 / GLM 首选或能力较强执行侧（线程生命周期语义，修复面小但需谨慎）；codex 可接（粒度+验收关口齐）

## 已核实事实（取证报告已双核，下游直接引用）

1. 死锁环：完成回调（消息线程）`renderHandle.reset()`→`~Handle`→`renderThread.join()`（VPC:696，tracktion_Renderer.cpp:663-667）vs 渲染线程捕获析构→`~NodeRenderContext:168 callBlocking` 等消息线程——FIFO 保证失败路径确定性互等。
2. 两违规点：①VPC:696 `renderHandle.reset()` ②VPC:691→:979-985 `releaseWedgedRenderHandle` erase（迟到回调路径）。
3. 看门狗=消息线程 50ms timer 驱动（VPC:972-977 仅写原子时间戳；checkRenderWatchdog 由 tick 调用）——与冻点共死。
4. parking 先例现成：`wedgedRenderHandles`（VPC:1006-1010）。
5. 触发面：任何"启动后失败/取消"的离线渲染；MIDI-only 工程（无 instrument）必现（NRC:92-96 "Didn't find any audio to render" 失败路径不 reset context）。
6. `cancel_render` 已注册（CommandDispatcher.cpp:3155）当前被死锁掩埋。

## 目标（三腿，A+B 同卡不可拆）

1. **腿 A（根因）**：完成回调与迟到回调中永不在线程上析构 Handle——`renderHandle.reset()` 改为 move 给 detached 清理线程（或并入 wedgedRenderHandles 后台清扫），join 发生在清理线程上。两违规点同改。真 wedged 插件场景允许清理线程暂存（泄漏线程告警即可，不冻消息线程）。
2. **腿 A'（看门狗加固）**：`armRenderWatchdog` 起 detached 定时线程，到期只做原子标志翻转+发布 render_failed（不碰 Handle 析构）。timer 路径保留兼容。
3. **腿 B（前置短路）**：`startOfflineRender` 同步段预检工程有无可发声内容（audio clip 或 instrument），无则同步回明确 error（人话："工程里没有任何可发声内容（MIDI 轨无乐器），离线渲染必失败"），不起 EditRenderer。
4. 门：`cd agent && go build ./... && go test ./... -count=1` 0 FAIL（回归面）；C++ 侧编译 exit 0。
5. 真栈烟测扩展（dev_agent_smoke.ps1 场景，参照 KRF1 复现序列反转为绿）：MIDI-only 工程→render.start→**收到明确 render_failed/render error 回执（不再冻结）**→后续命令（get_project_state/ping）继续正常响应→render_failed 遥测到达→cancel_render 路径可用断言。**exit 0 方算交付**。
6. 回执：diff 锚点、新内核 sha256、烟测 run ID、泊位声明、HEAD。

## 文件域

- `VitApp/Source/Service/VitProductionCoordinator.cpp`（腿 A 两违规点+腿 A' 看门狗+腿 B 预检）
- `scripts/dev_agent_smoke.ps1`（新场景或扩展 midi_register 同域场景）
- 测试如需：VitApp/Tests/ 下新增
- **禁改**：tracktion_engine submodule（third_party 红线）、agent 侧任何文件、CommandDispatcher.cpp

## 约束

- 接口零变化：render.start/render_failed 遥测 schema 不动（B 腿的同步 error 回执是新语义，回执申报）。
- 清理线程与下一 job 的 jobId 竞态沿用现有 stale-jid 判定；L2 probe 分支（VPC:699-719）同一完成 lambda 同受益，验收断言覆盖。
- 真栈泊位纪律；渲染回归面：`scripts/run_ab_result_smoke.ps1` 须 PASS（渲染族不受损）。
- 提交：分支 `port/kernel-render-freeze-fix-1`，coord 直推 main，实现等验收 cherry-pick。

## 停止条件

- 修复后仍冻结（机制理解偏差）→ 栈工件取证上交，不盲改。
- 清理线程方案撞上 tracktion 内部假设（Handle 生命周期契约）→ 停上交，带证据。

## 领取：2026-10-06 深夜 / origin/main `97af10a9` / 分支 `port/kernel-render-freeze-fix-1`（领取前工作树=Workspace 运行时状态文件+coord 未跟踪 run 工件，非本卡域）
## 回执：
- 实现 commit：`port/kernel-render-freeze-fix-1` @ `4984ef66`（4 文件：VitProductionCoordinator.cpp/.h + RenderWatchdogTests.cpp + dev_agent_smoke.ps1，+615/-17；等验收 cherry-pick）
- 烟测 run ID：`coord/runs/KERNEL-RENDER-FREEZE-FIX-1/smoke_render_freeze_1`（exit 0，17 断言组全绿）+ `smoke_render_freeze_2`（复跑 exit 0）+ `smoke_midi_register_regress`（共享脚本回归 exit 0）；先行验证探针 `20261006_verify1`（run 清单见 `coord/runs/KERNEL-RENDER-FREEZE-FIX-1/manifest.md`）
- 新内核 sha256：`b6565dcf85d1da867e16333052a7b928a5413565940603adbcf4abd59dd3123b`（已部署 `Export/staging/runtime/VitApp.exe`；部署前=`959c06cf…2bd7c9`=取证构建）
- 泊位声明：两轮 berth 自起自拆（agent+kernel stopped，端口 5555/5556/7878 复查净，无残留进程）；探针内核 2 次 taskkill //F 已申报于 manifest；go 全量 0 FAIL（55 包）；VitRenderWatchdogTests exit 0；run_ab_result_smoke.ps1 exit 0；C++ Release 编译 exit 0
- diff 锚点：腿 A=`retireRenderHandleOffThread`（VPC 匿名 ns）+完成回调 4 处 reset 改 retire（:696 原位+同文件 dual-tap :831/:871/:909 同型违规一并修，属卡面"消息线程零 Handle 析构"不变式内）+`releaseWedgedRenderHandle` erase 改 move-out+retire；腿 A'=`armRenderWatchdog` detached 定时线程+`renderWatchdogSignalled` exactly-once 门（timer 路径 force-clear/park 行为保留兼容，测试日志计数断言改鲁棒下界）；腿 B=`editHasRenderableAudioContent` 预检+`reason=no_renderable_audio_content` 同步 error（新回执语义申报：消息为 ASCII 英文——内核无 /utf-8、CP936 源码解码下中文字面量运行时乱码，卡面中文为人话语义、语义已对齐）
- 端测边界声明：场景确定性断言=腿 B 同步拒绝+不可写目标盘异步失败反转（render_failed 遥测经 kernel PUB→agent 到达+命令面活，零竞态：既有 createDirectory 结果被忽略行为，已实证 2 次）；live-cancel 组因渲染速度与取消往返存在真竞态（两轮分别 failed/ready），仅断言 cancel 回 ok+任一终态+命令面活，不设竞态门；看门狗 A' 线程路径由 VitRenderWatchdogTests 覆盖（真栈 122s 触发不进烟测）；渲染族回归=AB 烟测+空范围 render_done+文件存在断言
- 附加发现：`checkNodesForAudio`/`props.hasAudio` 按全工程判定（音频剪辑在范围外→静音 render_done 而非失败）——空范围不能当异步失败触发器，已记入 manifest；PS 5.1 `Invoke-WebRequest` 错误体在用户 catch 前被吸干，场景 helper 走 `System.Net.Http.HttpClient`
## 验收：**pass 2026-10-06 深夜（决策侧）**——裁定 [rulings/2026-10-06-KERNEL-RENDER-FREEZE-FIX-1-pass.md](../../rulings/2026-10-06-KERNEL-RENDER-FREEZE-FIX-1-pass.md)；cherry-pick 4984ef66 → main `79b8ecdc`。亲核：diff 四处 retire+exactly-once 门+保守权衡全对；烟测双轮工件亲读（腿 B 同步拒绝/腿 A failed 终态+命令面活=旧内核物理不可能的回执自证/不可写目标盘零竞态触发器方法学认可）；内核 sha256 逐字一致；决策侧复跑 go 全量 88 包 0 FAIL+AB 冒烟 exit 0。dual-tap 三处同型扩展+live-cancel 竞态边界+ASCII 消息申报均认可。K1 关闭；JOURNEY-1 渲染环节解锁。
