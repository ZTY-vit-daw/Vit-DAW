# EDU-LEVEL-CONTENT-1：《小星星》乐局内容——乐局卡 v0 定稿 + 工程模板素材（乐局 2）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[rulings §五-10/16/18/19](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) + [257 报告](../../reports/2026-11-05-edu-runtime-recon-1.md) 块 2 修订三条）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 279；目标=coord 内容工件（乐局卡+素材规格）；P1 / ~1 天；L1-L2 / PC；零栈零 LLM；**设计先行可立即开工，定稿须在 276 验收后**（判定器枚举/返回形态对齐）；**前置裁定：18 终局确认+写入降级（用户）**

## 目标

1. **乐局卡 v0 格式定稿**（`levels/first-run-twinkle/level.yaml` 或 coord 下同级工件）：关卡节点（拍谱开局→读谱纠错→定速度→找错音=听辨模进/重复→选变奏方向→共同完成→命名+导出终局）+ 每节点 done_when（判定器枚举**只用首版 3 个**：note_sequence_match/pitch_set_match/rhythm_match）+ 教学目标/检查挂接 + **bpm 与小节起拍显式进卡**（257 修订②：判定器不做小节推断）+ persona_slot（默认人格，提示词块级）+ 拉回配置。
2. **终局判定落卡**（rulings §五-18）：闭合判定条件（结构完整=全节点 done_when+整曲可播；工作面安静）→ agent 问询→确认→命名+导出→沉淀（memory_writes 清单）。
3. **工程模板素材规格与制作**：主题 MIDI（公版旋律）、改错版（故意错一个音）、伴奏空白轨、bpm 默认值；素材为新建测试资产+可安装工程模板，出处标注（公版+Vit 原创编曲，doc 1 §3.4 版权口径）。
4. **写入边界声明进卡**（257 修订③）：本关允许的写入命令族清单；RiskConfirm 是否乐局内降级=用户裁定（默认不降级）。

## 约束

- 文件域=coord/levels/（或 docs 同级新目录）+素材文件；**不碰 agent 代码**（277 前不触 capability_pack）；
- 格式必须人可手写（专家可写原则，rulings §一-4）；YAML schema 带 schema_version。

## 验收标准

- 乐局卡 v0 全节点+done_when+终局判定完整，YAML 可解析，判定器名全部在首版 3 枚举内；
- 素材清单带哈希入档；格式与 280（3a）的 lesson_card 校验门需求一致（双方对齐签字=回执互引）。

## 停止条件

- 裁定 18/写入降级未下→定稿暂停（设计稿可先行），上交等待。

## 领取

- 2026-11-06 / **先行稿轮**（设计先行；定稿须 276 终验后+裁定 18 用户确认后另轮执行）
- owner=GLM 主会话 L2-worker 子代理（ZCode / PC）；验收负责人=GLM 主管决策流
- worktree=`D:/Vit_DAW_wt_279impl` @ 分支 `port/edu-level-content-1`
- 领取基线：origin/main `9eebccdf`；worktree `git status --short` 空、`git diff --stat` 空（净树领取）
- 文件域=coord/levels/（新目录）+本卡面；零 agent 代码零内核；零真栈腿不占栈（纯文档设计）；零 LLM 计费

### 定稿轮（2026-11-07）

- **前置齐备**（按主管定稿轮派发指令 2026-11-07）：①PD-1 裁定 18 用户确认=按现文生效（2026-11-07 凌晨；main rulings 文本的确认态回写属决策侧账务，本回执如实标注此差异）；②276 段 3 终验合入（rulings `2026-11-07-EDU-SYMBOL-CORE-1-pass` + merge `be76f431`；edu 包以 main 为准逐文件核对=与先行稿对齐表零漂移）；③写入降级=用户裁定不下、`confirm_downgrade: none`（默认不降级）。
- owner/worktree 续用先行稿轮：GLM 主会话 L2-worker 子代理（ZCode / PC）；worktree=`D:/Vit_DAW_wt_279impl` @ `port/edu-level-content-1`（已 ff 至最新 main）；卡面协调提交走独立分支 `coord/279-edu-level-content-1`（worktree `D:/Vit_DAW_wt_279coord`，PROTOCOL §2.1-2 卡面提交直推 main）
- 领取基线：origin/main `24f7280a`；impl worktree `git status --short` 空、`git diff --stat` 空（净树领取）
- 文件域=coord/levels/first-run-twinkle/ 内容工件+本卡面；零 agent 代码零内核零真栈零 LLM 计费；机械复算=edu 包只读复制到中立临时目录（276 附录 A 先例）

