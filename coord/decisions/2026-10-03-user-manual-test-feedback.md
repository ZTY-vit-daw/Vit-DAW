# 2026-10-03 用户手测反馈与裁定（合并手测场）

> 场次：coord/reports/2026-10-03-manual-test-checklist.md 四组执行；取证见 [runs/MANUAL-TEST-20261003/FORENSIC.md](../runs/MANUAL-TEST-20261003/FORENSIC.md)。用户裁定原话要点与决策侧定性如下。

## 七项反馈定性

| # | 反馈 | 定性 | 去向 |
|---|---|---|---|
| 1 | 拉伸角标在矩形外；应矩形内，且**四边=延展拉伸、四角=等比缩放** | 设计修订（交互语义重设计） | VITNOTE-CONTAINER-3 |
| 2 | 锚点（收起后小圆点）：视觉太简陋看不清；被新建轨道头遮盖（应在视窗下方固定层）；**可自由移动=逻辑错误**——锚点应拟物为**图钉**：固定便签、不可随意移动；note 激活展开后应能**显示该 note 的圈选作用域** | 设计修订+**推翻 2026-10-02 裁定 1 中"锚点拖动=移动位置"语义**（用户新裁定：图钉固定） | VITNOTE-CONTAINER-3 + 本文件裁定 |
| 3 | note 圈选新建的会话在 webui 显示"未命名对话流"——需默认命名协议（按 id 或规划位置），该对话流是针对该区域的特定流 | 会话语义缺口 | 并入 NOTESTREAM-2（卡面已修订） |
| 4 | webui 启动不自动新建**主对话流**——产品逻辑：note=子代理/副对话流，webui=主对话流，应自动新建并命名（或留给用户命名） | 产品语义设计（IA 面） | WEBUI-SESSION-SEMANTICS-1 |
| 5 | webui 初启布局比例全挤压（截图 09:30:12），一次对话后自行展开 | 渲染缺陷 | WEBUI-INIT-LAYOUT-1（截图在 runs/） |
| 6 | 输入框上方执行轨迹不随对话流切换：note 中输入 hi 后，webui 新建对话流工作时 hi 仍在轨迹上 | 渲染绑定缺陷 | WEBUI-SESSION-SEMANTICS-1 |
| 7 | A/B 判定后无新回应；二轮输入插队显示在首轮输出上方、不报错但无正确输出 | **P1 缺陷（三症状）**：agent 侧判定→结算链已实证全通（事件 seq 28/34/36/37+checkpoint 确认文案）；缺口=①judgment.settled 后无消息投递事件（结算确认未进对话流）②二轮 turn stop_reason=project_revision_stale（结算应用推高修订号后新 goal 撞陈旧守卫）③二轮消息插队（渲染序） | SETTLE-DELIVER-1 |

## 用户裁定落档

1. **锚点=图钉语义**（修订 2026-10-02 裁定 1）：收起态锚点不可自由拖动；图钉固定便签。**展开态需显示圈选作用域**（该 note 圈住的范围）。锚点视觉升级（当前小圆点看不清）。锚点层级：不被时间线内容（轨道头等）遮盖——布局在视窗下方固定层的方向由 CONTAINER-3 设计收口。
2. **容器拉伸语义**：角标收进矩形内；四边=延展拉伸、四角=等比缩放（等比的具体含义——等比缩放面板尺寸——由 CONTAINER-3 设计段确认）。
3. **会话命名协议方向**：note 会话按 note id 或圈选规划位置默认命名，用户可改；webui 主对话流自动新建+命名或留给用户（SESSION-SEMANTICS 卡内定稿）。
4. note/webui 主副会话语志：note 流=副（子代理），webui 流=主——产品形态定调（2026-10-02 webui IA 愿景的延伸）。

## conditional 卡处置

- JUDGMENT-SETTLE-STALL-1：**判定通道修复实证生效**（seq 28 user_judgment.requested 等）；终验判据"可见新回复"被 SETTLE-DELIVER-1 缺陷阻挡——维持 conditional，随 SETTLE-DELIVER-1 修复复验后转正。
- WEBUI-IA-REDESIGN-1：侧边栏/三区目检通过（用户好评）；初启布局挤压为 IA 面缺陷——维持 conditional，随 WEBUI-INIT-LAYOUT-1 修复复验后转正。
- VITNOTE-V2-IMPL-D / VITNOTE-CONTAINER-2：功能链走通（用户："形态基本完整，可拖拽可拉伸"）；交互设计修订走 CONTAINER-3——两卡维持 conditional，CONTAINER-3 完成后同场复验转正。
