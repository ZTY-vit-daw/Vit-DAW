# CONV-WINDOW-SHELL-1（428，A 方案）验收：PASS（零栈）

- 卡：`2026-11-18-CONV-WINDOW-SHELL-1`（池序 428）；主管=lead；执行=lead/convrail418。
- commit `4cb47032`（5 文件 +270/−32）；已合入 main `7b815d57` 并推送。

## 交付

- 新增 `ConversationWindow` 宿主（右侧窗口壳）+ 两个持久叠放流层（launcher/project）；已挂载层在 surface 切换/工程打开/管理视图往返时**不卸载**，非活动层 `hidden+inert+aria-hidden`，活动层 200ms opacity+translateX 入场（reduced-motion 关闭）。
- App entry 改为窗口壳 + 双流层；launcher 层首次进入后挂载标记保持；project 层沿用 managedProject/surface 挂载门；返回管理视图再进工程时层不重建。
- 保留层内 scrollTop/输入草稿/加载卡状态；ConvergedLauncher 在持久层恢复活动时补一次 recents 刷新（等价原 mount fetch，ENTRY388 旅程仍绿）。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；63 files / **755 tests** |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**114 checks / 0 failed**（101 既有 + 13 新增） |
| `npm run test:render` | exit 0；APPENTRY/ENTRY388/RESULT/LAUNCHER/PROJECT/LOADING/FIDELITY/SHADOWBOX/REC-TAPE/WIDGETS/CONVERGED 全 PASS |
| 主检出 dist | 重建 `dist/index.html` SHA256=`7C30A2B6C277A4996799CC78F2143BA92670BF30F1DE64477FAF0AD52A677D46` |

## 边界/裁定事项

1. **首次 launcher→project**：沿用 2026-11-15 用户裁定「进工程后启动台面整体卸载」（REC-TAPE 真实入口守卫），这一次 launcher 层不保留；此后 project↔launcher（管理视图）往返两层持久挂载、窗口壳/层 DOM 不重建。若要求“首次打开工程也不卸载 launcher 层”，需改该旧裁定/守卫——待用户确认是否扩域。
2. project 层仍由 managedProject/surface 门控：宿主回 launcher（DAW 退出/工程解绑）时卸载属会话生命周期终止，不是 surface 切换。
3. 首次打开工程仍有一次真实 context/历史加载（A 方案接受）；层内 frame 沿用（不重做统一标题栏/composer），B3 单窗常驻未触发。
4. 零真栈（Playwright+mock）；真实端侧/手测待用户。

## 验收结论

管理视图↔工程面往返与活动流层的“窗口不刷新、内容切换”已达成；左栏 EntryShell 同一节点不变；加载卡不重播、滚动保持。用户手测确认观感后可关闭或决定是否升级。
