# UI-LAUNCHER-1 执行进度（freeflash 子代理）

- 分支：port/ui-launcher-1（基线 origin/main = e854944621f6b9ec420bfd1cb9c2f71df64ede15）
- worktree：D:/Vit_DAW_wt_uilaunch1
- 领取复核（2026-11-14）：卡在 todo/ 已确认；doing 三卡（ARRLOOP-AUTHBRIDGE-1 / CLOSURE-ROUTE-ADAPT-1 / WAVESHELL-LOADING-FORENSICS-1）均在 agent Go 域，与本卡 webui 域不相交；UI-OS-1（349）仍在 todo 未领取；344 基座（38798da7+fb50912f）已在 e8549446 历史中，12 组件/tokens/sprite/gallery 均在。
- 下一步：卡面回填领取段 → git mv todo→doing → add 新路径 → coord-only 提交推 HEAD:main。

## 里程碑日志

- [x] M0 领取提交（1ea83f48 已推 main，e8549446..1ea83f48 fast-forward）
- [x] M0.5 勘察完成（2026-11-14）：
  - 后端契约（agent/internal/chat/launcher_flow.go + server.go）：GET /agent/launcher/greeting → {status,launcher,reply,recent_project?:{name,file_path},recent_count}（recents 失败时 recent_count=0+降级模板，HTTP 200）；POST /agent/chat 请求体键 `context`，launcher 键= `context.launcher=true`；回执字段 launcher_directive{action,file_path,project_name,reason,requires_governance 恒 true} + project_result_cards[{kind:launcher_project,type:project_open_candidate,id,name,file_path,launcher_directive}] + context_flow("launcher"/"project")；list 意图词面="有哪些工程"等（确定性零 LLM）；open/new 由 UI 不执行。
  - 前端契约：src/lib/api.ts 的 apiPath() 可复用（不改该文件）；ChatRequest.context 存在于 types.ts（不改 types.ts，launcher 类型自持）；createConversationID 模式=webui_<ts36>。
  - 344 基座：components.css 全部规则作用域 .v5-design 下 → launcher 根节点须挂 .v5-design 类；tokens/sprite/Button/TextField/ProjectBadge 可复用；gallery fixture 标注模式与 render-smoke 断言/截图/对比度/裁切/外部网络禁令全读毕。
  - 设计决定：surface 分发走 App.tsx 包一层（function App→MainApp + 新 wrapper App，零 hooks 成本）；drawer 完整 recents 列表经 launcher chat list 意图（"有哪些工程"）获取——greeting 只给首项+计数，无独立全量 recents HTTP 端点；fixture 接缝=LauncherSurface props（gallery/smoke 不出网）。
  - 文件域解释：卡面验收强制「launcher 渲染面进 test:render 底座」→ src/gallery/main.tsx + render-smoke.mjs 的扩展属验收要求本身，按最小扩展执行并计入回执边界声明；不改 src/project/**、styles.css、types.ts、src/lib/api.ts、344 基座文件（components/design/tokens 只 import 不改）。
- [ ] M1 surface 骨架
- [ ] M2 greeting 消费
- [ ] M3 结果卡渲染
- [ ] M4 渲染烟测扩展
- [ ] M5 回执+done 移动

## 长命令记录（重派时判断是否需重跑）

- npm install（agent/webui）：已完成，exit 0（2026-11-14，playwright 1.65.0 钉版未动）
- （待记：npm run test / npm run build / npm run test:render）