## 回执

### 先行稿轮（2026-11-06，GLM 主会话 L2-worker 子代理）

- **产物**：[coord/levels/first-run-twinkle/DESIGN-DRAFT-2026-11-06.md](../../levels/first-run-twinkle/DESIGN-DRAFT-2026-11-06.md)（自包含：schema 草案 §1 + 实例素材规格 §2 + 目录提案 §3 + 裁定 18 三锚点映射 §4 + 280 对齐清单 §5 + 开放问题 §6 + 定稿轮差额 §7 + 机械验证附录 A）；实现 commit `95990fba`（@ `port/edu-level-content-1`，已推 origin；原 30c06198 经 239 合入后 rebase 改号，内容零改动）；协调 commit（本回执）另推 main。
- **格式要点**：schema_version=`vit.level_card.v0`；done_when 三叶一阶文法（**判定器只用首版 3 枚举**——本实例 done_when 仅用 note_sequence_match；交互事件/结构谓词另立闭集）；timing 块 bpm+小节起拍显式进卡（257 修订②）；write_boundary 白名单+confirm_downgrade: none（257 修订③，默认不降级）；persona_slot 提示词块级（裁定 19）；finale 块按裁定 18 现文（完成先于导出/三锚点闭合/免问询直通/完成≠停止≠冻结）+budget 止损（§8）。
- **对齐**：276 检测核可判定字段逐项对齐表（sequence 双语义=Key 开关非请求参数、min≥2、range 半开容差、断言字段族、bar 守门=挂在 n2 后）；280 校验门拒绝路径 12 条 + 新增对齐项 6 条（互引面就绪，待 280 回执互引）。
- **判定预期机械验证（附录 A）**：以 276 核只读复制（@f2950cd7）离线复算——min 阶梯 30/20/8/6/6；B 窗口 [8,32)+min∈[5,7] 恰 3 条 findings；no-key 漏检全部 diatonic（必须带 key 实证）；改错轨差分=14 音 A→A' repeat 消失+8 音后截出现；judge 恰 1 处 mismatch（idx 33/窗口 5）。**初版四处手推误断言经复算推翻并已在轮内自纠**（诚实边界：修正记录进附录 A）。§2.5 YAML 块 PyYAML 解析 OK（16 顶层键/7 节点/judge 引用全在枚举内）。
- **开放问题 13 条**（§6，定稿前须确认项摘要）：①裁定 18 用户确认态（先行稿按现文设计）②276 段 3 终验后 schema 冻结 ③素材版权口径（公版+Vit 原创，发布前懂版权者核实）④RiskConfirm 乐局内降级（用户裁定，默认不降）⑤280 判定器三件语义定案 ⑥拍谱 OMR 容差（286）⑦谱面排版来源 ⑧人格块终审 ⑨真栈复跑校准（定稿轮占栈）⑩止损数值 ⑪空白轨豁免口径 ⑫素材力度约定 ⑬280 互测夹具物化与否。
- **状态**：先行稿交付完毕，**卡保持 doing——定稿轮待前置**（裁定 18 确认+276 终验+上述归属用户/主管项）；未做任何定稿动作（无 level.yaml 真文件、无素材二进制、未碰 agent 代码/capability_pack）。

### 定稿轮（2026-11-07，GLM 主会话 L2-worker 子代理）

