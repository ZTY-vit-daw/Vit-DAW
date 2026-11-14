# UI-PROJECT-1 · 主管裁定：pass

- 日期：2026-11-14 晚窗
- 裁定人：GLM 主管决策流（ZCode 会话）
- 执行：freeflash 免费车道子代理（领取 a0544d96 / 回执 ec12c3e5 / 实现 port/ui-project-1 @ c8a1fc1d+899378c2+16985f5f+c8228c0f）

## 证据核验（主管亲证）

1. **主管复跑三门**（执行侧 worktree）：`npm run test` → 529 passed（498+52 新增：reducer 红线 21+surface 31）exit 0；`npm run build` exit 0；`npm run test:render` → FOUNDATION/LAUNCHER/PROJECT 三 PASS，textMin 5.077/graphicMin 4.305 不变（344/345 断言零回退）。
2. **App.tsx 三处最小分发亲审**：345 同款（import+type+3 行 wrapper），full/main/workbench 零变化。
3. **语义红线单测在卷**：applied 仅经 macroApplySucceeded（在途+选择一致+非空 snapshotRef 三反证）；appliedWithoutSelection=0 DOM 断言；撤销 ref 不一致拒绝；失败保选择+原因+重试。
4. **styles.css 零改动**（优于卡域允许——project.css 自包含）；fixture 全标注；live 模式无协议处 service 抛错拦截面（不伪造），协议缺口按停止条件上交（宏快照应用/撤销、A/B 渲染 agent 侧无现成协议——**候主管另开卡的协议缺口登记**）。
5. `data-scroll-intentional` 豁免标记仅本卡页面使用，344/345 页面无标记行为不变（复验数值亲证）。
6. 344/345 工件被共享烟测再生成 → 执行侧 git restore 还原已提交版本，处置正确。

## 合入

- c8a1fc1d+899378c2+16985f5f+c8228c0f cherry-pick 至 main；worktree 清理；port/ui-project-1 留远端存档。

## 解锁与登记

- **347 UI-LOADING-1 依赖达成**，但与 348 同持 App.tsx → **串行：348 先（P0/P1+真栈空窗），347 随后**。
- 协议缺口登记（后续卡候选）：宏快照 apply/undo、A/B 渲染的 agent 协议——live 面已拦，需要时另开卡。

## 边界

- gallery fixture 页渲染面成立；live `?surface=project` 真实端点行为待真栈验收；E2E-WEBUI-1/JOURNEY-1 层级未建，不声称端到端。
