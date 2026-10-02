# WEBUI-STATUSBAR-ID-1：执行状态栏移除哈希类会话 id——换短编号或删除（用户补充诉求）

- 池序 17（P3 小修，**晚窗参考序 2**——2026-10-01 早窗校准：flash 量产窗口可接）；来源=用户 2026-09-30 晚窗补充："执行状态栏（输入框上方矩形）中有会话记录的哈希 id 一类的数字，最好从 webui 去掉，要么换一套更方便观察的编号 id，要么不需要"
- 优先级 / 预估 / 依赖：P3 / 0.25 天 / 无；webui 域（与同域卡串行）
- 模型分级：L0 写卡即走 / flash 可接
- **裁定（2026-09-30 用户澄清后修订，覆盖前裁定）**：**默认直接移除**该哈希 id 显示（编号只是给用户看的、无信息量）；若实锚发现该位置需要一个区分占位，用**纯线性数字**（1、2、3…，无字母前缀无哈希）——不得用 T3/webui_xxx 类带前缀形态。完整 id 不上 UI；调试需要时走 DOM/网络面板查（零成本）。
- 目标：实锚该矩形组件（输入框上方执行状态栏）中哈希 id 的渲染点 → 按裁定替换/移除 → 回归（该元素若被既有 webui 测试/E2E 断言引用，同步更新断言）。
- 文件域：agent/webui/src/（状态栏组件+样式）+ 若断言涉及则对应测试/烟测脚本。
- 验收标准：npm run test 全绿+build+E2E-WEBUI-1 exit 0（若改了被断言元素）；回执附前后截图或 DOM 片段。
- 停止条件：常规止损。
- 领取：并入 WEBUI-IA-REDESIGN-1 一并执行（2026-10-02 20:43，doing 卡领取记录在案；回执/验收随主卡）
- 回执：随主卡 WEBUI-IA-REDESIGN-1 交付（实现 commit=6501e051，分支 port/webui-ia-redesign-1）——PlanBar `Task {id}` 可见文本移除、data-task-id 属性保留、无占位编号（单任务窄条无区分需求，按裁定默认移除式）；PlanBar.test.tsx 断言同步更新；E2E ia-redesign-statusbar-IA3 组红绿验证在案。验收随主卡。
- **并卡注记（2026-10-02 决策侧）**：本卡并入 WEBUI-IA-REDESIGN-1（池序 36）一期目标 3 执行——两卡同域同分支一次交付，本卡不再单独领取（[decisions/2026-10-02-webui-ia-vision.md](../../decisions/2026-10-02-webui-ia-vision.md) 用户裁定）。
- 验收：**pass（2026-10-02，随 WEBUI-IA-REDESIGN-1 并卡交付）**——[rulings/2026-10-02-WEBUI-IA-REDESIGN-1-conditional.md](../../rulings/2026-10-02-WEBUI-IA-REDESIGN-1-conditional.md)；可见 Task 文本移除+data-task-id 属性保留（调试走 DOM）+两处断言同步+E2E IA3 组绿；commit=f6640d42
