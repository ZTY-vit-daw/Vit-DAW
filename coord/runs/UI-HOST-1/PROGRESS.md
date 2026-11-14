# UI-HOST-1 执行进度（断点续跑协议）—— **终态：DONE**

- 卡：`coord/cards/done/2026-11-14-UI-HOST-1.md`（池序 343，P0；执行侧自验完成待主管验收）
- owner：freeflash（GLM-5.3 Flash）免费车道子代理 · PC · ZCode 主管会话派发
- 结果：**PASS run_20261114_184412 exit 0**（autoload 腿全断言 + legacy 腿）；详情 `RUN-SUMMARY.md`

## 已完成（全部）
- [x] S0 读纪律/卡面/R0 + fetch 核对 + doing 文件域核对 + worktree 建立
- [x] S1 领取提交 a52c1c07（→ main，owner 核对 ✓）
- [x] S2 前端仓 worktree（main @ ff301b3）
- [x] S3 勘察（结论见历史段）
- [x] S4 实现 5 commits（前端）+ 烟测脚本 1 commit（主仓）
- [x] S5 真栈占用 fee52451 → ≤60s 复核 → 4 轮 run（3 装置修复+1 PASS）→ 拆栈释放（记录已推 main）
- [x] S6 卡面回执 + doing→done（4fb5193e cherry-pick → main b81438ae）+ 两仓 port/ui-host-1 分支推送

## 分支/提交索引
- 主仓 port/ui-host-1（已推远端）：b58f42cd 烟测脚本 → 55852f94 占用登记 → d5932c5e 脚本修复+工件 → 4fb5193e done
- 前端仓 port/ui-host-1（已推远端）：bece950 autoload 骨架 → 719001a dock 委托 → 6d85041 start_page+cpp → caaef19 dll 重编 → e55e901 sconsign
- main 上的 coord 提交：a52c1c07（领取）→ fee52451（占用）→ b81438ae（done+释放）

## S5 真栈结果（2026-11-14）
- PASS run = run_20261114_184412，exit 0（autoload 腿全断言过 + legacy 腿过）
- 核心断言：child_hwnd=4523840 跨 StartPage→VitDockRoot 五证据行全同（同一 HWND）；navigate_count=1；URL http://127.0.0.1:7878/app/ 未变；Win32 枚举恰 1 窗 title=Vit；最小化/召回过；foreground_probe_is_chat=true；legacy 无 boot 行+进程存活
- R0 §4 真栈未知项取证：①autoload _ready 时 main_hwnd=2558138 非 0 ✓ ②同一 HWND/页面跨 change_scene_to_packed 存活 ✓ ③detached 顶层窗任务栏条目/前台在场 ✓（DPI/多屏未测，见 RUN-SUMMARY 边界）
- §8 历史：183806（缺 .godot 缓存→补 --import 预跑）/183958/184205（FindWindowExW 装置面→EnumWindows 改造）三失败轮均脚本装置缺陷修复轮，非功能重跑
- 拆栈亲核：四端口 free + 四类进程=0 + fixture 源哈希前后一致（F6CA36FA…）

## 断了几次
- 本会话未被外部重派中断；内部重试：领取/done 推送因远端并发前进被拒共 3 次（均按 §2.1.3 cherry-pick 重推成功）；烟测脚本装置缺陷修复 3 轮（RUN-SUMMARY §8）

## 历史勘察结论（S3，保留供追溯）
- VitAgentShell autoload（CanvasLayer 胶囊）与本卡原生窗 autoload 职责不冲突
- host 创建链：dock `_ensure_agent_webui_host`→`create_window(detached)`；重载点=`_navigate_agent_webui`；销毁点=`_exit_tree`→`_close_agent_webui_host`
- cpp：detached owner_or_parent=nullptr 顶层窗；`pump_until(completed,10000)` 同步等待；parent=0 有 DisplayServer fallback
- 进入工程仅主动链（无被动监听）→ 烟测驱动钩子落 agent_chat_window.gd（VIT_SMOKE_AUTO_OPEN_PROJECT 双门控，生产零行为）
- 取证面：host.debug_state()（child_hwnd/current_url）+ `[AgentChatWindow]` 结构化日志行

## 关键决策记录
- 烟测腿用同模式新脚本 scripts/ui_host_smoke.ps1（避开 doing 卡 ARRLOOP-AUTHBRIDGE-1「烟测加参」潜在交集）
- 烟测驱动钩子落 agent_chat_window.gd（文件域内授权文件），双环境变量门控
- worktree 编译 extension 用 junction 借用主检出 godot-cpp（构建依赖未跟踪目录，objs 缓存共享写入已记录）

## 中断恢复指引（已终结）
卡面在 done/；最终验收归主管决策流。worktree D:/Vit_DAW_wt_uihost1 与 D:/Godot/vit-daw-frontend_wt_uihost1 保留待验收后由决策侧处置（§2.1 worktree 纪律）。
