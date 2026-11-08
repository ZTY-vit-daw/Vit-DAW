# 乐局产品式体验设计 v1（LEGAME-UX-DESIGN-1）

- 产出：ZCode GLM-5.3 L2-worker PC 执行子代理（会话 313legame）/ 2026-11-08 深夜段 / 零实现零栈纯设计
- 权威输入（裁定链，冲突时以裁定链为准）：[decisions/2026-11-08-product-style-handtest-ruling.md](../decisions/2026-11-08-product-style-handtest-ruling.md)（六节全读——产品式手测标准+三违例事故账+形态方向+首轮裁定+Persona 否决+上一版定稿）；[rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md](../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) §五-7/8/10/15/16/17/18/19、§六、§七；[first-run-twinkle level.yaml](../levels/first-run-twinkle/level.yaml) + [persona_default.md v1.1](../levels/first-run-twinkle/persona_default.md)；[docs/CONTEXT_LAYERING_V1_DESIGN.md](../../docs/CONTEXT_LAYERING_V1_DESIGN.md) §2.2（L2 用户偏好层）；[M2-HANDTEST-PREP-1 回执](../cards/done/2026-11-08-M2-HANDTEST-PREP-1.md)（事故细节）
- 消费方：主管终验 + 用户裁定（分档与 L1-d 两案）→ 拆实现卡
- 本稿不改任何在案裁定；发现的设计-裁定张力全部集中在 §7（冲突面与协调点），无一自作主张

---

## 0. 设计目标一句话

把 2026-11-08 用户首验后的正确形态落成可实现的设计：**用户从 Godot 拉起 GUI 进入工程 → 搭档（agent 人格）自动开场 → 全程对话式共创**——零终端指令、零环境变量、零手动服务（产品式手测标准，decisions §一）；并为「命名/人格制定环节」给出三段式 onboarding 的定稿细化（decisions §六上一版定稿照骨架细化，骨架不动）。

---

## 1. 现状事实基线（本轮亲核锚点，实现卡直接引用）

| # | 事实 | 锚点 |
|---|---|---|
| F1 | 乐局目录解析=env `VIT_EDU_LEVELS_DIR` 优先 → 回退 `coord/levels`（**相对 cwd**）；前端拉起 agent 时 cwd≠仓库根且无 env → 卡装载 fail-closed（违例一根源） | `agent/internal/chat/edu_jam_chat.go:1108-1113` |
| F2 | 乐局进入=命令词「开始乐局/进入乐局/开局乐局/edu_jam enter」（确定性正则闭集），自然语不入局；enter 幂等拒绝重复（活跃局在案即拒） | `edu_jam_chat.go:92-125, 214-277` |
| F3 | 开场叙事=模板直出（`eduJamRenderEnter(card)`，零 LLM）；乐局面（enter/推进/问询/终局）全部确定性拦截，与 LLM 会话轮正交 | `edu_jam_chat.go:279`；281 卡回执 |
| F4 | 乐局会话态 `eduJamSession`：conversation_id+project_uuid 双绑定、status、Jam 节点态、事件账；持久化走 `projectAgentRuntimeState`+`persistCurrentProjectWorkspace`（工程旁 agent 工作区），restore 逐会话 fail-closed | `edu_jam_chat.go:61-75`；`server.go:206, 7430, 7542, 7595` |
| F5 | **B2 事故教训**：chat 请求内 workspace activation 与 EduJamSession 整表 restore 存在时序竞态（activation 前开局 → 空态 restore 整表覆盖在途会话）——任何新的会话注入必须排在 activation+restore 完成之后 | 281 卡回执根因段 |
| F6 | agent 用户目录惯例两级在案：`~/.vit/config.json`（引擎配置，config 包）+ `~/.vit/genapi_providers.json`（生成 provider，genapi 包；`VIT_GENAPI_CONFIG` env 覆写供测试）——「用户目录默认路径 + env 覆写 + 缺文件回退」是既有模式 | `agent/internal/config/config.go:145-149`；`agent/internal/genapi/config.go:16-27` |
| F7 | Godot 聊天面板已有「Ask Vit API 设置」弹窗，走 `/agent/config` GET/POST——settings 面的现役先例（新节可挂同窗） | 前端仓 `app/shell/LLM_Chat_Controller.gd:3807-3843`（约 3839 行 add_child(panel)）；`server.go:717, 2507-2540` |
| F8 | YuE2=yue-server.exe 本地引擎（127.0.0.1:8087，4 个 gguf 模型参数，`/health` 可探活）；现由脚本手动 Start-Process；providers 配置已是用户目录文件（yue_local 条目样例在 ExampleConfig） | `scripts/run_yue_dialect_smoke.ps1:223-245`；`genapi/config.go:31-68` |
| F9 | persona_default.md v1.1 已终审态：卡装载+校验已接（lesson_card.go），**系统提示词注入未接线**（现为 voice spec）；`{model_name}/{user_name}` 由运行时替换（消费方未接） | `coord/levels/first-run-twinkle/persona_default.md:5` |
| F10 | 乐局状态数据已在 `eduJamSession`（节点进度/状态/事件账），但无只读 HTTP 披露面（「现在第几关」是命令词走 chat 轮）；HUD 缺的是披露面+GUI 窄条 | `edu_jam_chat.go:101-107`；§4 |
| F11 | 手测工程布位=m2_handtest_prep.py（save_as_project 到 `%APPDATA%\Vit-DAW\projects\手测·小星星乐局.vit`，启动页列表=get_recent_projects 同源）；预演事故=布位腿同一工程实跑 enter→n0→exit（违例三） | `scripts/m2_handtest_prep.py`；311 回执 |
| F12 | CONTEXT_LAYERING_V1（L1-4）状态=**规划未实现**：L2 用户偏好层载体设计为 `<用户级 workspace>/user_profile.v1.json`，ProfileEntry{PrefID/Statement/Class/Condition/EvidenceRefs/Status/SupersededBy/ConfirmedAt/Source}，聚合键=人（v1 user_id=local），缺文件=空层非错误 | `docs/CONTEXT_LAYERING_V1_DESIGN.md:3, §2.2, §8.3` |
| F13 | 展演形态裁定：YuE2=**局外成品**（离线渲染进 deck，GEN-YUE-DIALECT-1 留答辩后，不进 12 月链）；n4/n5 生成后端可插拔（1b 条件卡押后检查点） | rulings §七-20/21/22 |
| F14 | 手测入口裁定（2026-09-12）：用户从 Godot 拉起 DAW 前端再进工程；脚本栈仅供自动化烟测——本设计全部「产品级」判据以此链路为准 | AGENTS §5 |

