# Ruling：FIX-AUDITION-TRAIL-1 pass（2026-09-29，决策侧）

- **裁定：pass**。实现合入 main（cherry-pick b192c2ab）；手测复现路径复验留待用户下次手测顺带（不阻塞：渲染面已有 E2E 真栈实证）。
- **决策侧核验（非转述）**：
  - **取证采信**：定性=渲染侧（webui 域内，未触停止条件）——证据链完整：内核 audition::Session 无 turn_id 字段（真栈 fixture seq27/28/30 实证）→ prepare 期遥测以未绑定身份落 lane → 族内每 eventType 各占一条逻辑消息（audition_events.go `audition:{sid}:{type}`）不折叠。**并发现 AUDITION-LANE-1（2026-09-14 同类手测）当时按回合域绑定排除的盲区**（漏无域遥测行）——本卡补族身份排除（isAuditionFamilyActivity），历史修复盲区闭环。
  - **diff 审**：5 文件 +451/−4 全在卡面域；turnGroups.ts 族身份排除+laneVisibleActivities 收敛口亲核（`audition:` 前缀判定+isUnboundActivity && !isAuditionFamilyActivity 过滤）；判定卡为族内唯一动态单表面（与 AUDITION-UNSTICK-1 既有卡面形态一致）。
  - **决策侧复跑**：npm run test **331/331 全绿**（我复跑，含新增 auditionTrail.test.ts 7 用例）。
  - **E2E-WEBUI-1 亲核**：工件 webui_rendered_dom_report.json 顶层 verdict=pass（含新增 audition-trail-T1 两阶段断言：处理中 lane 0 行+判定卡 preparing；判定后 lane 0+card settled；既有 A-H 全组在列无回归）；真 agent 进程+真构建 bundle+真浏览器（msedge 154）。
  - **红证边界披露采信**：修复前红态在单测层锚定（遥测行存在且无 turn 域、旧判据必落 lane），未做修复前 E2E 对照——诚实且充分。
- **端测边界声明采信**：渲染面=E2E 覆盖；用户旅程（Godot 手测复现）未覆盖——下次用户手测时顺带复验 M1 现象是否消失，异常再开卡。
