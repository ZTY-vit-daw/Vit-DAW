# LAUNCHER-ZEROFRICTION-1：启动面首开零摩擦判据修复（取证+方案先行）

- 发卡：GLM 主管决策流 / 2026-11-14 晚窗（依据=[UI-BRIDGE-1 终验转裁定①](../../rulings/2026-11-14-UI-BRIDGE-1-pass.md)——真栈推翻「无在开工程首开零摩擦」）
- 派发确认：已确认（主管发卡）；**取证腿先行（只读零栈），实施腿候方案裁定且排 271 收口后**（agent chat 域重叠）
- 验收负责人：GLM 主管决策流
- 池序 352；P1 / 取证 0.5 天+实施 0.5–1 天 / 依赖=348 工件在库；PC

## 缺陷事实（348 真栈实证，工件在 coord/runs/UI-BRIDGE-1/）

React launcher 面加载即发 recents list 聊天（345 设计：完整列表经 list 意图确定性获取）→ agent 工作区激活 → hint 物化 boot scratch "Unsaved" 路径 → 337 保守治理判据（有 path 即三键）；独立探针（无聊天）path="" 零摩擦成立。**设计要求首开零摩擦，现状被 list 查询副作用破坏。**

## 目标

1. **取证腿（只读）**：锚定 hint 物化链（谁在 recents list 响应后写 boot scratch/激活工作区——file:line）；锚定 337 治理判据面（`launcher_flow.go` 判据与 340 修复的 UI 声明判据关系）；确认 list 意图的响应路径是否可区分 launcher-scope 只读查询与真实 open 意图。
2. **方案腿（候裁定）**：至少两案对比——a) agent 侧：launcher-scope 只读查询不物化工作区/hint（或判据放宽：boot scratch 不算在开工程）；b) webui 侧：recents 获取改走非聊天面（若有只读端点）或延迟首次 list 查询到用户交互；给出推荐+代价+验收门槛。**不自行实施。**

## 文件域

取证=只读（agent/internal/chat/launcher_flow.go 等+webui src/launcher/）；方案文档=`coord/runs/LAUNCHER-ZEROFRICTION-1/`；实施域候方案裁定再定。

## 验收标准

取证报告含三锚点链+两案矩阵+推荐；file:line 证据；无生产改动。

## 停止条件

发现判据属有意设计（在案裁定保护）→ 记录边界转裁定。

## 领取

- 时间 / origin/main hash / owner / 分支 / worktree / 领取提交：待回填

## 回执

- 报告链接 / 端测覆盖边界声明：待回填

## 验收

- 裁定文件 / 验收 commit：待回填