---

## 2. L1 即修件设计（产品式手测 v2 硬前置，四件）

> 四件共同验收前提：**用户手测路径上零 env、零终端、零手动服务**（decisions §一）。每件给方案 / 改动面文件域 / 验收口径（零栈单测 + 真栈腿）/ 估时，直接对应 §6 拆卡。

### 2.1 L1-a 乐局目录产品级解析（去 VIT_EDU_LEVELS_DIR env 前置）

**方案**（对齐 F6 既有「用户目录默认 + env 覆写」模式，不新增配置面）：

1. 解析顺序改为三级：`VIT_EDU_LEVELS_DIR`（**保留**，测试/开发态覆写，genapi `EnvConfigPath` 同款语义）→ `~/.vit/levels/`（**产品主落点**，安装布局）→ `coord/levels`（仓库相对路径，开发态现状回退，保留兼容）。
2. 安装布局=`~/.vit/levels/<level_id>/`整卡目录（level.yaml + persona_default.md + assets/ + tools/），与仓库 `coord/levels/` 结构一致——装载逻辑（`loadEduJamLevelCard` 按 `<dir>/<levelID>/level.yaml` 寻址）零改动，只改目录解析函数。
3. 内容分发归发布/布位面，agent 不自装：布位脚本（2.3 的重铺腿）与发布脚本（build_release.ps1 可选段）负责把 `coord/levels/first-run-twinkle/` 复制到 `~/.vit/levels/`，带哈希核对（MANIFEST 同源纪律，§10）。
4. **不进 config.json 键**：理由——手测零配置目标下，多一个配置键=多一个出错面；两级先例（config/genapi）证明「固定默认+env 覆写」够用；若未来需要用户改目录，届时加键不迟（缺省语义=键缺省走三级解析）。
5. 失败语义维持 fail-closed：三级都解析不出可用卡时，enter 报错文案产品化（「乐局内容未安装」+指引，不暴露 env 名）。

**改动面文件域**：`agent/internal/chat/edu_jam_chat.go`（`eduJamLevelsDir()` ~10 行 + enter 报错文案）；`agent/internal/chat/edu_jam_chat_test.go`（解析顺序表驱动测试）；`scripts/`（布位/重铺腿加 levels 安装段，见 2.3）。

**验收口径**：
- 零栈单测：三级解析顺序×存在性矩阵（env 设/不设 × ~/.vit/levels 有/无 × cwd 在/不在仓库根）全表驱动断言；env 态用 `t.Setenv`（既有测试先例 `edu_jam_chat_test.go:106`）。
- 真栈腿（按 §5 三件套模式）：**不设任何 env、agent 以非仓库根 cwd 由脚本/Godot 链路拉起** → chat「开始乐局」enter 生效（回执含 first-run-twinkle 开场叙事，证明卡自 `~/.vit/levels/` 装载）→ exit 0。
- 产品式判据：手测步骤清单 §〇 删除「必做环境前置 VIT_EDU_LEVELS_DIR」整条。

**估时**：0.5 天（含布位腿安装段与测试）。

### 2.2 L1-b 进入工程自动开场（+重进恢复播报）

**方案**：

1. **工程↔level 绑定元数据**（声明性绑定，与 F4 会话态区分）：工程旁 agent 工作区 sidecar `.vit_agent/legame_binding.json`：

```json
{
  "schema_version": "vit.legame_binding.v1",
  "level_id": "first-run-twinkle",
  "bound_at": "2026-11-08T21:00:00+08:00",
  "bound_source": "placement_script | project_wizard(预留)",
  "opened_at": null,
  "opened_conversation_id": null
}
```

   写入者=布位脚本（手测工程，本卡）与未来新建工程向导（产品线，本卡只留 `bound_source` 枚举）；agent 只读。旧态缺省：无 binding 文件=普通工程，行为与现状零差异（§11 兼容义务）。

