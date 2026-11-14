# UI-VISUAL-FIDELITY-1 差距清单（M1 产出）

- 对照权威：`agent/webui/design/main-window-v5/SPEC-V5.md`（v5.1，冲突时优先）+ `screens/p1.png`（P1 启动台）+ `screens/p3.png`（P3 工程内）+ `screens/s5.png`（S5 组件板）+ launcher 路线稿 `design/launcher-route-mondrian-v2.html`（卡面指定 launcher 视觉权威）+ `IMPLEMENTATION-HANDOFF.md`。
- 被对照实现：`agent/webui/src/styles/launcher.css`、`src/project/project.css`、`src/launcher/LauncherSurface.tsx`、`src/project/ProjectSurface.tsx`、344 基座 `src/components/components.css`（只读对照）。
- 基线渲染证据：`coord/runs/UI-{FOUNDATION,LAUNCHER,PROJECT,LOADING}-1/` 四报告全 PASS（本 worktree 复跑，1440×900@1）。
- 标记含义：〔本卡修〕=纯视觉层/尺寸策略，本卡文件域内修复；〔缺口上交〕=344 基座组件结构差或需功能接线，按卡面停止条件不硬改；〔移交 SHELL〕=宿主窗口域；〔保持〕=已在案裁定或已符合定稿。

## A. Launcher surface（P1 启动台）

| # | 定稿稿 | 实现 | 处置 |
|---|---|---|---|
| L1 | P1 问候=屏级标题 28px/700（SPEC §6 字阶；P1 主区首要素） | `.v5-launcher-greeting` 18px/700 | 〔本卡修〕→28px/700 |
| L2 | P1 建议区=大动作卡：蓝壳底（control-shell 系）、1px 蓝灰边、4px 圆角、约两列网格、14–16px 文字；SPEC §5.3 明令「不用胶囊形」 | `.v5-launcher-suggestion` 胶囊（border-radius 999px）白底 1px 灰边 13px | 〔本卡修〕→蓝壳动作卡语言、4px 圆角、两列网格 |
| L3 | P1 抽屉工程卡：磁带 icon（较大）+工程名 14–15/600+次要信息小字；卡面 1px 边、4px 角 | `.v5-launcher-recent` 白底 1px 灰边 6px 圆角、badge 20px、名 13px、path 13px 同灰 | 〔本卡修〕→badge 24px、名 14/600、path mono 12px、4px 角 |
| L4 | S5 消息板+P3：用户消息右对齐、Vit 消息左对齐，不加色块/徽章（§8.1 二选一里定稿选了「对齐」） | launcher 自有 `.v5-launcher-msg--user` 蓝壳气泡（右）、`--assistant` 白底带框气泡（左） | 〔本卡修〕→去底色去框、按对齐区分（launcher msg 样式为 345 surface 自有层，非 344 原语） |
| L5 | SPEC §5.3：圆角只出现在交互控件（4px）与状态点；面 0 | directive 卡 8px、bridge-exec 块 10px、msg 8px、gallery 窗框 10px | 〔本卡修〕→统一 4px（窗框 8px 近似 P1 样图窗角） |
| L6 | v5.1 纪律+P1：标题栏仅「Vit」文本 | 标题栏=「Vit」+「启动台」副标签 | 〔本卡修〕→去副标签（烟测断言只锁 brand span，不受影响） |
| L7 | 宿主窗口尺寸不确定（96%/94% 夹取+125–150% DPI）；HANDOFF：「不能把固定 1440×900 样图直接锁死成用户窗口尺寸」 | live 面 viewport 绑定+线程内滚动已就位；无矮视口/窄视口紧凑档 | 〔本卡修〕→≤900px 宽收窄抽屉、≤720px 高收紧留白；+多分辨率渲染断言（见 D 节） |
| L8 | P1 抽屉顶部「新建工程」「打开文件夹」双按钮（带 icon） | 未实施（345 仅有桥「从文件打开…」按钮）；346 卡面明示「左缘抽屉（新建/打开）不在本卡目标清单，另卡接入」 | 〔缺口上交〕需桥 directive 交互语义，随交互卡接入；样式语言可复用本卡 L2/L3 成果 |
| L9 | P1 首张工程卡选中态=黄底+2px 墨框（「继续上局」语义） | recents 为展示列表，无 onClick、无选中态 | 〔缺口上交〕选中语义=功能接线（点选→open directive）；不虚加样式假态 |
| L10 | mondrian-v2 态A：右侧磁带卡列（144×94 磁带 icon+气氛渐变+OPEN 按钮）、底部磁带流红把手带 | 实现为 P1 结构（左抽屉+建议卡），无磁带卡列/磁带流 | 〔保持〕mondrian-v2（11-09）与 SPEC-V5 P1（11-14，冲突优先）同为用户确认稿；当前实现取 P1 结构。磁带卡血亲语言（icon/气氛渐变）归 344 ProjectBadge/未来卡片，不在本卡硬仿两稿叠加 |
| L11 | — | 抽屉 title「最近工程」+count「最近工程 · 3 个」字面重复 | 〔保持〕count 文案被 launcherData.test.ts:88 锁定（`toBe("最近工程 · 1 个")`），改动收益低、翻测试成本不成比例；记录不动 |

