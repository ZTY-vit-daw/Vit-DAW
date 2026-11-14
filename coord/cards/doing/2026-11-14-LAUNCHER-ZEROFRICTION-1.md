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

- 时间 / origin/main hash / owner / 分支 / worktree / 领取提交：2026-11-14 晚窗 / 073b594f / freeflash（GLM-5.3 Flash）免费车道子代理 · PC · ZCode 主管会话派发 / port/launcher-zerofriction-1 / D:/Vit_DAW_wt_zerofric1 / 领取提交 a682b2e6
- 腿范围：仅取证腿（只读+coord/runs/LAUNCHER-ZEROFRICTION-1/ 方案文档）；实施腿候方案裁定，不领

## 回执

- 报告链接 / 端测覆盖边界声明：**取证腿完成**（2026-11-14 晚窗，owner=freeflash·PC）——报告 `coord/runs/LAUNCHER-ZEROFRICTION-1/REPORT.md`（本分支+coord-only 推送入库），回执/进度 `coord/runs/LAUNCHER-ZEROFRICTION-1/PROGRESS.md`。
  - 三锚点结论：①hint 物化链=launcher recents 聊天轮内对话节点落盘经 `Open("")` 物化 boot scratch（history.go:1020-1021/2509-2529，server.go:2680）→ chat 收尾/调度器激活绑定草稿（server.go:7743-7780）→ hint 发布进读侧（server.go:7798）→ /agent/state 回放（server.go:1065/1076）→ Godot 三键（start_page.gd:598-612）；348 attempt1 运行时日志逐跳佐证（activation pending→chat→activated Unsaved.vit 同秒）。
  - ②治理判据面：BE 门 340 已废 uuid 判据（UI 声明+user_entered_project 双门）；UI 治理面维持 path 保守判据=设计稿 §3.3 在案设计；boot scratch 算「在开工程」=判据输入被聊天副作用污染+草稿分类无在案定义。
  - ③可区分性：可区分数据全在卷（context.launcher/会话前缀/确定性意图分类）但现有顺序不生效；修复无需新协议字段。
  - 方案矩阵候裁定：**A1（agent 侧 launcher 卷门控：scope 内无绑定工作区聊天不激活/不落盘）=推荐主案**；A2 判据放宽（候裁定备选，触碰 §3.3 边界不推荐）；B1 greeting 扩展全量列表（可选补充，单独对打字路径不充分）；B2 延迟查询（单独不充分）。实施腿候裁定且排 271 收口后（chat 域重叠）。
- 端测覆盖边界声明：本腿=只读取证+方案文档，零生产改动、零真实栈、零测试运行（AGENTS §5 门槛不适用于本腿产物；实施腿验收门槛已在 REPORT 案 A/B 各列）。
- 状态：**卡留 doing，附「方案候裁定」——取证腿完成≠整卡完成**。

## 验收

- 裁定文件 / 验收 commit：待回填（候主管裁定：方案 A1 vs A2/B1 取舍；裁定后实施腿另领）

## 方案裁定（主管 2026-11-14 深夜）

- 取证腿验收：**pass**——三锚点主管亲证（server.go:2682 RecordConversationNodeForProject→history.Open→defaultDraftProjectPath 物化→server.go:7798 SetProjectPathHint→start_page.gd:598 治理判据，行号±2 为今晚提交漂移）；报告 `coord/runs/LAUNCHER-ZEROFRICTION-1/REPORT.md`（main @ 428a43bd）。
- **采 A1 为主案**：launcher scope（context.launcher/webui_launcher_ 会话前缀）内、无绑定工作区时——聊天不激活工作区、不落盘对话节点（根因面修复，list/打字/未来 launcher 面全免疫；与 §3.3 判据、340 双门、launcher 卷进程内存设计零冲突）。B1（greeting 扩全量列表）列为后续可选补充卡；A2（判据放宽）触碰 §3.3 在案边界——**不采**。
- 实施腿：本卡留在 doing，**排 271（CLOSURE-ROUTE-ADAPT-1）收口后另领**（域=agent/internal/chat 重叠）；实施验收门槛=348 烟测 ui_bridge_smoke 的零摩擦腿复跑（无聊天探针 path="" 语义在真栈 launcher 卷成立）+单测红转绿。
