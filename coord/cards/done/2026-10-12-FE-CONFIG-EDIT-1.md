# FE-CONFIG-EDIT-1：设置页 LLM 配置输入体验修复（保存清空 key 框+占位符乱码+输入防扰）

- 发卡：Mac 决策侧 / 2026-10-12 深夜（依据=用户实报"一开始能输入然后无法输入"+Mac 决策侧浏览器级复现取证）
- 派发确认：已确认（用户 2026-10-12"这个肯定得修"）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 105；目标仓库=D:\Vit_DAW
- 归属：Mac 线（webui 渲染面）；P2 / 0.5 天 / 无依赖
- 模型分级：L1 / GLM 或 flash 执行会话

## 取证底账（Mac 决策侧 2026-10-12 实测）

1. 源码（dev 5178）与部署 dist（agent/webui/dist，9-19 构建）在 Chromium（IAB）下三字段均可输入（含空态/删后重输/带后端）：**输入处理本身无缺陷**。
2. 带真实 agent 后端复现：**点击"保存"后 API Key 框立即清空**（saveConfig→mergeSettingsConfig 强制 apiKey:""，设计为"留空保持不变"）——用户换配置场景（输入新 key→保存→字段清空）体验为"刚打的字消失"；"重载"按钮同效。此为主诉可复现核。
3. 用户容器中"清空后再也打不进"未能在 Chromium 复现——归因候选=浏览器/引擎差异（Safari/WKWebView 的 password 类字段自动填充干预、中文输入法 composition 与受控输入组合），本卡一并加固并在用户容器验证。
4. 默认模型占位符为乱码（"供应商模型 ID"双重编码为 `渚涘簲鍟嗘ā鍨?ID`，App.tsx:1890，仓内已提交态）。

## 修复四件

1. **保存后不清空已输入的 key**：saveConfig 成功后 apiKey 编辑值保持（掩码显示），状态行明示"已保存"；不再让用户以为输入丢失。（mergeSettingsConfig 的 apiKey:"" 仅保留给"重新打开设置页"的初始加载语义。）
2. **占位符乱码修复**为"供应商模型 ID"，并全设置页扫一遍同类双重编码串。
3. **输入防扰加固**：API Key 框加 autoComplete="new-password"、其余文本框 autoComplete="off"；确认无 keydown/preventDefault 类拦截（取证已核无，入回执复核）。
4. **保存语义提示**：状态行区分"已保存（key 保持输入态）"与"留空=沿用已存 key"两种情形文案。

## 文件域

agent/webui/src/App.tsx（设置页区块+SettingsTextField）；vitest 测试新增；**渲染面卡**——验收按 AGENTS §5：vitest DOM 断言（保存后输入值保持/占位符正确/autoComplete 属性在场）+浏览器级复验（执行会话以 headless 或 IAB 同模式断言一轮）+用户容器手测清单（Safari/Godot webview 实机输一条 key，[等待用户]）。

## 验收标准

vitest 全绿+npm run build exit 0+浏览器级断言 exit 0+手测清单交付；乱码串全页清零（grep 甄别双重编码形态）。

## 停止条件

保存后保持输入态与"留空保持不变"语义冲突需重新设计→上交决策侧定交互。

- 领取：2026-10-12 23:02 / 151b0fcb / Mac 执行侧（GLM-5.3） / port/fe-config-edit-1 / 领取提交=本条 mv+回填（coord-only 直推 main，commit be290b36）
- 回执：实现 commit ea8df384 @ port/fe-config-edit-1（+56/-10 App.tsx + 新增 settingsConfigEdit.test.tsx 133 行 + run 工件目录）。
  测试清单：① `npm run test` exit 0（38 文件 / 454 测试全绿，含新增 10 条：占位符正确+乱码字符类清零断言、LLM/路由 autoComplete 断言、retainSecretInputs 保存保持/留空不填值/mergeSettingsConfig 对照清空、settingsSavedStatus 两情形文案）；② `npm run build` exit 0；③ 浏览器级断言 PASS 16/16（coord/runs/FE-CONFIG-EDIT-1/20261012-2315-browser/browser_assertions.json）：真实栈=vite dev 源码态 + worktree 构建 vitagent（隔离 HOME，非用户 ~/.vit），真键盘路径（cua.type 逐字符）两轮 34 字符全部入框（无按键拦截实证）、保存后主 key/路由 key 输入态保持、状态行两情形文案在场、重载=初始加载清空语义保留、落盘核对（留空保存后服务端 PreserveBlankSecrets 保留主 key）。
  乱码复核：全 src/ grep 双重编码字符类（[渚涘簲鍟嗘ā鍨] 及广谱 GBK→UTF8 形态）仅原 App.tsx:2001 一处，已修复；设置页 DOM 浏览器级再断言 false。
  按键拦截复核（第 4 件）：代码面=App.tsx 全部 keydown 监听（2206/6895/7310 三处全局）均为菜单开时仅响应 Escape 且无 preventDefault，组件级 onKeyDown 不涉设置页；运行面=真键盘 34 字符无丢失。
  端测边界声明：已覆盖=vitest DOM（renderToStaticMarkup 静态断言+保存态合并纯函数）+ 浏览器级（IAB Chromium，交互式断言非脚本化 exit-code 烟测）+ 隔离 agent 后端全链路（保存 POST→RedactSecrets 回传→UI 保持→落盘）。未覆盖=① Safari/WKWebView（Godot webview 容器）自动填充/IME 组合场景——正是用户容器归因候选，留手测核验；② 真实内核/Godot 三件套（本卡渲染面无需内核）；③ 部署 dist 构建（改动在源码，dist 由发布流程重建）。
  [等待用户] 手测清单（Safari/Godot webview 实机，任一容器执行）：
  1. 打开设置→LLM API：确认"默认模型"占位符显示为"供应商模型 ID"（无乱码）；
  2. API Key 框输入一条真实 key（用系统输入法，含中文输入法开关切换一次）→ 点"保存"→ 确认框内 key 不消失、Agent 标签状态行出现"API Key 已更新，输入框保留所输内容"；
  3. 点"重载"→ 确认 key 框清空且占位符为"API Key 已保存；留空保持不变"；再点"保存"→ 状态行出现"API Key 留空=沿用已存 key"；
  4. 多模态路由任一卡的 API Key 框输入→保存→确认不消失；
  5. 全程确认无"打不进字"复现（尤其 Safari 密码自动填充弹窗出现/关闭后仍可继续输入）。
- 回执：（commit / 测试清单 / 手测清单 / 端测边界声明）
- 验收：pass 2026-10-13 Mac 决策侧（ruling [2026-10-13-FE-CONFIG-EDIT-1-pass.md](../../rulings/2026-10-13-FE-CONFIG-EDIT-1-pass.md)；复验 run 20261013-1015-reverify 四门全绿；merge 合入 main；手测后置=Safari/WKWebView IME 场景待 dist 重建后用户执行，复现则返工；域外 3 处存量乱码另立 FE-MOJIBAKE-CLEANUP-1）
