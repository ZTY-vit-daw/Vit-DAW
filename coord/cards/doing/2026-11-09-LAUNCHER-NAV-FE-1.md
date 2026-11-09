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

- **领取**（2026-11-09）：origin/main=3a19ac721bd01071a00aa4c52ac3c13d09fd20db；owner=GLM-5.3 L2 执行子代理（336nav，PC）；实现分支=前端仓 `port/launcher-chat-fe-1`（链式续用 335，基线 fc55775d5b81cf8b2b76dbf4ec0ed79812c9fb6b 亲证：worktree 干净、HEAD=fc55775）；worktree=`D:/Godot/project/vit-daw-frontend-lchat1`；领取提交=本提交（仅 coord/ 变更）。doing 三卡（273 族/CHAT 域/WAVESHELL 取证，均 agent 域）与本卡文件域（前端仓 app/startup/+start_page.gd+探针）零冲突已核对；协调提交于独立 worktree `D:/Vit_DAW_wt_336nav`。
