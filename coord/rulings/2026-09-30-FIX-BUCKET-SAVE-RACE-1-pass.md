# Ruling：FIX-BUCKET-SAVE-RACE-1 — pass（2026-09-30 决策会话）

- 实现：2e71cbf3@port/fix-bucket-save-race-1 → 合并 main 1bf0025e（cherry-pick）
- 亲核项：
  1. diff 对应：3 文件 +280/−15（App.tsx +28/bucketSaveRace.test.ts 139/smoke 脚本 +128），与回执一致。
  2. **修复 hunk 亲核**：restore 效应补 `messages` 依赖（与 save 同提交重跑）+声明在前先执行+布防既有 `skipNextStoredMessageSaveRef` 令 save 让位+`mergeRestoredChatMessages` 合流核抽出——与回执方案一句话逐点对应；`restoredMessageScopeRef` 每桶只真读一次的稳态约束保留。
  3. **我复跑**：npm run test 于合并态 main——**375/375 全绿**（373→375，新增 2 用例）。
  4. E2E 红绿双 run 工件亲读：绿 run 20260930_195338 verdict=**pass**（F1 PASS，samples 10/10 首采样 2.5s 即回——同提交链恢复）；**红证 run 20260930_195015 verdict=fail 且唯一红组=F1**（samples 0/10、桶 5→3 marker 丢，与 190233/190740 取证同形）——断言非恒真，红绿纪律成立。
- 边界裁定：
  - ②smoke 脚本扩域（+128）：**采信合规**——E2E 加组是卡面目标 3 且验收门槛"E2E-WEBUI-1 exit 0"内生所需（K1/K2/O1 同款先例链）。
  - ①单测层无法复刻 effect 时序（测试栈无 jsdom）→ 时序覆盖走 E2E：方案合理，采信。
  - ③跨桶 orphan 形态（末轮落 unsaved_root 桶+刷新后 scope 直迁不迁移旧桶）：**记观察项不开卡**——与本案不同机制、未实证发生面；若 M8/后续手测出现"刷新丢末轮"再取证开卡。
- 遗留：无阻塞项；观察问答折叠形态手测复验仍留用户在场（OPT 卡条款）。
