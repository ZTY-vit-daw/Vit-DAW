# SHELL-SETTINGS-DUP-1（426）验收：PASS（零栈；用户手测缺陷）

- 卡：`2026-11-18-SHELL-SETTINGS-DUP-1`（池序 426）；主管=lead；执行=lead/convrail418。
- 分支 `codex/shell-settings-dup-426`，commit `479897dc`（3 文件 +26/−9）；已合入 main `65142940` 并推送。

## 缺陷与修复

- 用户手测：工程页对话窗下出现重复设置按钮。根因=App entry 分支同时把 `onOpenSettings` 传给 `ConvergedLauncher`/`ProjectSurface`，两组件收到 prop 就渲染各自 footer 设置；而左栏 footer 已提供唯一入口（B2a 418）。启动页抽屉底部同样重复。
- 修复：App entry 分支移除两处 prop；EntryShell footer `[data-entry-rail-foot] [data-entry-rail-settings]` 成为 live 入口唯一设置入口。未改 EntryShell/ProjectSurface/ConvergedLauncher 结构；非 entry MainApp 旧路径保持原样。
- 断言：ENTRY388 设置断言改指左栏 footer（点击/焦点归还/420/1440 可访问），并新增唯一性断言（live 入口 `data-launcher-settings`=0、`data-project-settings`=0）；RWP 新增 launcher/project surface 单一设置入口断言。

## 主管复核（亲跑）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；62 files / **746 tests** |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**75 checks / 0 failed** |
| `npm run test:render` | exit 0；ENTRY388 全绿（唯一性断言 PASS），其余段全 PASS，fixture/gallery 不回归 |
| 主检出 dist | 重建 `dist/index.html` SHA256=`A14BE8702EF21C7E80964CD7489C44D7E27EFD616D923A290F03350D650EA87A`（手测入口生效） |

## 边界/后续

- 零真栈/零 GUI；浏览器级渲染断言。
- 非 entry MainApp（旧主窗路径）仍保留自身设置入口；用户手测入口（EntryShell）已唯一。
- 请用户重启/刷新手测入口验证设置按钮只剩左栏 footer 一个。