2. **接线点**（避 F5 竞态）：chat 首请求的 `activateCurrentProjectWorkspace` + restore 完成之后的检查点（`handleChat` 既有前导段尾部，server.go:2584-2586 附近 ~15 行插点）。注入条件五项全查：binding 在案 ∧ `opened_at` 为空 ∧ restore 后该工程无活跃（非终态）EduJamSession ∧ enter gate 通过（工程已开好）∧ 当前请求是 chat 主链路（非 note/工具回调）。条件满足→复用 `eduJamEnter` 内核建局+开场叙事作为本会话 assistant 消息注入 → 写 `opened_at`+`opened_conversation_id`（binding 文件回写）。
3. **幂等/重入语义**（「开场是工程级一次性事件，恢复是会话级播报」）：
   - **重进工程（跨进程重启/同会话再触达）**：restore 出活跃局→不重开场，注入**恢复播报**（确定性模板，零 LLM）：「接着上局：{节点标题}（{n}/{total}）——继续或问我都行」；终态局（completed/exited）→静默（HUD 照常显示终态）。
   - **多会话**：`opened_at` 已写→第二会话不注入开场；乐局面照常服务该会话（「现在第几关」/推进检查/HUD），即多会话共享工程级局、各自会话可推进——与 F4 会话模型一致（同一 conversation 粒度上的幂等由既有 eduJamSession 保证）。
   - 「开始乐局」命令词**保留**：手动入口（开发者路径+将来新开局），自动开场与命令词走同一 enter 内核，语义单源。
4. **竞态回归测试**（F5 教训入卡）：单测建模「请求内 activation 切换」场景——注入必须观察到 activation+restore 完成后的最终态，不得在 pre-activation 窗口写会话表或 binding。

**改动面文件域**：`agent/internal/chat/edu_jam_chat.go`（开场注入+恢复播报+binding 读写 ~120 行）；`agent/internal/chat/server.go`（handleChat 检查点插点 ~15 行）；`scripts/m2_handtest_prep.py` 或同类布位脚本（写 binding 段）；对应测试文件。

**验收口径**：
- 零栈单测：binding×会话态矩阵（binding 有/无 × opened_at 空/非空 × 活跃局/终态局/无局 → 注入开场/恢复播报/静默/零差异 四象限全断言）；幂等断言（同工程第二会话不重开场）；F5 型竞态回归测试。
- 真栈腿：布位带 binding 的乐局工程 → 三件套拉起 → chat 发任意白话（如「在吗」）→ 回执=开场叙事（含 `edu_jam_entered` 事件，**无命令词**）→ 同工程新开会话发白话 → 回执=恢复播报非重开场 → exit 0。
- 端测边界声明：Godot 聊天面板视觉呈现不在断言面（API 级验证，GUI 最后一米=用户手测本体——AGENTS §5 既有边界如实声明）。

**估时**：1 天。

### 2.3 L1-c fresh 工程保证（重铺腿+预演副本制）

**方案**（堵违例三：用户打开被预演过的工程）：

1. **重铺腿**：布位脚本加 `-Relay`（重铺）函数：删除目标工程 `.vit` + 工作区（`.vit_agent/`、`.vit_history/`）→ 重新布位 fixture（3 轨 42+42 音+t3 留白）→ 回读核对（clips 42/42 + `get_recent_projects` 首项=布位路径 + **binding 在案 + 会话残留为零**：无 EduJamSession、无会话历史）。幂等：连续两次执行结果恒等。
2. **预演副本制**（推论入律：前置腿只验副本/旁路，或交验前重铺——decisions §一推论）：一切预演/彩排/烟测**禁止在用户目标工程上实跑乐局面**——布位到隔离工作区副本（run 工件目录或 `%TEMP%` 专用目录，§10 防污染同源），目标工程仅作只读参照。手测交付检查单固定三问：重铺是否执行（exit 0 在卷）／目标工程哈希与重铺终态一致／无会话残留。
3. 步骤清单模板化：`coord/transfer/` 手测步骤清单的 §〇（前置声明）改为「零前置」（L1-a/b/d 落地后），检查单附重铺记录位。

**改动面文件域**：`scripts/m2_handtest_prep.py`（重铺函数+副本参数）或按同模式新增 `scripts/` 脚本段；`coord/transfer/` 步骤清单模板；零 agent 源码改动。

**验收口径**：
- 零栈（脚本域）：重铺腿连跑两次 exit 0 恒等；预演副本腿后断言目标工程 `.vit` 哈希不变 + `.vit_history` 会话数不增。
- 真栈腿：并入 L1-b 真栈腿（重铺→开场→重铺→再开场，验证重铺后 binding/会话残留归零、自动开场可再次触发——重铺即「新工程」语义自证）。

**估时**：0.5 天。

### 2.4 L1-d 生成引擎产品化前置（两案并列，交用户裁定）

**案 A：产品化（agent 托管 YuE2 生命周期）**
- 方案：genapi 加 engine lifecycle 段——provider 探测（yue_local 在案且 `extra_params.engine_path` 扩展键声明引擎位置）→ 惰性自启（首次生成请求触发，Start-Process 参数照 run_yue_dialect_smoke.ps1:229-241 同款）→ 健康检查（`/health` 轮询复用 `Wait-HttpReady` 语义）→ 失败引导（chat 路由回执产品化文案：「生成引擎起不来：{原因}。可以检查 {路径} 或先不用生成功能」，不阻塞非生成会话）。引擎关闭策略=随 agent 退（或空闲超时，实现卡定）。
- 改动面：`agent/internal/genapi/`（lifecycle ~200 行+config 扩展键）；`agent/internal/chat/` 生成路由失败引导文案；前端提示（可选）。
- 验收：零栈单测（lifecycle 状态机+失败分类）；真栈腿=不预启引擎→生成请求→引擎自起→路由三证据（311 目标 4 同款）→exit 0；失败路径=错误 engine_path→回执含引导文案非裸错。
- 估时：1.5-2 天。

