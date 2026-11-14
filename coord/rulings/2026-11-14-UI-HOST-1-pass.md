# UI-HOST-1 · 主管裁定：pass

- 日期：2026-11-14 晚窗
- 裁定人：GLM 主管决策流（ZCode 会话）
- 执行：freeflash 免费车道子代理（领取 a52c1c07 / 回执 b81438ae / run_20261114_184412 exit 0）

## 证据核验（主管亲证）

1. **真栈烟测 exit 0**：run_20261114_184412 报告全 true——同一 HWND=4523840 跨 StartPage→VitDockRoot 五证据行全同、navigate_count=1、URL 未变；VitWebViewHostChildWindow 顶层窗恰 1 个、title="Vit"、最小化/召回、foreground_probe_is_chat=true；legacy 腿进程存活+零 boot 行。
2. **栈纪律**：占用 fee52451（19:12，≤60s 复核在卷）→ 释放 b81438ae（18:46 亲拆+四端口/四类进程零）；主管验收前独立复核 netstat/tasklist 全零 ✓；fixture 912.vit 源哈希前后一致。
3. **文件域合规（diff 亲审）**：project.godot +1 行 autoload；start_page.gd +21 行纯隐藏/fallback 钩子（window_unavailable_changed 恢复链在卷）；vit_dock_root.gd +78 行委托（autoload 活跃时只显隐/聚焦不建窗）；cpp +53 行必要项（focus 前台/set_visible 恢复/icon+pragma shell32）；烟测钩子双门控（VIT_AGENT_WINDOW_MODE=autoload ∧ VIT_SMOKE_AUTO_OPEN_PROJECT）生产零行为。
4. **R0 §4 未知项**：①main HWND 可用 ②跨场景同窗存活 ③任务栏/前台——真栈取证达成；125%/150% DPI 与多屏明确列为未覆盖（边界声明诚实）。
5. flag 默认关闭（legacy 启动页为现行为）——生产面零变化，回归风险受控。

## 合入处置

- **前端仓**：main ff 至 `caaef19`（10 文件 +499 行）已推 origin/main。**排除 `e55e901`**（.sconsign.dblite 构建-state 膨胀 15MB，§12 构建产物不混入）；`.sconsign.dblite` 本身被跟踪属历史遗留，登记卫生项候后续裁定（untrack 或 ignore）。
- **主仓**：精选合入 `scripts/ui_host_smoke.ps1` + `coord/runs/UI-HOST-1/`（RUN-SUMMARY/PROGRESS/四 run 日志与报告）；**两份 27841 行插件清单 JSON 留远端 port/ui-host-1 作证据链不进 main**（精简件原则）。
- worktree 复验期结束：`D:/Vit_DAW_wt_uihost1`、`D:/Godot/vit-daw-frontend_wt_uihost1` 由主管清理；两仓 port/ui-host-1 分支留远端存档。

## 解锁与排程（主管排序）

- **349 UI-OS-1 即刻可派**（依赖 343 ✓，栈空闲）；348 仍等 345。
- **341 维持 held**：与 349 文件域重叠（start_page.gd/vit_dock_root.gd）+ v5.1 React launcher（345+348）落地后价值面收缩；345+348 合入后再裁（降级 legacy fallback 最小修缮或撤销）。
- **326 排 349 后**：栈串行（349 烟测占栈）+「±同族脚本」域与 349 烟测脚本面有交集风险，串行最稳。
- junction 借用主检出 godot-cpp（objs 共享写）记录在案：仅构建缓存面，无源码污染。

## 边界（不得超读）

- 本卡证明 autoload 承载结构成立与真栈断言链；**不证明** React 视觉质量（launcher surface 属 345）、DPI/多屏行为、渲染面浏览器级断言（E2E-WEBUI-1 层级）。
