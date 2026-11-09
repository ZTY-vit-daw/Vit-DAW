# LAUNCHER-NAV-FE-1：directive 消费+open 链+治理确认（332 拆卡 C2，Godot 域）

- 发卡：GLM 主管决策流 / 2026-11-09；派发确认：已确认；验收负责人：GLM 主管决策流+用户 GUI 终审
- 池序 336；P1 / 1.5 天 / 依赖=335（C1）；L2 / PC（Godot 前端仓）；零栈+真栈腿并入 339

## 目标

1. 窗消费 `launcher_directive` 回执（schema=332 设计稿 §4.1：action/file_path/project_name/reason/requires_governance）→ 复用 start_page 既有 open/new 链（含加密 .vit/采样率跟随，抽公共函数 ~20 行重构行为不变）；
2. 治理确认对话框（332 §3.3 保守版：有在开工程必确认「保存并切换/直接切换/取消」，保存遇 require_path 转另存为）；
3. 结果卡点选开工程（确定性退路，零 LLM 依赖）；
4. 进入工程后自动带出现有浮动对话窗（show 既有 LLM_Chat_Window 一次，~5 行，非新逻辑——案 B 承接形态）。

## 文件域

前端仓 `app/startup/`（C1 组件扩展+确认对话框新文件）+`start_page.gd` open/new 链抽函数；红线零触碰聊天控制器内部。

## 验收标准

契约解析表驱动（headless）+真栈腿并入 339（directive→治理确认→进 dock→浮动窗带出全链）。

## 领取/回执/验收