**案 B：v2 手测裁出生成旅程（推荐）**
- 方案：手测 v2 范围裁出旅程 B（生成体验），步骤清单移除「YuE2 引擎+providers 手动前置」整段；YuE2 维持开发态（278 烟测脚本栈）；生成能力以「12 月展演=局外成品」口径对外（F13）。
- 改动面：`coord/transfer/` 步骤清单（删段+边界声明）；零代码。
- 验收：清单评审（无手动服务前置）；无真栈腿。
- 估时：0.2 天。

**建议**：**案 B 先行**——依据在案裁定链：§七-22 已裁 YuE2=局外成品展示（不进 12 月链）、n4/n5 后端可插拔（1b 押后检查点）；案 A 的产品化价值在真实用户分发场景，属产品线任务，且 VRAM/引擎稳定性风险不该进 12 月窗口。**此为设计建议，最终归用户裁定**（裁定点见 §5 分档表）。

---

## 3. Onboarding 设计定稿（三段式，骨架=decisions §六上一版定稿，照此细化不改骨架）

### 3.1 骨架与触发

```
[中介段] 机构经理（中介 UI）问 3 个硬问题 —— 用户替搭档起名、报自己称呼、选底版
   ↓
[交接段] 搭档本人开口问 3-4 个软问题（全可跳过）—— 每答一题，人格预览气泡变味
   ↓
[自述收尾] 搭档用生成完的人格自我介绍（一次 LLM 调用）—— 进聊天面板首条
```

- **触发**：全局一次性——agent 无搭档档案（`~/.vit/partner_profile.v1.json` 缺失）时，Godot 聊天面板首次打开显示引导卡「先认识一下你的搭档」（进入/暂不进入两键；暂不进入则每次面板打开低调提示一次，不劫持工程会话）。
- **不与乐局开场打架**：onboarding 是全局层（先做），乐局自动开场是工程层（后做）；未完成 onboarding 也可直接进工程——此时 persona 走卡内 `persona_slot` 底版（F9），onboarding 完成后的工程会话才用全局搭档（优先级规则见 3.6）。
- **归属语义**（decisions §五-2 照录）：搭档全局继承，settings 可切换（3.7），新建工程时可选（新建向导=产品线，本稿留接口）。
- **形态=对话式引导**（裁定五-3「问题引导共塑」）：中介段是结构化问答卡（输入框+选项），交接段是搭档在聊天面板里开口问——不是静态表单。

### 3.2 中介段硬问题 3 题

| # | 问题 | 必答 | 形态 | 落库字段 |
|---|---|---|---|---|
| Q1 | 它叫什么？ | **必答** | 文本框；空则展示候选名集（例：「阿和 / Echo / 小满 / Vitto / 阿盐 / 阿稳」——风格多样供点选，也可自填） | `partner.display_name` |
| Q2 | 怎么称呼你？ | **必答** | 文本框 | `user.display_name` |
| Q3 | 先给它挑个底版？ | 选答（可跳过） | 三选一卡：**制作人**（直接、爱分享推理）/ **陪练**（耐心、循序渐进）/ **沙龙主理人**（发散、爱聊音乐本身）；UI 标注「**只是起点——后面聊几句就变了**」 | `partner.base_style`（缺省=producer） |

### 3.3 交接段软问题 3-4 题（搭档开口问，全可跳过，每答一题气泡变味）

| # | 问题（搭档语气原样问） | 答案形态 | 落库字段 |
|---|---|---|---|
| S1 | 「最近在听什么？」 | 自由文本 → 口味锚 | `partner.taste_anchors[]`（同时落用户档案 `user.taste`） |
| S2 | 「你习惯怎么学？」 | 三选一：先例后理 / 先理后例 / 直接上手 | `partner.teaching_frame` |
| S3 | 「我说话直接点还是温和点？」 | 二选一：直接 / 温和 | `partner.tone` |
| S4（可选） | 「这一局你想要点什么？」 | 自由文本 | **会话/工程级**（`session_wish`，不进全局档案——进乐局会话上下文，见 3.5 缺省语义） |

每题答完→气泡立即重渲染（3.4）；跳过→该维度保持缺省味。**问答有可见后果**（附录 A 教训三的落地）：答 S1 后气泡引用用户提到的音乐、答 S3 后气泡语气整体切换。

### 3.4 人格预览气泡=确定性模板渲染版（零 LLM 零延迟）

- **机制**：profile 参数 → 固定样例句集拼接重渲染。纯函数（agent 侧单一真相：Go 函数产出句集 JSON，随 onboarding state API 下发；前端组件纯展示零逻辑——前后端无双份映射表）。
- **渲染规则**：气泡=2-3 句=底版基础句 + 语气对照句 +（有值时）教学框架句/口味引用句；`{user_name}` 始终在位（双向称呼的可见后果）。参数缺省=该句不渲染，气泡始终至少 2 句。
- **样例句集**（每参数维度≥2 味对照；实现卡可直接采用为 v1 句集，文案终审归主管）：

