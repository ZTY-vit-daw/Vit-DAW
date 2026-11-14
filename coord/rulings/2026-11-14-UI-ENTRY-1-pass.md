# UI-ENTRY-1 · 主管裁定：pass

- 日期：2026-11-14 深夜（手测 v3 返工当日闭环）
- 裁定人：GLM 主管决策流（ZCode 会话）
- 执行：freeflash 免费车道子代理（领取 7eb334b4 / done f79bf70f / run_20261114_233821 exit 0 三腿全绿）

## 证据核验（主管亲证）

1. **真栈单窗断言**：启动态可见顶层窗恰 1 个（React 启动台 hwnd=1444096）+Godot 主窗 IsWindowVisible=false → 打开工程 → 主窗恢复（maximized）+同 HWND 五行全同+navigate_count==1+surface.ack(project) React 真收；回退/legacy 腿过。栈占用 da674ce1→释放 37d5581d，主管独立复核端口/进程零 ✓。
2. **主管复跑三门**：563 测（560+3）/build/四渲染面 PASS。
3. **引擎限制处置接受**：Godot 4.6.1 主窗 set_visible(false) 降级为最小化（probe_20261114_232856 铁证）——cpp `set_parent_window_shown`（Win32 SW_HIDE/SW_SHOW 直达）为卡面预留域的正当落地；改动手术式（+23 行+stub 分支），sconsign 未入库 ✓。
4. **残余闪现**（引擎窗口创建先于一切运行时，deferred hide 前数帧 splash 可见）——**接受为已知边界**，列手测 v4 观测项，不阻塞放行。
5. 既有烟测脚本 `page_url=="/app/"` 断言过时注记接受（历史工件不追改）。

## 合入

- 前端 main ff→`ae24b52` 已推（含双变体 dll 重编）；主仓精选合入 webui entry+ui_entry_smoke.ps1+run 工件（再生成旧工件排除）；worktree 清理；port/ui-entry-1 留远端存档；主检出 webui dist 为手测 v4 重建。

## 放行

**手测 v4 放行**：单窗形态复验+残余闪现程度观测+任务栏交互（用户 Godot 直跑模式）。方案 B 语义闭环：启动=React 启动台独占（v5.1 视觉）、进工程=同窗切 project surface、失败回退 Godot 启动页、legacy 不变。
