# UI-OS-1 PROGRESS（断点续跑）

- 卡：coord/cards/doing/2026-11-14-UI-OS-1.md（池序 349，P2）
- owner：GLM-5.3-Flash 执行子代理（freeflash），PC，2026-11-14
- 基线：主仓 origin/main=e8549446（领取时）→ 协调重推期间远端前进（345/346 领取与收口，均与本卡域不相交）；前端仓 caaef19
- 状态：**DONE（执行侧自验）**——真栈 run_20261114_193656 exit 0，终态待主管验收合入

## 步骤日志

- S1 [done] 读 PROTOCOL/卡面/UI-HOST-1 RUN-SUMMARY/agent_chat_window.gd/vit_webview_host.h；核对双仓基线一致；doing 三卡（ARRLOOP-AUTHBRIDGE/CLOSURE-ROUTE-ADAPT/WAVESHELL-FORENSICS）域=agent Go，与本卡前端域不相交。
- S2 [done] 双 worktree 建立：主仓 D:/Vit_DAW_wt_uios1 @ port/ui-os-1（e8549446）；前端 D:/Godot/vit-daw-frontend_wt_uios1 @ port/ui-os-1（caaef19）。
- S3 [done] 领取提交：首次推被拒（远端 main 前进=345 并行领取，不同文件）→ 协调分支 coord/349-claim cherry-pick 重推成功。领取提交=3ebca93b。远端卡面 owner 已核对=正式领取成立。
- S4 [done] 勘察+设计（依据 SPEC-V5 §8.1/§8.7/§8.8 + R0 勘察报告）：WM_CLOSE(detached) 真关、topmost 仅打开态、Shell_NotifyIconW 通知桥（前台门控/三态映射/点击召回/瞬时 tray）、teardown+重建语义、零 webui 改动（触发链留 348 桥域）。
- S5 [done] 前端实现三提交：1e0713a（extension OS 语义+通知桥+debug_state 扩展）/ 9dab9fa（gd 生命周期+notify 桥+烟测钩子）/ 128477b（dock can_present_window 重建入口+start_page lifecycle 兜底）。junction D:/Godot/vit-daw-frontend_wt_uios1/godot-cpp → D:/Vit_DAW/godot-cpp（主检出 frontend/godot-cpp 为空目录，真实依赖在主仓 git-ignore 目录）。scons api_version=4.6 双变体编译过；--import 无解析错。
- S6 [done] ui_os_smoke.ps1（343 同模式新脚本）+ RUNBOOK（§8 预案）落盘。
- S7 [done] 真栈占用：登记 203c33e9 → ≤60s 复核（远端记录重读+进程=0/五端口=0）→ 启动。
- S8 [done] 烟测三轮：run_193333 exit 1（脚本缺陷：child_hwnd 取行错位+JSON 键序）；run_193526 exit 0 但 recreated_navigate_count 断言未 gate 且预期与自身设计矛盾（校准为 ==1+补门禁）；**run_193656 exit 0 全 PASS（71 断言，含 NIN_BALLOONSHOW OS 上屏实证；fixture 哈希前后一致零污染）**。三轮前端实现零改动（装置面修复）。
- S9 [done] 拆栈核净（进程=0/端口=0）+ 释放记录推送；RUN-SUMMARY 入库。
- S10 [done] 卡面回执回填 → git mv doing→done → coord-only 推送；双仓 port/ui-os-1 推远端留待主管验收。

## 遗留边界（交接主管/348）

- OS toast 实点召回（NIN_BALLOONUSERCLICK 路径）留人工目检；脚本以同一生产召回路径合成驱动。
- 通知动作按钮：Win32 balloon 无动作按钮，动作以 ≤2 条元数据契约传递（vit_os_notify.actions/vit_notify_activated）；WinRT toast 需 AppUserModelID=另卡裁定。
- 通知触发链（agent 事件→vit_os_notify）与页内定位消费=348 桥域；本卡零 agent 语义/零 webui 改动。
- S7 三音合成（完成/确认/出错提示音）=触发域后续；本卡通知音=OS 默认系统音（真实通知才响，非原型增益）。
- DPI：脚本记录 chat_window_dpi=168（系统 175%）；125%/150% 视觉缩放=人工目检项。