| 维度 | 值 A（句） | 值 B（句） |
|---|---|---|
| 底版·producer | 「{user_name}，我先把我的想法摆桌面上，你来砍。」 | 「这段我为什么这么写，说给你听——」 |
| 底版·coach（陪练） | 「{user_name}，不急，这一小节咱们多听两遍。」 | 「你已经听出不对劲了，就差最后一步。」 |
| 底版·salonhost | 「{user_name}，这句让我想起好几种写法，都值得聊聊。」 | 「先不急着改——这旋律本身有点意思。」 |
| 语气·direct | 「第 10 小节第 6 个音低了半音，改它。」 | 「这里不对。咱们直接定位到那一个音。」 |
| 语气·gentle | 「我听第 10 小节好像有点不太一样，你再听听看？」 | 「有一个音想跟你商量一下，你先听听第 10 小节？」 |
| 教学·example_first | 「先听这两句——像不像？只是搬了家。」 | 「别管术语，先听，听出重复感我们就赢了一半。」 |
| 教学·theory_first | 「这叫模进：动机在不同音高上重复。来，听例子。」 | 「先说清楚规则，你再听就明白哪里在作弊。」 |
| 教学·hands_on | 「你来改改这个音，改完我们听效果。」 | 「动手。改错了也没关系，撤销键就在那儿。」 |
| 口味锚·有 | 「你最近在听的那个，这里的处理跟它一个思路。」 | 「按你的口味，我猜你会喜欢这个方向。」 |
| 口味锚·无（对照=不渲染） | —（该句省略，气泡仍是完整 2-3 句） | — |

### 3.5 profile schema（字段表+持久化位置+旧态缺省语义）

**搭档档案**（人格参数的权威源）：`~/.vit/partner_profile.v1.json`（用户级、工程包外，与 config.json 同域——「全局继承」的物理形态）

| 字段 | 类型 | 缺省（旧态/未答） | 消费方 |
|---|---|---|---|
| `schema_version` | string | 必填 `"vit.partner_profile.v1"` | 读写器 fail-closed 校验（未知版本拒载+报错文案） |
| `partner.display_name` | string | 无档案=未 onboarding（触发引导）；有档案缺字段=报错（必答题不可能缺） | persona 块 `{model_name}` 槽 |
| `partner.base_style` | enum producer/coach/salonhost | `producer` | persona 块底版段选择 |
| `partner.tone` | enum direct/gentle | 继承底版默认（producer/hands_on 倾向 direct；缺省=`direct`） | persona 块语气段+气泡 |
| `partner.teaching_frame` | enum example_first/theory_first/hands_on | 底版内置框架（producer=example_first、coach=example_first、salonhost=hands_on） | persona 块教学框架段+气泡 |
| `partner.taste_anchors[]` | []string | 空（不渲染口味段） | persona 块口味段+气泡 |
| `partner.pacing` / `verbosity` | enum（复盘调音回写，3.7） | 缺省 `normal` | 乐局运行时拉回参数（guidance 面，281 消费） |
| `user.display_name` | string | 同必答规则 | persona 块 `{user_name}` 槽（快照冗余，权威源=用户档案，见下） |
| `updated_at` / `updated_source` | 时间/来源票据 | — | 再调音审计 |

**用户档案**：`~/.vit/user_profile.v1.json`——**对接 CONTEXT_LAYERING_V1 §2.2 既有载体设计**（ProfileEntry 结构原样采用，不发明第二 schema）。onboarding 是它的第一个确权来源（`Source="onboarding:<session_id>"`、`Class="core"`、`Status="active"`）：

| 条目（PrefID 稳定 ID） | Statement 形态 | 进 L2 前缀渲染 |
|---|---|---|
| `user.display_name` | 「称呼我为 {name}」 | 是（core） |
| `user.taste` | 「最近在听：{自由文本}」 | 是（core） |
| `user.learning_style` | 「习惯先例后理/先理后例/直接上手」（答 S2 才有） | 是（core） |
| `user.expression_preference` | 「希望表达直接/温和」（答 S3 才有） | 是（core） |
| `user.session_wish` | **不进全局档案**——工程/会话级，落乐局会话上下文（eduJamSession 事件或工程账本条目） | 否 |

**旧态缺省语义（§11 义务）**：两档案均为新增文件，缺文件=未 onboarding（搭档）/空层（用户偏好），非错误、零迁移；CONTEXT_LAYERING T-C4 同款往返测试进实现卡。**依赖声明**：L1-4-IMPL-B 未实现，onboarding 后端卡自带 user_profile.v1.json 最小读写器（按 §2.2 schema 子集），L1-4-IMPL-B 落地时收编该读写器进 contextruntime——文件域协调点见 §7-1。

### 3.6 persona 块消费与注线（onboarding 是注线的参数来源）

