# Ruling：WEBUI-IA-REDESIGN-1 — conditional pass（实现+E2E 面全过；用户目检复验留待）（2026-10-02 决策会话）

## 验收四层

1. **diff 亲核**（6501e051@port/webui-ia-redesign-1，9 文件 +1434/-2861）：
   - 删除边界核实：App.tsx 净 −1654（四件视窗族+focus 切换体系+死码 helper 族）；styles.css 死族清理；**dawTracksFromUIState/DawTrack 模型族保留正确**（确认卡/工程结果卡消费者实证）。**同族超额删除采信**（mixer-strips+无调用点的 Rack V1 死码——decision §能力边界「手工细调归 DAW GUI」同族论证成立，同一提交内独立可回滚）。
   - 侧边栏：SessionFlowSidebar.tsx+sessionFlow.ts 注册表+14 单测；新建走既有轻量通路（webui_ id）**不建工作树不建分支**（E2E 断言原文在案）；切换=scope 暂停+锚定改写+消息本地桶水合（复用既有恢复效应，无平行状态机）；重命名/归档还原/折叠（localStorage）；note 流零接触；注册表 fail-open（损坏 JSON 丢弃不阻塞，§11 单测钉）。
   - STATUSBAR-ID-1 并入交付：可见 Task 文本移除+data-task-id 属性保留（调试走 DOM）+两处断言同步——裁定形态（默认移除）兑现。
2. **我方独立复跑**：npm run test **413/413** exit 0+`npm run build`（tsc+vite）exit 0（worktree 内亲跑）。
3. **E2E 工件亲读**（三 run root 互不覆盖+prereq 溯源全量：head/二进制 SHA256/dist 哈希/泊位 7899 验空/桥端口覆盖/agent 每轮 stopped）——**RED（旧 bundle）28 组中恰好三个新 IA 组红**（侧边栏未渲染/.side-rail+.rail-button×8 在场/Task 文本可见=断言真能抓住旧形态）；**GREEN2（新 bundle）28/28 verdict=pass**；green 首轮 IA3 误判的采样位修正过程如实申报（会话切换后任务轨迹清空是合法行为）。
4. **三核对结论采信**：E2E/单测对三件套零断言引用（"daw" 命中=daw_action 消息 fixture）；midi-editor-preview 全仓唯一引用=自身（零复用，未触发停止条件）；demo 素材清单无镜头引用（登记）。

## 裁定

- **conditional pass**：实现面+E2E 渲染面全过；**用户目检复验**（三区布局/侧边栏操作手感/视窗已删）留待下次打开 webui 顺手确认，非阻塞项。
- cherry-pick 6501e051 合 main；**STATUSBAR-ID-1 随本卡关闭**（同 commit 交付，独立卡归档）。
- 二期钩子（工程概览证据卡+A/B 卡可播放渲染）已在卡面登记，届时另立卡。
