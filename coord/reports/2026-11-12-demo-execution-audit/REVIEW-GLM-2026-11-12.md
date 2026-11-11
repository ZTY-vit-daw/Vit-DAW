# 展演乐局 GLM 独立复核与用户裁定落档（2026-11-12）

- 记录：GLM 决策侧会话（ZCode / GLM-5.3）；在 Codex 执行审计与 Claude 修订清单（REVISIONS-2026-11-12.md）之后进行。
- 性质：只读独立复核 + 本轮用户裁定的落档。未修改生产代码、未启动真实栈或生成引擎、未领取或派发卡。
- 关系：本文件不覆盖 REVISIONS，而是在其上叠加增量；三方（审计 README、REVISIONS、本文件）冲突时，以本文件 §2 用户裁定为准。

## 1. 对审计与修订清单的核对结论

复核同意审计与修订清单主体：W18/W19 拆分、W05 整段覆盖简化、A2UI 暂缓改扩展现有 interactionCardKit、G0 三关口、卡粒度分层、HANDOFF §4 旧卡处理表。

独立抽查六处代码事实，全部属实：

| 抽查项 | 锚点 | 结论 |
|---|---|---|
| 只认 `vit.level_card.v0` 且 KnownFields 拒未知字段 | `agent/internal/edu/lesson_card.go:49/514` | 属实（H1） |
| 普通轮只做拉回计数，阈值才注入 Goal | `agent/internal/chat/edu_jam_chat.go:151-164` | 属实（H2） |
| binding 落工程父目录，无 UUID 隔离 | `edu_jam_chat.go:1115` vs `agent/internal/projectstore/store.go:117-124` | 属实（H3；且 `:1113` 注释声称"与 AgentDirName 同域"有误导——projectstore 实为 `Agent/<UUID>` 二级目录） |
| 内核无 set_time_signature 命令，拍号只读入 musicContext | `VitApp/Source/Service/CommandDispatcher.cpp:2245-2247`（仅 setProperty）、`:2688`（仅 set_tempo 注册） | 属实 |
| auditionPrepareArgs 强制恰好 2 候选 | `agent/internal/kernel/audition.go:43` | 属实 |
| yueRequestBuilder 不发 prompt，ExtraParams 最后覆盖 | `agent/internal/genapi/dialects.go:208-229` | 属实 |

## 2. 本轮用户裁定（2026-11-12）

| 项 | 裁定 |
|---|---|
| D1 调号字段 | **采纳**：设计源 schema 增 `workspace.timing.key_signature`（本课默认 C，材料条目可覆盖）；供 W18 音高拼写与 W10 YuE2 ABC `K:` 消费。属乐局内容层设定，开发后实验可调 |
| D3 版本快照 | **采纳**：学生作品版本快照/差分归 W02；clip alias 解析与版本快照列为 W02/W05/W10/W19 共用契约，W00 冻结契约清单写入 |
| D4 拍号两层 | **用户澄清**：工程拍号统一不变（YAML 声明、创建时应用、本课 2/4）；Tap 控件内"拍号选择"保留，但语义是 **agent 教学演示/节拍器用途的局部拍号**——不写工程、不进检查计量。W07 units 与验收按此改，非矛盾修正 |
| 其余增量 | D2、D5、D6、D7、D8、R1–R5、排期算术与 11/19 三条件口径：用户未再补充，按本文件 §3 并入 W00 执行 |

## 3. 追加到 W00 的更正/合入条目

在 REVISIONS §3 更正表与 §4 包调整之上追加：

| 对象 | 改法 |
|---|---|
| `design.v1.yaml` workspace.timing | 加 `key_signature: C`（与 revision 3 修订同期；catalog 材料已有 `demo_key` 覆盖机制，材料侧无需另改） |
| WORK-PACKAGES W02 | depends_on 加 W18（c-q1/c-q3 的"时值/休止与声明范围合法"判定用 W18 记谱模型） |
| WORK-PACKAGES W07 | units "拍号选择"改"演示拍号选择（agent 教学演示/节拍器局部状态，不写工程）"；验收补"演示拍号不改变工程拍号与检查计量" |
| WORK-PACKAGES W04 | units 加"WebView2 宿主内 Tap/谱面按键验证一次"（键盘焦点风险提前到选型阶段，不等 W16） |
| W00 冻结契约清单 | 明确 clip alias 解析与版本快照为 W02/W05/W10/W19 共用契约；W10 序列化只取 region 绑定 clip（辅助鼓轨天然排除，不得混入学生旋律条件谱）；每轮注入 token 分层预算（材料引用+摘要注入、详情按需取，注入规模有上界） |
| HANDOFF §2 归属范围 | 加：Godot 前端仓（`D:\Godot\project\vit-daw-frontend`）本线改动核对与提交——341 返工文件域在该仓，主仓共享提交不覆盖它 |
| W16 first_slice | 显式含浏览器级 DOM 断言基建（E2E-WEBUI-1 方向），不默认现有按钮驱动已够谱面/播放器类交互 |
| W15 材料优先级 | 按节点顺序而非 catalog 顺序：n1 五条 pitch 例、n3 四条 contour＋两条综合例、n5 《三十里铺》原旋律版 ≈ 10–12 条，11/17 前完成转写＋机器比对（听核可后置到 11/21 批次） |
| `edu_jam_chat.go` legameBindingPath | 修复卡执行时：binding 路径直接 UUID 化（对齐 projectstore.Resolve 的 `Agent/<UUID>` 结构），同步修正 `:1113` 误导注释；不留"独立目录"隐性约定 |
| W06 前置 | 开工前半小时级只读盘点内核命令面（solo / mute / loop / play-from-to），确认"单源试听"拼装路径，避免 G0 运行时才发现需新内核命令 |

## 4. 排期口径（按用户 2026-11-12 终裁定更新）

- 开工定为 **11/14 下午（额度重置后）**；首版窗口 11/14 下午–11/19 ≈ 5 天半，三执行线 ≈ 100–135 可用小时 vs 首版 120–208 专注小时（M1 已收窄为导入-only 后取下限段）——**11/19 需三条件同时成立**：(a) G0 三关全绿（11/14 下午派卡、11/15 晚结论）；(b) M1 范围收窄（已裁定执行）；(c) 谱面线无 ≥1 日返工。用户已表态"顺着执行、不抢跑"：11/19 只达 G2 部分不算失败，滑到 11/20–21 正常处理，锚点是 11/30。
- **W00 共享提交随 11/14 开工首位执行**（约 1–2 小时），完成后再派 G0 三关探针；不提前抢做。执行环境槽位数（Codex 4 槽 vs GLM 侧 flash 窗口等）是每日变量，每日开工记录应写明当日环境与槽位，它直接改变上述算术。

## 5. 仍待决（2026-11-12 更新）

已决：M1 入口范围＝方案 A；G0＝11/14 下午派卡、11/15 晚结论；11/14–15 周末开决策流（14 下午开工）；工程包语义与统一 YAML 意图文档（决策记录 §15）。

仍待决：实践 1/2 空谱 2/4 写法（导师，建议 8 拍＝2/4 四小节）；题 3 参照自由度（导师，维持现稿口径）；展演传播的版权口径。

## 6. 本轮未做

未领取卡、未修改生产代码、未启动运行栈；仅新增本文件，并在审计 README §1、HANDOFF §1/§5、决策记录 §14 落档入口与裁定。