- **渲染管线**：搭档档案参数 → persona 块模板（persona_default.md v1.1 的参数化扩展版——底版/语气/教学框架/口味四段模板槽，v1.1 作为 producer 底版全量样例已在案）→ `{model_name}`/`{user_name}` 运行时替换 → 注入系统提示词（**注线本体=F9 未接线项，本设计收口其消费端**）。
- **优先级规则**（裁定五-2「全局继承+可选」的落法）：会话建立时 persona 解析=**用户显式选定搭档（settings/未来新建向导）> 全局默认搭档（partner_profile）> 卡内 `persona_slot`（底版缺省）**。first-run-twinkle 的 default_producer 在未 onboarding 用户处继续生效（手测兜底路径不变）。
- **注线范围 v1**：①persona 块进 chat 系统提示词装配（普通会话轮）；②乐局拦截轮（enter/推进/问询）模板直出 v0 保持（281 现状），仅做 tone 参数化（直接/温和两套问询模板词汇——确定性，零 LLM）。语义模板的 LLM 人格渲染=产品线（§五-19 ② 的完整形态）。
- **结构 × 人格分离不变式（§五-19）**：persona 块只管「怎么说」；节点推进/判定/确认全部留在确定性拦截面——注线不得给人格任何「替用户确认/自宣完成」的措辞空间（persona 块内已写铁律句，运行时另有结构层执法，双保险）。

### 3.7 再调音（两种路径）

1. **settings 重开访谈（增量改）**：设置窗（F7「Ask Vit API 设置」同窗加「搭档」节，或独立「搭档」窗——实现卡按 UI 一致性定）：展示当前档案摘要+气泡预览；「重新聊聊」按钮→增量访谈（只重问要改的维度，逐题更新逐题气泡变味）；改完 `updated_source` 记票据，用户档案条目走 `superseded` 链（CONTEXT_LAYERING 语义，不删历史）。
2. **乐局复盘节拍调音（运行中自然调）**：乐局 settle 段（终局沉淀，level.yaml `settle.memory_writes` 已有挂点）加搭档问询：「这一局，我的节奏快了吗？太啰嗦了吗？」→ 答案回写 `partner.pacing/verbosity`（三档枚举）→ 下局生效。**写回走档案更新通道（同 1），不另开第二真相**。此腿=~10 行 settle 钩子扩展+问询模板，归 ONBOARD-BE 卡但触 281 域文件，协调点见 §7-3。

### 3.8 设计不变式（轨道铁律，写进设计不随实现漂移）

1. **人格不可替用户确认节点**——节点 done 判定/闭合确认/终局确认全部在确定性拦截面，LLM 人格轮的确认类措辞不构成事件（`user_confirmed` 类事件只由命令词/结构化应答产生）。
2. **人格不可自宣完成**——completed 状态只由闭合判定+用户确认链路写入；persona 块含铁律句+运行时结构层执法（双保险，persona_default.md:6 同款）。
3. **人格不可染指判定权分布**——耳朵/审美/意图类判定问用户、符号/参数类模型自查（§五-7/19），persona 块无权重排。
4. onboarding 全程答案**只用于人格渲染与偏好层**，不得进入判定器参数（判定参照=level.yaml notes/assets，与人格无关）。
5. 以上为运行时层执行的结构约束，**不依赖人格自觉**；violations 应在拦截面有测试覆盖（红测：伪造人格轮确认类输出→节点状态不变）。

---

## 4. Session HUD（L2.5：关卡进度窄条）

- **GUI 落点（勘察结论）**：**Godot 侧**——乐局交互与手测链路都在 Godot 聊天面板（`app/shell/LLM_Chat_Controller.gd`，前端仓 `D:\Godot\project\vit-daw-frontend`，**仓库外工程，拆卡注明双仓域**）；HUD=聊天面板顶部窄条 Control（新组件文件，非侵入 LLM_Chat_Controller 主逻辑，数据源实接新 GET 面——§5.1 探针装配面口径：生产场景树节点在场+数据源实接，禁止探针自建实例）。webui（React）侧 HUD=后续可选（手测链路不走 webui）。
- **数据面**：agent 新增只读 GET `/agent/edu_jam/status?conversation=<id>`（对齐 `g_runtime_readonly_smoke.ps1` 白名单 GET 模式；数据源=eduJamSession+`eduJamCurrentNodeID`，F10 纯披露零新状态）：

```json
{ "status": "ok", "level_id": "first-run-twinkle", "level_title": "一个旋律的变奏（小星星乐局）",
  "jam_status": "active", "current_node_id": "n3_find_wrong_note", "current_node_title": "找错音",
  "node_index": 3, "node_total": 7, "nodes_done": ["n0_photo_score_open", "n1_read_and_fix", "n2_set_tempo"] }
```

  无局时 `{ "status": "ok", "jam_status": "none" }`（前端隐藏窄条）。更新节奏=前端随既有 chat 轮询/事件面节拍拉取（不新增长连接）。
- **最小形态**：单行窄条「🎼 小星星乐局 · 找错音 · 3/7」+状态色（进行中/已收尾灰化；无局隐藏）——不做地图/不做时间轴（那是产品线成长树面）。窄条点击=展开当前节点 goal 一行（level.yaml node.goal 直出，零 LLM）。

---

## 5. 分档表：12 月切片 vs 产品线