## B. Project surface（P3 工程内）

| # | 定稿稿 | 实现 | 处置 |
|---|---|---|---|
| P1 | S5/P3：用户消息右对齐无色块，Vit 左对齐 | 344 `Message` 原语=用户 canvas 气泡左对齐 | 〔本卡修〕project.css surface 层覆盖（`.v5-project-thread .v5-message--user` 右对齐+去底）；344 原语文件不动（见 P6 缺口） |
| P2 | P3：轨迹/回执/A/B 平铺在工作面上（无卡底）；A/B=右侧「对比 A/B」次按钮+note | `.v5-project-ab` 白卡底 8px 圆角 | 〔本卡修〕→去卡底去圆角，平铺对齐 P3 |
| P3 | P3 线程顶部上下文行：[→ 鼓组总线] ContextTarget chip+「6 条轨道」+右「引用轨道」按钮 | titlebar=i-project+工程名+detail 文本；无 chip 行、无引用按钮 | 〔缺口上交〕上下文 chip 行与「引用轨道」需真实上下文目标数据+attach 语义接线（功能域）；344 ContextTarget 原语已在卷可供直接接入 |
| P4 | S5 宏卡：三选项**横排三列**、每格自带频响轮廓（选中黄曲线、未选蓝）；P3 同 | 344 `MacroCard`=选项纵排全宽+**共享单一**轮廓槽（选中的那条） | 〔缺口上交〕344 基座结构差——surface 层 CSS 硬掰会造出「第三种解读」（有列无每格轮廓），按停止条件不改 344；建议另开 344 基座宏卡布局修订卡 |
| P5 | P3 窗内左缘竖条抽屉（新建工程/打开文件夹 icon 竖排） | 未实施 | 〔缺口上交〕卡面已注明 SHELL-2 抽屉按钮在保真后样式上做——移交 SHELL-2 |
| P6 | S5：用户消息无色块右对齐 | 344 `Message` 原语默认=canvas 气泡左对齐（gallery 页按原语呈现） | 〔缺口上交〕344 Message 原语与 S5 定稿的差异（本卡只在 project surface 层覆盖，344 默认不动） |
| P7 | S5：输入框发送键=ink 底 icon+「发送/停止」文字 | 344 `TextField` 发送键=icon-only（aria-label 有文字） | 〔缺口上交〕344 原语微差，surface 层无法补文字内容 |
| P8 | S5 动作后果卡：可撤销=浅蓝壳底 | 344 `ActionCard` 可撤销=ui-plate 灰底（confirm 黄/irreversible 红 ✓） | 〔缺口上交〕仅 gallery 使用，不涉 launcher/project 两面 |
| P9 | P3 窗框 1px 边+柔影+约 58–62% 宽 | `min(880px,100%)`=1440 下 61% ✓；radius 10px | 〔本卡修〕radius→8px；其余已符合 |
| P10 | P2 装载面（347） | 磁带带量三帧/三步轨迹/装载门已在卷 | 〔保持〕347 已交付且基线 PASS，本卡不动 loading |

## C. 窗口裁切（用户实机「显示不全」）

| # | 证据 | 归属 |
|---|---|---|
| W1 | 宿主窗口尺寸策略：`D:/Godot/project/vit-daw-frontend/app/shell/agent_chat_window.gd` `_initial_window_size()`（L742-744）：`mini(natural, screen*0.96 / screen*0.94)`——无 DPI 逻辑/物理坐标换算、无 `screen_get_usable_rect`（任务栏）修正、无最小尺寸下限。125–150% 缩放下 `screen_get_size` 返回值与 WebView2 child hwnd 的坐标基准差异=裁切根因候选（窗口本体尺寸归宿主） | 〔移交 SHELL-1〕建议：usable_rect 夹取+DPI 修正+child hwnd 按 client rect 尺寸+最小可用下限（如 960×600） |
| W2 | webui 侧缓解：live 面 `#root` 100%+overflow hidden、线程内滚动（data-scroll-intentional）已在卷；本卡补矮/窄视口紧凑档（L7）与多分辨率渲染断言（D 节） | 〔本卡修〕 |

## D. 多分辨率渲染断言（M4 扩展，工件落本卡目录）

- 视口组：1440×900@1（样图基准）、1920×1080@1、1536×864@1.25（Windows 125% 等价）、1280×720@1.5（Windows 150% 等价）。
- 断言面：launcher-live / project-live（fixture+fill 离线渲染页，本卡新增）每视口 ①文档无横向溢出 ②无未声明裁切容器 ③composer/建议/输入在视口内可见（bounding rect）④正文对比度/最小字号扫描 ⑤截图留档。
- 断言与工件路径：`coord/runs/UI-VISUAL-FIDELITY-1/{render-smoke-fidelity-report.json, screens/}`；344/345/346/347 原断言与工件路径不变。

## E. 修复统计口径（回执用）

- 本卡预计修：launcher L1–L7、project P1/P2/P9、多分辨率 D/W2；缺口上交 L8/L9/P3/P4/P5/P6/P7/P8；移交 W1；保持 L10/L11/P10。
