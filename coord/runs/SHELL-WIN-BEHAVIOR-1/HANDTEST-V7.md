# 391 手测 v7 放行包（2026-11-18 晚，PC 主管 lead）

- 自动验收证据：414 组合真栈 attempt03 PASS（`coord/runs/SHELL-WIN-COMBO-1/run_20261118_190253`）：409 原生分类器 PASS + 395 呈现 DOM PASS + chain_detail=workspace_ready + 零 LLM + 卫生还原。
- 手测入口（产品包装入口，已按用户裁定）：`D:\Godot\project\vit-daw-frontend\VitShell\build\Release\启动VitShell.cmd`
- 本机被测产物哈希（均为 415 修复后）：
  | 产物 | SHA256 |
  |---|---|
  | VitShell.exe | `F8FD36ABEFA46D09983E119C0DA2457D87BB7ABBB577501461454199BECD8B0F` |
  | extension debug DLL | `B270362D1E3380D27A709747611FED149A8A0F54B11BA00C1229834510CD2EB7` |
  | VitAgent.exe | `27D09FA09D0CD6F710F9BFAD7E06F44242159987178241953431B13C435C3768` |
  | webui dist/index.html | `0886058EF839F421FBA09FDEAE381583FE5E8911D561DD9C4E5866161EFCFB9F` |
  | 启动VitShell.cmd | `293E9B48F6974E5A8CA436B35AADDFDBC1C245FDE18BB548ED80119822BE7CD9` |
- 自动验收建议的正式单次：shell 条件（对话窗前台）——用户手测可自然操作，无特殊环境变量要求。

## 手测清单（按 391 原卡判据）

1. 双击 `启动VitShell.cmd`：VitShell 启动台出现（最近工程/新建/打开），Vit 系进程由宿主托管。
2. 打开一个既有工程（最近磁带卡或打开按钮）：
   - 对话窗应保持前台，DAW 大窗出现在对话窗**之后**，不抢焦点；
   - 对话流内出现**真实装载步骤**（非假进度），完成后显示完成卡「工作台已打开，工程内容已载入」。
3. 聚焦行为：点击 DAW 窗口使其到前台——对话窗标头应出现轻量提示「工程已在前台」；点回对话窗后提示消失（轮询约 250ms）。
4. 同窗切换：关闭 DAW/返回启动页应同窗无重载；再次打开工程重复步骤 2-3。
5. 可选负例：文件选择取消、无工程状态下的打开链；不得出现假百分比、假文件/音频步骤。

## 已知边界（如实声明）

- 自动证据只覆盖单次 shell 自然前台成功路径；不构成稳定性/成功率结论，帧级层级闪动不在断言内（409 裁定允许显示后一次纠正兜底，本轮未用到）。
- 导出模板与 `template_release` DLL 未刷新，发布/导出路径不在本批。
- 396 GODOT-RETRY-LAUNCHER-1（失败重试导航 1 行 gd 微卡）未并入本批；重试失败路径行为维持旧版，不属 391 判据。
- 395-B directive 占位文案（launcherData.ts，域外）未合入；不影响装载反馈/焦点提示主链。
- 焦点提示正例需要真实把 DAW 置前；自动化只覆盖了 shell 条件下的负例（无 DAW 焦点声明/无 hint）。

## 回传给主管的内容

- 通过/不通过；
- 若有问题：具体现象 + 复现步骤 + 是否可稳定复现（截图/录屏更好）；
- 手测通过后由主管出 391 终验 ruling 并收口。