| 项 | 档位 | 依据/说明 |
|---|---|---|
| L1-a 目录解析 | **12 月必须** | 事故账违例一直接对应；手测 v2 硬前置 |
| L1-b 自动开场+恢复播报 | **12 月必须** | 用户首验核心诉求（decisions §二/§四）；违例三「进入后无引导」的对症件 |
| L1-c 重铺+副本制 | **12 月必须** | 违例三制度性收口；每次手测交付前执行 |
| L1-d 案 B（裁出生成旅程） | **12 月必须（建议）** | §七-22 YuE2=局外成品在案；**案 A vs B 归用户裁定** |
| L1-d 案 A（引擎托管） | 产品线（若裁定 A 则 12 月+1.5-2 天） | 真实分发场景价值；VRAM 风险高 |
| Onboarding 三段式+气泡（BE+FE 两卡） | **建议 12 月**（待用户裁定） | 裁定五-4「先讨论好设计再定」——本稿即设计输入；量级 3-3.5 天 |
| Session HUD | 12 月可选（搭车件） | 量级 1 天；无 HUD 不阻塞手测（「现在第几关」命令词在） |
| 复盘节拍调音 | 12 月可选 | ~0.2 天，随 ONBOARD-BE；settings 重开访谈=随 ONBOARD-FE 必带 |
| 人格包系统（多档案/导入导出/切换市场） | 产品线 | 裁定六+§五-19（12 月=最小人格槽） |
| 成长树/人格随局进化 | 产品线 | §五-19； rulings doc 1 线 |
| personal-agent 独立壳 | 产品线（已否决独立 app） | 裁定五-1：壳=内嵌工程形式 |
| 即兴乐局/推断触发小课 | 产品线 | §五-16/17 |
| 语义模板 LLM 人格渲染 | 产品线 | §五-19 ② 完整形态；v1=模板直出+tone 参数化 |

**12 月窗口量级汇总（建议案）**：L1 三件 2 天 + 案 B 0.2 天 + onboarding 3-3.5 天 + HUD 1 天（可选）≈ **5.2-5.7 天**（不含 HUD 4.2-4.7 天）——对照 11 月中开工舒适/11 月底压线的时间轴（rulings §一-6），进 12 月可行但须尽早开工。

---

## 6. 实现拆卡清单（7 卡，各卡=目标/文件域/验收[零栈单测+真栈腿]/估时）

| # | 卡（建议名） | 目标 | 文件域 | 验收（零栈单测 + 真栈腿） | 估时 | 依赖 |
|---|---|---|---|---|---|---|
| 1 | **LEGAME-DIR-1** | L1-a：乐局目录三级解析+布位安装段 | `chat/edu_jam_chat.go`(~10 行)+`edu_jam_chat_test.go`+`scripts/` 布位安装段 | 单测=三级解析矩阵（2.1）；真栈腿=无 env 非 repo cwd 三件套→enter 生效→exit 0；清单删 env 前置 | 0.5 天 | 无 |
| 2 | **LEGAME-AUTOINTRO-1** | L1-b：binding 元数据+自动开场注入+恢复播报 | `chat/edu_jam_chat.go`(~120 行)+`server.go`(handleChat ~15 行插点)+布位脚本 binding 段+测试 | 单测=binding×会话四象限+幂等+F5 竞态回归（2.2）；真栈腿=白话触发开场→重进恢复播报→exit 0；GUI 边界声明 | 1 天 | 卡 1（布位段同改） |
| 3 | **LEGAME-FRESH-1** | L1-c：重铺腿+预演副本制+检查单模板 | `scripts/` 布位脚本扩展+`coord/transfer/` 清单模板；零 agent 源码 | 脚本域=重铺幂等两连跑+副本制哈希断言（2.3）；真栈腿=并入卡 2（重铺后再开场） | 0.5 天 | 与卡 2 布位段协调（建议同领或卡 2 先） |
| 4 | **LEGAME-GENSCOPE-1** | L1-d 案 B：手测 v2 裁出生成旅程（若裁定案 A 则另拆 LEGAME-GENHOST-1：genapi lifecycle+失败引导，1.5-2 天，真栈腿=自起+失败引导双路径） | 案 B=`coord/transfer/` 清单；案 A=`genapi/`+chat 路由文案 | 案 B=清单评审；案 A=单测 lifecycle 状态机+真栈自起/失败双路径 exit 0 | 0.2 天 / 1.5-2 天 | 用户裁定案别 |
| 5 | **LEGAME-ONBOARD-BE-1** | onboarding 后端：状态机（idle→mediating→handoff→intro→done）+两档案读写器+气泡句集纯函数+persona 块渲染与注线（优先级规则）+settle 复盘问询钩子 | `chat/legame_onboarding.go`(新)+`chat/edu_jam_chat.go`(settle ~10 行插点)+persona 模板参数化文件（levels 域新模板或参数化 v1.1）+测试；**user_profile 读写器归属协调见 §7-1** | 单测=状态机全迁移+缺省语义往返（§11）+气泡纯函数快照（每维度≥2 味）+注线优先级+铁律红测（3.8-5 伪造人格确认→节点不变）；真栈腿=API 驱动全流程（中介 3 答→软问题 2 答 1 跳→自述 1 次 LLM 调用[§8：1 调用/预算≈¥0.05/失败分类：超时·空回退·拒绝→模板兜底]）→档案落盘核对→exit 0 | 1.5 天 | 无（与卡 2 正交；注线消费 persona_default v1.1 在案） |
| 6 | **LEGAME-ONBOARD-FE-1** | onboarding 前端：引导卡+中介问答 UI+交接段对话+气泡组件（消费 agent 句集 JSON）+settings「搭档」节+重开访谈 | **前端仓** `app/shell/`（LLM_Chat_Controller.gd 域+新组件文件）——仓库外工程，卡面注明 | Godot 侧=探针装配面断言（§5.1：生产场景树节点在场+数据源实接）+引导流程 GUI 走查腿；渲染面边界显式声明（AGENTS §5：E2E-WEBUI-1 建设卡之前） | 1.5-2 天 | 卡 5（API 就位） |
| 7 | **LEGAME-HUD-1** | HUD：GET 披露面+Godot 窄条 | agent 面=`chat/server.go`+`edu_jam_chat.go`(只读拼装 ~40 行)；Godot 面=前端仓新组件+LLM_Chat_Controller 挂点 | agent 面=单测（有局/无局/终态三态 JSON）+只读烟测（g_runtime_readonly_smoke 白名单加 GET）；Godot 面=装配面断言（窄条在场+数据实接） | 1 天（agent 0.5+Godot 0.5） | 无（可与 5/6 并行） |

