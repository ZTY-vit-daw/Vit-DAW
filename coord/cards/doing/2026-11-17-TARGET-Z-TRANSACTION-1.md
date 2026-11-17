# TARGET-Z-TRANSACTION-1：目标自线程临时z事务

- 池序408；P1；PC；发卡/验收=root；派发确认=用户继续直到手测，主管据407真实反例拆必要卡。owner=/root/restore403 Sol/high，PC；407停写转408，前端同406独立tree/branch@ae86c7d，主同403tree/branch@1aec3670，trackedclean/旧cache保留；共享main基线8dadb001，领取push重读后实施。
- 单目标=在被测Godot自己的UI线程上，窗口首次ordinary转换应用前将其z提议固定到自有guard后，保geometry/mode/noFG、零显示后纠正。不是宿主跨进程hook；仅目标显式testloader的同线程self-subclass。
- 文件域4：原observer.h/.cpp、probe.cpp、主既有vitshell_smoke.ps1必要显式408/TestTargetZGuard门。testonly新机制、无生产源码。parent referenceHWND/PID/nonce必须已验证；childcallback无IO/heap/pump。
- 默认observer-only不改；仅新显式env启用z事务。safe_armed后已有临时NOACTIVATE、原5步/API顺序不改。WM_WINDOWPOSCHANGING每一嵌套层先原样记录entry，DefSubclassProc后记录downstream实际提议，再在返回USER32前检查exactlive reference与nonce/PID/TID/状态，用已核reference设置WINDOWPOS.hwndInsertAfter=reference，并清SWP_NOZORDER（仅需要时）；禁止修改NOACTIVATE/NOOWNERZORDER（文档该消息改无效），必须原发起API与exstyle已保证。不得改geometry或吞消息/阻断Godot正常缓存同步。
- 必须区分entry/downstream_after/guard_after三个原始phase+同token/depth，guard_after是实际交回USER32的proposal，原评估器对应最终proposal核geometry/z；完整保留downstream错误proposal作为已拦截证据，不能伪称不存在。CHANGED/可见native state始终原样，任何已可见guardbelow/错误geometry/activation还是FAIL。若nested已提交错误不能外层后来修正抵销。无效guard/错线程/overflow等fail-closed且不能再show，结束按原精确remove/depth0/自然卸载。
- pure tests覆盖off零改、invalidref/nonce/PID/TID拒绝、nested每层apply、NOZORDER清除/只改anchor、NOACTIVATE/NOOWNERZORDER原值保留、downstreambad但guarded交回正确、nestedbad已可见仍FAIL、缺guardphase/错误配对/卸载后不修改、真实JSONL新schema与formatter往返。原96geometry/34policy语义保留；未受guard的原407badtrace仍FAIL，不回写旧证据。
- 首段只实现build/纯无窗/真实PS5参数静态，提交短回执后主管审再授权≤2runtime。不能在运行中无穷调整guard；同确定性断点两次必须另诊断。成功只解锁独立产品修订，完整真栈/398等不混入。

## 主管交接与用户裁定（2026-11-17 晚，GLM 接任）

- 用户三项裁定（本会话，落档 [decisions/2026-11-17-window-z-path-and-seam-ruling.md](../../decisions/2026-11-17-window-z-path-and-seam-ruling.md)）：①按本卡原计划继续；②产品路径允许显示后立即一次z纠正；③Godot进程内显式加载原生缝可进产品。本卡取证契约不变（三阶段原始记录/不以后纠正抵销首显反例/fail-closed 门冻结），②③只作用于后续产品修订卡。
- 主管自 Codex `/root` 移交 GLM 主管决策流；执行=L2 子代理（GLM 派发）。worktree/branch/文件域不变：主 `C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW`（branch codex/win32-restore-forensics-403，HEAD 1aec3670，另有未提交 vitshell_smoke.ps1 +12/-7）；前端 `C:/Users/timoz/.codex/worktrees/shell-window-transition-406/vit-daw-frontend`（branch codex/window-transition-406，HEAD ae86c7d4，另有未提交三文件 diff：probe.cpp +75 / observer.cpp +44 / observer.h +22）——均为前任中断时在飞改动，按 §12 权威输入保留，不得丢弃/回退/stash；回执须区分前任在飞 diff 与本轮新增。
- 首段（无窗：完成实现+双仓 build+纯测试+PS5 静态）随本交接放行；短回执后主管审，再单独授权 ≤2 runtime。栈占用/释放登记照旧。

