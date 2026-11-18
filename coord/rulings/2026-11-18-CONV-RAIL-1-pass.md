# CONV-RAIL-1（B2a）验收：PASS（零栈）

- 卡：`2026-11-18-CONV-RAIL-1`（池序 418）；主管=lead。
- 执行：`lead/convrail418`（DeepSeek-V4.1-Flash）；分支 `codex/shell-conv-rail-418`，commit `9b1b2d584e617b4de600cbbbf39320b865c8230b`（父 f529e3f2）；已合入主仓 main `c2f7b2c2` 并推送。
- 交付（10 文件，1640+/55−）：工程面左栏「对话流」树（当前工作树→分支）、首条消息摘要标题/分支·活动副标/当前高亮、只读预览（Message 渲染、composer 蒙版禁用、不触发 checkout）、【继续此对话】=version.checkout（confirm/guard/loading/失败保留预览）、【＋新对话】=从活动节点 version.branch_create（必要时补 checkout）、左栏 footer 常驻设置+头像槽、收展 localStorage（`vit_entry_rail_collapsed`）、rail 模式隐藏主标题栏重复 Vit 品牌。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；61 files / **735 tests**（722 基线 + 13 纯函数） |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**73 checks / 0 failed**（既有 50 + 新增 24，B1 占位断言被真实树断言替换） |
| `npm run test:render` | exit 0；APPENTRY/ENTRY388/RESULT/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/CONVERGED 全 PASS，fixture/gallery 零改 |
| mock 生产缝 | version.checkout 与 version.branch_create 的 /agent/invoke 实参/上下文已断言；取消确认=0 invoke 且预览保留；checkout_blocked 禁用并解释 |
| 视觉/交互断言 | 24 条新断言覆盖树/预览/蒙版/继续/新对话/footer 常驻/收展持久化/品牌去重 |

- 主检出 `agent/webui/dist` 已重建（`dist/index.html` SHA256=`6304D7D8C713FC885865CFFA5AF0437980FA11CE4F95299C40BF903FC2243635`），手测入口已含 B1+B2a。

## 边界与后续

- B2a 只服务**当前工作树**；多工作树时分支归属仍是 repo 级 branches，可能把其它工作树分支列在当前工作树下——**B2b（跨工作树）必须先加归属过滤/分组再交用户手测**。
- 「invoke 失败」分支已实现但 render 未单列断言（取消确认/blocked 已覆盖）；真实栈与端到端候 B2b。
- 新分支无消息时预览显示分叉点继承路径（与 agent conversationMessagesForGraph 空分支口径一致）。