拆卡说明：①卡 2 与卡 3 布位脚本同文件，建议同领或 2→3 顺序；②卡 5 的 settle 钩子触 281 已合入域（`edu_jam_chat.go`），文件域冲突在卡面预声明即可（281 已验收合入，非在飞卡）；③卡 6/7 的 Godot 面在仓库外前端工程——按 PROTOCOL §0.1 归 PC 侧管辖，卡面写明归属；④onboarding 进不进 12 月归用户裁定（§5），不进则卡 5/6/7 全部后置不影响 L1 四件。

---

## 7. 冲突面与协调点清单（无一自作主张，全部交裁定/协调）

1. **user_profile 读写器归属（协调点，非冲突）**：CONTEXT_LAYERING_V1（L1-4）状态=规划未实现，其 L2 载体 `user_profile.v1.json` schema 已定（F12）。onboarding 用户档案按同 schema 自带最小读写器（ONBOARD-BE 卡），L1-4-IMPL-B 落地时收编进 contextruntime 包。**协调动作**：ONBOARD-BE 卡面注明「user_profile.v1.json 读写器为 L1-4-IMPL-B 预留收编面」；两卡排期相遇时由主管定收编时点。不阻塞任何一方。
2. **persona 注线与 L1-4 前缀装配的迁移路径（设计内声明）**：v1 注线落现行 chat system 装配路径（追加段，快路径）；L1-4-IMPL-A/B 落地后 persona 段应迁移为 promptruntime Section（Stable=true，随 `profile_updated` 显式断裂）——迁移是 L1-4 实现卡的接线项，本设计不预支。v1 追加段对 KV cache 的影响=低频变化段（档案确权制，§2.2 断裂语义），可接受。
3. **ONBOARD-BE 的 settle 钩子与 281 域（文件域预声明）**：复盘节拍调音在 `edu_jam_chat.go` settle 段 ~10 行插点——281 已验收合入非在飞，无并行冲突；卡面预声明即可。
4. **level.yaml `persona_slot` 与全局搭档的关系（规则已定，非冲突）**：优先级=显式选定>全局搭档>卡内 persona_slot（3.6）；first-run-twinkle 无需改卡——其 persona_slot 语义从「本局人格」收窄为「未 onboarding 用户的底版缺省」。**若主管认为需要卡面注释级改动（level.yaml 注释一行），归 ONBOARD-BE 卡内最小改动并卡面声明**。
5. **自动开场 vs 乐局挂起/切换语义（一致性核验，非冲突）**：§五-16 挂起走 park/continuation、enter/exit/suspend 12 月只定接口——自动开场只在「binding 在案∧无活跃局」触发，恢复播报只读不动状态，均不触碰挂起语义；12 月无「切换」实现面，无冲突。
6. **未发现设计与在案裁定的实质冲突**——以上 1-5 均为协调点/预声明项；若主管验收时对 3.5 用户档案落点（`~/.vit` vs CONTEXT_LAYERING 的「<workspace>」表述）有不同读法，以裁定为准（本稿取「用户级目录」读法，依据=§2.2 聚合键=人+工程包外）。

---

## 附录 A. 市场校准注记（两段）

三个参照产品的教训与本设计的对应：**Replika** 用极短问卷（起名+几个兴趣）开局，人格深化延迟到聊天中完成——对应本设计中介段只问 3 个硬问题（必答仅双向称呼），人格厚度交给交接段软问题与运行期再调音，不在 onboarding 一次问完（防问卷疲劳）。**ChatGPT 自定义指令**证明两问是最小有效轴（上下文×表达）——对应本设计必答面收窄到「它叫什么/怎么称呼你」（关系轴），表达轴（直接/温和）降级为可跳过软问题，上下文轴（口味/学法）同为软问题。**问答须有可见后果（构筑非调查）**是三者共同的失败边界警示——问卷若只收集不反馈，用户答着答着就敷衍——对应本设计的两个强制反馈点：每答一题人格预览气泡立即变味（3.3/3.4，答案可看见地改变搭档），收尾自述=全部答案的一次总渲染（答案变成人格本身）。乐局终局的复盘调音（3.7-2）把「问答有后果」延伸到运行期：问了节奏感受，下局真的变节奏。

---

*本设计稿由 313 LEGAME-UX-DESIGN-1 卡产出（零实现零栈）；所有裁定引用以原文为准，本稿不自创裁定。分档裁定点（§5：L1-d 案别、onboarding/HUD 进不进 12 月）待用户拍板后拆实现卡。*
