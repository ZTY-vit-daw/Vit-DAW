# UI-OS-1 真栈烟测 RUNBOOK（AGENTS §8 预案，运行前落盘）

- run ID：`run_<yyyyMMdd_HHmmss>`（脚本自动生成于 coord/runs/UI-OS-1/）
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uios1/scripts/ui_os_smoke.ps1`
- 被测：主仓 worktree D:/Vit_DAW_wt_uios1 @ port/ui-os-1（1e0713a→128477b+脚本）；前端 worktree D:/Godot/vit-daw-frontend_wt_uios1 @ port/ui-os-1（1e0713a/9dab9fa/128477b + 双变体 extension 重编）
- 内核：只读复用主检出 VitApp.exe（零内核改动）；agent=现场构建隔离二进制（run 目录）

## 运行计划

- 运行次数：首轮 1 次；装置缺陷（脚本/环境面）修复轮不计入功能判定轮，每轮修复须有明确根因；同一功能断点连续失败 2 次 → 止损上交（§8）。
- 成功条件：脚本 exit 0；全部硬断言 true（B1–B7 + 343 不变量 A 面 + legacy 腿）。
- 软断言（记录不判败）：`n2_balloon_on_screen_os_confirmed`（NIN_BALLOONSHOW——系统勿扰/专注助手可能吞 toast 上屏事件；此时真实触发证据=Shell_NotifyIcon NIM_ADD/NIM_MODIFY 成功+os_event notification_shown）。

## 失败分类

- `infra_port_busy`：任一端口 7878/5555/5556/8787 已监听 → 即停上报，不静默复用（§2.2）。
- `infra_env_interrupt`：进程启动失败/网络中断/机器资源不足——有原始日志或退出证据方可排除出有效轮次。
- `fixture_defect`：worktree 缺 .godot 缓存/junction 断/extension 未加载（GDExtension class missing）→ 装置修复轮。
- `script_defect`：断言解析/时序/Win32 声明面缺陷（343 三轮先例）→ 装置修复轮。
- `assert_fail_*`：功能断言失败（gating/topmost/关闭语义/重建/HWND 一致性）→ 记录原始日志+失败类型，同代码版本同断点两连败 → 止损上交，不原样重跑。
- `manual_item`：OS toast 实点、DPI 视觉缩放、双屏行为——不在脚本断言面，留人工目检清单。

## 止损线

- 总轮次（含装置修复轮）≤4；功能断言失败轮 ≤2（连续同断点 2 败即停）。
- 止损只停止投入：未发生的成功不得改写为成功；环境中断须留原始日志归因。

## 占用纪律（§2.2）

- 占用记录推送成功 → ≤60s 内复核进程/端口 → 启动；finally 拆栈核净 → 推「空闲」+run ID。