## 首段验收 PASS（2026-11-17 21:40 +0800，GLM 主管亲证）

- 提交核对一致：前端 `codex/window-transition-406` e3e6374（三文件 +127/-14，前任在飞 diff 逐字节保留、本轮源码零改）；主仓 `codex/win32-restore-forensics-403` 24a84b7d（vitshell_smoke.ps1 +12/-7 + 27 个验证工件）。按先例（405/406/407 同型）段级实现留执行分支，卡级收口时再议合入 main；主管 cherry-pick 尝试因 main 无前置段实现已中止，工作树无损。
- 主管亲跑复核（全部无窗、亲证退出码）：geometry selftest exit0（126 例 0 失败，windows_created=0，96 原语义全数保留+30 新增）；identity selftest exit0（23/0，window_operations=0）；observer policy exit0（34/0，windows_created=0）；PS5 静态门 exit0（ParseFile 0 错误、lease helper 10/10、选项门 10/10、origin/main 实际 lease 对 408 正确拒绝=未发明运行时权限）。
- 契约冲突=无。执行侧发现并补齐的唯一缺口（前任 schema 审计跑在 formatter 定稿前）已入 REPORT；前任 pending 2 失败→修复的中间证据按诚实留痕保留。
- 未证明边界（维持）：真实 Godot 首 ordinary 转换下 guard 激活时序/嵌套深度/reference 存活、真栈 arm 门、408 开关下 engine0/2s/自然卸载。runtime（≤2 次）待主管单独授权后执行，栈登记照旧。

## runtime_01/02 授权（2026-11-17 21:58 +0800，GLM 主管；栈预留见 PC-RUNTIME-STACK.md 同刻条目）

- 执行=GLM L2 子代理（用户裁定今晚本会话闭环）。**只跑已审代码**：前端 e3e6374（构建产物）、主仓 24a84b7d（ps1）；运行中零代码改动；任一异常即停，无补跑、无运行中调 guard。
- 两腿各≤1 个全新 Godot child、≤15 分钟：`runtime_01`=synthetic 守卫前台；`runtime_02`=自然 external 前台（前提缺失记 not_exercised，不强造）。每腿先落 `coord/runs/TARGET-Z-TRANSACTION-1/runtime_0X/RUNBOOK.md` 再执行；ps1 显式 408 门全五开关+`-ForensicsCard 408`。
- 通过门（每腿，全部原样核、不降判据）：①安全 startup/ready 前提（safe iconic、child 从未 FG）；②恢复候选一次执行；③首次 ordinary 转换三 phase 完整：entry/downstream_after/guard_after 同 token/depth 配对，guard_after=实际交回 USER32 的最终 proposal，z=已核 reference（guard above child）、SWP_NOZORDER 按需清除、NOACTIVATE/NOOWNERZORDER/geometry 原值；④已可见错误 proposal 保留为拦截证据，CHANGED/native 末态原样，任何已可见 guardbelow/错 geometry/activation 仍 FAIL；⑤geometry 16DIP/engine mode0 不变、零 FG/focus/TOPMOST、client/mode 一致≥2s；⑥nonce stop 请求→应用自卸载闭环（removed_verified/shutdown/engine_ack）→child 自然 exit 0；⑦收尾全进程/端口归零、recents 前后哈希一致。
- 失败分类照 §8（probe_defect/环境/机制失败分开）；任一腿 FAIL/coverage 缺口→停止，不跑下一腿，保全原始工件。装置腿 lease 门若因 owner 字符串不匹配拒绝：按资源记录 21:58 条目的 owner 原串传参对齐，禁止改门。
- 工件：raw JSONL+decoder 摘要+REPORT 入 run 目录，两树独立 commit（不 push）。token 经济纪律（PROTOCOL §0.3）：原始日志只走 decoder/计数摘要进上下文，不整读 raw。
