# UI-ENTRY-2 · 主管裁定：pass（含一项降级采纳）

- 日期：2026-11-15 凌晨
- 裁定人：GLM 主管决策流（ZCode 会话）
- 执行：freeflash 免费车道子代理（done 7b480b74+95bf0304 / run_20261115_003340 exit 0 四腿全绿 / 占用 98ed4ed7→释放 ee31d4d8）

## 证据核验（主管亲证）

1. **关窗即退出**：WM_CLOSE→os_event closed→`app.exit(chat_window_closed)`→Godot/agent/kernel 三进程退出+端口释放（pid 逐一实测）。
2. **失败链零 Godot 回退**：全程零 main_window.restore（日志断言）；WebView2 不可用=系统错误框+退出；agent 未就绪=窗内错误+有界重试轮（240×2.5s）。
3. **354 遗留竞速修复**：presented surface 早于 React 挂载致 surface.ack 丢失——pending surface+就绪宣告重投+5s 兜底（post_web_message 面，343 不变量不动）。
4. **webui 零 diff 亲核**（主仓分支 0 webui 文件）→563 测不回退引用执行侧回执+构建链；boot_splash 压制入 project.godot；legacy 零变化腿过。
5. 栈纪律全履行+主管独立复核端口/进程零 ✓。

## 降级采纳（M3 停止条件）

**内核无 close-project 命令**（CommandDispatcher 仅 new/open/save；project_client/dock 无回 launcher 路径）→ **关工程=退出应用**（M1 语义天然覆盖）。接受为当前终态语义；「工程关闭/切换回启动台」登记为内核卡候选（KERNEL-PROJECT-CLOSE-1，未发——多工程工作流的产品价值由用户裁定后开卡）。

## 合入

- 前端 main ff→`83390f5`（3f852f0 实现+83390f5 竞速修复）已推；主仓精选合入 ui_entry2_smoke.ps1+run 工件；worktree 清理；port/ui-entry-2 留远端存档；webui dist 无需重建（零 diff）。

## 放行

**手测 v5 放行**：关 React 窗=应用整体退出、失败无 Godot 页、splash 残余（纯色数帧）为观测项、工程内切换工程=退出重启（降级语义）。手测 v3→v5 三轮形态收敛至此闭环：启动台=唯一门面。
