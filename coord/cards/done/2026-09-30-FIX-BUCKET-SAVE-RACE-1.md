# FIX-BUCKET-SAVE-RACE-1：webui 会话桶 save/restore 时序缺口——仅存本地桶的末轮消息刷新后结构性丢失

- 池序 8；来源=OPT-OBSERVE-OUTPUT-1 上交①（裁定=[rulings/2026-09-30-OPT-OBSERVE-OUTPUT-1-pass.md](../../rulings/2026-09-30-OPT-OBSERVE-OUTPUT-1-pass.md)）
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无；webui 域（与 OPT/VITNOTE-IMPL-5 同域串行）
- 模型分级：L1 / flash 可接
- 已核实事实（执行侧取证，决策侧已稳定化证据工件）：
  - 现象：reload 时 scope 物化拍的 initial **同步 setMessages（图消息）触发 save 效应覆写本地消息桶**，而 restore 效果因 scopedConversationRef 同 commit 内晚置、最早须等下一轮询拍（~8s）才首跑——两者时序颠倒。
  - 后果：**仅存本地桶的驱动消息（未及服务端 checkpoint 的末轮）刷新后结构性丢失**；真水合/服务端图路径不受影响。
  - 证据：coord/runs/OPT-OBSERVE-OUTPUT-1/evidence/（mini_repro.mjs——无图消息时 restore 正常 ~10s 合并回屏，反证时序归因；run-190233/190740 dom-observe-reload-samples.json 两轮采样）。
- 目标：
  1. 修复时序：restore 在同 scope 首次 save 生效前完成，或 save 对"restore 未完成的桶"跳过/合并而非整桶覆写（实现面领取后实锚 App.tsx 水合/save 效应段，选破坏最小方案并申报）。
  2. 回归用例：复刻 mini_repro 场景——有图消息+仅存本地桶的末轮驱动消息→刷新后该消息仍在（单测或组件测层面）。
  3. E2E 加组（可选，若单测已覆盖充分则回执申报理由）：reload 保消息断言。
- 文件域：agent/webui/src/（App.tsx 水合/save 效应段+涉及测试）；不动 agent Go 侧。
- 验收标准：npm run test 全绿含新回归用例+build 过+（若加组）E2E-WEBUI-1 exit 0。
- 停止条件：取证发现既有桶覆写时序另有功能依赖（修复会破坏其他路径）→ 实证上交定方案。
- 领取：2026-09-30 21:05 / c4a10d54398ed3b12a7596d0e68d58892116c824 / port/fix-bucket-save-race-1（worktree D:/Vit_DAW_worktrees/fix-bucket-save-race-1）
- 回执：
  - commit：port/fix-bucket-save-race-1 @ 2e71cbf3（3 文件：App.tsx + bucketSaveRace.test.ts + webui_rendered_dom_smoke.mjs，+280/-15；worktree D:/Vit_DAW_worktrees/fix-bucket-save-race-1 保留复验）
  - 修复方案一句话：restore 效应依赖补 `messages`——与 save 效应同一提交重跑且因声明在前先执行：先读桶并布防既有 `skipNextStoredMessageSaveRef` 令 save 让位，合并流再下一提交落盘；`restoredMessageScopeRef` 每桶键只真读一次不变，稳态零变化（空桶新会话无落盘延迟——restore 同提交发现空桶即放行 save）。合流核抽出 `mergeRestoredChatMessages` 供单测驱动。
  - 回归用例：agent/webui/src/bucketSaveRace.test.ts（2 用例：①覆写丢末轮 premise 复现 + 修复管线保末轮——图消息 initial 换流 + 仅存本地桶末轮，先演示 naive save 整桶覆写即丢、再走修复管线末轮回流且桶保全；②裸启动桶起流分支 = mini_repro 无图反证形态）。
  - E2E 加组：bucket-save-race-F1（context F，真栈 reload 保消息：真 /agent/ui/state 图水合 + 驱动末轮仅落桶 + reload 保 URL 会话 id）。绿：run 20260930_195338 exit 0 / verdict=pass / F1 PASS——samples 10/10 回屏且首个采样（2.5s）即回（同提交链恢复，无需等 8s 轮询拍），桶刷新前后均 n=5 含 marker。红证：run 20260930_195015（-SkipBuild + 回退 deps 的 dist + 上轮二进制 sha256=6846843A…）唯一红组 F1——samples 0/10、桶 5→3 且 marker 丢（结构性丢失复现，与 190233/190740 取证同形）→ 断言非恒真。工件：worktree artifacts/e2e_webui1/{20260930_195338,20260930_195015}/（dom-verify-bucket-save-race-f.json + webui_rendered_dom_report.json）。
  - 验收命令：npm run test 375/375（原 373 + 新 2）；npm run build 过；E2E-WEBUI-1 exit 0（run 20260930_195338）。
  - 端测边界声明：① 单测层无法复刻 effect 时序（测试栈无 jsdom/组件渲染，tsx 测试仅 renderToStaticMarkup 不跑效应）——时序覆盖按卡目标 3 走 E2E 加组，理由如上；② scripts/webui_rendered_dom_smoke.mjs 在卡文件域 agent/webui/src/ 之外，系卡目标 3「E2E 加组」选项与 AGENTS §5 渲染面端侧烟测门槛所需（复用现有脚本体系加场景），如判越域可 rework；③ 已知未修边界：跨桶 orphan 形态（刷新前 scope 未物化、末轮落 unsaved_root 桶；刷新后 scope 直接物化为工程 scope——initial 分支不迁移旧桶，末轮滞留孤儿桶）与本卡同 scope 时序是两个机制，未动、未实证其发生面；④ E2E 采样窗 25s（10×2.5s），真栈真浏览器。
- 验收：**pass（2026-09-30 决策会话）**——裁定=[rulings/2026-09-30-FIX-BUCKET-SAVE-RACE-1-pass.md](../../rulings/2026-09-30-FIX-BUCKET-SAVE-RACE-1-pass.md)；合并 commit 1bf0025e（cherry-pick）；复跑 375/375+E2E 红绿双 run 工件亲读（绿 pass/红证唯一红组=F1，断言非恒真）+修复 hunk 亲核；跨桶 orphan 形态记观察项不开卡。
