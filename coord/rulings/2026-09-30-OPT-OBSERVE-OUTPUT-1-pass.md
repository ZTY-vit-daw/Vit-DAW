# Ruling：OPT-OBSERVE-OUTPUT-1 — pass（P1 实现段，2026-09-30 决策会话）

- 实现：7c5df281@port/opt-observe-output-1 → 合并 main 2bf331d7（cherry-pick）
- 亲核项：
  1. diff 对应：6 文件 +880/−2（observeOutputLayering.ts 122 行新文件+两测试+App.tsx 挂点 +27+styles+smoke 脚本 O1 组），零 Go 改动，与回执一致；smoke 脚本扩域=卡面"E2E-WEBUI-1 断言"内生（K1/K2 先例），采信。
  2. **我复跑**：npm run test 于合并态 main——**373/373 全绿**（354→373，新增 19 用例与回执清单一致）。
  3. E2E 工件亲读：run 20260930_192143 verdict=**pass**；O1a-O1d 四段 DOM 工件在位（collapsed/expanded/hydrated/rehydrated + 卡/回执不折叠三族）。
  4. 谓词实现亲核：observeOutputLayering.ts 与设计 §4.2/§5 逐条对应——600 字符/14 行双阈值、lead 段逻辑（≤4 行整段否则 3 行截断）、保守排除（回执/交互卡/pending/error 七族）、纯 UI 态不进 content 不进持久化。
  5. 水合等价断言（O1c）覆盖设计关键约束：刷新再水合默认态重算+持久化桶全文等长。
- 环境中断采信：run 20260930_192037 因前序取证临时 agent 残留占用 UDP 4444/4445 未就绪（§9 单机单实例实录第三案）——停止残留后复跑即过，按环境中断记录不归因功能。
- **上交①采信并开卡**：既有 save/restore 时序缺口（reload 时 initial 同步 setMessages 触发 save 覆写本地桶、restore 最早 8s 后首跑→仅存本地桶的末轮驱动消息刷新后结构性丢失）——P1 域外、本卡未动水合/存储代码（diff 亲核印证 App.tsx 仅 +27 渲染挂点）；证据工件已稳定化至 coord/runs/OPT-OBSERVE-OUTPUT-1/evidence/（mini_repro.mjs+两 run reload-samples）；**开卡 FIX-BUCKET-SAVE-RACE-1**。上交②（测试侧竞态两处）已在本卡 E2E 内修复，随本实现合并。上交③（E2E 扩域）如上采信。
- 遗留：手测复验（观察问答输出形态用户认可）留用户在场；P2（agent presentation 结构化真三层）按设计拆卡待排，GLM 首选。
