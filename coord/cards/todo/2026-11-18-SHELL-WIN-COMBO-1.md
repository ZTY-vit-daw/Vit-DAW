# SHELL-WIN-COMBO-1：391 组合真栈验收（391 解锁卡）

- 池序 414；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚批准方案 A）。
- 依赖：410/411/412 验收合入 + 413 最终产物刷新完成；真栈单 owner（本主管会话放行与登记）。
- 单目标：单次真实产品入口组合验收（VitShell→React 启动面→打开工程→装载完成呈现），覆盖：

1. **409 产品窗口断言**：首显在对话窗下 / 16DIP / 零激活焦点 / 无后纠正（复用 `scripts/vitshell_zproduct_smoke.ps1` 与 `scripts/vitshell_zproduct_verdict.mjs`）。
2. **395 真实事件链**：CDP 断言 `data-window-launch-feedback` 出现、真实步骤与终态（`ready` 或如实降级 `accepted`/`failed`）、`data-native-focus-hint` 正/负边界（正例必须有自然 DAW 前台事实，不可伪造）。
3. **396 retry 腿**：`scripts/ui_entry2_smoke.ps1` 重试路径 exit 0（与单次栈冲突时由同 owner 串行第二腿）。
4. **卫生**：fixture 副本、recents 快照/finally 哈希、权威输入哈希、栈释放亲证。

## 资源与工件

跑前在 `coord/resources/PC-RUNTIME-STACK.md` 登记唯一 owner、推 main 并重读；启动前 ≤60 秒即时核进程/端口净空。run 目录 `coord/runs/SHELL-WIN-COMBO-1/<stamp>/`；跑前成文运行次数、成功条件、失败分类（product/coverage/environment/harness 分开）、止损线。

## 通过条件

script/wrapper/host 均 exit 0 + 断言摘要 + 原始工件。任何 `not_exercised` / 覆盖缺口如实声明，不得抵作 PASS；不以事后正确状态抵销首显反例。

## 停止条件

任一门失败即停并保留原工件；同一确定性断点两次→停转诊断；环境中断单列，不记产品结论。

## 领取/回执/验收

待 410–413 收口后回填（owner=lead/combo 或主管亲跑；分支/worktree、资源记录、执行时间同批登记）。
