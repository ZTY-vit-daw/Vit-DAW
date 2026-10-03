# Ruling：WEBUI-INIT-LAYOUT-1 — conditional pass（2026-10-03 决策会话）

## 验收

1. **diff 亲核**（65343834@port/webui-init-layout-1，2 文件）：根因=CSS 初始值层——`.app-shell` 3 行网格模板自 8df48982（09-03 副注条移除）起仅 2 子元素，空行吃掉 `minmax(0,1fr)`、workspace 滑入 auto 行随内容收缩；修复=两处改 2 行+行数=子元素数约束注释。最小面（6 行样式+141 行烟测组）✓。
2. **我方独立复跑**（验证 worktree 亲跑）：vitest 420/420 ✓；tsc ✓；vite build ✓；**E2E 渲染烟测我方复跑 exit 0**（run 20261003_192139：verdict=pass、failed_groups 空、新增 initial-layout-L1 组+既有组全 PASS、隔离 agent 7897 即起即拆）。
3. **复现论证采信**：headless 三视口复现+机制注入（对话增长撑满="对话后自行展开"实证）+RED 双钉（注回旧 3 行值→tracks 3≠2 children+fill slack 双红，计算值与修复前逐字节一致）——"headless 可完整复现"边界声明成立。
4. 8df48982 旧回执"空 auto 行无害"的几何实证缺陷（只验 composer 在视口内恒真、未验不满窗）定位准确，教训记录在案。

## 裁定

- **conditional pass**：实现+E2E 面全过；**用户目检复验（初启即展开）**为转正条件——与 WEBUI-IA-REDESIGN-1 同场目检转正。
- cherry-pick 65343834=**b2829d5f** 合 main。
- dist 需随下次手测前重建收口（本裁定后 main 含两笔新实现）。
