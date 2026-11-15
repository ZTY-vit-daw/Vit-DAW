# PROGRESS — UI-MSG-SHADOWBOX-1（池序 362）

- 卡：coord/cards/todo/2026-11-15-UI-MSG-SHADOWBOX-1.md（P1，零栈浏览器渲染面）
- 工件目录：D:/Vit_DAW/coord/runs/UI-MSG-SHADOWBOX-1/
- 骤起执行会话：GLM-5.3-Flash 执行子代理（PC，Git Bash，worktree D:/Vit_DAW_wt_msgsb1）
- 步骤约定：每步完成后在此追加一行（步骤号/做了什么/产物路径/下一步），长命令先记入再执行。

## 步骤日志

- S0 | 检查断点：工件目录不存在→创建；读取任务卡+裁定档案+PROTOCOL §0/§2 完成 | 产物=本文件 | 下一步=S1 领取协议（fetch→worktree→回填领取节→推 main）
- S1 | 领取协议完成：worktree D:/Vit_DAW_wt_msgsb1 @ port/ui-msg-shadowbox-1（基线 origin/main 2b2e3ea3）；卡 todo→doing+领取节回填+POOL-INDEX 362 行 doing；协调提交 0504bb99 已推 main（仅 coord 变更 3 files） | 产物=coord/cards/doing/2026-11-15-UI-MSG-SHADOWBOX-1.md @ main | 下一步=S2 SPEC-V5 修订+样式实施
- S2 侦察结论（记录）：工程面=ProjectSurface v5 线程，样式锚点 project.css:136（App.tsx 不动）；令牌复用 --v5-ui-border（#D3D3D3）+ --v5-shadow-macro-card（tokens.css:74）；两面线程底=--v5-read-workspace #F0F1F2；SPEC 消息条款实际在 §8.1（裁定写的 §6 系字阶表）；render-smoke 断言挂点=launcher §7 ldom / project §8 pdDom 两 evaluate + 尾部截图模式（361 同款）
- S3 | SPEC-V5 修订完成：§8.1 消息条款改「右对齐+阴影框标识」+S5 标记态描述+修订依据引裁定档案（相对链接 4 级上溯已校正）；§11 已裁定补第 6 条 | 产物=agent/webui/design/main-window-v5/SPEC-V5.md | 下一步=S4 已完成（launcher.css）→S5 已完成（project.css）→S6 已完成（render-smoke 断言）
- S4/S5/S6 | 实施完成：launcher.css .v5-launcher-msg--user 加 1px 灰边(--v5-ui-border)+宏卡柔影(--v5-shadow-macro-card)+白底+6px/10px 内边距+radius-control，注释更新；project.css .v5-project-thread .v5-message--user 同款（width 改 max-width 贴内容右收）；render-smoke.mjs 两 fixture 面 computed border+box-shadow 断言+assistant/vit 无框负断言+两面截图+汇总报告 render-smoke-shadowbox-report.json | 下一步=S7 三门验证（命令：cd agent/webui && npm run test / npm run build / npm run test:render；基线 vitest 573 不回退）
- S7 | 三门验证完成：npm run test=exit 0（573/573 基线不回退，npm ci 先行装依赖）；npm run build=exit 0；npm run test:render=exit 0（gallery/launcher/project/loading/fidelity/appentry 全 PASS+SHADOWBOX_RESULT=PASS，computed frame 两面在卷）。首跑 FAIL 一次=alignSelf computed 值为 flow-end（Chromium used value 序列化），断言放宽为 /^(flex|flow|self)-end$/ 后全绿——非样式缺陷，断言精度问题。负对照：临时摘除 launcher 框样式→LAUNCHER_RESULT=FAIL 拦截 exit 1，复原复跑 exit 0 | 工件=screens/4 张+render-smoke-shadowbox-report.json | 下一步=S8 提交+回执
- S8 记录 | 提交范围：仅本卡 4 代码文件+coord/runs/UI-MSG-SHADOWBOX-1/**；其他卡目录（FOUNDATION/LAUNCHER/PROJECT/LOADING/SPRITE-MOUNT/FIDELITY）被冒测复跑重写的 report/截图不随本卡提交（361 惯例），留工作区