- **产物**（impl commit `0ef7da9c` @ `port/edu-level-content-1`，已推 origin；8 文件 +803 行）：
  - `coord/levels/first-run-twinkle/level.yaml` —— `vit.level_card.v0` 首个定稿实例（蓝本=设计稿 §2.5）：42 音全表 + 7 节点（done_when 判定器名全在首版 3 枚举内，本实例仅用 note_sequence_match）+ finale（裁定 18 现文全量落卡）+ write_boundary（9 命令实名）+ **runtime_slots 三轨落位（新增块**=开放问题⑤-4 实例落位，§2.3 轨布局卡内化）+ budget（60/120）+ guidance；
  - 素材：`assets/theme_twinkle_v0.mid`（42 音，419B，sha256 `276e1ffb…8952`）/ `assets/theme_twinkle_corrupted_v0.mid`（@37→68，419B，sha256 `2a58dbfb…bd33`）。**MIDI 生成方式设计决策存档**：Python stdlib 确定性生成器（`tools/gen_theme_midi.py`，同输入逐字节相同；notes.theme 为唯一数据权威、MIDI=其投影非独立数据；format 0/PPQ 480/ch 0/vel 100 固定=开放问题⑫定稿；tempo meta 100bpm+4/4 仅播放器显示，工程 tempo 走 n2 set_tempo——import 不读 tempo meta）；`assets/MANIFEST.yaml`（哈希+出处+produced_by 存档；score PDF=deferred_286 登记 deferred_files，未入 assets——校验门拒绝路径 10 要求文件+哈希在档）；
  - `persona_default.md`（§2.7 骨架填充 v1；终审=开放问题⑧待用户/主管）；`coord/levels/README.md`（目录语义+格式索引=§3 提案落成）；`tools/check_level_yaml.py`（结构自检，280 校验门落地前作内容侧 lint）。
- **schema 对齐（开放问题②关闭）**：edu 包四文件（edu/transform/judge/command.go @ main `24f7280a`，276 合入版）与先行稿 §1.3 对齐表逐项核对=**零漂移**（parseAnalyzeArgs 字段族/半开窗口 1e-6 容差/min≥2/key 双语义/Finding 投影/bar 守门全一致）；§6-2 漂移修订规则未触发。write_boundary 逐名核对修正两处：`mutate_midi_notes`/`delete_midi_notes` 复数实名（先行稿单数为占位笔误）、`add_midi_notes_bulk` 独立实名分列（先行稿"含 bulk"表述修正）。
- **机械复算（附录 A 模式；main edu 包只读复制至中立临时目录 `D:/tmp_279_edu_verify2/`，零仓库改动零栈零 LLM）**：
  - 结构自检 `python tools/check_level_yaml.py`：**319 项通过 / 0 失败，exit 0**（PyYAML 解析；判定器名全在 3 枚举；事件 6/结构谓词 3 闭集；组合子单层；轨槽与资产引用可解；bar 引用域；write_boundary ⊆ 注册面；assets↔MANIFEST↔实际文件哈希三方对账）；
  - 检测核复算 `go test ./edu/ -v`（go1.26.2，同包复用 windowNotes/buildAnalyzeSegment 真代码）：**5 测试全 PASS，exit 0**——①min 阶梯 3/4/5/6/7→30/20/8/6/6（=附录 A 基线）；②hook 断言：concept_b_section 3 条 expected_findings 全对（proto_bars/match_bars 经 command.go bar 投影真代码换算）+diagnose expect_absent（14 音 repeat bars1-4→9-12）未出现/expect_present（8 音后截 bars2-4→10-12）出现；③B 窗口 [8,32) min∈{5,6,7} 均恰 3 条且内容一致；④差分：完好 6 条/改错 6 条、共有 5 条逐条一致；⑤judge 全曲恰 1 mismatch idx=33（actual 68/expected 69/proto 69，rhythm_matched=true）+窗口 [32,48) idx=5；
  - MIDI 回环对账：两文件音符↔level.yaml notes 表逐音相等（42/42；PPQ=480/ch=0/vel=100 约定核对）。
  - 诚实边界：check_level_yaml.py 初版两处口径缺陷（notes_ref 点路径解析、派生资产免 notes_ref）轮内自纠后全绿；extract_spec.py 初版 MIDI 解析器状态字节推进 bug 轮内自纠——均未影响定稿产物（生成器与 level.yaml 一次成型）。