- **领取**（2026-11-09）：origin/main=3a19ac721bd01071a00aa4c52ac3c13d09fd20db；owner=GLM-5.3 L2 执行子代理（336nav，PC）；实现分支=前端仓 `port/launcher-chat-fe-1`（链式续用 335，基线 fc55775d5b81cf8b2b76dbf4ec0ed79812c9fb6b 亲证：worktree 干净、HEAD=fc55775）；worktree=`D:/Godot/project/vit-daw-frontend-lchat1`；领取提交=cec2acb1（仅 coord/ 变更，远端 owner 亲证后开工）。doing 三卡（273 族/CHAT 域/WAVESHELL 取证，均 agent 域）与本卡文件域（前端仓 app/startup/+start_page.gd+探针）零冲突已核对；协调提交于独立 worktree `D:/Vit_DAW_wt_336nav`。
- **回执**（2026-11-09，执行侧自验完成，done 候主管验收）：
  - **实现 commit**：前端仓 `port/launcher-chat-fe-1` @ff301b3（已推远端，7 文件 +828/-24）：新增 `app/startup/launcher_nav_bridge.gd`（64 行：§4.1 校验纯函数+跨场景一次性带出浮动窗静态桥）、`app/startup/launcher_governance_dialog.gd`（65 行：三键治理确认 AcceptDialog）；修改 `launcher_chat_panel.gd`（+106：directive_received 信号/render_chat_response 337 适配面/apply_greeting 原位换+追加两分支/结果卡内嵌 directive 点选同链/file_path 键兼容/requires_governance 呈现）、`start_page.gd`（+311：消费链全量——send→POST /agent/chat(context.launcher=true)、greeting 真端点 fetch+降级、directive→治理自查(/agent/state current_project)→三键确认(save 遇 require_path 转另存为 F14)→open_project_by_path 公共链抽出(行为不变)→进工程 LauncherNavBridge 带出浮动窗一次、_http_post_json 辅助）、探针 `launcher_chat_panel_probe.gd`（38→71 检）。
  - **红线自检（亲证）**：`git diff HEAD -- app/shell/LLM_Chat_Controller.gd app/shell/LLM_Chat_Window.tscn` 输出为空；全量 diff 文件清单=上述 7 文件，无一越域。LLM_Chat_Window 仅经 legacy 回退 `Window.show()` 调用（红线允许面），控制器内部零触碰。
  - **headless 探针**（`godot --headless --path . --script tools/diagnostics/launcher_chat_panel_probe.gd`，Godot 4.6.1 console exe）：**71/71 检 ×3 轮全过，exit 0**。335 原 38 检零回退全过；新增 33 检覆盖：①装配面（directive_received/send_requested 经 callable 归属判定接到 start_page 生产实件）；②§4.1 校验表驱动 5 例（open ok/缺 file_path、new ok/缺 project_name、未知 action）；③治理三键全路径（态未知拒连→保守确认弹出+「直接切换」走 open 公共链 FakeClient 亲证 file_path 透传+失败错误框+标记/busy/in-flight 三清；态已知无工程→零摩擦直行不弹框；取消→零工程动作；保存并切换→save_project→require_path→另存为 FileDialog→save_as_project(用户路径)→续走 open 链；保存失败→放弃切换+专属错误框）；④337 真实回执消费（reply 文本+project_result_cards 并存渲染；kind=launcher_project 卡 file_path 键绑定；OPEN 点选→内嵌 directive 同链执行亲证）；⑤开场白真端点（拒连降级保留 mock 开场白+ok 载荷对话已开始时追加分支；独立面板原位替换分支+无重复）；⑥chat 端点失败降级提示（不裸错）+in-flight 释放；⑦浮动窗带出桥（隐藏起点双调用恰一次、已可见不再 toggle、legacy LLM_Chat_Window 直接 show、dock 公共面 toggle_chat_window）。
  - **回归**：`f5_startup_smoke` exit 0（start_page children 装配无回退）；`clip_fade_gain_gui_probe` exit 0（status ok/failures 空，start_page 入口链无回退）。
  - **与 337 契约对齐清单**：①directive schema §4.1 五字段原样消费（reason 呈现+requires_governance 治理预告行）；②回执文本字段=reply（335 冻结面 text 经 render_chat_response 的 reply/text 双取兼容）；③ProjectResultCards 卡形 {kind:launcher_project, type, id, name, file_path, launcher_directive} 内嵌 directive 点选同链（确定性退路，零 LLM）；④GET /agent/launcher/greeting 就绪即取（幂等一次）+失败保留本地模板；⑤/agent/state current_project{path,uuid} 治理自查双保险（agent 侧 requires_governance 恒 true，UI 仍自查不信任回执标记）。338 的 context_flow/volume_switch 面为工程内消费，本卡不涉。
  - **诚实边界/渲染面声明（AGENTS §5）**：①本卡零真栈（真栈腿并入 339）：directive→治理确认→进 dock→浮动窗带出的**真实全链未在真栈驱动**——探针以 FakeProjectClient（脚本化 IPC 回执）+死端口 HTTP（确定性拒连）驱动同构链路；②渲染观感（对话框布局/卡片视觉）未过像素级断言，headless 只覆盖节点树+数据绑定+状态机+信号接线，候 339 真栈腿+用户 GUI 终审；③探针注意项：本 worktree `.godot` 类缓存需含三个 Launcher* 类名（`--headless --import` 已刷新；缓存 gitignored，新检出首跑探针前须先 import——与 335 同前提）；④首轮探针两次无效运行（类缓存缺失致 start_page 解析失败→探针挂起+断言级联失败）已按环境中断处理并修复后跑正式三轮，非功能面失败；⑤另存为 FileDialog/治理对话框的人工交互路径（真实点击）未驱动，探针直调处理器等价覆盖。
  - **实现说明**：`_entering_from_launcher` 标记在 `_enter_main_scene` 顶部取后清（成功/失败不重放）+`_finish_ipc_failure` 兜底清（IPC 失败防后续手动 open 误带出）；面板 render_reply 335 冻结面签名不变（mock 驱动路径兼容），新增 directive_received 发射不影响既有三形态断言。

### 终验（主管，2026-11-10 凌晨）

- **pass（零栈段）**——主管亲测：探针 71/71 PASS exit 0（335 原 38 检零回退+新增 33 检全绿）；红线零 diff 亲证（LLM_Chat_Window 仅 legacy show 允许面）；337 对齐五点清单核验；治理确认保守版与 F14 require_path→另存为语义合规。前端分支已合并入前端 main（fast-forward ff301b3，用户手测入口即刻可见，双轨保险=磁带流抽屉）。真栈腿候 339。