- **开放问题终态（13 条全处置）**：①**关闭**——裁定 18 按现文生效（主管定稿轮派发指令 2026-11-07 述明用户确认；**main rulings 文本仍标"待确认"，回写=决策侧账务，如实标注**）；②**关闭**——零漂移；③卡内口径落定（公版+vit_original 入 MANIFEST license；发布前懂版权者核实=发布前置条件，保留）；④**关闭**——`confirm_downgrade: none`；⑤**开放（归 280 互引）**——本卡侧实例落位：runtime_slots 块+accept_transposed 两处（false）+组合子全单层+pitch_set_match 本实例零使用待 280 定案；⑥**开放（归 286）**——n0 fallback 已兜底；⑦口径**关闭**（自排 vit_original）+工件延后（deferred_286）；⑧**开放**——v1 骨架已交付，终审=用户/主管；⑨**开放**——本卡零栈，真栈复跑校准移交（280 运行时卡或后续占栈卡，届时按 PROTOCOL §2.2 登记）；⑩**定稿**=60/120+`preserve_scene_and_escalate`（运行期读数校准归 280/281）；⑪**定稿**=无豁免通道（用户明示留白不构成闭合条件）；⑫**定稿**=velocity 100 固定/channel 0；⑬**消解**——真 level.yaml 已在档，280 直接引用为互测夹具（无需从 §2.5 另行物化）。
- **280 校验门对齐面终态**：12 条拒绝路径在本实例全部有可判落点（1/2/3 枚举闭集、4 bar 域、5 range 形态、6 min≥2、7 key、8 transformations 白名单、9 node id、10 assets+sha256、11 schema_version、12 objectives check+verbal_rubric 显式豁免）；6 新增项：①事件/结构谓词枚举（本卡自检已按闭集 lint）②pitch_set_match 语义（待 280）③accept_transposed（本卡两处实证卡面可写）④组合子单层（实例全单层）⑤内嵌 notes 判定输入源（42 行实证零栈可判）⑥runtime_slots（**变化：本轮已落位为卡字段提案**——280 需裁定升格必检或保持实例惯例）。
- **端测覆盖边界声明（AGENTS §5）**：本卡纯内容工件（coord 下 YAML/MIDI/md/生成校验工具），零 agent 代码零内核零 webui——渲染面与用户旅程门槛不适用；本卡验收面=结构自检+机械复算（全绿如上）；真实栈校准=开放项⑨显式移交。
- **状态**：定稿轮交付完毕，自验通过，卡转 **done 待主管验收**；project_template（§3 提案位）未入 §7 差额清单本轮不做，留 280/后续卡。

## 验收

- **先行稿轮验收（2026-11-06 深夜，主管）：pass**——627 行自包含设计稿结构完整（schema/素材/目录/裁定 18 映射/280 对齐/开放问题/定稿差额）；**离线机械复算实证=质量典范**（276 核只读复跑全套+YAML 自检；初版四处手推断言被复算推翻轮内自纠留档——诚实边界执行到位）；开放问题 13 条归属清晰（用户 5/主管 3/280-286 执行侧 5）。定稿轮阻塞三件维持（PD-1 裁定 18 确认+写入降级+276 段 3 终验），卡保持 doing 合规。
- （定稿轮验收：待主管——定稿轮回执已交，见上定稿轮段）
- **定稿轮终验（2026-11-07 凌晨，主管）：pass——首个乐局卡定稿实例交付**——319 项机械复算全 PASS+卡面验收标准四条全达+开放项 13 条终态规范；裁定 18 账务已回写 rulings 文件；280 解锁发车。合入随本批。详见 [rulings/2026-11-07-EDU-LEVEL-CONTENT-1-pass.md](../../rulings/2026-11-07-EDU-LEVEL-CONTENT-1-pass.md)。
